import { Activity, ArrowRight, Bot, FileClock, GitBranch, ShieldCheck, TestTube2, Unplug } from 'lucide-react';
import { Link, useParams } from 'react-router-dom';
import { Button, StatusMark } from '@gantry/design-system';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useAdminApi } from '../../api/ApiProvider';
import { ErrorState, LoadingState } from '../../components/AsyncState';

export function AgentOverviewPage() {
  const { agentId = '' } = useParams();
  const api = useAdminApi();
  const queryClient = useQueryClient();
  const overview = useQuery({ queryKey: ['admin-agent-lifecycle', agentId], queryFn: () => api.getAgentLifecycle(agentId), enabled: agentId !== '' });
  const revisions = useQuery({ queryKey: ['admin-agent-revisions', agentId], queryFn: () => api.listRevisions(agentId), enabled: agentId !== '' });
  const stopDeployment = useMutation({ mutationFn: (deploymentId: string) => api.stopTestDeployment(agentId, deploymentId), onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['admin-agent-lifecycle', agentId] }) });

  if (overview.isLoading || revisions.isLoading) return <LoadingState label="Loading agent lifecycle" />;
  if (overview.error || revisions.error || !overview.data) return <div className="admin-page"><ErrorState message="This Agent lifecycle is unavailable in your administrative scope." /></div>;

  const { agent, main_draft: mainDraft, drafts, production_deployment: production, test_deployments: tests, revision_count: revisionCount, recent_activity: activity } = overview.data;
  return <section className="admin-page">
    <header className="admin-detail-heading"><div><div className="admin-detail-title"><Bot size={22} /><h1>{agent.display_name}</h1><StatusMark status={agent.lifecycle_status} /></div><p>{agent.description}</p></div><Link className="ds-button ds-button-primary" to={`/agents/${agent.id}/design`}><ArrowRight size={16} /> Edit Main Draft</Link></header>

    <div className="admin-overview-grid">
      <section className="admin-detail-block"><div className="admin-section-heading"><div><span>Production</span><h2>Default Deployment</h2></div><ShieldCheck size={18} /></div>{production ? <><div className="admin-overview-highlight"><strong>{shortHash(production.revision_hash)}</strong><StatusMark status={production.status} /></div><Detail label="Revision digest" value={production.spec_digest} /><Detail label="Last moved" value={production.updated_at} /><Link className="admin-inline-link" to={`/agents/${agent.id}/revisions/${production.revision_hash}`}>Inspect exact Revision <ArrowRight size={14} /></Link></> : <p className="admin-muted">No Production Deployment.</p>}</section>
      <section className="admin-detail-block"><div className="admin-section-heading"><div><span>Working copy</span><h2>Main Draft</h2></div><FileClock size={18} /></div><div className="admin-overview-highlight"><strong>ETag {mainDraft.working_copy_etag}</strong><StatusMark status={mainDraft.validation_status} /></div><Detail label="Named Drafts" value={String(drafts.length)} /><Detail label="Committed Revisions" value={String(revisionCount)} /><Link className="admin-inline-link" to={`/agents/${agent.id}/design`}>Open Main Draft <ArrowRight size={14} /></Link></section>
    </div>

    <div className="admin-overview-grid"><section className="admin-detail-block"><div className="admin-section-heading"><div><span>Experiments</span><h2>Test Deployments</h2></div><TestTube2 size={18} /></div>{tests.length === 0 ? <p className="admin-muted">No active or stopped Test Deployments.</p> : <ul className="admin-detail-list">{tests.map((deployment) => <li key={deployment.id}><div className="admin-overview-highlight"><strong>{deployment.name} <span>{shortHash(deployment.revision_hash)}</span></strong>{deployment.status === 'active' ? <Button size="sm" variant="quiet" disabled={stopDeployment.isPending} onClick={() => stopDeployment.mutate(deployment.id)} title={`Stop ${deployment.name}`}><Unplug size={14} /> Stop</Button> : <StatusMark status={deployment.status} />}</div><span>{deployment.purpose || 'No purpose recorded'}{deployment.expires_at ? ` · expires ${deployment.expires_at}` : ''}</span></li>)}</ul>}</section>
      <section className="admin-detail-block"><div className="admin-section-heading"><div><span>Immutable history</span><h2>Recent Revisions</h2></div><GitBranch size={18} /></div>{(revisions.data?.items ?? []).length === 0 ? <p className="admin-muted">No committed Revisions.</p> : <ul className="admin-detail-list">{revisions.data!.items.slice(0, 6).map((revision) => <li key={revision.revision_hash}><Link className="admin-overview-row-link" to={`/agents/${agent.id}/revisions/${revision.revision_hash}`}><strong>{shortHash(revision.revision_hash)} <span>{revision.message}</span></strong><span>{revision.created_at}</span></Link></li>)}</ul>}</section></div>

    <section className="admin-detail-block admin-overview-activity"><div className="admin-section-heading"><div><span>Immutable record</span><h2>Recent activity</h2></div><Activity size={18} /></div>{activity.length === 0 ? <p className="admin-muted">No lifecycle activity recorded.</p> : <ul className="admin-detail-list">{activity.map((item) => <li key={item.id}><strong>{item.event_type}</strong><span>{item.created_at}</span></li>)}</ul>}</section>
  </section>;
}

function Detail({ label, value }: { label: string; value: string }) { return <div className="admin-detail-field"><span>{label}</span><strong>{value}</strong></div>; }
function shortHash(value: string) { return value.replace(/^sha256:/, '').slice(0, 12); }
