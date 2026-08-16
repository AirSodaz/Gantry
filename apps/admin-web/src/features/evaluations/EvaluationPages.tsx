import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, Check, ClipboardCheck, FilePlus2, Play, RefreshCw } from 'lucide-react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { Button, Select, type SelectOption } from '@gantry/design-system';
import { useAdminApi } from '../../api/ApiProvider';
import type { EvaluationSuite } from '../../api/types';
import { ErrorState, LoadingState } from '../../components/AsyncState';

export function EvaluationsPage() {
  const api = useAdminApi();
  const navigate = useNavigate();
  const [workspaceId, setWorkspaceId] = useState('');
  const [name, setName] = useState('');
  const workspaces = useQuery({ queryKey: ['admin-workspaces'], queryFn: () => api.listWorkspaces() });
  const suites = useQuery({ queryKey: ['admin-evaluation-suites', workspaceId], queryFn: () => api.listEvaluationSuites({ workspaceId }), enabled: workspaceId !== '' });
  const create = useMutation({ mutationFn: () => api.createEvaluationSuite({ workspace_id: workspaceId, name }), onSuccess: (item) => { void suites.refetch(); navigate(`/evaluations/${item.id}`); } });
  if (workspaces.isLoading || (workspaceId !== '' && suites.isLoading)) return <LoadingState label="Loading evaluations" />;
  if (workspaces.error || suites.error) return <div className="admin-page"><ErrorState message="The evaluation workspace could not be loaded." /></div>;
  const options: SelectOption[] = (workspaces.data?.items ?? []).map((item) => ({ value: item.id, label: item.display_name }));
  return <section className="admin-page"><header className="admin-page-heading"><div><h1>Evaluations</h1><p>Freeze typed cases and fixture manifests before requesting comparable Runs.</p></div></header><div className="admin-filter-bar"><Select label="Workspace" options={options} value={workspaceId} onChange={setWorkspaceId} placeholder="Choose a workspace" /></div>{workspaceId ? <form className="admin-form-panel" onSubmit={(event) => { event.preventDefault(); create.mutate(); }}><div className="admin-form-actions"><input className="ds-input" value={name} onChange={(event) => setName(event.target.value)} placeholder="New suite name" required /><Button type="submit" isLoading={create.isPending}><FilePlus2 size={15} /> Create suite</Button></div></form> : null}{workspaceId && (suites.data?.items ?? []).length === 0 ? <div className="admin-empty"><ClipboardCheck size={22} /><strong>No Evaluation Suites</strong><span>Create a Draft to define cases and fixtures.</span></div> : <div className="admin-table-wrap"><table className="admin-table"><thead><tr><th>Name</th><th>State</th><th>Version</th><th>ETag</th></tr></thead><tbody>{suites.data?.items.map((suite) => <tr key={suite.id}><td><Link className="admin-inline-link" to={`/evaluations/${suite.id}`}>{suite.name}</Link><br /><span className="admin-muted">{suite.id}</span></td><td>{suite.state}</td><td>{suite.latest_version_id ?? 'Unpublished'}</td><td>{suite.etag}</td></tr>)}</tbody></table></div>}</section>;
}

