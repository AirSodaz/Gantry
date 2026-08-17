# Product Design

## 1. Product Definition

Gantry is a self-hosted enterprise agent platform. It gives platform teams a
controlled way to design and publish agents while giving employees a simple
Copilot experience for using only the agents, tools, data, and actions they are
authorized to access.

The product is intentionally split into two applications.

### Gantry Admin

Gantry Admin is an operations-focused console for platform administrators,
agent designers, security reviewers, auditors, and operators. It owns the agent
lifecycle from draft through publication, monitoring, evaluation, and
retirement.

### Gantry Copilot

Gantry Copilot is an employee-facing application. It provides a catalog of
approved agents, conversational and task-oriented execution, action approvals
from the current employee's tasks, run history, and access to generated artifacts. It does
not expose prompts, credentials, infrastructure controls, raw policy internals,
or unrestricted terminal access.

Both applications use the same identity provider and control-plane APIs, but
they have separate frontend builds, navigation models, authorization audiences,
and recommended deployment origins.

Enterprise systems may also invoke published agents through the server-to-
server Agent Invocation API. For example, an HR management system can call an
approved HR agent and present its structured result in the HR system's own UI.
This is a third consumption channel over the same governed Session/Run model, not a
third Gantry user interface.

## 2. Product Goals

- Make enterprise agents declarative, versioned, reviewable, and reproducible.
- Allow employees to use agents without learning infrastructure or model
  configuration.
- Allow registered enterprise systems to invoke explicitly published agents
  through stable, schema-driven APIs.
- Enforce user, agent, tool, command, data, and destination permissions at the
  point of action.
- Support long-running tasks, asynchronous approvals, cancellation, retry, and
  durable recovery.
- Preserve a complete, tamper-evident execution record suitable for operations,
  incident review, and compliance.
- Convert successful production trajectories into sanitized regression cases.
- Evaluate side-effecting agents without changing production systems.

## 3. Non-Goals for the First Usable Release

- General-purpose multi-agent graph orchestration.
- Hosting arbitrary third-party agent frameworks as opaque, privileged
  processes.
- A public multi-tenant SaaS control plane.
- SCIM provisioning, custom policy languages, or a full marketplace.
- Generic interception of every RPC protocol.
- Exact restoration of arbitrary process memory after durable suspension.
- Display or persistence of raw model chain-of-thought.
- Unrestricted remote shell access for employees.

## 4. Actors and Responsibilities

| Actor | Primary application | Responsibilities |
| --- | --- | --- |
| Organization Administrator | Admin | Platform settings, providers, runner pools, retention, emergency controls |
| Workspace Agent Editor | Admin | Draft specs, prompts, tools, model policies, tests, and release notes within assigned Workspaces |
| Security Reviewer | Admin | Review permissions, egress, credentials, commands, publication changes, and export scoped audit evidence |
| Operator | Admin | Monitor runs, inspect failures, cancel or retry runs, manage capacity, and inspect audit evidence without export |
| Auditor | Admin | Search immutable events, inspect approvals, and export scoped evidence |
| Employee | Copilot | Discover approved Agents, use personal or shared Sessions, submit instructions, review progress, and receive Artifacts |
| Integration Client | Agent Invocation API | Authenticate a backend call made only on behalf of a verified delegated user; never become a Run requester |

One person may hold several roles. Authorization is evaluated from the union of
assigned roles, constrained by organization and workspace boundaries.

## 5. Organization Model

The first deployment model is one enterprise organization per installation.
Each installation contains logical workspaces for departments, projects, or
data boundaries. A workspace owns agents, policies, evaluation suites, and run
visibility.

This model avoids claiming SaaS-grade tenant isolation while preserving stable
tenant identifiers in schemas and APIs. A future hosted edition may map each
organization to a stronger isolation boundary without changing core resource
identifiers.

### Platform Settings Boundary

Admin exposes one `/platform/settings` route with an explicit
`Organization | Workspace` scope. Organization Administrators define defaults
and non-negotiable bounds; Workspace values inherit or narrow within those
bounds. Settings composes typed Organization, Retention, Legal Hold,
Classification, Limit, and Environment resources without replacing their
ownership or creating a second Audit, Policy, Integration, Provider, or Runner
configuration surface. Copilot never exposes Platform Settings.

