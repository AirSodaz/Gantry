import type { Agent, RunStatus, SubmitTaskInput, Task } from './types';

const baseUrl = import.meta.env.VITE_COPILOT_API_BASE ?? '/api/copilot/v1';

export class CopilotApiError extends Error {
  constructor(public readonly status: number, message: string) {
    super(message);
    this.name = 'CopilotApiError';
  }
}

type TokenProvider = () => string | null;

export class CopilotApi {
  constructor(private readonly tokenProvider: TokenProvider) {}

  listAgents(search = '', category = '') {
    const params = new URLSearchParams();
    if (search) params.set('search', search);
    if (category) params.set('category', category);
    return this.request<{ items: Agent[] }>(`/agents?${params.toString()}`);
  }

  listTasks(status = '') {
    const params = new URLSearchParams();
    if (status) params.set('status', status);
    return this.request<{ items: Task[] }>(`/tasks?${params.toString()}`);
  }

  getTask(taskId: string) {
    return this.request<Task>(`/tasks/${encodeURIComponent(taskId)}`);
  }

  submitTask(input: SubmitTaskInput, idempotencyKey: string) {
    return this.request<Task>('/tasks', {
      method: 'POST',
      headers: { 'Idempotency-Key': idempotencyKey },
      body: JSON.stringify(input),
    });
  }

  cancelRun(taskId: string, runId: string) {
    return this.request<RunStatus>(`/tasks/${encodeURIComponent(taskId)}/runs/${encodeURIComponent(runId)}:cancel`, {
      method: 'POST',
      body: '{}',
    });
  }

  retryTask(taskId: string) {
    return this.request<Task>(`/tasks/${encodeURIComponent(taskId)}:retry`, {
      method: 'POST',
      body: '{}',
    });
  }

  private async request<T>(path: string, init: RequestInit = {}): Promise<T> {
    const token = this.tokenProvider();
    if (!token) throw new CopilotApiError(401, 'Your session has expired.');
    const response = await fetch(`${baseUrl}${path}`, {
      ...init,
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
        ...init.headers,
      },
    });
    if (!response.ok) {
      let message = `Request failed with status ${response.status}.`;
      try {
        const payload = await response.json() as { error?: { message?: string } };
        message = payload.error?.message ?? message;
      } catch {
        // Preserve the status-based message when the server did not return JSON.
      }
      throw new CopilotApiError(response.status, message);
    }
    return response.json() as Promise<T>;
  }
}
