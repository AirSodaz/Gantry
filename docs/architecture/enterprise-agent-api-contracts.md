# Enterprise Agent API Contracts

## 1. Scope and Status

This document is the typed target contract for the server-to-server Enterprise
Agent Invocation API. It complements
[Enterprise Integration API](enterprise-integration-api.md), which explains the
authority model and integration workflow.

The target audience is `gantry-agent-api`; the route base is
`/api/agent/v1`. This contract is not yet a callable public API: it becomes
implemented only when an OpenAPI source, generated clients, handlers,
persistence, integration publication checks, and focused authorization tests
exist. Until then, the existing Admin and Copilot OpenAPI documents remain the
only callable public contracts.

The API has three separate surfaces:

1. Enterprise delegated-user invocation and result projection.
2. Copilot management of a user's own inbound automation triggers.
3. A public HMAC/mTLS-protected inbound hook that accepts external events.

The existing Admin `Webhook Endpoint` remains an outbound delivery resource.
It is not reused for inbound Agent triggers.

## 2. Authority and Scope

```yaml
EnterpriseActor:
  type: object
  required: [client_id, organization_id, subject_principal_id,
             token_exchange_id]
  properties:
    client_id: {type: string}
    organization_id: {type: string}
    subject_principal_id: {type: string}
    token_exchange_id: {type: string}
```

The integration client remains the authenticated caller, but every invocation
also requires a verified delegated subject. A caller-supplied employee ID,
email, or subject field is business input and never establishes identity. A
machine client without a delegated subject cannot create an Enterprise Session;
it must invoke a pre-configured owner-bound Webhook or scheduled trigger.

Every invocation must match an active Integration Publication binding the
client, Workspace, Agent Revision or approved release channel, delegated-subject
policy, input/output contracts, result projection, quotas, and environment. The
caller cannot select a mutable Draft, model route, Tool, credential, runner
image, or network destination.

The effective authority is the intersection of client grants, Publication,
the verified subject's Agent access, Agent Revision, organization/Workspace
Policy, and action-time authorization. The verified subject owns the created
Session, is the first Run requester, and is the only possible approver for that
Run's Agent actions.

## 3. Common Types and Errors

