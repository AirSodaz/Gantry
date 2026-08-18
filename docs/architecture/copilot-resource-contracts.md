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

- Agent discovery and preference reads require `metadata.read` and an active
  Deployment in an allowed Workspace. Creating a Session or submitting an
  instruction additionally requires `execute`; catalog visibility never
  implies execution authority.
- Session reads require current membership and applicable Workspace,
  classification, and retention policy. Membership uses the fixed roles
  `owner`, `contributor`, and `viewer`; it never grants Agent configuration or
  execution authority.
- Each accepted instruction creates a Run whose immutable requester is the
  authenticated principal, or the human owner of an authorized Trigger. Only
  that Run requester may decide its Agent-action approval. Session ownership,
  other membership, Admin roles, operator access, and business approver roles
  do not grant this command.
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
- Session responses return a conversation `ETag`. Appending a message
  requires `If-Match`; stale state returns `409 conversation_changed` with the
  current Session projection and no new Message or Run.
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

User-owned inbound automation trigger schemas and routes are defined in
[Enterprise Agent API Contracts](enterprise-agent-api-contracts.md). They use
the Copilot audience for management and a separate signed hook endpoint for
external events.

### 4.1 Agent catalog

```yaml
CatalogInputExample:
  type: object
  required: [label, description]
  properties:
    label: {type: string, maxLength: 120}
    description: {type: string, maxLength: 500}
    required: {type: boolean, default: false}

ExpectedOutput:
  type: object
  required: [kind, description]
  properties:
    kind: {type: string, enum: [text, structured, artifact, mixed]}
    description: {type: string, maxLength: 500}
    schema: {type: object, nullable: true}

DataDisclosure:
  type: object
  required: [input_classifications, output_classifications, summary]
  properties:
    input_classifications: {type: array, items: {type: string}, uniqueItems: true}
    output_classifications: {type: array, items: {type: string}, uniqueItems: true}
    summary: {type: string, maxLength: 1000}
    retention_notice: {type: string, maxLength: 500, nullable: true}

ActionDisclosure:
  type: object
  required: [effect_level, summary, approval_behavior]
  properties:
    effect_level: {type: string, enum: [none, read_only, writes_or_external_effects]}
    summary: {type: string, maxLength: 1000}
    approval_behavior: {type: string, enum: [never, may_be_requested, always_before_effect]}

PublishedCatalogMetadata:
  type: object
  required: [typical_inputs, expected_output, capability_summary,
             data_disclosure, action_disclosure]
  properties:
    typical_inputs: {type: array, maxItems: 8, items: {$ref: '#/components/schemas/CatalogInputExample'}}
    expected_output: {$ref: '#/components/schemas/ExpectedOutput'}
    capability_summary: {type: string, maxLength: 1000}
    data_disclosure: {$ref: '#/components/schemas/DataDisclosure'}
    action_disclosure: {$ref: '#/components/schemas/ActionDisclosure'}

PublicationAvailability:
  type: object
  required: [state]
  properties:
    state: {type: string, enum: [available, temporarily_unavailable]}
    reason_code: {type: string, enum: [maintenance, dependency, policy, capacity], nullable: true}
    message: {type: string, maxLength: 500, nullable: true}
    effective_until: {type: string, format: date-time, nullable: true}

CopilotAgent:
  type: object
  required: [id, display_name, description, category, owner, input_contract,
             published_metadata, availability, is_favorite]
  properties:
    id: {type: string}
    display_name: {type: string}
    description: {type: string}
    category: {type: string}
    owner: {$ref: '#/components/schemas/SupportContact'}
    input_contract: {type: object, description: Published structured-input JSON Schema}
    published_metadata: {$ref: '#/components/schemas/PublishedCatalogMetadata'}
    availability: {$ref: '#/components/schemas/PublicationAvailability'}
    is_favorite: {type: boolean}
    last_used_at: {type: string, format: date-time, nullable: true}
```

This projection is produced from an exact active Deployment and published
metadata. Stable identity fields (`display_name`, `description`, `category`, and
owner) come from the Agent identity; `PublishedCatalogMetadata` is authored in
the Draft and frozen into the immutable Revision. `PublicationAvailability` is
owned by the active Deployment and may temporarily hide an otherwise published
Agent. Neither source exposes Revision hashes, prompts, model routes, Tool
bindings, Policy rules, or credential references.

