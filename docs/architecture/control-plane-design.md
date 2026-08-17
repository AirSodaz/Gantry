# Control-Plane Design Contract

## 1. Scope and Architectural Rule

This document defines the implementation boundary for Gantry's Go control
plane: module ownership, dependency direction, persistence, transactions,
idempotency, asynchronous work, and restart recovery. It complements
[System Architecture](system-architecture.md), which describes logical
components, and [Data and Event Model](data-and-event-model.md), which defines
the durable entities and state machines.

The control plane is a modular monolith. One deployable and one PostgreSQL
database do not imply shared ownership. Each durable resource and state
transition has one owning module. Transport, persistence, domain transition,
infrastructure wiring, and development fixtures remain separate
responsibilities. When an existing package would mix them, it is split before
the new use case is added.

This is a target contract, not a claim that every package already follows it.
[Implementation Status](../delivery/implementation-status.md) remains the
checked-in capability baseline.

## 2. Dependency Direction

Each module uses the following layers only where the use case needs them:

```text
OpenAPI / private RPC adapter
            |
            v
Application command or query service
            |
            v
Domain aggregate and transition rules
            |
            v
Repository ports + transaction-scoped event/outbox ports
            |
            v
PostgreSQL / object store / gateway adapters
```

Dependencies point inward. Domain packages contain no HTTP, generated OpenAPI,
SQL driver, object-store SDK, queue worker, environment-variable, or development
fixture types. Transport handlers authenticate, parse, and translate; they do
not own authorization decisions or state transitions. Repositories persist
already-authorized domain changes; they do not infer business policy from SQL
errors.

Cross-module calls use narrow application interfaces and stable IDs or
snapshots. Modules do not read another module's tables directly to avoid calling
its authorization or invariant logic. Reporting projections are the exception:
they consume committed events or documented read models and cannot mutate the
source resource.

The bootstrap package constructs adapters and workers. It contains no domain
rules. Development fixtures live under an explicit development composition
root and never become fallback production behavior.

## 3. Module Ownership

| Module | Owns | May depend on | Must not own |
| --- | --- | --- | --- |
| Identity | Principal mapping and normalized authentication context | OIDC verifier, identity repository | Resource authorization, external IdP sessions/passwords |
| Authorization | Role bindings, Agent ACL evaluation, scope/capability decisions | Identity context, Agent access facts, Policy decision interface | Grant persistence, domain state transitions, browser-side filtering |
| Agent Registry | Agents, Drafts, immutable Revisions, reviews, Deployments, Agent access grants | Configuration resolver, Evaluation gate query, Policy validation | Sessions, runner leases, provider credentials |
| Configuration Assets | Skill Artifacts, Plugin Versions, Tool Servers/Descriptors, CLI profiles, Tool Bindings, manifest compilation | Object metadata, health adapters, Policy validation | External package version authority, Agent prompt editing, runtime effects |
| Policies | Policy Drafts, immutable Versions, Bindings, typed evaluation and simulation | Scope facts and typed input snapshots | Approvals, Tool invocation, secrets, generic script execution |
| Sessions | Sessions, fixed membership, Messages, Runs, requester commands, sequential Run queue, content projections | Agent deployment resolver, Attachment binder, scheduler port, event appender | Approval decisions, runner connection lifecycle, object bytes |
| Actions and Approvals | Durable actions, Policy decision binding, approval requests/decisions, execution permits, external business-approval wait references and callback transitions | Sessions' Run facts, Policy evaluator, credential/tool gateway ports, callback authentication adapter | Business workflow decisions, generic Admin approval inbox |
| Execution Events | Canonical Run and Session sequence allocation, semantic event append, content-segment index, cursor snapshots | Transaction manager, object metadata | Domain transition authority, raw token/PTY row-per-chunk storage |
| Scheduler and Runner Sessions | Queue claims, runner registration, assignment, lease epoch, heartbeat, suspension/resume orchestration | Session/Run transition port, manifest compiler, runner RPC adapter | Run requester commands, Tool effects, Agent authoring |
| Attachments and Artifacts | Upload declarations/grants, quarantine, scans, Attachment materialization broker, Artifact metadata, retention/tombstones | Object store, scanner, Session authorization facts, Runner Session transport | Session lifecycle, unrestricted object-store credentials |
| Evaluations | Suites, Cases, immutable Suite Versions, Evaluation Runs, fixtures, scores, gates | Agent snapshots, scheduler, VCR/object adapters | Production Deployment mutation, normal employee Sessions |
| Integrations | Clients, Publications, outbound endpoints, inbound owner-bound Triggers, occurrence receipts, quotas and external invocation contracts | Session submission port, signing/secret broker, delivery transport | Copilot impersonation, Run outcomes, business approval decisions |
| Platform | Provider/route metadata, Runner Pools, credential references, classifications, limits, environment profiles, composed Settings | Health and secret-store adapters, Policy validation | Secret values, domain-specific Policy/Integration mutation |
| Audit | Canonical immutable Audit projection, integrity checkpoints and exports | Committed domain/audit envelopes, object storage, signing | Alternate resource timelines, mutation authorization |
| Retention | Retention plans, Legal Holds, deletion jobs and tombstones | Resource ownership registry, object storage, Audit | Immediate blind deletes, ownership of source resources |
| Jobs and Outbox | Generic claim/lease/delivery mechanics and consumer checkpoints | PostgreSQL and clock | Domain retry policy, domain payload interpretation |

