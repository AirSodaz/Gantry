# External Business Approval Callback Contracts

## 1. Scope and Status

This document defines the target contract for a business Tool that, after an
approved Gantry action is invoked, transfers the request into an external
business approval workflow. It covers the pending Tool result, durable wait,
signed callback, Run suspension/resume, authorization, idempotency, and Audit
evidence.

Gantry does not decide leave, expense, purchase, access, or other business
workflow approvals. The owning Tool or enterprise system presents and decides
that workflow. Gantry owns only the earlier requester approval of the exact
Agent action digest.

This contract is not implemented until its Tool result schema, callback route,
persistence, outbox worker, runner resume protocol, and focused race and
authorization tests exist.

## 2. Required Sequence

1. The Agent may call read-only Tools to collect information and produce a
   concrete plan.
2. An effect-bearing Tool call becomes a durable Gantry action and is evaluated
   by current Policy.
3. When requester confirmation is required, only the Run requester may approve
   the exact action digest.
4. The gateway consumes one execution permit and invokes the business Tool.
5. The Tool either returns a terminal result or a typed
   `external_approval_pending` result with an opaque external reference.
6. A pending result suspends the same Run with
   `suspension_reason=external_business_approval`.
7. The external system sends a signed, idempotent callback. Gantry records the
   result and resumes the same Run for the next Agent loop.
8. The Agent may complete the Run or propose a new action. Any new effect has a
   new digest and a new Gantry approval decision when required.

The callback never approves a Gantry action, replays the original Tool call, or
creates an execution permit.

## 3. Typed Resources

```yaml
ExternalApprovalPendingResult:
  type: object
  required: [type, external_system, external_reference, callback_profile_id,
             expected_expires_at]
  properties:
    type: {type: string, enum: [external_approval_pending]}
    external_system: {type: string}
    external_reference: {type: string}
    callback_profile_id: {type: string}
    expected_expires_at: {type: string, format: date-time}
    requester_message: {type: string, maxLength: 500, nullable: true}

ExternalBusinessApprovalState:
  type: string
  enum: [pending, approved, rejected, expired, canceled]

ExternalBusinessApprovalWait:
  type: object
  required: [id, session_id, run_id, action_id, action_digest, external_system,
             external_reference, state, revision, created_at, updated_at]
  properties:
    id: {type: string}
    session_id: {type: string}
    run_id: {type: string}
    action_id: {type: string}
    action_digest: {type: string, pattern: '^sha256:'}
    external_system: {type: string}
    external_reference: {type: string}
    state: {$ref: '#/components/schemas/ExternalBusinessApprovalState'}
    revision: {type: integer, format: int64, minimum: 1}
    expected_expires_at: {type: string, format: date-time, nullable: true}
    resolved_at: {type: string, format: date-time, nullable: true}
    result_summary: {type: object, nullable: true}
    created_at: {type: string, format: date-time}
    updated_at: {type: string, format: date-time}

ExternalBusinessApprovalCallback:
  type: object
  required: [event_id, external_reference, action_digest, decision,
             occurred_at]
  properties:
    event_id: {type: string}
    external_reference: {type: string}
    action_digest: {type: string, pattern: '^sha256:'}
    decision: {type: string, enum: [approved, rejected, expired]}
    effect_state:
      type: string
      enum: [completed, no_effect, additional_work_required, unknown]
    result: {type: object, nullable: true}
    occurred_at: {type: string, format: date-time}
```

`external_reference` is opaque and not an authorization credential. The
callback binding is the intersection of callback profile, external system,
wait ID, action digest, and current wait revision.

## 4. Tool Gateway Boundary

Only a Tool Descriptor explicitly declaring external business approval support
may return `ExternalApprovalPendingResult`. The descriptor pins:

- callback profile and accepted authentication modes;
- external system and environment;
- approval-reference format constraints without embedding secrets;
- maximum wait duration and callback payload schema;
- result classification, retention, and redaction rules.

The gateway rejects a pending result when the action was not requester-approved
or Policy-allowed, the callback profile is not bound to the Tool, the reference
is empty or reused for another action, or the wait exceeds platform bounds.
Persisting the wait, moving the action to `awaiting_external_approval`, moving
the Run to `suspended`, appending events/Audit, and inserting expiry/resume
outbox work is one transaction.

The Runner receives only the normalized pending result and suspension outcome.
It never receives callback credentials or a capability to poll arbitrary
external approval records.