### 4.1.1 Agent preferences

```yaml
AgentPreference:
  type: object
  required: [agent_id, workspace_id, is_favorite]
  properties:
    agent_id: {type: string}
    workspace_id: {type: string}
    is_favorite: {type: boolean}
    last_used_at: {type: string, format: date-time, nullable: true}
```

Preferences are principal-owned and keyed by `(principal_id, workspace_id,
agent_id)`. The server derives both the principal and Workspace from the
authorized Agent; request JSON cannot select another subject or Workspace.
`last_used_at` is updated only after the corresponding `POST /sessions` commits
successfully. Opening a catalog row, validation failure, idempotency conflict,
or a rejected submission is not recent use. The server keeps the eight most
recent successfully submitted Agents per requester and Workspace; favorite
rows are retained even when they fall outside that recent window. A later
successful use moves an Agent to the front and pruning never removes its
favorite flag.

Favorite mutation requires the same `metadata.read` catalog access as Agent
discovery and is idempotent. It never grants metadata, execution, or ACL
capabilities and cannot make a temporarily unavailable publication usable.

### 4.2 Session, membership, Message, and Run

```yaml
SessionMode:
  type: string
  enum: [personal, shared, channel]

SessionState:
  type: string
  enum: [active, archived]

SessionMemberRole:
  type: string
  enum: [owner, contributor, viewer]

RunState:
  type: string
  enum: [queued, provisioning, running, awaiting_approval, suspended, canceling,
         completed, failed, canceled, expired]

Session:
  type: object
  required: [id, owner_principal_id, mode, agent, state,
             conversation_revision, queued_run_count, members, messages,
             created_at, updated_at]
  properties:
    id: {type: string}
    owner_principal_id: {type: string}
    mode: {$ref: '#/components/schemas/SessionMode'}
    source_tags:
      type: array
      uniqueItems: true
      items: {type: string, enum: [webhook, schedule, channel]}
    agent: {$ref: '#/components/schemas/SessionAgentSnapshot'}
    title: {type: string, nullable: true}
    state: {$ref: '#/components/schemas/SessionState'}
    conversation_revision: {type: integer, format: int64, minimum: 1}
    my_action: {type: string, enum: [none, approval]}
    executing_run: {$ref: '#/components/schemas/RunSummary', nullable: true}
    queued_run_count: {type: integer, minimum: 0}
    members: {type: array, items: {$ref: '#/components/schemas/SessionMember'}}
    messages: {type: array, items: {$ref: '#/components/schemas/SessionMessage'}}
    artifacts: {type: array, items: {$ref: '#/components/schemas/Artifact'}}
    created_at: {type: string, format: date-time}
    updated_at: {type: string, format: date-time}

SessionMember:
  type: object
  required: [principal_id, role, joined_at]
  properties:
    principal_id: {type: string}
    display_name: {type: string}
    role: {$ref: '#/components/schemas/SessionMemberRole'}
    joined_at: {type: string, format: date-time}

SessionAgentSnapshot:
  type: object
  required: [agent_id, display_name]
  properties:
    agent_id: {type: string}
    display_name: {type: string}
    support_contact: {$ref: '#/components/schemas/SupportContact'}

SessionMessage:
  type: object
  required: [id, session_sequence, author_kind, parts, created_at]
  properties:
    id: {type: string}
    run_id: {type: string, nullable: true}
    session_sequence: {type: integer, format: int64, minimum: 1}
    author_kind: {type: string, enum: [principal, trigger, agent, system_summary]}
    author_principal_id: {type: string, nullable: true}
    trigger_id: {type: string, nullable: true}
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
  required: [id, session_sequence, requester_id, state, created_at]
  properties:
    id: {type: string}
    session_sequence: {type: integer, format: int64, minimum: 1}
    requester_id: {type: string}
    state: {$ref: '#/components/schemas/RunState'}
    outcome:
      type: string
      enum: [succeeded, requester_input_required, failed, canceled, expired]
      nullable: true
    state_reason: {$ref: '#/components/schemas/UserFacingReason'}
    retry_of_run_id: {type: string, nullable: true}
    trigger_occurrence_id: {type: string, nullable: true}
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

The Session remains active across successful, failed, rejected, or expired Runs
until its owner archives it. Earlier Run summaries remain immutable after
reaching a terminal state. Any authorized contributor may append a new
instruction while another Run is active; the command atomically records the
Message and creates a queued Run. A Session has at most one executing Run, and
queued Runs start in Session-sequence order after the prior Run reaches a safe
terminal boundary. A retry is a new queued Run with `retry_of_run_id` and the
authenticated retrying principal as requester.

Session mode is server-derived rather than an arbitrary client flag. A newly
created owner-only Session is `personal`; adding another human member makes it
`shared`; an active verified external channel binding makes it `channel`.
Removing the last non-owner member may return an unbound Session to `personal`.
Channel binding details require their own connector contract and cannot be
asserted by ordinary Session JSON.

`source_tags` are presentation and traceability metadata only. A Trigger may
create a new Session or append to one exact owner-bound Session. The resulting
Run uses the same approval, Artifact, event, cancellation, and retention
contracts as a human-submitted Run; the Trigger owner is its requester.

### 4.3 Approval

```yaml
CopilotApproval:
  type: object
  required: [id, session_id, run_id, requester_id, action_id, action_digest,
             approval_revision, state, preview, expires_at, created_at]
  properties:
    id: {type: string}
    session_id: {type: string}
    run_id: {type: string}
    requester_id: {type: string}
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
    bound_session_id: {type: string, nullable: true}
    created_at: {type: string, format: date-time}
    expires_at: {type: string, format: date-time}

