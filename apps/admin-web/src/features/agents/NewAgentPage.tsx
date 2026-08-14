import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft } from 'lucide-react';
import { Button, TextInput } from '@gantry/design-system';
import { Link, useNavigate } from 'react-router-dom';
import { useState } from 'react';
import { useAdminApi } from '../../api/ApiProvider';
import { ErrorState, LoadingState } from '../../components/AsyncState';

export function NewAgentPage() {
  const api = useAdminApi();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const workspaces = useQuery({ queryKey: ['admin-workspaces'], queryFn: () => api.listWorkspaces() });
  const [form, setForm] = useState({ workspace_id: '', slug: '', display_name: '', description: '', category: 'Development' });
  const create = useMutation({
    mutationFn: () => api.createAgent(form),
    onSuccess(agent) { void queryClient.invalidateQueries({ queryKey: ['admin-agents'] }); navigate(`/agents/${agent.id}`); },
  });
  if (workspaces.isLoading) return <LoadingState label="Loading workspaces" />;
  if (workspaces.error) return <div className="admin-page"><ErrorState message="Workspaces could not be loaded." /></div>;
  const set = (field: keyof typeof form, value: string) => setForm((current) => ({ ...current, [field]: value }));
  return <section className="admin-page admin-editor-page">
    <Link className="admin-back-link" to="/"><ArrowLeft size={16} />Agents</Link>
    <header className="admin-page-heading"><div><h1>New agent</h1><p>Start with a valid deterministic execution draft. It remains invisible to Copilot until publication.</p></div></header>
    <form className="admin-form" onSubmit={(event) => { event.preventDefault(); create.mutate(); }}>
      <label className="admin-field"><span>Workspace</span><select required value={form.workspace_id} onChange={(event) => set('workspace_id', event.target.value)}><option value="">Choose a workspace</option>{workspaces.data?.items.map((workspace) => <option key={workspace.id} value={workspace.id}>{workspace.display_name}</option>)}</select></label>
      <TextInput label="Display name" required value={form.display_name} onChange={(event) => set('display_name', event.target.value)} />
      <TextInput label="Slug" required pattern="[a-z0-9]+(?:-[a-z0-9]+)*" value={form.slug} onChange={(event) => set('slug', event.target.value)} />
      <TextInput label="Category" required value={form.category} onChange={(event) => set('category', event.target.value)} />
      <label className="admin-field"><span>Description</span><textarea required value={form.description} onChange={(event) => set('description', event.target.value)} /></label>
      {create.error ? <ErrorState message={create.error.message} /> : null}
      <div className="admin-form-actions"><Button type="submit" disabled={create.isPending}>{create.isPending ? 'Creating...' : 'Create draft'}</Button><Link className="ds-button ds-button-quiet" to="/">Cancel</Link></div>
    </form>
  </section>;
}
