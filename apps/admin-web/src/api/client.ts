import type { Agent, AgentVersion, CreateAgentInput, Draft, Workspace } from './types';

const baseUrl = import.meta.env.VITE_ADMIN_API_BASE ?? '/api/admin/v1';

export class AdminApiError extends Error {
  constructor(public readonly status: number, message: string) {
    super(message);
    this.name = 'AdminApiError';
  }
}

type TokenProvider = () => string | null;

export class AdminApi {
  constructor(private readonly tokenProvider: TokenProvider) {}

  listWorkspaces() { return this.request<{ items: Workspace[] }>('/workspaces'); }
  listAgents(workspaceId = '') {
    const query = workspaceId ? `?workspace_id=${encodeURIComponent(workspaceId)}` : '';
    return this.request<{ items: Agent[] }>(`/agents${query}`);
  }
  getAgent(agentId: string) { return this.request<Agent>(`/agents/${encodeURIComponent(agentId)}`); }
  createAgent(input: CreateAgentInput) { return this.request<Agent>('/agents', { method: 'POST', body: JSON.stringify(input) }); }
  getDraft(agentId: string) { return this.request<Draft>(`/agents/${encodeURIComponent(agentId)}/draft`); }
  updateDraft(agentId: string, revision: number, spec: unknown) {
    return this.request<Draft>(`/agents/${encodeURIComponent(agentId)}/draft`, {
      method: 'PUT', headers: { 'If-Match': String(revision) }, body: JSON.stringify({ spec }),
    });
  }
  listVersions(agentId: string) { return this.request<{ items: AgentVersion[] }>(`/agents/${encodeURIComponent(agentId)}/versions`); }
  publish(agentId: string, revision: number) {
    return this.request<AgentVersion>(`/agents/${encodeURIComponent(agentId)}:publish`, { method: 'POST', headers: { 'If-Match': String(revision) }, body: '{}' });
  }
  retire(agentId: string) { return this.request<void>(`/agents/${encodeURIComponent(agentId)}:retire`, { method: 'POST', body: '{}' }); }

  private async request<T>(path: string, init: RequestInit = {}): Promise<T> {
    const token = this.tokenProvider();
    if (!token) throw new AdminApiError(401, 'Your session has expired.');
    const response = await fetch(`${baseUrl}${path}`, {
      ...init,
      headers: { Accept: 'application/json', 'Content-Type': 'application/json', Authorization: `Bearer ${token}`, ...init.headers },
    });
    if (!response.ok) {
      let message = `Request failed with status ${response.status}.`;
      try {
        const payload = await response.json() as { error?: { message?: string } };
        message = payload.error?.message ?? message;
      } catch { /* Keep the HTTP status fallback. */ }
      throw new AdminApiError(response.status, message);
    }
    if (response.status === 204) return undefined as T;
    return response.json() as Promise<T>;
  }
}
