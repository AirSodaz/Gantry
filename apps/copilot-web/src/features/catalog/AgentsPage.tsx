import { useNavigate, useLocation } from 'react-router-dom';
import { ArrowRight, Bot } from 'lucide-react';
import { Button } from '@gantry/design-system';
import { AgentPicker } from './AgentPicker';
import type { Agent } from '../../api/types';

export function AgentsPage() {
  const navigate = useNavigate();
  const location = useLocation();
  const selectAgent = (agent: Agent) => navigate(`/?${new URLSearchParams({ ...Object.fromEntries(new URLSearchParams(location.search)), agent: agent.id }).toString()}`);
  return (
    <div className="page-wrap narrow-page">
      <div className="page-heading">
        <div><span className="eyebrow">Catalog</span><h1>Agents</h1><p>Approved capabilities available to you.</p></div>
        <Button variant="secondary" onClick={() => navigate('/')}><Bot size={16} /> New task <ArrowRight size={15} /></Button>
      </div>
      <AgentPicker selectedId="" onSelect={selectAgent} />
    </div>
  );
}
