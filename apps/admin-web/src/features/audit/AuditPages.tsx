import { useMemo, useState, type ReactNode } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import {
  Activity,
  ArrowDownToLine,
  ArrowLeft,
  ArrowRight,
  FileSearch,
  ShieldCheck,
} from "lucide-react";
import { Link, useParams, useSearchParams } from "react-router-dom";
import {
  Button,
  CodeBlock,
  EmptyState,
  formatTime,
  Select,
  type SelectOption,
  shortHash,
} from "@gantry/design-system";
import { useAdminApi } from "../../api/ApiProvider";
import type { AdminAuditEvent } from "../../api/types";
import { ErrorState, LoadingState } from "../../components/AsyncState";

const resourceTypes: SelectOption[] = [
  { value: "", label: "All resources" },
  ...[
    "agent",
    "agent_revision",
    "agent_revision_review",
    "skill",
    "plugin",
    "tool",
    "run",
    "policy",
  ].map((value) => ({
    value,
    label: value.replace(/_/g, " "),
  })),
];

export function AuditPage() {
  const api = useAdminApi();
  const [searchParams, setSearchParams] = useSearchParams();
  const workspace = searchParams.get("workspace") ?? "";
  const resourceType = searchParams.get("resource_type") ?? "";
  const resourceId = searchParams.get("resource_id") ?? "";
  const eventType = searchParams.get("event_type") ?? "";
  const actorId = searchParams.get("actor_id") ?? "";
  const outcome = searchParams.get("outcome") ?? "";
  const risk = searchParams.get("risk") ?? "";
  const correlationId = searchParams.get("correlation_id") ?? "";
  const runId = searchParams.get("run_id") ?? "";
  const revisionHash = searchParams.get("revision_hash") ?? "";

  const workspaces = useQuery({
    queryKey: ["admin-workspaces"],
    queryFn: () => api.listWorkspaces(),
  });
  const events = useQuery({
    queryKey: [
      "admin-audit",
      workspace,
      resourceType,
      resourceId,
      eventType,
      actorId,
      outcome,
      risk,
      correlationId,
      runId,
      revisionHash,
    ],
    queryFn: () =>
      api.listAuditEvents({
        workspaceId: workspace,
        resourceType,
        resourceId,
        eventType,
        actorId,
        outcome,
        risk,
        correlationId,
        runId,
        revisionHash,
      }),
  });

  const workspaceOptions = useMemo<SelectOption[]>(
    () => [
      { value: "", label: "All manageable workspaces" },
      ...(workspaces.data?.items ?? []).map((item) => ({
        value: item.id,
        label: item.display_name,
      })),
    ],
    [workspaces.data?.items],
  );

  const update = (key: string, value: string) => {
    const next = new URLSearchParams(searchParams);
    if (value) next.set(key, value);
    else next.delete(key);
    next.delete("cursor");
    setSearchParams(next);
  };

  if (workspaces.isLoading || events.isLoading)
    return <LoadingState label="Loading audit evidence" />;
  if (workspaces.error || events.error) {
    return (
      <div className="admin-page">
        <ErrorState message="The authorized audit explorer could not be loaded." />
      </div>
    );
  }

  const items = events.data?.items ?? [];
  const exportOptions = {
    workspaceId: workspace,
    resourceType,
    resourceId,
    eventType,
    actorId,
    outcome,
    risk,
    correlationId,
    runId,
    revisionHash,
  };

  return (
    <section className="admin-page">
      <header className="admin-page-heading">
        <div>
          <h1>Audit</h1>
          <p>Immutable, attributable configuration and runtime evidence.</p>
        </div>
        <AuditExportPanel options={exportOptions} />
      </header>

      <div className="admin-filter-bar admin-audit-filter-bar">
        <Select
          label="Workspace"
          options={workspaceOptions}
          value={workspace}
          onChange={(value) => update("workspace", value)}
          placeholder="All manageable workspaces"
        />
        <Select
          label="Resource"
          options={resourceTypes}
          value={resourceType}
          onChange={(value) => update("resource_type", value)}
          placeholder="All resources"
        />
        <FilterInput
          label="Resource ID"
          value={resourceId}
          onChange={(value) => update("resource_id", value)}
          placeholder="agt_..."
        />
        <FilterInput
          label="Event type"
          value={eventType}
          onChange={(value) => update("event_type", value)}
          placeholder="configuration_asset.created"
        />
        <FilterInput
          label="Actor ID"
          value={actorId}
          onChange={(value) => update("actor_id", value)}
          placeholder="prn_..."
        />
        <FilterInput
          label="Outcome"
          value={outcome}
          onChange={(value) => update("outcome", value)}
          placeholder="success"
        />
        <FilterInput
          label="Risk"
          value={risk}
          onChange={(value) => update("risk", value)}
          placeholder="high"
        />
        <FilterInput
          label="Correlation ID"
          value={correlationId}
          onChange={(value) => update("correlation_id", value)}
          placeholder="corr_..."
        />
      </div>

      {items.length === 0 ? (
        <EmptyState
          icon={<FileSearch size={22} />}
          title="No audit events match this scope"
          description="Immutable activity appears here when authorized resources change."
        />
      ) : (
        <>
          <div className="admin-table-wrap">
            <table className="admin-table admin-audit-table">
              <thead>
                <tr>
                  <th>Time</th>
                  <th>Actor</th>
                  <th>Action</th>
                  <th>Resource</th>
                  <th>Scope</th>
                  <th>Outcome</th>
                  <th>Risk</th>
                  <th>Evidence</th>
                </tr>
              </thead>
              <tbody>
                {items.map((event) => (
                  <AuditRow key={event.id} event={event} />
                ))}
              </tbody>
            </table>
          </div>
          {events.data?.page_info.has_more ? (
            <div className="admin-pagination">
              <Button
                size="sm"
                variant="secondary"
                onClick={() =>
                  update("cursor", events.data?.page_info.next_cursor ?? "")
                }
              >
                Next page <ArrowRight size={15} />
              </Button>
            </div>
          ) : null}
        </>
      )}
    </section>
  );
}

