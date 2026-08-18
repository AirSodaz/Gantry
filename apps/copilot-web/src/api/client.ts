import type {
  Agent,
  AgentList,
  AppendSessionMessageInput,
  Approval,
  ApprovalList,
  Artifact,
  ArtifactDownloadGrant,
  ArtifactList,
  Attachment,
  AttachmentUploadGrant,
  CreateAttachmentInput,
  CreateSessionInput,
  RunSummary,
  RunSummaryList,
  Session,
  SessionEventsTicket,
  SessionList,
} from "./types";

const baseUrl = import.meta.env.VITE_COPILOT_API_BASE ?? "/api/copilot/v1";

type CopilotProblem = {
  code?: string;
  message?: string;
  correlation_id?: string;
  retryable?: boolean;
  current_resource?: unknown;
};

type TokenProvider = () => string | null;

export class CopilotApiError extends Error {
  constructor(
    public readonly status: number,
    message: string,
    public readonly code?: string,
    public readonly currentResource?: unknown,
    public readonly correlationId?: string,
    public readonly retryable?: boolean,
  ) {
    super(message);
    this.name = "CopilotApiError";
  }
}

export class CopilotApi {
  constructor(private readonly tokenProvider: TokenProvider) {}

  listAgents(
    search = "",
    category = "",
    cursor = "",
    collection: "all" | "favorites" | "recent" = "all",
  ) {
    const params = new URLSearchParams();
    if (search) params.set("search", search);
    if (category) params.set("category", category);
    if (cursor) params.set("cursor", cursor);
    if (collection !== "all") params.set("collection", collection);
    return this.request<AgentList>(`/agents?${params}`);
  }

  setAgentFavorite(
    id: string,
    isFavorite: boolean,
    key: string = crypto.randomUUID(),
  ) {
    return this.request<Agent>(`/agents/${encodeURIComponent(id)}/favorite`, {
      method: "PUT",
      headers: { "Idempotency-Key": key },
      body: JSON.stringify({ is_favorite: isFavorite }),
    });
  }

  listSessions(
    filters: {
      state?: string;
      mode?: string;
      agentId?: string;
      myAction?: string;
      updatedAfter?: string;
      cursor?: string;
    } = {},
  ) {
    const params = new URLSearchParams();
    if (filters.state) params.set("state", filters.state);
    if (filters.mode) params.set("mode", filters.mode);
    if (filters.agentId) params.set("agent_id", filters.agentId);
    if (filters.myAction) params.set("my_action", filters.myAction);
    if (filters.updatedAfter) params.set("updated_after", filters.updatedAfter);
    if (filters.cursor) params.set("cursor", filters.cursor);
    return this.request<SessionList>(`/sessions?${params}`);
  }

  getSession(id: string) {
    return this.requestSession(`/sessions/${encodeURIComponent(id)}`);
  }

  createSession(input: CreateSessionInput, key: string) {
    return this.requestSession("/sessions", {
      method: "POST",
      headers: { "Idempotency-Key": key },
      body: JSON.stringify(input),
    });
  }

  appendSessionMessage(
    id: string,
    input: AppendSessionMessageInput,
    key: string,
    etag: string,
  ) {
    return this.requestSession(`/sessions/${encodeURIComponent(id)}/messages`, {
      method: "POST",
      headers: { "Idempotency-Key": key, "If-Match": etag },
      body: JSON.stringify(input),
    });
  }

  listSessionRuns(id: string, cursor = "") {
    const params = new URLSearchParams();
    if (cursor) params.set("cursor", cursor);
    return this.request<RunSummaryList>(
      `/sessions/${encodeURIComponent(id)}/runs?${params}`,
    );
  }

  cancelSessionRun(id: string, run: string, key: string) {
    return this.request<RunSummary>(
      `/sessions/${encodeURIComponent(id)}/runs/${encodeURIComponent(run)}:cancel`,
      { method: "POST", headers: { "Idempotency-Key": key } },
    );
  }

  retrySessionRun(
    id: string,
    run: string,
    key: string,
    etag: string,
    revision: "original_revision" | "current_production_revision",
  ) {
    return this.request<RunSummary>(
      `/sessions/${encodeURIComponent(id)}/runs/${encodeURIComponent(run)}:retry`,
      {
        method: "POST",
        headers: { "Idempotency-Key": key, "If-Match": etag },
        body: JSON.stringify({ revision_selection: revision }),
      },
    );
  }

  createSessionEventsTicket(id: string, cursor?: string) {
    return this.request<SessionEventsTicket>(
      `/sessions/${encodeURIComponent(id)}/events:ticket`,
      {
        method: "POST",
        body: JSON.stringify(cursor ? { last_cursor: cursor } : {}),
      },
    );
  }

