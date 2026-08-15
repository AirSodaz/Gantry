# API and Protocols

## 1. Protocol Strategy

- OpenAPI 3.1 is the source contract for browser-facing and enterprise HTTPS
  JSON APIs. Generated TypeScript clients consume those documents.
- Browser-facing APIs use versioned HTTPS JSON endpoints and WebSocket event
  streams.
- Enterprise applications use a separate server-to-server Agent Invocation API
  with OAuth client identity, polling, and signed webhooks.
- Internal control-plane modules call each other in-process initially.
- Runner communication uses bidirectional gRPC over mutual TLS. The Go gateway
  uses Connect only for this private runner listener; no public HTTP API offers
  Connect or protobuf endpoints.
- LLM and enterprise API calls use trusted gateways with provider-specific
  adapters.
- Contracts are defined once and generate Go, Rust, and TypeScript types.

GraphQL is not required for the first release. Resource-oriented APIs and typed
stream events provide clearer authorization and operational behavior.

The browser Copilot API and server-to-server Agent Invocation API are different
consumption surfaces. An enterprise HR backend uses the Agent Invocation API;
it does not impersonate the Gantry Copilot frontend or reuse its browser token.

## 2. API Separation

### Admin API

Audience: `gantry-admin-api`.

Representative resources:

- `/api/admin/v1/workspaces`
- `/api/admin/v1/agents`
- `/api/admin/v1/agents/{id}/draft`
- `/api/admin/v1/agents/{id}/versions`
- `/api/admin/v1/publications`
- `/api/admin/v1/runs`
- `/api/admin/v1/approval-policies`
- `/api/admin/v1/evaluation-suites`
- `/api/admin/v1/tools`
- `/api/admin/v1/policies`
- `/api/admin/v1/audit-events`
- `/api/admin/v1/platform/runner-pools`

### Copilot API

Audience: `gantry-copilot-api`.

Representative resources:

- `/api/copilot/v1/agents`
- `/api/copilot/v1/tasks`
- `/api/copilot/v1/tasks/{id}`
- `/api/copilot/v1/tasks/{id}/runs`
- `/api/copilot/v1/approvals`
- `/api/copilot/v1/artifacts`

The Copilot API uses employee-oriented response types. It does not return raw
agent specs and rely on the frontend to hide privileged fields.

### Enterprise Agent Invocation API

Audience: `gantry-agent-api`.

This API allows registered enterprise systems such as HR management to invoke
agents explicitly published to them. It supports application identity and
verified delegated-user identity. It creates the same task/run resources but
returns a stable integration projection and never exposes Admin internals.

Representative resources:

- `/api/agent/v1/agents`
- `/api/agent/v1/tasks`
- `/api/agent/v1/tasks/{id}`
- `/api/agent/v1/tasks/{id}/events`
- `/api/agent/v1/artifacts/{id}`
- `/api/agent/v1/webhook-endpoints`

The full authentication, publication, request, result, webhook, approval, and
idempotency contract is defined in
[Enterprise Agent Invocation API](enterprise-integration-api.md).

## 3. HTTP Conventions

- Resource IDs are opaque and never encode authorization-relevant data.
- Creation and command endpoints accept `Idempotency-Key`.
- An idempotent response is recoverable for at least the resource lifetime plus
  the published maximum client retry interval. After response content expires,
  a tombstone still prevents the same actor and route from reusing the key with
  a different request digest.
- If the key and request digest match but the original resource has been deleted,
  the server returns `410 idempotency_resource_expired` with the original opaque
  resource ID; it never recreates the command implicitly.
- Mutable drafts use ETags and `If-Match` for optimistic concurrency.
- Lists use cursor pagination and stable, documented sort orders.
- Timestamps use UTC RFC 3339.
- Errors use a stable code, human-readable message, correlation ID, and optional
  field violations.
- Authorization failures do not reveal whether an inaccessible resource exists.
- Long-running commands return the accepted resource and current state rather
  than holding the HTTP request open.

Example error:

```json
{
  "error": {
    "code": "approval_expired",
    "message": "This approval request has expired.",
    "correlation_id": "cor_01...",
    "details": []
  }
}
```

## 4. Key Commands

### Submit a Task

`POST /api/copilot/v1/tasks`

The request includes the agent ID, structured input or message, attachment IDs,
and client-generated idempotency key. The response binds the task to the
effective published agent version and returns the first run.

Enterprise applications submit through `POST /api/agent/v1/tasks`. That route
requires an integration publication and a separate API audience; it is not a
service-account shortcut into the employee Copilot endpoint.

### Cancel a Run

`POST /api/copilot/v1/tasks/{task_id}/runs/{run_id}:cancel`

Cancellation is an idempotent command. The response reports `canceling` or the
already terminal state.

### Retry a Task

`POST /api/copilot/v1/tasks/{task_id}:retry`

The server determines whether to reuse the original agent version or current
published version according to an explicit request field and permission.

### Record an Agent Action Approval Decision

`POST /api/copilot/v1/approvals/{id}:decide`

The request includes decision, reason, action digest, and idempotency key. A
stale action digest is rejected. This route is only for a concrete agent action;
it is not a general enterprise business-approval endpoint.

### Publish an Agent Version

`POST /api/admin/v1/agents/{id}/draft:publish`

The command includes the expected draft revision, release notes, publication
targets, and acknowledged warnings. Policy determines required reviews and
evaluation gates.

## 5. Browser Event Stream

Clients first create a 60-second, task-bound event ticket with
`POST /api/copilot/v1/tasks/{task_id}/events:ticket`, then connect to a task or run stream:

`WSS /api/copilot/v1/tasks/{task_id}/events?ticket={ticket}&after={cursor}`

