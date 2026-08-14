import type { AdminApiComponents } from '@gantry/api-client';

type Components = AdminApiComponents;
export type Workspace = Components['schemas']['Workspace'];
export type Agent = Components['schemas']['Agent'];
export type Draft = Components['schemas']['AgentDraft'];
export type AgentVersion = Components['schemas']['AgentVersion'];
export type CreateAgentInput = Components['schemas']['CreateAgentRequest'];

export type DemoSpec = { kind: 'gantry.phase0.demo/v1'; mode: 'complete' | 'await_cancel' };
