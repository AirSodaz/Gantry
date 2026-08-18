# ADR-037: Agent Sessions and Run Requesters

**Status:** Accepted; supersedes Task-as-conversation semantics in ADR-032,
amends ADR-022, ADR-035, and ADR-036, and is complemented by ADR-038 scheduled
Trigger timing.

**Date:** 2026-08-17

## Context

The original Task resource combined a durable conversation, one immutable
requester, workflow status, multiple Run attempts, approvals, and historical
visibility. That model supported one employee working with one Agent, but it did
not cleanly support shared finance work, personal use of the same company Agent,
or a long-lived Agent bound to a work-group channel.

Introducing both Agent Templates and per-user Agent Instances would add another
ACL, lifecycle, version-following rule, and migration boundary even though the
current Agent already owns Drafts, immutable Revisions, Deployments, and access.

## Decision

1. The existing Agent remains the company-governed capability definition.
   Gantry does not add a separate Agent Template or Agent Instance in the initial
   model. Independent conversation history and collaboration live in Sessions.
2. Session replaces Task as the durable Copilot conversation and collaboration
   aggregate. A Session binds one Agent and is `personal`, `shared`, or
   `channel`; it remains active until its human owner archives it.
3. Session collaboration uses fixed `owner`, `contributor`, and `viewer` roles,
   not Allow/Deny rules or a second capability ACL. A principal sees only
   Sessions where it is a current member. A `workspace_agent_editor` may
   explicitly add itself as a `viewer` to a Session whose Agent is in its
   managed Workspace; this is an audited self-enrollment exception, not an
   implicit Admin bypass. Membership does not grant Agent configuration access,
   Agent `execute` authority, approval authority, or the ability to invite
   other members.
4. Every accepted human or Trigger instruction appends one immutable Session
   Message and creates one Run. The authenticated instruction author, or human
   Trigger owner, is the immutable Run requester.
5. Only the Run requester may decide that Run's Agent-action approvals. Session
   ownership, membership, Admin roles, and business-approver roles cannot decide
   them.
6. The Run requester or current Session owner may cancel a Run. This restrictive
   authority does not grant the Session owner approval or execution authority.
7. A Session has at most one executing Run. Additional accepted instructions
   create queued Runs ordered by Session sequence. Approval and external
   business-approval waits retain the execution slot; queued work cannot
   overtake them.
8. A Webhook or scheduled Trigger chooses `new_session` or `bound_session` when
   configured. A bound Trigger requires its human owner to own an active Session
   using the same Agent. Invocation payloads cannot select another Session.
9. Trigger occurrences are strongly idempotent before queueing. Webhooks key on
   `(trigger_id, event_id)` and schedules use a stable occurrence ID. A replay
   with the same digest returns the original Session/Run receipt; a digest
   mismatch conflicts. HTTP retries, outbox retries, scheduler retries, and
   worker restarts cannot enqueue the same occurrence twice.

## Consequences

- Personal weekly-report use needs only a personal Session for the shared
  company Agent; it does not create a private Agent clone.
- Finance and other team workflows use a shared Session whose members can read
  or contribute according to fixed roles and current Agent authorization.
- A work-group adapter binds the external channel or thread to a shared Session;
  verified message authors remain individually attributable as Run requesters.
- Runs, rather than Sessions, carry execution, approval, retry, cancellation,
  suspension, and terminal outcome state.
- Historical Session reads depend on current membership and data policy. New
  instructions additionally require current Agent `execute` authority.
- Copilot Web defaults to owner-owned Sessions and offers an explicit
  accessible scope for the caller's current owner, contributor, and viewer
  memberships. The server applies both scopes before pagination.
- Workspace Agent Editor self-enrollment is limited to the caller becoming a
  viewer on a Session for an Agent in the editor's managed Workspace. It is
  auditable, cannot target another principal, and grants no write or Agent
  capability.
- A future Agent Instance resource remains possible only when independent
  long-term memory, credentials, or configuration must persist across multiple
  Sessions.

## Rejected Alternatives

- **Clone an Agent from a Template for every user:** rejected because ordinary
  personal history does not justify separate versioning, deployment, and ACL
  lifecycles.
- **Keep Task and add Session above it:** rejected because deciding whether each
  chat message continues or creates a Task adds an ambiguous extra hierarchy.
- **Let any Session member approve:** rejected because it breaks the accountable
  human requester boundary for consequential effects.
- **Allow invocation payloads to choose Session IDs:** rejected because a Hook
  could inject work into a Session outside its reviewed binding.
- **Deduplicate only in queue workers:** rejected because concurrent HTTP and
  outbox retries could still create duplicate Messages or Runs.
