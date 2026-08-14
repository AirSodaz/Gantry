import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { TaskPage } from './TaskPage';

const mocked = vi.hoisted(() => ({
  api: {
    getTask: vi.fn(),
    cancelRun: vi.fn(),
    retryTask: vi.fn(),
  },
}));
vi.mock('../../api/ApiProvider', () => ({ useCopilotApi: () => mocked.api }));

const baseTask = {
  id: 'tsk_1',
  agent_id: 'agt_1',
  agent_display_name: 'Lifecycle Demo',
  created_at: '2026-08-14T08:00:00Z',
  current_run: { id: 'run_1', status: 'running' },
};

function renderTask() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={['/tasks/tsk_1']}>
        <Routes><Route path="/tasks/:taskId" element={<TaskPage />} /></Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('TaskPage', () => {
  beforeEach(() => vi.clearAllMocks());

  it('shows cancellation for an active run and sends the fenced run identity', async () => {
    mocked.api.getTask.mockResolvedValue({ ...baseTask, status: 'running' });
    mocked.api.cancelRun.mockResolvedValue({ id: 'run_1', status: 'canceling' });
    const user = userEvent.setup();
    renderTask();

    const cancel = await screen.findByRole('button', { name: /Cancel run/ });
    await user.click(cancel);

    await waitFor(() => expect(mocked.api.cancelRun).toHaveBeenCalledWith('tsk_1', 'run_1'));
  });

  it('polls while the task remains active', async () => {
    mocked.api.getTask.mockResolvedValue({ ...baseTask, status: 'running' });
    renderTask();

    await screen.findByText('Task tsk_1');
    await waitFor(() => expect(mocked.api.getTask.mock.calls.length).toBeGreaterThan(1), { timeout: 2_500 });
  });

  it('shows retry only for failed tasks and starts a new task run', async () => {
    mocked.api.getTask.mockResolvedValue({ ...baseTask, status: 'failed', current_run: { id: 'run_1', status: 'failed', status_reason: 'Runner disconnected.' } });
    mocked.api.retryTask.mockResolvedValue({ ...baseTask, status: 'queued', current_run: { id: 'run_2', status: 'assigned' } });
    const user = userEvent.setup();
    renderTask();

    expect(await screen.findByText('Runner disconnected.')).toBeInTheDocument();
    const retry = screen.getByRole('button', { name: /Retry task/ });
    await user.click(retry);

    await waitFor(() => expect(mocked.api.retryTask).toHaveBeenCalledWith('tsk_1'));
  });
});
