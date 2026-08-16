import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { TaskPage } from './TaskPage';

const mocked = vi.hoisted(() => ({
  api: {
    getTask: vi.fn(),
    createEventsTicket: vi.fn(),
    requestArtifactDownload: vi.fn(),
    cancelRun: vi.fn(),
    retryTask: vi.fn(),
    appendTaskMessage: vi.fn(),
    listTaskRuns: vi.fn(),
    listApprovals: vi.fn(),
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
  beforeEach(() => {
    vi.clearAllMocks();
    MockWebSocket.instances = [];
  });
  afterEach(() => vi.unstubAllGlobals());

  it('shows cancellation for an active run and sends the fenced run identity', async () => {
    mocked.api.getTask.mockResolvedValue({ ...baseTask, status: 'running' });
    mocked.api.createEventsTicket.mockResolvedValue({ ticket: 'evt.test', task_id: 'tsk_1', expires_at: '2026-08-14T08:01:00Z' });
    mocked.api.cancelRun.mockResolvedValue({ id: 'run_1', status: 'canceling' });
    const user = userEvent.setup();
    renderTask();

    const cancel = await screen.findByRole('button', { name: /Cancel run/ });
    await user.click(cancel);

    await waitFor(() => expect(mocked.api.cancelRun).toHaveBeenCalledWith('tsk_1', 'run_1', expect.any(String)));
  });

  it('polls while the task remains active', async () => {
    mocked.api.getTask.mockResolvedValue({ ...baseTask, status: 'running' });
    mocked.api.createEventsTicket.mockResolvedValue({ ticket: 'evt.test', task_id: 'tsk_1', expires_at: '2026-08-14T08:01:00Z' });
    renderTask();

    await screen.findByText('Task tsk_1');
    await waitFor(() => expect(mocked.api.getTask.mock.calls.length).toBeGreaterThan(1), { timeout: 2_500 });
  });

  it('shows retry only for failed tasks and starts a new task run', async () => {
    mocked.api.getTask.mockResolvedValue({ ...baseTask, status: 'failed', current_run: { id: 'run_1', status: 'failed', status_reason: 'Runner disconnected.' } });
    mocked.api.createEventsTicket.mockResolvedValue({ ticket: 'evt.test', task_id: 'tsk_1', expires_at: '2026-08-14T08:01:00Z' });
    mocked.api.getTask.mockResolvedValue({ ...baseTask, status: 'failed', conversation_etag: '"1"', current_run: { id: 'run_1', status: 'failed', status_reason: 'Runner disconnected.' } });
    mocked.api.retryTask.mockResolvedValue({ ...baseTask, status: 'queued', conversation_etag: '"2"', current_run: { id: 'run_2', status: 'assigned' } });
    const user = userEvent.setup();
    renderTask();

    expect(await screen.findByText('Runner disconnected.')).toBeInTheDocument();
    const retry = screen.getByRole('button', { name: /Retry task/ });
    await user.click(retry);

    await waitFor(() => expect(mocked.api.retryTask).toHaveBeenCalledWith('tsk_1', expect.any(String), '"1"'));
  });

  it('accumulates event output and reconnects with the rendered cursor', async () => {
    mocked.api.getTask.mockResolvedValue({ ...baseTask, status: 'running' });
    mocked.api.createEventsTicket.mockResolvedValue({ ticket: 'evt.test', task_id: 'tsk_1', expires_at: '2026-08-14T08:01:00Z' });
    vi.stubGlobal('WebSocket', MockWebSocket);
    renderTask();

    await waitFor(() => expect(MockWebSocket.instances).toHaveLength(1));
    MockWebSocket.instances[0].emit({ type: 'event', cursor: 'cur_1', event: { sequence: 1, type: 'model.delta', payload: { text: 'Hello' } } });
    expect(await screen.findByText('Hello')).toBeInTheDocument();

    MockWebSocket.instances[0].close();
    await waitFor(() => expect(MockWebSocket.instances).toHaveLength(2), { timeout: 1_500 });
    expect(MockWebSocket.instances[1].url).toContain('after=cur_1');
  });

  it('shows a retry action when an artifact download fails', async () => {
    mocked.api.getTask.mockResolvedValue({ ...baseTask, status: 'completed', current_run: { id: 'run_1', status: 'completed' }, artifacts: [{ id: 'art_1', filename: 'result.txt', media_type: 'text/plain', size_bytes: 3, state: 'available', scan_status: 'passed' }] });
    mocked.api.createEventsTicket.mockResolvedValue({ ticket: 'evt.test', task_id: 'tsk_1', expires_at: '2026-08-14T08:01:00Z' });
    mocked.api.requestArtifactDownload.mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce({ download_url: 'http://example.test/result.txt' });
    const user = userEvent.setup();
    renderTask();

    await user.click(await screen.findByRole('button', { name: 'Download' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Download failed');
    expect(screen.getByRole('button', { name: 'Retry download' })).toBeInTheDocument();
  });

  it('continues an approval-rejected task through a new idempotent message', async () => {
    mocked.api.getTask.mockResolvedValue({ ...baseTask, status: 'awaiting_requester_input', conversation_etag: '"4"', current_run: { id: 'run_1', status: 'failed' }, messages: [{ id: 'msg_1', role: 'requester', content: 'Update the record', created_at: '2026-08-16T08:00:00Z' }] });
    mocked.api.createEventsTicket.mockResolvedValue({ ticket: 'evt.test', task_id: 'tsk_1', expires_at: '2026-08-14T08:01:00Z' });
    mocked.api.appendTaskMessage.mockResolvedValue({ ...baseTask, status: 'queued', current_run: { id: 'run_2', status: 'queued' } });
    const user = userEvent.setup();
    renderTask();

    await user.type(await screen.findByLabelText('Continue this task'), 'Use a different target');
    await user.click(screen.getByRole('button', { name: 'Continue' }));

    await waitFor(() => expect(mocked.api.appendTaskMessage).toHaveBeenCalledWith('tsk_1', { message: 'Use a different target' }, expect.any(String), '"4"'));
  });
});

class MockWebSocket {
  static instances: MockWebSocket[] = [];
  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onclose: ((event: Event) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;

  constructor(readonly url: string) {
    MockWebSocket.instances.push(this);
    queueMicrotask(() => this.onopen?.(new Event('open')));
  }

  close() { this.onclose?.(new Event('close')); }
  emit(frame: unknown) { this.onmessage?.(new MessageEvent('message', { data: JSON.stringify(frame) })); }
}
