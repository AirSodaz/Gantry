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
timestamps. Effective policy is resolved from exact Policy Bindings rather than
one mutable policy-set pointer.

### Principal

Represents a user or service account. It stores the stable external subject,
principal type, display metadata, and lifecycle state. Group and role mappings
are modeled separately and retain their source.

### Role Binding

Binds a principal or group to a role at organization, workspace, or Agent scope.
A binding has validity timestamps and provenance. Agent action approval eligibility
is not granted by a role binding: the authenticated Run requester is the only
eligible human approver. Business workflow approval remains owned by the external
tool or enterprise system.

### Integration Client

Represents a registered enterprise application and environment. It records the
OAuth client reference, owner, status, delegated-user token-exchange policy,
scopes, quotas, data classification, credential metadata, and operational
contacts. The client authenticates transport but never becomes a Run requester
or approval identity. Secret or private-key material remains in the identity or
secret system.

### Credential Reference and Lease

A Credential Reference is a logical, organization-owned name such as
`crm-read-as-user`; it contains owner, allowed modes, target service, data
classification, rotation state, and policy metadata, but never secret material.
A Credential Lease is a short-lived, run/action-bound authorization issued by the
credential broker after action-time policy succeeds:

- reference ID, run/action ID, actor and delegated subject;
- audience, scope, lease epoch, issued/expiry times, and revocation state;
- broker and secret-version provenance.

Lease values are not persisted in prompts, manifests, events, logs, or browser
responses. Rotation or revocation prevents new leases while preserving historical
reference metadata.

### Model Provider and Route

A Model Provider records provider identity, environment, supported models,
transport adapter, data-handling class, health state, credential reference,
capacity, and emergency status. A Provider Route pins an allowed model set,
fallback order, timeout, budget policy, and classification constraints. Route
selection is recorded in the Run Manifest and cannot silently change a published
Agent Revision.

### Runner Pool and Runner

A Runner Pool defines an isolation tier, compatible protocol/image set, capacity
limits, network posture, and drain/quarantine state. A Runner records workload
identity, pool, protocol version, health, lease epoch, and last heartbeat.
Scheduling may choose only a compatible non-draining Runner; quarantine prevents
new assignment and is auditable.

### Retention Policy

A typed retention Policy controls one or more data classes: Audit metadata,
operational metadata, prompts and outputs, terminal streams, Artifacts, or
Evaluation fixtures. The organization defines the permitted bounds; a Workspace
may choose a value within those bounds. Exact default durations are deployment
configuration pending Legal and Security approval, not constants in the product
model.

Retention Policies distinguish content deletion from integrity evidence. Audit
metadata and signed checkpoints remain through the configured minimum, while
content classes may expire earlier. A Policy change never retroactively erases
evidence outside an authorized deletion workflow.

### Legal Hold

A Legal Hold prevents scheduled deletion and key destruction for selected
resources or data classes:

- owner, authority basis, scope or selector, and affected content classes
- status, set time, optional release time, and release actor
- matching records, protected deletion jobs, and provenance

The selector is a bounded typed expression over organization/workspace, resource
IDs, Session/Run/Artifact IDs, classification, and time range; arbitrary SQL or
unbounded predicates are not accepted. The selector is immutable after activation.
Creation records a match-preview snapshot for evidence, but active Holds are
re-evaluated for newly created matching data and immediately before deletion.
Hold creation and release are immutable, attributable events. A Hold is checked
before deletion enters execution and may leave a deletion request pending until
all matching Holds are released.

### Platform Settings Composition

Platform Settings is a scope-aware projection over typed resources, not a
mutable catch-all settings entity. The projection resolves an effective value
from the organization default/bound and an optional Workspace override. Every
returned value carries its source, bound, owner resource, validation state,
ETag, and last-change metadata. Resolution is monotonic: Workspace settings
may narrow an organization value but may not broaden it.

The owning records are:

- **Organization Profile**: organization identity metadata, status, defaults,
  and non-negotiable bounds. Authentication subjects and secrets remain outside
  this record.
