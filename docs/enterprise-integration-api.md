# Enterprise Agent Invocation API

## 1. Purpose

Enterprise applications such as human resources, finance, CRM, service desk,
and procurement systems can invoke published Gantry agents through a server-to-
server API. The calling application keeps its own user interface and business
workflow. Gantry provides governed agent execution, tools, approvals, artifacts,
events, and audit evidence.

This is API integration, not UI embedding. An HR management system may, for
example, submit employee policy data to an approved HR agent and display the
structured answer inside the HR system without loading the Gantry Copilot web
application.

The Enterprise Agent Invocation API is separate from the browser-oriented
Copilot API. Both create the same durable Task and Run identities and use the
same authorization, action-time execution, and audit model, but each API returns
an independently authorized projection. Enterprise responses are limited by the
published Integration contract and never become a shortcut to Admin diagnostics
or the Copilot conversation. A business workflow approval remains owned by the
enterprise tool that defines that workflow.

## 2. Integration Roles

### Enterprise Application

A registered confidential OAuth client representing one enterprise system,
such as `hr-management-production`. Registration records:

- Owner and operational contacts.
- Allowed organization and workspaces.
- Allowed agents or integration publications.
- Permitted invocation and result scopes.
- Approved authority modes.
- Rate, concurrency, token, cost, attachment, and artifact limits.
- Allowed webhook destinations and authentication method.
- Credential rotation and expiry policy.
- Data classification and environment.

### Subject User

An employee on whose behalf the application may invoke an agent. The subject is
present only in delegated-user mode and must come from a verifiable token
exchange. A caller-supplied employee ID or email is business input, not proof of
identity.

### Agent Principal

The runtime identity of the immutable deployed Agent Revision. Its maximum tool,
credential, data, and network authority remains independent of the calling
application.

## 3. Authority Modes

### Application Identity

The enterprise application uses OAuth 2.0 client credentials and invokes an
agent as itself. This is suitable for system-owned workflows such as nightly
policy classification or processing an HR queue.

Effective authority is the intersection of:

1. The integration client's grants.
2. The integration publication's agent and workspace assignment.
3. The deployed Agent Revision's maximum permissions.
4. Organization and workspace policy.
5. Tool, credential, data, and network policy at action time.

The application cannot claim an employee's permissions. It cannot create a
requester-bound human approval because it has no authenticated human requester;
an action requiring human confirmation must be denied or handled by the owning
external business workflow. Application identity therefore does not enter the
Copilot requester-input flow.

### Delegated User Identity

The HR system acts for its currently authenticated employee. Its backend
exchanges a verifiable user token for a short-lived Gantry token using OAuth 2.0
Token Exchange or a deployment-approved equivalent. Gantry records both the
calling application and subject user.

Effective authority also intersects the subject user's current permissions.
The application cannot select another subject through request JSON, forward a
browser token directly to the agent, or turn a delegated call into application-
wide authority.

## 4. Publication to Integrations

An agent is not callable through this API merely because it appears in Gantry
Copilot. Administrators create an **integration publication** that binds:

- One immutable Agent Revision or a controlled release channel.
- One or more registered integration clients.
- Workspace and environment.
- Allowed authority modes.
- Input and output contract versions.
- Exposed artifact types.
- Allowed result-event projection.
- Approval behavior and callback policy.
- Quotas, budgets, retention, and expiry.

This makes external API exposure an explicit reviewed release action. Changing
the integration contract or broadening authority creates a new publication
revision.

## 5. Canonical Invocation Flow

```text
HR browser -> HR backend -> OAuth token -> Gantry Invocation API
                                      -> validate integration publication
                                      -> validate input and business context
                                      -> create Task and Run
                                      -> execute published HR Agent
                                      -> poll or signed webhook events
HR backend <- structured result/artifact references <- Gantry
```

The HR backend, not browser JavaScript, holds client credentials and calls
Gantry. The canonical flow is asynchronous because tools and approvals may take
longer than an HTTP request timeout.

## 6. API Surface

Audience: `gantry-agent-api`.

Representative endpoints:

- `GET /api/agent/v1/agents`
- `GET /api/agent/v1/agents/{agent_id}`
- `POST /api/agent/v1/tasks`
- `GET /api/agent/v1/tasks/{task_id}`
- `GET /api/agent/v1/tasks/{task_id}/events`
- `POST /api/agent/v1/tasks/{task_id}:cancel`
- `POST /api/agent/v1/tasks/{task_id}:retry`
- `GET /api/agent/v1/artifacts/{artifact_id}`
- `POST /api/agent/v1/webhook-endpoints`
- `GET /api/agent/v1/webhook-deliveries/{delivery_id}`