Artifact:
  type: object
  required: [id, session_id, run_id, filename, media_type, size_bytes, digest,
             classification, state, scan_state, created_at]
  properties:
    id: {type: string}
    session_id: {type: string}
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
are never persisted in a Session projection. Attachment bytes remain quarantined
until size, digest, media type, classification, malware, and policy checks pass.
An instruction command atomically binds only uploader-owned `available`
Attachments; a bound Attachment cannot be rebound or replaced. Expiry or
deletion retains a digest-bearing tombstone when evidence policy requires it.

### 4.5 Session event stream

```yaml
SessionEventSnapshot:
  type: object
  required: [schema_version, session, runs, approvals, cursor]
  properties:
    schema_version: {type: string, enum: ['gantry.copilot.snapshot/v1']}
    session: {$ref: '#/components/schemas/Session'}
    runs: {type: array, items: {$ref: '#/components/schemas/RunSummary'}}
    approvals: {type: array, items: {$ref: '#/components/schemas/CopilotApproval'}}
    cursor: {type: string}

SessionEventFrame:
  type: object
  required: [schema_version, session_id, session_sequence, cursor, event]
  properties:
    schema_version: {type: string, enum: ['gantry.copilot.event/v1']}
    session_id: {type: string}
    run_id: {type: string, nullable: true}
    session_sequence: {type: integer, format: int64, minimum: 1}
    run_sequence: {type: integer, format: int64, nullable: true}
    cursor: {type: string}
    event:
      oneOf:
        - {$ref: '#/components/schemas/MessageCommittedEvent'}
        - {$ref: '#/components/schemas/ContentSegmentEvent'}
        - {$ref: '#/components/schemas/RunStateChangedEvent'}
        - {$ref: '#/components/schemas/SessionChangedEvent'}
        - {$ref: '#/components/schemas/ApprovalChangedEvent'}
        - {$ref: '#/components/schemas/ArtifactChangedEvent'}
      discriminator: {propertyName: type}

MessageCommittedEvent:
  type: object
  required: [type, message]
  properties:
    type: {type: string, enum: [message_committed]}
    message: {$ref: '#/components/schemas/SessionMessage'}

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

SessionChangedEvent:
  type: object
  required: [type, state, mode, conversation_revision, queued_run_count]
  properties:
    type: {type: string, enum: [session_changed]}
    state: {$ref: '#/components/schemas/SessionState'}
    mode: {$ref: '#/components/schemas/SessionMode'}
    conversation_revision: {type: integer, format: int64, minimum: 1}
    queued_run_count: {type: integer, minimum: 0}
    members: {type: array, items: {$ref: '#/components/schemas/SessionMember'}}

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

SessionEventTicket:
  type: object
  required: [ticket, session_id, websocket_url, expires_at]
  properties:
    ticket: {type: string, description: Returned only when the ticket is created}
    session_id: {type: string}
    websocket_url: {type: string, format: uri}
    expires_at: {type: string, format: date-time}
```