- **Workspace Setting Override**: a typed override keyed by Workspace and
  setting owner, with an explicit inherit/reset state, validation result,
  issuer, reason, ETag, and timestamps. Resetting removes the override while
  preserving the organization value.
- **Data Classification Definition**: label, handling posture, defaulting
  rule, allowed providers/tools/integrations, and retention class mapping.
- **Limit Policy**: organization ceilings and Workspace allocations for
  concurrency, duration, volume, output/artifact size, and budget. Provider,
  runner, and Integration-specific quotas remain on their owning resources.
- **Environment Profile**: named environment, publication posture, data
  handling class, emergency state, and allowed target controls. Credentials,
  private keys, and webhook secrets are referenced by owning resources only.
- **Retention Deletion Job**: asynchronous estimate, eligible window, Hold
  matches, protected and blocked counts, execution attempts, completion/failure
  state, and digest-preserving Tombstone summary without retained content.
- **Audit Event**: immutable cross-resource evidence envelope containing event ID,
  scope, actor, subject, action, resource, outcome, risk, correlation/causation
  references, schema version, redaction metadata, and linked immutable digests.
  Run events may link to an Audit Event but do not replace this projection.
- **Audit Export Package**: a scoped, redacted export request with requester,
  query/scope digest, package digest, state (`requested`, `processing`,
  `ready`, `expired`, or `failed`), expiry, download count, and
  failure reason. The package contains no secrets or raw chain-of-thought and
  has independent retention from the source evidence.

Settings reads are projections and do not create a second history store. Changes
are represented by the owning typed record plus canonical Audit events.

## 3. Agent Configuration Entities

### Agent

Mutable identity and ownership record:

- `id`, `organization_id`, `workspace_id`
- `slug`, `display_name`, `description`, `category`
- `owner_principal_id`, `lifecycle_status`
- creation and update metadata

This is the company-governed Agent definition used for authoring, revision,
deployment, discovery, and execution authorization. Gantry does not introduce a
separate Agent Template or per-user Agent Instance in the initial model.
Personal history, shared work, and channel-bound continuity belong to Sessions;
an Agent instance becomes justified only if a future product requirement needs
independent long-term memory, credentials, or configuration across Sessions.

### Agent Access Grant

An explicit Agent-scoped capability grant:

The typed Admin resource, route matrix, owner-transfer transaction, and
persistence constraints are defined in [Agent Access Contracts](agent-access-contracts.md).

- `id`, `agent_id`, subject type and subject ID
- capability: `metadata.read`, `configuration.read`, `draft.edit`,
  `review.decide`, `deployment.test`, `deployment.production`, `runs.read`,
  `execute`, or `access.manage`
- `grant_batch_id` and optional informational `source_preset`
- grant status, issuer, reason, effective interval, and timestamps

The Agent row also carries a monotonic `access_revision` used as the ACL
collection ETag. Grant and owner mutations update it in the same transaction as
the canonical Audit event.

The effective permission is the intersection of organization/workspace
authorization, Agent Access Grants, resource state, and action-time policy.
Grants do not contain credentials and do not broaden Tool Binding authority.
Access changes and denied decisions are append-only audit events.
Preset names are not evaluated at authorization time. A preset creates explicit
capability grants, and later preset-definition changes never rewrite them.
The initial model stores no Deny grants: absent or revoked Allow grants are
denied, while an outer constraint is recorded as the reason for an ineffective
grant.

### Agent Draft

Named mutable work line:

- `id`, `agent_id`, `name`, `status`
- optional `derived_from_revision_hash`, `latest_revision_hash`
- working `spec_json`, `schema_version`, and `working_copy_etag`
- `validation_status`, `validation_findings`
- `created_by`, `updated_by`, creation and update timestamps

Every Agent has one Main Draft and may have multiple named Drafts. Archiving a
Draft removes it from active editing without deleting its Revisions.

### Agent Revision

Immutable configuration commit:

- `id`, `agent_id`, full `revision_hash`
- required `message`, author, committed timestamp, and source Draft
- canonical `spec_json` and `spec_digest`
- `runtime_image_digest`
- prompt-snapshot digest, skill-artifact identity and content digest,
  plugin-version, model-policy,
  tool-descriptor, tool-binding,
  command-profile, network-policy, approval-policy, and evaluation-gate version
  references