Agent discovery returns only integration publications assigned to the calling
client. It exposes public metadata, input/output JSON Schemas, contract version,
authority modes, and limits, not the system prompt or internal tool bindings.

## 7. Task Submission Contract

Example application-identity request:

```http
POST /api/agent/v1/tasks
Authorization: Bearer <application-access-token>
Idempotency-Key: hr-case-84721-policy-review-v1
Content-Type: application/json
Prefer: respond-async
```

```json
{
  "agent_id": "agt_hr_policy_advisor",
  "contract_version": "2026-08-01",
  "input": {
    "question": "Is this employee eligible for parental leave?",
    "employee": {
      "employee_reference": "emp_84721",
      "country": "CN",
      "employment_type": "full_time",
      "start_date": "2023-03-01"
    }
  },
  "context": {
    "source_system": "hr-management",
    "source_resource": {
      "type": "employee_case",
      "id": "case_29318"
    },
    "correlation_id": "hr-flow-75931"
  },
  "delivery": {
    "webhook_endpoint_id": "whe_hr_production",
    "events": ["task.awaiting_approval", "task.completed", "task.failed"]
  }
}
```

Response:

```http
HTTP/1.1 202 Accepted
Location: /api/agent/v1/tasks/tsk_01...
```

```json
{
  "id": "tsk_01...",
  "status": "queued",
  "agent": {
    "id": "agt_hr_policy_advisor",
    "version": "12",
    "contract_version": "2026-08-01"
  },
  "authority": {
    "mode": "application",
    "client_id": "int_hr_management_production"
  },
  "created_at": "2026-08-13T08:00:00Z",
  "links": {
    "self": "/api/agent/v1/tasks/tsk_01...",
    "events": "/api/agent/v1/tasks/tsk_01.../events"
  }
}
```

`Prefer: wait=<seconds>` may wait up to the server's configured maximum and
return a completed result when available. Otherwise the same request returns
`202 Accepted`; it never starts a second synchronous execution path.

## 8. Input and Business Context

Agent input is validated against the published JSON Schema. The optional
`context` envelope provides traceability and typed references, not additional
authority.

- The application may send only fields allowed by the integration contract.
- Resource references are treated as claims until a trusted tool resolves and
  authorizes them.
- Gantry records source system and correlation IDs for audit and support.
- Sensitive inputs are classified, encrypted, redacted from infrastructure
  logs, and retained according to the publication policy.
- Caller-supplied prompt fragments cannot replace system instructions, enable
  tools, select credentials, broaden egress, or choose a mutable Draft.
- Large input documents use the governed attachment upload flow rather than
  inline base64 payloads.

## 9. Result Contract

Completed tasks expose a stable application-oriented projection:

```json
{
  "id": "tsk_01...",
  "status": "completed",
  "result": {
    "contract_version": "2026-08-01",
    "output": {
      "eligible": true,
      "policy_basis": ["CN-PARENTAL-LEAVE-4.2"],
      "explanation": "The supplied employment record satisfies the policy."
    },
    "artifacts": []
  },
  "usage": {
    "duration_ms": 8421
  },
  "completed_at": "2026-08-13T08:00:08Z"
}
```

The output is validated against the published output schema before the task can
be marked successfully completed. The external projection excludes private
reasoning, terminal streams, internal prompts, credentials, and administrator-
only policy details.

## 10. Status and Event Projection

API clients may poll the task or retrieve cursor-paginated events. The external
event projection includes only integration-relevant states:

- `task.accepted`
- `task.running`
- `task.awaiting_approval`
- `task.suspended`
- `task.completed`
- `task.failed`
- `task.canceled`
- `artifact.available`

Internal model deltas, PTY output, tool payloads, policy internals, and raw run
events are not exposed through the Agent Invocation API.

For the current Enterprise projection, approval rejection and expiry return a
schema-valid `action_denied` or `approval_expired` result and do not add a new
externally visible Task state. Interactive continuation after that result is a
Copilot capability. A future delegated-user continuation API must define its own
message route and event projection before Enterprise tasks are described as
interactive.

## 11. Webhooks

Webhooks provide asynchronous status and terminal result notification.

- Endpoint registration is an administrator-approved operation.
- Destinations use HTTPS, pass SSRF and private-network policy, and are bound to
  an integration client and environment.
- Deliveries contain event ID, delivery ID, task ID, event type, timestamp,
  attempt, contract version, and a minimal payload.
