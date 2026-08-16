import { useMemo, type ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Activity, ArrowLeft, Bot, Boxes, FileCheck2, ShieldCheck, Wrench } from 'lucide-react';
import { Select, StatusMark, type SelectOption } from '@gantry/design-system';
import { Link, useParams, useSearchParams } from 'react-router-dom';
import { useAdminApi } from '../../api/ApiProvider';
import type { AdminRun } from '../../api/types';
import { ErrorState, LoadingState } from '../../components/AsyncState';

const statuses: SelectOption[] = [
  { value: '', label: 'All states' },
  ...['queued', 'assigned', 'accepted', 'awaiting_approval', 'canceling', 'completed', 'failed', 'canceled'].map((value) => ({ value, label: value.replace(/_/g, ' ') })),
];

export function RunsPage() {
  const api = useAdminApi();
  const [searchParams, setSearchParams] = useSearchParams();
  const workspaceID = searchParams.get('workspace') ?? '';
  const agentID = searchParams.get('agent') ?? '';
  const revisionHash = searchParams.get('revision') ?? '';
  const status = searchParams.get('status') ?? '';
  const workspaces = useQuery({ queryKey: ['admin-workspaces'], queryFn: () => api.listWorkspaces() });
  const runs = useQuery({ queryKey: ['admin-runs', workspaceID, agentID, revisionHash, status], queryFn: () => api.listRuns({ workspaceId: workspaceID, agentId: agentID, revisionHash, status: status as AdminRun['status'] }) });
  const workspaceOptions = useMemo<SelectOption[]>(() => [{ value: '', label: 'All manageable workspaces' }, ...(workspaces.data?.items ?? []).map((workspace) => ({ value: workspace.id, label: workspace.display_name }))], [workspaces.data?.items]);
  const update = (key: string, value: string) => {
    const next = new URLSearchParams(searchParams);
    if (value) next.set(key, value); else next.delete(key);
    setSearchParams(next);
  };

  if (workspaces.isLoading || runs.isLoading) return <LoadingState label="Loading operational runs" />;
  if (workspaces.error || runs.error) return <div className="admin-page"><ErrorState message="The authorized run workbench could not be loaded." /></div>;
  const items = runs.data?.items ?? [];
  return <section className="admin-page">
    <header className="admin-page-heading"><div><h1>Runs</h1><p>Durable operational evidence across your authorized workspaces.</p></div></header>
    <div className="admin-filter-bar admin-run-filter-bar">
      <Select label="Workspace" options={workspaceOptions} value={workspaceID} onChange={(value) => update('workspace', value)} placeholder="All manageable workspaces" />
      <Select label="State" options={statuses} value={status} onChange={(value) => update('status', value)} placeholder="All states" />
      <label className="admin-filter-input"><span className="admin-field-label">Agent ID</span><input className="ds-input" value={agentID} onChange={(event) => update('agent', event.target.value)} placeholder="Filter by Agent" /></label>
      <label className="admin-filter-input"><span className="admin-field-label">Revision hash</span><input className="ds-input" value={revisionHash} onChange={(event) => update('revision', event.target.value)} placeholder="sha256:..." /></label>
    </div>
    {items.length === 0 ? <div className="admin-empty"><Activity size={22} /><strong>No Runs match this scope</strong><span>Historical Runs remain available when their workspace is in your administrative scope.</span></div> : <div className="admin-table-wrap"><table className="admin-table admin-runs-table"><thead><tr><th>Run</th><th>Agent and Revision</th><th>Deployment</th><th>Requester</th><th>State</th><th>Evidence</th><th>Last event</th></tr></thead><tbody>{items.map((run) => <RunRow key={run.id} run={run} />)}</tbody></table></div>}
  </section>;
}

function RunRow({ run }: { run: AdminRun }) {
  return <tr><td><Link className="admin-inline-link" to={`/runs/${run.id}`}>{run.id}</Link><br /><span className="admin-muted">Task {run.task_id}</span></td><td><strong>{run.agent_name}</strong><br /><Link className="admin-inline-link" to={`/agents/${run.agent_id}/revisions/${run.revision_hash}`}>{shortHash(run.revision_hash)}</Link></td><td>{run.deployment_name ? <><strong>{run.deployment_name}</strong><br /><span className="admin-muted">{run.deployment_id}</span></> : <span className="admin-muted">Unrecorded</span>}</td><td>{run.requester_name}</td><td><StatusMark status={run.status} />{run.status_reason ? <small className="admin-run-reason">{run.status_reason}</small> : null}</td><td><strong>{run.action_count}</strong> tools<br /><strong>{run.approval_count}</strong> approvals</td><td>{formatTime(run.last_event_at || run.completed_at || run.started_at || run.created_at)}</td></tr>;
}

