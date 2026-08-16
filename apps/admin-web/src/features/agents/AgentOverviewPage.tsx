import { Activity, ArrowRight, Bot, FileCheck2, GitBranch, ShieldCheck } from 'lucide-react';
import { Link, useParams } from 'react-router-dom';
import { StatusMark } from '@gantry/design-system';
import { useQuery } from '@tanstack/react-query';
import { useAdminApi } from '../../api/ApiProvider';
import { ErrorState, LoadingState } from '../../components/AsyncState';

export function AgentOverviewPage() {
  const { agentId = '' } = useParams();
  const api = useAdminApi();
  const overview = useQuery({
    queryKey: ['admin-agent-overview', agentId],
    queryFn: () => api.getAgentOverview(agentId),
    enabled: agentId !== '',
  });

  if (overview.isLoading) return <LoadingState label="Loading agent overview" />;
  if (overview.error || !overview.data) {
    return <div className="admin-page"><ErrorState message="This agent overview is unavailable in your administrative scope." /></div>;
  }

  const { agent, draft, current_version: currentVersion, version_count: versionCount, recent_activity: activity } = overview.data;
  return (
    <section className="admin-page">
      <header className="admin-detail-heading">
        <div>
          <div className="admin-detail-title"><Bot size={22} /><h1>{agent.display_name}</h1><StatusMark status={agent.lifecycle_status} /></div>
          <p>{agent.description}</p>
        </div>
        <Link className="ds-button ds-button-primary" to={`/agents/${agent.id}/design`}><ArrowRight size={16} /> Edit design</Link>
      </header>

      <div className="admin-overview-grid">
        <section className="admin-detail-block">
          <div className="admin-section-heading"><div><span>Production</span><h2>Current deployment</h2></div><ShieldCheck size={18} /></div>
          {currentVersion ? <>
            <div className="admin-overview-highlight"><strong>Version {currentVersion.version}</strong><StatusMark status="published" /></div>
            <Detail label="Content digest" value={currentVersion.spec_digest} />
            <Detail label="Prompt snapshot" value={currentVersion.prompt_snapshot.content_digest} />
            <Detail label="Published at" value={currentVersion.published_at || 'Not available'} />
            <Link className="admin-inline-link" to={`/agents/${agent.id}/versions/${currentVersion.id}`}>Inspect immutable version <ArrowRight size={14} /></Link>
          </> : <p className="admin-muted">No production deployment.</p>}
        </section>

        <section className="admin-detail-block">
          <div className="admin-section-heading"><div><span>Working copy</span><h2>Draft status</h2></div><FileCheck2 size={18} /></div>
          <div className="admin-overview-highlight"><strong>Draft revision {draft.revision}</strong><StatusMark status={draft.validation_status} /></div>
          <Detail label="Validation findings" value={String(draft.validation_findings.length)} />
          <Detail label="Published versions" value={String(versionCount)} />
          <Link className="admin-inline-link" to={`/agents/${agent.id}/design`}>Open draft designer <ArrowRight size={14} /></Link>
        </section>
      </div>

      <section className="admin-detail-block admin-overview-activity">
        <div className="admin-section-heading"><div><span>Immutable record</span><h2>Recent activity</h2></div><Activity size={18} /></div>
        {activity.length === 0 ? <p className="admin-muted">No activity recorded.</p> : <ul className="admin-detail-list">{activity.map((item) => <li key={item.id}><strong>{item.event_type}</strong><span>{item.created_at}</span></li>)}</ul>}
      </section>

      <div className="admin-overview-footnote"><GitBranch size={15} /><span>Revision history remains available from the immutable version list.</span><Link to={`/agents/${agent.id}/design`}>Open design <ArrowRight size={14} /></Link></div>
    </section>
  );
}

function Detail({ label, value }: { label: string; value: string }) {
  return <div className="admin-detail-field"><span>{label}</span><strong>{value}</strong></div>;
}
