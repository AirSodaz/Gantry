import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Cable, Database, Package, Plus } from 'lucide-react';
import { Button, Select, type SelectOption, StatusMark } from '@gantry/design-system';
import { useAdminApi } from '../../api/ApiProvider';
import type { Plugin, Skill, Tool } from '../../api/types';
import { ErrorState, LoadingState } from '../../components/AsyncState';

type AssetKind = 'skills' | 'plugins' | 'tools';
type AssetItem = Skill | Plugin | Tool;

const headings: Record<AssetKind, { title: string; description: string; icon: typeof Package }> = {
  skills: { title: 'Skills', description: 'Imported workspace artifacts pinned by Agent revisions.', icon: Package },
  plugins: { title: 'Plugins', description: 'Organization package versions available for workspace enablement.', icon: Database },
  tools: { title: 'Tools', description: 'Registered descriptor versions available to governed bindings.', icon: Cable },
};

export function SkillsPage() { return <AssetCatalog kind="skills" />; }
export function PluginsPage() { return <AssetCatalog kind="plugins" />; }
export function ToolsPage() { return <AssetCatalog kind="tools" />; }

function AssetCatalog({ kind }: { kind: AssetKind }) {
  const api = useAdminApi();
  const queryClient = useQueryClient();
  const [workspaceID, setWorkspaceID] = useState('');
  const [isAdding, setIsAdding] = useState(false);
  const [form, setForm] = useState<Record<string, string>>({});
  const meta = headings[kind];

  const workspaces = useQuery({ queryKey: ['admin-workspaces'], queryFn: () => api.listWorkspaces(), enabled: kind !== 'tools' });
  const query = useQuery<{ items: AssetItem[] }>({
    queryKey: ['admin-assets', kind, workspaceID],
    queryFn: async () => {
      if (kind === 'skills') return api.listSkills(workspaceID) as Promise<{ items: AssetItem[] }>;
      if (kind === 'plugins') return api.listPlugins() as Promise<{ items: AssetItem[] }>;
      return api.listTools() as Promise<{ items: AssetItem[] }>;
    },
  });
  const mutation = useMutation<AssetItem, Error, void>({
    mutationFn: async () => {
      if (kind === 'skills') return api.registerSkill({ workspace_id: form.workspace_id ?? workspaceID, slug: form.slug ?? '', display_name: form.display_name ?? '', description: form.description ?? '', source_type: (form.source_type ?? 'locator') as Skill['source_type'], source_ref: form.source_ref ?? '', declared_version: form.declared_version ?? '', content_digest: form.content_digest ?? '' });
      if (kind === 'plugins') {
        const plugin = await api.registerPlugin({ slug: form.slug ?? '', display_name: form.display_name ?? '', description: form.description ?? '', version: form.version ?? '', content_digest: form.content_digest ?? '' });
        await api.enablePlugin(plugin.id, form.workspace_id ?? '');
        return plugin;
      }
      return api.registerTool({ server_name: form.server_name ?? '', server_type: form.server_type ?? 'builtin', endpoint_ref: form.endpoint_ref ?? '', fully_qualified_name: form.fully_qualified_name ?? '', version: form.version ?? '', effect: form.effect ?? 'read', idempotency: form.idempotency ?? 'read_only', content_digest: form.content_digest ?? '' });
    },
    onSuccess: () => { setForm({}); setIsAdding(false); void queryClient.invalidateQueries({ queryKey: ['admin-assets', kind] }); },
  });

  const workspaceOptions = useMemo<SelectOption[]>(() => (workspaces.data?.items ?? []).map((workspace) => ({ value: workspace.id, label: workspace.display_name })), [workspaces.data?.items]);
  const items = query.data?.items ?? [];
  const Icon = meta.icon;

  if (query.isLoading || (kind !== 'tools' && workspaces.isLoading)) return <LoadingState label={`Loading ${meta.title.toLowerCase()}`} />;
  if (query.error || (kind !== 'tools' && workspaces.error)) return <div className="admin-page"><ErrorState message={`The ${meta.title.toLowerCase()} catalog could not be loaded.`} /></div>;

  return (
    <section className="admin-page">
      <header className="admin-page-heading">
        <div><h1>{meta.title}</h1><p>{meta.description}</p></div>
        <Button onClick={() => setIsAdding((value) => !value)}><Plus size={16} /> {isAdding ? 'Close' : `Register ${kind === 'skills' ? 'skill' : kind === 'plugins' ? 'plugin' : 'tool'}`}</Button>
      </header>

      {kind === 'skills' ? <div className="admin-filter-bar"><Select label="Workspace" options={workspaceOptions} value={workspaceID} onChange={setWorkspaceID} placeholder="All manageable workspaces" /></div> : null}

      {isAdding ? <AssetForm kind={kind} form={form} setForm={setForm} workspaceOptions={workspaceOptions} onSubmit={() => mutation.mutate()} busy={mutation.isPending} error={mutation.error?.message} /> : null}
      {items.length === 0 ? <div className="admin-empty"><Icon size={22} /><strong>No {kind} registered</strong><span>Register an immutable catalog entry to use it in an Agent draft.</span></div> : (
        <div className="admin-agent-list">
          {items.map((item) => {
            const name = 'display_name' in item ? item.display_name : item.fully_qualified_name;
            const version = 'version' in item ? item.version : 'declared_version' in item ? (item.declared_version || '未声明') : '';
            return <div className="admin-agent-row" key={item.id}><span className="admin-agent-icon"><Icon size={17} /></span><span className="admin-agent-copy"><strong>{name}</strong><span>{version} · {item.content_digest.slice(0, 18)}...</span></span><StatusMark status={item.status} /></div>;
          })}
        </div>
      )}
    </section>
  );
}

