# Copilot Resource Contracts

## 1. Scope and Status

This document is the target typed contract for the employee-facing Copilot
surface. It turns the page behavior in
[Copilot Site Design](../product/copilot-site-design.md) into resource schemas,
HTTP commands, event frames, authorization rules, and recovery semantics.

The checked-in OpenAPI document remains authoritative for callable routes. A
target rule here is implemented only after it appears in
`packages/contracts/openapi/copilot-api.yaml`, generated clients, the owning
control-plane module, and focused transition and authorization tests. The
current delivery boundary is recorded in
[Implementation Status](../delivery/implementation-status.md).

Copilot uses audience `gantry-copilot-api`. Its responses are projections for
the authenticated employee, not redacted Admin resources. Enterprise callers
use the separate Agent Invocation API and cannot present service credentials to
Copilot routes.

## 2. Identity and Authorization Boundary

Every request is evaluated against a normalized authentication context:

```yaml
CopilotActor:
  type: object
  required: [principal_id, organization_id, workspace_ids, session_id]
  properties:
    principal_id: {type: string}
    organization_id: {type: string}
    workspace_ids: {type: array, items: {type: string}}
    session_id: {type: string}
    authentication_strength: {type: string}
```

The actor is derived from the access token and server-side identity mapping;
request JSON cannot override it. Resource authorization follows these rules:

- Agent discovery requires safe metadata access, Agent execution permission,
  and an active Deployment in an allowed Workspace.
- Creating a Task records the authenticated principal as its immutable
  requester. Copilot Task, Run, Message, Attachment, Approval, Event, and
  Artifact reads require that same requester unless a later explicit sharing
  resource is introduced.
- Only the Task requester may decide its Agent action approval. Admin roles,
  Workspace ownership, operator access, and business approver roles do not
  grant this command.
- A resource outside the caller's projection returns `404 resource_not_found`.
  The API does not distinguish absent, cross-requester, cross-Workspace, or
  policy-hidden resources.
- List filters are authorization constraints applied in the query, not
  post-query redaction. Cursors cannot be replayed by another principal or
  against a different filter set.
- Business workflow approvals remain in the owning Tool or enterprise system.
  Copilot may render a Tool-provided status summary but cannot decide it.

There is no generic `copilot.admin` or `read_all_tasks` capability. Cross-user
investigation is an Admin operational projection with separate response types
and audit requirements.

## 3. Common HTTP Rules

### 3.1 Commands and concurrency

- Durable commands require an `Idempotency-Key` header scoped to principal,
  route template, and target resource. The server stores the canonical request
  digest and final response for at least the command retry window.
- Reusing a key with the same digest returns the original status and response.
  Reusing it with a different digest returns `409 idempotency_conflict`.
- Task responses return a conversation `ETag`. Appending a follow-up message
  requires `If-Match`; stale state returns `409 conversation_changed` with the
  current Task projection and no new Message or Run.
- Approval decisions require the latest `approval_revision` and exact
  `action_digest`. A stale revision returns `409 approval_changed`; a mismatched
  digest returns `412 action_changed`.
- Cancellation and retry are reconciled against server state. A terminal Run
  is never moved backward because an older response arrives.
- Event tickets and download grants are short-lived capabilities, not durable
  commands. Creating one is not idempotent and never changes domain state.

### 3.2 Common response types

```yaml
PageInfo:
  type: object
  required: [has_more]
  properties:
    next_cursor: {type: string, nullable: true}
    has_more: {type: boolean}

CopilotProblem:
  type: object
  required: [code, message, correlation_id, retryable]
  properties:
    code: {type: string}
    message: {type: string}
    correlation_id: {type: string}
    retryable: {type: boolean}
    retry_after_seconds: {type: integer, nullable: true}
    current_resource: {type: object, nullable: true}

CommandMeta:
  type: object
  required: [correlation_id]
  properties:
    correlation_id: {type: string}
    replayed: {type: boolean, default: false}

SupportContact:
  type: object
  required: [display_name]
  properties:
    display_name: {type: string}
    contact_uri: {type: string, nullable: true}
```

Validation failures use `422`; rate limits use `429`; temporary dependency
failure uses `503`. Employee messages never include runner IDs, lease epochs,
object keys, provider credentials, raw Policy documents, or internal prompts.

## 4. Typed Resource Schemas

The following OpenAPI-like schemas define the target shape. Optional fields are
omitted when they do not apply; privileged fields are never present.

