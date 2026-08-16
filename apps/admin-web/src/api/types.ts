import type { AdminApiComponents } from '@gantry/api-client';

type Components = AdminApiComponents;
export type Workspace = Components['schemas']['Workspace'];
export type Agent = Components['schemas']['Agent'];
export type Draft = Components['schemas']['AgentDraft'];
export type AgentVersion = Components['schemas']['AgentVersion'];
export type CreateAgentInput = Components['schemas']['CreateAgentRequest'];
export type AgentReview = Components['schemas']['AgentReview'];
export type DiffEntry = Components['schemas']['DiffEntry'];
export type Skill = Components['schemas']['Skill'];
export type Plugin = Components['schemas']['Plugin'];
export type Tool = Components['schemas']['Tool'];
export type AssetUsage = Components['schemas']['AssetUsage'];
export type PluginDetail = Components['schemas']['PluginDetail'];

export type AgentSpec = {
  kind: 'gantry.agent/v1';
  model: { provider: 'scripted'; model: string };
  workspace_root: string;
  limits: { max_turns: number; max_output_bytes: number };
  checkpoint: { enabled: boolean };
  command_policy: { allow_shell: boolean };
  skills?: { artifact_id: string }[];
  plugins?: { plugin_version_id: string }[];
  tool_bindings?: { descriptor_id: string; operations?: string[] }[];
};