The target cursor is opaque, Session-bound, member-bound, projection-bound,
and filter-bound. `session_sequence` orders durable employee-visible changes across
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

SessionList:
  type: object
  required: [items, page_info]
  properties:
    items: {type: array, items: {$ref: '#/components/schemas/Session'}}
    page_info: {$ref: '#/components/schemas/PageInfo'}

SessionMemberList:
  type: object
  required: [items, page_info]
  properties:
    items: {type: array, items: {$ref: '#/components/schemas/SessionMember'}}
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

CreateSessionRequest:
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

SetAgentFavoriteRequest:
  type: object
  required: [is_favorite]
  properties:
    is_favorite: {type: boolean}

AppendSessionMessageRequest:
  type: object
  required: [message]
  properties:
    message: {type: string, minLength: 1}
    attachment_ids: {type: array, uniqueItems: true, items: {type: string}}

AddSessionMemberRequest:
  type: object
  required: [principal_id, role]
  properties:
    principal_id: {type: string}
    role: {type: string, enum: [contributor, viewer]}

UpdateSessionMemberRequest:
  type: object
  required: [role]
  properties:
    role: {type: string, enum: [contributor, viewer]}

TransferSessionOwnerRequest:
  type: object
  required: [new_owner_principal_id]
  properties:
    new_owner_principal_id: {type: string}

RetryRunRequest:
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

`CreateSessionRequest` accepts exactly one of `message` and `structured_input`.
The selected Agent's published input contract may further constrain the value.

### 5.2 Routes

All routes are relative to `/api/copilot/v1`.

| Method and route | Request or filters | Success | Required rule |
| --- | --- | --- | --- |
| `GET /agents` | cursor, limit, search, category, collection=`all\|favorites\|recent` | `200 CopilotAgentList` | `metadata.read`, active Deployment, and Workspace scope; `favorites` and `recent` are requester-scoped |
| `PUT /agents/{agent_id}/favorite` | `SetAgentFavoriteRequest`; `Idempotency-Key` | `200 CopilotAgent` | `metadata.read`; requester-owned preference only; does not alter Agent authorization or availability |
| `POST /attachments` | metadata; `Idempotency-Key` | `201 AttachmentUploadGrant`, `200` replay | Creates requester-owned quarantine record |
| `GET /attachments/{id}` | none | `200 Attachment` | Requester only; no upload secret |
| `PUT /attachments/{id}/content` | bytes and upload token | `204` | Exact length limit and one active grant |
| `POST /attachments/{id}:complete` | `Idempotency-Key` | `202 Attachment`, `200` replay | Verifies digest and schedules scan |
| `POST /sessions` | Agent, message or structured input, Attachment IDs; `Idempotency-Key` | `201 Session`, `200` replay | Creates a personal Session and first queued Run; `execute` plus active Deployment; updates recent use after commit |
| `GET /sessions` | cursor, limit, state, mode, Agent, my action, time | `200 SessionList` | Membership is mandatory in the storage query |
| `GET /sessions/{id}` | none | `200 Session` plus `ETag` | Member-authorized conversation projection |
| `POST /sessions/{id}/messages` | message and optional Attachments; `If-Match`, `Idempotency-Key` | `201 Session`, `200` replay | Owner/contributor plus current Agent `execute`; creates one queued Run |
| `GET /sessions/{id}/members` | cursor, limit | `200 SessionMemberList` | Current member |
| `POST /sessions/{id}/members` | `AddSessionMemberRequest`; `If-Match`, `Idempotency-Key` | `201 Session` | Owner only; Workspace and data-policy checks |
| `PATCH /sessions/{id}/members/{principal_id}` | `UpdateSessionMemberRequest`; `If-Match`, `Idempotency-Key` | `200 Session` | Owner only; owner role is not assignable here |
| `DELETE /sessions/{id}/members/{principal_id}` | `If-Match`, `Idempotency-Key` | `200 Session` | Owner only; cannot remove owner |
| `POST /sessions/{id}:transfer-owner` | `TransferSessionOwnerRequest`; `If-Match`, `Idempotency-Key` | `200 Session` | Current owner only; target must be an eligible current contributor |
| `POST /sessions/{id}:archive` | `If-Match`, `Idempotency-Key` | `200 Session` | Owner only; blocks new instructions and Trigger occurrences |
| `GET /sessions/{id}/runs` | cursor, limit | `200 RunSummaryList` | Compact member projection |
| `POST /sessions/{id}/runs/{run_id}:cancel` | `Idempotency-Key` | `202 RunSummary`, `200` terminal/replay | Run requester or current Session owner; cancellation may reconcile |
| `POST /sessions/{id}/runs/{run_id}:retry` | revision selection; `If-Match`, `Idempotency-Key` | `201 RunSummary`, `200` replay | Authorized contributor; creates a queued Run and records the caller as requester |
| `POST /sessions/{id}/events:ticket` | optional last cursor | `200 SessionEventTicket` | Short-lived, Session- and member-bound |
| `GET /approvals` | cursor, limit, state | `200 ApprovalList` | Requester-bound; pending is default filter |
| `GET /approvals/{id}` | none | `200 CopilotApproval` | Projects effective expiry; worker owns durable transition |
| `POST /approvals/{id}:decide` | decision, reason, digest, revision; `Idempotency-Key` | `200 CopilotApproval` | Requester only; decision is not execution |
| `GET /artifacts` | cursor, limit, Session, classification, state | `200 ArtifactList` | Session membership and classification policy |
| `GET /artifacts/{id}` | none | `200 Artifact` | Metadata only |
| `POST /artifacts/{id}:download` | none | `200 ArtifactDownloadGrant` | Rechecks auth, scan, retention; audited access |