### 4.1 Agent catalog

```yaml
CopilotAgent:
  type: object
  required: [id, display_name, description, availability, input_contract,
             output_disclosure, data_disclosure]
  properties:
    id: {type: string}
    display_name: {type: string}
    description: {type: string}
    category: {type: string, nullable: true}
    owner: {$ref: '#/components/schemas/SupportContact'}
    availability: {type: string, enum: [available, temporarily_unavailable]}
    input_contract: {type: object}
    output_disclosure: {type: object}
    data_disclosure: {type: object}
    action_disclosure: {type: object}
    is_favorite: {type: boolean}
```

This projection is produced from an exact active Deployment and published
metadata. It does not expose Revision hashes, prompts, model routes, Tool
bindings, Policy rules, or credential references.

### 4.2 Task, Message, and Run

```yaml
TaskState:
  type: string
  enum: [queued, provisioning, running, awaiting_approval,
         awaiting_requester_input, suspended, canceling,
         completed, failed, canceled, expired]

RunState:
  type: string
  enum: [queued, provisioning, running, awaiting_approval, suspended, canceling,
         completed, failed, canceled, expired]

Task:
  type: object
  required: [id, requester_id, agent, state, conversation_revision,
             current_run, created_at, updated_at]
  properties:
    id: {type: string}
    requester_id: {type: string}
    agent: {$ref: '#/components/schemas/TaskAgentSnapshot'}
    title: {type: string, nullable: true}
    state: {$ref: '#/components/schemas/TaskState'}
    state_reason: {$ref: '#/components/schemas/UserFacingReason'}
    conversation_revision: {type: integer, format: int64, minimum: 1}
    requester_action: {type: string, enum: [none, approval, input]}
    current_run: {$ref: '#/components/schemas/RunSummary'}
    messages: {type: array, items: {$ref: '#/components/schemas/TaskMessage'}}
    artifacts: {type: array, items: {$ref: '#/components/schemas/Artifact'}}
    created_at: {type: string, format: date-time}
    updated_at: {type: string, format: date-time}

TaskAgentSnapshot:
  type: object
  required: [agent_id, display_name]
  properties:
    agent_id: {type: string}
    display_name: {type: string}
    support_contact: {$ref: '#/components/schemas/SupportContact'}

TaskMessage:
  type: object
  required: [id, task_sequence, role, parts, created_at]
  properties:
    id: {type: string}
    run_id: {type: string, nullable: true}
    task_sequence: {type: integer, format: int64, minimum: 1}
    role: {type: string, enum: [requester, agent, system_summary]}
    parts:
      type: array
      items:
        oneOf:
          - {$ref: '#/components/schemas/TextPart'}
          - {$ref: '#/components/schemas/ArtifactPart'}
          - {$ref: '#/components/schemas/ActionSummaryPart'}
          - {$ref: '#/components/schemas/StatusPart'}
        discriminator: {propertyName: type}
    created_at: {type: string, format: date-time}

TextPart:
  type: object
  required: [type, text]
  properties:
    type: {type: string, enum: [text]}
    text: {type: string}

ArtifactPart:
  type: object
  required: [type, artifact_id, label]
  properties:
    type: {type: string, enum: [artifact]}
    artifact_id: {type: string}
    label: {type: string}

ActionSummaryPart:
  type: object
  required: [type, action_id, summary, state]
  properties:
    type: {type: string, enum: [action_summary]}
    action_id: {type: string}
    summary: {type: string}
    state: {type: string}

StatusPart:
  type: object
  required: [type, code, message]
  properties:
    type: {type: string, enum: [status]}
    code: {type: string}
    message: {type: string}

RunSummary:
  type: object
  required: [id, attempt_number, state, created_at]
  properties:
    id: {type: string}
    attempt_number: {type: integer, minimum: 1}
    state: {$ref: '#/components/schemas/RunState'}
    outcome:
      type: string
      enum: [succeeded, requester_input_required, failed, canceled, expired]
      nullable: true
    state_reason: {$ref: '#/components/schemas/UserFacingReason'}
    retry_of_run_id: {type: string, nullable: true}
    created_at: {type: string, format: date-time}
    started_at: {type: string, format: date-time, nullable: true}
    completed_at: {type: string, format: date-time, nullable: true}

UserFacingReason:
  type: object
  required: [code, message]
  properties:
    code: {type: string}
    message: {type: string}
    next_action: {type: string, enum: [none, wait, approve, provide_input,
                                      retry, contact_support]}
    correlation_id: {type: string, nullable: true}
```