The table describes ownership, not a requirement for one Go package per row.
A module may contain multiple packages when transport, application, domain, and
adapters would otherwise mix. Conversely, a package named `service` does not
become a place for unrelated resources.

## 4. Public and Private Transport Boundaries

- Admin, Copilot, and Enterprise OpenAPI documents generate separate transport
  types and validate separate OAuth audiences. They may call the same domain
  capability only through audience-specific application queries and
  projections.
- Handlers never return a domain aggregate and rely on JSON omission for
  security. Each audience has an explicit response mapper owned beside its
  query service.
- Private runner protobuf/Connect traffic terminates in the Runner Session
  adapter. It cannot call public HTTP handlers or reuse browser tokens.
- Infrastructure callbacks such as scanner results and webhook delivery
  acknowledgements use authenticated private commands with their own
  idempotency and fencing values.
- API compatibility is defined by checked-in OpenAPI/protobuf contracts. Before
  the first released shape, replace superseded fields directly rather than
  maintaining transitional routes or dual behavior.

## 5. Persistence Ownership and Access

PostgreSQL is the source of truth for durable metadata and current state.
Tables use one owning repository even when foreign keys cross modules. A foreign
key permits referential integrity, not direct mutation by the referencing
module.

Repository contracts follow these rules:

- IDs are opaque application identities. Database surrogate details never
  become an authorization claim.
- Mutable aggregates carry an integer revision or content ETag. Updates use
  `WHERE id = ? AND revision = ?` and return a typed conflict when zero rows are
  changed.
- Append-only Versions, Revisions, decisions, events, and Audit envelopes are
  never updated in place. Corrections append superseding evidence.
- Queries include organization, Workspace, principal, and capability scope in
  SQL where applicable. Fetch-then-hide is not an authorization strategy.
- `SELECT ... FOR UPDATE` is reserved for short transition transactions. No
  transaction waits on a runner, model, scanner, Tool, webhook, or object store.
- JSON columns hold schema-versioned documents or snapshots, not mutable bags
  used to bypass typed columns and state constraints.
- Foreign-key, uniqueness, and check constraints defend invariants, while
  application code returns stable problem codes instead of leaking SQL errors.

Object storage holds immutable bytes. PostgreSQL records the object key,
digest, size, encryption metadata, classification, and lifecycle state. Object
keys are infrastructure data and are not public resource URLs.

## 6. Transaction Contracts

### 6.1 Transaction envelope

Every durable command runs through one transaction envelope:

1. Claim or load the idempotency record.
2. Load the authorized aggregate and expected revision.
3. Evaluate domain transition rules using immutable input snapshots.
4. Persist state and append semantic Session/Run and Audit envelopes.
5. Insert outbox work needed after commit.
6. Store the canonical command result and commit.