## 6. Core Product Concepts

### Agent

A named enterprise capability visible to employees, such as "Procurement
Analyst" or "Incident Triage". An Agent has stable descriptive metadata, named
Drafts, immutable Revisions, and environment Deployments.

The initial product does not split this resource into Agent Template and Agent
Instance. Personal continuity and team collaboration are Session concerns. A
future instance resource is justified only by independent long-term memory,
credentials, or configuration that must span multiple Sessions.

### Agent Draft

A named mutable editing space used for design and debugging. Each Draft has one
optimistically locked working copy, an optional source Revision, and a latest
Revision reference. An Agent has a Main Draft and may create multiple additional
Drafts from the content of any Revision. Drafts and Revisions do not form a
branch tree.

### Agent Revision

An immutable configuration snapshot identified by a cryptographic hash,
required message, author, time, and a separate canonical content digest. It
contains the Agent Prompt Snapshot and exact Skill artifact, Plugin, Tool
Descriptor, Tool Binding, model-policy, command-policy, input/output-contract,
runtime, artifact, approval, and evaluation references.

### Agent Deployment

A named environment pointer to one exact Agent Revision. Multiple test
Deployments may coexist; one workspace-owned Agent has one default Production
Deployment. Publication and rollback move deployment pointers without changing
Revision history.

### Agent Access

Every Agent has an explicit ACL whose capabilities independently control
metadata discovery, configuration read, Draft editing, review decisions, test
Deployment management, Production publication, run inspection, execution, and
ACL management. Configuration read never implies execution, and execution never
implies access to Prompts or internal Tools. Effective access remains the
intersection of organization/workspace authorization, Agent ACL, resource state,
Deployment, integration publication, and action-time policy.

### Agent Prompt

Agent-owned system instructions edited inside a Draft. Committing a Revision
freezes the text, variables, ordering, provenance, compiler version, and digest
as an immutable Prompt Snapshot. Reusable guidance belongs in Skills rather than
a separate Prompt catalog.

### Skill

A reusable system-prompt package imported from an external skills source or
added as a complete package/local directory. The source package declares its
own version; Gantry only displays that value and records the source reference
and content digest. If the package does not declare a version, the UI shows
`未声明` and Gantry does not invent one. Multiple imported artifacts of the
same Skill may coexist so test Agents can compare package versions. Imported
content is not edited inline in Gantry. A Skill helps an Agent use governed
capabilities consistently but cannot grant tool authority by itself.

### Plugin

An installable, versioned container for one or more business Skills plus
optional MCP servers, Tool Descriptors, configuration schemas, binding
templates, and presentation metadata. Plugins are installed at organization
scope, enabled per workspace, and selectively bound by an Agent. They never
grant all contained tools automatically.

### Tool Server and Tool Descriptor

A Tool Server registers a built-in, MCP, or governed CLI provider. Immutable
Tool Descriptor Versions define names, schemas, effects, idempotency, data
classification, credential capabilities, destinations, and execution limits.

### Session and Run

A Session is a durable personal, shared, or channel-bound conversation around
one Agent. It has one human owner and fixed owner, contributor, and viewer member
roles. A Run executes one accepted instruction against a specific Agent Revision
and Policy snapshot. The instruction author is the immutable Run requester;
retrying creates a new Run rather than rewriting history. Copilot presents
Sessions, conversation, requester approvals, and ordered Run history. Admin
presents the same immutable Run records as a cross-actor operational and
diagnostic workbench, subject to role, scope, and redaction controls.

### Tool Binding

An Agent-owned, versioned reference to one immutable Tool Descriptor. It narrows
allowed operations, inputs, credentials, destinations, approval policy, and
runtime limits. It never broadens the registered descriptor.

### Policy

An organization- or Workspace-owned typed rule resource with one mutable Draft,
immutable Versions, and explicit Bindings. Organization and Workspace Policies
intersect, so a lower scope can only narrow outer authority. Agent Revisions
pin exact Policy Versions; a Draft or movable latest Version never controls a
run. Approval Policies configure Agent action decisions but do not own pending
approvals or business workflow approvals.

### Approval