`Task.state` is the employee workflow projection and is not a copy of Run state.
Earlier Run summaries remain immutable after reaching a terminal state. After
the Agent consumes a rejected, expired, superseded, or revoked action result and
reaches a safe input boundary, the Run completes with outcome
`requester_input_required` and the Task becomes `awaiting_requester_input`.
Appending a message atomically records the Message and creates the next queued
Run. At most one Run per Task is non-terminal.

### 4.3 Approval

```yaml
CopilotApproval:
  type: object
  required: [id, task_id, run_id, action_id, action_digest,
             approval_revision, state, preview, expires_at, created_at]
  properties:
    id: {type: string}
    task_id: {type: string}
    run_id: {type: string}
    action_id: {type: string}
    action_digest: {type: string, pattern: '^sha256:'}
    approval_revision: {type: integer, format: int64, minimum: 1}
    state: {type: string, enum: [pending, approved, rejected, expired,
                                 superseded, revoked]}
    preview:
      type: object
      required: [summary, effect, risk_class]
      properties:
        summary: {type: string}
        tool_display_name: {type: string}
        operation_display_name: {type: string}
        target: {type: string, nullable: true}
        effect: {type: string}
        risk_class: {type: string}
        redacted_details: {type: object}
        policy_reason: {type: string, nullable: true}
    decision: {$ref: '#/components/schemas/ApprovalDecisionEvidence'}
    expires_at: {type: string, format: date-time}
    created_at: {type: string, format: date-time}

ApprovalDecisionEvidence:
  type: object
  required: [decision, decided_at]
  properties:
    decision: {type: string, enum: [approve, reject]}
    reason: {type: string, nullable: true}
    decided_by_current_requester: {type: boolean, enum: [true]}
    decided_at: {type: string, format: date-time}
```

Approval is evidence for one immutable action. Approval never means that the
action executed, succeeded, or remains authorized after a Policy, lease,
credential, cancellation, or target change.

### 4.4 Attachment and Artifact

```yaml
Attachment:
  type: object
  required: [id, filename, media_type, size_bytes, digest, classification,
             state, scan_state, created_at, expires_at]
  properties:
    id: {type: string}
    filename: {type: string}
    media_type: {type: string}
    size_bytes: {type: integer, format: int64, minimum: 0}
    digest: {type: string, pattern: '^sha256:'}
    classification: {type: string}
    state: {type: string, enum: [declared, uploading, quarantined, available,
                                 bound, rejected, expired, deleted]}
    scan_state: {type: string, enum: [pending, passed, failed, unavailable]}
    rejection_reason: {$ref: '#/components/schemas/UserFacingReason'}
    bound_task_id: {type: string, nullable: true}
    created_at: {type: string, format: date-time}
    expires_at: {type: string, format: date-time}

Artifact:
  type: object
  required: [id, task_id, run_id, filename, media_type, size_bytes, digest,
             classification, state, scan_state, created_at]
  properties:
    id: {type: string}
    task_id: {type: string}
    run_id: {type: string}
    filename: {type: string}
    media_type: {type: string}
    size_bytes: {type: integer, format: int64}
    digest: {type: string, pattern: '^sha256:'}
    classification: {type: string}
    state: {type: string, enum: [declared, uploading, quarantined, available,
                                 rejected, expired, deleted]}
    scan_state: {type: string, enum: [pending, passed, failed, unavailable]}
    retention: {type: object}
    created_at: {type: string, format: date-time}

AttachmentUploadGrant:
  type: object
  required: [attachment, upload_path, upload_token, expires_at]
  properties:
    attachment: {$ref: '#/components/schemas/Attachment'}
    upload_path: {type: string}
    upload_token: {type: string, description: Returned only when the grant is created}
    expires_at: {type: string, format: date-time}

ArtifactDownloadGrant:
  type: object
  required: [artifact_id, download_url, expires_at]
  properties:
    artifact_id: {type: string}
    download_url: {type: string, format: uri, description: Returned only by the download command}
    expires_at: {type: string, format: date-time}
```

Upload credentials and download grants are separate one-response schemas and
are never persisted in a Task projection. Attachment bytes remain quarantined
until size, digest, media type, classification, malware, and policy checks pass.
A Task submission atomically binds only requester-owned `available`
Attachments; a bound Attachment cannot be rebound or replaced. Expiry or
deletion retains a digest-bearing tombstone when evidence policy requires it.

