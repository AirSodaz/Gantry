import { useMemo, type ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Activity, AlertTriangle, ArrowRight, Bot, CheckCircle2, CircleAlert, Clock3, Layers, RefreshCw, ShieldAlert } from 'lucide-react';
import { Button, Select, type SelectOption } from '@gantry/design-system';
import { Link, useSearchParams } from 'react-router-dom';
import { useAdminApi } from '../../api/ApiProvider';
import { ErrorState, LoadingState } from '../../components/AsyncState';

export function OverviewPage() {
  const api = useAdminApi();
  const [searchParams, setSearchParams] = useSearchParams();
  const workspaceID = searchParams.get('workspace') ?? '';
  const workspaces = useQuery({ queryKey: ['admin-workspaces'], queryFn: () => api.listWorkspaces() });
  const overview = useQuery({ queryKey: ['admin-overview', workspaceID], queryFn: () => api.getOverview(workspaceID) });

  const workspaceOptions = useMemo<SelectOption[]>(() => [
    { value: '', label: 'All manageable workspaces', icon: <Layers size={13} /> },
    ...(workspaces.data?.items ?? []).map((workspace) => ({ value: workspace.id, label: workspace.display_name, icon: <Layers size={13} /> })),
  ], [workspaces.data?.items]);

  if (workspaces.isLoading || overview.isLoading) return <LoadingState label="Loading overview" />;
  if (workspaces.error || overview.error || !overview.data) return <div className="admin-page"><ErrorState message="The administrative overview could not be loaded." /></div>;

  const { metrics, attention, recent_publications: publications, recent_activity: activity, unavailable_signals: unavailable } = overview.data;
  return (
    <section className="admin-page">
      <header className="admin-page-heading">
        <div>
          <h1>Overview</h1>
          <p>Current configuration, governance, and execution signals for {overview.data.scope.label.toLowerCase()}.</p>
        </div>
        <Button variant="quiet" size="sm" onClick={() => void overview.refetch()}><RefreshCw size={15} /> Refresh</Button>
      </header>

      <div className="admin-filter-bar">
        <div className="admin-filter-select-wrap"><Select label="Workspace" options={workspaceOptions} value={workspaceID} onChange={(value) => setSearchParams(value ? { workspace: value } : {})} placeholder="All manageable workspaces" /></div>
      </div>

      <section className="admin-overview-attention" aria-labelledby="overview-attention-heading">
        <div className="admin-section-heading"><div><span>Attention queue</span><h2 id="overview-attention-heading">Needs action</h2></div><ShieldAlert size={18} /></div>
        {attention.length === 0 ? <div className="admin-overview-empty"><CheckCircle2 size={18} /><span>No action is required in this scope.</span></div> : <ul className="admin-overview-list">{attention.map((item) => <li key={item.id}><span className={`admin-attention-severity admin-attention-${item.severity}`}><AlertTriangle size={15} /></span><div><strong>{item.title}</strong><span>{item.description}</span></div><Link to={item.href} aria-label={`Open ${item.title}`}><ArrowRight size={15} /></Link></li>)}</ul>}
      </section>

      <section className="admin-overview-metrics" aria-label="Scope metrics">
        <Metric icon={<Bot size={16} />} label="Agents" value={metrics.agents_total} detail={`${metrics.published_agents} published`} href="/agents" />
        <Metric icon={<CircleAlert size={16} />} label="Reviews" value={metrics.drafts_needing_review} detail={`${metrics.invalid_drafts} invalid drafts`} href="/agents" />
        <Metric icon={<Activity size={16} />} label="Active runs" value={metrics.active_runs} detail={`${metrics.failed_runs_24_hours} failed in 24h`} />
        <Metric icon={<Clock3 size={16} />} label="Requester waits" value={metrics.awaiting_approvals} detail="Awaiting exact-action approval" />
      </section>

      <div className="admin-overview-grid">
        <section className="admin-detail-block">
          <div className="admin-section-heading"><div><span>Governance</span><h2>Recent publications</h2></div><Layers size={18} /></div>
          {publications.length === 0 ? <p className="admin-muted">No Production Deployments in this scope.</p> : <ul className="admin-detail-list">{publications.map((publication) => <li key={`${publication.agent_id}-${publication.revision_hash}`}><Link className="admin-overview-row-link" to={`/agents/${publication.agent_id}/revisions/${publication.revision_hash}`}><strong>{publication.agent_name} <span>{shortHash(publication.revision_hash)}</span></strong><span>{formatTime(publication.published_at)}</span></Link></li>)}</ul>}
        </section>
        <section className="admin-detail-block">
          <div className="admin-section-heading"><div><span>Immutable record</span><h2>Recent activity</h2></div><Activity size={18} /></div>
          {activity.length === 0 ? <p className="admin-muted">No scoped Agent activity recorded.</p> : <ul className="admin-detail-list">{activity.map((item) => <li key={item.id}><strong>{item.event_type}</strong><span>{formatTime(item.created_at)}</span></li>)}</ul>}
        </section>
      </div>

      <section className="admin-overview-unavailable" aria-label="Unavailable signals">
        <span>Not yet available</span>
        <ul>{unavailable.map((signal) => <li key={signal}>{signal}</li>)}</ul>
      </section>
    </section>
  );
}

function Metric({ icon, label, value, detail, href }: { icon: ReactNode; label: string; value: number; detail: string; href?: string }) {
  const content = <><span className="admin-overview-metric-icon">{icon}</span><div><span>{label}</span><strong>{value}</strong><small>{detail}</small></div></>;
  return href ? <Link className="admin-overview-metric" to={href}>{content}</Link> : <div className="admin-overview-metric">{content}</div>;
}

function formatTime(value: string) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

function shortHash(value: string) {
  return value.replace(/^sha256:/, '').slice(0, 12);
}
