import { useState, type ReactNode } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, Check, ClipboardCheck, FilePlus2, Play, RefreshCw, ShieldCheck, TrendingDown } from 'lucide-react';
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { Button, Select, type SelectOption } from '@gantry/design-system';
import { useAdminApi } from '../../api/ApiProvider';
import type { EvaluationGate, EvaluationRun, EvaluationSuite } from '../../api/types';
import { ErrorState, LoadingState } from '../../components/AsyncState';
import './EvaluationPages.css';

const DEFAULT_CASE_INPUT = '{"prompt":"hello"}';
const DEFAULT_RUNTIME_DIGEST = 'sha256:development-evaluator';
const DEFAULT_ENVIRONMENT_DIGEST = 'sha256:development';

function parseCaseInput(value: string): Record<string, unknown> {
  const parsed: unknown = JSON.parse(value);
  if (parsed === null || Array.isArray(parsed) || typeof parsed !== 'object') throw new Error('Case input must be a JSON object.');
  return parsed as Record<string, unknown>;
}

function SectionHeading({ eyebrow, title, icon }: { eyebrow: string; title: string; icon: ReactNode }) {
  return <div className="evaluation-section-heading"><div><span>{eyebrow}</span><h2>{title}</h2></div>{icon}</div>;
}

export function EvaluationsPage() {
  const api = useAdminApi();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const workspaceId = searchParams.get('workspace') ?? '';
  const [name, setName] = useState('');
  const workspaces = useQuery({ queryKey: ['admin-workspaces'], queryFn: () => api.listWorkspaces() });
  const suites = useQuery({ queryKey: ['admin-evaluation-suites', workspaceId], queryFn: () => api.listEvaluationSuites({ workspaceId }), enabled: workspaceId !== '' });
  const create = useMutation({ mutationFn: () => api.createEvaluationSuite({ workspace_id: workspaceId, name }), onSuccess: (item) => { void suites.refetch(); navigate(`/evaluations/${item.id}`); } });
  if (workspaces.isLoading || (workspaceId !== '' && suites.isLoading)) return <LoadingState label="Loading evaluations" />;
  if (workspaces.error || suites.error) return <main className="admin-page"><ErrorState message="The evaluation workspace could not be loaded." /></main>;
  const options: SelectOption[] = (workspaces.data?.items ?? []).map((item) => ({ value: item.id, label: item.display_name }));
  const items = suites.data?.items ?? [];

  return <main className="admin-page evaluation-page">
    <header className="admin-page-heading"><div><h1>Evaluations</h1><p>Freeze typed cases and fixture manifests before requesting comparable Runs.</p></div></header>
    <div className="evaluation-toolbar"><Select label="Workspace" options={options} value={workspaceId} onChange={(value) => setSearchParams(value ? { workspace: value } : {})} placeholder="Choose a workspace" /></div>
    {workspaceId ? <form className="evaluation-create" onSubmit={(event) => { event.preventDefault(); create.mutate(); }}><label className="admin-field"><span className="admin-field-label">Suite name</span><input className="ds-input" value={name} onChange={(event) => setName(event.target.value)} required /></label><Button type="submit" isLoading={create.isPending}><FilePlus2 size={15} /> Create suite</Button></form> : null}
    {workspaceId && items.length === 0 ? <div className="admin-empty"><ClipboardCheck size={22} /><strong>No Evaluation Suites</strong><span>Create a Draft to define cases and fixtures.</span></div> : null}
    {items.length > 0 ? <div className="admin-table-wrap"><table className="admin-table"><thead><tr><th>Name</th><th>State</th><th>Version</th><th>Gate use</th></tr></thead><tbody>{items.map((suite) => <tr key={suite.id}><td><Link className="admin-inline-link" to={`/evaluations/${suite.id}`}>{suite.name}</Link><br /><span className="admin-muted">{suite.id}</span></td><td>{suite.state}</td><td>{suite.latest_version_id ?? 'Unpublished'}</td><td>{suite.gate_usage_count}</td></tr>)}</tbody></table></div> : null}
  </main>;
}