An authorization denial that must be audited records a minimal denial envelope
without persisting a partial domain mutation. Authentication failures at the
edge are recorded through security telemetry and the canonical authentication
Audit path, not a resource transaction.

### 6.2 Required atomic command groups

| Command | One transaction contains |
| --- | --- |
| Create Copilot Session | Idempotency, Deployment snapshot, Attachment bindings, Session and owner membership, initial Message, first queued Run/requester, principal/Workspace recent-use update, Session/Run events, Audit, scheduling outbox |
| Create Enterprise Session | Idempotency, client/Publication/verified-subject checks, owner/requester binding, input contract, Session, first queued Run, events, Audit, scheduling outbox |
| Append Session instruction | Idempotency, Session conversation CAS, membership and Agent `execute` checks, Message, requester-bound queued Run, Session/Run events, Audit, scheduling outbox |
| Change Session membership | Idempotency, Session ETag, owner command, Workspace/data-policy checks, membership row, Session event, Audit |
| Cancel Run | Idempotency, Run/action/permit CAS, cancel state, lease fencing when required, events, Audit, cancellation outbox |
| Retry Run | Idempotency, eligibility and contributor checks, selected immutable Revision snapshot, new queued Run/requester, Session/Run events, Audit, scheduling outbox |
| Decide approval | Idempotency, Run requester and approval revision CAS, decision append, exact action transition, Session/Run events, Audit, resume outbox |
| Claim action execution | Action CAS from `ready` to `executing`, current Run and lease checks, single-use permit, event, Audit |
| Start external business-approval wait | Consumed action permit, typed pending Tool result, external wait, action/Run suspension CAS, events, Audit, expiry outbox |
| Resolve external business-approval wait | Callback idempotency, signature profile and digest binding, wait CAS, structured Tool result, events, Audit, same-Run resume outbox |
| Commit semantic runner event | Runner-session/client-sequence dedupe, canonical Run sequence, state transition if applicable, Session sequence/projection, outbox |
| Create owner-bound Trigger | Idempotency, owner/Agent execute and active Production Deployment checks, optional bound-Session validation, published input-contract snapshot, Trigger and secret reference or schedule state, Audit |
| Update Trigger | Owner and ETag checks, immutable Agent/kind validation, bound-Session validation, schedule-revision/next-planned update when applicable, Audit |
| Rotate Webhook Trigger secret | Idempotency, owner/kind checks, atomic new key-version activation and previous-version retirement, redacted replay result, Audit; raw secret is returned once and never persisted in the command result or Audit |
| Accept Trigger occurrence | Current-key transport authentication, existing occurrence/digest resolution, or when absent Trigger/schema/owner/quota checks followed by occurrence claim, new or bound Session resolution, Message, queued Run/requester, canonical receipt, Session/Run events, Audit, outbox |
| Materialize scheduled occurrence | Trigger/schedule-revision and next-planned CAS, owner/current-authority checks, stable occurrence ID, new or bound Session resolution, Message, queued Run/requester, canonical receipt, advanced next-planned instant, events, Audit, outbox |
| Publish immutable config | Draft ETag check, immutable Version/Revision, validation evidence, message, Audit; activation is separate |
| Bind/activate config | Exact immutable identity/digest, scope compatibility, binding CAS, Audit, invalidation outbox |
| Set Copilot favorite | Idempotency, authorized Agent/Workspace check, requester preference row, Audit |

No external network or object-store call occurs inside these transactions.
Where an external side effect is required, the transaction records intent and
an outbox/job performs it after commit.

### 6.3 Event and projection atomicity

The domain owner decides whether a transition is legal. The Execution Events
module supplies a transaction-scoped appender that allocates canonical
sequences and writes typed envelopes. Current Session/Run state, member-facing
projection revision, semantic event, Audit envelope, and outbox entry commit
together when they describe the same transition.

Run sequence is strictly increasing per Run. Session sequence is strictly
increasing across employee-visible Messages, Run summaries, approvals, and
Artifacts for one Session. Neither is a global ordering guarantee. Runner client
sequence deduplicates delivery but is never exposed as canonical history.

High-frequency output crosses an object-store boundary and therefore uses a
two-step protocol: upload immutable segment by digest, then transactionally
index its offsets and append the durable segment event. Orphan uploads are
garbage-collected; an event never references an object that failed verification.