## 5. Callback Route

The callback is a Tool-integration route, not a Copilot approval route and not
the Enterprise Session invocation API:

```text
POST /api/tool-callbacks/v1/external-approvals/{wait_id}
X-Gantry-Callback-Timestamp: <unix-seconds>
X-Gantry-Callback-Event-Id: <stable-event-id>
X-Gantry-Callback-Signature: sha256=<signature>
Content-Type: application/json
```

The callback profile may require mTLS, asymmetric request signing, or HMAC for
lower-assurance compatibility. The signature covers the timestamp, event ID,
route, and exact request bytes. Credentials are resolved by the trusted callback
adapter and never appear in the wait, Session, Run, event, or browser projection.

Processing performs these checks before mutation:

1. Authenticate the callback profile and external system.
2. Reject stale timestamps, oversized bodies, and invalid signatures.
3. Load the wait by opaque ID and require matching external reference and
   action digest.
4. Claim `(callback_profile_id, event_id)` idempotency and compare the request
   digest on replay.
5. Require the current wait state `pending`; a duplicate returns the winning
   result and a conflicting terminal decision returns `409 wait_changed`.
6. Persist the terminal external result and append Audit/events.
7. If the Session and Run are still resumable, enqueue same-Run resume through the outbox;
   otherwise retain the callback as late evidence without resuming execution.

The success response is `202 Accepted` for a new terminal callback and `200 OK`
for an identical replay. Unknown, cross-environment, or unauthorized wait IDs
return non-leaking `404 resource_not_found`.

## 6. Resume and Agent Loop

The callback does not directly wake a sandbox. The resume worker validates the
winning wait revision, Session and Run state, current lease fencing, Deployment,
requester identity, and current Policy before transitioning the suspended Run
to queued/resumable state. Durable suspension may provision a new sandbox, but
the Run identity and historical action remain unchanged.

The resumed Agent receives a structured Tool result containing the external
decision, effect state, bounded result projection, external system, and opaque
reference. It does not receive callback credentials or raw business-approval
records.

- `approved + completed` may allow the Agent to produce the final result.
- `approved + additional_work_required` allows another planning step; any new
  effect requires a new action digest and current authorization.
- `rejected`, `expired`, or `no_effect` lets the Agent finish, revise the plan,
  or request user direction without replaying the original call.
- `unknown` cannot be represented as success; reconciliation or a new explicit
  action is required.

## 7. Expiry, Cancellation, and Recovery

An expiry worker resolves elapsed pending waits to `expired` using the same
compare-and-swap transition as callbacks. Cancellation marks a still-pending
wait `canceled`, invalidates resume work, and sends a best-effort external
cancellation only when the Tool contract supports an idempotent cancellation
operation. It never claims that an already completed business effect was
reversed.

Callback, expiry, Run cancellation, Tool revocation, and Policy revocation may
race. Exactly one terminal wait transition wins. Late valid callbacks are
retained as reconciliation evidence and may update external-observation fields,
but do not rewrite the original winning state or automatically resume the Run.

Server restart reconstructs pending waits from PostgreSQL and durable outbox
records. No in-memory timer or open runner connection is required for expiry or
resume correctness.

## 8. Copilot and Audit Projection

Copilot renders the current Run as `suspended` inside its Session with a safe reason such as "Waiting for
approval in the purchasing system". It has no Gantry approve/reject controls
for that wait. The Run requester or Session owner may cancel the Run; external business decisions
remain in the owning system.

Audit evidence includes the Gantry action approval, execution-permit claim,
pending Tool result, external reference digest, callback authentication
profile, event ID, terminal external decision, resume outcome, and correlation
IDs. It excludes callback secrets, raw signing material, and business payload
fields outside the approved result projection.

## 9. Acceptance Contract

- A Tool cannot create an external wait before the exact Gantry action is ready
  and its single-use execution permit has been consumed.
- Pending wait, action state, Run suspension, events, Audit, and outbox intent
  commit atomically.
- Callback tests cover signature and mTLS failures, stale timestamp, replay,
  digest/reference substitution, conflicting decisions, expiry, cancellation,
  Policy/Tool revocation, and restart recovery.
- A terminal callback resumes at most one same-Run Agent loop and never replays
  the original Tool call.
- Every new effect proposed after resume receives a new action digest and
  current requester/Policy decision.
- Copilot exposes an external wait as a suspended Run reason, never as a
  Gantry Approval Request or Admin business-approval queue.
