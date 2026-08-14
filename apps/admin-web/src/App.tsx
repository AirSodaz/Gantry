import { Route, Routes } from 'react-router-dom';
import { Button } from '@gantry/design-system';
import { useAuth } from './auth/AuthProvider';
import { AppShell } from './components/AppShell';
import { LoadingState } from './components/AsyncState';
import { AgentsPage } from './features/agents/AgentsPage';
import { AgentDetailPage } from './features/agents/AgentDetailPage';
import { NewAgentPage } from './features/agents/NewAgentPage';

export default function App() {
  const { user, isLoading, error, signIn } = useAuth();
  if (isLoading) return <LoadingState label="Preparing Admin" />;
  if (!user) return <main className="admin-sign-in"><section><div className="admin-brand"><span className="admin-brand-mark">G</span><span>Gantry Admin</span></div><h1>Control the agent lifecycle.</h1><p>Sign in to manage draft execution configurations and their immutable published versions.</p>{error ? <p className="admin-error" role="alert">{error.message}</p> : null}<Button onClick={() => void signIn()}>Sign in to continue</Button></section></main>;
  return <Routes><Route element={<AppShell />}><Route index element={<AgentsPage />} /><Route path="agents/new" element={<NewAgentPage />} /><Route path="agents/:agentId" element={<AgentDetailPage />} /><Route path="*" element={<AgentsPage />} /></Route></Routes>;
}
