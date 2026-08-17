# ADR-035: Copilot Published Metadata and Workspace-Scoped Preferences

**Status:** Accepted

## Context

Copilot needs enough information for an employee to choose an Agent safely:
typical inputs, expected output, capability summary, data classification,
action disclosure, and current availability. These fields must not expose the
Admin Draft, prompt, model route, Tool bindings, Policy rules, or credentials.
Copilot also needs favorites and recently used Agents without turning a user
interface preference into authorization.

The Agent lifecycle already separates mutable Drafts, immutable hash-addressed
Revisions, and test/Production Deployment pointers. Agent catalog access is
Workspace-scoped, while the authenticated principal owns the personal Session.

## Decision

1. Stable Agent identity fields (`display_name`, `description`, `category`, and
   owner/support contact) remain on the Agent identity.
2. Typical inputs, expected output, capability summary, data disclosure, and
   action disclosure are typed Draft metadata. A committed Revision freezes the
   canonical metadata and its digest. Later Draft edits cannot change a
   published projection.
3. Deployment owns only publication-specific availability: `available` or
   `temporarily_unavailable`, with a bounded reason and optional expiry. It
   cannot widen the Revision's disclosures or classifications. Session creation
   rechecks availability.
4. Preferences use one requester-owned record keyed by
   `(principal_id, workspace_id, agent_id)`. A favorite is set idempotently by
   `PUT /api/copilot/v1/agents/{agent_id}/favorite`; catalog discovery and this
   mutation require `metadata.read`, while Session creation separately requires
   `execute`. A preference grants no ACL capability and cannot override
   Deployment availability.
5. `last_used_at` is written only after successful `POST /sessions` commit. The
   recent collection returns at most the eight most recently submitted Agents
   in that Workspace. Non-favorite rows outside the window may be pruned;
   favorite rows are retained. Failed, rejected, validation-only, and replayed
   submissions do not create another recent-use event.
6. The Copilot projection joins the authorized active Deployment, exact
   Revision metadata, stable Agent identity, and only the caller's preference.
   `collection=favorites|recent` is an authorization-constrained catalog
   filter, not an alternate access path.

## Consequences

- Publishing a Revision is sufficient to make its employee-facing disclosures
  immutable and auditable.
- A temporary outage can be communicated without mutating historical Revisions
  or user preferences.
- The schema avoids a separate preference versioning system and keeps recent
  use deterministic and small.
- Implementations must add the typed OpenAPI fields, a preference table and
  transaction/outbox update, Admin Draft validation, and focused projection and
  authorization tests before claiming completion.

## Rejected alternatives

- **Preferences as ACL grants:** rejected because a favorite must never grant
  metadata, execution, configuration, or access-management authority.
- **Global preferences per principal:** rejected because the same principal
  may work in multiple Workspace catalogs with different availability.
- **Recent use on page open:** rejected because it is noisy and does not
  represent a created Session.
- **Deployment-specific copies of all metadata:** rejected because it would
  duplicate Revision authority and allow publication drift.
