# ADR-032: Inbound Webhook Tasks Reuse the Normal Task Contract

**Status:** Superseded by ADR-037

**Date:** 2026-08-17

## Context

An external event may need to start an Agent Task, but introducing a second
automation-specific Task, permission, or approval model would split the Copilot
workflow and make authorization harder to reason about.

## Decision

An inbound Webhook Trigger is an owner-scoped, non-shareable Task creation
endpoint. The trigger owner must have Agent `execute` permission when creating
the trigger, and the server rechecks the Agent ACL, Deployment, Policy,
quarantine, Workspace, and quota for every event.

Each accepted event creates an ordinary Task with one optional `webhook` source
tag. The trigger owner is recorded as `requester_id` and is the requester who
may approve or reject Agent actions through the normal Copilot approval flow.
The trigger uses a fixed Service Principal for runtime execution, but that
execution identity does not replace the requester or approve actions. The
webhook request itself can never approve an action.

HMAC or mTLS authentication, replay protection, event-id idempotency, schema
validation, quotas, and audit evidence remain mandatory transport and abuse
controls. They do not form a separate Automation ACL. Shared triggers,
`automation.invoke`, and schedule cadence/misfire semantics were outside this
decision. ADR-036 later established the same human-owner authority model for
scheduled triggers, and ADR-038 defines their time semantics.

## Consequences

Webhook-created Tasks appear in the owner's ordinary task and approval views,
and existing Task, Run, event, Artifact, continuation, expiry, and audit
contracts apply unchanged. Duplicate event delivery maps to the original Task
within the idempotency window. Revoking the owner's Agent execution access
blocks new events while retaining historical Tasks and their requester
identity.

ADR-037 preserves the human-owner, Service Principal, transport, and
idempotency boundaries while replacing each Task reference with either a new or
bound Session plus an ordinary Run.