A durable requester decision for one proposed Agent action. It includes an
exact action preview, risk explanation, policy reason, expiry, authenticated
Run requester, and the resulting decision. Business workflow approvals remain
owned by the tool or enterprise system that defines them.

### Golden Case

A sanitized, immutable evaluation fixture created from an authored scenario or
an eligible production run. It contains inputs, replay fixtures, assertions,
expected policy decisions, and provenance.

## 7. Gantry Admin Capabilities

### Agent Design

- Create agents from templates or a blank draft.
- Edit instructions with variables and structured input/output schemas.
- Edit the Agent Prompt, select imported Skill artifacts and Plugins, and inspect their
  effective compiled instruction order.
- Select a model policy rather than embedding a provider-specific model ID.
- Register or select built-in, MCP, and governed CLI descriptors; bind
  namespaced tools and constrain operations and argument patterns.
- Configure command allowlists, denied patterns, filesystem mounts, egress
  destinations, resource limits, TTL, and approval rules.
- Validate a draft statically before any execution.
- Create multiple named Drafts from any Revision without changing Production.
- Commit immutable Revisions with a hash and message before execution.
- Manage Agent-specific access for configuration readers, editors, reviewers,
  publishers, operators, and executors.
- Run Revisions only through designated test Deployments and credentials.
- Compare Revisions and require review for security-relevant changes.

The initial designer is form-led. React Flow is used where a graph clarifies
the execution policy or later orchestration, not as a mandatory canvas for
simple agents.

### Publication

- Submit an exact Revision for review with release notes.
- Show a semantic diff of prompts, tools, permissions, policies, and schemas.
- Require designated reviewers based on risk classification.
- Move the default Production Deployment to an approved Revision.
- Roll back Production to a prior approved Revision without deleting history.
- Deprecate, quarantine, and retire Revisions while preserving historical runs.

### Operations

- Monitor queue depth, active sandboxes, runner capacity, error rate, latency,
  token usage, cost estimates, and approval wait time.
- Filter runs by workspace, agent, version, user, status, provider, and time.
- Inspect structured timelines, terminal output, tool calls, artifacts, policy
  decisions, and resource usage.
- Cancel active runs, retry eligible failures, quarantine versions, and disable
  providers or tools through an emergency control.

### Evaluation

- Author golden cases and export eligible production trajectories after
  redaction.
- Run suites against an exact test or Production Revision.
- Inspect deterministic assertions, rubric scores, policy violations, state
  deltas, latency, tokens, and estimated cost.
- Compare candidate and baseline Revisions before publication.

### Governance

- Browse all Policy types in one scope-aware catalog.
- Edit the single Draft for a Policy and publish immutable exact Versions.
- Bind a Version explicitly and preview affected Agents and environments.
- Explain how organization and Workspace Policies intersect without allowing a
  lower scope to broaden authority.
- Simulate candidate and effective decisions without executing actions,
  resolving secrets, or creating approvals.
- Keep Approval Policy configuration separate from Copilot approval decisions
  and tool-owned business workflow approvals.

## 8. Gantry Copilot Capabilities

### Agent Discovery

- Show only agents published to the employee's groups and workspace scopes.
- Support search, categories, favorites, recently used agents, ownership, and
  clear capability descriptions.
- Display the data and action scope at a useful level without exposing secrets
  or internal policy implementation.
- Freeze typical inputs, expected output, capability summary, data disclosure,
  and action disclosure with the published Agent Revision. Deployment-specific
  temporary availability is separate from requester preferences; favorites and
  recently used Agents are scoped to the requester and Workspace.

### Session Experience

- Start from conversational input or an agent-specific structured form.
- Attach files subject to type, size, malware-scan, and workspace policy.
- Stream status, assistant output, tool activity summaries, and artifacts.
- Distinguish waiting, running, awaiting approval, suspended, failed, canceled,
  and completed states.
- Allow cancellation and safe retry when policy permits.
- Keep the Session open across completed, failed, rejected, and expired Runs so
  an authorized contributor can provide the next instruction.
- After an approved action enters a Tool-owned business approval workflow,
  suspend the Run with a safe external-wait reason. A signed callback resumes
  the same Run's next Agent loop; it does not approve or replay the action.
- Resume the Session view after browser disconnect or a later login.
- Allow Session owners to invite contributors and viewers; membership never
  grants Agent configuration or execution authority.
