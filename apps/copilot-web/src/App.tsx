import { Routes, Route } from 'react-router-dom';
import { Button } from '@gantry/design-system';
import { LoadingState } from './components/AsyncState';
import { useAuth } from './auth/AuthProvider';
import { AppShell } from './components/AppShell';
import { AgentsPage } from './features/catalog/AgentsPage';
import { NewTaskPage } from './features/tasks/NewTaskPage';
import { MyTasksPage } from './features/tasks/MyTasksPage';
import { TaskPage } from './features/tasks/TaskPage';
import { ApprovalsPage } from './features/approvals/ApprovalsPage';

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
        <Route path="*" element={<NewTaskPage />} />
      </Route>
    </Routes>
  );
}

function SignInScreen({ error, onSignIn }: { error: Error | null; onSignIn: () => void }) {
  return (
    <main className="sign-in-screen">
      <section className="sign-in-panel">
        <div className="brand-lockup"><div className="brand-mark" aria-hidden="true">G</div><span>Gantry Copilot</span></div>
        <h1>Your work, in motion.</h1>
        <p>Sign in to discover approved capabilities and follow your task runs.</p>
        {error ? <p className="inline-error" role="alert">{error.message}</p> : null}
        <Button onClick={onSignIn}>Sign in to continue</Button>
      </section>
    </main>
  );
}

export default App;