export function EvaluationDetailPage() {
  const { suiteId = '' } = useParams();
  const api = useAdminApi();
  const queryClient = useQueryClient();
  const suite = useQuery({ queryKey: ['admin-evaluation-suite', suiteId], queryFn: () => api.getEvaluationSuite(suiteId), enabled: suiteId !== '' });
  const cases = useQuery({ queryKey: ['admin-evaluation-cases', suiteId], queryFn: () => api.listEvaluationCases(suiteId), enabled: suiteId !== '' });
  const versions = useQuery({ queryKey: ['admin-evaluation-versions', suiteId], queryFn: () => api.listEvaluationVersions(suiteId), enabled: suiteId !== '' });
  const runs = useQuery({ queryKey: ['admin-evaluation-runs', suiteId], queryFn: () => api.listEvaluationRuns(suiteId), enabled: suiteId !== '' });
  const [caseInput, setCaseInput] = useState('{"prompt":"hello"}');
  const [revisionHash, setRevisionHash] = useState('');
  const [environmentDigest, setEnvironmentDigest] = useState('sha256:development');
  const [runtimeDigest, setRuntimeDigest] = useState('sha256:development-evaluator');
  const createCase = useMutation({ mutationFn: () => api.createEvaluationCase(suiteId, { input: JSON.parse(caseInput) as Record<string, unknown>, fixture_manifest: { files: [] }, assertions: [{ type: 'contains', value: 'deterministic' }] }), onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ['admin-evaluation-cases', suiteId] }); } });
  const validate = useMutation({ mutationFn: () => api.validateEvaluationSuite(suiteId) });
  const publish = useMutation({ mutationFn: () => api.publishEvaluationVersion(suiteId, suite.data?.etag ?? '', { runtime_image_digest: runtimeDigest }, `evaluation-publish-${suiteId}-${suite.data?.etag ?? ''}`), onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ['admin-evaluation-suite', suiteId] }); void queryClient.invalidateQueries({ queryKey: ['admin-evaluation-versions', suiteId] }); } });
  const startRun = useMutation({ mutationFn: () => { const list = versions.data?.items ?? []; return api.createEvaluationRun(suiteId, { suite_version_id: list.length ? list[list.length - 1].id : '', candidate_revision_hash: revisionHash, environment_digest: environmentDigest }, `evaluation-run-${suiteId}-${revisionHash}`); }, onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ['admin-evaluation-runs', suiteId] }); } });
  if (suite.isLoading || cases.isLoading || versions.isLoading || runs.isLoading) return <LoadingState label="Loading evaluation suite" />;
  if (suite.error || cases.error || versions.error || runs.error || !suite.data) return <div className="admin-page"><ErrorState message="This Evaluation Suite is unavailable in your scope." /></div>;
  const item: EvaluationSuite = suite.data;
  return <section className="admin-page"><Link className="admin-back-link" to="/evaluations"><ArrowLeft size={15} /> Evaluations</Link><header className="admin-detail-heading"><div><div className="admin-detail-title"><ClipboardCheck size={22} /><h1>{item.name}</h1></div><p>{item.state} · {item.id}</p></div><div className="admin-detail-actions"><Button size="sm" variant="secondary" onClick={() => validate.mutate()} isLoading={validate.isPending}><RefreshCw size={15} /> Validate</Button><Button size="sm" onClick={() => publish.mutate()} isLoading={publish.isPending} disabled={!cases.data?.items.length || !runtimeDigest}><Check size={15} /> Publish Version</Button></div></header><div className="admin-detail-grid"><section className="admin-detail-block"><div className="admin-section-heading"><div><span>Working copy</span><h2>Cases</h2></div><ClipboardCheck size={18} /></div>{cases.data?.items.map((testCase) => <div className="admin-list-row" key={testCase.id}><strong>{testCase.id}</strong><span>ETag {testCase.etag} · fixture manifest pinned</span></div>)}<div className="admin-form-actions"><input className="ds-input" value={caseInput} onChange={(event) => setCaseInput(event.target.value)} /><Button size="sm" onClick={() => createCase.mutate()} isLoading={createCase.isPending}><FilePlus2 size={15} /> Add case</Button></div></section><section className="admin-detail-block"><div className="admin-section-heading"><div><span>Frozen artifacts</span><h2>Versions</h2></div><Check size={18} /></div>{versions.data?.items.map((version) => <div className="admin-list-row" key={version.id}><strong>{version.id}</strong><span>{version.content_digest.slice(0, 18)} · {version.runtime_image_digest}</span></div>)}<input className="ds-input admin-form-spaced" value={runtimeDigest} onChange={(event) => setRuntimeDigest(event.target.value)} aria-label="Runtime image digest" /></section></div><section className="admin-detail-block"><div className="admin-section-heading"><div><span>Exact Revision + immutable Version</span><h2>Runs</h2></div><Play size={18} /></div>{runs.data?.items.map((run) => <div className="admin-list-row" key={run.id}><strong>{run.state}</strong><span>{run.id} · {run.candidate_revision_hash}</span></div>)}<div className="admin-form-actions"><input className="ds-input" value={revisionHash} onChange={(event) => setRevisionHash(event.target.value)} placeholder="sha256:agent-revision" /><input className="ds-input" value={environmentDigest} onChange={(event) => setEnvironmentDigest(event.target.value)} /><Button size="sm" variant="secondary" onClick={() => startRun.mutate()} isLoading={startRun.isPending} disabled={!revisionHash || !versions.data?.items.length}><Play size={15} /> Request Run</Button></div>{startRun.error ? <p className="admin-error" role="alert">Run request was rejected because the exact Revision or immutable environment contract is missing.</p> : null}</section></section>;
}