The checked-in contract uses the explicit audited Artifact download command.
Approval decisions use the common `Idempotency-Key` command header; the request
body carries only the decision evidence for the exact action.

## 6. State and Command Semantics

### 6.1 Session creation and instructions

Session creation performs one transaction: claim idempotency, authorize the
Agent, resolve the active Deployment, validate and bind Attachments, create the
personal Session and owner membership, append the first principal Message,
create the first queued Run with that principal as requester, append Session/Run
events, and enqueue scheduling through the outbox. Failure before commit creates
none of these resources.

Appending an instruction uses the Session conversation ETag, current membership,
and current Agent `execute` authorization. It records one immutable Message and
one queued Run in the same transaction. One executing Run owns the Session's
runtime slot; queued Runs are selected strictly by Session sequence. A Run that
is awaiting Agent-action or external business approval continues to own the
slot so later instructions cannot overtake its context.

Owner transfer atomically changes the one owner membership, demotes the previous
owner to contributor, appends Session and Audit evidence, and disables every
bound Trigger whose owner no longer owns the Session. Trigger ownership is never
transferred implicitly.

A rejected, expired, superseded, or revoked action approval produces a durable
structured action result. After the Agent reaches a safe input boundary, the
Run ends with durable requester-input-required evidence and the Session composer
remains usable. A later instruction never changes the denied action or reuses
its approval; any newly proposed effect has
a new action ID, digest, Policy decision, and approval if required.

### 6.2 Cancel and retry

The Run requester or current Session owner may cancel. This is restrictive
authority and does not grant approval, retry, membership, or Agent execution
authority. Canceling a queued Run atomically marks it canceled and removes it
from scheduling order. For an executing Run, cancellation wins only before an
execution permit is consumed. After that point, the UI shows `canceling` until
the effect is observed or reconciled and must not claim that no external action
occurred. Duplicate cancellation returns the winning state.

Retry creates a new queued Run under the same Session. Selecting the original
Revision pins that immutable Revision; selecting the current Production Revision
requires an explicit field and current execution authorization. Prior Messages,
Runs, approvals, and Artifacts remain evidence and are not rewritten.

### 6.3 Approval races

Decision, expiry, cancellation, action supersession, Policy revocation, and
lease loss compete through expected state and revision. Exactly one terminal
approval transition wins. A duplicate decision returns the winning evidence;
a late conflicting decision returns `409 approval_changed`. Even an approved
action must pass action-time checks and atomically consume a single-use permit.

### 6.4 Catalog publication and preferences

An Admin Draft editor with `draft.edit` may update the employee-facing catalog
metadata alongside the Agent configuration. Validation requires bounded,
plain-language disclosures, a valid structured-input JSON Schema when one is
declared, and data classifications that are no broader than the bound
Attachments, Tools, and Policy maximums. Committing the Draft copies the
canonical metadata into the Revision digest; later Draft edits cannot change a
published projection.