## 7. Idempotency Ownership

The module owning a command owns its idempotency namespace and canonical result.
A shared library may implement storage mechanics but cannot decide command
equivalence or retention.

An idempotency record contains principal or workload identity, route/command
name, target scope, key, canonical request digest, state (`in_progress`,
`completed`, `failed_retryable`), response code/body reference, creation time,
and expiry. Concurrent claims serialize on the unique namespace:

- Same key and digest after completion returns the stored result.
- Same key and different digest returns `idempotency_conflict`.
- A live `in_progress` claim returns a bounded retry response rather than
  executing concurrently.
- An abandoned claim is recovered only after its lease expires and the owner
  proves no committed result exists.
- Deterministic validation or authorization failures may be retained for the
  retry window. Transient pre-transaction failures are not cached as success.

Secret-bearing Trigger creation and rotation are the deliberate exception to
byte-for-byte response replay. Their durable idempotency result contains only
Trigger identity, key version, and command outcome. The first successful
transport response receives the newly generated plaintext secret exactly once;
later replay returns the same redacted result and cannot recover the secret.

Browser command records survive normal client retry and deployment restart.
Runner commands additionally key on runner session, Run ID, lease epoch, and
client sequence. Inbound Trigger occurrence idempotency is owned by Integrations
and committed with Session Message and Run creation. Current-key transport
authentication happens first. The transaction then returns an existing matching
occurrence before evaluating current acceptance rules; only an absent occurrence
performs Trigger-state, schema, owner-authority, and quota checks before the
claim. A rejected new occurrence cannot reserve an event ID. Outbound webhook
delivery idempotency remains a separate delivery concern.

## 8. Action Execution and Approval Linearization

Every effect-bearing Tool call has a durable action with normalized arguments,
target, effect class, credential mode, Policy versions, Run, actor, lease epoch,
and `action_digest`.

```text
proposed -> denied
proposed -> awaiting_approval -> ready
proposed ---------------------> ready
ready -> executing -> succeeded | failed | unknown_outcome | awaiting_external_approval
awaiting_external_approval -> succeeded | failed | external_rejected | external_expired
awaiting_approval -> rejected | expired | revoked | superseded
ready | awaiting_approval | awaiting_external_approval -> canceled
```

Consuming the single-use execution permit is the linearization point. The
transaction rechecks action digest, approval, Policy, Run state, cancellation,
lease epoch, credential reference state, and permit absence. Only the permit
owner may report the outcome. Permit expiry after invocation creates
`unknown_outcome`; it does not make a non-repeatable action eligible for replay.

Approval decision only moves the exact action toward `ready`. Business workflow
approval stays external. After the approved Tool call is claimed, a typed
pending result may atomically create an external wait and suspend the same Run.
A signed callback resolves the wait and enqueues the next Agent loop without
replaying the call or restoring the consumed permit. It does not use the Gantry
approval request table or Copilot decision route. See
[External Business Approval Callback Contracts](external-business-approval-contracts.md).

## 9. Scheduler and Lease Fencing

Queued Runs and background jobs use durable claims. A claim has owner, lease
deadline, attempt, and monotonically increasing fencing value. Claim selection
uses a short `FOR UPDATE SKIP LOCKED` transaction; work occurs after commit.

Run assignment increments `lease_epoch`. Reassignment, durable suspension,
terminal transition, and forced cancellation invalidate the old epoch in the
same transaction as the state change. Runner, LLM, Tool, credential, attachment,
and Artifact gateways require the current epoch for every privileged request.

Heartbeat extends only the matching live lease. A stale runner cannot regain
authority by sending a late heartbeat. Lost leases are reconciled by an owner
worker that either resumes from an eligible durable checkpoint with a new epoch
or fails the Run with a typed reason.

## 10. Jobs, Outbox, and Delivery

### 10.1 Mechanics

The outbox is committed with domain state and delivered at least once. Each
record has event ID, owner module, schema version, payload reference, available
time, attempt count, and trace context. Consumers deduplicate by event ID and
consumer name before applying a projection or scheduling another job.

