import { useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Bot, ChevronRight, Layers, Plus } from 'lucide-react';
import { Select, type SelectOption, StatusMark } from '@gantry/design-system';
import { Link, useSearchParams } from 'react-router-dom';
import { useAdminApi } from '../../api/ApiProvider';
import { ErrorState, LoadingState } from '../../components/AsyncState';

export function AgentsPage() {
  const api = useAdminApi();
  const [searchParams, setSearchParams] = useSearchParams();
  const workspaceID = searchParams.get('workspace') ?? '';

  const workspaces = useQuery({
    queryKey: ['admin-workspaces'],
    queryFn: () => api.listWorkspaces(),
  });

  const agents = useQuery({
    queryKey: ['admin-agents', workspaceID],
    queryFn: () => api.listAgents(workspaceID),
  });

  const workspaceOptions = useMemo<SelectOption[]>(() => {
    return [
      { value: '', label: 'All manageable workspaces', icon: <Layers size={13} /> },
      ...(workspaces.data?.items ?? []).map((w) => ({
        value: w.id,
        label: w.display_name,
        icon: <Layers size={13} />,
      })),
    ];
  }, [workspaces.data?.items]);

  if (workspaces.isLoading || agents.isLoading) {
    return <LoadingState label="Loading agents" />;
  }

  if (workspaces.error || agents.error) {
    return (
      <div className="admin-page">
        <ErrorState message="The agent directory could not be loaded." />
      </div>
    );
  }

  const handleWorkspaceChange = (newVal: string) => {
    setSearchParams(newVal ? { workspace: newVal } : {});
  };

  return (
    <section className="admin-page">
      <header className="admin-page-heading">
        <div>
          <h1>Agents</h1>
          <p>Create, revise, and publish immutable execution configurations.</p>
        </div>
        <Link className="ds-button ds-button-primary admin-primary-link" to="/new">
          <Plus size={16} strokeWidth={2.5} />
          <span>New agent</span>
        </Link>
      </header>

      <div className="admin-filter-bar">
        <div className="admin-filter-select-wrap">
          <Select
            label="Workspace"
            options={workspaceOptions}
            value={workspaceID}
            onChange={handleWorkspaceChange}
            placeholder="All manageable workspaces"
          />
        </div>
      </div>

      {agents.data?.items.length === 0 ? (
        <div className="admin-empty">
          <Bot size={22} />
          <strong>No agents in this scope</strong>
          <span>Create an agent to start an editable draft.</span>
        </div>
      ) : (
        <div className="admin-agent-list">
          {agents.data?.items.map((agent) => (
            <Link className="admin-agent-row" key={agent.id} to={`/agents/${agent.id}`}>
              <span className="admin-agent-icon">
                <Bot size={17} />
              </span>
              <span className="admin-agent-copy">
                <strong>{agent.display_name}</strong>
                <span>
                  {agent.category} · {agent.slug}
                </span>
              </span>
              <StatusMark status={agent.lifecycle_status} />
              <ChevronRight size={17} className="admin-agent-chevron" />
            </Link>
          ))}
        </div>
      )}
    </section>
  );
}