The revision hash covers the canonical revision envelope and commit metadata.
The separate specification digest covers executable configuration content, so
two history commits may be behaviorally identical without sharing a revision
hash.

### Agent Deployment

Assigns one Agent Revision to a named environment or audience:

- `id`, `agent_id`, `name`, `environment_kind`
- exact `revision_hash` and `spec_digest`
- status, owner, purpose, optional expiry, and environment policy
- effective interval, changed-by actor, review/evaluation evidence, and rollback
  linkage

An Agent may have multiple test Deployments but one default Production
Deployment in its workspace. Publication and rollback create deployment events
and move the pointer without creating or rewriting a Revision.

### Published Copilot Metadata and Preferences

Employee-facing catalog metadata is split at the same boundary as the Agent
lifecycle. Stable identity fields remain on the Agent. Typical inputs, the
expected output shape, capability summary, data disclosure, and action
disclosure are authored in the Draft's typed catalog metadata and copied into
the immutable Revision snapshot. A Deployment contributes only publication
availability (available or temporarily unavailable, with a bounded reason and
optional expiry). The Copilot projection joins the active Deployment, exact
Revision metadata, stable Agent identity, and the caller's own preference row;
it never exposes the raw Draft, prompt, Tool bindings, model route, or Policy.

`copilot_agent_preferences` is keyed by `(principal_id, workspace_id,
agent_id)` and stores `is_favorite` plus nullable `last_used_at`. The principal
and Workspace are derived from authorization, not request fields. A successful
Session creation updates `last_used_at` in the same durable command/outbox
boundary. A small maintenance step prunes non-favorite rows outside the eight
most recent successful Agent uses for that principal and Workspace; favorite
rows remain available to the favorites collection. Preference mutations are
ordinary audited commands and cannot grant Agent ACL capabilities or override
Deployment availability.

### Integration Publication

Binds an immutable Agent Revision or controlled release channel to registered
integration clients. It pins input/output contract versions, workspace,
delegated-subject requirements, visible artifacts and events, webhook policy,
quotas, budgets, retention, reviewers, and effective interval.

### Agent Prompt Snapshot

The editable system prompt is stored in an Agent Draft working copy. Committing
a Revision embeds an immutable Prompt Snapshot containing instruction text,
declared variables, instruction ordering, classification, provenance, compiler
version, and content digest. It has no independent catalog lifecycle.

### Skill and Imported Skill Artifact

A Skill is a workspace-owned reusable capability identity backed by packages
from an external skills source. Gantry does not create or manage Skill versions.
An imported Skill Artifact records the source type and source reference, package
identity, version declared by the package, prompt fragment, input/output constraints,
preconditions, required Tool Binding constraints, optional rule fragments,
artifact expectations, risk metadata, provenance, validation state, and content
digest. Multiple artifacts of one Skill may coexist for testing. The declared
package version is display metadata; when it is absent, the UI shows `未声明`
and Gantry does not synthesize one. The source reference and content digest
identify the exact artifact used by an Agent Revision.

### Plugin Installation, Enablement, and Version

A Plugin Version is an immutable organization-installable package containing
Skill package references, optional Tool Server and Tool Descriptor references,
configuration schemas, default binding templates, publisher/provenance,
compatibility, risk metadata, and content digest.

Plugin Installation records organization review and operational status.
Workspace Plugin Enablement records which exact Plugin Versions are available to
a workspace. Multiple versions may coexist for testing and migration. Agent
Revisions separately pin only the contained Skill artifacts and Tool Bindings
selected by the designer; enablement does not imply authorization.

### Tool Server

An organization-owned Tool Server describes provider type (`builtin`, `mcp`, or
`cli`), owner, environment, operational status, transport, endpoint reference,
trust tier, credential capabilities, data classifications, destination class,
discovery policy, and health metadata. Secret connection material remains in a
trusted identity or secret adapter.

### Tool Descriptor Version

