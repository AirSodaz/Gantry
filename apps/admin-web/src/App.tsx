import { Route, Routes } from 'react-router-dom';
import { Bot, LogIn } from 'lucide-react';
import { Button } from '@gantry/design-system';
import { useAuth } from './auth/AuthProvider';
import { AppShell } from './components/AppShell';
import { LoadingState } from './components/AsyncState';
import { AgentsPage } from './features/agents/AgentsPage';
import { AgentDetailPage } from './features/agents/AgentDetailPage';
import { NewAgentPage } from './features/agents/NewAgentPage';
import { AgentOverviewPage } from './features/agents/AgentOverviewPage';
import { AgentVersionPage } from './features/agents/AgentVersionPage';
import { AgentVersionsPage } from './features/agents/AgentVersionsPage';
import { AssetDetailPage, PluginsPage, SkillsPage, ToolsPage } from './features/assets/AssetPages';
import { OverviewPage } from './features/overview/OverviewPage';
import { RunDetailPage, RunsPage } from './features/runs/RunPages';

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
          <p>Sign in to manage named Drafts, immutable Revisions, and exact Deployments.</p>
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
        <Route index element={<OverviewPage />} />
        <Route path="new" element={<NewAgentPage />} />
        <Route path="agents" element={<AgentsPage />} />
        <Route path="agents/new" element={<NewAgentPage />} />
        <Route path="agents/:agentId" element={<AgentOverviewPage />} />
        <Route path="agents/:agentId/design" element={<AgentDetailPage />} />
        <Route path="agents/:agentId/versions" element={<AgentVersionsPage />} />
        <Route path="agents/:agentId/revisions/:revisionHash" element={<AgentVersionPage />} />
        <Route path="skills" element={<SkillsPage />} />
        <Route path="skills/:assetId" element={<AssetDetailPage kind="skills" />} />
        <Route path="plugins" element={<PluginsPage />} />
        <Route path="plugins/:assetId" element={<AssetDetailPage kind="plugins" />} />
        <Route path="tools" element={<ToolsPage />} />
        <Route path="tools/:assetId" element={<AssetDetailPage kind="tools" />} />
        <Route path="runs" element={<RunsPage />} />
        <Route path="runs/:runId" element={<RunDetailPage />} />
        <Route path="*" element={<OverviewPage />} />
      </Route>
    </Routes>
  );
}
