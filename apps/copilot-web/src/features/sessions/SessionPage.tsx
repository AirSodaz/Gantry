import { useMemo, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { ArrowLeft, Ban, RotateCcw, Send } from "lucide-react";
import { Button, StatusMark } from "@gantry/design-system";
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useCopilotApi } from "../../api/ApiProvider";
import type { RunSummary } from "../../api/types";
import { ErrorState, LoadingState } from "../../components/AsyncState";
import { useSessionStream } from "./sessionStream";

const cancellableRunStates = new Set([
  "queued",
  "provisioning",
  "running",
  "awaiting_approval",
  "suspended",
  "canceling",
]);

export function SessionPage() {
  const { sessionId = "" } = useParams();
  const api = useCopilotApi();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [followUp, setFollowUp] = useState("");
  const [retryRevision, setRetryRevision] = useState<
    "original_revision" | "current_production_revision"
  >("original_revision");
  const followUpKey = useRef<string | null>(null);
  const cancelKey = useRef<string | null>(null);
  const retryKey = useRef<string | null>(null);
  const sessionQuery = useQuery({
    queryKey: ["session", sessionId],
    queryFn: () => api.getSession(sessionId),
    enabled: Boolean(sessionId),
  });
  const runsQuery = useInfiniteQuery({
    queryKey: ["session-runs", sessionId],
    initialPageParam: "",
    queryFn: ({ pageParam }) => api.listSessionRuns(sessionId, pageParam),
    getNextPageParam: (page) => page.page_info.has_more ? page.page_info.next_cursor : undefined,
    enabled: Boolean(sessionId),
  });
  const approvalsQuery = useQuery({
    queryKey: ["session-approvals", sessionId],
    queryFn: () => api.listApprovals(),
    enabled: sessionQuery.data?.my_action === "approval",
  });
  const stream = useSessionStream({ api, queryClient, sessionId });
  const cancel = useMutation({
    mutationFn: () => {
      const run = sessionQuery.data?.executing_run;
      if (!run) throw new Error("This session has no active run.");
      if (!cancelKey.current) cancelKey.current = crypto.randomUUID();
      return api.cancelSessionRun(sessionId, run.id, cancelKey.current);
    },
    onSuccess: () => {
      cancelKey.current = null;
      void queryClient.invalidateQueries({ queryKey: ["session", sessionId] });
      void queryClient.invalidateQueries({ queryKey: ["session-runs", sessionId] });
    },
  });
  const retry = useMutation({
    mutationFn: () => {
      const run = latestRetryableRun(runsQuery.data?.pages.flatMap((page) => page.items));
      const etag = sessionQuery.data?.conversation_etag;
      if (!run || !etag) throw new Error("Refresh this session before retrying.");
      if (!retryKey.current) retryKey.current = crypto.randomUUID();
      return api.retrySessionRun(sessionId, run.id, retryKey.current, etag, retryRevision);
    },
    onSuccess: () => {
      retryKey.current = null;
      void queryClient.invalidateQueries({ queryKey: ["session", sessionId] });
      void queryClient.invalidateQueries({ queryKey: ["session-runs", sessionId] });
    },
  });
  const continueSession = useMutation({
    mutationFn: () => {
      const etag = sessionQuery.data?.conversation_etag;
      if (!etag) throw new Error("Refresh this session before continuing.");
      if (!followUpKey.current) followUpKey.current = crypto.randomUUID();
      return api.appendSessionMessage(sessionId, { message: followUp.trim() }, followUpKey.current, etag);
    },
    onSuccess: (session) => {
      followUpKey.current = null;
      setFollowUp("");
      queryClient.setQueryData(["session", sessionId], session);
      void queryClient.invalidateQueries({ queryKey: ["session-runs", sessionId] });
    },
  });

  const session = sessionQuery.data;
  const runs = runsQuery.data?.pages.flatMap((page) => page.items) ?? [];
  const activeRun = session?.executing_run;
  const canCancel = Boolean(activeRun && cancellableRunStates.has(activeRun.state));
  const retryableRun = latestRetryableRun(runs);
  const pendingApproval = approvalsQuery.data?.items.find(
    (approval) => approval.state === "pending" && approval.run_id === activeRun?.id,
  );
  const messageRows = useMemo(
    () => session?.messages.map((message) => ({ ...message, text: messageText(message.parts) })) ?? [],
    [session?.messages],
  );
  if (sessionQuery.isLoading) return <LoadingState label="Loading session" />;
  if (sessionQuery.isError || !session)
    return <ErrorState message="This session could not be loaded." onRetry={() => void sessionQuery.refetch()} />;

  return (
    <div className="page-wrap session-page">
      <Link to="/sessions" className="back-link">
        <ArrowLeft size={15} /> My sessions
      </Link>
      <div className="session-workbench">
        <section className="conversation-panel">
          <div className="conversation-heading">
            <div>
              <span className="eyebrow">Session detail</span>
              <h1>{session.title || session.agent.display_name}</h1>
              <p>Session {session.id}</p>
            </div>
            <StatusMark status={activeRun?.state ?? session.state} />
          </div>
          <div
            className="conversation-messages"
            aria-label="Session conversation"
          >
            {messageRows.map((message) => (
              <div
                className={`request-message ${message.author_kind === "agent" ? "agent-message" : ""}`}
                key={message.id}
              >
                <span className="message-label">
                  {messageLabel(message.author_kind)}
                </span>
                <p>{message.text}</p>
              </div>
            ))}
          </div>
          <div className="run-output" aria-live="polite">
            <div className="output-heading">
              <span>Live output</span>
              <span className={`stream-state ${stream.state}`}>
                {stream.state}
              </span>
            </div>
            {stream.notice ? (
              <p className="stream-notice" role="status">
                {stream.notice}
              </p>
            ) : null}
            <pre>{stream.output || "Waiting for runner output..."}</pre>
          </div>
          {activeRun?.state_reason ? (
            <div className="reason-box">
              <strong>Run note</strong>
              <p>{activeRun.state_reason.message}</p>
            </div>
          ) : null}
          {runs[0]?.outcome === "requester_input_required" ? (
            <form
              className="follow-up-composer"
              onSubmit={(event) => {
                event.preventDefault();
                if (followUp.trim()) continueSession.mutate();
              }}
            >
              <label htmlFor="session-follow-up">Continue this session</label>
              <textarea
                id="session-follow-up"
                value={followUp}
                onChange={(event) => setFollowUp(event.target.value)}
                disabled={continueSession.isPending}
              />
              <Button
                type="submit"
                disabled={!followUp.trim() || continueSession.isPending}
              >
                <Send size={16} />
                {continueSession.isPending ? "Starting..." : "Continue"}
              </Button>
            </form>
          ) : null}
          <div className="session-actions">
            <Button
              variant="danger"
              disabled={!canCancel || cancel.isPending}
              onClick={() => cancel.mutate()}
            >
              <Ban size={16} />
              {cancel.isPending ? "Canceling..." : "Cancel run"}
            </Button>
            {retryableRun ? (
              <>
                <select
                  aria-label="Retry version"
                  value={retryRevision}
                  onChange={(event) =>
                    setRetryRevision(
                      event.target.value as typeof retryRevision,
                    )
                  }
                  disabled={retry.isPending}
                >
                  <option value="original_revision">Original version</option>
                  <option value="current_production_revision">
                    Current production version
                  </option>
                </select>
                <Button
                  variant="secondary"
                  disabled={retry.isPending}
                  onClick={() => retry.mutate()}
                >
                  <RotateCcw size={16} />
                  {retry.isPending ? "Retrying..." : "Retry run"}
                </Button>
              </>
            ) : null}
            {pendingApproval ? (
              <Button
                variant="secondary"
                onClick={() => navigate(`/approvals/${pendingApproval.id}`)}
              >
                Open approval
              </Button>
            ) : null}
          </div>
          {cancel.isError || retry.isError || continueSession.isError ? (
            <p className="inline-error" role="alert">
              {commandError(
                cancel.error ?? retry.error ?? continueSession.error,
              )}
            </p>
          ) : null}
          <section className="run-attempts">
            <h2>Runs</h2>
            {runs.map((run) => (
              <div className="run-attempt-row" key={run.id}>
                <span>{run.id}</span>
                <StatusMark status={run.state} />
                <span>{run.state_reason?.message}</span>
              </div>
            ))}
            {runsQuery.hasNextPage ? (
              <Button
                variant="secondary"
                onClick={() => void runsQuery.fetchNextPage()}
                disabled={runsQuery.isFetchingNextPage}
              >
                {runsQuery.isFetchingNextPage
                  ? "Loading..."
                  : "Load more runs"}
              </Button>
            ) : null}
          </section>
        </section>
      </div>
    </div>
  );
}

function latestRetryableRun(runs: RunSummary[] | undefined) {
  return runs?.find(
    (run) => run.state === "failed" || run.state === "canceled",
  );
}

function commandError(error: unknown) {
  return error instanceof Error
    ? error.message
    : "The command could not be completed.";
}

function messageText(parts: Array<Record<string, unknown>>) {
  return parts
    .map((part) => {
      if (typeof part.text === "string") return part.text;
      if (typeof part.summary === "string") return part.summary;
      if (typeof part.message === "string") return part.message;
      if (typeof part.label === "string") return part.label;
      return "";
    })
    .filter(Boolean)
    .join("\n");
}

function messageLabel(author: string) {
  if (author === "agent") return "Agent";
  if (author === "system_summary") return "Activity";
  return "You";
}