```yaml
PageInfo:
  type: object
  required: [has_more]
  properties:
    next_cursor: {type: string, nullable: true}
    has_more: {type: boolean}

EnterpriseProblem:
  type: object
  required: [code, message, correlation_id, retryable]
  properties:
    code: {type: string}
    message: {type: string}
    correlation_id: {type: string}
    retryable: {type: boolean}
    retry_after_seconds: {type: integer, nullable: true}

DelegatedSubjectPolicy:
  type: object
  required: [token_exchange_required, required_claims]
  properties:
    token_exchange_required: {type: boolean, enum: [true]}
    required_claims: {type: array, items: {type: string}, uniqueItems: true}
    minimum_authentication_strength: {type: string, nullable: true}

EnterpriseAgent:
  type: object
  required: [id, display_name, contract_version, delegated_subject_policy, limits]
  properties:
    id: {type: string}
    display_name: {type: string}
    description: {type: string}
    contract_version: {type: string}
    input_schema: {type: object}
    output_schema: {type: object}
    delegated_subject_policy: {$ref: '#/components/schemas/DelegatedSubjectPolicy'}
    limits: {type: object}
    exposed_artifact_types: {type: array, items: {type: string}}

SourceContext:
  type: object
  properties:
    source_system: {type: string}
    source_resource: {type: object}
    correlation_id: {type: string}

InvocationDelivery:
  type: object
  properties:
    webhook_endpoint_id: {type: string, nullable: true}
    events: {type: array, items: {type: string}}

EnterpriseSession:
  type: object
  required: [id, state, agent, authority, owner_principal_id, current_run,
             created_at]
  properties:
    id: {type: string}
    state: {type: string, enum: [active, archived]}
    agent: {type: object}
    authority: {$ref: '#/components/schemas/EnterpriseActor'}
    owner_principal_id: {type: string}
    current_run:
      type: object
      required: [id, requester_principal_id, status]
      properties:
        id: {type: string}
        requester_principal_id: {type: string}
        status:
          type: string
          enum: [queued, provisioning, running, awaiting_approval, suspended,
                 completed, failed, canceled, expired]
    result: {type: object, nullable: true}
    failure: {type: object, nullable: true}
    created_at: {type: string, format: date-time}
    completed_at: {type: string, format: date-time, nullable: true}
    links: {type: object}

EnterpriseEvent:
  type: object
  required: [event_id, session_id, run_id, sequence, type, occurred_at]
  properties:
    event_id: {type: string}
    session_id: {type: string}
    run_id: {type: string}
    sequence: {type: integer, format: int64}
    type:
      type: string
      enum: [run.accepted, run.running, run.awaiting_approval,
             run.suspended, run.completed, run.failed, run.canceled,
             artifact.available]
    payload: {type: object}
    occurred_at: {type: string, format: date-time}

EnterpriseArtifact:
  type: object
  required: [id, session_id, run_id, filename, media_type, size_bytes, digest,
             state]
  properties:
    id: {type: string}
    session_id: {type: string}
    run_id: {type: string}
    filename: {type: string}
    media_type: {type: string}
    size_bytes: {type: integer, format: int64}
    digest: {type: string, pattern: '^sha256:'}
    state: {type: string, enum: [available, rejected, expired, deleted]}
    download_url: {type: string, format: uri, nullable: true}
    download_url_expires_at: {type: string, format: date-time, nullable: true}

AgentList:
  type: object
  required: [items, page_info]
  properties:
    items: {type: array, items: {$ref: '#/components/schemas/EnterpriseAgent'}}
    page_info: {$ref: '#/components/schemas/PageInfo'}

EventPage:
  type: object
  required: [items, page_info]
  properties:
    items: {type: array, items: {$ref: '#/components/schemas/EnterpriseEvent'}}
    page_info: {$ref: '#/components/schemas/PageInfo'}

DownloadGrant:
  type: object
  required: [artifact_id, download_url, expires_at]
  properties:
    artifact_id: {type: string}
    download_url: {type: string, format: uri}
    expires_at: {type: string, format: date-time}

CreateEnterpriseSessionRequest:
  type: object
  required: [agent_id, contract_version, input]
  properties:
    agent_id: {type: string}
    contract_version: {type: string}
    input: {type: object}
    context: {$ref: '#/components/schemas/SourceContext'}
    attachment_ids: {type: array, uniqueItems: true, items: {type: string}}
    delivery: {$ref: '#/components/schemas/InvocationDelivery'}

RetryRunRequest:
  type: object
  required: [revision_selection]
  properties:
    revision_selection:
      type: string
      enum: [original_revision, current_publication_revision]
```

Enterprise projections never contain raw prompts, chain-of-thought, terminal
streams, credentials, internal Tool payloads, Admin Policy documents, runner
leases, or object-store keys. Output is accepted as `completed` only after it
validates against the published output schema.

After requester approval, a Tool may enter its own business approval workflow.
The Run then uses the normal `suspended` state with
`suspension_reason=external_business_approval`; the signed callback and same-Run
resume contract is defined in
[External Business Approval Callback Contracts](external-business-approval-contracts.md).

## 4. Enterprise Invocation Routes

All routes require the `gantry-agent-api` audience. Durable commands require an
`Idempotency-Key`; cursors are opaque, client-bound, and publication-bound.

| Method and route | Success | Authorization and behavior |
| --- | --- | --- |
| `GET /agents` | `200 AgentList` | Active client/Publication plus verified subject `metadata.read` |
| `GET /agents/{agent_id}` | `200 EnterpriseAgent` | Exact Publication, contract, and subject projection |
| `POST /sessions` | `202 EnterpriseSession`, `200` replay | Verified subject `execute`; creates a personal Session and first Run with the subject as owner/requester |
| `GET /sessions/{session_id}` | `200 EnterpriseSession` | Same client and owner subject; Publication-scoped projection |
| `GET /sessions/{session_id}/events` | `200 EventPage` | Same client/owner; ordered integration projection only |
| `POST /sessions/{session_id}/runs/{run_id}:cancel` | `202 EnterpriseSession`, `200` replay | Same client; verified subject is the Run requester or current Session owner with cancellation authority, which grants no approval authority |
| `POST /sessions/{session_id}/runs/{run_id}:retry` | `202 EnterpriseSession`, `200` replay | Same client/owner; rechecks `execute`, Publication, and revision selection |
| `GET /artifacts/{artifact_id}` | `200 EnterpriseArtifact` | Same client/requester and Publication-exposed Artifact type |
| `POST /artifacts/{artifact_id}:download` | `200 DownloadGrant` | Same client/requester plus fresh scan, retention, and classification checks |

