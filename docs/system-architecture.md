# System Architecture

## 1. Architectural Style

Gantry begins as a modular control-plane monolith in Go, a separately deployed
Rust runner, two React applications, and infrastructure adapters. This is a
deployment decision, not permission to blur module boundaries.

The initial architecture favors transactional correctness, operability, and a
small number of deployables. Modules may be extracted only when measured scale,
failure isolation, or team ownership justifies the operational cost.

## 2. Logical Architecture

```text
Gantry Admin                 Gantry Copilot
     |                             |
     +---------- HTTPS/WSS --------+
                    |
Enterprise Systems -- OAuth/HTTPS/Webhooks
                    |
             Edge / API Gateway
                    |
        +-----------+------------+
        | Go Control Plane       |
        |                        |
        | Identity Context       |
        | Agent Registry         |
        | Task Service           |
        | Policy Decision Point  |
        | Approval Service       |
        | Scheduler              |
        | Event Service          |
        | Evaluation Service     |
        | Credential Broker      |
        | LLM Router             |
        +----+-----------+-------+
             |           |
       PostgreSQL    Object Storage
             |
       Durable Work Queue
             |
      Runner Gateway (gRPC)
             |
   +---------+-----------------------------+
   | Isolated Sandbox                      |
   | Rust Agent Runner                     |
   | PTY / MCP clients / artifact staging  |
   | Optional VCR sidecar in eval mode     |
   +---------------------------------------+
```

## 3. Control-Plane Modules

### Edge and API Gateway

- Terminates TLS and validates the OAuth access-token audience.
- Applies request limits, body limits, correlation IDs, and coarse routing.
- Does not replace resource-level authorization in services.
- Separates Admin, Copilot, and enterprise Agent Invocation API surfaces even
  when they share internal code.

### Identity Context

Normalizes OIDC subject, organization, groups, roles, workspace membership,
service-account scopes, and authentication strength into a request context. It
does not persist external identity-provider passwords or sessions.

### Agent Registry

Owns agents, drafts, immutable versions, publication assignments, tool bindings,
schemas, policy references, and release metadata. It performs static validation
and computes change-risk summaries.

### Task Service

Accepts idempotent task submissions, binds the submission to immutable resource
versions, creates runs, owns run state transitions, and exposes cancellation
and retry commands.

For enterprise callers, it also validates the integration publication, binds
the application and optional delegated-user principals, validates versioned
input contracts, and produces the restricted external result projection.

### Integration Registry and Delivery

Owns confidential client registrations, integration publications, scopes,
contract assignments, webhook endpoints, delivery attempts, quotas, and
environment bindings. It signs webhook messages and manages at-least-once
delivery without changing task outcomes when delivery fails.

### Policy Decision Point

Evaluates actor, agent, version, workspace, tool, action, arguments, destination,
credential class, runtime context, and requested effect. It returns `allow`,
`deny`, or `require_approval` plus obligations such as redaction, limits, and
eligible approver sets.

The first implementation uses typed policy evaluators in Go backed by versioned
policy documents. A general policy language is deferred until real rules prove
the need.

### Approval Service

Creates durable approval requests, resolves eligible approvers, enforces
separation of duties, expiry, and single-use decisions, and appends the result
to the run event stream. It revalidates the action and approver authorization
immediately before execution resumes.

Concurrent approver decisions are appended with a uniqueness constraint on
request and approver. Each append locks or compare-and-swaps the approval
projection revision, recomputes the immutable threshold and rejection rule, and
attempts the bound action transition in the same database transaction. Exactly
one transaction may emit `approval.satisfied` and move the action to `ready`;
late decisions observe the terminal approval state and cannot create another
execution opportunity.

### Scheduler

Claims queued runs, selects a compatible runner pool, requests sandbox
provisioning, maintains leases, detects lost runners, and triggers cleanup. Queue
claims and state changes use durable database transactions.