### 4.5 Task event stream

```yaml
TaskEventSnapshot:
  type: object
  required: [schema_version, task, runs, approvals, cursor]
  properties:
    schema_version: {type: string, enum: ['gantry.copilot.snapshot/v1']}
    task: {$ref: '#/components/schemas/Task'}
    runs: {type: array, items: {$ref: '#/components/schemas/RunSummary'}}
    approvals: {type: array, items: {$ref: '#/components/schemas/CopilotApproval'}}
    cursor: {type: string}

TaskEventFrame:
  type: object
  required: [schema_version, task_id, task_sequence, cursor, event]
  properties:
    schema_version: {type: string, enum: ['gantry.copilot.event/v1']}
    task_id: {type: string}
    run_id: {type: string, nullable: true}
    task_sequence: {type: integer, format: int64, minimum: 1}
    run_sequence: {type: integer, format: int64, nullable: true}
    cursor: {type: string}
    event:
      oneOf:
        - {$ref: '#/components/schemas/MessageCommittedEvent'}
        - {$ref: '#/components/schemas/ContentSegmentEvent'}
        - {$ref: '#/components/schemas/RunStateChangedEvent'}
        - {$ref: '#/components/schemas/ApprovalChangedEvent'}
        - {$ref: '#/components/schemas/ArtifactChangedEvent'}
      discriminator: {propertyName: type}

MessageCommittedEvent:
  type: object
  required: [type, message]
  properties:
    type: {type: string, enum: [message_committed]}
    message: {$ref: '#/components/schemas/TaskMessage'}

ContentSegmentEvent:
  type: object
  required: [type, message_id, segment_index, text]
  properties:
    type: {type: string, enum: [content_segment]}
    message_id: {type: string}
    segment_index: {type: integer, minimum: 0}
    text: {type: string}
    final: {type: boolean}

RunStateChangedEvent:
  type: object
  required: [type, run]
  properties:
    type: {type: string, enum: [run_state_changed]}
    run: {$ref: '#/components/schemas/RunSummary'}

ApprovalChangedEvent:
  type: object
  required: [type, approval]
  properties:
    type: {type: string, enum: [approval_changed]}
    approval: {$ref: '#/components/schemas/CopilotApproval'}

ArtifactChangedEvent:
  type: object
  required: [type, artifact]
  properties:
    type: {type: string, enum: [artifact_changed]}
    artifact: {$ref: '#/components/schemas/Artifact'}

TaskEventTicket:
  type: object
  required: [ticket, task_id, websocket_url, expires_at]
  properties:
    ticket: {type: string, description: Returned only when the ticket is created}
    task_id: {type: string}
    websocket_url: {type: string, format: uri}
    expires_at: {type: string, format: date-time}
```

The target cursor is opaque, Task-bound, requester-bound, projection-bound,
and filter-bound. `task_sequence` orders durable employee-visible changes across
Runs; `run_sequence` preserves the diagnostic order inside one Run. Provisional
content has no durable cursor and is replaced by committed content segments.

List responses are explicit OpenAPI components; they do not use an untyped
generic envelope.

```yaml
CopilotAgentList:
  type: object
  required: [items, page_info]
  properties:
    items: {type: array, items: {$ref: '#/components/schemas/CopilotAgent'}}
    page_info: {$ref: '#/components/schemas/PageInfo'}

TaskList:
  type: object
  required: [items, page_info]
  properties:
    items: {type: array, items: {$ref: '#/components/schemas/Task'}}
    page_info: {$ref: '#/components/schemas/PageInfo'}

RunSummaryList:
  type: object
  required: [items, page_info]
  properties:
    items: {type: array, items: {$ref: '#/components/schemas/RunSummary'}}
    page_info: {$ref: '#/components/schemas/PageInfo'}

ApprovalList:
  type: object
  required: [items, page_info]
  properties:
    items: {type: array, items: {$ref: '#/components/schemas/CopilotApproval'}}
    page_info: {$ref: '#/components/schemas/PageInfo'}

ArtifactList:
  type: object
  required: [items, page_info]
  properties:
    items: {type: array, items: {$ref: '#/components/schemas/Artifact'}}
    page_info: {$ref: '#/components/schemas/PageInfo'}
```

## 5. HTTP Route Contract

### 5.1 Command schemas