`Prefer: wait=<seconds>` may wait up to the configured maximum for a result;
it never creates a second synchronous execution path. Enterprise Sessions do
not have a message route in this first contract. Interactive continuation
after rejection or expiry remains a Copilot capability.

Common failures are `401 invalid_token`, `403 publication_forbidden`, `403
delegated_subject_required`, `404 resource_not_found`, `409
idempotency_conflict`, `409 session_state_conflict`, `409 revision_unavailable`,
`422 input_schema_invalid`, `429 quota_exceeded`, and `503
dependency_unavailable`. A cross-client resource is represented as `404
resource_not_found`.

## 5. Owner-Bound Automation Triggers

An Automation Trigger is a user-owned, non-shareable configuration resource
that starts a governed Agent workflow without requiring the owner to press a
submit button. Its effects may be visible in a shared Session, but only the
human owner can manage the Trigger or approve its Runs.
Webhook and scheduled variants share one owner/requester boundary; neither is
an application authority mode. The Trigger is distinct from an outbound Admin
Webhook Endpoint and from an enterprise Integration Client.

```yaml
AutomationTrigger:
  type: object
  required: [id, name, kind, owner_principal_id, agent_id, deployment_id, state,
             execution_identity_kind, session_mode, configuration, created_at]
  properties:
    id: {type: string}
    name: {type: string, minLength: 1, maxLength: 120}
    kind: {type: string, enum: [webhook, schedule]}
    owner_principal_id: {type: string}
    agent_id: {type: string}
    deployment_id: {type: string}
    state: {type: string, enum: [active, disabled, expired, quarantined]}
    state_reason:
      type: string
      enum: [manual, owner_authority_lost, agent_unavailable,
             deployment_unavailable, bound_session_unavailable, expired,
             quarantined]
      nullable: true
    execution_identity_kind: {type: string, enum: [service_principal]}
    session_mode: {type: string, enum: [new_session, bound_session]}
    bound_session_id: {type: string, nullable: true}
    configuration:
      oneOf:
        - {$ref: '#/components/schemas/WebhookTriggerConfiguration'}
        - {$ref: '#/components/schemas/ScheduledTriggerConfiguration'}
    policy_snapshot: {type: object}
    expires_at: {type: string, format: date-time, nullable: true}
    created_at: {type: string, format: date-time}
    updated_at: {type: string, format: date-time}
  allOf:
    - oneOf:
        - properties:
            kind: {type: string, enum: [webhook]}
            configuration: {$ref: '#/components/schemas/WebhookTriggerConfiguration'}
        - properties:
            kind: {type: string, enum: [schedule]}
            configuration: {$ref: '#/components/schemas/ScheduledTriggerConfiguration'}
    - oneOf:
        - properties:
            session_mode: {type: string, enum: [new_session]}
          not: {required: [bound_session_id]}
        - required: [bound_session_id]
          properties:
            session_mode: {type: string, enum: [bound_session]}

WebhookTriggerConfiguration:
  type: object
  required: [endpoint, input_schema, key_version]
  properties:
    endpoint: {type: string, format: uri}
    input_schema: {type: object, description: Read-only published Agent input contract snapshot}
    key_version: {type: integer, format: int64, minimum: 1}
    rotated_at: {type: string, format: date-time, nullable: true}
    rate_limit: {type: object}

ScheduledTriggerConfiguration:
  type: object
  required: [expression, time_zone, misfire_policy, dst_overlap_policy,
             fixed_input, schedule_revision, next_planned_at]
  properties:
    expression: {type: string, format: cron-5}
    time_zone: {type: string, format: iana-time-zone}
    misfire_policy: {type: string, enum: [skip]}
    dst_overlap_policy: {type: string, enum: [first]}
    fixed_input: {type: object}
    schedule_revision: {type: integer, format: int64, minimum: 1}
    next_planned_at: {type: string, format: date-time, nullable: true}
    last_planned_at: {type: string, format: date-time, nullable: true}

CreateAutomationTriggerRequest:
  type: object
  required: [name, kind, agent_id, session_mode, configuration]
  properties:
    name: {type: string, minLength: 1, maxLength: 120}
    kind: {type: string, enum: [webhook, schedule]}
    agent_id: {type: string}
    session_mode: {type: string, enum: [new_session, bound_session]}
    bound_session_id: {type: string, nullable: true}
    configuration:
      oneOf:
        - {$ref: '#/components/schemas/CreateWebhookTriggerConfiguration'}
        - {$ref: '#/components/schemas/CreateScheduledTriggerConfiguration'}
    expires_at: {type: string, format: date-time, nullable: true}
  allOf:
    - oneOf:
        - properties:
            kind: {type: string, enum: [webhook]}
            configuration: {$ref: '#/components/schemas/CreateWebhookTriggerConfiguration'}
        - properties:
            kind: {type: string, enum: [schedule]}
            configuration: {$ref: '#/components/schemas/CreateScheduledTriggerConfiguration'}
    - oneOf:
        - properties:
            session_mode: {type: string, enum: [new_session]}
          not: {required: [bound_session_id]}
        - required: [bound_session_id]
          properties:
            session_mode: {type: string, enum: [bound_session]}

CreateWebhookTriggerConfiguration:
  type: object
  properties:
    rate_limit: {type: object}

CreateScheduledTriggerConfiguration:
  type: object
  required: [expression, time_zone, fixed_input]
  properties:
    expression: {type: string, format: cron-5}
    time_zone: {type: string, format: iana-time-zone}
    fixed_input: {type: object}

UpdateAutomationTriggerRequest:
  type: object
  required: [name, session_mode, configuration]
  properties:
    name: {type: string, minLength: 1, maxLength: 120}
    session_mode: {type: string, enum: [new_session, bound_session]}
    bound_session_id: {type: string, nullable: true}
    configuration:
      oneOf:
        - {$ref: '#/components/schemas/CreateWebhookTriggerConfiguration'}
        - {$ref: '#/components/schemas/CreateScheduledTriggerConfiguration'}
    expires_at: {type: string, format: date-time, nullable: true}
  oneOf:
    - properties:
        session_mode: {type: string, enum: [new_session]}
      not: {required: [bound_session_id]}
    - required: [bound_session_id]
      properties:
        session_mode: {type: string, enum: [bound_session]}

TriggerList:
  type: object
  required: [items, page_info]
  properties:
    items: {type: array, items: {$ref: '#/components/schemas/AutomationTrigger'}}
    page_info: {$ref: '#/components/schemas/PageInfo'}

CreateAutomationTriggerResult:
  type: object
  required: [trigger, secret_grant]
  properties:
    trigger: {$ref: '#/components/schemas/AutomationTrigger'}
    secret_grant:
      allOf: [{$ref: '#/components/schemas/SecretGrant'}]
      nullable: true

SecretGrant:
  type: object
  required: [trigger_id, key_version, secret, expires_at]
  properties:
    trigger_id: {type: string}
    key_version: {type: integer, format: int64, minimum: 1}
    secret: {type: string, description: Returned once and never readable again}
    expires_at: {type: string, format: date-time}

RotateTriggerSecretResult:
  type: object
  required: [trigger_id, key_version, secret_grant, rotated_at]
  properties:
    trigger_id: {type: string}
    key_version: {type: integer, format: int64, minimum: 1}
    secret_grant:
      allOf: [{$ref: '#/components/schemas/SecretGrant'}]
      nullable: true
    rotated_at: {type: string, format: date-time}

InboundHookRequest:
  type: object
  required: [event_id, payload]
  properties:
    event_id: {type: string}
    source: {type: object}
    payload: {type: object}
    correlation_id: {type: string, nullable: true}

TriggerOccurrenceReceipt:
  type: object
  required: [trigger_id, occurrence_id, session_id, run_id, session_disposition,
             replayed, received_at]
  properties:
    trigger_id: {type: string}
    occurrence_id: {type: string}
    session_id: {type: string}
    run_id: {type: string}
    session_disposition: {type: string, enum: [created_new, appended_bound]}
    replayed: {type: boolean}
    planned_at: {type: string, format: date-time, nullable: true}
    received_at: {type: string, format: date-time}

TriggerOccurrenceList:
  type: object
  required: [items, page_info]
  properties:
    items:
      type: array
      items: {$ref: '#/components/schemas/TriggerOccurrenceReceipt'}
    page_info: {$ref: '#/components/schemas/PageInfo'}
```

