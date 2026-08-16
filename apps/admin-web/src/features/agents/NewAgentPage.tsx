import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, Layers, Plus } from 'lucide-react';
import { Button, Select, type SelectOption, TextInput } from '@gantry/design-system';
import { Link, useNavigate } from 'react-router-dom';
import { useAdminApi } from '../../api/ApiProvider';
import { ErrorState, LoadingState } from '../../components/AsyncState';

export function NewAgentPage() {
  const api = useAdminApi();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const workspaces = useQuery({
    queryKey: ['admin-workspaces'],
    queryFn: () => api.listWorkspaces(),
  });

  const [form, setForm] = useState({
    workspace_id: '',
    slug: '',
    display_name: '',
    description: '',
    category: 'Development',
  });

  const workspaceOptions = useMemo<SelectOption[]>(() => {
    return [
      { value: '', label: 'Choose a workspace…' },
      ...(workspaces.data?.items ?? []).map((w) => ({
        value: w.id,
        label: w.display_name,
        icon: <Layers size={13} />,
      })),
    ];
  }, [workspaces.data?.items]);

  const create = useMutation({
    mutationFn: () => api.createAgent(form),
    onSuccess(agent) {
      void queryClient.invalidateQueries({ queryKey: ['admin-agents'] });
      navigate(`/agents/${agent.id}/design`);
    },
  });

  if (workspaces.isLoading) return <LoadingState label="Loading workspaces" />;
  if (workspaces.error) {
    return (
      <div className="admin-page">
        <ErrorState message="Workspaces could not be loaded." />
      </div>
    );
  }

  const set = (field: keyof typeof form, value: string) =>
    setForm((current) => ({ ...current, [field]: value }));

  return (
    <section className="admin-page admin-editor-page">
      <Link className="admin-back-link" to="/">
        <ArrowLeft size={16} />
        <span>Agents</span>
      </Link>

      <header className="admin-page-heading">
        <div>
          <h1>New agent</h1>
          <p>
            Start with a valid deterministic execution draft. It remains invisible to Copilot until
            publication.
          </p>
        </div>
      </header>

      <form
        className="admin-form"
        onSubmit={(event) => {
          event.preventDefault();
          create.mutate();
        }}
      >
        <div className="admin-form-field">
          <Select
            label="Workspace"
            options={workspaceOptions}
            value={form.workspace_id}
            onChange={(val) => set('workspace_id', val)}
            placeholder="Choose a workspace"
          />
        </div>

        <TextInput
          label="Display name"
          required
          value={form.display_name}
          onChange={(event) => set('display_name', event.target.value)}
          placeholder="e.g. Code Review Assistant"
        />

        <TextInput
          label="Slug"
          required
          pattern="[a-z0-9]+(?:-[a-z0-9]+)*"
          value={form.slug}
          onChange={(event) => set('slug', event.target.value)}
          placeholder="e.g. code-review-assistant"
        />

        <TextInput
          label="Category"
          required
          value={form.category}
          onChange={(event) => set('category', event.target.value)}
          placeholder="e.g. Development, Operations, Support"
        />

        <label className="admin-field">
          <span className="admin-field-label">Description</span>
          <textarea
            required
            value={form.description}
            onChange={(event) => set('description', event.target.value)}
            placeholder="Describe what this agent does and its expected behavior."
            className="ds-input admin-textarea"
            rows={3}
          />
        </label>

        {create.error ? (
          <p className="inline-error" role="alert">
            {create.error instanceof Error ? create.error.message : 'Failed to create agent draft.'}
          </p>
        ) : null}

        <div className="admin-form-actions">
          <Button
            type="submit"
            disabled={!form.workspace_id || !form.display_name.trim() || create.isPending}
          >
            <Plus size={16} />
            <span>{create.isPending ? 'Creating…' : 'Create draft'}</span>
          </Button>
          <Link className="ds-button ds-button-quiet" to="/">
            Cancel
          </Link>
        </div>
      </form>
    </section>
  );
}