An immutable descriptor contains fully qualified tool name, input/output
schemas, effect and idempotency classifications, credential capability, data
classifications, destination class, execution limits, compatibility metadata,
version, lifecycle state, and content digest.
Multiple descriptor versions for the same fully qualified name may be active
simultaneously for testing or migration. There is no default descriptor version;
an Agent Tool Binding selects one exact version and digest.

### CLI Command Profile

A versioned command descriptor contains executable identity, structured argument
schema/rendering, working-directory and filesystem constraints, environment
allowlist, runtime-image requirements, timeout/output/process limits, artifact
rules, effect/idempotency classification, and content digest.

### Tool Binding

An Agent-owned binding references one immutable Tool Descriptor Version and
narrows allowed operations, arguments, credential mode/reference, destination,
data classification, approval policy, timeout, output, concurrency, and budget
limits. Agent Revisions pin the binding and descriptor digests.

## 4. Policy Configuration Entities

### Policy

A stable organization-owned policy identity:

- `id`, `organization_id`, optional owning `workspace_id`
- type, name, description, owner, lifecycle state
- creation and update metadata

Policy types include approval, model, tool or command, network, credential and
data, budget or limit, retention, and evaluation or publication rules. They
share one lifecycle without requiring one universal user-authored policy
language.

### Policy Draft

The single mutable working copy for one Policy:

- `policy_id`, typed `document_json`, and `schema_version`
- `working_copy_etag`, validation state, and validation findings
- author and update timestamps

A Draft cannot participate in runtime authorization. A Policy has no parallel
Drafts, branches, merge, or rebase semantics.

### Policy Version

An immutable published policy document:

- exact `id`, `policy_id`, `content_digest`, and `schema_version`
- canonical `document_json`, required message, author, and creation time
- validation/compiler evidence and policy type

Publishing creates a Version but does not activate or bind it. A referenced
Version is retained with its exact content and evidence.

### Policy Binding

Applies one exact Policy Version to an organization, Workspace, governed
platform resource, Deployment, or Integration Publication. It records target,
environment, effective interval, status, actor, reason, and provenance. An
Agent Revision directly pins its exact Policy Version references rather than
resolving a movable latest Version.

Effective policy is the intersection of active Organization and Workspace
Bindings, the exact Agent Revision references, and applicable resource
constraints. A lower-scope Binding may only narrow outer authority. The Run
Manifest records every contributing Policy Version ID and content digest so a
later Binding change does not rewrite historical execution evidence.

The typed Admin resource contract for Policy Drafts, Versions, Bindings, and
Simulation is defined in
[Admin Governed Resource Contracts](admin-governed-resource-contracts.md).

### Evaluation Suite and Run

An Evaluation Suite is a Workspace-scoped identity with mutable authoring state
and immutable Suite Versions. A Suite Version freezes case IDs, fixture and
evaluator manifests, runtime image, compatibility constraints, and content
digests. An Evaluation Run references exactly one Suite Version, candidate Agent
Revision, optional baseline Revision, evaluation environment digest, and signed
evidence manifest. A Run is invalid when evidence is incomparable or fixture
integrity is violated; invalid is not a pass or an ordinary failure.

### Evaluation Gate and Override

A Publication Gate binds a Suite Version and requirement to an exact Agent
Revision. A Gate Override records reviewer, reason, expiry, scope, and the gate
state it supersedes. It never mutates the Evaluation Run or turns an invalid
run into a valid result.

### Integration Client and Publication

An Integration is an organization-owned external-system identity. An
Integration Client is environment-bound and stores authentication modes,
audience, credential fingerprint, status, and expiry metadata. An Agent
Publication binds one exact Agent Revision and input/output contract pair to a
Client or Integration environment. Disabling or revoking a Client or
Publication blocks new invocations while retaining Sessions, Runs, deliveries,
and evidence. A verified delegated subject is mandatory for direct Enterprise
invocation; owner-bound Webhook or scheduled Triggers create ordinary Runs for
unattended work.

### Webhook Endpoint and Delivery

A Webhook Endpoint stores validated destination metadata, subscribed event
projection, signing-key fingerprint, and retry policy. A Webhook Delivery is an
immutable attempt identified by event ID and delivery ID. Redelivery creates a
new attempt and never changes the Run result.

