import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Save, ShieldCheck } from 'lucide-react';
import { Button } from '@gantry/design-system';
import { useAdminApi } from '../../api/ApiProvider';
import { ErrorState, LoadingState } from '../../components/AsyncState';

type Scope = 'organization' | 'workspace';

export function PlatformSettingsPage() {
  const api = useAdminApi();
  const qc = useQueryClient();
  const [scope, setScope] = useState<Scope>('organization');
  const [workspaceID, setWorkspaceID] = useState('');
  const [draft, setDraft] = useState('{}');
  const activeWorkspace = scope === 'workspace' ? workspaceID.trim() : undefined;
  const settings = useQuery({ queryKey: ['admin-platform-settings', scope, activeWorkspace], queryFn: () => api.getPlatformSettings(scope, activeWorkspace), enabled: scope === 'organization' || Boolean(activeWorkspace) });
  const values = useMemo(() => settings.data?.values ?? {}, [settings.data?.values]);
  const validate = useMutation({ mutationFn: () => api.validatePlatformSettings({ workspace_id: activeWorkspace, values: JSON.parse(draft) as Record<string, unknown> }) });
  const apply = useMutation({ mutationFn: () => api.applyPlatformSettings(settings.data?.etag ?? '', { workspace_id: activeWorkspace, values: JSON.parse(draft) as Record<string, unknown> }, crypto.randomUUID()), onSuccess: () => { void qc.invalidateQueries({ queryKey: ['admin-platform-settings'] }); } });

  if (settings.isLoading) return <LoadingState label="Loading platform settings" />;
  if (settings.error) return <ErrorState message="Platform settings could not be loaded." />;

  const limitPolicies = Array.isArray(values.limit_policies) ? values.limit_policies as Array<Record<string, unknown>> : [];
  const environments = Array.isArray(values.environment_profiles) ? values.environment_profiles as Array<Record<string, unknown>> : [];
  const classifications = Array.isArray(values.data_classifications) ? values.data_classifications as Array<Record<string, unknown>> : [];

  return <div className="admin-page">
    <div className="admin-page-header"><div><h1>Platform settings</h1><p>Scope-aware defaults, bounded workspace overrides, and environment posture.</p></div><ShieldCheck size={22} aria-hidden="true" /></div>
    <section className="admin-detail-block">
      <div className="admin-form-row"><label className="admin-field"><span className="admin-field-label">Scope</span><select className="ds-input" value={scope} onChange={(event) => setScope(event.target.value as Scope)}><option value="organization">Organization</option><option value="workspace">Workspace</option></select></label>{scope === 'workspace' ? <label className="admin-field"><span className="admin-field-label">Workspace ID</span><input className="ds-input" value={workspaceID} onChange={(event) => setWorkspaceID(event.target.value)} placeholder="wsp_development" /></label> : null}</div>
      <div className="admin-form-row"><span>ETag <strong>{settings.data?.etag}</strong></span><span>Validation <strong>{settings.data?.validation_state}</strong></span><span>Scope <strong>{scope === 'organization' ? 'Organization' : workspaceID}</strong></span></div>
    </section>
    <section className="admin-detail-block"><h2>Limit policies</h2>{limitPolicies.length ? <table className="admin-table"><thead><tr><th>Scope</th><th>Concurrency</th><th>Duration</th><th>Output bytes</th><th>ETag</th></tr></thead><tbody>{limitPolicies.map((item) => <tr key={String(item.id)}><td>{item.workspace_id ? `Workspace ${String(item.workspace_id)}` : 'Organization bound'}</td><td>{String(item.concurrency)}</td><td>{String(item.duration_seconds)}s</td><td>{String(item.output_bytes)}</td><td>{String(item.etag)}</td></tr>)}</tbody></table> : <p className="admin-muted">No limit policies have been defined.</p>}</section>
    <section className="admin-detail-block"><h2>Environment profiles</h2>{environments.length ? <table className="admin-table"><thead><tr><th>Environment</th><th>Posture</th><th>State</th><th>Scope</th><th>ETag</th></tr></thead><tbody>{environments.map((item) => <tr key={String(item.id)}><td>{String(item.name)}</td><td>{String(item.publication_posture)}</td><td>{String(item.state)}</td><td>{item.workspace_id ? String(item.workspace_id) : 'Organization bound'}</td><td>{String(item.etag)}</td></tr>)}</tbody></table> : <p className="admin-muted">No environment profiles have been defined.</p>}</section>
    <section className="admin-detail-block"><h2>Data classifications</h2>{classifications.length ? <ul className="admin-detail-list">{classifications.map((item) => <li key={String(item.id)}><strong>{String(item.label)}</strong><span>{String(item.handling)} · retention {String(item.retention_class)}</span></li>)}</ul> : <p className="admin-muted">No data classifications have been defined.</p>}</section>
    <section className="admin-detail-block"><h2>Settings command</h2><label className="admin-field"><span className="admin-field-label">Section values</span><textarea className="ds-input admin-code-input" rows={6} value={draft} onChange={(event) => setDraft(event.target.value)} /></label><div className="admin-form-row"><Button variant="secondary" disabled={validate.isPending} onClick={() => { try { validate.mutate(); } catch { /* JSON errors are surfaced by the request state. */ } }}><ShieldCheck size={15} /> Validate</Button><Button disabled={apply.isPending || !settings.data?.etag} onClick={() => { try { apply.mutate(); } catch { /* JSON errors are surfaced by the request state. */ } }}><Save size={15} /> Apply</Button>{validate.data ? <span className="admin-muted">{validate.data.state}</span> : null}</div></section>
  </div>;
}