Trigger management is exposed through the user's Copilot projection. A trigger
is an owner-scoped resource rather than a separate authorization domain: the
owner can inspect and manage their own triggers, while creation additionally
requires Agent `execute` permission. The request selects only an employee-
visible Agent. The server resolves its current executable Production Deployment
and records that opaque Deployment identity; Copilot never asks the employee to
choose a Deployment, Revision, Tool, Policy, credential, or Service Principal.

`new_session` is the default product behavior: every accepted occurrence creates
one personal Session and its first Run. `bound_session` appends the occurrence
as a Trigger-authored message to one exact existing Session and creates one
queued Run. Binding requires the Trigger owner to be the Session owner, the
Session to be active and use the same Agent, and the owner to retain Agent
`execute` authority. The request uses `oneOf` semantics: `bound_session_id` is
required only for `bound_session` and forbidden for `new_session`.
The `kind` discriminator must match its `configuration` variant. Webhook input
is validated against the selected Agent's published input contract snapshot;
the employee does not author a JSON Schema. Scheduled `fixed_input` is rendered
and validated from the same contract when the Trigger is created or updated.

| Method and route | Success | Required capability |
| --- | --- | --- |
| `GET /automation-triggers` | `200 TriggerList` | Authenticated owner |
| `POST /automation-triggers` | `201 CreateAutomationTriggerResult`, `200` replay | Authenticated owner, Agent `execute`; first Webhook response returns its secret once |
| `GET /automation-triggers/{id}` | `200 AutomationTrigger` plus `ETag` | Owner |
| `GET /automation-triggers/{id}/occurrences` | `200 TriggerOccurrenceList` | Owner; committed occurrence projection only |
| `PATCH /automation-triggers/{id}` | `200 AutomationTrigger` | `UpdateAutomationTriggerRequest`; owner, `If-Match` |
| `POST /automation-triggers/{id}:disable` | `200 AutomationTrigger` | Owner |
| `POST /automation-triggers/{id}:enable` | `200 AutomationTrigger` | Owner, Agent still executable |
| `POST /automation-triggers/{id}:rotate-secret` | `200 RotateTriggerSecretResult` | Owner; Webhook only; atomically invalidates the previous key version and returns the new secret once |

