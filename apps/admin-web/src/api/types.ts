import type { AdminApiComponents } from '@gantry/api-client';

type Components = AdminApiComponents;
export type Workspace = Components['schemas']['Workspace'];
export type Agent = Components['schemas']['Agent'];
export type Draft = Components['schemas']['AgentDraft'];
export type AgentVersion = Components['schemas']['AgentVersion'];
export type AgentOverview = Components['schemas']['AgentOverview'];
export type AdminOverview = Components['schemas']['AdminOverview'];
export type ActivityItem = Components['schemas']['ActivityItem'];
export type PromptSnapshot = Components['schemas']['PromptSnapshot'];
export type CreateAgentInput = Components['schemas']['CreateAgentRequest'];
export type CreateSkillInput = Components['schemas']['RegisterSkillRequest'];
export type CreatePluginInput = Components['schemas']['RegisterPluginRequest'];
export type CreateToolInput = Components['schemas']['RegisterToolRequest'];
export type AgentReview = Components['schemas']['AgentReview'];
export type DiffEntry = Components['schemas']['DiffEntry'];
export type Skill = Components['schemas']['Skill'];
export type Plugin = Components['schemas']['Plugin'];
export type Tool = Components['schemas']['Tool'];
export type AssetUsage = Components['schemas']['AssetUsage'];
export type PluginDetail = Components['schemas']['PluginDetail'];

export type AgentSpec = {
  kind: 'gantry.agent/v1';
  system_prompt?: string;
  user_input?: string;
  model: { provider: 'scripted' | 'openai' | 'openai-compatible' | 'anthropic'; model: string };
  workspace_root: string;
  limits: { max_turns: number; max_output_bytes: number };
  checkpoint: { enabled: boolean };
  command_policy: { allow_shell: boolean };
  skills?: { artifact_id: string }[];
  plugins?: { plugin_version_id: string }[];
  tool_bindings?: { descriptor_id: string; operations?: string[] }[];
};