  listApprovals(cursor = "", state = "") {
    const params = new URLSearchParams();
    if (cursor) params.set("cursor", cursor);
    if (state) params.set("state", state);
    return this.request<ApprovalList>(`/approvals?${params}`);
  }

  getApproval(id: string) {
    return this.request<Approval>(`/approvals/${encodeURIComponent(id)}`);
  }

  decideApproval(
    id: string,
    decision: "approve" | "reject",
    digest: string,
    revision: number,
    reason = "",
    key: string = crypto.randomUUID(),
  ) {
    return this.request<Approval>(`/approvals/${encodeURIComponent(id)}:decide`, {
      method: "POST",
      headers: { "Idempotency-Key": key },
      body: JSON.stringify({
        decision,
        action_digest: digest,
        approval_revision: revision,
        reason,
      }),
    });
  }

  listArtifacts(
    sessionId = "",
    classification = "",
    cursor = "",
    state = "",
  ) {
    const params = new URLSearchParams();
    if (sessionId) params.set("session_id", sessionId);
    if (classification) params.set("classification", classification);
    if (cursor) params.set("cursor", cursor);
    if (state) params.set("state", state);
    return this.request<ArtifactList>(`/artifacts?${params}`);
  }

  getArtifact(id: string) {
    return this.request<Artifact>(`/artifacts/${encodeURIComponent(id)}`);
  }

  requestArtifactDownload(id: string) {
    return this.request<ArtifactDownloadGrant>(
      `/artifacts/${encodeURIComponent(id)}:download`,
      { method: "POST" },
    );
  }

  createAttachment(
    input: CreateAttachmentInput,
    key = crypto.randomUUID(),
  ) {
    return this.request<AttachmentUploadGrant>("/attachments", {
      method: "POST",
      headers: { "Idempotency-Key": key },
      body: JSON.stringify(input),
    });
  }

  uploadAttachment(
    grant: AttachmentUploadGrant,
    file: File,
    progress?: (percent: number) => void,
  ) {
    return new Promise<void>((resolve, reject) => {
      const token = this.tokenProvider();
      if (!token) {
        reject(new CopilotApiError(401, "Your session has expired."));
        return;
      }
      const xhr = new XMLHttpRequest();
      xhr.open("PUT", `${baseUrl}${grant.upload_path}`);
      xhr.setRequestHeader("Content-Type", file.type || "application/octet-stream");
      xhr.setRequestHeader("Authorization", `Bearer ${token}`);
      xhr.setRequestHeader("X-Gantry-Upload-Token", grant.upload_token);
      xhr.upload.onprogress = (event) => {
        if (event.lengthComputable) {
          progress?.(Math.round((event.loaded / event.total) * 100));
        }
      };
      xhr.onerror = () => reject(new CopilotApiError(0, "Attachment upload failed."));
      xhr.onload = () => {
        if (xhr.status >= 200 && xhr.status < 300) resolve();
        else reject(new CopilotApiError(xhr.status, "Attachment upload was rejected."));
      };
      xhr.send(file);
    });
  }

  completeAttachment(id: string, key = crypto.randomUUID()) {
    return this.request<Attachment>(
      `/attachments/${encodeURIComponent(id)}:complete`,
      { method: "POST", headers: { "Idempotency-Key": key } },
    );
  }

  private async request<T>(path: string, init: RequestInit = {}) {
    return (await this.response(path, init)).json() as Promise<T>;
  }

  private async requestSession(path: string, init: RequestInit = {}) {
    const response = await this.response(path, init);
    const session = (await response.json()) as Session;
    return {
      ...session,
      conversation_etag: response.headers.get("ETag") ?? undefined,
    };
  }

  private async response(path: string, init: RequestInit = {}) {
    const token = this.tokenProvider();
    if (!token) throw new CopilotApiError(401, "Your session has expired.");
    const response = await fetch(`${baseUrl}${path}`, {
      ...init,
      headers: {
        Accept: "application/json",
        "Content-Type": "application/json",
        Authorization: `Bearer ${token}`,
        ...init.headers,
      },
    });
    if (response.ok) return response;

    let problem: CopilotProblem | undefined;
    try {
      problem = (await response.json()) as CopilotProblem;
    } catch {
      // Preserve the HTTP status when the body is not a CopilotProblem.
    }
    throw new CopilotApiError(
      response.status,
      problem?.message ?? `Request failed with status ${response.status}.`,
      problem?.code,
      problem?.current_resource,
      problem?.correlation_id,
      problem?.retryable,
    );
  }
}
