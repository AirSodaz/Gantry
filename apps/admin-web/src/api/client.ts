import type { AdminAuditEvent, AdminAuditEventDetail, AdminAuditExport, AdminAuditExportDownload, AdminOverview, AdminRun, AdminRunDetail, Agent, AgentDeployment, AgentLifecycleOverview, AgentRevision, AgentRevisionReview, AssetUsage, CreateAgentInput, CreatePolicyInput, EvaluationCase, EvaluationRun, EvaluationSuite, EvaluationSuiteVersion, NamedAgentDraft, Plugin, PluginDetail, Policy, PolicyBinding, PolicyDraft, PolicySimulation, PolicyVersion, Skill, Tool, Workspace, Integration, IntegrationClient, AgentPublication, WebhookEndpoint, WebhookDelivery, ModelProvider, ProviderRoute, RunnerPool, Runner, CredentialReference, DataClassification } from './types';

const baseUrl = import.meta.env.VITE_ADMIN_API_BASE ?? '/api/admin/v1';

export class AdminApiError extends Error {
  constructor(public readonly status: number, message: string) {
    super(message);
    this.name = 'AdminApiError';
  }
}

type TokenProvider = () => string | null;
export type AssetListOptions = { workspaceId?: string; search?: string; status?: string };
export type RunListOptions = { workspaceId?: string; agentId?: string; revisionHash?: string; status?: AdminRun['status']; limit?: number };
export type AuditListOptions = { workspaceId?: string; resourceType?: string; resourceId?: string; actorId?: string; eventType?: string; outcome?: string; risk?: string; correlationId?: string; runId?: string; revisionHash?: string; policyVersionId?: string; before?: string; after?: string; cursor?: string; limit?: number };
export type PolicyListOptions = { type?: string; workspaceId?: string; state?: Policy['state']; ownerId?: string; bindingTarget?: string; cursor?: string; limit?: number };
export type EvaluationSuiteListOptions = { workspaceId?: string; state?: EvaluationSuite['state']; search?: string; limit?: number };
export type IntegrationListOptions = { state?: Integration['state'] };

export class AdminApi {
  constructor(private readonly tokenProvider: TokenProvider) {}

