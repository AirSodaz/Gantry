# ADR-036: Human Requesters and Owner-Bound Automation

**Status:** Accepted; supersedes ADR-018 application authority and is amended by
ADR-037 Session semantics and ADR-038 scheduled Trigger timing.

## Context

Gantry Agent-action approval belongs only to the authenticated Run requester.
A confidential integration client or runtime Service Principal has no human
identity and cannot responsibly receive or decide that approval. Allowing a
machine identity to create an otherwise ordinary approval-bearing Run would
produce orphaned decisions or require a second approver model.

Enterprise systems still need delegated employee invocation, while unattended
work needs scheduled and event-triggered entry points.

## Decision

1. Direct Enterprise Agent API invocation requires a verified delegated user.
   The integration client authenticates the backend call, but the verified
   subject becomes Session owner and first Run requester, and is the only
   eligible Agent-action approver for that Run.
2. Gantry does not support `application` Run authority. Request JSON cannot
   assert a user, and a client credential never becomes requester.
3. Unattended work uses a human-owner-bound Webhook or scheduled Trigger. Each
   occurrence creates an ordinary Run in a new or pre-bound Session with source
   tag `webhook` or `schedule`.
4. A trigger's fixed Service Principal is execution-only. It cannot approve an
   action, broaden owner authority, or replace the owner in Session/Run/Audit evidence.
5. Trigger creation and each occurrence recheck the owner's Agent `execute`,
   Workspace, Deployment, Policy, quota, and lifecycle state. Losing authority
   blocks new occurrences without rewriting historical Sessions or Runs.
6. Unattended means no user must start the Run. It does not mean automatic
   approval: Policy-allowed actions may proceed, while approval-required actions
   wait for the human owner in Copilot.
7. Trigger ownership is not inferred or reassigned automatically. Disabled or
   departed owners disable new occurrences until an explicit supported transfer
   or replacement operation is completed.

## Consequences

- Every effect-bearing Run has one accountable human requester.
- Enterprise clients remain useful for delegated calls, delivery, quotas, and
  contract projection without becoming an authorization principal for Runs.
- Machine-only direct invocation is intentionally unavailable; organizations
  configure an owner-bound trigger instead.
- Webhook and scheduled Runs reuse normal Session, Run, approval, Artifact,
  Audit, cancellation, and retention contracts.
- Scheduled Triggers use five-field cron, an explicit IANA time zone, one Run
  for the first instant of a repeated local time, and `skip` misfire semantics.
  These timing rules cannot change requester or approval ownership.

## Rejected Alternatives

- **Application identity as requester:** rejected because no human can receive
  requester-bound action approval and a client credential is not accountability.
- **Admin or group approval fallback:** rejected because it would reintroduce a
  generic approval role and break the requester-only model.
- **Service Principal as owner:** rejected because it is runtime identity, not a
  human approval subject.
- **A separate automation Run type:** rejected because trigger source does not
  change Session, Run, approval, or evidence semantics.
