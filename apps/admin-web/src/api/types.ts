import type { AdminApiComponents } from "@gantry/api-client";

type Components = AdminApiComponents;
export type Workspace = Components["schemas"]["Workspace"];
export type Agent = Components["schemas"]["Agent"];
export type AgentLifecycleOverview =
  Components["schemas"]["AgentLifecycleOverview"];
export type NamedAgentDraft = Components["schemas"]["NamedAgentDraft"];
export type AgentRevision = Components["schemas"]["AgentRevision"];
export type AgentRevisionReview = Components["schemas"]["AgentRevisionReview"];
export type AgentDeployment = Components["schemas"]["AgentDeployment"];
export type AdminOverview = Components["schemas"]["AdminOverview"];
export type AdminRun = Components["schemas"]["AdminRun"];
export type AdminRunDetail = Components["schemas"]["AdminRunDetail"];
export type ActivityItem = Components["schemas"]["ActivityItem"];
export type PromptSnapshot = Components["schemas"]["PromptSnapshot"];
export type CreateAgentInput = Components["schemas"]["CreateAgentRequest"];
export type CreateSkillInput = Components["schemas"]["RegisterSkillRequest"];
export type CreatePluginInput = Components["schemas"]["RegisterPluginRequest"];
export type CreateToolInput = Components["schemas"]["RegisterToolRequest"];
export type DiffEntry = Components["schemas"]["DiffEntry"];
export type Skill = Components["schemas"]["Skill"];
export type Plugin = Components["schemas"]["Plugin"];
export type Tool = Components["schemas"]["Tool"];
export type AssetUsage = Components["schemas"]["AssetUsage"];
export type PluginDetail = Components["schemas"]["PluginDetail"];
export type AdminAuditEvent = Components["schemas"]["AdminAuditEvent"];
export type AdminAuditEventDetail =
  Components["schemas"]["AdminAuditEventDetail"];
export type AdminAuditExport = Components["schemas"]["AdminAuditExport"];
export type AdminAuditExportDownload =
  Components["schemas"]["AdminAuditExportDownload"];
export type Policy = Components["schemas"]["Policy"];
export type PolicyDraft = Components["schemas"]["PolicyDraft"];
export type PolicyVersion = Components["schemas"]["PolicyVersion"];
export type PolicyBinding = Components["schemas"]["PolicyBinding"];
export type PolicySimulation = Components["schemas"]["PolicySimulation"];
export type CreatePolicyInput = Components["schemas"]["CreatePolicyRequest"];
export type EvaluationSuite = Components["schemas"]["EvaluationSuite"];
export type EvaluationCase = Components["schemas"]["EvaluationCase"];
export type EvaluationSuiteVersion =
  Components["schemas"]["EvaluationSuiteVersion"];
export type EvaluationRun = Components["schemas"]["EvaluationRun"];
export type EvaluationGate = Components["schemas"]["EvaluationGate"];
export type EvaluationRegression =
  Components["schemas"]["EvaluationRegression"];
export type EvaluationRegressionList =
  Components["schemas"]["EvaluationRegressionList"];
export type Integration = Components["schemas"]["Integration"];
export type IntegrationClient = Components["schemas"]["IntegrationClient"];
export type AgentPublication = Components["schemas"]["AgentPublication"];
export type WebhookEndpoint = Components["schemas"]["WebhookEndpoint"];
export type WebhookDelivery = Components["schemas"]["WebhookDelivery"];
export type ModelProvider = Components["schemas"]["ModelProvider"];
export type ProviderRoute = Components["schemas"]["ProviderRoute"];
export type RunnerPool = Components["schemas"]["RunnerPool"];
export type Runner = Components["schemas"]["Runner"];
export type CredentialReference = Components["schemas"]["CredentialReference"];
export type DataClassification = Components["schemas"]["DataClassification"];
export type LimitPolicy = Components["schemas"]["LimitPolicy"];
export type EnvironmentProfile = Components["schemas"]["EnvironmentProfile"];
export type PlatformSettingsProjection =
  Components["schemas"]["PlatformSettingsProjection"];
export type SettingsValidation = Components["schemas"]["SettingsValidation"];

export type AgentSpec = {
  kind: "gantry.agent/v1";
  system_prompt?: string;
  user_input?: string;
  model: {
    provider: "scripted" | "openai" | "openai-compatible" | "anthropic";
    model: string;
  };
  workspace_root: string;
  limits: { max_turns: number; max_output_bytes: number };
  checkpoint: { enabled: boolean };
  command_policy: { allow_shell: boolean };
  skills?: { artifact_id: string }[];
  plugins?: { plugin_version_id: string }[];
  tool_bindings?: { descriptor_id: string; operations?: string[] }[];
};