Admin has a corresponding run endpoint with richer event types.

Server frames contain:

```json
{
  "cursor": "cur_...",
  "event": {
    "sequence": 42,
    "type": "tool.call.started",
    "occurred_at": "2026-08-13T08:00:00Z",
    "payload": {}
  }
}
```

Protocol requirements:

- Cursors are opaque, resumable, and bounded to the authorized resource.
- Durable cursors advance only through committed semantic events and content
  segments. Live frames after the last durable cursor are marked provisional
  and include stream byte offsets.
- The server sends heartbeats and closes streams when authorization is revoked.
- Clients reconnect with exponential backoff and request events after the last
  rendered cursor.
- If a cursor is older than retained history, the server returns
  `cursor_expired`, the earliest available cursor, and a current resource
  snapshot. It never silently skips missing content.
- The server may coalesce high-frequency deltas for slow consumers but must
  preserve stream offsets, ordered content, and terminal states.
- Each application receives only its permitted event projection.
- The stream sends an initial resource snapshot and 20-second heartbeats. A
  ticket expiry closes the stream; clients request a new ticket before reconnecting.

## 6. Runner gRPC Protocol

The runner opens an outbound `RunnerSession` bidirectional stream.

The local Compose smoke stack uses plaintext HTTP/2 on the private Docker
network. Deployment environments terminate TLS and require a runner client
certificate before the Connect handler is reachable.

### Runner to Control Plane

- `RegisterRunner`
- `Heartbeat`
- `RunAccepted`
- `RunEventBatch`
- `TerminalOutput`
- `ArtifactDeclaration`
- `CheckpointAvailable`
- `RunFinished`

### Control Plane to Runner

- `AssignRun`
- `AcknowledgeEvents`
- `ApprovalResolution`
- `CancelRun`
- `SuspendRun`
- `ResumeAction`
- `RotateSession`
- `DrainRunner`

Every message includes protocol version, runner session, message ID, correlation
identifiers, and the run's `lease_epoch` where applicable. Run assignments are
signed and expiring. The runner rejects assignments whose image, capability,
organization, or protocol requirements do not match. Gateways independently
reject a stale epoch; assignment expiry alone is not the fencing mechanism.

Flow control limits unacknowledged event bytes and acknowledgement age. Model
and terminal content use bounded batches and may use separate streams so verbose
output cannot starve lifecycle, cancellation, lease, or approval messages.

## 7. LLM Gateway Protocol

The runner sends a normalized request containing messages, tool schemas, model
policy reference, classification, budget remaining, a short-lived run identity,
and the current lease epoch. The gateway:

1. Authorizes the run and policy.
2. Selects an allowed provider route.
3. Applies provider-specific formatting and credentials.
4. Streams normalized output and tool-call proposals.
5. Records usage, latency, route, and provider request identifiers.

Provider request and response bodies are not placed in infrastructure logs.
Fallback is constrained by the same classification and policy.

## 8. Tool Gateway Envelope

For mediated tools, the runner sends:

- Run and actor identity.
- Agent and tool-binding version.
- Fully qualified tool name and descriptor digest.
- Canonical arguments.
- Stable tool-call and idempotency IDs.
- Expected effect and destination.
- Approval reference when applicable.
- Current lease epoch and expected action revision.

The gateway canonicalizes again and atomically claims the durable action by
expected state, revision, and lease epoch. It re-evaluates current policy,
validates approval and cancellation state, then creates a single-use execution
permit before resolving credentials or contacting the target. A stale runner,
duplicate approval, or concurrent claimant therefore fails before any effect.
The gateway records the result against the permit and returns a normalized
result and audit reference. Unknown outcomes of non-repeatable actions require
operator reconciliation and are never automatically claimed again.

The successful action claim is the execution linearization point. A cancellation
that commits first prevents the claim. If cancellation arrives after the permit
is consumed, the gateway attempts interruption but reports `unknown_outcome`
unless the target result can be established; cancellation never rewrites an
observed external result.

In evaluation mode, the signed manifest identifies the fixture set and every
mediated tool invocation is routed to an evaluation adapter or replay proxy.
The gateway must reject fixture misses before resolving production credentials,
DNS, or network destinations. Direct gateway execution against a real target is
forbidden in evaluation mode.

Tool servers must not trust claims supplied only inside model-generated
arguments.

## 9. Artifact Protocol

- Clients upload attachments through pre-authorized, size-limited upload URLs.
- Uploaded objects remain quarantined until validation and malware scanning
  complete.
- Runners declare output metadata and digest before receiving a short-lived,
  lease-bound upload authorization. The runner uploads to Gantry's private
  endpoint and never receives object-storage credentials.
- Downloads require a fresh authorization check; object-storage URLs are short
  lived and scoped to one object.
- Preview generation occurs in an isolated service and never executes active
  content in the application origin.

## 10. Compatibility and Versioning

- Public API path versions change only for breaking HTTP contract changes.
- Event and protobuf messages evolve additively within a supported major
  protocol version.
- Control plane and runner advertise minimum and maximum protocol versions.
- Deployment prevents scheduling to an incompatible runner.
- Agent, tool, policy, and evaluation schemas have independent versions.
- A compatibility test suite runs against the oldest supported runner and web
  client contracts before release.

## 11. Limits

Organization and workspace policy define task input size, attachment count and
size, event payload size, terminal throughput, artifact quotas, concurrent runs,
approval lifetime, model budgets, tool timeouts, and stream connections.

Limit failures use explicit error codes and generate operational metrics. Silent
truncation is prohibited for commands, tool arguments, approval payloads, and
audit-relevant data.
