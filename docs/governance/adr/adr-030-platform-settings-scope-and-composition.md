# ADR-030: Platform Settings Scope and Composition

## Status

Accepted.

## Decision

Admin exposes one `/platform/settings` route with an explicit
`Organization | Workspace` scope switcher. Organization Administrators define
organization defaults and non-negotiable bounds. Workspace values inherit or
narrow within those bounds; they cannot broaden organization authority or
capacity. A separate Workspace Settings page is not introduced.

The page is a composed projection over typed resources:

- Organization Profile
- Retention Policies and Legal Holds
- Data Classification Definitions
- Limit Policies
- Environment Profiles

The projection includes effective value, source, organization bound, owning
resource, validation state, ETag, and last-change metadata. It is not a mutable
catch-all `PlatformSettings` entity and does not replace the owning resource's
authorization or lifecycle.

Settings mutations are section-scoped commands. They support side-effect-free
validation, semantic diffs, expected-ETag conflict detection, explicit
confirmation, idempotency, correlation IDs, and canonical Audit events. Recent
activity links to the global Audit explorer rather than creating a second audit
store. Retention deletion remains an asynchronous, Legal Hold-aware workflow.

## Consequences

Organization policy remains the visible outer boundary while Workspace teams
retain bounded operational control. The UI stays understandable without an
Allow/Deny rule builder, while the server continues to enforce explicit
capabilities and action-time policy.

The Settings endpoint is a read projection and command facade; persistence and
authorization remain with typed platform resources. Provider budgets, runner
capacity, Integration quotas, credential secrets, and Policy Bindings continue
to be managed on their owning pages.