Long-running jobs are durable records, not goroutines started by handlers. A
job has owner module, kind, resource identity, expected resource revision,
state, lease/fencing value, attempts, next-attempt time, deadline, last safe
error, and correlation ID.

Domain owners define retry policy:

- Retry only typed transient failures with capped exponential backoff and
  jitter.
- Re-authorize and re-check resource revision before an effect, not merely when
  the job was created.
- Do not retry non-repeatable external effects after `unknown_outcome`.
- Move exhausted or structurally invalid work to `needs_reconciliation`, not a
  hidden infinite loop.
- Manual reconciliation is an authorized command that appends evidence; it
  cannot edit job rows or domain history directly.

### 10.2 Job ownership

| Work | Owner | Completion effect |
| --- | --- | --- |
| Runner provisioning/cleanup | Scheduler | Run/lease transition through Sessions port |
| Attachment/Artifact scan | Attachments and Artifacts | Scan state and Session event |
| Outbound Webhook delivery/redelivery | Integrations | Delivery attempt only; never Run outcome |
| Inbound Trigger occurrence recovery | Integrations | Resume the already committed occurrence outbox; never create a second Message or Run |
| Scheduled Trigger due scan | Integrations | Materialize one due occurrence and advance the next planned instant; skip uncommitted past instants after recovery |
| Evaluation execution/scoring | Evaluations | Evaluation Run and Gate projection |
| Audit export/signing | Audit | Immutable export package and expiry |
| Retention deletion/key destruction | Retention | Tombstone after hold and policy recheck |
| Provider/runner health probe | Platform | Bounded health projection, no config mutation |
| Outbox fan-out | Jobs and Outbox mechanics | Consumer checkpoint after idempotent handling |

## 11. Object-Store Consistency

PostgreSQL and object storage do not share a transaction. Byte lifecycles use
explicit states and repairable protocols:

1. Persist declaration and short-lived grant metadata.
2. Upload to a quarantine or staging key.
3. Verify size and digest, then record uploaded state.
4. Scan and authorize classification.
5. Promote by immutable reference or verified copy, then mark available.
6. Issue download grants only after a fresh authorization and lifecycle check.

Retries are content-addressed by digest. A database record cannot become
`available` before object verification. Orphan objects, missing objects, and
stuck metadata are found by a reconciliation worker. Deletion writes pending
intent, checks Legal Holds and minimum evidence retention, deletes bytes or
keys, and finally writes a digest-preserving tombstone.

## 12. Evaluation Isolation

Evaluation Runs use the same immutable Agent and Tool contracts but a distinct
execution purpose and environment profile. They cannot silently route to
Production credentials, live Tool endpoints, employee Sessions/Runs, or
Production Deployment state.

An Evaluation manifest pins Suite Version, Case, Agent Revision, Policy
Versions, fixture set, VCR mode, model route, sandbox image, scorer versions,
and budgets. Fixture resolution occurs before scheduling. A fixture miss fails
with `fixture_miss`; it never falls through to a live external call unless the
Run was explicitly created in an authorized record mode.

Evaluation completion updates only the Evaluation Run and Gate projection.
Publishing or moving a Production Deployment pointer remains a separate Agent
Registry command that consumes exact Gate evidence and rechecks current Policy.

## 13. Integration and Webhook Boundary

Integration Publications bind a client, Workspace, Agent, immutable input/output
contract, scopes, quotas, environment, and webhook policy. The enterprise API
submits through a dedicated application command that never fabricates a
Copilot requester.

Webhook delivery uses an immutable event ID and signed payload. Each attempt
records endpoint Version, payload digest, signature key reference, status,
timing, and safe response summary. Delivery is at least once; receivers dedupe
by event ID. Redelivery creates a new attempt for the same event and payload
digest, not a new Session event. Delivery failure never changes a completed Run
to failed.

Secret rotation changes a reference/version and affects new signing or client
authentication work. Secret values are resolved only inside the trusted broker
or delivery adapter and never stored in job, event, API, or Audit payloads.

## 14. Audit Without Dual Writes

Each domain command emits one canonical, schema-versioned Audit envelope in its
transaction. The Audit module owns indexing, integrity chaining, retention, and
export; resource modules own the truthful action/outcome facts.