Every assignment carries a monotonically increasing `lease_epoch`. Reassignment,
durable resume, forced cancellation, and terminal transition atomically advance
or invalidate the epoch. The LLM, tool, credential, and artifact gateways reject
requests unless the caller presents the current run identity and lease epoch.
This fences a disconnected or paused runner before it can create an external
effect after its lease has been lost.

### Event Service

Appends ordered run events, serves cursor-based history, fans out live events,
and produces audit and telemetry projections. It is the canonical history of a
run, while relational tables hold queryable current state.

The service distinguishes semantic events from high-frequency content streams.
Lifecycle, policy, tool, approval, artifact, and stream-boundary events use the
transactional event log. Model token deltas and PTY bytes are batched into
immutable content segments and referenced by ordered stream events. Live fan-out
may publish buffered deltas before a segment closes, but acknowledged durable
cursors never advance past content committed to object storage and indexed in
PostgreSQL.

### Credential Broker

Maps authorized credential references to short-lived provider or enterprise
credentials. Credentials are injected only into the trusted outbound request
path and are never returned to the runner as durable plaintext.

### LLM Router

Selects a provider/model from a versioned model policy, enforces budgets and
data-handling constraints, injects provider credentials, normalizes streaming
responses and usage, and records the material route without logging secrets.

### Evaluation Service

Builds immutable evaluation manifests, provisions clean fixtures, coordinates
VCR replay and assertions, scores results, compares a candidate to a baseline,
and enforces publication gates.

## 4. Storage Architecture

### PostgreSQL

PostgreSQL is the source of truth for configuration, identity mappings,
authorization assignments, tasks, runs, approvals, event metadata, evaluation
definitions, and outbox records.

Semantic run events use append-only rows with a per-run sequence number.
Current-state tables are updated in the same transaction as the event append.
An outbox table reliably publishes notifications, stream fan-out messages, and
asynchronous projection work. PostgreSQL does not receive one transaction per
model token or PTY fragment: high-frequency content is buffered under bounded
memory and latency limits, uploaded as immutable segments, then indexed with one
transaction containing its stream offset range, digest, size, object key, and
retention class.

### Object Storage

S3-compatible object storage holds artifacts, model-output and terminal stream
segments, encrypted snapshots, VCR fixtures, evaluation attachments, and export
bundles. Database records contain digests, size, media type, encryption metadata,
retention class, object keys, and stream offset ranges.

### Cache and Ephemeral Coordination

Redis may be introduced for rate limiting, presence, and fan-out optimization,
but correctness must not depend on Redis. The first implementation may use
PostgreSQL advisory locks and notifications where adequate.

## 5. Rust Agent Runner

The runner is a small, non-privileged daemon packaged into a pinned sandbox
image. Static linking is a target where supported, but a binary-size promise is
not a release criterion until dependencies and target platforms are measured.

Runner responsibilities:

- Establish an outbound mutually authenticated gRPC stream to the control
  plane; no inbound sandbox port is required.
- Materialize the signed run manifest and reject invalid or expired manifests.
- Start and retain a PTY session through `portable-pty` when shell access is
  enabled.
- Execute the agent loop using control-plane LLM routing and authorized tools.
- Act as an MCP client for approved built-in and application servers.
- Enforce local defense-in-depth limits for commands, paths, process count,
  output size, and deadlines.
- Stage artifacts and send content digests before upload.
- Emit heartbeats, structured events, terminal output, usage, and terminal
  status.
- Handle cancellation and terminate descendant processes before exit.

The control plane remains authoritative for policy. Runner-side checks prevent
obvious bypasses and fail closed during control-plane disconnection.

## 6. Tool Namespace and Resolution

Every tool has a stable fully qualified name:

- `builtin::<server>/<tool>` for platform-provided tools
- `app::<server>/<tool>` for enterprise application tools

