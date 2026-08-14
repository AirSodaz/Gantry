import { useQuery } from '@tanstack/react-query';
import { Plus, ChevronRight, Bot } from 'lucide-react';
import { StatusMark } from '@gantry/design-system';
import { Link, useSearchParams } from 'react-router-dom';
import { useAdminApi } from '../../api/ApiProvider';
import { ErrorState, LoadingState } from '../../components/AsyncState';

export function AgentsPage() {
  const api = useAdminApi();
  const [searchParams, setSearchParams] = useSearchParams();
  const workspaceID = searchParams.get('workspace') ?? '';
  const workspaces = useQuery({ queryKey: ['admin-workspaces'], queryFn: () => api.listWorkspaces() });
  const agents = useQuery({ queryKey: ['admin-agents', workspaceID], queryFn: () => api.listAgents(workspaceID) });
  if (workspaces.isLoading || agents.isLoading) return <LoadingState label="Loading agents" />;
  if (workspaces.error || agents.error) return <div className="admin-page"><ErrorState message="The agent directory could not be loaded." /></div>;
  return (
    <section className="admin-page">
      <header className="admin-page-heading">
        <div><h1>Agents</h1><p>Create, revise, and publish immutable execution configurations.</p></div>
        <Link className="ds-button ds-button-primary admin-primary-link" to="/agents/new"><Plus size={17} />New agent</Link>
      </header>
      <label className="admin-filter"><span>Workspace</span><select value={workspaceID} onChange={(event) => setSearchParams(event.target.value ? { workspace: event.target.value } : {})}><option value="">All manageable workspaces</option>{workspaces.data?.items.map((workspace) => <option key={workspace.id} value={workspace.id}>{workspace.display_name}</option>)}</select></label>
      {agents.data?.items.length === 0 ? <div className="admin-empty"><Bot size={20} /><strong>No agents in this scope</strong><span>Create an agent to start an editable draft.</span></div> : <div className="admin-agent-list">
        {agents.data?.items.map((agent) => <Link className="admin-agent-row" key={agent.id} to={`/agents/${agent.id}`}><span className="admin-agent-icon"><Bot size={17} /></span><span className="admin-agent-copy"><strong>{agent.display_name}</strong><span>{agent.category} · {agent.slug}</span></span><StatusMark status={agent.lifecycle_status} /><ChevronRight size={17} /></Link>)}
      </div>}
    </section>
  );
}
