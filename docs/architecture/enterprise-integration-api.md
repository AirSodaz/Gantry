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

The typed target route, schema, authorization, and inbound automation trigger
contract is in
[Enterprise Agent API Contracts](enterprise-agent-api-contracts.md).

The Enterprise Agent Invocation API is separate from the browser-oriented
Copilot API. Both create the same durable Session and Run identities and use the
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
- Approved delegated-user invocation policy.
- Rate, concurrency, token, cost, attachment, and artifact limits.
- Allowed webhook destinations and authentication method.
- Credential rotation and expiry policy.
- Data classification and environment.

### Subject User

An employee on whose behalf the integration client may invoke an Agent. Every
Enterprise Session/Run invocation has a subject user from a verifiable token exchange. A
caller-supplied employee ID or email is business input, not proof of identity.

### Agent Principal

The runtime identity of the immutable deployed Agent Revision. Its maximum tool,
credential, data, and network authority remains independent of the calling
application.

## 3. Authority Modes

### Delegated User Identity

The HR system acts for its currently authenticated employee. Its backend
exchanges a verifiable user token for a short-lived Gantry token using OAuth 2.0
Token Exchange or a deployment-approved equivalent. Gantry records both the
calling application and subject user.

Effective authority intersects the subject user's current permissions.
The application cannot select another subject through request JSON, forward a
browser token directly to the agent, or turn a delegated call into application-
wide authority.

There is no application authority in the first contract. A client credential
authenticates the integration caller, but never becomes a Run requester or
approval identity. Unattended work is configured as an owner-bound scheduled
or Webhook trigger; its owner is the requester and its runtime Service
Principal is execution-only.

## 4. Publication to Integrations

An agent is not callable through this API merely because it appears in Gantry
Copilot. Administrators create an **integration publication** that binds:

- One immutable Agent Revision or a controlled release channel.
- One or more registered integration clients.
- Workspace and environment.
- Delegated-user subject and token-exchange requirements.
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
HR browser -> HR backend -> delegated OAuth token -> Gantry Invocation API
                                      -> validate integration publication
                                      -> validate input and business context
                                      -> create personal Session and first Run
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
- `POST /api/agent/v1/sessions`
- `GET /api/agent/v1/sessions/{session_id}`
- `GET /api/agent/v1/sessions/{session_id}/events`
- `POST /api/agent/v1/sessions/{session_id}/runs/{run_id}:cancel`
- `POST /api/agent/v1/sessions/{session_id}/runs/{run_id}:retry`
- `GET /api/agent/v1/artifacts/{artifact_id}`

Outbound Webhook Endpoint registration and delivery inspection remain Admin
Integration resources. They are not created by an enterprise client through
this invocation API. User-owned inbound automation triggers are managed through
the Copilot projection and receive external events at the separate hook endpoint
defined in [Enterprise Agent API Contracts](enterprise-agent-api-contracts.md).

Agent discovery returns only integration publications assigned to the calling
client and allowed by the verified subject's Agent metadata access. It exposes
public metadata, input/output JSON Schemas, contract version, delegated-subject
requirements, and limits, not the system prompt or internal tool bindings.

## 7. Session Invocation Contract

Example delegated-user request:

```http
POST /api/agent/v1/sessions
Authorization: Bearer <delegated-user-access-token>
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
    "events": ["run.awaiting_approval", "run.completed", "run.failed"]
  }
}
```

Response:

```http
HTTP/1.1 202 Accepted
Location: /api/agent/v1/sessions/ses_01...
```