`secret_grant` is non-null only when a Webhook is created. The secret is never
returned by list, detail, replay, or refresh. The occurrence list contains only
committed occurrences and their Session/Run links; signature failures and
uncommitted rejected requests remain redacted security/Audit evidence rather
than an employee-visible alternate Audit log.

Trigger creation requires an `Idempotency-Key`. If the first Webhook response is
lost, replay returns the committed Trigger with `secret_grant=null`; the owner
must rotate the secret rather than recovering the original plaintext. List
filters are limited to kind, state, and Agent and always include owner scope in
the storage query.

Update replaces the complete editable projection rather than merging arbitrary
JSON. Agent and Trigger kind are immutable; changing either requires a new
Trigger. The configuration variant must match the existing kind. Enable,
disable, rotation, and creation use idempotency keys; update uses the resource
ETag and rejects stale state.

Secret rotation has no overlap window. Its transaction increments the key
version, activates the new secret reference, and irreversibly retires the
previous version before returning success. The first response contains a
non-null `secret_grant`; replay with the same idempotency key returns the same
Trigger/key-version result with `secret_grant=null`. If that first response is
lost, the owner performs another rotation with a new idempotency key.

The external event endpoint is separate and does not accept OAuth user tokens:

```text
POST /api/hooks/v1/{trigger_id}
X-Gantry-Hook-Timestamp: <unix-seconds>
X-Gantry-Hook-Event-Id: <stable-event-id>
X-Gantry-Hook-Signature: sha256=<hmac>
Content-Type: application/json
```

