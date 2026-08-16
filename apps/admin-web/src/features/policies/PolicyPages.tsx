import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, Check, FileKey2, GitBranch, Plus, RotateCcw, Shield, SlidersHorizontal } from 'lucide-react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { Button, Select, type SelectOption } from '@gantry/design-system';
import { useAdminApi } from '../../api/ApiProvider';
import type { CreatePolicyInput, Policy, PolicyDraft } from '../../api/types';
import { ErrorState, LoadingState } from '../../components/AsyncState';

const types: SelectOption[] = [
  { value: '', label: 'All policy types' },
  ...['approval', 'model', 'tool', 'command', 'network', 'credential', 'data', 'budget', 'retention', 'evaluation', 'publication'].map((value) => ({ value, label: value })),
];

export function PoliciesPage() {
  const api = useAdminApi();
  const navigate = useNavigate();
  const [type, setType] = useState('');
  const [showCreate, setShowCreate] = useState(false);
  const policies = useQuery({ queryKey: ['admin-policies', type], queryFn: () => api.listPolicies({ type }) });
  const create = useMutation<{ policy: Policy; draft: PolicyDraft }, Error, CreatePolicyInput>({ mutationFn: (input) => api.createPolicy(input), onSuccess: (item) => { void policies.refetch(); navigate(`/policies/${item.policy.id}`); } });
  const [name, setName] = useState('');
  const [createType, setCreateType] = useState('approval');
  const [document, setDocument] = useState('{"kind":"approval","rules":[],"default_effect":"deny"}');
  if (policies.isLoading) return <LoadingState label="Loading policies" />;
  if (policies.error) return <div className="admin-page"><ErrorState message="The policy catalog could not be loaded." /></div>;
  return <section className="admin-page">
    <header className="admin-page-heading"><div><h1>Policies</h1><p>Typed governance documents, immutable Versions, and exact Bindings.</p></div><Button size="sm" onClick={() => setShowCreate((value) => !value)}><Plus size={15} /> New policy</Button></header>
    {showCreate ? <form className="admin-form-panel" onSubmit={(event) => { event.preventDefault(); let parsed: Record<string, unknown>; try { parsed = JSON.parse(document) as Record<string, unknown>; } catch { return; } create.mutate({ type: createType as Policy['type'], name, document: parsed }); }}><div className="admin-form-grid"><label className="admin-filter-input"><span className="admin-field-label">Name</span><input className="ds-input" value={name} onChange={(event) => setName(event.target.value)} required /></label><Select label="Type" options={types.slice(1)} value={createType} onChange={setCreateType} /><label className="admin-filter-input admin-form-wide"><span className="admin-field-label">Typed document (JSON)</span><textarea className="ds-input admin-code-input" value={document} onChange={(event) => setDocument(event.target.value)} rows={5} required /></label></div><div className="admin-form-actions"><Button type="submit" isLoading={create.isPending}><Check size={15} /> Create Draft</Button>{create.error ? <span className="admin-error" role="alert">The policy could not be created.</span> : null}</div></form> : null}
    <div className="admin-filter-bar"><Select label="Type" options={types} value={type} onChange={setType} /></div>
    {(policies.data?.items ?? []).length === 0 ? <div className="admin-empty"><Shield size={22} /><strong>No policies in this scope</strong><span>Create a typed Draft to begin governance.</span></div> : <div className="admin-table-wrap"><table className="admin-table"><thead><tr><th>Name</th><th>Type</th><th>Scope</th><th>State</th><th>Bindings</th><th>Draft ETag</th></tr></thead><tbody>{policies.data?.items.map((policy) => <tr key={policy.id}><td><Link className="admin-inline-link" to={`/policies/${policy.id}`}>{policy.name}</Link><br /><span className="admin-muted">{policy.id}</span></td><td>{policy.type}</td><td>{policy.workspace_id ?? 'Organization'}</td><td>{policy.state}</td><td>{policy.active_binding_count}</td><td>{policy.draft_etag}</td></tr>)}</tbody></table></div>}
  </section>;
}

