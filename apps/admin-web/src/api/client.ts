import type { AdminOverview, Agent, AgentOverview, AgentReview, AgentVersion, AssetUsage, CreateAgentInput, Draft, Plugin, PluginDetail, Skill, Tool, Workspace } from './types';

const baseUrl = import.meta.env.VITE_ADMIN_API_BASE ?? '/api/admin/v1';

export class AdminApiError extends Error {
  constructor(public readonly status: number, message: string) {
    super(message);
    this.name = 'AdminApiError';
  }
}

type TokenProvider = () => string | null;
export type AssetListOptions = { workspaceId?: string; search?: string; status?: string };

export class AdminApi {
  constructor(private readonly tokenProvider: TokenProvider) {}

  listWorkspaces() { return this.request<{ items: Workspace[] }>('/workspaces'); }
  getOverview(workspaceId = '') {
    const query = workspaceId ? `?workspace_id=${encodeURIComponent(workspaceId)}` : '';
    return this.request<AdminOverview>(`/overview${query}`);
  }
  listSkills(options: AssetListOptions = {}) {
    return this.request<{ items: Skill[] }>(`/skills${this.assetListQuery(options)}`);
  }
  getSkill(skillId: string) { return this.request<Skill>(`/skills/${encodeURIComponent(skillId)}`); }
  listSkillUsage(skillId: string) { return this.request<{ items: AssetUsage[] }>(`/skills/${encodeURIComponent(skillId)}/usage`); }
  registerSkill(input: { workspace_id: string; slug: string; display_name: string; description?: string; source_type: Skill['source_type']; source_ref: string; declared_version?: string; content_digest: string; metadata_json?: Record<string, unknown> }) {
    return this.request<Skill>('/skills', { method: 'POST', body: JSON.stringify(input) });
  }
  listPlugins(options: AssetListOptions = {}) { return this.request<{ items: Plugin[] }>(`/plugins${this.assetListQuery(options)}`); }
  getPlugin(pluginId: string) { return this.request<PluginDetail>(`/plugins/${encodeURIComponent(pluginId)}`); }
  listPluginUsage(pluginId: string) { return this.request<{ items: AssetUsage[] }>(`/plugins/${encodeURIComponent(pluginId)}/usage`); }
  registerPlugin(input: { slug: string; display_name: string; description?: string; version: string; content_digest: string; manifest_json?: Record<string, unknown> }) {
    return this.request<Plugin>('/plugins', { method: 'POST', body: JSON.stringify(input) });
  }
  enablePlugin(pluginId: string, workspaceId: string) {
    return this.request<void>(`/plugins/${encodeURIComponent(pluginId)}/enable`, { method: 'POST', body: JSON.stringify({ workspace_id: workspaceId }) });
  }
  disablePlugin(pluginId: string, workspaceId: string) {
    return this.request<void>(`/plugins/${encodeURIComponent(pluginId)}/disable`, { method: 'POST', body: JSON.stringify({ workspace_id: workspaceId }) });
  }
  activateSkill(skillId: string, reason = '') { return this.assetStatus(`/skills/${encodeURIComponent(skillId)}:activate`, reason); }
  deprecateSkill(skillId: string, reason = '') { return this.assetStatus(`/skills/${encodeURIComponent(skillId)}:deprecate`, reason); }
  retireSkill(skillId: string, reason = '') { return this.assetStatus(`/skills/${encodeURIComponent(skillId)}:retire`, reason); }
  activatePlugin(pluginId: string, reason = '') { return this.assetStatus(`/plugins/${encodeURIComponent(pluginId)}:activate`, reason); }
  deprecatePlugin(pluginId: string, reason = '') { return this.assetStatus(`/plugins/${encodeURIComponent(pluginId)}:deprecate`, reason); }
  retirePlugin(pluginId: string, reason = '') { return this.assetStatus(`/plugins/${encodeURIComponent(pluginId)}:retire`, reason); }
  activateTool(toolId: string, reason = '') { return this.assetStatus(`/tools/${encodeURIComponent(toolId)}:activate`, reason); }
  deprecateTool(toolId: string, reason = '') { return this.assetStatus(`/tools/${encodeURIComponent(toolId)}:deprecate`, reason); }
  retireTool(toolId: string, reason = '') { return this.assetStatus(`/tools/${encodeURIComponent(toolId)}:retire`, reason); }
  listTools(options: AssetListOptions = {}) { return this.request<{ items: Tool[] }>(`/tools${this.assetListQuery(options)}`); }
  getTool(toolId: string) { return this.request<Tool>(`/tools/${encodeURIComponent(toolId)}`); }
  listToolUsage(toolId: string) { return this.request<{ items: AssetUsage[] }>(`/tools/${encodeURIComponent(toolId)}/usage`); }
  registerTool(input: { server_name: string; server_type: Tool['server_type']; endpoint_ref?: string; fully_qualified_name: string; version: string; effect: Tool['effect']; idempotency: Tool['idempotency']; content_digest: string; schema_json?: Record<string, unknown> }) {
    return this.request<Tool>('/tools', { method: 'POST', body: JSON.stringify(input) });
  }
  listAgents(workspaceId = '') {
    const query = workspaceId ? `?workspace_id=${encodeURIComponent(workspaceId)}` : '';
    return this.request<{ items: Agent[] }>(`/agents${query}`);
  }
  getAgent(agentId: string) { return this.request<Agent>(`/agents/${encodeURIComponent(agentId)}`); }
  getAgentOverview(agentId: string) { return this.request<AgentOverview>(`/agents/${encodeURIComponent(agentId)}/overview`); }
  createAgent(input: CreateAgentInput) { return this.request<Agent>('/agents', { method: 'POST', body: JSON.stringify(input) }); }
  getDraft(agentId: string) { return this.request<Draft>(`/agents/${encodeURIComponent(agentId)}/draft`); }
  updateDraft(agentId: string, revision: number, spec: unknown) {
    return this.request<Draft>(`/agents/${encodeURIComponent(agentId)}/draft`, {
      method: 'PUT', headers: { 'If-Match': String(revision) }, body: JSON.stringify({ spec }),
    });
  }
  listVersions(agentId: string) { return this.request<{ items: AgentVersion[] }>(`/agents/${encodeURIComponent(agentId)}/versions`); }
  getVersion(agentId: string, versionId: string) { return this.request<AgentVersion>(`/agents/${encodeURIComponent(agentId)}/versions/${encodeURIComponent(versionId)}`); }
  getReview(agentId: string) { return this.request<AgentReview>(`/agents/${encodeURIComponent(agentId)}/review`); }
  submitReview(agentId: string, revision: number, releaseNotes: string) {
    return this.request<AgentReview>(`/agents/${encodeURIComponent(agentId)}:review`, { method: 'POST', headers: { 'If-Match': String(revision) }, body: JSON.stringify({ release_notes: releaseNotes }) });
  }
  decideReview(agentId: string, decision: 'approve' | 'reject', reason: string) {
    return this.request<AgentReview>(`/agents/${encodeURIComponent(agentId)}:review-decision`, { method: 'POST', body: JSON.stringify({ decision, reason }) });
  }
  publish(agentId: string, revision: number) {
    return this.request<AgentVersion>(`/agents/${encodeURIComponent(agentId)}:publish`, { method: 'POST', headers: { 'If-Match': String(revision) }, body: '{}' });
  }
  retire(agentId: string) { return this.request<void>(`/agents/${encodeURIComponent(agentId)}:retire`, { method: 'POST', body: '{}' }); }
  rollback(agentId: string, versionId: string) { return this.request<void>(`/agents/${encodeURIComponent(agentId)}:rollback`, { method: 'POST', body: JSON.stringify({ version_id: versionId }) }); }

  private assetStatus(path: string, reason: string) {
    return this.request<void>(path, { method: 'POST', body: JSON.stringify({ reason }) });
  }

  private assetListQuery(options: AssetListOptions) {
    const query = new URLSearchParams();
    if (options.workspaceId) query.set('workspace_id', options.workspaceId);
    if (options.search) query.set('search', options.search);
    if (options.status) query.set('status', options.status);
    const value = query.toString();
    return value ? `?${value}` : '';
  }

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
