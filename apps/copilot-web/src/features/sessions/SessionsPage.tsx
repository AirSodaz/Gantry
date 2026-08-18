import { useMemo } from "react";
import { Link, useSearchParams } from "react-router-dom";
import {
  Archive,
  ArrowUpRight,
  CheckCircle2,
  Filter,
  ListTodo,
} from "lucide-react";
import {
  Button,
  Select,
  type SelectOption,
  StatusMark,
} from "@gantry/design-system";
import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { useCopilotApi } from "../../api/ApiProvider";
import {
  EmptyState,
  ErrorState,
  LoadingState,
} from "../../components/AsyncState";

const STATUS_OPTIONS: SelectOption[] = [
  { value: "", label: "All statuses", icon: <Filter size={13} /> },
  { value: "active", label: "Active", icon: <CheckCircle2 size={13} /> },
  { value: "archived", label: "Archived", icon: <Archive size={13} /> },
];

const ACTION_OPTIONS: SelectOption[] = [
  { value: "", label: "All requester actions" },
  { value: "approval", label: "Approval needed" },
];

const TIME_OPTIONS: SelectOption[] = [
  { value: "", label: "Any time" },
  { value: "7d", label: "Last 7 days" },
  { value: "30d", label: "Last 30 days" },
];

export function SessionsPage() {
  const api = useCopilotApi();
  const [params, setParams] = useSearchParams();
  const status = params.get("status") ?? "";
  const agentId = params.get("agent_id") ?? "";
  const requesterAction = params.get("requester_action") ?? "";
  const timeRange = params.get("time_range") ?? "";
  const createdAfter = useMemo(() => rangeStart(timeRange), [timeRange]);
  const agentsQuery = useQuery({
    queryKey: ["agents", "session-history"],
    queryFn: () => api.listAgents(),
  });
  const agentOptions = useMemo<SelectOption[]>(
    () => [
      { value: "", label: "All agents" },
      ...(agentsQuery.data?.items ?? []).map((agent) => ({
        value: agent.id,
        label: agent.display_name,
      })),
    ],
    [agentsQuery.data],
  );
  const query = useInfiniteQuery({
    queryKey: ["sessions", status, agentId, requesterAction, timeRange],
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) =>
      api.listSessions({
        state: status,
        agentId,
        myAction: requesterAction,
        updatedAfter: createdAfter,
        cursor: pageParam,
      }),
    getNextPageParam: (page) =>
      page.page_info?.has_more ? page.page_info.next_cursor : undefined,
  });
  const items = query.data?.pages.flatMap((page) => page.items) ?? [];
  const setFilter = (
    key: "status" | "agent_id" | "requester_action" | "time_range",
    value: string,
  ) => {
    const next = new URLSearchParams(params);
    if (value) next.set(key, value);
    else next.delete(key);
    setParams(next, { replace: true });
  };

  return (
    <div className="page-wrap narrow-page">
      <div className="page-heading page-heading-inline">
        <div>
          <span className="eyebrow">History</span>
          <h1>My sessions</h1>
          <p>Every conversation you started or joined.</p>
        </div>

        <div className="history-filter-wrap">
          <Select
            label="Status"
            options={STATUS_OPTIONS}
            value={status}
            onChange={(value) => setFilter("status", value)}
            placeholder="All statuses"
          />
          <Select
            label="Agent"
            options={agentOptions}
            value={agentId}
            onChange={(value) => setFilter("agent_id", value)}
            placeholder="All agents"
          />
          <Select
            label="Requester action"
            options={ACTION_OPTIONS}
            value={requesterAction}
            onChange={(value) => setFilter("requester_action", value)}
            placeholder="All requester actions"
          />
          <Select
            label="Time"
            options={TIME_OPTIONS}
            value={timeRange}
            onChange={(value) => setFilter("time_range", value)}
            placeholder="Any time"
          />
          {status || agentId || requesterAction || timeRange ? (
            <Button
              variant="quiet"
              size="sm"
              onClick={() => setParams({}, { replace: true })}
              className="history-filter-clear"
            >
              Clear filters
            </Button>
          ) : null}
        </div>
      </div>

      {query.isLoading ? <LoadingState label="Loading sessions" /> : null}
      {query.isError ? (
        <ErrorState
          message={
            query.error instanceof Error
              ? query.error.message
              : "Your sessions could not be loaded."
          }
          onRetry={() => void query.refetch()}
        />
      ) : null}
      {!query.isLoading && !query.isError && items.length === 0 ? (
        <EmptyState
          icon={<ListTodo size={22} />}
          title="No sessions yet"
          description={
            status || agentId || requesterAction || timeRange
              ? "No sessions match the selected filters."
              : "Start a new session when you are ready."
          }
          action={
            status || agentId || requesterAction || timeRange ? (
              <Button
                variant="secondary"
                size="sm"
                onClick={() => setParams({}, { replace: true })}
              >
                Reset filters
              </Button>
            ) : (
              <Link className="ds-button ds-button-primary ds-button-sm" to="/">
                Start a session
              </Link>
            )
          }
        />
      ) : null}
      <div className="session-list" aria-label="My sessions">
        {items.map((session) => (
          <Link
            to={`/sessions/${session.id}`}
            className="session-row"
            key={session.id}
          >
            <span className="session-row-icon" aria-hidden="true">
              <ListTodo size={17} />
            </span>
            <span className="session-row-copy">
              <strong>{session.agent.display_name}</strong>
              <span className="session-row-title">
                {session.title || "Untitled request"}
              </span>
              <span className="session-row-meta">
                {formatDate(session.updated_at ?? session.created_at)} ·{" "}
                {requesterActionLabel(session.my_action)} ·{" "}
                {artifactAvailability(session.artifacts)}
              </span>
            </span>
            <StatusMark
              status={session.executing_run?.state ?? session.state}
            />
            <ArrowUpRight
              size={16}
              className="session-row-arrow"
              aria-hidden="true"
            />
          </Link>
        ))}
      </div>
      {query.hasNextPage ? (
        <div className="list-more">
          <Button
            variant="secondary"
            onClick={() => void query.fetchNextPage()}
            disabled={query.isFetchingNextPage}
          >
            {query.isFetchingNextPage ? "Loading..." : "Load more"}
          </Button>
        </div>
      ) : null}
    </div>
  );
}

function rangeStart(range: string) {
  if (range !== "7d" && range !== "30d") return undefined;
  const date = new Date();
  date.setDate(date.getDate() - (range === "7d" ? 7 : 30));
  return date.toISOString();
}

function formatDate(value?: string) {
  if (!value) return "Recently submitted";
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

function requesterActionLabel(action?: string) {
  if (action === "approval") return "Approval needed";
  return "No action needed";
}

function artifactAvailability(
  artifacts?: Array<{ state?: string; scan_state?: string }>,
) {
  if (!artifacts?.length) return "No artifacts";
  return artifacts.every(
    (artifact) =>
      artifact.state === "available" && artifact.scan_state === "passed",
  )
    ? "Artifacts ready"
    : "Artifacts processing";
}