function AuditRow({ event }: { event: AdminAuditEvent }) {
  return (
    <tr>
      <td>
        <Link className="admin-inline-link" to={`/audit/events/${event.id}`}>
          {formatTime(event.created_at)}
        </Link>
      </td>
      <td>
        <strong>{event.actor_name}</strong>
        <br />
        <span className="admin-muted">{event.actor_id}</span>
      </td>
      <td>{event.event_type}</td>
      <td>
        <strong>{event.resource_type}</strong>
        <br />
        <span className="admin-muted">{event.resource_id}</span>
      </td>
      <td>{event.scope || "Organization"}</td>
      <td>{event.outcome || "Unrecorded"}</td>
      <td>{event.risk || "Unrecorded"}</td>
      <td>
        {event.run_id ? (
          <Link className="admin-inline-link" to={`/runs/${event.run_id}`}>
            Run {event.run_id}
          </Link>
        ) : event.revision_hash ? (
          <span className="admin-muted">
            Revision {shortHash(event.revision_hash)}
          </span>
        ) : (
          <span className="admin-muted">Event detail</span>
        )}
      </td>
    </tr>
  );
}

export function AuditEventDetailPage() {
  const { eventId = "" } = useParams();
  const api = useAdminApi();
  const detail = useQuery({
    queryKey: ["admin-audit-event", eventId],
    queryFn: () => api.getAuditEvent(eventId),
    enabled: eventId !== "",
  });

  if (detail.isLoading) return <LoadingState label="Loading audit event" />;
  if (detail.error || !detail.data) {
    return (
      <div className="admin-page">
        <ErrorState message="This audit event is unavailable in your administrative scope." />
      </div>
    );
  }

  const event = detail.data;

  return (
    <section className="admin-page">
      <Link className="admin-back-link" to="/audit">
        <ArrowLeft size={15} /> Audit
      </Link>
      <header className="admin-detail-heading">
        <div>
          <div className="admin-detail-title">
            <ShieldCheck size={22} />
            <h1>Audit event {event.id}</h1>
          </div>
          <p>
            {event.event_type} · {formatTime(event.created_at)}
          </p>
        </div>
        <AuditExportPanel
          options={{
            resourceType: event.resource_type,
            resourceId: event.resource_id,
          }}
        />
      </header>

      <div className="admin-detail-grid admin-audit-detail-grid">
        <EvidenceBlock title="Immutable envelope" icon={<Activity size={18} />}>
          <Detail
            label="Actor"
            value={`${event.actor_name} (${event.actor_id})`}
          />
          <Detail
            label="Resource"
            value={
              resourceHref(event) ? (
                <Link
                  className="admin-inline-link"
                  to={resourceHref(event) ?? "#"}
                >
                  {`${event.resource_type} / ${event.resource_id}`}
                </Link>
              ) : (
                `${event.resource_type} / ${event.resource_id}`
              )
            }
          />
          <Detail label="Scope" value={event.scope || "Organization"} />
          <Detail label="Outcome" value={event.outcome || "Unrecorded"} />
          <Detail label="Risk" value={event.risk || "Unrecorded"} />
          <Detail
            label="Correlation"
            value={event.correlation_id || "Unrecorded"}
          />
          <Detail label="Run" value={event.run_id || "Unlinked"} />
          <Detail
            label="Revision"
            value={
              event.revision_hash ? shortHash(event.revision_hash) : "None"
            }
          />
        </EvidenceBlock>

        <EvidenceBlock title="Payload" icon={<FileSearch size={18} />}>
          <p className="admin-muted">
            Redaction mode: {event.redaction_metadata.mode}. Redacted fields:{" "}
            {event.redaction_metadata.redacted_fields.length
              ? event.redaction_metadata.redacted_fields.join(", ")
              : "None recorded"}
            .
          </p>
          <CodeBlock
            code={JSON.stringify(event.payload, null, 2)}
            language="json"
            maxHeight={350}
          />
        </EvidenceBlock>
      </div>
    </section>
  );
}