### Platform Resource Records

Model Providers, Provider Routes, Runner Pools, Runners, Credential References,
Data Classifications, Limit Policies, and Environment Profiles are separate
organization or Workspace-scoped records with independent lifecycle and
authorization. Platform Settings is only their effective projection and
section command facade. The typed fields and route ownership are defined in
[Admin Governed Resource Contracts](admin-governed-resource-contracts.md).

## 5. Session and Run Entities

### Session

A durable personal or collaborative Agent context:

- `id`, organization and Workspace scope, selected Agent
- `owner_principal_id`, mode (`personal`, `shared`, or `channel`), lifecycle
  state (`active` or `archived`), and retention class
- ordered member, message, Run-summary, and Artifact references with a
  monotonic conversation revision and Session sequence
- optional external channel binding
- optional owner-bound trigger references and source tags (`webhook` or
  `schedule`) when automation appends work to the Session
- current executing Run, queued Run count, and timestamps

A Session binds an Agent identity, not one mutable Draft or implicit latest
Revision. Every Run resolves an allowed Deployment and freezes the exact Agent
Revision and Policy snapshots used for that execution. Session history can
therefore continue across Production updates without rewriting prior evidence.

### Session Member

Session collaboration uses three fixed roles rather than another general ACL:

- `owner`: read, contribute, manage membership, archive, and bind a Trigger or
  external channel;
- `contributor`: read and submit instructions when the principal still has
  Agent `execute` authority;
- `viewer`: read authorized Session history and Artifacts without execution.

Every Session has exactly one human owner. Membership never grants Agent
configuration access, Agent execution authority, or approval authority. Adding
or removing a member is an audited Session command subject to Workspace and
data-classification policy. Historical reads require current Session membership
and applicable data policy; starting a new Run separately requires current
Agent `execute` authority. Owner transfer is explicit and atomic; it disables
old-owner Triggers bound to that Session rather than transferring them silently.

Copilot lists owner-owned Sessions by default and can explicitly request all
current memberships. A `workspace_agent_editor` may use the Admin audience to
add only itself as a normal `viewer` on a Session for an Agent in its assigned
Workspace. The membership is visible to the owner and other members, grants
read-only Copilot access, and carries canonical Audit evidence; the role is not
an implicit read path and cannot create `contributor` or owner membership.

### Session Message

An employee-visible conversation turn within a Session:

- `id`, `session_id`, monotonic Session sequence, author kind, and author
  identity or Trigger reference
- message kind, redacted content or structured payload reference, and
  classification
- optional initiating or resulting Run reference
- causation/correlation references, visibility, and timestamps

An accepted human or Trigger instruction advances the conversation revision and
creates one queued Run. It never mutates a prior message, Approval Request, or
completed Run. Agent and system-summary messages are appended by the owning
runtime transitions and never establish a requester identity.

### Run Attachment Input

Each bound Attachment is copied into an immutable Run input snapshot before
assignment. The snapshot carries the Attachment ID, digest, size, media type,
classification, generated sandbox-relative path, opaque Run-bound reference,
and expiry. It contains no object key or secret. The Runner reads this input
only through the lease-fenced Control Plane broker described in
[Runner Attachment Contracts](runner-attachment-contracts.md).

Materialization attempts record the Run, Attachment, Runner session, lease
epoch, byte count, result digest, and terminal outcome. A stale lease, revoked
scan state, Policy change, missing object, or digest mismatch prevents the
input from becoming visible in the sandbox; historical evidence remains.

### Run

One execution of an accepted Session instruction, or a retry of a prior Run:

- `id`, `session_id`, Session sequence, initiating message, `requester_id`
- optional `retry_of_run_id` and Trigger occurrence reference
- immutable agent and policy snapshot references
- execution status, status reason, and terminal outcome such as
  `requester_input_required`
- scheduler priority, runner pool, sandbox and lease references, `lease_epoch`
- canonical event sequence, checkpoint reference
- start, completion, cancellation, and expiry timestamps
- usage and cost projections

