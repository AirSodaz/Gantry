# ADR-033: Agent ACL Grants and Explicit Owner Transfer

**Status:** Accepted

## Context

Agent configuration, execution, review, deployment, and ACL administration
serve different audiences. A single role flag or replace-all ACL document
would make partial revocation, group membership changes, concurrency, and
recovery difficult to audit.

## Decision

Agent access is represented by explicit `AgentAccessGrant` resources. Each
grant targets exactly one `principal`, `group`, or registered
`service_identity` and contains an explicit Allow-only capability set. The
initial capabilities are `metadata.read`, `configuration.read`, `draft.edit`,
`review.decide`, `deployment.test`, `deployment.production`, `runs.read`,
`execute`, and `access.manage`. Capabilities have no implicit inheritance.

Agent ownership is stored separately as one `owner_principal_id`. Ownership is
not a substitute for an ACL grant, except that the current owner may invoke
the narrow owner-transfer command. `POST /agents/{agent_id}:transfer-owner`
atomically updates ownership and ensures the target principal has an explicit
`access.manage` grant. Existing grants for the old owner are not silently
revoked.

Every ACL collection has a monotonic ETag. Grant mutations and owner transfer
use idempotency keys, compare-and-swap ETags, a recovery-path invariant, and a
single transaction containing the state change and canonical Audit event.
The last active recovery path cannot be removed without owner transfer or the
separately authorized break-glass procedure.

## Consequences

- Group membership is resolved from the current identity projection at action
  time; membership is not copied into the ACL.
- Service identities can execute only through the same Agent, Deployment,
  Publication, and Policy intersection and can never become owners or satisfy
  the recovery invariant.
- Presets materialize explicit capabilities and are not evaluated at runtime;
  later preset edits do not rewrite grants.
- Copilot receives only effective Agent availability and its own authorized
  projection. Admin receives full ACL evidence within scope, while Enterprise
  Invocation intersects the grant with its Publication.
- Revocation or outer-policy changes block new work without deleting historical
  Runs or Audit evidence.
