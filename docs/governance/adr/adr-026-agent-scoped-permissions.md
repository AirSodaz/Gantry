# ADR-026: Agent-Scoped Independent Permissions

## Status

Accepted.

## Decision

Every Agent has an explicit ACL whose capabilities independently control:

- safe metadata discovery;
- configuration read, including Prompt Snapshots, Skills, Plugins, Tools, and
  policies subject to redaction;
- Draft editing and Revision commits;
- Review decisions;
- test Deployment management;
- Production publication and rollback;
- run and artifact inspection;
- Copilot or integration execution;
- Agent ACL management.

`execute` does not imply configuration read, and configuration read does not
imply execution. A service identity may invoke an Agent without receiving
internal Prompt or Tool Binding content. A reviewer may inspect a bound Revision
and decide its Review without editing the Draft.

Drafts inherit the owning Agent's `configuration.read` and `draft.edit`
capabilities. They do not maintain a separate private-sharing ACL.

Effective access is the intersection of organization and workspace
authorization, Agent ACL, resource state, active Deployment or integration
publication, and action-time policy. ACL grants cannot broaden Tool Binding,
credential, network, approval, or data-classification authority.

The initial ACL model is Allow-only. No explicit Deny grant is stored or
evaluated. Absence or revocation of an Allow is denial. If a granted capability
is blocked by an outer constraint, the system records and displays that
constraint rather than creating a competing Deny rule. Emergency execution
stops use Agent or Deployment quarantine.

## Reason

Agent configuration and Agent execution have different audiences. Per-Agent
control is required for confidential prompts, delegated editors, security
reviewers, operators, and service identities that should execute without
configuration access. Independent capabilities make those boundaries explicit
and auditable.

## Consequences

- Access is default-deny when no applicable Agent capability is granted.
- Removing execution prevents new tasks while preserving historical run and
  audit evidence.
- Removing configuration read redacts or denies raw configuration, even when
  safe metadata or run status remains visible.
- Preset roles may simplify administration, but the Agent ACL remains the source
  of truth.
- A preset materializes a visible explicit capability set. Editing that set
  produces a Custom assignment, and changing a preset definition never mutates
  existing grants.
- Every grant, revocation, denied read, denied execution, and emergency
  revocation is audited.
- The first release avoids explicit Deny precedence and conflict-resolution
  rules; a future exception model would require a separate decision.
