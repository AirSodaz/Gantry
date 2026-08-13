# Data and Event Model

## 1. Modeling Principles

- Configuration is versioned; published execution inputs are immutable.
- Commands express intent, events record accepted facts.
- Relational projections provide current state; events provide history.
- Large or binary content lives in object storage and is referenced by digest.
- Every resource is scoped to an organization and, where applicable, a
  workspace.
- Retrying creates a new attempt rather than mutating the previous result.
- Secrets are referenced, never embedded in specifications or events.

## 2. Identity and Scope Entities

### Organization

Represents one installed enterprise organization. Key fields include `id`,
`slug`, `display_name`, `status`, `default_retention_policy_id`, and timestamps.

### Workspace

Provides a logical ownership and policy boundary. Key fields include `id`,
`organization_id`, `slug`, `display_name`, `classification`, `status`, and
`policy_set_id`.

### Principal

Represents a user or service account. It stores the stable external subject,
principal type, display metadata, and lifecycle state. Group and role mappings
are modeled separately and retain their source.

### Role Binding

Binds a principal or group to a role at organization, workspace, agent, or
approval scope. A binding has validity timestamps and provenance.

### Integration Client

Represents a registered enterprise application and environment. It records the
OAuth client reference, owner, status, allowed authority modes, scopes, quotas,
data classification, credential metadata, and operational contacts. Secret or
private-key material remains in the identity or secret system.

## 3. Agent Configuration Entities

### Agent

Mutable identity and ownership record:

- `id`, `organization_id`, `workspace_id`
- `slug`, `display_name`, `description`, `category`
- `owner_principal_id`, `lifecycle_status`
- `current_published_version_id`
- creation and update metadata

### Agent Draft

Editable specification with optimistic concurrency:

- `id`, `agent_id`, `revision`
- `spec_json`, `schema_version`
- `validation_status`, `validation_findings`
- `updated_by`, `updated_at`

### Agent Version

Immutable publication candidate or release:

- `id`, `agent_id`, semantic or monotonic `version`
- canonical `spec_json` and `spec_digest`
- `runtime_image_digest`
- model-policy, tool-descriptor, command-policy, network-policy, approval-policy,
  and evaluation-gate version references
- `created_from_draft_revision`, author, release notes, and timestamps

### Publication

Assigns one agent version to workspace/group audiences. It has an effective
interval, status, reviewers, review decision, and rollback linkage.

### Integration Publication

Binds an immutable agent version or controlled release channel to registered
integration clients. It pins input/output contract versions, workspace,
authority modes, visible artifacts and events, webhook policy, quotas, budgets,
retention, reviewers, and effective interval.

### Tool Server and Tool Descriptor

A tool server record describes ownership, transport, trust, and credential
capabilities. Immutable descriptors contain namespaced tool names, input and
output schemas, effect classification, idempotency classification, version, and
content digest.

## 4. Task and Run Entities

### Task

A durable user request:

- `id`, scope, requester, selected agent
- input envelope and attachment references
- visibility and retention class
- invocation channel, integration client, optional delegated subject, source
  system/resource references, and external correlation ID
- current run reference and aggregate status
- idempotency key and timestamps

### Run

One attempt:

- `id`, `task_id`, `attempt_number`
- immutable agent and policy snapshot references
- status and status reason
- scheduler priority, runner pool, sandbox and lease references, `lease_epoch`
- canonical event sequence, checkpoint reference
- start, completion, cancellation, and expiry timestamps
- usage and cost projections

### Run Manifest

A signed, expiring document delivered to a runner. It contains only the
configuration needed for one run, resource limits, gateway endpoints, scoped
workload identity, and immutable digests. It contains no durable secret values.

### Artifact

Metadata for a generated or uploaded object:

- `id`, task/run, producer and owner
- object key, digest, size, media type
- classification, scan status, retention class
- visibility and download-policy fields

### Webhook Endpoint and Delivery

An endpoint is bound to one integration client and environment and stores an
approved destination, authentication mode, subscribed event types, status, and
rotation metadata. Each delivery records event sequence, attempt, signature-key
reference, response class, next attempt, and terminal delivery status.

## 5. Approval Entities

### Approval Request

- `id`, run and event sequence
- immutable `action_digest`
- normalized action preview and redacted payload reference
- risk class, policy reason, eligible approver rule
- threshold, status, expiry, and supersession reference

### Approval Decision

- request and approver
- `approve` or `reject`
- reason, authentication context, decision time
- policy evaluation reference and unique idempotency key