Agent specs reference fully qualified names. Short aliases may be displayed in
the UI only when unambiguous. A new tool may not silently shadow or replace an
existing binding. Tool descriptors and schemas are content-addressed and pinned
in published agent versions.

## 7. Sandbox Model

### Development

Docker Compose provides PostgreSQL, object storage, the control plane, local
runner workers, mock identity, and optional local model/provider adapters.
Containers use explicit resource limits and isolated networks even in local
development.

MinIO is the development S3-compatible object-storage adapter. The control
plane owns an `ObjectStore` port, so no domain model or service accepts MinIO
SDK types. Public HTTP is on the OpenAPI listener, while the private runner
listener exposes only the Connect-served gRPC session.

### Production

Kubernetes is the production scheduler. Each run receives an ephemeral Pod or
equivalent sandbox with:

- gVisor runtime class by default for untrusted tool and shell execution.
- Non-root user, read-only root filesystem, dropped Linux capabilities, and
  seccomp/AppArmor or equivalent profiles.
- An ephemeral writable workspace and only explicitly declared read-only
  mounts.
- CPU, memory, process, disk, output, and wall-clock limits.
- Default-deny ingress and egress network policy.
- A hard TTL and cleanup finalizer.
- Workload identity limited to contacting Gantry gateways.

Jobs needing stronger isolation can later use a microVM-backed runner pool.

## 8. Run Lifecycle

```text
Accepted
  -> Queued
  -> Provisioning
  -> Running
      -> AwaitingApproval -> Running
      -> Suspended -> Queued/Running
      -> Canceling -> Canceled/Failed
      -> Completed
      -> Failed
Provisioning -> Failed/Canceled
Queued -> Canceled/Expired
AwaitingApproval -> Running/Failed/Canceled
```

An employee task creates a task record and an initial run. The scheduler claims
the run with a lease, provisions a sandbox, and sends a signed manifest. The
runner emits events with monotonically increasing client sequence numbers and
presents the assignment's lease epoch to every trusted gateway. The control
plane assigns canonical run sequence numbers and acknowledges receipt only after
the semantic event or referenced content segment is durable.

Every proposed external effect has a durable action record. Authorization moves
it from `proposed` to `ready` or `awaiting_approval`; a valid approval can move
the exact action digest to `ready`. Immediately before invocation, the gateway
uses a compare-and-swap transaction to move `ready` to `executing`, validates
the current lease epoch, run state, policy, approval, and cancellation state,
and issues a single-use execution permit. Completion records `succeeded`,
`failed`, or `unknown_outcome`. Only the owner of the permit may report the
result, and a non-repeatable `unknown_outcome` requires reconciliation rather
than automatic replay.

The first agent loop permits only one action to be `awaiting_approval`, `ready`,
or `executing` at a time. This serial product behavior does not remove the need
for concurrency control: duplicate decisions, approval expiry, cancellation,
policy revocation, lease loss, webhook retries, and durable resume can race in
separate processes. All transitions therefore use expected state and revision,
and terminal or superseding transitions invalidate unconsumed execution permits.

Consuming the execution permit is the linearization point. Cancellation that
wins before the claim prevents invocation. Cancellation received after the
permit is consumed requests cooperative interruption, but cannot claim that an
external effect did not occur; the action records its observed result or
`unknown_outcome`, and the run remains canceling until reconciliation policy has
been applied. An unresolved result reaches `Failed` with reason
`action_outcome_unknown` by a bounded reconciliation deadline; later evidence is
recorded as a reconciliation event and never rewrites the original observation.

`Rejected` and approval `Expired` are approval outcomes, not run states. The
immutable approval policy specifies whether either outcome resumes the loop with
a structured action-denied result or terminates the run as `Failed` with reason
`approval_rejected` or `approval_expired`. Cancellation always wins before an
execution permit is consumed. Task aggregate state is then derived from the run
rather than inventing a separate approval terminal state.

## 9. Suspension and Recovery