function CaseSection({ suiteId, suiteState, input, onInputChange }: { suiteId: string; suiteState: EvaluationSuite['state']; input: string; onInputChange: (value: string) => void }) {
  const api = useAdminApi();
  const queryClient = useQueryClient();
  const isDraft = suiteState === 'draft';
  const cases = useQuery({ queryKey: ['admin-evaluation-cases', suiteId], queryFn: () => api.listEvaluationCases(suiteId) });
  const create = useMutation({ mutationFn: () => api.createEvaluationCase(suiteId, { input: parseCaseInput(input), fixture_manifest: { files: [] }, assertions: [{ type: 'contains', value: 'deterministic' }] }), onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['admin-evaluation-cases', suiteId] }) });
  return <section className="evaluation-section"><SectionHeading eyebrow="Working copy" title="Cases" icon={<ClipboardCheck size={18} />} />{cases.data?.items.map((item) => <div className="evaluation-row" key={item.id}><strong>{item.id}</strong><span>ETag {item.etag} · fixture manifest pinned</span></div>)}<div className="evaluation-command"><label className="admin-field"><span className="admin-field-label">Case input</span><input className="ds-input" value={input} onChange={(event) => onInputChange(event.target.value)} disabled={!isDraft} /></label><Button size="sm" onClick={() => create.mutate()} isLoading={create.isPending} disabled={!isDraft}><FilePlus2 size={15} /> Add case</Button></div>{create.error ? <p className="admin-error" role="alert">The case could not be added in the current Suite state.</p> : null}</section>;
}

function VersionSection({ suite, runtimeDigest, onRuntimeDigestChange }: { suite: EvaluationSuite; runtimeDigest: string; onRuntimeDigestChange: (value: string) => void }) {
  const api = useAdminApi();
  const queryClient = useQueryClient();
  const isDraft = suite.state === 'draft';
  const versions = useQuery({ queryKey: ['admin-evaluation-versions', suite.id], queryFn: () => api.listEvaluationVersions(suite.id) });
  const cases = useQuery({ queryKey: ['admin-evaluation-cases', suite.id], queryFn: () => api.listEvaluationCases(suite.id) });
  const publish = useMutation({ mutationFn: () => api.publishEvaluationVersion(suite.id, suite.etag, { runtime_image_digest: runtimeDigest }, `evaluation-publish-${suite.id}-${suite.etag}`), onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ['admin-evaluation-suite', suite.id] }); void queryClient.invalidateQueries({ queryKey: ['admin-evaluation-versions', suite.id] }); } });
  return <section className="evaluation-section"><SectionHeading eyebrow="Frozen artifacts" title="Versions" icon={<Check size={18} />} />{versions.data?.items.map((item) => <div className="evaluation-row" key={item.id}><strong>{item.id}</strong><span>{item.content_digest.slice(0, 18)} · {item.runtime_image_digest}</span></div>)}<div className="evaluation-command"><label className="admin-field"><span className="admin-field-label">Runtime image digest</span><input className="ds-input" value={runtimeDigest} onChange={(event) => onRuntimeDigestChange(event.target.value)} disabled={!isDraft} /></label><Button size="sm" onClick={() => publish.mutate()} isLoading={publish.isPending} disabled={!isDraft || !cases.data?.items.length || !runtimeDigest}><Check size={15} /> Publish version</Button></div></section>;
}

function RunRow({ item, onSelect }: { item: EvaluationRun; onSelect: (id: string) => void }) {
  return <div className="evaluation-row"><div><strong>{item.state}</strong><span>{item.id} · {item.candidate_revision_hash}</span></div><Button size="sm" variant="quiet" onClick={() => onSelect(item.id)}>Regressions</Button></div>;
}

