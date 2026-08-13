export interface Organization {
  id: string;
  name: string;
}

export interface Workspace {
  id: string;
  organizationId: string;
  name: string;
}

export interface Agent {
  id: string;
  workspaceId: string;
  name: string;
}

export interface AgentVersion {
  id: string;
  agentId: string;
  version: number;
}

export interface Task {
  id: string;
  agentId: string;
  status: string;
}

export interface Run {
  id: string;
  taskId: string;
  agentVersionId: string;
  status: string;
}
