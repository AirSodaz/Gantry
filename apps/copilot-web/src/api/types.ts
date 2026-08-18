import type { CopilotApiComponents } from "@gantry/api-client";

type components = CopilotApiComponents;

export type Agent = components["schemas"]["CopilotAgent"];
export type AgentList = components["schemas"]["CopilotAgentList"];
export type Session = components["schemas"]["Session"] & { conversation_etag?: string };
export type SessionList = components["schemas"]["SessionList"];
export type SessionMessage = components["schemas"]["SessionMessage"];
export type RunSummary = components["schemas"]["RunSummary"];
export type RunSummaryList = components["schemas"]["RunSummaryList"];
export type Approval = components["schemas"]["CopilotApproval"];
export type ApprovalList = components["schemas"]["CopilotApprovalList"];
export type Artifact = components["schemas"]["Artifact"];
export type ArtifactList = components["schemas"]["ArtifactList"];
export type ArtifactDownloadGrant = components["schemas"]["ArtifactDownloadGrant"];
export type Attachment = components["schemas"]["Attachment"];
export type AttachmentUploadGrant = components["schemas"]["AttachmentUploadGrant"];
export type CreateAttachmentInput = components["schemas"]["CreateAttachmentRequest"];
export type SessionEventsTicket = components["schemas"]["SessionEventsTicket"];
export type SessionEventSnapshot = components["schemas"]["SessionEventSnapshot"];
export type SessionEventFrame = components["schemas"]["SessionEventFrame"];
export type CreateSessionInput = components["schemas"]["CreateSessionRequest"];
export type AppendSessionMessageInput = components["schemas"]["AppendSessionMessageRequest"];
