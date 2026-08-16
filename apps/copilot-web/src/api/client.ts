import type { AgentList, AppendTaskMessageInput, Approval, ApprovalList, Artifact, ArtifactDownloadGrant, ArtifactList, Attachment, CreateAttachmentInput, EventsTicket, RunAttempt, RunStatus, SubmitTaskInput, Task, TaskList } from './types';

const baseUrl = import.meta.env.VITE_COPILOT_API_BASE ?? '/api/copilot/v1';

export class CopilotApiError extends Error {
  constructor(
    public readonly status: number,
    message: string,
    public readonly code?: string,
    public readonly currentResource?: unknown,
  ) {
    super(message);
    this.name = 'CopilotApiError';
  }
}

type TokenProvider = () => string | null;

export class CopilotApi {
  constructor(private readonly tokenProvider: TokenProvider) {}

  listAgents(search = '', category = '', cursor = '') {
    const params = new URLSearchParams();
    if (search) params.set('search', search);
    if (category) params.set('category', category);
    if (cursor) params.set('cursor', cursor);
    return this.request<AgentList>(`/agents?${params.toString()}`);
  }

  listTasks(filters: { status?: string; agentId?: string; requesterAction?: string; createdAfter?: string; cursor?: string } = {}) {
    const params = new URLSearchParams();
    if (filters.status) params.set('status', filters.status);
    if (filters.agentId) params.set('agent_id', filters.agentId);
    if (filters.requesterAction) params.set('requester_action', filters.requesterAction);
    if (filters.createdAfter) params.set('created_after', filters.createdAfter);
	if (filters.cursor) params.set('cursor', filters.cursor);
    return this.request<TaskList>(`/tasks?${params.toString()}`);
  }

  listApprovals(cursor = '') {
    const params = new URLSearchParams();
    if (cursor) params.set('cursor', cursor);
    return this.request<ApprovalList>(`/approvals?${params.toString()}`);
  }

  decideApproval(approvalId: string, decision: 'approve' | 'reject', actionDigest: string, approvalRevision: number, reason = '', idempotencyKey: string = crypto.randomUUID()) {
    return this.request<Approval>(`/approvals/${encodeURIComponent(approvalId)}:decide`, {
      method: 'POST',
      headers: { 'Idempotency-Key': idempotencyKey },
      body: JSON.stringify({ decision, action_digest: actionDigest, approval_revision: approvalRevision, reason }),
    });
  }

  getTask(taskId: string) {
    return this.requestTask(`/tasks/${encodeURIComponent(taskId)}`);
  }

  appendTaskMessage(taskId: string, input: AppendTaskMessageInput, idempotencyKey: string, conversationETag: string) {
    return this.requestTask(`/tasks/${encodeURIComponent(taskId)}/messages`, {
      method: 'POST',
      headers: { 'Idempotency-Key': idempotencyKey, 'If-Match': conversationETag },
      body: JSON.stringify(input),
    });
  }

  listTaskRuns(taskId: string) {
    return this.request<{ items: RunAttempt[] }>(`/tasks/${encodeURIComponent(taskId)}/runs`);
  }

  createEventsTicket(taskId: string, lastCursor?: string) {
    return this.request<EventsTicket>(`/tasks/${encodeURIComponent(taskId)}/events:ticket`, {
      method: 'POST',
      body: JSON.stringify(lastCursor ? { last_cursor: lastCursor } : {}),
    });
  }

  getArtifact(artifactId: string) {
    return this.request<Artifact>(`/artifacts/${encodeURIComponent(artifactId)}`);
  }

  requestArtifactDownload(artifactId: string) {
    return this.request<ArtifactDownloadGrant>(`/artifacts/${encodeURIComponent(artifactId)}:download`, { method: 'POST' });
  }

  listArtifacts(taskId = '', classification = '', cursor = '') {
    const params = new URLSearchParams();
    if (taskId) params.set('task_id', taskId);
    if (classification) params.set('classification', classification);
    if (cursor) params.set('cursor', cursor);
    return this.request<ArtifactList>(`/artifacts?${params.toString()}`);
  }

