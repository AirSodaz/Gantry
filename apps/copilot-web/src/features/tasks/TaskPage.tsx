import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  ArrowLeft,
  Ban,
  CheckCircle2,
  RotateCcw,
  Send,
  Timer,
  XCircle,
} from "lucide-react";
import {
  Button,
  formatBytes,
  formatDate,
  StatusMark,
} from "@gantry/design-system";
import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { useCopilotApi } from "../../api/ApiProvider";
import type {
  Approval,
  Artifact,
  TaskEventFrame as ApiTaskEventFrame,
  TaskEventSnapshot,
} from "../../api/types";
import { ErrorState, LoadingState } from "../../components/AsyncState";
import {
  AttachmentUploadControl,
  type AttachmentUploadState,
} from "./AttachmentUploadControl";

const activeStatuses = new Set([
  "queued",
  "running",
  "canceling",
  "provisioning",
  "awaiting_approval",
  "suspended",
]);

export function TaskPage() {
  const { taskId = "" } = useParams();
  const navigate = useNavigate();
  const api = useCopilotApi();
  const queryClient = useQueryClient();
  const [output, setOutput] = useState("");
  const [followUp, setFollowUp] = useState("");
  const [followUpAttachments, setFollowUpAttachments] =
    useState<AttachmentUploadState>({ attachmentIDs: [], hasPending: false });
  const [followUpComposerVersion, setFollowUpComposerVersion] = useState(0);
  const [retryRevisionSelection, setRetryRevisionSelection] = useState<
    "original_revision" | "current_production_revision"
  >("original_revision");
  const [streamState, setStreamState] = useState<
    "connecting" | "connected" | "reconnecting" | "closed"
  >("connecting");
  const [streamNotice, setStreamNotice] = useState("");
  const cursorRef = useRef("");
  const seenSequences = useRef(new Set<number>());
  const outputMessageIDRef = useRef<string | null>(null);
  const followUpKeyRef = useRef<string | null>(null);
  const cancelKeyRef = useRef<string | null>(null);
  const retryKeyRef = useRef<string | null>(null);
  const onFollowUpAttachmentsChange = useCallback(
    (state: AttachmentUploadState) => setFollowUpAttachments(state),
    [],
  );
  const taskQuery = useQuery({
    queryKey: ["task", taskId],
    queryFn: () => api.getTask(taskId),
    enabled: Boolean(taskId),
    refetchInterval: (query) =>
      activeStatuses.has(query.state.data?.status ?? "") ? 1000 : false,
  });
  const cancelMutation = useMutation({
    mutationFn: () => {
      const runId = taskQuery.data?.current_run?.id;
      if (!runId) throw new Error("This task has no active run.");
      if (!cancelKeyRef.current) cancelKeyRef.current = crypto.randomUUID();
      return api.cancelRun(taskId, runId, cancelKeyRef.current);
    },
    onSuccess: () => {
      cancelKeyRef.current = null;
      void queryClient.invalidateQueries({ queryKey: ["task", taskId] });
    },
  });
  const retryMutation = useMutation({
    mutationFn: () => {
      const conversationETag = taskQuery.data?.conversation_etag;
      if (!conversationETag)
        throw new Error("This task needs to be refreshed before retrying.");
      if (!retryKeyRef.current) retryKeyRef.current = crypto.randomUUID();
      return api.retryTask(
        taskId,
        retryKeyRef.current,
        conversationETag,
        retryRevisionSelection,
      );
    },
    onSuccess: () => {
      retryKeyRef.current = null;
      void queryClient.invalidateQueries({ queryKey: ["task", taskId] });
    },
  });
  const followUpMutation = useMutation({
    mutationFn: () => {
      const conversationETag = taskQuery.data?.conversation_etag;
      if (!conversationETag)
        throw new Error("This task needs to be refreshed before continuing.");
      if (!followUpKeyRef.current) followUpKeyRef.current = crypto.randomUUID();
      return api.appendTaskMessage(
        taskId,
        {
          message: followUp,
          ...(followUpAttachments.attachmentIDs.length
            ? { attachment_ids: followUpAttachments.attachmentIDs }
            : {}),
        },
        followUpKeyRef.current,
        conversationETag,
      );
    },
    onSuccess: (nextTask) => {
      setFollowUp("");
      setFollowUpAttachments({ attachmentIDs: [], hasPending: false });
      setFollowUpComposerVersion((current) => current + 1);
      followUpKeyRef.current = null;
      void queryClient.setQueryData(["task", taskId], nextTask);
      void queryClient.invalidateQueries({ queryKey: ["task-runs", taskId] });
    },
  });
  const task = taskQuery.data;
  const runsQuery = useInfiniteQuery({
    queryKey: ["task-runs", taskId],
    initialPageParam: "",
    queryFn: ({ pageParam }) => api.listTaskRuns(taskId, pageParam),
    getNextPageParam: (page) =>
      page?.page_info?.has_more
        ? (page.page_info.next_cursor ?? undefined)
        : undefined,
    enabled: Boolean(taskId),
  });
  const approvalsQuery = useQuery({
    queryKey: ["task-approvals", taskId],
    queryFn: () => api.listApprovals(),
    enabled: task?.status === "awaiting_approval",
    refetchInterval: task?.status === "awaiting_approval" ? 5000 : false,
  });
  const canCancel = Boolean(
    task &&
    activeStatuses.has(task.status) &&
    task.current_run?.id &&
    task.status !== "canceling",
  );
  const canRetry = Boolean(
    task && (task.status === "failed" || task.status === "canceled"),
  );
  const canContinue = task?.status === "awaiting_requester_input";
  const pendingApproval =
    task?.status === "awaiting_approval"
      ? approvalsQuery.data?.items.find(
          (approval) =>
            approval.status === "pending" &&
            approval.run_id === task.current_run?.id,
        )
      : undefined;
  const timeline = useMemo(() => buildTimeline(task?.status), [task?.status]);

  useEffect(() => {
    if (!taskId) return;
    let cancelled = false;
    let socket: WebSocket | null = null;
    let retryTimer: number | undefined;
    let attempt = 0;
    let terminal = false;

    const connect = async () => {
      if (cancelled) return;
      setStreamState(attempt === 0 ? "connecting" : "reconnecting");
      try {
        const ticket = await api.createEventsTicket(
          taskId,
          cursorRef.current || undefined,
        );
        if (cancelled) return;
        if (typeof WebSocket === "undefined") {
          setStreamState("closed");
          return;
        }
        const base = import.meta.env.VITE_COPILOT_API_BASE ?? "/api/copilot/v1";
        const wsOrigin = `${window.location.protocol === "https:" ? "wss:" : "ws:"}//${window.location.host}`;
        const fallbackURL = `${wsOrigin}${base}/tasks/${encodeURIComponent(taskId)}/events`;
        const streamURL = new URL(
          ticket.websocket_url ?? fallbackURL,
          window.location.origin,
        );
        streamURL.searchParams.set("ticket", ticket.ticket);
        if (cursorRef.current)
          streamURL.searchParams.set("after", cursorRef.current);
        socket = new WebSocket(streamURL.toString());
        socket.onopen = () => {
          attempt = 0;
          setStreamState("connected");
        };
        socket.onmessage = (message) => {
          try {
            const frame = JSON.parse(message.data as string) as EventFrame;
            if (frame.type === "cursor_expired") {
              setStreamNotice(
                "Earlier live history expired, so the current task state was refreshed.",
              );
              if (isSnapshotFrame(frame.snapshot)) {
                applySnapshot(
                  frame.snapshot,
                  taskId,
                  queryClient,
                  setOutput,
                  cursorRef,
                  seenSequences,
                  outputMessageIDRef,
                );
              } else {
                cursorRef.current = "";
                seenSequences.current.clear();
                setOutput("");
              }
              void queryClient.invalidateQueries({
                queryKey: ["task", taskId],
              });
              socket?.close();
              return;
            }
            if (frame.type === "error" && frame.code === "cursor_invalid") {
              setStreamNotice(
                "The saved live position was no longer valid. Refreshing the current task state.",
              );
              cursorRef.current = "";
              seenSequences.current.clear();
              socket?.close();
              return;
            }
            if (
              frame.type === "error" &&
              frame.code === "event_ticket_expired"
            ) {
              setStreamNotice(
                "The live event ticket expired. Reconnecting to the task.",
              );
              socket?.close();
              return;
            }
            if (isSnapshotFrame(frame)) {
              applySnapshot(
                frame,
                taskId,
                queryClient,
                setOutput,
                cursorRef,
                seenSequences,
                outputMessageIDRef,
              );
              return;
            }
            if (isTaskEventFrame(frame)) {
              cursorRef.current = frame.cursor ?? cursorRef.current;
              if (seenSequences.current.has(frame.task_sequence)) return;
              seenSequences.current.add(frame.task_sequence);
              const event = frame.event;
              if (event.type === "content_segment") {
                if (outputMessageIDRef.current !== event.message_id) {
                  outputMessageIDRef.current = event.message_id;
                  setOutput("");
                }
                setOutput((current) => current + event.text);
              } else {
                if (
                  event.type === "message_committed" &&
                  outputMessageIDRef.current === event.message.id
                ) {
                  outputMessageIDRef.current = null;
                  setOutput("");
                }
                if (event.type === "approval_changed") {
                  applyApprovalChange(queryClient, taskId, event.approval);
                }
                void queryClient.invalidateQueries({
                  queryKey: ["task", taskId],
                });
                if (event.type === "run_state_changed") {
                  void queryClient.invalidateQueries({
                    queryKey: ["task-runs", taskId],
                  });
                }
              }
              if (isTerminalEvent(event)) {
                terminal = true;
                socket?.close();
              }
            }
          } catch {
            // Ignore malformed frames; the next state poll remains authoritative.
          }
        };
        socket.onclose = () => {
          if (cancelled || terminal) {
            setStreamState("closed");
            return;
          }
          attempt += 1;
          retryTimer = window.setTimeout(
            connect,
            Math.min(10_000, 500 * 2 ** Math.min(attempt, 5)),
          );
        };
        socket.onerror = () => socket?.close();
      } catch {
        attempt += 1;
        retryTimer = window.setTimeout(
          connect,
          Math.min(10_000, 500 * 2 ** Math.min(attempt, 5)),
        );
      }
    };
    void connect();
    return () => {
      cancelled = true;
      if (retryTimer !== undefined) window.clearTimeout(retryTimer);
      socket?.close();
      setStreamState("closed");
    };
  }, [api, queryClient, taskId]);

  if (taskQuery.isLoading)
    return (
      <div className="page-wrap">
        <LoadingState label="Loading task" />
      </div>
    );
  if (taskQuery.isError || !task)
    return (
      <div className="page-wrap">
        <ErrorState
          message={
            taskQuery.error instanceof Error
              ? taskQuery.error.message
              : "This task could not be loaded."
          }
          onRetry={() => void taskQuery.refetch()}
        />
      </div>
    );

  return (
    <div className="page-wrap task-page">
      <Link to="/tasks" className="back-link">
        <ArrowLeft size={15} /> My tasks
      </Link>
      <div className="task-workbench">
        <section className="conversation-panel">
          <div className="conversation-heading">
            <div>
              <span className="eyebrow">Task detail</span>
              <h1>{task.agent_display_name ?? task.agent_id}</h1>
              <p>Task {task.id}</p>
            </div>
            <StatusMark status={task.status} />
          </div>
          <div className="conversation-messages" aria-label="Task conversation">
            {task.messages?.map((message) => (
              <div
                className={`request-message ${message.role === "agent" ? "agent-message" : ""}`}
                key={message.id}
              >
                <span className="message-label">
                  {messageLabel(message.role)}
                </span>
                <p>{messageText(message.parts)}</p>
              </div>
            ))}
            {!task.messages?.length ? (
              <div className="request-message">
                <span className="message-label">Your request</span>
                <p>
                  This task was submitted to{" "}
                  {task.agent_display_name ?? task.agent_id}.
                </p>
              </div>
            ) : null}
          </div>
          <div className="run-output" aria-live="polite">
            <div className="output-heading">
              <span>Live output</span>
              <span className={`stream-state ${streamState}`}>
                {streamState}
              </span>
            </div>
            {streamNotice ? (
              <p className="stream-notice" role="status">
                {streamNotice}
              </p>
            ) : null}
            {streamState === "reconnecting" ? (
              <p className="stream-notice" role="status">
                Live updates are reconnecting. The task continues on the server.
              </p>
            ) : null}
            <pre>{output || "Waiting for runner output…"}</pre>
          </div>
          <div className="timeline" aria-label="Task timeline">
            {timeline.map(({ label, status, Icon }) => (
              <div className={`timeline-item ${status}`} key={label}>
                <span className="timeline-icon">
                  <Icon size={16} />
                </span>
                <span>{label}</span>
              </div>
            ))}
          </div>
          {task.current_run?.status_reason ? (
            <div className="reason-box">
              <strong>Run note</strong>
              <p>{task.current_run.status_reason}</p>
            </div>
          ) : null}
          {canContinue ? (
            <form
              className="follow-up-composer"
              onSubmit={(event) => {
                event.preventDefault();
                if (followUp.trim() && !followUpAttachments.hasPending)
                  followUpMutation.mutate();
              }}
            >
              <label htmlFor="task-follow-up">Continue this task</label>
              <textarea
                id="task-follow-up"
                value={followUp}
                onChange={(event) => setFollowUp(event.target.value)}
                placeholder="Describe what should change before the next attempt"
                maxLength={8000}
                disabled={followUpMutation.isPending}
              />
              <AttachmentUploadControl
                key={followUpComposerVersion}
                disabled={followUpMutation.isPending}
                onChange={onFollowUpAttachmentsChange}
              />
              <Button
                type="submit"
                disabled={
                  !followUp.trim() ||
                  followUpAttachments.hasPending ||
                  followUpMutation.isPending
                }
              >
                <Send size={16} />{" "}
                {followUpMutation.isPending ? "Starting…" : "Continue"}
              </Button>
            </form>
          ) : null}
          {cancelMutation.isError ||
          retryMutation.isError ||
          followUpMutation.isError ? (
            <p className="inline-error" role="alert">
              {(cancelMutation.error ??
                retryMutation.error ??
                followUpMutation.error) instanceof Error
                ? (
                    cancelMutation.error ??
                    retryMutation.error ??
                    followUpMutation.error
                  )?.message
                : "The command could not be completed."}
            </p>
          ) : null}
          <div className="task-actions">
            <Button
              variant="danger"
              disabled={!canCancel || cancelMutation.isPending}
              onClick={() => cancelMutation.mutate()}
            >
              <Ban size={16} />{" "}
              {cancelMutation.isPending ? "Canceling…" : "Cancel run"}
            </Button>
            {canRetry ? (
              <>
                <label className="sr-only" htmlFor="retry-revision">
                  Retry version
                </label>
                <select
                  id="retry-revision"
                  value={retryRevisionSelection}
                  onChange={(event) =>
                    setRetryRevisionSelection(
                      event.target.value as typeof retryRevisionSelection,
                    )
                  }
                  disabled={retryMutation.isPending}
                >
                  <option value="original_revision">Original version</option>
                  <option value="current_production_revision">
                    Current production version
                  </option>
                </select>
                <Button
                  variant="secondary"
                  disabled={retryMutation.isPending}
                  onClick={() => retryMutation.mutate()}
                >
                  <RotateCcw size={16} />{" "}
                  {retryMutation.isPending ? "Retrying…" : "Retry task"}
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
            <Button variant="quiet" onClick={() => navigate("/tasks")}>
              Back to history
            </Button>
          </div>
          {task.artifacts?.length ? (
            <div className="artifact-list">
              <h2>Artifacts</h2>
              {task.artifacts.map((artifact) => (
                <ArtifactRow
                  key={artifact.id}
                  artifact={artifact}
                  onDownload={async () => {
                    const result = await api.requestArtifactDownload(
                      artifact.id,
                    );
                    if (result.download_url)
                      window.open(
                        result.download_url,
                        "_blank",
                        "noopener,noreferrer",
                      );
                  }}
                />
              ))}
            </div>
          ) : null}
          {runsQuery.data?.pages.some((page) => page?.items?.length) ? (
            <section className="run-attempts">
              <h2>Run attempts</h2>
              {runsQuery.data.pages
                .flatMap((page) => page?.items ?? [])
                .map((run) => (
                  <div className="run-attempt-row" key={run.id}>
                    <span>Attempt {run.attempt_number}</span>
                    <StatusMark status={run.status} />
                    <span>
                      {run.status_reason ||
                        formatDate(
                          run.completed_at ?? run.started_at ?? run.created_at,
                        )}
                    </span>
                  </div>
                ))}
              {runsQuery.hasNextPage ? (
                <Button
                  variant="quiet"
                  disabled={runsQuery.isFetchingNextPage}
                  onClick={() => void runsQuery.fetchNextPage()}
                >
                  {runsQuery.isFetchingNextPage
                    ? "Loading attempts..."
                    : "Load more run attempts"}
                </Button>
              ) : null}
            </section>
          ) : null}
        </section>
        <aside className="run-inspector">
          <span className="context-kicker">Run inspector</span>
          <div className="inspector-row">
            <span>Run</span>
            <strong>{task.current_run?.id ?? "Not assigned"}</strong>
          </div>
          <div className="inspector-row">
            <span>State</span>
            <StatusMark status={task.current_run?.status ?? task.status} />
          </div>
          <div className="inspector-row">
            <span>Created</span>
            <strong>{formatDate(task.created_at)}</strong>
          </div>
          <div className="inspector-note">
            <Timer size={16} />
            <span>State refreshes automatically while the run is active.</span>
          </div>
        </aside>
      </div>
    </div>
  );
}

type EventFrame = {
  type?: string;
  code?: string;
  schema_version?: string;
  cursor?: string;
  task?: unknown;
  runs?: unknown[];
  approvals?: unknown[];
  snapshot?: unknown;
  task_sequence?: number;
  event?: unknown;
};

type TaskEvent = ApiTaskEventFrame["event"];

type SnapshotFrame = TaskEventSnapshot & {
  type: "snapshot";
};

type TaskEventFrame = ApiTaskEventFrame;

function isSnapshotFrame(value: unknown): value is SnapshotFrame {
  const frame = value as Partial<SnapshotFrame> | null;
  return (
    frame?.type === "snapshot" &&
    frame.schema_version === "gantry.copilot.snapshot/v1" &&
    typeof frame.cursor === "string" &&
    Array.isArray(frame.runs) &&
    Array.isArray(frame.approvals) &&
    frame.task != null
  );
}

function isTaskEventFrame(value: EventFrame): value is TaskEventFrame {
  const frame = value as Partial<TaskEventFrame>;
  return (
    frame.schema_version === "gantry.copilot.event/v1" &&
    typeof frame.cursor === "string" &&
    typeof frame.task_sequence === "number" &&
    isTaskEvent(frame.event)
  );
}

function isTaskEvent(value: unknown): value is TaskEvent {
  if (
    !value ||
    typeof value !== "object" ||
    typeof (value as { type?: unknown }).type !== "string"
  )
    return false;
  const event = value as {
    type: string;
    text?: unknown;
    message_id?: unknown;
    segment_index?: unknown;
  };
  if (event.type === "content_segment")
    return (
      typeof event.text === "string" &&
      typeof event.message_id === "string" &&
      typeof event.segment_index === "number"
    );
  return (
    event.type === "message_committed" ||
    event.type === "run_state_changed" ||
    event.type === "approval_changed" ||
    event.type === "artifact_changed"
  );
}

function applySnapshot(
  snapshot: SnapshotFrame,
  taskId: string,
  queryClient: ReturnType<typeof useQueryClient>,
  setOutput: (value: string) => void,
  cursorRef: { current: string },
  seenSequences: { current: Set<number> },
  outputMessageIDRef: { current: string | null },
) {
  cursorRef.current = snapshot.cursor;
  seenSequences.current.clear();
  outputMessageIDRef.current = null;
  setOutput("");
  queryClient.setQueryData(["task", taskId], snapshot.task);
  queryClient.setQueryData(["task-runs", taskId], {
    pages: [{ items: snapshot.runs, page_info: { has_more: false } }],
    pageParams: [""],
  });
  queryClient.setQueryData(["task-approvals", taskId], {
    items: snapshot.approvals,
  });
}

function applyApprovalChange(
  queryClient: ReturnType<typeof useQueryClient>,
  taskId: string,
  approval: Approval,
) {
  queryClient.setQueryData<{ items: Approval[] }>(
    ["task-approvals", taskId],
    (current) => ({
      items: [
        ...(current?.items ?? []).filter((item) => item.id !== approval.id),
        approval,
      ],
    }),
  );
}

function ArtifactRow({
  artifact,
  onDownload,
}: {
  artifact: Artifact;
  onDownload: () => Promise<void>;
}) {
  const [downloadError, setDownloadError] = useState(false);
  const available =
    artifact.state === "available" && artifact.scan_status === "passed";
  const download = async () => {
    try {
      setDownloadError(false);
      await onDownload();
    } catch {
      setDownloadError(true);
    }
  };
  return (
    <div className="artifact-row">
      <div>
        <strong>{artifact.filename ?? artifact.id}</strong>
        <span>
          {artifact.media_type ?? "Artifact"} ·{" "}
          {formatBytes(artifact.size_bytes ?? 0)} ·{" "}
          {available ? "Ready" : "Processing"}
        </span>
        {downloadError ? (
          <span className="artifact-error" role="alert">
            Download failed. Try again.
          </span>
        ) : null}
      </div>
      <Button
        variant="secondary"
        disabled={!available}
        onClick={() => void download()}
      >
        {downloadError ? "Retry download" : "Download"}
      </Button>
    </div>
  );
}

function isTerminalEvent(event: TaskEvent) {
  return (
    event.type === "run_state_changed" &&
    (event.run.status === "completed" ||
      event.run.status === "failed" ||
      event.run.status === "canceled")
  );
}
function buildTimeline(status?: string) {
  const completed = status === "completed";
  const failed = status === "failed";
  const canceled = status === "canceled";
  return [
    { label: "Task accepted", status: "done", Icon: CheckCircle2 },
    {
      label: "Run assigned",
      status:
        activeStatuses.has(status ?? "") || completed || failed || canceled
          ? "done"
          : "pending",
      Icon: CheckCircle2,
    },
    {
      label: completed
        ? "Run completed"
        : failed
          ? "Run failed"
          : canceled
            ? "Run canceled"
            : "Run in progress",
      status: completed || failed || canceled ? "done" : "current",
      Icon: completed ? CheckCircle2 : failed ? XCircle : Timer,
    },
  ];
}
function messageText(parts: Array<Record<string, unknown>> | undefined) {
  return (parts ?? [])
    .map((part) => {
      if (part.type === "text" && typeof part.text === "string")
        return part.text;
      if (part.type === "status" && typeof part.message === "string")
        return part.message;
      if (part.type === "action_summary" && typeof part.summary === "string")
        return part.summary;
      if (part.type === "artifact" && typeof part.label === "string")
        return part.label;
      return "";
    })
    .filter(Boolean)
    .join("\n");
}

function messageLabel(role: string) {
  if (role === "agent") return "Agent";
  if (role === "system_summary") return "Activity";
  return "Your request";
}
