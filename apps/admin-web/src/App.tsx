import { Route, Routes } from 'react-router-dom';
import { Bot, LogIn } from 'lucide-react';
import { Button } from '@gantry/design-system';
import { useAuth } from './auth/AuthProvider';
import { AppShell } from './components/AppShell';
import { LoadingState } from './components/AsyncState';
import { AgentsPage } from './features/agents/AgentsPage';
import { AgentDetailPage } from './features/agents/AgentDetailPage';
import { NewAgentPage } from './features/agents/NewAgentPage';
import { PluginsPage, SkillsPage, ToolsPage } from './features/assets/AssetPages';

export default function App() {
  const { user, isLoading, error, signIn } = useAuth();
  if (isLoading) return <LoadingState label="Preparing Admin" />;
  if (!user) {
    return (
      <main className="admin-sign-in">
        <section>
          <div className="admin-brand">
            <span className="admin-brand-mark" aria-hidden="true">
              <Bot size={22} strokeWidth={2.2} />
            </span>
            <span style={{ fontSize: '16px', fontWeight: 700, letterSpacing: '-0.01em' }}>
              Gantry Admin
            </span>
          </div>
          <h1>Control the agent lifecycle.</h1>
          <p>Sign in to manage draft execution configurations and their immutable published versions.</p>
          {error ? (
            <p className="admin-error" role="alert" style={{ marginBottom: '20px' }}>
              {error.message}
            </p>
          ) : null}
          <Button size="lg" fullWidth onClick={() => void signIn()}>
            <LogIn size={17} /> Sign in to continue
          </Button>
        </section>
      </main>
    );
  }

  return (
    <Routes>
      <Route element={<AppShell />}>
        <Route index element={<AgentsPage />} />
        <Route path="new" element={<NewAgentPage />} />
        <Route path="agents/new" element={<NewAgentPage />} />
        <Route path="agents/:agentId" element={<AgentDetailPage />} />
        <Route path="skills" element={<SkillsPage />} />
        <Route path="plugins" element={<PluginsPage />} />
        <Route path="tools" element={<ToolsPage />} />
        <Route path="*" element={<AgentsPage />} />
      </Route>
    </Routes>
  );
}