The principal or Trigger owner responsible for the initiating instruction is
the immutable Run requester and the only person who can decide an Agent-action
approval proposed by that Run. Session ownership, membership, Admin roles, and
business-approver roles do not replace the Run requester.

The Run requester and current Session owner may cancel the Run. Owner
cancellation is a restrictive Session-management command and never grants the
owner approval authority. A queued cancellation removes the Run from scheduling
order; an executing cancellation follows the action permit and reconciliation
rules below.

A Session has at most one executing Run across provisioning, running, approval,
external-wait, and cancellation states. Additional accepted instructions may
create queued Runs, ordered by Session sequence. The next queued Run starts only
after the current Run reaches a safe terminal boundary. A rejected or expired
action is delivered to the Agent as a structured result; the Run may finish
with `requester_input_required`, while the active Session remains open for any
authorized contributor to submit a new instruction.

### Run Manifest

A signed, expiring document delivered to a runner. It contains only the
configuration needed for one run, resource limits, gateway endpoints, scoped
workload identity, and immutable digests. It contains no durable secret values.

### Session and Run Event Ordering

Each Run retains its own strictly increasing event sequence for operational
diagnostics. A multi-Run Session stream additionally requires a Session-level sequence
for requester messages and Run summaries; that sequence is the authority for a
Session-bound browser cursor. Until that aggregate stream exists, a cursor is
run-bound and a Run change requires snapshot replacement and reconnect.

### Artifact

Metadata for a generated or uploaded object:

- `id`, session/run, producer and owner
- object key, digest, size, media type
- classification, scan status, retention class
- visibility and download-policy fields

### Owner-Bound Automation Trigger and Occurrence

An owner-bound Trigger records its employee-visible name, kind (`webhook` or
`schedule`), human owner, Agent, server-resolved Production Deployment, fixed
Service Principal execution identity, state, expiry, and Session mode.
`new_session` creates a Session for each occurrence;
`bound_session` stores one exact Session ID and is valid only while the Trigger
owner owns that active Session and can execute the same Agent.

A Webhook configuration owns a read-only snapshot of the Agent's published input
contract, one active monotonically versioned secret/key reference, and rate
limits; the owner never supplies a JSON Schema. Rotation atomically activates a
new key version and retires the previous version without an overlap window. A
scheduled configuration owns a validated five-field cron expression,
canonical IANA time zone, fixed input validated by the same published contract,
`skip` misfire policy, first-instant daylight-saving overlap policy,
monotonically increasing schedule revision, and the next planned UTC instant.
Changing its expression, time zone, fixed input, Deployment, or Session target
increments the schedule revision and recomputes the next instant without
changing prior occurrences.

Each Webhook occurrence records `(trigger_id, event_id)`, canonical request
digest, signature-key version, source metadata, authorization result, Session
and Run IDs, canonical receipt, and timestamps. A scheduled occurrence uses the
same record with a stable occurrence ID derived from Trigger ID, schedule
revision, and planned UTC instant. The occurrence record, Session Message,
queued Run, canonical receipt, Audit evidence, advanced next-planned instant,
and outbox entry commit together.

After restart, re-enable, or outage recovery, planned instants that passed
without a committed occurrence are skipped. A committed occurrence continues
through ordinary outbox retry. A nonexistent daylight-saving local time is
skipped; a repeated local time maps only to its first UTC instant.

## 6. Approval Entities

### Approval Request

- `id`, run and event sequence
- immutable `action_digest`
- normalized action preview and redacted payload reference
- risk class, policy reason, requester identity
- status, expiry, and supersession reference

The approval request is for one concrete agent action. It is not a general
business-workflow approval record: leave, expense, purchase, and similar
decisions remain in the tool or enterprise system that owns that process. The
authenticated Run requester is the only eligible human approver. A published
policy may allow or deny the action without human confirmation, but it cannot
route the request to a different person, group, or administrator.

### Approval Decision

- request and authenticated requester
- `approve` or `reject`
- reason, authentication context, decision time
- policy evaluation reference and unique idempotency key

