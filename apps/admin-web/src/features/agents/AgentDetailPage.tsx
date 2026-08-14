import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, CheckCircle2, FileClock, Send, TriangleAlert } from 'lucide-react';
import { Button, StatusMark } from '@gantry/design-system';
import { Link, useParams } from 'react-router-dom';
import { useEffect, useState } from 'react';
import { useAdminApi } from '../../api/ApiProvider';
import type { DemoSpec } from '../../api/types';
import { ErrorState, LoadingState } from '../../components/AsyncState';

const defaultSpec: DemoSpec = { kind: 'gantry.phase0.demo/v1', mode: 'complete' };

export function AgentDetailPage() {
  const { agentId = '' } = useParams();
  const api = useAdminApi();
  const queryClient = useQueryClient();
  const agent = useQuery({ queryKey: ['admin-agent', agentId], queryFn: () => api.getAgent(agentId) });
  const draft = useQuery({ queryKey: ['admin-draft', agentId], queryFn: () => api.getDraft(agentId) });
  const versions = useQuery({ queryKey: ['admin-versions', agentId], queryFn: () => api.listVersions(agentId) });
  const [spec, setSpec] = useState<DemoSpec>(defaultSpec);
  useEffect(() => { if (draft.data?.spec) setSpec(draft.data.spec as unknown as DemoSpec); }, [draft.data?.spec]);
  const refresh = () => Promise.all([
    queryClient.invalidateQueries({ queryKey: ['admin-agent', agentId] }),
    queryClient.invalidateQueries({ queryKey: ['admin-draft', agentId] }),
    queryClient.invalidateQueries({ queryKey: ['admin-versions', agentId] }),
    queryClient.invalidateQueries({ queryKey: ['admin-agents'] }),
  ]);
  const save = useMutation({ mutationFn: () => api.updateDraft(agentId, draft.data!.revision, spec), onSuccess: refresh });
  const publish = useMutation({ mutationFn: () => api.publish(agentId, draft.data!.revision), onSuccess: refresh });
  const retire = useMutation({ mutationFn: () => api.retire(agentId), onSuccess: refresh });
  if (agent.isLoading || draft.isLoading || versions.isLoading) return <LoadingState label="Loading agent" />;
  if (agent.error || draft.error || versions.error || !agent.data || !draft.data) return <div className="admin-page"><ErrorState message="This agent is unavailable in your administrative scope." /></div>;
  const isDirty = spec.mode !== (draft.data.spec as unknown as DemoSpec).mode;
  const busy = save.isPending || publish.isPending || retire.isPending;
  return <section className="admin-page admin-detail-page">
    <Link className="admin-back-link" to="/"><ArrowLeft size={16} />Agents</Link>
    <header className="admin-detail-heading"><div><div className="admin-detail-title"><h1>{agent.data.display_name}</h1><StatusMark status={agent.data.lifecycle_status} /></div><p>{agent.data.description}</p></div><div className="admin-command-row"><Button variant="secondary" onClick={() => save.mutate()} disabled={!isDirty || busy}>Save revision</Button><Button onClick={() => publish.mutate()} disabled={draft.data.validation_status !== 'valid' || isDirty || busy}><Send size={16} />Publish</Button>{agent.data.lifecycle_status === 'published' ? <Button variant="danger" onClick={() => retire.mutate()} disabled={busy}>Retire</Button> : null}</div></header>
    {(save.error || publish.error || retire.error) ? <ErrorState message={(save.error ?? publish.error ?? retire.error)!.message} /> : null}
    <div className="admin-detail-grid">
      <section className="admin-editor"><div className="admin-section-heading"><div><span>Draft revision {draft.data.revision}</span><h2>Execution configuration</h2></div><FileClock size={19} /></div>
        <label className="admin-field"><span>Lifecycle mode</span><select value={spec.mode} onChange={(event) => setSpec({ ...spec, mode: event.target.value as DemoSpec['mode'] })} disabled={busy}><option value="complete">Complete</option><option value="await_cancel">Await cancel</option></select></label>
        <div className={`admin-validation ${draft.data.validation_status === 'valid' ? 'admin-validation-valid' : 'admin-validation-invalid'}`}>{draft.data.validation_status === 'valid' ? <CheckCircle2 size={17} /> : <TriangleAlert size={17} />}<div><strong>{draft.data.validation_status === 'valid' ? 'Draft is valid' : 'Draft needs attention'}</strong>{draft.data.validation_findings.map((finding) => <span key={`${finding.path}-${finding.message}`}>{finding.path || 'Specification'}: {finding.message}</span>)}</div></div>
        <dl className="admin-spec-summary"><div><dt>Manifest kind</dt><dd>{spec.kind}</dd></div><div><dt>Mode</dt><dd>{spec.mode}</dd></div></dl>
      </section>
      <aside className="admin-version-panel"><div className="admin-section-heading"><div><span>Immutable</span><h2>Version history</h2></div></div>{versions.data?.items.length === 0 ? <p className="admin-muted">No version has been published.</p> : <ol className="admin-version-list">{versions.data?.items.map((version) => <li key={version.id}><strong>Version {version.version}</strong><span>Draft revision {version.source_draft_revision}</span><code>{version.spec_digest.slice(0, 19)}...</code></li>)}</ol>}</aside>
    </div>
  </section>;
}