Resource activity lists, Run timelines, webhook attempts, and Policy history
are domain projections. They may link to Audit IDs but cannot create alternate
Audit tables, signing rules, or export formats. Audit projection lag never rolls
back a successful domain command; the committed envelope remains recoverable
from the outbox/event store.

Sensitive content is referenced by digest and protected object identity.
Secrets, raw credentials, unrestricted prompts, and chain-of-thought are
forbidden in Audit. Redaction creates a derivative with provenance; it does not
rewrite signed history.

## 15. Restart and Failure Recovery

On process start, the control plane does not infer success from in-memory work.
Workers reclaim only expired claims, compare expected resource revisions, and
resume from durable checkpoints.

- `in_progress` idempotency records are reconciled against committed domain
  state before being reclaimed.
- Queued Runs and expired scheduler leases are claimed with a new fence.
- `executing` actions without a trustworthy outcome enter bounded
  reconciliation; non-repeatable effects are never automatically replayed.
- Outbox records remain until every required consumer checkpoint is durable.
- Attachment/Artifact state is reconciled with object existence and digest.
- Webhook attempts resume according to endpoint retry policy and deadline.
- Scheduled Trigger recovery resumes committed occurrence outboxes but does not
  backfill uncommitted planned instants that passed during downtime. The next
  instant is recomputed from validated cron and IANA time-zone data.
- Evaluation jobs resume from case/scorer checkpoints or fail explicitly when
  the environment is not reproducible.
- Expired approvals are transitioned durably by a worker; read-time expiry may
  accelerate the same idempotent command but is not the only production path.
- Cleanup jobs are independent of terminal Run outcome and surface exhausted
  cleanup as an operational incident.

A recovery action always emits correlation and Audit evidence. Operators use
typed reconciliation commands; direct database edits are outside normal
operation.

## 16. Authorization Placement

Authorization occurs twice where effects require it:

1. The application service authorizes the requested command and scope before
   loading or changing the aggregate.
2. The execution adapter rechecks current action-time authority immediately
   before a credential, model, Tool, webhook, download, or deletion effect.

Workers carry a workload identity plus the initiating actor/resource facts;
they do not inherit the full role of the process. A queued job is not a durable
authorization grant. Revocation, quarantine, expired Publication, Legal Hold,
Policy change, or lease loss can block work at execution time.

Denials return audience-safe problem codes and create required Audit evidence.
Authorization libraries may evaluate facts but do not fetch arbitrary domain
tables; the owning module supplies typed authorization facts.

## 17. Verification and Enforcement

Every module extension includes tests proportional to its boundary:

- Domain table tests cover every legal and illegal state transition.
- Repository tests cover revision CAS, uniqueness, scoped queries, sequence
  allocation, and rollback on injected failure.
- Command tests cover idempotent replay, conflicting keys, stale revisions, and
  concurrent winners.
- Authorization tests cover audience, organization, Workspace, principal,
  resource capability, non-leaking reads, and action-time revocation.
- Worker tests cover lease expiry, fencing, restart, backoff, deduplication,
  exhausted retries, and manual reconciliation.
- Object lifecycle tests cover database/object-store partial failure in both
  directions.
- Contract tests verify OpenAPI/protobuf generation and prevent privileged
  fields from entering Copilot or Enterprise projections.

Import rules should enforce that transport packages cannot be imported by
domain modules, development fixtures cannot be imported by production
composition, and Admin/Copilot generated response types do not cross audiences.
Architecture checks fail on direct cross-module table access or a new package
that combines handler, SQL, transition rules, worker loop, and fixture setup.

## 18. Extraction Criteria

A module remains in the monolith until measured needs justify extraction.
Candidate triggers are independent scaling, fault isolation, regulatory
segmentation, or stable separate ownership. Extraction requires a durable
protocol, independent deployment/rollback, tracing, idempotency, and failure
semantics; it is not justified solely by package size.

Until then, in-process application interfaces and one transaction preserve
correctness. No internal HTTP facade, duplicated DTO layer, or transitional
compatibility service is added in anticipation of a future split.
