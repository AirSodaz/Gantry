import type { CopilotApiComponents } from '@gantry/api-client';

type components = CopilotApiComponents;

export type Agent = components['schemas']['CopilotAgent'];
export type Task = components['schemas']['TaskResponse'];
export type TaskStatus = Task['status'];
export type RunStatus = components['schemas']['RunStatus'];
export type RunAttempt = components['schemas']['RunAttempt'];
export type TaskMessage = components['schemas']['TaskMessage'];
export type Approval = components['schemas']['CopilotApproval'];
export type ApprovalDecisionResponse = components['schemas']['ApprovalDecisionResponse'];
export type Artifact = components['schemas']['ArtifactResponse'];
export type Attachment = components['schemas']['AttachmentResponse'];
export type CreateAttachmentInput = components['schemas']['CreateAttachmentRequest'];
export type EventsTicket = components['schemas']['TaskEventsTicket'];

export type SubmitTaskInput = {
  agent_id: string;
  message?: string;
  structured_input?: Record<string, unknown>;
  attachment_ids?: string[];
};

export type AppendTaskMessageInput = components['schemas']['AppendTaskMessageRequest'];