  createAttachment(input: CreateAttachmentInput) {
    return this.request<Attachment>('/attachments', {
      method: 'POST',
      body: JSON.stringify(input),
    });
  }

  uploadAttachment(attachment: Attachment, file: File, onProgress?: (percent: number) => void) {
    const token = this.tokenProvider();
    if (!token) return Promise.reject(new CopilotApiError(401, 'Your session has expired.'));
    if (!attachment.upload_url || !attachment.upload_token) {
      return Promise.reject(new CopilotApiError(409, 'This attachment upload grant is no longer available.'));
    }
    const uploadURL = attachment.upload_url;
    const uploadToken = attachment.upload_token;
    return new Promise<void>((resolve, reject) => {
      const request = new XMLHttpRequest();
      request.open('PUT', uploadURL);
      request.setRequestHeader('Authorization', `Bearer ${token}`);
      request.setRequestHeader('Content-Type', file.type || 'application/octet-stream');
      request.setRequestHeader('X-Gantry-Upload-Token', uploadToken);
      request.upload.onprogress = (event) => {
        if (event.lengthComputable) onProgress?.(Math.round((event.loaded / event.total) * 100));
      };
      request.onerror = () => reject(new CopilotApiError(0, 'Attachment upload failed.'));
      request.onload = () => {
        if (request.status >= 200 && request.status < 300) {
          resolve();
          return;
        }
        reject(new CopilotApiError(request.status, 'Attachment upload was rejected.'));
      };
      request.send(file);
    });
  }

  completeAttachment(attachmentId: string) {
    return this.request<Attachment>(`/attachments/${encodeURIComponent(attachmentId)}:complete`, {
      method: 'POST',
      body: '{}',
    });
  }

  getApproval(approvalId: string) {
    return this.request<Approval>(`/approvals/${encodeURIComponent(approvalId)}`);
  }

  submitTask(input: SubmitTaskInput, idempotencyKey: string) {
    return this.request<Task>('/tasks', {
      method: 'POST',
      headers: { 'Idempotency-Key': idempotencyKey },
      body: JSON.stringify(input),
    });
  }

  cancelRun(taskId: string, runId: string, idempotencyKey: string) {
    return this.request<RunStatus>(`/tasks/${encodeURIComponent(taskId)}/runs/${encodeURIComponent(runId)}:cancel`, {
      method: 'POST',
      headers: { 'Idempotency-Key': idempotencyKey },
      body: '{}',
    });
  }

  retryTask(taskId: string, idempotencyKey: string, conversationETag: string, revisionSelection: 'original_revision' | 'current_production_revision') {
    return this.requestTask(`/tasks/${encodeURIComponent(taskId)}:retry`, {
      method: 'POST',
      headers: { 'Idempotency-Key': idempotencyKey, 'If-Match': conversationETag },
      body: JSON.stringify({ revision_selection: revisionSelection }),
    });
  }

  private async request<T>(path: string, init: RequestInit = {}): Promise<T> {
    const response = await this.response(path, init);
    return response.json() as Promise<T>;
  }

  private async requestTask(path: string, init: RequestInit = {}): Promise<Task> {
    const response = await this.response(path, init);
    const task = await response.json() as Task;
    return { ...task, conversation_etag: response.headers.get('ETag') ?? undefined };
  }

  private async response(path: string, init: RequestInit = {}): Promise<Response> {
    const token = this.tokenProvider();
    if (!token) throw new CopilotApiError(401, 'Your session has expired.');
    const response = await fetch(`${baseUrl}${path}`, {
      ...init,
      headers: {
        Accept: 'application/json',
        'Content-Type': 'application/json',
        Authorization: `Bearer ${token}`,
        ...init.headers,
      },
    });
    if (!response.ok) {
      let message = `Request failed with status ${response.status}.`;
      let payload: { error?: { code?: string; message?: string; current_resource?: unknown } } | undefined;
      try {
        payload = await response.json() as { error?: { code?: string; message?: string; current_resource?: unknown } };
      } catch {
        // Preserve the status-based message when the server did not return JSON.
      }
      message = payload?.error?.message ?? message;
      throw new CopilotApiError(response.status, message, payload?.error?.code, payload?.error?.current_resource);
    }
    return response;
  }
}