export function RunDetailPage() {
  const { runId = '' } = useParams();
  const api = useAdminApi();
  const detail = useQuery({ queryKey: ['admin-run', runId], queryFn: () => api.getRun(runId), enabled: runId !== '' });
  if (detail.isLoading) return <LoadingState label="Loading run evidence" />;
  if (detail.error || !detail.data) return <div className="admin-page"><ErrorState message="This Run is unavailable in your administrative scope." /></div>;
  const { run, events, actions, approvals, artifacts } = detail.data;
  return <section className="admin-page"><Link className="admin-back-link" to="/runs"><ArrowLeft size={15} /> Runs</Link><header className="admin-detail-heading"><div><div className="admin-detail-title"><Activity size={22} /><h1>{run.id}</h1><StatusMark status={run.status} /></div><p>Task {run.task_id} · Attempt {run.attempt_number} · {run.workspace_name}</p></div></header>
    <div className="admin-detail-grid admin-run-detail-grid">
      <EvidenceBlock title="Execution identity" icon={<Bot size={18} />}><Detail label="Agent" value={run.agent_name} /><Detail label="Revision" value={run.revision_hash} link={`/agents/${run.agent_id}/revisions/${run.revision_hash}`} /><Detail label="Deployment" value={run.deployment_name || 'Unrecorded'} /><Detail label="Manifest digest" value={run.manifest_digest || 'Unrecorded'} /><Detail label="Requester" value={run.requester_name} /></EvidenceBlock>
      <EvidenceBlock title="Runner and timing" icon={<Boxes size={18} />}><Detail label="Runner" value={run.runner_id || 'Not assigned'} /><Detail label="Created" value={formatTime(run.created_at)} /><Detail label="Started" value={formatTime(run.started_at)} /><Detail label="Completed" value={formatTime(run.completed_at)} /><Detail label="Reason" value={run.status_reason || 'None'} /></EvidenceBlock>
    </div>
    <EvidenceBlock title="Timeline" icon={<Activity size={18} />}>{events.length === 0 ? <p className="admin-muted">No durable events were recorded.</p> : <ol className="admin-run-timeline">{events.map((event) => <li key={`${event.sequence}-${event.type}`}><strong>{event.type}</strong><span>#{event.sequence} · {formatTime(event.created_at)}</span>{Object.keys(event.payload).length > 0 ? <pre className="admin-json-block">{JSON.stringify(event.payload, null, 2)}</pre> : null}</li>)}</ol>}</EvidenceBlock>
    <div className="admin-detail-grid admin-run-detail-grid">
      <EvidenceBlock title="Tool actions" icon={<Wrench size={18} />}>{actions.length === 0 ? <p className="admin-muted">No Tool actions were recorded.</p> : <ul className="admin-detail-list">{actions.map((action) => <li key={action.id}><strong>{action.tool_name} · {action.operation}</strong><span>{action.state} · {action.effect} · {shortHash(action.action_digest)}</span></li>)}</ul>}</EvidenceBlock>
      <EvidenceBlock title="Requester approvals" icon={<ShieldCheck size={18} />}>{approvals.length === 0 ? <p className="admin-muted">No approval evidence was recorded.</p> : <ul className="admin-detail-list">{approvals.map((approval) => <li key={approval.id}><strong>{approval.status} · {approval.risk_class}</strong><span>{shortHash(approval.action_digest)} · expires {formatTime(approval.expires_at)}</span></li>)}</ul>}</EvidenceBlock>
    </div>
    <EvidenceBlock title="Artifacts" icon={<FileCheck2 size={18} />}>{artifacts.length === 0 ? <p className="admin-muted">No Artifacts were recorded.</p> : <ul className="admin-detail-list">{artifacts.map((artifact) => <li key={artifact.id}><strong>{artifact.filename}</strong><span>{artifact.state} · {artifact.scan_status} · {artifact.size_bytes} bytes · {shortHash(artifact.digest)}</span></li>)}</ul>}</EvidenceBlock>
  </section>;
}

function EvidenceBlock({ title, icon, children }: { title: string; icon: ReactNode; children: ReactNode }) {
  return <section className="admin-detail-block admin-run-evidence"><div className="admin-section-heading"><div><span>Immutable evidence</span><h2>{title}</h2></div>{icon}</div>{children}</section>;
}

function Detail({ label, value, link }: { label: string; value: string; link?: string }) {
  return <div className="admin-detail-row"><span>{label}</span>{link ? <Link className="admin-inline-link" to={link}>{value}</Link> : <strong>{value}</strong>}</div>;
}

function formatTime(value: string | undefined) {
  if (!value) return 'Not recorded';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

function shortHash(value: string) { return value.replace(/^sha256:/, '').slice(0, 12); }
