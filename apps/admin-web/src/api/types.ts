import type { AdminApiComponents } from '@gantry/api-client';

type Components = AdminApiComponents;
export type Workspace = Components['schemas']['Workspace'];
export type Agent = Components['schemas']['Agent'];
export type Draft = Components['schemas']['AgentDraft'];
export type AgentVersion = Components['schemas']['AgentVersion'];
export type CreateAgentInput = Components['schemas']['CreateAgentRequest'];
export type AgentReview = Components['schemas']['AgentReview'];
export type DiffEntry = Components['schemas']['DiffEntry'];

export type AgentSpec = {
  kind: 'gantry.agent/v1';
  model: { provider: 'scripted'; model: string };
  workspace_root: string;
  limits: { max_turns: number; max_output_bytes: number };
  checkpoint: { enabled: boolean };
  command_policy: { allow_shell: boolean };
};