  listWorkspaces() { return this.request<{ items: Workspace[] }>('/workspaces'); }
  getOverview(workspaceId = '') {
    const query = workspaceId ? `?workspace_id=${encodeURIComponent(workspaceId)}` : '';
    return this.request<AdminOverview>(`/overview${query}`);
  }
  listRuns(options: RunListOptions = {}) {
    const query = new URLSearchParams();
    if (options.workspaceId) query.set('workspace_id', options.workspaceId);
    if (options.agentId) query.set('agent_id', options.agentId);
    if (options.revisionHash) query.set('revision_hash', options.revisionHash);
    if (options.status) query.set('status', options.status);
    if (options.limit) query.set('limit', String(options.limit));
    const value = query.toString();
    return this.request<{ items: AdminRun[] }>(`/runs${value ? `?${value}` : ''}`);
  }
  getRun(runId: string) { return this.request<AdminRunDetail>(`/runs/${encodeURIComponent(runId)}`); }
  listAuditEvents(options: AuditListOptions = {}) {
    const query = new URLSearchParams();
    const entries: [string, string | undefined][] = [
      ['workspace_id', options.workspaceId], ['resource_type', options.resourceType], ['resource_id', options.resourceId], ['actor_id', options.actorId],
      ['event_type', options.eventType], ['outcome', options.outcome], ['risk', options.risk], ['correlation_id', options.correlationId], ['run_id', options.runId],
      ['revision_hash', options.revisionHash], ['policy_version_id', options.policyVersionId], ['before', options.before], ['after', options.after], ['cursor', options.cursor],
    ];
    for (const [key, value] of entries) if (value) query.set(key, value);
    if (options.limit) query.set('limit', String(options.limit));
    const value = query.toString();
    return this.request<{ items: AdminAuditEvent[]; page_info: { has_more: boolean; next_cursor?: string } }>(`/audit-events${value ? `?${value}` : ''}`);
  }
  getAuditEvent(eventId: string) { return this.request<AdminAuditEventDetail>(`/audit-events/${encodeURIComponent(eventId)}`); }
  createAuditExport(options: Omit<AuditListOptions, 'cursor' | 'limit'> = {}) {
    return this.request<AdminAuditExport>('/audit-events:export', { method: 'POST', body: JSON.stringify({
      workspace_id: options.workspaceId, resource_type: options.resourceType, resource_id: options.resourceId, actor_id: options.actorId,
      event_type: options.eventType, outcome: options.outcome, risk: options.risk, correlation_id: options.correlationId, run_id: options.runId,
      revision_hash: options.revisionHash, policy_version_id: options.policyVersionId, before: options.before, after: options.after,
    }) });
  }
  getAuditExport(exportId: string) { return this.request<AdminAuditExport>(`/audit-exports/${encodeURIComponent(exportId)}`); }
  downloadAuditExport(exportId: string) { return this.request<AdminAuditExportDownload>(`/audit-exports/${encodeURIComponent(exportId)}/download`); }
  listPolicies(options: PolicyListOptions = {}) {
    const query = new URLSearchParams();
    const entries: [string, string | undefined][] = [['type', options.type], ['workspace_id', options.workspaceId], ['state', options.state], ['owner_id', options.ownerId], ['binding_target', options.bindingTarget], ['cursor', options.cursor]];
    for (const [key, value] of entries) if (value) query.set(key, value);
    if (options.limit) query.set('limit', String(options.limit));
    const value = query.toString();
    return this.request<{ items: Policy[]; page_info: { next_cursor: string | null } }>(`/policies${value ? `?${value}` : ''}`);
  }
  createPolicy(input: CreatePolicyInput) { return this.request<{ policy: Policy; draft: PolicyDraft }>('/policies', { method: 'POST', body: JSON.stringify(input) }); }
  getPolicy(policyId: string) { return this.request<Policy>(`/policies/${encodeURIComponent(policyId)}`); }
  getPolicyDraft(policyId: string) { return this.request<PolicyDraft>(`/policies/${encodeURIComponent(policyId)}/draft`); }
  updatePolicyDraft(policyId: string, etag: string, input: { document: Record<string, unknown>; schema_version?: string }) { return this.request<PolicyDraft>(`/policies/${encodeURIComponent(policyId)}/draft`, { method: 'PATCH', headers: { 'If-Match': `"${etag.replace(/^"|"$/g, '')}"` }, body: JSON.stringify(input) }); }
  validatePolicy(policyId: string) { return this.request<PolicyDraft>(`/policies/${encodeURIComponent(policyId)}:validate`, { method: 'POST', body: '{}' }); }
  listPolicyVersions(policyId: string) { return this.request<{ items: PolicyVersion[] }>(`/policies/${encodeURIComponent(policyId)}/versions`); }
  publishPolicyVersion(policyId: string, etag: string, message: string, idempotencyKey: string) { return this.request<PolicyVersion>(`/policies/${encodeURIComponent(policyId)}/versions`, { method: 'POST', headers: { 'If-Match': `"${etag.replace(/^"|"$/g, '')}"`, 'Idempotency-Key': idempotencyKey }, body: JSON.stringify({ message }) }); }
  listPolicyBindings(policyId: string) { return this.request<{ items: PolicyBinding[] }>(`/policies/${encodeURIComponent(policyId)}/bindings`); }
  bindPolicy(policyId: string, input: { version_id: string; scope: 'organization' | 'workspace'; workspace_id?: string; target_resource_id?: string; environment: 'development' | 'staging' | 'production'; reason?: string }, idempotencyKey: string) { return this.request<PolicyBinding>(`/policies/${encodeURIComponent(policyId)}/bindings`, { method: 'POST', headers: { 'Idempotency-Key': idempotencyKey }, body: JSON.stringify(input) }); }
  revokePolicyBinding(bindingId: string, reason: string, idempotencyKey: string) { return this.request<PolicyBinding>(`/policy-bindings/${encodeURIComponent(bindingId)}:revoke`, { method: 'POST', headers: { 'Idempotency-Key': idempotencyKey }, body: JSON.stringify({ reason }) }); }
  simulatePolicy(policyId: string, input: { version_id?: string; action?: Record<string, unknown> }) { return this.request<PolicySimulation>(`/policies/${encodeURIComponent(policyId)}:simulate`, { method: 'POST', body: JSON.stringify(input) }); }
  retirePolicy(policyId: string, reason: string, idempotencyKey: string) { return this.request<Policy>(`/policies/${encodeURIComponent(policyId)}:retire`, { method: 'POST', headers: { 'Idempotency-Key': idempotencyKey }, body: JSON.stringify({ reason }) }); }
  listEvaluationSuites(options: EvaluationSuiteListOptions = {}) { const query = new URLSearchParams(); if (options.workspaceId) query.set('workspace_id', options.workspaceId); if (options.state) query.set('state', options.state); if (options.search) query.set('search', options.search); if (options.limit) query.set('limit', String(options.limit)); const value = query.toString(); return this.request<{ items: EvaluationSuite[]; page_info: { next_cursor: string | null } }>(`/evaluation-suites${value ? `?${value}` : ''}`); }
  createEvaluationSuite(input: { workspace_id: string; name: string }) { return this.request<EvaluationSuite>('/evaluation-suites', { method: 'POST', body: JSON.stringify(input) }); }
  getEvaluationSuite(suiteId: string) { return this.request<EvaluationSuite>(`/evaluation-suites/${encodeURIComponent(suiteId)}`); }
  patchEvaluationSuite(suiteId: string, etag: string, name: string) { return this.request<EvaluationSuite>(`/evaluation-suites/${encodeURIComponent(suiteId)}`, { method: 'PATCH', headers: { 'If-Match': `"${etag.replace(/^"|"$/g, '')}"` }, body: JSON.stringify({ name }) }); }
  validateEvaluationSuite(suiteId: string) { return this.request<{ state: string; findings: Record<string, unknown>[] }>(`/evaluation-suites/${encodeURIComponent(suiteId)}:validate`, { method: 'POST', body: '{}' }); }
  listEvaluationCases(suiteId: string) { return this.request<{ items: EvaluationCase[] }>(`/evaluation-suites/${encodeURIComponent(suiteId)}/cases`); }
  createEvaluationCase(suiteId: string, input: { input: Record<string, unknown>; fixture_manifest: Record<string, unknown>; assertions: Record<string, unknown>[]; rubric?: Record<string, unknown>; compatibility?: Record<string, unknown> }) { return this.request<EvaluationCase>(`/evaluation-suites/${encodeURIComponent(suiteId)}/cases`, { method: 'POST', body: JSON.stringify(input) }); }
  listEvaluationVersions(suiteId: string) { return this.request<{ items: EvaluationSuiteVersion[] }>(`/evaluation-suites/${encodeURIComponent(suiteId)}/versions`); }
  publishEvaluationVersion(suiteId: string, etag: string, input: { runtime_image_digest: string; evaluator_policy_version_id?: string }, idempotencyKey: string) { return this.request<EvaluationSuiteVersion>(`/evaluation-suites/${encodeURIComponent(suiteId)}/versions`, { method: 'POST', headers: { 'If-Match': `"${etag.replace(/^"|"$/g, '')}"`, 'Idempotency-Key': idempotencyKey }, body: JSON.stringify(input) }); }
  listEvaluationRuns(suiteId: string) { return this.request<{ items: EvaluationRun[] }>(`/evaluation-suites/${encodeURIComponent(suiteId)}/runs`); }
  createEvaluationRun(suiteId: string, input: { suite_version_id: string; candidate_revision_hash: string; baseline_revision_hash?: string; environment_digest: string }, idempotencyKey: string) { return this.request<EvaluationRun>(`/evaluation-suites/${encodeURIComponent(suiteId)}/runs`, { method: 'POST', headers: { 'Idempotency-Key': idempotencyKey }, body: JSON.stringify(input) }); }
  getEvaluationRun(runId: string) { return this.request<EvaluationRun>(`/evaluation-runs/${encodeURIComponent(runId)}`); }
  cancelEvaluationRun(runId: string) { return this.request<EvaluationRun>(`/evaluation-runs/${encodeURIComponent(runId)}:cancel`, { method: 'POST', body: '{}' }); }
  listIntegrations(options: IntegrationListOptions = {}) { const query = options.state ? `?state=${encodeURIComponent(options.state)}` : ''; return this.request<{ items: Integration[]; page_info: { next_cursor: string | null } }>(`/integrations${query}`); }
  createIntegration(input: { slug: string; display_name: string }) { return this.request<Integration>('/integrations', { method: 'POST', body: JSON.stringify(input) }); }
  getIntegration(id: string) { return this.request<Integration>(`/integrations/${encodeURIComponent(id)}`); }
  patchIntegration(id: string, displayName: string) { return this.request<Integration>(`/integrations/${encodeURIComponent(id)}`, { method: 'PATCH', body: JSON.stringify({ display_name: displayName }) }); }
  listIntegrationClients(id: string) { return this.request<{ items: IntegrationClient[] }>(`/integrations/${encodeURIComponent(id)}/clients`); }
  createIntegrationClient(id: string, input: { environment: IntegrationClient['environment']; auth_modes: string[]; audience?: string; credential_fingerprint: string; expires_at?: string }) { return this.request<IntegrationClient>(`/integrations/${encodeURIComponent(id)}/clients`, { method: 'POST', body: JSON.stringify(input) }); }
  rotateIntegrationClient(id: string, credentialFingerprint: string) { return this.request<IntegrationClient>(`/integration-clients/${encodeURIComponent(id)}:rotate`, { method: 'POST', body: JSON.stringify({ credential_fingerprint: credentialFingerprint }) }); }
  disableIntegrationClient(id: string) { return this.request<void>(`/integration-clients/${encodeURIComponent(id)}:disable`, { method: 'POST', body: '{}' }); }
  listIntegrationPublications(id: string) { return this.request<{ items: AgentPublication[] }>(`/integrations/${encodeURIComponent(id)}/publications`); }
  createIntegrationPublication(id: string, input: Omit<AgentPublication, 'id' | 'integration_id' | 'state'>) { return this.request<AgentPublication>(`/integrations/${encodeURIComponent(id)}/publications`, { method: 'POST', body: JSON.stringify(input) }); }
  revokeIntegrationPublication(id: string) { return this.request<void>(`/integration-publications/${encodeURIComponent(id)}:revoke`, { method: 'POST', body: '{}' }); }
  listIntegrationWebhooks(id: string) { return this.request<{ items: WebhookEndpoint[] }>(`/integrations/${encodeURIComponent(id)}/webhooks`); }
  createIntegrationWebhook(id: string, input: { environment: WebhookEndpoint['environment']; destination: string; signing_key_fingerprint: string; subscribed_events: string[]; retry_policy?: Record<string, unknown> }) { return this.request<WebhookEndpoint>(`/integrations/${encodeURIComponent(id)}/webhooks`, { method: 'POST', body: JSON.stringify(input) }); }
  redeliverWebhook(id: string, deliveryId: string) { return this.request<WebhookDelivery>(`/webhook-endpoints/${encodeURIComponent(id)}:redeliver`, { method: 'POST', body: JSON.stringify({ delivery_id: deliveryId }) }); }
  listPlatformProviders() { return this.request<{ items: ModelProvider[] }>('/platform/model-providers'); }
  createPlatformProvider(input: { name: string; data_classes: string[]; credential_reference_id: string }) { return this.request<ModelProvider>('/platform/model-providers', { method: 'POST', body: JSON.stringify(input) }); }
  listProviderRoutes(providerId: string) { return this.request<{ items: ProviderRoute[] }>(`/platform/model-providers/${encodeURIComponent(providerId)}/routes`); }
  putProviderRoute(providerId: string, routeId: string, etag: string, input: { allowed_models: string[]; fallback_route_ids: string[]; state: ProviderRoute['state']; budget_policy_id?: string; classification_constraints?: Record<string, unknown> }) { return this.request<ProviderRoute>(`/platform/model-providers/${encodeURIComponent(providerId)}/routes/${encodeURIComponent(routeId)}`, { method: 'PUT', headers: { 'If-Match': `"${etag.replace(/^"|"$/g, '')}"` }, body: JSON.stringify(input) }); }
  quarantineProvider(providerId: string) { return this.request<ModelProvider>(`/platform/model-providers/${encodeURIComponent(providerId)}:quarantine`, { method: 'POST', body: '{}' }); }
  listRunnerPools() { return this.request<{ items: RunnerPool[] }>('/platform/runner-pools'); }
  createRunnerPool(input: { isolation_tier: RunnerPool['isolation_tier']; compatible_protocols: string[]; capacity: Record<string, unknown> }) { return this.request<RunnerPool>('/platform/runner-pools', { method: 'POST', body: JSON.stringify(input) }); }
  listRunners(poolId: string) { return this.request<{ items: Runner[] }>(`/platform/runner-pools/${encodeURIComponent(poolId)}/runners`); }
  drainRunnerPool(poolId: string) { return this.request<RunnerPool>(`/platform/runner-pools/${encodeURIComponent(poolId)}:drain`, { method: 'POST', body: '{}' }); }
  quarantineRunnerPool(poolId: string) { return this.request<RunnerPool>(`/platform/runner-pools/${encodeURIComponent(poolId)}:quarantine`, { method: 'POST', body: '{}' }); }
  listPlatformCredentials() { return this.request<{ items: CredentialReference[] }>('/platform/credentials'); }
  rotatePlatformCredential(id: string) { return this.request<CredentialReference>(`/platform/credentials/${encodeURIComponent(id)}:rotate`, { method: 'POST', body: '{}' }); }
  revokePlatformCredential(id: string) { return this.request<CredentialReference>(`/platform/credentials/${encodeURIComponent(id)}:revoke`, { method: 'POST', body: '{}' }); }
  listDataClassifications() { return this.request<{ items: DataClassification[] }>('/platform/data-classifications'); }
  createDataClassification(input: { label: string; handling: DataClassification['handling']; retention_class: string; allowed_provider_ids: string[]; allowed_tool_classes: string[] }) { return this.request<DataClassification>('/platform/data-classifications', { method: 'POST', body: JSON.stringify(input) }); }
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
  listAgents(workspaceId = '', search = '', status = '') {
    const params = new URLSearchParams();
    if (workspaceId) params.set('workspace_id', workspaceId);
    if (search) params.set('search', search);
    if (status) params.set('status', status);
    const query = params.toString() ? `?${params.toString()}` : '';
    return this.request<{ items: Agent[] }>(`/agents${query}`);
  }
  getAgent(agentId: string) { return this.request<Agent>(`/agents/${encodeURIComponent(agentId)}`); }
  getAgentLifecycle(agentId: string) { return this.request<AgentLifecycleOverview>(`/agents/${encodeURIComponent(agentId)}/lifecycle`); }
  listDrafts(agentId: string) { return this.request<{ items: NamedAgentDraft[] }>(`/agents/${encodeURIComponent(agentId)}/drafts`); }
  getNamedDraft(agentId: string, draftId: string) { return this.request<NamedAgentDraft>(`/agents/${encodeURIComponent(agentId)}/drafts/${encodeURIComponent(draftId)}`); }
  createDraft(agentId: string, input: { name: string; from_revision_hash?: string }) { return this.request<NamedAgentDraft>(`/agents/${encodeURIComponent(agentId)}/drafts`, { method: 'POST', body: JSON.stringify(input) }); }
  updateNamedDraft(agentId: string, draftId: string, etag: number, spec: unknown) { return this.request<NamedAgentDraft>(`/agents/${encodeURIComponent(agentId)}/drafts/${encodeURIComponent(draftId)}`, { method: 'PUT', headers: { 'If-Match': String(etag) }, body: JSON.stringify({ spec }) }); }
  archiveDraft(agentId: string, draftId: string) { return this.request<void>(`/agents/${encodeURIComponent(agentId)}/drafts/${encodeURIComponent(draftId)}:archive`, { method: 'POST', body: '{}' }); }
  commitDraft(agentId: string, draftId: string, message: string) { return this.request<AgentRevision>(`/agents/${encodeURIComponent(agentId)}/drafts/${encodeURIComponent(draftId)}:commit`, { method: 'POST', body: JSON.stringify({ message }) }); }
  listRevisions(agentId: string) { return this.request<{ items: AgentRevision[] }>(`/agents/${encodeURIComponent(agentId)}/revisions`); }
  getRevision(agentId: string, revisionHash: string) { return this.request<AgentRevision>(`/agents/${encodeURIComponent(agentId)}/revisions/${encodeURIComponent(revisionHash)}`); }
  getRevisionReview(agentId: string, revisionHash: string) { return this.request<AgentRevisionReview>(`/agents/${encodeURIComponent(agentId)}/revisions/${encodeURIComponent(revisionHash)}/review`); }
  submitRevisionReview(agentId: string, revisionHash: string, releaseNotes: string) { return this.request<AgentRevisionReview>(`/agents/${encodeURIComponent(agentId)}/revisions/${encodeURIComponent(revisionHash)}/review`, { method: 'POST', body: JSON.stringify({ release_notes: releaseNotes }) }); }
  decideRevisionReview(agentId: string, revisionHash: string, decision: 'approve' | 'reject', reason: string) { return this.request<AgentRevisionReview>(`/agents/${encodeURIComponent(agentId)}/revisions/${encodeURIComponent(revisionHash)}:review-decision`, { method: 'POST', body: JSON.stringify({ decision, reason }) }); }
  publishRevision(agentId: string, revisionHash: string, expectedProductionRevisionHash = '') { return this.request<AgentDeployment>(`/agents/${encodeURIComponent(agentId)}/revisions/${encodeURIComponent(revisionHash)}:publish`, { method: 'POST', body: JSON.stringify({ expected_production_revision_hash: expectedProductionRevisionHash }) }); }
  listDeployments(agentId: string) { return this.request<{ items: AgentDeployment[] }>(`/agents/${encodeURIComponent(agentId)}/deployments`); }
  createTestDeployment(agentId: string, input: { name: string; revision_hash: string; purpose?: string; expires_at?: string; environment_policy?: Record<string, unknown> }) { return this.request<AgentDeployment>(`/agents/${encodeURIComponent(agentId)}/deployments`, { method: 'POST', body: JSON.stringify(input) }); }
  stopTestDeployment(agentId: string, deploymentId: string) { return this.request<void>(`/agents/${encodeURIComponent(agentId)}/deployments/${encodeURIComponent(deploymentId)}:stop`, { method: 'POST', body: '{}' }); }
  createAgent(input: CreateAgentInput) { return this.request<Agent>('/agents', { method: 'POST', body: JSON.stringify(input) }); }

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