```json
{
  "id": "ses_01...",
  "state": "active",
  "agent": {
    "id": "agt_hr_policy_advisor",
    "version": "12",
    "contract_version": "2026-08-01"
  },
  "authority": {
    "client_id": "int_hr_management_production",
    "subject_principal_id": "usr_01...",
    "token_exchange_id": "txe_01..."
  },
  "current_run": {
    "id": "run_01...",
    "requester_principal_id": "usr_01...",
    "status": "queued"
  },
  "created_at": "2026-08-13T08:00:00Z",
  "links": {
    "self": "/api/agent/v1/sessions/ses_01...",
    "events": "/api/agent/v1/sessions/ses_01.../events"
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

- The integration client may send only fields allowed by the integration
  contract.
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

Completed Runs expose a stable application-oriented projection:

```json
{
  "session_id": "ses_01...",
  "run_id": "run_01...",
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

The output is validated against the published output schema before the Run can
be marked successfully completed. The external projection excludes private
reasoning, terminal streams, internal prompts, credentials, and administrator-
only policy details.

## 10. Status and Event Projection

API clients may poll the Session or retrieve cursor-paginated events. The external
event projection includes only integration-relevant states:

- `run.accepted`
- `run.running`
- `run.awaiting_approval`
- `run.suspended`
- `run.completed`
- `run.failed`
- `run.canceled`
- `artifact.available`

Internal model deltas, PTY output, tool payloads, policy internals, and raw run
events are not exposed through the Agent Invocation API.

For the current Enterprise projection, approval rejection and expiry return a
schema-valid `action_denied` or `approval_expired` result and do not add a new
externally visible Session state. Interactive continuation after that result is a
Copilot capability. A future delegated-user continuation API must define its own
message route and event projection before Enterprise Sessions are described as
interactive.

## 11. Webhooks

Webhooks provide asynchronous status and terminal result notification.

- Outbound Endpoint registration is an administrator-approved operation and is
  not part of the enterprise invocation client surface.
- Destinations use HTTPS, pass SSRF and private-network policy, and are bound to
  an integration client and environment.
- Deliveries contain event ID, delivery ID, Session ID, Run ID, event type, timestamp,
  attempt, contract version, and a minimal payload.
- Gantry signs the timestamp and raw body with an endpoint secret or uses mTLS.
- Consumers reject old timestamps and duplicate delivery IDs.
- Delivery is at least once with exponential backoff and a finite retry window.
- Ordering is guaranteed per Session through event sequence, not across Sessions.
- A delivery log supports inspection and explicit redelivery.
- Webhook failure does not change the Run result.

Sensitive result fields may be omitted from webhooks. The consumer fetches the
authorized Run result from the API when policy requires it.

## 12. Approvals

An API-started Run may enter `awaiting_approval`.

- Delegated-user calls route any required Agent action approval only to the
  verified subject user who initiated the Run.
- The enterprise application receives status and a safe approval reference; it
  cannot approve merely by possessing the application client credential.
- Agent action approvals are decided only by the authenticated Run requester
  through the Copilot approval surface or a requester-authenticated approval
  API. The decision is bound to the exact action digest.
- Business approvals such as leave, expense, or purchase approval are initiated
  and presented by the tool's own workflow system. Gantry receives only a
  signed, idempotent external status callback and resumes the Run after
  validating the external approval reference and bound digest.
- A pending business approval suspends the same Run with a typed external wait.
  The callback resumes the next Agent loop; it never approves a Gantry action,
  replays the original Tool call, or reuses its execution permit. See
  [External Business Approval Callback Contracts](external-business-approval-contracts.md).
- Agent action rejection and expiry return a structured denial result in the
  Enterprise projection. Copilot may keep the requester conversation open, but
  that interaction is not currently part of this server-to-server contract.

## 13. Artifacts

- The application may access only artifact types exposed by the integration
  publication.
- Downloads require an authorized API request and return a short-lived object
  URL or streamed content.
- Artifact access is audited independently of Session access.
- Active content is served from a separate origin with safe disposition and
  preview rules.
- Retention and deletion follow the Session's integration publication policy.

## 14. Reliability and Idempotency

- Every Session invocation requires an `Idempotency-Key` unique within the
  integration client. The mapping is retained through the Run's terminal state
  plus the published maximum client retry interval. After Session content is
  deleted, a tombstone containing the request digest and original Session ID still
  prevents key reuse for that interval.
- Repeating the same key and semantically identical request returns the original
  Session and Run. Reusing the key with a different request returns a conflict.
- The API publishes its minimum idempotency retention interval. It never creates
  a new Session merely because the original response body has expired.
- If the same request is repeated after the original Session has been deleted but
  its idempotency tombstone remains, the API returns
  `410 idempotency_resource_expired` and the original Session ID. The caller must
  make an explicit new business decision with a new key before starting another
  Session.
- The application uses its own correlation ID for end-to-end tracing.
- Cancellation, retry, and webhook redelivery are idempotent commands.
- A non-repeatable tool action with an unknown outcome is never automatically
  replayed.
- API clients must handle `429` with `Retry-After`, transient `5xx`, token
  expiry, and Run states that outlive the caller process.

## 15. Security Requirements

- Confidential clients authenticate using private-key JWT or mTLS where
  possible; shared client secrets are a lower-assurance compatibility option.
- Client credentials never appear in browsers, mobile packages, URLs, Session
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
2. The HR backend exchanges the signed-in employee's token for a delegated
   Gantry token.
3. The backend submits typed employee and case data using an idempotency key.
4. Gantry binds the Run to the published HR Agent version and executes it.
5. If the Agent proposes a protected HR action, current policy may require a
   human approval.
6. The HR backend receives a signed terminal webhook or polls the Session.
7. It renders the schema-validated result in its own HR interface.
8. Gantry retains the complete governed audit trail; the HR system retains its
   own business record and Gantry Session/Run reference.

## 17. SDK and Contract Distribution

The protocol remains usable with ordinary HTTP clients. Gantry should generate
and publish server-side SDKs only after the OpenAPI contract stabilizes, starting
with languages used by target enterprise systems. SDKs provide authentication,
idempotency, polling, webhook verification, error types, and schema models; they
must not hide Run states or retry non-repeatable actions automatically.