- Execute at most one Run per Session while accepting later instructions into an
  ordered queue.

### Session History

- List only Sessions where the current employee is a member and organize them by
  intent, mode, current Run, queued work, last activity, my action, and Artifacts.
- Show Run history inside Session detail without exposing runner, lease,
  credential, raw prompt, or cross-user operational internals.
- Keep retry, cancel, and requester guidance attached to the Run; the Run
  requester or Session owner may cancel, while every retry
  creates a new immutable Run.
- Link to the same Session, Run, event, and Artifact identities used by Admin when
  the current actor is authorized to see the corresponding evidence.

### Employee Approvals

Employees may approve only actions from Runs they initiated and only when their
current authorization still permits the action. The approval view must show the exact
target, arguments or human-readable diff, credential identity class, expected
side effects, and expiry. Approval is never represented as a generic "continue"
button without an action preview.

## 9. Enterprise System Integration

Enterprise applications call Gantry from their backend using a separate OAuth
audience. The first-class use case is an existing system such as HR management
invoking a domain agent while keeping its own user interface and workflow.

- Administrators explicitly publish an immutable agent contract to registered
  integration clients.
- API calls require verified delegated-user identity; the integration client
  authenticates the caller but never becomes the Run requester.
- Inputs and outputs follow versioned JSON Schemas.
- The canonical interaction is asynchronous and supports polling and signed
  webhooks.
- Integration clients receive a restricted business-result projection, not
  prompts, terminal output, internal tool payloads, or policy internals.
- Business context such as an employee or case reference improves traceability
  but never grants authority by itself.
- The caller cannot select unapproved tools, credentials, model providers,
  runtime images, or mutable Drafts.
- An employee may own an inbound automation trigger when they have Agent
  `execute`; Trigger management is non-shareable. It creates a Run in a new or
  owner-bound Session with the owner as requester and uses a fixed Service
  Principal for runtime execution rather than inheriting the employee's full
  permissions.
- Inbound hook events require signature verification, replay protection,
  event-id idempotency, schema validation, and trigger-scoped quotas. The
  resulting Run can wait for the owner's ordinary Agent-action approval in
  Copilot; the hook request never approves an action.
- Unattended work is represented by an owner-bound schedule or Webhook trigger,
  not application authority. Each occurrence creates an ordinary Run whose
  human owner is requester; a runtime Service Principal remains execution-only.
- Trigger configuration selects a new Session per occurrence or one exact bound
  Session. Bound occurrences execute sequentially, and a stable occurrence ID
  prevents the same delivery from being queued twice.
- Scheduled Triggers use five-field cron with minute granularity and an explicit
  IANA time zone. Missed or nonexistent local instants are skipped; a repeated
  local instant runs once at its first UTC mapping. Schedule edits create a new
  schedule revision rather than changing historical occurrence identity.

The complete contract is defined in
[Enterprise Agent Invocation API](../architecture/enterprise-integration-api.md)
and [Enterprise Agent API Contracts](../architecture/enterprise-agent-api-contracts.md).

## 10. Functional Requirements