```yaml
CreateAttachmentRequest:
  type: object
  required: [filename, media_type, size_bytes, digest, classification]
  properties:
    filename: {type: string, minLength: 1}
    media_type: {type: string, minLength: 1}
    size_bytes: {type: integer, format: int64, minimum: 0}
    digest: {type: string, pattern: '^sha256:'}
    classification: {type: string}

SubmitTaskRequest:
  type: object
  required: [agent_id]
  properties:
    agent_id: {type: string}
    message: {type: string, minLength: 1}
    structured_input: {type: object}
    attachment_ids: {type: array, uniqueItems: true, items: {type: string}}
  oneOf:
    - required: [message]
    - required: [structured_input]

AppendTaskMessageRequest:
  type: object
  required: [message]
  properties:
    message: {type: string, minLength: 1}
    attachment_ids: {type: array, uniqueItems: true, items: {type: string}}

RetryTaskRequest:
  type: object
  required: [revision_selection]
  properties:
    revision_selection:
      type: string
      enum: [original_revision, current_production_revision]

ApprovalDecisionRequest:
  type: object
  required: [decision, action_digest, approval_revision]
  properties:
    decision: {type: string, enum: [approve, reject]}
    reason: {type: string, nullable: true}
    action_digest: {type: string, pattern: '^sha256:'}
    approval_revision: {type: integer, format: int64, minimum: 1}
```

`SubmitTaskRequest` accepts exactly one of `message` and `structured_input`.
The selected Agent's published input contract may further constrain the value.

### 5.2 Routes

All routes are relative to `/api/copilot/v1`.

| Method and route | Request or filters | Success | Required rule |
| --- | --- | --- | --- |
| `GET /agents` | cursor, limit, search, category | `200 CopilotAgentList` | Server-authorized active catalog only |
| `POST /attachments` | metadata; `Idempotency-Key` | `201 AttachmentUploadGrant`, `200` replay | Creates requester-owned quarantine record |
| `GET /attachments/{id}` | none | `200 Attachment` | Requester only; no upload secret |
| `PUT /attachments/{id}/content` | bytes and upload token | `204` | Exact length limit and one active grant |
| `POST /attachments/{id}:complete` | `Idempotency-Key` | `202 Attachment`, `200` replay | Verifies digest and schedules scan |
| `POST /tasks` | Agent, message or structured input, Attachment IDs; `Idempotency-Key` | `201 Task`, `200` replay | Binds exact Deployment and available Attachments |
| `GET /tasks` | cursor, limit, state, Agent, requester action, time | `200 TaskList` | Requester filter is mandatory in storage query |
| `GET /tasks/{id}` | none | `200 Task` plus `ETag` | Conversation-first projection |
| `POST /tasks/{id}/messages` | message and optional Attachments; `If-Match`, `Idempotency-Key` | `201 Task`, `200` replay | Only `awaiting_requester_input`; creates next Run |
| `GET /tasks/{id}/runs` | cursor, limit | `200 RunSummaryList` | Compact requester projection only |
| `POST /tasks/{id}/runs/{run_id}:cancel` | `Idempotency-Key` | `202 RunSummary`, `200` terminal/replay | Expected current Run; cancellation may reconcile |
| `POST /tasks/{id}:retry` | revision selection; `If-Match`, `Idempotency-Key` | `201 Task`, `200` replay | Only eligible terminal state; creates Run |
| `POST /tasks/{id}/events:ticket` | optional last cursor | `200 TaskEventTicket` | Short-lived, Task- and principal-bound |
| `GET /approvals` | cursor, limit, state | `200 ApprovalList` | Requester-bound; pending is default filter |
| `GET /approvals/{id}` | none | `200 CopilotApproval` | Projects effective expiry; worker owns durable transition |
| `POST /approvals/{id}:decide` | decision, reason, digest, revision; `Idempotency-Key` | `200 CopilotApproval` | Requester only; decision is not execution |
| `GET /artifacts` | cursor, limit, Task, classification, state | `200 ArtifactList` | Requester-owned Task scope only |
| `GET /artifacts/{id}` | none | `200 Artifact` | Metadata only |
| `POST /artifacts/{id}:download` | none | `200 ArtifactDownloadGrant` | Rechecks auth, scan, retention; audited access |

The checked-in contract currently returns an Artifact download URL from
`GET /artifacts/{id}` and carries approval idempotency in the request body. The
target replaces those shapes with an explicit audited download command and the
common command header; no compatibility route is required before a public API
release.

## 6. State and Command Semantics