function RunSection({ suite, revisionHash, environmentDigest, onRevisionHashChange, onEnvironmentDigestChange, onSelectRun }: { suite: EvaluationSuite; revisionHash: string; environmentDigest: string; onRevisionHashChange: (value: string) => void; onEnvironmentDigestChange: (value: string) => void; onSelectRun: (id: string) => void }) {
  const api = useAdminApi();
  const queryClient = useQueryClient();
  const versions = useQuery({ queryKey: ['admin-evaluation-versions', suite.id], queryFn: () => api.listEvaluationVersions(suite.id) });
  const runs = useQuery({ queryKey: ['admin-evaluation-runs', suite.id], queryFn: () => api.listEvaluationRuns(suite.id) });
  const requestRun = useMutation({ mutationFn: () => {
    const versionItems = versions.data?.items ?? [];
    const version = versionItems[versionItems.length - 1];
    return api.createEvaluationRun(suite.id, { suite_version_id: version?.id ?? '', candidate_revision_hash: revisionHash, environment_digest: environmentDigest }, `evaluation-run-${suite.id}-${revisionHash}`);
  }, onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['admin-evaluation-runs', suite.id] }) });
  return <section className="evaluation-section"><SectionHeading eyebrow="Exact Revision + immutable Version" title="Runs" icon={<Play size={18} />} />{runs.data?.items.map((item) => <RunRow key={item.id} item={item} onSelect={onSelectRun} />)}<div className="evaluation-command evaluation-command-wide"><label className="admin-field"><span className="admin-field-label">Candidate revision</span><input className="ds-input" value={revisionHash} onChange={(event) => onRevisionHashChange(event.target.value)} placeholder="sha256:agent-revision" /></label><label className="admin-field"><span className="admin-field-label">Environment digest</span><input className="ds-input" value={environmentDigest} onChange={(event) => onEnvironmentDigestChange(event.target.value)} /></label><Button size="sm" variant="secondary" onClick={() => requestRun.mutate()} isLoading={requestRun.isPending} disabled={!revisionHash || !versions.data?.items.length}><Play size={15} /> Request Run</Button></div>{requestRun.error ? <p className="admin-error" role="alert">Run request was rejected because the exact Revision or immutable environment contract is missing.</p> : null}</section>;
}

function GateSection({ suite, agentRevisionHash }: { suite: EvaluationSuite; agentRevisionHash: string }) {
  const api = useAdminApi();
  const queryClient = useQueryClient();
  const [selectedGate, setSelectedGate] = useState<EvaluationGate | null>(null);
  const [reason, setReason] = useState('');
  const [expiry, setExpiry] = useState('');
  const gates = useQuery({ queryKey: ['admin-evaluation-gates', suite.workspace_id, agentRevisionHash], queryFn: () => api.listEvaluationGates({ workspaceId: suite.workspace_id, agentRevisionHash: agentRevisionHash || undefined }) });
  const override = useMutation({ mutationFn: () => api.overrideEvaluationGate(selectedGate?.id ?? '', { reason, expires_at: new Date(expiry).toISOString() }), onSuccess: () => { setSelectedGate(null); setReason(''); setExpiry(''); void queryClient.invalidateQueries({ queryKey: ['admin-evaluation-gates'] }); } });
  return <section className="evaluation-section"><SectionHeading eyebrow="Publication evidence" title="Gates" icon={<ShieldCheck size={18} />} />{gates.data?.items.map((item) => <div className="evaluation-row" key={item.id}><div><strong>{item.state}</strong><span>{item.agent_revision_hash} · {item.suite_version_id}</span></div>{item.state !== 'passed' && item.state !== 'expired' ? <Button size="sm" variant="quiet" onClick={() => setSelectedGate(item)}>Override</Button> : null}</div>)}{gates.isLoading ? <p className="admin-muted">Loading gate evidence.</p> : null}{!gates.data?.items.length && !gates.isLoading ? <p className="admin-muted">A required Gate is created when an immutable Run is requested.</p> : null}{selectedGate ? <form className="evaluation-command evaluation-command-wide" onSubmit={(event) => { event.preventDefault(); override.mutate(); }}><label className="admin-field"><span className="admin-field-label">Override reason</span><input className="ds-input" value={reason} onChange={(event) => setReason(event.target.value)} required /></label><label className="admin-field"><span className="admin-field-label">Expiry</span><input className="ds-input" type="datetime-local" value={expiry} onChange={(event) => setExpiry(event.target.value)} required /></label><Button size="sm" type="submit" disabled={!reason || !expiry || override.isPending}>Apply override</Button></form> : null}</section>;
}