Message history alone cannot restore arbitrary PTY and process state. Gantry
therefore defines two forms of suspension.

### Retained Suspension

The sandbox and PTY remain alive for a bounded TTL while waiting for a short
approval or operator action. The scheduler retains the lease and continues
resource accounting. TTL expiry transitions to durable suspension or failure
according to agent policy.

### Durable Suspension

The agent may suspend only at a declared checkpoint where no external action is
in flight. Gantry persists:

- Event cursor and normalized conversation state.
- Agent loop state and pending action descriptor.
- Workspace snapshot or artifact references when configured.
- Tool checkpoint data for tools that explicitly support it.
- All immutable configuration and policy references.

The sandbox is then destroyed. Resume creates a new sandbox and restores the
checkpoint. Arbitrary background processes, sockets, and uncommitted database
transactions are not restored. Agent authors must treat a durable checkpoint as
a process boundary.

## 10. Failure Semantics

- At-least-once command delivery is paired with idempotency keys and expected
  state revisions.
- A tool action declares whether it is read-only, idempotent, compensatable, or
  non-repeatable.
- Gantry never automatically retries a non-repeatable action with an unknown
  result.
- Runner lease loss transitions the run to `Failed` unless a valid durable
  checkpoint allows a new attempt.
- Gateways reject stale lease epochs even if the old runner's workload identity
  or assignment signature has not yet expired.
- Cancellation is cooperative first, then forceful after a grace period.
- Cleanup failures are observable platform incidents and retried independently
  from the terminal run result.

## 11. Deployment Units

Initial deployables:

1. `gantry-admin-web`
2. `gantry-copilot-web`
3. `gantry-control-plane`
4. `gantry-runner` sandbox image
5. `gantry-vcr-proxy` sidecar for evaluation mode
6. PostgreSQL
7. S3-compatible object storage

The control-plane binary may expose separate Admin, Copilot, enterprise Agent
Invocation, internal runner, and provider-gateway listeners so network policy
can enforce their distinct trust levels.

## 12. Recommended Repository Layout

```text
apps/
  admin-web/
  copilot-web/
control-plane/
  cmd/gantry/
  internal/agentregistry/
  internal/tasks/
  internal/policy/
  internal/approvals/
  internal/scheduler/
  internal/events/
  internal/credentials/
  internal/llmrouter/
  internal/evaluation/
runner/
  crates/runner/
  crates/protocol/
  crates/pty/
  crates/mcp/
  crates/policy-enforcement/
sidecars/
  vcr-proxy/
packages/
  design-system/
  api-client/
  contracts/
deploy/
  compose/
  helm/
docs/
```

Contract schemas are generated into language-specific clients. Generated code
is not edited manually.

## 12.1 Development Topology

Moon coordinates all workspace tasks. Compose exposes PostgreSQL (`5432`),
MinIO API (`9000`), MinIO console (`9001`), public OpenAPI HTTP (`8080`), and
the runner gRPC listener (`8081`). Admin and Copilot Vite servers run outside
Compose on `3001` and `3002`, proxying `/api` to the public listener. See the
[Developer Guide](developer-guide.md) for bootstrap, environment variables, and
the generated-code check.

## 13. Observability

All services use OpenTelemetry-compatible traces, metrics, and structured logs.
Every task, run, approval, tool call, provider request, sandbox, and artifact has
a stable identifier included in telemetry.

Required platform signals include:

- Request and stream latency, errors, and saturation.
- Queue age, claim latency, active leases, and runner capacity.
- Sandbox provisioning and cleanup duration.
- Model latency, tokens, normalized cost estimate, and provider error class.
- Tool latency, denial, approval, timeout, and unknown-result counts.
- Event append lag and WebSocket fan-out lag.
- Approval backlog and age.
- Evaluation fixture mismatch and contamination indicators.

Sensitive prompt, payload, output, and credential content is excluded from
telemetry by default.