function EvidenceBlock({
  title,
  icon,
  children,
}: {
  title: string;
  icon: ReactNode;
  children: ReactNode;
}) {
  return (
    <section className="admin-detail-block admin-run-evidence">
      <div className="admin-section-heading">
        <div>
          <span>Immutable evidence</span>
          <h2>{title}</h2>
        </div>
        {icon}
      </div>
      {children}
    </section>
  );
}

function Detail({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="admin-detail-row">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function FilterInput({
  label,
  value,
  onChange,
  placeholder,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder: string;
}) {
  return (
    <label className="admin-filter-input">
      <span className="admin-field-label">{label}</span>
      <input
        className="ds-input"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        placeholder={placeholder}
      />
    </label>
  );
}

function resourceHref(event: AdminAuditEvent) {
  if (event.resource_type === "agent") return `/agents/${event.resource_id}`;
  if (event.resource_type === "run") return `/runs/${event.resource_id}`;
  if (event.resource_type === "policy") return `/policies/${event.resource_id}`;
  return undefined;
}

function AuditExportPanel({
  options,
}: {
  options: {
    workspaceId?: string;
    resourceType?: string;
    resourceId?: string;
    actorId?: string;
    eventType?: string;
    outcome?: string;
    risk?: string;
    correlationId?: string;
    runId?: string;
    revisionHash?: string;
  };
}) {
  const api = useAdminApi();
  const [exportId, setExportId] = useState("");
  const create = useMutation({
    mutationFn: () => api.createAuditExport(options),
    onSuccess: (item) => setExportId(item.id),
  });
  const status = useQuery({
    queryKey: ["admin-audit-export", exportId],
    queryFn: () => api.getAuditExport(exportId),
    enabled: exportId !== "",
    refetchInterval: (query) =>
      ["requested", "processing"].includes(query.state.data?.state ?? "")
        ? 1000
        : false,
  });
  const download = useMutation({
    mutationFn: () => api.downloadAuditExport(exportId),
  });
  const item = status.data ?? create.data;

  return (
    <div className="admin-export-panel">
      <Button
        size="sm"
        variant="secondary"
        isLoading={create.isPending}
        onClick={() => create.mutate()}
      >
        <ArrowDownToLine size={15} /> Export evidence
      </Button>
      {item ? (
        <span className="admin-export-status">
          {item.state}
          {item.package_digest ? ` · ${shortHash(item.package_digest)}` : ""}
        </span>
      ) : null}
      {item?.state === "ready" ? (
        <Button
          size="sm"
          variant="quiet"
          isLoading={download.isPending}
          onClick={() => download.mutate()}
        >
          <ArrowDownToLine size={15} /> Prepare download
        </Button>
      ) : null}
      {download.data ? (
        <a
          className="admin-inline-link"
          href={download.data.url}
          target="_blank"
          rel="noreferrer"
        >
          Download package
        </a>
      ) : null}
      {create.error || status.error || download.error ? (
        <span className="admin-error" role="alert">
          Audit export unavailable.
        </span>
      ) : null}
    </div>
  );
}
