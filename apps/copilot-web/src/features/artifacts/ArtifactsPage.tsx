import { useSearchParams, Link } from "react-router-dom";
import { FileArchive, Filter, ArrowUpRight } from "lucide-react";
import {
  Button,
  formatBytes,
  Select,
  StatusMark,
  type SelectOption,
} from "@gantry/design-system";
import { useInfiniteQuery } from "@tanstack/react-query";
import { useCopilotApi } from "../../api/ApiProvider";
import {
  EmptyState,
  ErrorState,
  LoadingState,
} from "../../components/AsyncState";
const CLASSIFICATION_OPTIONS: SelectOption[] = [
  { value: "", label: "All classifications", icon: <Filter size={13} /> },
  { value: "public", label: "Public" },
  { value: "internal", label: "Internal" },
  { value: "confidential", label: "Confidential" },
];

export function ArtifactsPage() {
  const api = useCopilotApi();
  const [params, setParams] = useSearchParams();
  const taskId = params.get("task_id") ?? "";
  const classification = params.get("classification") ?? "";
  const query = useInfiniteQuery({
    queryKey: ["artifacts", taskId, classification],
    initialPageParam: "",
    queryFn: ({ pageParam }) =>
      api.listArtifacts(taskId, classification, pageParam),
    getNextPageParam: (page) =>
      page.page_info?.has_more ? page.page_info.next_cursor : undefined,
  });
  const items = query.data?.pages.flatMap((page) => page.items) ?? [];
  const setFilter = (name: "task_id" | "classification", value: string) => {
    const next = new URLSearchParams(params);
    if (value) next.set(name, value);
    else next.delete(name);
    setParams(next, { replace: true });
  };

  return (
    <div className="page-wrap artifacts-page">
      <div className="page-heading page-heading-inline">
        <div>
          <span className="eyebrow">Files</span>
          <h1>Artifacts</h1>
          <p>Files produced by tasks you started.</p>
        </div>
      </div>
      <div className="artifact-filters">
        <label className="admin-field">
          <span className="admin-field-label">Task</span>
          <input
            className="ds-input"
            value={taskId}
            onChange={(event) =>
              setFilter("task_id", event.target.value.trim())
            }
            placeholder="Filter by task ID"
          />
        </label>
        <Select
          label="Classification"
          options={CLASSIFICATION_OPTIONS}
          value={classification}
          onChange={(value) => setFilter("classification", value)}
          placeholder="All classifications"
        />
        {taskId || classification ? (
          <Button
            variant="quiet"
            size="sm"
            onClick={() => setParams({}, { replace: true })}
          >
            Clear filters
          </Button>
        ) : null}
      </div>
      {query.isLoading ? <LoadingState label="Loading artifacts" /> : null}
      {query.isError ? (
        <ErrorState
          message={
            query.error instanceof Error
              ? query.error.message
              : "Artifacts could not be loaded."
          }
          onRetry={() => void query.refetch()}
        />
      ) : null}
      {!query.isLoading && !query.isError && items.length === 0 ? (
        <EmptyState
          title="No artifacts found"
          detail={
            taskId || classification
              ? "No artifacts match the current filter criteria."
              : "Files produced by your tasks will appear here."
          }
          action={
            taskId || classification ? (
              <Button
                variant="secondary"
                size="sm"
                onClick={() => setParams({}, { replace: true })}
              >
                Reset filters
              </Button>
            ) : undefined
          }
        />
      ) : null}
      <div className="artifact-browser-list" aria-label="Artifacts">
        {items.map((artifact) => (
          <Link
            className="artifact-browser-row"
            key={artifact.id}
            to={`/artifacts/${encodeURIComponent(artifact.id)}`}
          >
            <span className="artifact-browser-icon" aria-hidden="true">
              <FileArchive size={18} />
            </span>
            <span className="artifact-browser-copy">
              <strong>{artifact.filename}</strong>
              <span>
                {artifact.media_type} · {formatBytes(artifact.size_bytes)}
              </span>
              <small>{artifact.task_id}</small>
            </span>
            <span className="artifact-browser-states">
              <StatusMark status={artifact.classification ?? "internal"} />
              <StatusMark status={artifact.state} />
            </span>
            <ArrowUpRight size={17} aria-hidden="true" />
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