Production publication and test Deployment creation bind an exact Revision.
The Deployment may set a user-facing temporary availability state and bounded
reason, but cannot replace Revision metadata or widen its classifications. A
retired, revoked, expired, or unavailable Deployment is omitted from ordinary
catalog results or returned with `temporarily_unavailable` only when the
employee already has a stable catalog link; it remains non-submittable in both
cases. The server rechecks availability during Session creation and every new
instruction.

`GET /agents?collection=favorites` filters the authorized catalog by the
requester's favorite rows. `collection=recent` orders the authorized catalog by
`last_used_at` and returns at most eight rows. Search and category filters are
intersections with the collection, never alternate authorization paths.

The successful Session-creation transaction and the preference update share one
outbox event. A failed or replayed submission does not create a second recent
use; a same-key replay returns the original Session and leaves the original
timestamp unchanged. Preference writes are audited as ordinary access-control
telemetry, while metadata publication and Deployment availability changes use
the Agent lifecycle Audit stream.

### 6.5 External business approval waits

After the requester approves an exact Agent action, the invoked Tool may return
a typed external business-approval wait. The same Run moves to `suspended` with
`state_reason.code=external_business_approval` and `next_action=wait`. Copilot
shows the owning external system and a bounded message, but no Gantry approval
controls. The Run requester or Session owner may still cancel the Run.

A signed, idempotent callback records the Tool-owned business decision and
resumes the same Run for the next Agent loop. It never reuses the prior Gantry
approval or Tool execution permit. Every later consequential action receives a
new digest and current requester/Policy decision. The callback and recovery
contract is defined in
[External Business Approval Callback Contracts](external-business-approval-contracts.md).

## 7. Realtime, Reconnect, and Recovery

After creating a ticket, the client connects to
`WSS /api/copilot/v1/sessions/{session_id}/events?ticket={ticket}&after={cursor}`.
The WebSocket endpoint accepts a short-lived ticket and optional opaque cursor.
The server first returns either a snapshot or the next durable frame, then live
frames and heartbeats. Clients acknowledge only rendered durable cursors.

- A normal reconnect requests a new ticket and resumes after the last rendered
  cursor. Duplicate `session_sequence` values are ignored.
- `cursor_expired` includes a current `SessionEventSnapshot`, a replacement cursor,
  and a reason such as retention expiry or projection-version change. The
  client replaces its local projection atomically before resuming.
- `cursor_invalid` covers a different Session, member, or filter and returns no
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
  availability independently of historical Session and event evidence.
- Read access is re-authorized on every request. A Session being visible earlier
  does not create a permanent browser entitlement.
- Catalog metadata is employee-safe by construction. The input contract is a
  published structured-input schema, not an Admin Draft or prompt snapshot;
  availability messages are bounded and may not contain Policy, Tool, model,
  credential, or runner identifiers.
- Artifact download and approval decision emit canonical Audit events. Normal
  list/detail reads contribute access telemetry but do not create alternate
  resource-specific Audit stores.

## 9. Acceptance and Verification Contract

- Generated Copilot types contain no Admin-only runner, credential, Policy,
  prompt, raw Tool argument, or fields from Sessions where the actor is not a
  member.
- Tests prove non-leaking `404` behavior for every direct resource route and
  authorization inside list queries.
- Session creation, instruction append, membership, archive, cancel, retry,
  Attachment completion, and approval decision
  cover replay, conflicting key, stale precondition, and concurrent command
  cases.
- Rejection and expiry preserve Session conversation input; the next instruction
  creates exactly one queued Run and never replays the denied action.
- Session-level streams cross Run boundaries, deduplicate durable frames, replace
  snapshots on cursor expiry, and recover after process restart.
- Attachment and Artifact tests cover size/digest mismatch, scan failure,
  quarantine, expiry, deletion tombstones, and authorization at grant use time.
- Catalog tests prove Revision-frozen metadata, Deployment-bound availability,
  non-leaking projections, Workspace intersection, favorite idempotency, and
  recent-use pruning after successful Session creation only.
- Approval tests cover requester identity, digest substitution, duplicate and
  conflicting decisions, expiry, supersession, Policy revocation,
  cancellation, lease loss, and action execution failure.