Decisions are append-only. The approval request projection derives whether the
threshold is met, rejected, expired, superseded, or revoked and has its own
revision. A decision insert, projection recomputation, and any
`awaiting_approval -> ready` action transition occur in one transaction. The
unique request-and-approver constraint makes duplicate votes idempotent, while
the projection revision ensures exactly one transaction emits the
threshold-satisfied event. The immutable approval policy defines whether one
reject vote immediately rejects the request or a remaining threshold can still
be satisfied.

Rejection and expiry are approval outcomes, not run statuses. Each immutable
approval policy chooses `resume_with_denial` or `fail_run` for those outcomes.
Applying the outcome uses the same action revision transaction and emits the
corresponding run transition; task status continues to derive from its current
run.

### Action Execution

Every proposed external effect has a durable record containing `id`, `run_id`,
tool and operation, canonical argument digest, target, effect and idempotency
classification, policy and approval references, `lease_epoch`, state, revision,
execution-permit digest, attempt, and result reference.

The state machine is:

```text
proposed -> denied
proposed -> awaiting_approval -> ready
                         \-----> rejected/expired/superseded
proposed -> ready -> executing -> succeeded/failed/unknown_outcome
```

State changes use compare-and-swap on the expected revision. At most one action
per run may be `awaiting_approval`, `ready`, or `executing` in the first agent
loop. Approval decisions do not invoke a tool directly; they only make the
bound action eligible for the gateway's atomic execution claim.

The successful `ready -> executing` claim consumes the execution permit and is
the action's linearization point. Cancellation before that transaction prevents
the call. Cancellation afterward is cooperative; if the target result cannot be
proved, the action becomes `unknown_outcome` and retains reconciliation state
rather than being represented as safely canceled. Reconciliation has an owner
and deadline. If still unresolved at that deadline, the run becomes `Failed`
with reason `action_outcome_unknown`; later evidence is appended without
rewriting the original action result.

## 6. Evaluation Entities

### Golden Case

An immutable versioned case containing input, fixture-set reference, assertions,
rubrics, redaction provenance, and source lineage.

### Evaluation Suite

An ordered set of golden-case versions plus suite-level thresholds and policy
gates.

### Evaluation Run

References one candidate agent version, optional baseline version, immutable
suite manifest, execution environment, aggregate state, and results.

### Assertion Result

Records assertion type, expected value or digest, actual value or digest,
status, evidence reference, and severity. Probabilistic scores include evaluator
identity, prompt/version, repetitions, distribution, and confidence information.

## 7. Event Envelope

Every run event uses a common envelope:

```json
{
  "event_id": "evt_...",
  "organization_id": "org_...",
  "workspace_id": "wsp_...",
  "task_id": "tsk_...",
  "run_id": "run_...",
  "sequence": 42,
  "type": "tool.call.requested",
  "schema_version": 1,
  "occurred_at": "2026-08-13T08:00:00Z",
  "recorded_at": "2026-08-13T08:00:00.120Z",
  "actor": {
    "type": "runner",
    "id": "rnr_...",
    "on_behalf_of": "usr_..."
  },
  "correlation_id": "cor_...",
  "causation_event_id": "evt_...",
  "payload": {},
  "previous_hash": "sha256:...",
  "event_hash": "sha256:..."
}
```

Event payloads are typed and versioned. Unknown optional fields are ignored;
unknown event types are retained but not projected until supported.

## 8. Core Run Event Types

### Lifecycle

- `task.accepted`
- `run.queued`
- `run.provisioning_started`
- `run.started`
- `run.suspension_requested`
- `run.checkpoint_created`
- `run.suspended`
- `run.resumed`
- `run.cancel_requested`
- `run.completed`
- `run.failed`
- `run.canceled`
- `run.expired`

### Model Interaction

- `model.requested`
- `model.route_selected`
- `model.output_segment_committed`
- `model.output_completed`
- `model.usage_recorded`
- `model.failed`

Token deltas may be sent live, but durable history stores immutable content
segments rather than one event row per token. A segment event references the
stream ID, first and last byte offsets, content digest, object key, size, timing
range, and optional token-count metadata.

### Reasoning and Plans

- `agent.plan_updated`
- `agent.rationale_recorded`
- `agent.user_input_requested`

These contain concise, user-facing summaries. They are not raw hidden reasoning.

### Tools and Commands

- `tool.call.proposed`
- `tool.call.authorized`
- `tool.call.denied`
- `tool.call.requested`
- `tool.call.started`
- `tool.call.completed`
- `tool.call.failed`
- `command.started`
- `command.output_segment_committed`
- `command.completed`

### Approval

- `approval.requested`
- `approval.decision_recorded`
- `approval.satisfied`
- `approval.rejected`
- `approval.expired`
- `approval.superseded`
- `approval.outcome_applied`

### Artifacts and Operations

