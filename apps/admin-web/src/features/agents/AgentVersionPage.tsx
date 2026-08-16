import { ArrowLeft, FileText, ShieldCheck } from 'lucide-react';
import { Link, useParams } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { StatusMark } from '@gantry/design-system';
import { useAdminApi } from '../../api/ApiProvider';
import { ErrorState, LoadingState } from '../../components/AsyncState';

export function AgentVersionPage() {
  const { agentId = '', revisionHash = '' } = useParams();
  const api = useAdminApi();
  const revision = useQuery({ queryKey: ['admin-agent-revision', agentId, revisionHash], queryFn: () => api.getRevision(agentId, revisionHash), enabled: agentId !== '' && revisionHash !== '' });
  const review = useQuery({ queryKey: ['admin-revision-review', agentId, revisionHash], queryFn: () => api.getRevisionReview(agentId, revisionHash), enabled: agentId !== '' && revisionHash !== '' });
  if (revision.isLoading) return <LoadingState label="Loading immutable Revision" />;
  if (revision.error || !revision.data) return <div className="admin-page"><ErrorState message="This immutable Revision is unavailable in your administrative scope." /></div>;
  const item = revision.data;
  return <section className="admin-page"><Link className="admin-back-link" to={`/agents/${agentId}/versions`}><ArrowLeft size={15} /> Versions</Link><header className="admin-detail-heading"><div><div className="admin-detail-title"><FileText size={22} /><h1>{shortHash(item.revision_hash)}</h1><StatusMark status={review.data?.status ?? 'not_submitted'} /></div><p>{item.message}</p></div></header><div className="admin-detail-grid admin-version-detail-grid"><section className="admin-detail-block"><div className="admin-section-heading"><div><span>Identity</span><h2>Revision metadata</h2></div><ShieldCheck size={18} /></div><Detail label="Full revision hash" value={item.revision_hash} /><Detail label="Source Draft" value={item.source_draft_id} /><Detail label="Spec digest" value={item.spec_digest} /><Detail label="Prompt snapshot digest" value={item.prompt_snapshot.content_digest} /><Detail label="Compiler" value={item.prompt_snapshot.compiler_version} /><Detail label="Created" value={item.created_at} /><Detail label="Created by" value={item.created_by} /></section><section className="admin-detail-block"><div className="admin-section-heading"><div><span>Prompt snapshot</span><h2>Compiled content</h2></div><FileText size={18} /></div><pre className="admin-json-block admin-prompt-snapshot">{item.prompt_snapshot.compiled_text || '(empty prompt)'}</pre></section><section className="admin-detail-block admin-version-spec-block"><div className="admin-section-heading"><div><span>Canonical</span><h2>Configuration specification</h2></div></div><pre className="admin-json-block">{JSON.stringify(item.spec, null, 2)}</pre></section></div></section>;
}

function shortHash(value: string) { return value.replace(/^sha256:/, '').slice(0, 12); }
function Detail({ label, value }: { label: string; value: string }) { return <div className="admin-detail-field"><span>{label}</span><strong>{value}</strong></div>; }
