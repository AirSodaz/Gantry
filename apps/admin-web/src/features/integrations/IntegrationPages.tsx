import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link, useParams, useSearchParams } from 'react-router-dom';
import { ArrowLeft, Plus, RotateCw } from 'lucide-react';
import { Button, Select, type SelectOption } from '@gantry/design-system';
import { useAdminApi } from '../../api/ApiProvider';
import { ErrorState, LoadingState } from '../../components/AsyncState';
import './IntegrationPages.css';

const INTEGRATION_STATE_OPTIONS: SelectOption[] = [
  { value: 'active', label: 'Active' },
  { value: 'disabled', label: 'Disabled' },
  { value: 'retired', label: 'Retired' },
];
const INTEGRATION_ENVIRONMENT_OPTIONS: SelectOption[] = [
  { value: 'development', label: 'Development' },
  { value: 'staging', label: 'Staging' },
  { value: 'production', label: 'Production' },
];

export function IntegrationsPage() {
  const api = useAdminApi(); const qc = useQueryClient(); const [searchParams, setSearchParams] = useSearchParams(); const [slug, setSlug] = useState(''); const [name, setName] = useState('');
  const search = searchParams.get('search') ?? ''; const state = searchParams.get('state') ?? ''; const environment = searchParams.get('environment') ?? '';
  const list = useQuery({ queryKey: ['admin-integrations', state, search, environment], queryFn: () => api.listIntegrations({ state: state as 'active' | 'disabled' | 'retired' || undefined, search: search || undefined, environment: environment as 'development' | 'staging' | 'production' || undefined }) });
  const updateFilter = (key: string, value: string) => { const next = new URLSearchParams(searchParams); if (value) next.set(key, value); else next.delete(key); setSearchParams(next); };
  const create = useMutation({ mutationFn: () => api.createIntegration({ slug, display_name: name }), onSuccess: () => { setSlug(''); setName(''); void qc.invalidateQueries({ queryKey: ['admin-integrations'] }); } });
  if (list.isLoading) return <LoadingState label="Loading integrations" />; if (list.error) return <ErrorState message="Integrations could not be loaded." />;
  return <div className="admin-page integration-page"><div className="admin-page-header"><div><h1>Integrations</h1><p>Registered enterprise clients, exact Agent publications, and webhook endpoints.</p></div></div><div className="integration-toolbar"><label className="admin-field integration-search"><span className="admin-field-label">Search</span><input className="ds-input" value={search} onChange={(e) => updateFilter('search', e.target.value)} placeholder="Name or slug" /></label><Select label="State" options={INTEGRATION_STATE_OPTIONS} value={state} onChange={(value) => updateFilter('state', value)} placeholder="All states" /><Select label="Environment" options={INTEGRATION_ENVIRONMENT_OPTIONS} value={environment} onChange={(value) => updateFilter('environment', value)} placeholder="All environments" /></div><section className="admin-detail-block"><h2>Register integration</h2><form className="admin-form-grid" onSubmit={(e) => { e.preventDefault(); create.mutate(); }}><label className="admin-field"><span className="admin-field-label">Slug</span><input className="ds-input" value={slug} onChange={(e) => setSlug(e.target.value)} required /></label><label className="admin-field"><span className="admin-field-label">Display name</span><input className="ds-input" value={name} onChange={(e) => setName(e.target.value)} required /></label><Button type="submit" disabled={create.isPending}><Plus size={15} /> Register</Button></form>{create.error ? <p className="admin-error" role="alert">{create.error.message}</p> : null}</section>{list.data?.items.length ? <div className="admin-table-wrap"><table className="admin-table"><thead><tr><th>Name</th><th>Slug</th><th>State</th><th>Environments</th></tr></thead><tbody>{list.data.items.map((item) => <tr key={item.id}><td><Link to={`/integrations/${item.id}`}>{item.display_name}</Link></td><td>{item.slug}</td><td>{item.state}</td><td>{item.environments.join(', ') || 'No clients'}</td></tr>)}</tbody></table></div> : <div className="admin-empty"><strong>No integrations</strong><span>Register an integration identity to expose enterprise metadata.</span></div>}</div>;
}

export function IntegrationDetailPage() {
  const api = useAdminApi(); const { integrationId = '' } = useParams(); const [fingerprint, setFingerprint] = useState('');
  const integration = useQuery({ queryKey: ['admin-integration', integrationId], queryFn: () => api.getIntegration(integrationId), enabled: Boolean(integrationId) });
  const clients = useQuery({ queryKey: ['admin-integration-clients', integrationId], queryFn: () => api.listIntegrationClients(integrationId), enabled: Boolean(integrationId) });
  const publications = useQuery({ queryKey: ['admin-integration-publications', integrationId], queryFn: () => api.listIntegrationPublications(integrationId), enabled: Boolean(integrationId) });
  const webhooks = useQuery({ queryKey: ['admin-integration-webhooks', integrationId], queryFn: () => api.listIntegrationWebhooks(integrationId), enabled: Boolean(integrationId) });
  const rotate = useMutation({ mutationFn: (id: string) => api.rotateIntegrationClient(id, fingerprint), onSuccess: () => { setFingerprint(''); void clients.refetch(); } });
  if (integration.isLoading || clients.isLoading || publications.isLoading || webhooks.isLoading) return <LoadingState label="Loading integration" />; if (integration.error) return <ErrorState message="The integration could not be loaded." />; const item = integration.data!;
  return <div className="admin-page"><Link className="admin-inline-link" to="/integrations"><ArrowLeft size={14} /> Integrations</Link><div className="admin-page-header"><div><h1>{item.display_name}</h1><p>{item.slug} · {item.state}</p></div></div><section className="admin-detail-block"><h2>Clients</h2>{clients.data?.items.length ? <ul className="admin-detail-list">{clients.data.items.map((client) => <li key={client.id}><div><strong>{client.environment}</strong><span>{client.status} · {client.credential_fingerprint}</span></div><div className="admin-form-row"><input className="ds-input" placeholder="new credential fingerprint" value={fingerprint} onChange={(e) => setFingerprint(e.target.value)} /><Button size="sm" variant="secondary" disabled={!fingerprint || rotate.isPending} onClick={() => rotate.mutate(client.id)}><RotateCw size={14} /> Rotate</Button></div></li>)}</ul> : <p className="admin-muted">No clients registered.</p>}</section><section className="admin-detail-block"><h2>Publications</h2>{publications.data?.items.length ? <ul className="admin-detail-list">{publications.data.items.map((pub) => <li key={pub.id}><strong>{pub.revision_hash}</strong><span>{pub.environment} · {pub.state} · {pub.workspace_id}</span></li>)}</ul> : <p className="admin-muted">No Agent publications.</p>}</section><section className="admin-detail-block"><h2>Webhooks</h2>{webhooks.data?.items.length ? <ul className="admin-detail-list">{webhooks.data.items.map((hook) => <li key={hook.id}><strong>{hook.destination}</strong><span>{hook.environment} · {hook.status}</span></li>)}</ul> : <p className="admin-muted">No webhook endpoints.</p>}</section></div>;
}