- `artifact.declared`
- `artifact.uploaded`
- `artifact.scan_completed`
- `operator.terminal_attached`
- `operator.terminal_detached`
- `policy.evaluated`
- `sandbox.cleanup_completed`
- `sandbox.cleanup_failed`

## 9. Ordering and Idempotency

- The database allocates one strictly increasing sequence per run.
- Runner messages include runner-session ID and client sequence for
  deduplication.
- Commands from clients require an idempotency key scoped to actor and route.
  The request digest and original resource ID remain as a tombstone at least
  through the resource's terminal state plus the published maximum retry
  interval; content deletion does not make the key reusable.
- Tool calls have stable call IDs and effect classification.
- Approval decisions have a uniqueness constraint per request and approver.
- Outbox consumers are idempotent and track their last processed event.

Global ordering across different runs is neither required nor claimed.

## 10. High-Frequency Content Streams

Model output and PTY output are byte streams with independent monotonically
increasing offsets. The runner and gateway buffer them into segments using
configurable byte and time thresholds. Implementations must enforce a memory
ceiling, flush on lifecycle boundaries, apply backpressure when durable storage
lags, and never silently drop audit-required output.

A segment is durable only after its object is committed and a PostgreSQL index
row is written. The row records the run and stream, offset range, digest, object
key, media type or encoding, timing range, retention class, and encryption
metadata. Segment ranges may not overlap; an exact duplicate is deduplicated by
stream and offsets. A content-stream outbox entry makes durable segments visible
to reconnecting clients.

Semantic events and segment references share the run's canonical ordering, but
live subscribers may receive explicitly marked provisional deltas between
durable cursors. On reconnect, clients discard provisional content after their
last durable offset and replay committed segments. Flow control limits both
unacknowledged bytes and maximum acknowledgement delay so high-volume output
cannot starve cancellation, lease, approval, or terminal messages.

Prototype defaults are a 50 ms maximum live-display batch interval, a durable
segment flush at 256 KiB or 1 second (whichever comes first), and a 4 MiB
per-stream in-memory ceiling. Lifecycle and terminal boundaries force a flush.
These are tunable deployment defaults, not public protocol guarantees; Phase 0
load evidence must set supported values and prove bounded PostgreSQL rows and
WAL bytes per output GiB, segment commit latency, object request rate, reconnect
lag, memory use, and control-message latency under a slow object store.

An object committed without its index row is unreachable and is reclaimed by a
bounded orphan-object job after a safety interval. An index row is never exposed
before the referenced object and digest are readable. Segment finalization is
idempotent so a retry after either side of the storage boundary cannot create an
overlapping visible range.

## 11. Payload and Redaction Rules

Event envelopes remain queryable, but sensitive payloads may be stored as
encrypted objects referenced from the event. Each payload field is classified
as searchable metadata, encrypted content, secret-forbidden, or transient-only.

Redaction records the rule set and produces a new derivative artifact. Gantry
does not overwrite historical evidence and call the result sanitized.

## 12. State Projections

Current run status, pending approval, last event, usage totals, and health are
projections updated transactionally where possible. Projection rebuilding from
events is supported for repair and verification, but events are not used as an
excuse to make common queries scan an entire run history.

## 13. Cursor Retention, Compaction, and Deletion

The no-loss reconnect guarantee applies within the declared content-retention
window. A durable cursor identifies its projection, run, canonical sequence,
and content offsets. If a cursor predates available history, the API returns
`cursor_expired` with the earliest available cursor and a current task/run
snapshot; it never silently resumes from a newer position. The client replaces
its projection from that snapshot and then continues from the supplied cursor.

Semantic events are not rewritten in place. Content compaction creates a new
immutable aggregate segment whose manifest lists each source segment's offset
range and digest. The original signed hash-chain leaves and checkpoint remain
verifiable until their audit retention expires; deletion replaces content with
a hash-preserving tombstone. Audit verification checks the event chain,
checkpoint signature, aggregate manifest, and retained or tombstoned source
digests.

Retention operates by content class and workspace policy. A deletion workflow:

1. Authorizes and records the deletion request.
2. Places matching content in a pending-deletion state.
3. Respects legal holds and minimum audit retention.
4. Deletes object content and encryption keys where applicable.
5. Retains a tombstone with digest, scope, reason, and completion status.

## 14. Schema Evolution

- Database migrations are forward-only within a release.
- Public resource schemas and events carry explicit versions.
- Published agent specs retain their original schema plus a canonical normalized
  representation.
- Readers support at least the currently supported upgrade window.
- Destructive schema changes require a migration rehearsal against a production-
  sized copy and a documented rollback or roll-forward procedure.