Decisions are append-only. The approval request projection derives whether the
request is approved, rejected, expired, superseded, or revoked and has its own
revision. A decision insert, projection recomputation, and the corresponding
action transition occur in one transaction. The unique request-decision
constraint makes duplicate requester commands idempotent, while the projection
revision ensures exactly one terminal decision takes effect.

Rejection and expiry are approval outcomes, not run statuses. Both resume the
Agent loop with a schema-valid `action_denied` or `approval_expired` result and
leave the Session conversation open for additional contributor input. The Run requester
may direct the Agent to revise the action or continue differently; any new
consequential action receives a new digest. Applying either outcome uses the
same action revision transaction and emits the corresponding run transition.

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
proposed -> ready -> executing -> awaiting_external_approval
                              \-> succeeded/failed/unknown_outcome
awaiting_external_approval -> succeeded/failed/external_rejected/external_expired
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

### External Business Approval Wait

After the Run requester approves the exact Gantry action, the business Tool
may submit that action into its own approval workflow and return a typed
`external_approval_pending` result. Gantry then records an external wait with:

- Session, Run, action ID, action digest, and consumed execution-permit reference;
- external system, opaque approval reference, callback authentication profile,
  and external correlation ID;
- state (`pending`, `approved`, `rejected`, `expired`, or `canceled`), revision,
  expected expiry, callback event ID, and result digest;
- requester, Tool Binding, Policy Version, and audit correlation.

The Run remains `suspended` with
`suspension_reason=external_business_approval`; this is not another Gantry
Approval Request. A signed, idempotent callback may resolve only the matching
external reference and action digest. Resolution appends the structured Tool
result and resumes the same Run through the outbox for the next Agent loop. It
never replays the original Tool call or creates a new execution permit.

An approved callback may report that the business effect is complete or that
additional work is required. The resumed Agent may finish or propose another
action; every new consequential action receives a new digest and requester
approval when Policy requires it. Rejection or expiry is also returned to the
Agent as structured Tool evidence so it can finish, revise the plan, or request
new user direction. If the Run was canceled, the callback is retained as late
evidence but does not resume execution.

## 7. Evaluation Entities

### Golden Case

An immutable versioned case containing input, fixture-set reference, assertions,
rubrics, redaction provenance, and source lineage.

### Evaluation Suite

An ordered set of golden-case versions plus suite-level thresholds and policy
gates.

### Evaluation Run

References one candidate Agent Revision, optional baseline Revision, immutable
suite manifest, execution environment, aggregate state, and results.

### Assertion Result

Records assertion type, expected value or digest, actual value or digest,
status, evidence reference, and severity. Probabilistic scores include evaluator
identity, prompt/version, repetitions, distribution, and confidence information.

## 8. Event Envelope

Every run event uses a common envelope:

```json
{
  "event_id": "evt_...",
  "organization_id": "org_...",
  "workspace_id": "wsp_...",
  "session_id": "ses_...",
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

## 9. Core Run Event Types

### Lifecycle

- `session.created`
- `session.message.submitted`
- `session.member.added`
- `session.member.updated`
- `session.member.removed`
- `session.archived`
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
- `run.requester_input_required`

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
- `business_approval.wait_started`
- `business_approval.callback_recorded`
- `business_approval.wait_resolved`
- `business_approval.callback_ignored`

### Catalog and Preferences

- `agent.catalog_metadata_committed`
- `agent.publication_availability_changed`
- `copilot.agent_favorite_changed`
- `copilot.agent_recent_use_recorded`

Catalog metadata and availability events carry only immutable Revision or
Deployment identifiers plus bounded employee-safe summaries. Preference events
carry the requester, Workspace, Agent, and resulting favorite/recent timestamp;
they never contain prompts, Tool payloads, or ACL grants.

### Artifacts and Operations

- `artifact.declared`
- `artifact.uploaded`
- `artifact.scan_completed`
- `operator.terminal_attached`
- `operator.terminal_detached`
- `policy.evaluated`
- `platform.settings.validated`
- `platform.settings.changed`
- `classification.changed`
- `limit.policy.changed`
- `environment.profile.changed`
- `retention.deletion_requested`
- `retention.deletion_blocked`
- `retention.deletion_completed`
- `retention.deletion_failed`
- `legal_hold.created`
- `legal_hold.released`
- `sandbox.cleanup_completed`
- `sandbox.cleanup_failed`

## 10. Ordering and Idempotency

- The database allocates one strictly increasing sequence per Run. The target
  multi-Run Session projection also allocates a Session-level sequence for
  conversation turns and cross-Run summaries; it is distinct from each Run's
  diagnostic sequence.
- Runner messages include runner-session ID and client sequence for
  deduplication.
- Commands from clients require an idempotency key scoped to actor and route.
  The request digest and original resource ID remain as a tombstone through the
  published maximum retry interval and applicable resource retention; content
  deletion does not make the key reusable.
- Inbound Webhook occurrences use a uniqueness constraint on
  `(trigger_id, event_id)`. Scheduled occurrences derive a stable ID from the
  Trigger, schedule revision, and planned UTC instant. The occurrence claim,
  request digest, Session Message, queued Run, canonical receipt, and outbox row
  commit atomically, so no retry path can enqueue the same occurrence twice.
- Tool calls have stable call IDs and effect classification.
- Approval decisions have a uniqueness constraint per request.
- Outbox consumers are idempotent and track their last processed event.

Global ordering across different runs is neither required nor claimed.

## 11. High-Frequency Content Streams

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

## 12. Payload and Redaction Rules

Event envelopes remain queryable, but sensitive payloads may be stored as
encrypted objects referenced from the event. Each payload field is classified
as searchable metadata, encrypted content, secret-forbidden, or transient-only.

Redaction records the rule set and produces a new derivative artifact. Gantry
does not overwrite historical evidence and call the result sanitized.

## 13. State Projections

Current run status, pending approval, last event, usage totals, and health are
projections updated transactionally where possible. Projection rebuilding from
events is supported for repair and verification, but events are not used as an
excuse to make common queries scan an entire run history.

The same immutable Session and Run identities support separate authorized
projections. Copilot projections are member-scoped and conversation-first,
with compact Run attempts, user-visible events, approvals, and artifacts.
Admin projections are organization or Workspace-scoped and operational, with
cross-actor filtering, runner and lease diagnostics, policy and Tool evidence,
resource usage, and authorized commands. Enterprise projections are limited to
the published Integration contract. A projection never grants access to a
different projection or to fields omitted by its audience policy.

The exact Copilot projection schemas, Session/Run relationship, and command
preconditions are defined in
[Copilot Resource Contracts](copilot-resource-contracts.md). Control-plane
ownership and atomic projection updates are defined in
[Control-Plane Design Contract](control-plane-design.md).

Audit is one canonical cross-resource projection over append-only events. It
indexes resource, actor, scope, action, outcome, risk, correlation, and linked
immutable identities for search. Resource pages may render a small Recent
activity slice with a pre-filtered Audit link, but they do not maintain separate
Audit stores or alternate export contracts. Domain timelines such as Run event
streams and Webhook deliveries remain operational projections and link to the
canonical Audit evidence.

## 14. Cursor Retention, Compaction, and Deletion

The no-loss reconnect guarantee applies within the declared content-retention
window. A durable cursor identifies its projection, Session or Run scope,
canonical sequence, and content offsets. If a cursor predates available
history, the API returns
`cursor_expired` with the earliest available cursor and a current Session/Run
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

Deletion never removes an active Legal Hold's matching content or the minimum
Audit metadata and integrity checkpoints. A Tombstone preserves the deleted
object's digest, scope, classification, deletion reason, and verification state
without retaining the deleted content or secret material.

## 15. Schema Evolution

- Before the first released database shape, revise the bootstrap schema directly
  instead of carrying migration code. After release, database migrations are
  forward-only within a release.
- Public resource schemas and events carry explicit versions.
- Published agent specs retain their original schema plus a canonical normalized
  representation.
- Readers support at least the currently supported upgrade window.
- Destructive schema changes require a migration rehearsal against a production-
  sized copy and a documented rollback or roll-forward procedure.