function RegressionSection({ runID }: { runID: string }) {
  const api = useAdminApi();
  const regressions = useQuery({ queryKey: ['admin-evaluation-regressions', runID], queryFn: () => api.listEvaluationRunRegressions(runID), enabled: Boolean(runID) });
  return <section className="evaluation-section"><SectionHeading eyebrow="Candidate compared with baseline" title="Regressions" icon={<TrendingDown size={18} />} />{!runID ? <p className="admin-muted">Select a Run to inspect comparable regressions.</p> : null}{regressions.isLoading ? <p className="admin-muted">Comparing evidence.</p> : null}{regressions.data ? <><p className="admin-muted">{regressions.data.comparison_state}</p>{regressions.data.items.map((item) => <div className="evaluation-row" key={`${item.kind}-${item.case_id ?? item.message}`}><strong>{item.severity}</strong><span>{item.message}</span></div>)}</> : null}</section>;
}

export function EvaluationDetailPage() {
  const { suiteId = '' } = useParams();
  const api = useAdminApi();
  const [caseInput, setCaseInput] = useState(DEFAULT_CASE_INPUT);
  const [revisionHash, setRevisionHash] = useState('');
  const [environmentDigest, setEnvironmentDigest] = useState(DEFAULT_ENVIRONMENT_DIGEST);
  const [runtimeDigest, setRuntimeDigest] = useState(DEFAULT_RUNTIME_DIGEST);
  const [selectedRunID, setSelectedRunID] = useState('');
  const suite = useQuery({ queryKey: ['admin-evaluation-suite', suiteId], queryFn: () => api.getEvaluationSuite(suiteId), enabled: suiteId !== '' });
  const validate = useMutation({ mutationFn: () => api.validateEvaluationSuite(suiteId) });
  if (suite.isLoading) return <LoadingState label="Loading evaluation suite" />;
  if (suite.error || !suite.data) return <main className="admin-page"><ErrorState message="This Evaluation Suite is unavailable in your scope." /></main>;
  const item = suite.data;

  return <main className="admin-page evaluation-page"><Link className="admin-back-link" to="/evaluations"><ArrowLeft size={15} /> Evaluations</Link><header className="admin-detail-heading"><div><div className="admin-detail-title"><ClipboardCheck size={22} /><h1>{item.name}</h1></div><p>{item.state} · {item.id}</p></div><Button size="sm" variant="secondary" onClick={() => validate.mutate()} isLoading={validate.isPending}><RefreshCw size={15} /> Validate</Button></header><div className="evaluation-layout"><CaseSection suiteId={suiteId} suiteState={item.state} input={caseInput} onInputChange={setCaseInput} /><VersionSection suite={item} runtimeDigest={runtimeDigest} onRuntimeDigestChange={setRuntimeDigest} /><RunSection suite={item} revisionHash={revisionHash} environmentDigest={environmentDigest} onRevisionHashChange={setRevisionHash} onEnvironmentDigestChange={setEnvironmentDigest} onSelectRun={setSelectedRunID} /><GateSection suite={item} agentRevisionHash={revisionHash} /><RegressionSection runID={selectedRunID} /></div></main>;
}