| ID | Requirement |
| --- | --- |
| FR-01 | Every run references an immutable Agent Revision plus immutable policy and tool-binding versions. |
| FR-02 | Admin and Copilot enforce distinct API audiences and permission sets. |
| FR-03 | All action authorization is re-evaluated at execution time. |
| FR-04 | Live views reconnect without loss within the declared retention window; an expired cursor returns a replacement snapshot and earliest available cursor rather than silently skipping content. |
| FR-05 | Approval requests survive application and runner restarts. |
| FR-06 | Cancellation propagates to tools, PTY processes, and sandbox cleanup. |
| FR-07 | Provider and enterprise credentials are injected only by trusted gateways. |
| FR-08 | Evaluation mode prevents unapproved external writes. |
| FR-09 | Production trajectory export applies configurable redaction before storage. |
| FR-10 | Audit exports identify the actor, policy, resource version, action, and result. |
| FR-11 | Registered enterprise systems can invoke only agents explicitly published to their client identity. |
| FR-12 | API-started Sessions require a verifiable delegated user; integration clients and runtime Service Principals cannot impersonate the Run requester. |
| FR-13 | Integration Run results conform to a versioned output contract and support polling or signed webhooks. |
| FR-14 | Stale runner assignments are fenced at every effect-capable gateway by a monotonic lease epoch. |
| FR-15 | Model and PTY streams remain durably replayable without requiring one PostgreSQL transaction per token or output fragment. |
| FR-16 | Approval, cancellation, expiry, policy change, lease loss, and execution claims resolve through one atomic action state machine with at most one consumed execution permit. |
| FR-17 | Every Agent Revision pins an immutable Prompt Snapshot plus the exact Skill artifact, Plugin, Tool Descriptor, and Tool Binding digests. |
| FR-18 | Multiple named Drafts and test Deployments may coexist while one workspace-owned Agent retains one unambiguous default Production Deployment. |
| FR-19 | Skill requirements cannot grant or broaden tool authority; all required tools must be satisfied by explicit Agent Tool Bindings. |
| FR-20 | MCP discovery and CLI registration cannot silently change a deployed Agent Revision. |
| FR-21 | Installing or enabling a Plugin never grants an Agent all contained Skills or Tools; Agent bindings remain explicit. |
| FR-22 | Agent metadata read, configuration read, Draft edit, and execution permissions are independently grantable and revocable. |
| FR-23 | User-owned inbound Webhook Triggers create ordinary Runs in new or owner-bound Sessions; they cannot broaden Agent or Policy authority, and any required Agent-action approval belongs to the owner requester. |
| FR-24 | Unattended schedules and Webhooks are owner-bound Triggers; losing owner authority or archiving a bound Session disables new occurrences without rewriting historical evidence. |
| FR-25 | Personal, shared, and channel Sessions use fixed owner, contributor, and viewer roles; membership never grants Agent configuration or execution authority. |
| FR-26 | Every accepted instruction has one immutable Run requester, and only that requester can decide the Run's Agent-action approvals. |
| FR-27 | Stable Webhook event IDs and scheduled occurrence IDs are committed before queueing so retries cannot create duplicate Session Messages or Runs. |
| FR-28 | A Run requester or current Session owner may cancel a queued or active Run, but Session ownership never grants Agent-action approval authority. |
| FR-29 | Scheduled Triggers accept five-field cron plus an IANA time zone, use skip-only misfire semantics, execute a repeated local time once, and atomically advance the next planned instant with occurrence creation. |
| FR-30 | Webhook secret rotation atomically invalidates the previous key with no overlap; secret-bearing command replays are redacted, and rejected requests cannot reserve an occurrence event ID. |

## 11. Quality Attributes

### Security

Default-deny authorization and network policy, short-lived identities, isolated
execution, encrypted transport and storage, and attributable audit events.

### Reliability

Control-plane restarts must not lose accepted Session instructions or approval decisions.
Runner loss must produce a terminal or recoverable state rather than an
indefinitely active run.

### Reproducibility

A completed run must identify all material configuration versions, model route,
tool versions, runtime image digest, and evaluation fixtures used.

### Operability

The system must expose health, metrics, traces, structured logs, queue state,
capacity, and cleanup status without requiring sandbox access.

### Accessibility

Both web applications target WCAG 2.2 AA, full keyboard operation, visible
focus, screen-reader labels, non-color-only state indicators, and reduced-motion
support.

## 12. Success Measures

The first production pilot should measure:

- Median time to create, review, and publish an agent.
- Percentage of employee tasks completed without operator intervention.
- Approval request latency and abandonment rate.
- Run success, cancellation, infrastructure failure, and policy-denial rates.
- Percentage of active agents protected by a required evaluation suite.
- Regression escape rate after publication.
- Mean time to identify the cause of a failed run from the Admin timeline.
- Percentage of external actions executed with a traceable user and policy.
- Integration Run success, webhook delivery latency, duplicate suppression,
  and contract-validation failure rates.

Initial targets should be set after an internal baseline pilot rather than
invented before representative agents and workloads exist.

## 13. Frontend Design Boundary

This document defines product capabilities, not page-level behavior. The
[Admin Site Design](admin-site-design.md) and
[Copilot Site Design](copilot-site-design.md) own route and interaction details;
[Frontend UX Design](frontend-ux-design.md) owns cross-application rules.