- Gantry signs the timestamp and raw body with an endpoint secret or uses mTLS.
- Consumers reject old timestamps and duplicate delivery IDs.
- Delivery is at least once with exponential backoff and a finite retry window.
- Ordering is guaranteed per task through event sequence, not across tasks.
- A delivery log supports inspection and explicit redelivery.
- Webhook failure does not change the task result.

Sensitive result fields may be omitted from webhooks. The consumer fetches the
authorized task result from the API when policy requires it.

## 12. Approvals

An API-started task may enter `awaiting_approval`.

- Application-identity calls may execute only actions that policy allows without
  human confirmation. An application credential cannot stand in for a human
  requester or nominate an administrator as approver.
- Delegated-user calls route any required Agent action approval only to the
  verified subject user who initiated the task.
- The enterprise application receives status and a safe approval reference; it
  cannot approve merely by possessing the application client credential.
- Agent action approvals are decided only by the authenticated task requester
  through the Copilot approval surface or a requester-authenticated approval
  API. The decision is bound to the exact action digest.
- Business approvals such as leave, expense, or purchase approval are initiated
  and presented by the tool's own workflow system. Gantry receives only a
  signed, idempotent external status callback and resumes the action after
  validating the external approval reference and bound digest.
- Agent action rejection and expiry return a structured denial result in the
  Enterprise projection. Copilot may keep the requester conversation open, but
  that interaction is not currently part of this server-to-server contract.

## 13. Artifacts

- The application may access only artifact types exposed by the integration
  publication.
- Downloads require an authorized API request and return a short-lived object
  URL or streamed content.
- Artifact access is audited independently of task access.
- Active content is served from a separate origin with safe disposition and
  preview rules.
- Retention and deletion follow the task's integration publication policy.

## 14. Reliability and Idempotency

- Every task submission requires an `Idempotency-Key` unique within the
  integration client. The mapping is retained through the task's terminal state
  plus the published maximum client retry interval. After task content is
  deleted, a tombstone containing the request digest and original task ID still
  prevents key reuse for that interval.
- Repeating the same key and semantically identical request returns the original
  task. Reusing the key with a different request returns a conflict.
- The API publishes its minimum idempotency retention interval. It never creates
  a new task merely because the original response body has expired.
- If the same request is repeated after the original task has been deleted but
  its idempotency tombstone remains, the API returns
  `410 idempotency_resource_expired` and the original task ID. The caller must
  make an explicit new business decision with a new key before starting another
  task.
- The application uses its own correlation ID for end-to-end tracing.
- Cancellation, retry, and webhook redelivery are idempotent commands.
- A non-repeatable tool action with an unknown outcome is never automatically
  replayed.
- API clients must handle `429` with `Retry-After`, transient `5xx`, token
  expiry, and task states that outlive the caller process.

## 15. Security Requirements

- Confidential clients authenticate using private-key JWT or mTLS where
  possible; shared client secrets are a lower-assurance compatibility option.
- Client credentials never appear in browsers, mobile packages, URLs, task
  inputs, or Agent prompts.
- Access tokens are short lived, audience restricted, scope restricted, and
  bound to the registered environment.
- Each invocation is authorized against the current integration publication.
- Agent and tool authorization is re-evaluated at action time.
- The caller cannot request arbitrary model providers, tools, credentials,
  network destinations, runtime images, or Agent Revisions.
- Input, output, webhook, attachment, and artifact sizes are bounded.
- Audit records identify the integration client, subject user when present,
  source correlation, immutable Agent Revision, policy decisions, and outcome.

## 16. HR System Example

For an employee policy assistant inside an HR system:

1. An administrator publishes `HR Policy Advisor` to the registered HR client.
2. The HR backend obtains an application token or exchanges the signed-in
   employee's token for a delegated Gantry token.
3. The backend submits typed employee and case data using an idempotency key.
4. Gantry binds the task to the published HR Agent version and executes it.
5. If the Agent proposes a protected HR action, current policy may require a
   human approval.
6. The HR backend receives a signed terminal webhook or polls the task.
7. It renders the schema-validated result in its own HR interface.
8. Gantry retains the complete governed audit trail; the HR system retains its
   own business record and Gantry task reference.

## 17. SDK and Contract Distribution

The protocol remains usable with ordinary HTTP clients. Gantry should generate
and publish server-side SDKs only after the OpenAPI contract stabilizes, starting
with languages used by target enterprise systems. SDKs provide authentication,
idempotency, polling, webhook verification, error types, and schema models; they
must not hide task states or retry non-repeatable actions automatically.