### 6.1 Task submission and follow-up

Task submission performs one transaction: claim idempotency, authorize the
Agent, resolve the active Deployment, validate and bind Attachments, create the
Task and first requester Message, create Run attempt 1, append Task/Run events,
and enqueue scheduling through the outbox. Failure before commit creates none
of these resources.

A rejected, expired, superseded, or revoked action approval produces a durable
structured action result. After the Agent reaches a safe input boundary, the
employee projection moves to `awaiting_requester_input` and the composer remains
usable. Follow-up input never
changes the denied action or reuses its approval; any newly proposed effect has
a new action ID, digest, Policy decision, and approval if required.

### 6.2 Cancel and retry

Cancellation wins only before an execution permit is consumed. After that
point, the UI shows `canceling` until the effect is observed or reconciled and
must not claim that no external action occurred. Duplicate cancellation returns
the winning state.

Retry creates a new Run under the same Task. `use_latest_revision=false` pins
the original immutable Revision; selecting the current Production Revision
requires an explicit field and current execution authorization. Prior Messages,
Runs, approvals, and Artifacts remain evidence and are not rewritten.

### 6.3 Approval races

Decision, expiry, cancellation, action supersession, Policy revocation, and
lease loss compete through expected state and revision. Exactly one terminal
approval transition wins. A duplicate decision returns the winning evidence;
a late conflicting decision returns `409 approval_changed`. Even an approved
action must pass action-time checks and atomically consume a single-use permit.

## 7. Realtime, Reconnect, and Recovery

After creating a ticket, the client connects to
`WSS /api/copilot/v1/tasks/{task_id}/events?ticket={ticket}&after={cursor}`.
The WebSocket endpoint accepts a short-lived ticket and optional opaque cursor.
The server first returns either a snapshot or the next durable frame, then live
frames and heartbeats. Clients acknowledge only rendered durable cursors.

- A normal reconnect requests a new ticket and resumes after the last rendered
  cursor. Duplicate `task_sequence` values are ignored.
- `cursor_expired` includes a current `TaskEventSnapshot`, a replacement cursor,
  and a reason such as retention expiry or projection-version change. The
  client replaces its local projection atomically before resuming.
- `cursor_invalid` covers a different Task, principal, or filter and returns no
  snapshot.
- Ticket expiry closes the connection without changing the server-side Run.
- Server restart rebuilds current state from PostgreSQL and committed segments;
  provisional output after the last durable segment may be lost and is never
  presented as committed.
- A failed command with unknown client outcome is retried with the same
  idempotency key before any new command is issued.
- If event streaming is unavailable, HTTPS refresh may show current durable
  state. Polling does not become a second ordering or mutation mechanism.

## 8. Data Protection and Lifecycle

- Message parts, attachment metadata, Artifacts, approvals, and event frames
  carry only the employee-visible projection. Raw chain-of-thought, hidden
  prompts, secrets, raw Tool payloads, and unredacted model/provider errors are
  forbidden.
- Upload and download capabilities are short-lived, resource-bound, and
  non-transferable. Object keys and storage credentials never reach the normal
  metadata response.
- Scan failure, retention expiry, legal hold, quarantine, or deletion changes
  availability independently of historical Task and event evidence.
- Read access is re-authorized on every request. A Task being visible earlier
  does not create a permanent browser entitlement.
- Artifact download and approval decision emit canonical Audit events. Normal
  list/detail reads contribute access telemetry but do not create alternate
  resource-specific Audit stores.

## 9. Acceptance and Verification Contract

- Generated Copilot types contain no Admin-only runner, credential, Policy,
  prompt, Tool argument, or cross-requester fields.
- Tests prove non-leaking `404` behavior for every direct resource route and
  authorization inside list queries.
- Submit, follow-up, cancel, retry, Attachment completion, and approval decision
  cover replay, conflicting key, stale precondition, and concurrent command
  cases.
- Rejection and expiry preserve conversation input; the next message creates
  exactly one new Run and never replays the denied action.
- Task-level streams cross Run boundaries, deduplicate durable frames, replace
  snapshots on cursor expiry, and recover after process restart.
- Attachment and Artifact tests cover size/digest mismatch, scan failure,
  quarantine, expiry, deletion tombstones, and authorization at grant use time.
- Approval tests cover requester identity, digest substitution, duplicate and
  conflicting decisions, expiry, supersession, Policy revocation,
  cancellation, lease loss, and action execution failure.