The signature covers the timestamp, event ID, and exact request bytes. The
server requires the header event ID to equal the body `event_id` and rejects an
expired timestamp, invalid signature, or oversized body before entering the
occurrence transaction. These transport failures cannot reserve or poison an
event ID.

Inside the transaction, Gantry first loads `(trigger_id, event_id)`. An existing
occurrence with the same canonical request digest returns its original receipt
with `replayed=true`, even if the Trigger was later disabled or owner authority
changed; a different digest returns `409 idempotency_conflict`. When no
occurrence exists, the transaction checks current Trigger state, input schema,
owner authority, and quota before claiming the ID and creating the Session
Message and Run. A failed new-occurrence check commits no claim, so a corrected
request may reuse the still-unaccepted event ID.

After secret rotation commits, a request signed with the retired key returns
`401 invalid_signature` and creates no occurrence claim. If the event was not
previously accepted, the caller may sign the same bytes and stable event ID with
the new key and retry. If it was already accepted, valid new-key authentication
reaches the existing occurrence and returns its original receipt.

Every accepted event creates or targets a Session according to `session_mode`,
appends one immutable Trigger-authored Message, and creates one ordinary queued
Run. The trigger owner's principal is the Run `requester_id`; the fixed Service
Principal is its execution identity. The Run freezes the exact Deployment and
Policy snapshot and carries the correlation ID and `webhook` source tag. It does
not introduce a separate Run, approval, or queue type.

The hook payload cannot select or override a Session, Agent, Deployment,
Revision, requester, execution identity, credential, Tool, model, destination,
or approval decision. The trigger owner is the Run requester for every
occurrence and receives any ordinary Agent-action approval in Copilot. The
webhook request never approves an action, and the Service Principal execution
identity does not replace the requester for approval purposes.

For a bound Session, occurrences are ordered by Session sequence. At most one
Run executes in that Session; later distinct occurrences remain queued until the
current Run reaches a safe terminal boundary. An approval or external business
wait continues to hold the execution slot. The system never falls back to a new
Session merely because the bound Session is busy.

A scheduled Trigger uses a five-field POSIX-style cron expression with minute
granularity and one canonical IANA time zone. Seconds, year fields, `@` aliases,
fixed numeric offsets, and command suffixes are invalid. The initial and only
misfire policy is `skip`: restart, re-enable, or outage recovery does not create
Runs for earlier uncommitted planned instants. An occurrence committed before a
failure is retried from its outbox and is not a misfire.

When daylight saving moves the clock forward, a nonexistent local time is
skipped. When it moves backward, a repeated local time creates one occurrence
at the first corresponding UTC instant. Each due time derives one stable
occurrence ID from Trigger ID, `schedule_revision`, and planned UTC instant.
Changing the expression, time zone, fixed input, Deployment, or Session target
increments the revision and recomputes `next_planned_at`; it never rewrites an
older occurrence. The occurrence, Session Message, queued Run, receipt, Audit,
next planned instant, and outbox record commit atomically.

Scheduled and Webhook occurrences use the same `new_session` or
`bound_session` behavior, Session serialization, owner requester, and fixed
Service Principal rules. A scheduled Run carries source tag `schedule`. The
rationale and rejected alternatives are recorded in
[ADR-038](../governance/adr/adr-038-scheduled-trigger-time-semantics.md).

## 6. Trigger Ownership and Lifecycle

