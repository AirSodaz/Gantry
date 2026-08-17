# ADR-022: Separate Agent Action Approval from Business Workflow Approval

## Status

Accepted.

## Decision

Gantry owns action-time authorization for effect-bearing Agent operations. When
human confirmation is required, only the authenticated Run requester may
approve or reject the exact action digest. A published Policy may instead allow
or deny the action without human confirmation.

Business approvals such as leave, expense, or purchase requests remain owned and
presented by the tool or enterprise system that defines that workflow. Gantry
integrates them through a signed, idempotent external status callback bound to
the same action digest; it does not provide a universal Admin approver inbox.

The business wait can begin only after the requester's Gantry action approval
and Tool invocation. A pending Tool result suspends the same Run with reason
`external_business_approval`. A valid callback records the external outcome and
resumes the next Agent loop; it never approves a Gantry action, replays the
original Tool invocation, or reuses its execution permit.

Rejecting an Agent action or allowing its approval to expire returns a
structured denial or expiry result and leaves the Session conversation available
for requester input. A revised consequential action receives a new digest and
decision.

## Consequences

Agent action approval and business workflow approval have distinct owners,
identities, and audit evidence. Copilot can decide the requester's concrete
Agent action; Admin Runs and Audit expose read-only evidence; the external tool
remains responsible for its business process.
