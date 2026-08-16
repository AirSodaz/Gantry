import { Routes, Route } from 'react-router-dom';
import { Bot, LogIn } from 'lucide-react';
import { Button } from '@gantry/design-system';
import { LoadingState } from './components/AsyncState';
import { useAuth } from './auth/AuthProvider';
import { AppShell } from './components/AppShell';
import { AgentsPage } from './features/catalog/AgentsPage';
import { NewTaskPage } from './features/tasks/NewTaskPage';
import { MyTasksPage } from './features/tasks/MyTasksPage';
import { TaskPage } from './features/tasks/TaskPage';
import { ApprovalsPage } from './features/approvals/ApprovalsPage';
import { ApprovalDetailPage } from './features/approvals/ApprovalDetailPage';
import { ArtifactDetailPage } from './features/artifacts/ArtifactDetailPage';
import { ArtifactsPage } from './features/artifacts/ArtifactsPage';

function App() {
  const { user, isLoading, error, signIn } = useAuth();
  if (isLoading) return <LoadingState label="Preparing your workspace" />;
  if (!user) return <SignInScreen error={error} onSignIn={() => void signIn()} />;
  return (
    <Routes>
      <Route element={<AppShell />}>
        <Route index element={<NewTaskPage />} />
        <Route path="agents" element={<AgentsPage />} />
        <Route path="tasks" element={<MyTasksPage />} />
        <Route path="tasks/:taskId" element={<TaskPage />} />
        <Route path="approvals" element={<ApprovalsPage />} />
        <Route path="approvals/:approvalId" element={<ApprovalDetailPage />} />
        <Route path="artifacts" element={<ArtifactsPage />} />
        <Route path="artifacts/:artifactId" element={<ArtifactDetailPage />} />
        <Route path="*" element={<NewTaskPage />} />
      </Route>
    </Routes>
  );
}

function SignInScreen({ error, onSignIn }: { error: Error | null; onSignIn: () => void }) {
  return (
    <main className="sign-in-screen">
      <section className="sign-in-panel">
        <div className="brand-lockup">
          <div className="brand-mark" aria-hidden="true">
            <Bot size={22} strokeWidth={2.2} />
          </div>
          <span style={{ fontSize: '16px', fontWeight: 700, letterSpacing: '-0.01em' }}>
            Gantry Copilot
          </span>
        </div>
        <h1>Your work, in motion.</h1>
        <p>Sign in to discover approved capabilities and follow your task runs in real time.</p>
        {error ? (
          <p className="inline-error" role="alert" style={{ marginBottom: '20px' }}>
            {error.message}
          </p>
        ) : null}
        <Button size="lg" fullWidth onClick={onSignIn}>
          <LogIn size={17} /> Sign in to continue
        </Button>
      </section>
    </main>
  );
}

export default App;