The owner controls creation, update, enable/disable, rotation, and expiry of
their own triggers without a separate Automation capability. The owner must
have Agent `execute` permission at creation time; the server rechecks the Agent
ACL, Deployment, Policy, quarantine, Workspace, and quota at every occurrence.
Removing `execute`, disabling a Deployment, quarantining an Agent, or revoking
the owner disables new occurrence acceptance without deleting historical
Sessions or Runs. Archiving a bound Session disables new occurrences for that
Trigger; Gantry does not silently redirect them. Existing Runs retain the owner
as requester and follow the normal approval and continuation lifecycle.

The first release does not support shared Trigger management or
`automation.invoke`.
There is no user-selected Deny rule and no trigger that points at a mutable
Draft. Trigger secrets are stored as references, shown only once on initial
creation or rotation, and never appear in Session, Run, Audit, event, or error
payloads.
Trigger `kind` is immutable. Updating execution-affecting scheduled fields uses
`If-Match`, increments `schedule_revision`, and recomputes the next planned
instant; enabling a schedule performs the same current-authorization check and
starts from the first future matching instant.

## 7. Delivery, Audit, and Recovery

Accepted hook requests atomically persist the inbound event, canonical request
digest, occurrence result, Session Message, queued Run, and outbox record before
returning success. The HTTP response does not wait for Agent execution. The
event record contains trigger ID, owner,
signature key version, event ID, digest, source metadata, correlation ID,
authorization result, and resulting Session and Run IDs. A database uniqueness
constraint on `(trigger_id, event_id)` prevents duplicate queueing across
concurrent HTTP requests, worker retries, and process restarts.

Hook delivery is at-least-once from the caller's perspective and exactly-once
with respect to Session Message and Run creation for the stable event ID within
the retention window. Replays after that window require a new event ID and are
subject to trigger expiry and policy. A committed occurrence is never queued a
second time by an HTTP retry, outbox retry, scheduler retry, or worker restart.

Trigger creation, update, rotation, enable/disable, accepted/rejected hook
events, scheduled occurrences, quota blocks, signature failures, and resulting
Session/Run linkage emit canonical Audit evidence. Recovery emits one bounded
summary containing the skipped time window and newly computed next instant; it
does not enumerate one Audit event per missed instant. Audit never stores the
trigger secret, raw credential, or unredacted fixed input. A rotation event
records actor, Trigger, retired and activated key-version numbers, immediate
retirement policy, command outcome, and time without either secret value.
Trigger deletion is represented by disable plus retention/tombstone processing
so historical Session, Run, and Audit references remain verifiable.

## 8. Acceptance Contract

- Enterprise API schemas and routes are represented in a future
  `packages/contracts/openapi/agent-api.yaml` before the API is exposed.
- Copilot trigger management uses the same requester-safe response and error
  projection as other Copilot resources.
- A user cannot create a trigger for an Agent they cannot execute or a mutable
  Draft, and cannot use the trigger to broaden runtime authority.
- HMAC/mTLS verification, timestamp/replay protection, event-id idempotency,
  schema validation, rate limits, and quarantine are tested independently.
- Secret rotation atomically activates one new key version and invalidates the
  previous version with no overlap. First responses reveal the secret once;
  command replay is redacted and a lost response requires another rotation.
- A rejected signature, retired key, invalid schema, disabled Trigger, failed
  owner authorization, or quota denial does not claim the event ID. A corrected
  request may reuse that still-unaccepted stable ID.
- A hook restart or duplicate delivery creates at most one Session Message and
  Run and preserves the original response for the idempotency window.
- A webhook-created Run can enter `awaiting_approval`; the trigger owner is
  the requester who may approve or reject it through the normal Copilot
  approval flow. The webhook request itself never synthesizes an approval.
- Enterprise Session creation without a verified delegated subject is rejected;
  a client credential or runtime Service Principal never becomes requester.
- A scheduled occurrence uses validated five-field cron and an IANA time zone,
  skips missed and nonexistent local instants, runs a repeated local instant
  once, and uses the same owner authorization, ordinary Session, Run, approval,
  Audit, and idempotency rules as a Webhook occurrence.