function AssetForm({ kind, form, setForm, workspaceOptions, onSubmit, busy, error }: { kind: AssetKind; form: Record<string, string>; setForm: (value: Record<string, string>) => void; workspaceOptions: SelectOption[]; onSubmit: () => void; busy: boolean; error?: string }) {
  const update = (key: string, value: string) => setForm({ ...form, [key]: value });
  const fields = kind === 'skills' ? [['slug', 'Slug'], ['display_name', 'Display name'], ['description', 'Description'], ['source_ref', 'Source reference'], ['declared_version', 'Declared version'], ['content_digest', 'Content digest']] : kind === 'plugins' ? [['slug', 'Slug'], ['display_name', 'Display name'], ['description', 'Description'], ['version', 'Version'], ['content_digest', 'Content digest']] : [['server_name', 'Server name'], ['fully_qualified_name', 'Fully qualified tool name'], ['version', 'Version'], ['endpoint_ref', 'Endpoint reference'], ['content_digest', 'Content digest']];
  return <form className="admin-asset-form" onSubmit={(event) => { event.preventDefault(); onSubmit(); }}><div className="admin-form-grid">{kind === 'skills' || kind === 'plugins' ? <label className="admin-field"><span className="admin-field-label">{kind === 'plugins' ? 'Enable workspace' : 'Workspace'}</span><select className="ds-input" value={form.workspace_id ?? ''} onChange={(event) => update('workspace_id', event.target.value)} required><option value="">Choose a workspace</option>{workspaceOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}</select></label> : null}{fields.map(([key, label]) => <label className="admin-field" key={key}><span className="admin-field-label">{label}</span><input className="ds-input" value={form[key] ?? ''} onChange={(event) => update(key, event.target.value)} required={key !== 'description' && key !== 'endpoint_ref' && key !== 'declared_version'} /></label>)}{kind === 'skills' ? <label className="admin-field"><span className="admin-field-label">Source type</span><select className="ds-input" value={form.source_type ?? 'locator'} onChange={(event) => update('source_type', event.target.value)}><option value="marketplace">Marketplace</option><option value="locator">Locator</option><option value="upload">Upload</option><option value="local">Local</option></select></label> : null}{kind === 'tools' ? <><label className="admin-field"><span className="admin-field-label">Server type</span><select className="ds-input" value={form.server_type ?? 'builtin'} onChange={(event) => update('server_type', event.target.value)}><option value="builtin">Built-in</option><option value="mcp">MCP</option><option value="cli">CLI</option></select></label><label className="admin-field"><span className="admin-field-label">Effect</span><select className="ds-input" value={form.effect ?? 'read'} onChange={(event) => update('effect', event.target.value)}><option value="read">Read</option><option value="write">Write</option><option value="external_side_effect">External side effect</option><option value="administrative">Administrative</option></select></label><label className="admin-field"><span className="admin-field-label">Idempotency</span><select className="ds-input" value={form.idempotency ?? 'read_only'} onChange={(event) => update('idempotency', event.target.value)}><option value="read_only">Read only</option><option value="idempotent">Idempotent</option><option value="compensatable">Compensatable</option><option value="non_repeatable">Non-repeatable</option></select></label></> : null}</div>{error ? <p className="admin-error" role="alert">{error}</p> : null}<Button type="submit" disabled={busy}>{busy ? 'Registering…' : 'Register asset'}</Button></form>;
}
