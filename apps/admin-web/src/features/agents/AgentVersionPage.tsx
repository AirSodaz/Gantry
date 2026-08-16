import { ArrowLeft, FileText, ShieldCheck } from 'lucide-react';
import { Link, useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { StatusMark } from '@gantry/design-system';
import { useAdminApi } from '../../api/ApiProvider';
import { ErrorState, LoadingState } from '../../components/AsyncState';

export function AgentVersionPage() {
  const { agentId = '', versionId = '' } = useParams();
  const api = useAdminApi();
  const version = useQuery({
    queryKey: ['admin-agent-version', agentId, versionId],
    queryFn: () => api.getVersion(agentId, versionId),
    enabled: agentId !== '' && versionId !== '',
  });

  if (version.isLoading) return <LoadingState label="Loading immutable version" />;
  if (version.error || !version.data) {
    return <div className="admin-page"><ErrorState message="This immutable version is unavailable in your administrative scope." /></div>;
  }
  const item = version.data;
  return <section className="admin-page">
    <Link className="admin-back-link" to={`/agents/${agentId}`}><ArrowLeft size={15} /> Agent overview</Link>
    <header className="admin-detail-heading">
      <div><div className="admin-detail-title"><FileText size={22} /><h1>Version {item.version}</h1><StatusMark status={item.published ? 'published' : 'retired'} /></div><p>Immutable configuration snapshot for Agent {item.agent_id}</p></div>
    </header>
    <div className="admin-detail-grid admin-version-detail-grid">
      <section className="admin-detail-block"><div className="admin-section-heading"><div><span>Identity</span><h2>Version metadata</h2></div><ShieldCheck size={18} /></div>
        <Detail label="Version ID" value={item.id} /><Detail label="Source draft revision" value={String(item.source_draft_revision)} /><Detail label="Spec digest" value={item.spec_digest} /><Detail label="Prompt snapshot digest" value={item.prompt_snapshot.content_digest} /><Detail label="Compiler" value={item.prompt_snapshot.compiler_version} /><Detail label="Created" value={item.created_at || 'Not available'} /><Detail label="Created by" value={item.created_by || 'Not available'} />
      </section>
      <section className="admin-detail-block"><div className="admin-section-heading"><div><span>Prompt snapshot</span><h2>Compiled content</h2></div><FileText size={18} /></div><pre className="admin-json-block admin-prompt-snapshot">{item.prompt_snapshot.compiled_text || '(empty prompt)'}</pre></section>
      <section className="admin-detail-block admin-version-spec-block"><div className="admin-section-heading"><div><span>Canonical</span><h2>Configuration specification</h2></div></div><pre className="admin-json-block">{JSON.stringify(item.spec, null, 2)}</pre></section>
    </div>
  </section>;
}

function Detail({ label, value }: { label: string; value: string }) {
  return <div className="admin-detail-field"><span>{label}</span><strong>{value}</strong></div>;
}