export function PolicyDetailPage() {
  const { policyId = '' } = useParams();
  const api = useAdminApi();
  const queryClient = useQueryClient();
  const policy = useQuery({ queryKey: ['admin-policy', policyId], queryFn: () => api.getPolicy(policyId), enabled: policyId !== '' });
  const draft = useQuery({ queryKey: ['admin-policy-draft', policyId], queryFn: () => api.getPolicyDraft(policyId), enabled: policyId !== '' });
  const versions = useQuery({ queryKey: ['admin-policy-versions', policyId], queryFn: () => api.listPolicyVersions(policyId), enabled: policyId !== '' });
  const bindings = useQuery({ queryKey: ['admin-policy-bindings', policyId], queryFn: () => api.listPolicyBindings(policyId), enabled: policyId !== '' });
  const [draftText, setDraftText] = useState('');
  const [message, setMessage] = useState('');
  const [reason, setReason] = useState('');
  const currentDraft: PolicyDraft | undefined = draft.data;
  const draftDocument = useMemo(() => currentDraft?.document ?? {}, [currentDraft?.document]);
  const text = draftText || JSON.stringify(draftDocument, null, 2);
  const update = useMutation({ mutationFn: () => api.updatePolicyDraft(policyId, draft.data?.etag ?? '', { document: JSON.parse(text) as Record<string, unknown> }), onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ['admin-policy-draft', policyId] }); void queryClient.invalidateQueries({ queryKey: ['admin-policy', policyId] }); setDraftText(''); } });
  const validate = useMutation({ mutationFn: () => api.validatePolicy(policyId), onSuccess: (next) => { queryClient.setQueryData(['admin-policy-draft', policyId], next); } });
  const publish = useMutation({ mutationFn: () => api.publishPolicyVersion(policyId, draft.data?.etag ?? '', message, `policy-publish-${policyId}-${draft.data?.etag ?? ''}`), onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ['admin-policy-versions', policyId] }); void queryClient.invalidateQueries({ queryKey: ['admin-policy', policyId] }); } });
  const simulate = useMutation({ mutationFn: () => api.simulatePolicy(policyId, {} ) });
  const retire = useMutation({ mutationFn: () => api.retirePolicy(policyId, reason, `policy-retire-${policyId}`), onSuccess: () => { void queryClient.invalidateQueries({ queryKey: ['admin-policy', policyId] }); } });
  if (policy.isLoading || draft.isLoading || versions.isLoading || bindings.isLoading) return <LoadingState label="Loading policy" />;
  if (policy.error || draft.error || !policy.data || !draft.data) return <div className="admin-page"><ErrorState message="This policy is unavailable in your administrative scope." /></div>;
  const item = policy.data;
  return <section className="admin-page"><Link className="admin-back-link" to="/policies"><ArrowLeft size={15} /> Policies</Link><header className="admin-detail-heading"><div><div className="admin-detail-title"><Shield size={22} /><h1>{item.name}</h1></div><p>{item.type} · {item.state} · {item.id}</p></div><div className="admin-detail-actions"><Button size="sm" variant="secondary" onClick={() => validate.mutate()} isLoading={validate.isPending}><RotateCcw size={15} /> Validate</Button><Button size="sm" variant="secondary" onClick={() => simulate.mutate()} isLoading={simulate.isPending}><SlidersHorizontal size={15} /> Simulate</Button></div></header>
    <div className="admin-detail-grid"><section className="admin-detail-block"><div className="admin-section-heading"><div><span>Mutable working copy</span><h2>Draft</h2></div><FileKey2 size={18} /></div><p className="admin-muted">ETag {draft.data.etag} · validation {draft.data.validation.state}</p><textarea className="ds-input admin-code-input" value={text} onChange={(event) => setDraftText(event.target.value)} rows={15} /><div className="admin-form-actions"><Button size="sm" onClick={() => update.mutate()} isLoading={update.isPending}><Check size={15} /> Save Draft</Button><input className="ds-input" value={message} onChange={(event) => setMessage(event.target.value)} placeholder="Version message" /><Button size="sm" variant="secondary" onClick={() => publish.mutate()} isLoading={publish.isPending} disabled={!message || draft.data.validation.state !== 'valid'}><GitBranch size={15} /> Publish Version</Button></div>{update.error || publish.error ? <p className="admin-error" role="alert">The Draft command could not be completed.</p> : null}</section><section className="admin-detail-block"><div className="admin-section-heading"><div><span>Immutable history</span><h2>Versions</h2></div><GitBranch size={18} /></div>{versions.data?.items.length ? <ul className="admin-detail-list">{versions.data.items.map((version) => <li key={version.id}><strong>{version.content_digest.slice(0, 18)}</strong><span>{version.message}</span></li>)}</ul> : <p className="admin-muted">No published Versions.</p>}<div className="admin-section-heading admin-section-heading-spaced"><div><span>Exact target projections</span><h2>Bindings</h2></div></div>{bindings.data?.items.length ? <ul className="admin-detail-list">{bindings.data.items.map((binding) => <li key={binding.id}><strong>{binding.state} · {binding.environment}</strong><span>{binding.id} · {binding.version_id}</span></li>)}</ul> : <p className="admin-muted">No Bindings.</p>}<label className="admin-filter-input admin-form-spaced"><span className="admin-field-label">Retire reason</span><input className="ds-input" value={reason} onChange={(event) => setReason(event.target.value)} /></label><Button size="sm" variant="quiet" onClick={() => retire.mutate()} isLoading={retire.isPending} disabled={item.state === 'retired'}>Retire policy</Button></section></div>{simulate.data ? <section className="admin-detail-block"><div className="admin-section-heading"><div><span>Side-effect-free result</span><h2>Simulation</h2></div><SlidersHorizontal size={18} /></div><p><strong>{simulate.data.decision}</strong> · {simulate.data.explanation}</p></section> : null}</section>;
}
