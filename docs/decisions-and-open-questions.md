# Decisions and Open Questions

## 1. Accepted Decisions

### ADR-001: Separate Admin and Copilot Products

**Status:** Accepted

Gantry Admin and Gantry Copilot are separate frontend applications and OAuth
clients over shared governed platform services. They have distinct navigation,
API audiences, response models, and deployment origins.

**Reason:** Administrator capabilities, information density, and risk differ
materially from employee task use. A hidden-menu approach creates avoidable
authorization and usability risk.

### ADR-002: Single Organization per Initial Installation

**Status:** Accepted

The initial product is self-hosted with one enterprise organization per
installation and multiple logical workspaces.

**Reason:** This is an honest isolation claim for an enterprise deployment while
preserving organization identifiers for future hosted architecture.

### ADR-003: React for Both Web Applications

**Status:** Accepted

Use React and TypeScript for Admin and Copilot, with shared tokens, primitives,
and generated API contracts.

**Reason:** React Flow and Xterm.js are required ecosystem choices, and one
frontend framework reduces duplicated infrastructure and accessibility work.

### ADR-004: Modular Go Control Plane

**Status:** Accepted

Begin with a modular monolith rather than independent control-plane
microservices.

**Reason:** Run and approval transitions require strong transactional behavior.
Module boundaries allow later extraction without paying distributed-systems cost
before scale or ownership requires it.

### ADR-005: Rust Runner with Outbound gRPC

**Status:** Accepted

The Rust runner opens an outbound mutually authenticated gRPC stream to receive
assignments and send events. Sandboxes expose no inbound management port.

**Reason:** This fits isolated and restricted networks, simplifies identity and
firewall policy, and supports ordered bidirectional control.

### ADR-006: PostgreSQL plus Object Storage

**Status:** Accepted

PostgreSQL stores transactional state and append-only event metadata. Large
artifacts, terminal chunks, snapshots, and fixtures use S3-compatible object
storage.

**Reason:** The combination supports durable transactions and scalable binary
content without introducing a specialized event store prematurely.

### ADR-007: Kubernetes and gVisor in Production

**Status:** Accepted

Production runs use ephemeral Kubernetes workloads with gVisor by default.
Docker Compose is the supported local development path.

**Reason:** Kubernetes supplies scheduling and lifecycle primitives; gVisor adds
a stronger userspace-kernel boundary for untrusted command execution.

### ADR-008: Gantry Owns the Initial Agent Loop

**Status:** Accepted

The runner executes Gantry's agent-loop contract with pluggable LLM and MCP
adapters. Arbitrary third-party agent frameworks are not first-class opaque
runtimes in the initial release.

**Reason:** Governance, event semantics, approval boundaries, cancellation, and
evaluation require observable control points.

### ADR-009: Independent Single-Agent Runs First

**Status:** Accepted

The first release supports independent agent task runs. Multi-agent delegation
and workflow graphs are deferred.

**Reason:** Reliable single-run lifecycle, authority, and evaluation are
prerequisites for safe orchestration.

### ADR-010: Action-Time Authorization

**Status:** Accepted

Published configuration sets a maximum authority, but every concrete tool or
external action is authorized again using current actor and runtime context.

**Reason:** User roles, target resources, policy, credentials, and risk may
change between publication and execution.

### ADR-011: Credentials Stay Behind Trusted Gateways

**Status:** Accepted

Agent specs use credential references. Provider and enterprise credentials are
resolved and injected by trusted gateways, with very short-lived tool-side
exceptions only when proxying is impossible.

**Reason:** Prompt, tool, runner, and shell compromise must not expose durable
secrets.

### ADR-012: Default-Deny Network Egress

**Status:** Accepted

Sandbox network egress is denied unless traffic is mediated by Gantry or allowed
by an explicit reviewed destination policy.

**Reason:** Command allowlists and prompt controls cannot prevent all exfiltration
or server-side request forgery.

### ADR-013: Two Suspension Modes

**Status:** Accepted

Short waits may retain the sandbox and PTY under a TTL. Durable suspension
destroys the sandbox after an explicit checkpoint and resumes in a new sandbox.

**Reason:** Message history cannot restore arbitrary processes, sockets, or
transactions. The product must state its recovery guarantee accurately.

### ADR-014: No Raw Chain-of-Thought Product Surface

**Status:** Accepted

Gantry records observable actions, structured plans, and concise rationale
summaries, not private raw chain-of-thought.

**Reason:** Raw hidden reasoning is not a reliable audit source and creates
privacy, security, and product risks.

### ADR-015: HTTP-First VCR Evaluation

**Status:** Accepted

The first VCR proxy supports HTTP and HTTPS. Filesystem and PostgreSQL state
deltas complete the first side-effect assertion set.

**Reason:** Generic RPC recording has protocol-specific semantics. The initial
scope is sufficient to prove the evaluation architecture end to end.

### ADR-016: Form-Led Agent Designer

**Status:** Accepted

Agent configuration is primarily a validated form with an advanced source view.
React Flow is used only where a graph improves understanding.

**Reason:** A graph canvas adds friction and ambiguity for a single declarative
agent and should not become the product's configuration schema.

### ADR-017: First-Class Enterprise Agent Invocation API

**Status:** Accepted

Existing enterprise systems invoke explicitly published agents through a
separate server-to-server API audience. The enterprise system retains its own
UI; Gantry provides asynchronous governed execution and schema-validated
results.

**Reason:** HR, finance, CRM, and other systems need to consume Agent capability
without embedding the Gantry Copilot application or bypassing platform policy.

### ADR-018: Application and Delegated-User Authority Modes

**Status:** Accepted

Enterprise calls use either confidential application identity or a verifiable
delegated-user token exchange. Request fields cannot assert an employee
identity.

**Reason:** System-owned automation and employee-initiated work have different
authority and audit semantics. Supporting both explicitly prevents accidental
impersonation.

### ADR-019: Asynchronous Canonical Invocation

**Status:** Accepted

Enterprise invocation creates a durable task and returns `202 Accepted`, with
polling or signed webhooks for progress and results. A bounded wait preference
may return a quick result from the same task.

**Reason:** Agent execution may include tools, queueing, suspension, and human
approval and cannot rely on one request lifetime.

### ADR-020: Separate Semantic Events from High-Frequency Content Streams

**Status:** Accepted

PostgreSQL stores transactional semantic events and indexes immutable model and
PTY content segments in object storage. Durable cursors include stream offsets;
live provisional deltas never advance the durable cursor.

**Reason:** One transaction per token or terminal fragment would create avoidable
WAL, index, vacuum, and connection pressure. Bounded segmentation preserves
replay and auditability while keeping lifecycle transitions responsive.

### ADR-021: Lease Fencing and Atomic Action Execution

**Status:** Accepted

Every assignment has a monotonic lease epoch. Effect-capable gateways require
the current epoch and atomically claim an action revision before issuing one
single-use execution permit. Approval decisions only update the bound action;
they do not invoke tools.

**Reason:** A serial agent loop still has distributed races among control-plane
replicas, runners, users, timers, cancellation, and failover. Database CAS plus
gateway fencing prevents a stale runner or duplicate approval from producing a
second effect and provides a defined unknown-outcome state.

### ADR-022: Separate Agent Action Approval from Business Workflow Approval

**Status:** Accepted

Gantry owns action-time authorization for effect-bearing agent operations. When
human confirmation is required, only the authenticated task requester may
approve or reject the exact action digest; a published policy may instead allow
or deny it without human confirmation. Business approvals such as leave,
expense, or purchase requests remain owned and presented by the tool or
enterprise system that defines that workflow; Gantry integrates them through a
signed external status callback rather than a universal Admin approver inbox.
Rejecting an Agent action or allowing its approval to expire returns a
structured denial or expiry result to the Agent and leaves the task conversation
available for requester input; a revised consequential action receives a new
digest and decision.

**Reason:** A business approver decides a domain process, while an agent action
approver decides whether one concrete external effect may execute. Combining the
two creates ambiguous authority, duplicates enterprise workflows, and makes
the approval UI imply permissions it cannot safely enforce.

### ADR-023: Development Runner Model Routing

**Status:** Accepted for development deployments

Runner V1 may call OpenAI-compatible or Anthropic endpoints directly only when
the process explicitly enables development direct-provider mode. Production
must use a trusted LLM gateway and must not place provider credentials in Agent
specs, manifests, checkpoints, events, or logs.

**Reason:** Deterministic and local provider testing should exercise the real
agent loop without weakening the production credential and routing boundary.
See [ADR-023](adr-023-runner-model-routing.md).

### ADR-024: Versioned Agent Configuration Assets

**Status:** Accepted

Agent-owned Prompt Snapshots, externally sourced Skill artifacts, installable
Plugins, Tool Servers, Tool Descriptor Versions, CLI Command Profiles, and
Agent Tool Bindings form the governed configuration model. Agent Revisions pin
immutable content digests and Skill source identities. Gantry displays the
version declared by an imported Skill package (or `未声明` when absent) but does
not create a separate Skill version history; multiple artifacts of one Skill
may coexist for testing.
Plugins are installed by the organization, and one or more exact Plugin
Versions may be enabled per workspace for testing or migration before being
selectively bound by Agents. There is no implicit default Plugin Version.
Skills and Plugins cannot grant tool authority, bindings may only narrow
descriptors, multiple descriptor versions may coexist without a default, and
dynamic discovery cannot mutate published Agents.

**Reason:** Reproducibility and review require stable configuration ownership
instead of one mutable specification blob or runtime discovery. See
[ADR-024](adr-024-agent-configuration-assets.md).

### ADR-025: Flat Agent Revisions and Deployment Pointers

**Status:** Accepted

Agents support multiple independent named Drafts and immutable hash-identified
Revisions with required messages. Revision history is flat: optional
`derived from` metadata records provenance without parent, branch, fork, merge,
or rebase semantics. Multiple test Deployments may coexist, while one default
Production Deployment points to the active Revision.

**Reason:** Parallel Drafts and test pointers support debugging without imposing
a source-control graph or unsafe configuration merge expectations. Reviews,
runs, evaluations, publication, and rollback remain bound to exact immutable
Revisions. See [ADR-025](adr-025-flat-agent-revisions.md).

### ADR-026: Agent-Scoped Independent Permissions

**Status:** Accepted

Every Agent has an explicit ACL that independently controls metadata discovery,
configuration read, Draft editing, Review decisions, test Deployment
management, Production publication, run inspection, execution, and ACL
management. `execute` does not imply configuration read, and configuration read
does not imply execution. Effective access is the intersection of organization
and workspace authorization, Agent ACL, resource state, Deployment or
integration publication, and action-time policy.

**Reason:** Different teams and service identities need to use the same Agent
with different visibility and authority. Agent-level grants provide that control
without weakening the existing default-deny and action-time authorization
boundaries. See [ADR-026](adr-026-agent-scoped-permissions.md).

The initial ACL implementation is Allow-only. Unset or revoked capabilities are
denied; outer policy or resource constraints explain ineffective grants rather
than creating explicit Deny entries.

### ADR-027: External Skill Package Versions

**Status:** Accepted

Skills are imported from external package sources, such as an `npx skills`
marketplace, a Claude Code skills marketplace, a direct locator, or a manually
provided complete package/local directory. The package source owns the Skill's
declared version. Gantry records the source reference and content digest and
displays the declared version, or `未声明` when the package has no declaration;
it does not create a second Skill version or release lifecycle.

Each import is an immutable Skill Artifact. Manual edits are not performed in
Gantry; a changed package is imported as another artifact. Multiple artifacts,
including artifacts with the same declared version but different digests, may
coexist so test Agent Revisions can compare them without changing Production.
An Agent Revision pins one exact artifact, and later imports never mutate an
existing Revision or Run Manifest.

**Reason:** Version authority remains with the marketplace or package author,
while content-addressed artifacts preserve reproducibility and allow safe
testing without introducing a duplicate release system. See
[ADR-027](adr-027-external-skill-package-versions.md).

### ADR-028: Immutable Policy Versions and Narrowing Scope Intersection

**Status:** Accepted

All Policy types share one catalog and lifecycle. A Policy has one mutable
Draft, immutable Versions, and explicit Bindings; it has no branches or movable
latest-Version runtime pointer. Publishing and Binding are separate actions.
Organization and Workspace Policies compose by intersection, and a lower scope
may only narrow outer authority. Agent Revisions pin exact Policy Versions, and
Run Manifests preserve all contributing Version identities and digests.

Policy simulation is side-effect free. Approval Policies configure whether an
Agent action is allowed, denied, or requires its authenticated task requester;
they do not create an Admin approval queue or absorb business workflow
approvals.

**Reason:** Exact immutable inputs make authorization reproducible while a
narrowing intersection prevents local configuration from bypassing enterprise
guardrails. Separating authoring, publication, binding, simulation, and approval
decisions keeps each surface understandable and auditable. See
[ADR-028](adr-028-policy-lifecycle-and-scope-intersection.md).

### ADR-029: Retention Classes, Legal Holds, and Verifiable Deletion

**Status:** Accepted; exact durations deferred

Retention is classified by Audit metadata, operational metadata, prompts and
outputs, terminal streams, Artifacts, and Evaluation fixtures. Organization
settings define permitted bounds; Workspace settings may choose values within
those bounds. Audit metadata and signed integrity checkpoints have a minimum
retention floor, while content classes may expire earlier. The product does not
hard-code universal day counts before Legal and Security approve deployment
defaults.

Legal Holds identify an owner, authority basis, scope or selector, and affected
data classes. An active Hold blocks scheduled deletion and key destruction for
matching content and evidence. Deletion is authorized, asynchronous, and
auditable: it enters pending state, re-checks Holds and minimum retention,
deletes permitted content and keys, and retains a digest-preserving Tombstone.
Hold creation, release, deletion, failure, retry, and blocked records are Audit
events.

**Reason:** Separating retention classes keeps sensitive content shorter-lived
than compliance metadata without making a false universal legal assumption.
Holds and Tombstones preserve defensible evidence while allowing eligible
content to be removed. See [ADR-029](adr-029-retention-and-legal-hold.md).

### ADR-030: Single Platform Settings Route and Bounded Workspace Overrides

**Status:** Accepted

Admin exposes one `/platform/settings` route with an explicit
`Organization | Workspace` scope switcher. Organization Administrators own
organization defaults and non-negotiable bounds. Workspace settings inherit or
narrow within those bounds and cannot broaden organization authority or
capacity. There is no separate Workspace Settings page.

The page is a composed projection over typed Organization, Retention, Legal
Hold, Classification, Limit, and Environment resources. It does not create a
second Policy, Integration, Provider, Runner, or Audit configuration surface.
Mutations use section-scoped validation, semantic diffs, expected-ETag conflict
handling, explicit confirmation, and attributable Audit events. Recent activity
links to the canonical Audit explorer; retention deletion remains an
asynchronous, Hold-aware workflow.

**Reason:** One scope-aware entry point makes inheritance and boundaries visible
without duplicating Workspace administration or hiding ownership behind a
god-object. Typed resource ownership keeps authorization, lifecycle, and audit
semantics consistent across platform controls. See
[ADR-030](adr-030-platform-settings-scope-and-composition.md).

## 2. Deferred Questions

These questions do not block the first vertical slice. Each has a required
decision point.

### DQ-001: First Enterprise Identity Provider

Choose the first fully supported OIDC provider and its group/claim conventions.

**Default:** Standards-compliant OIDC plus documented examples for Microsoft
Entra ID and Dex.

**Decision gate:** Before Phase 1 identity integration is declared complete.

### DQ-002: First Model Providers

Choose the first production-supported provider routes based on enterprise data
terms, regions, tool-calling behavior, streaming, reliability, and customer
demand. ADR-023 resolves only development adapters and does not select the
production provider set.

**Default:** Keep the normalized OpenAI-compatible and Anthropic adapter
contracts, then enable only routes approved behind the production LLM gateway;
do not treat compatibility as identical semantics.

**Decision gate:** Before production model traffic or a Phase 1 completion
claim.

### DQ-003: Secret Store Integration

Choose the first supported secret manager.

**Default:** Define an internal interface and implement the deployment team's
existing manager; keep a development-only encrypted local adapter.

**Decision gate:** Before Phase 3.

### DQ-004: Event Content Retention Defaults

Set default retention for prompts, outputs, terminal streams, artifacts, audit
metadata, and evaluation fixtures with legal and security stakeholders.

**Accepted principle:** Shorter retention for content than Audit metadata,
organization-defined bounds, Workspace values within those bounds, Legal Hold
override, and digest-preserving Tombstones. Exact durations remain deferred and
are not product constants. See ADR-029.

**Decision gate:** Before production pilot data is admitted.

### DQ-005: Stronger Isolation Tier

Determine whether specific workloads require microVMs rather than gVisor.

**Default:** gVisor for the pilot; design runner-pool capability negotiation so
a later microVM pool does not change agent specs materially.

**Decision gate:** Threat model and pilot workload review.

### DQ-006: Operator Interactive Terminal

Determine whether a production operator may ever write to a live PTY.

**Default:** Read-only in the initial release. If interactive access is added,
require break-glass authorization, visible employee/operator attribution,
session recording, and a narrow incident-use policy.

**Decision gate:** After pilot operational evidence.

### DQ-007: Policy Engine Extraction

Determine whether typed Go policy modules remain sufficient or a policy engine
such as Cedar or OPA is justified.

**Default:** Typed modules with versioned policy documents and comprehensive
decision tests.

**Decision gate:** When rule ownership, cross-resource expressions, or external
policy authoring cannot be maintained cleanly.

### DQ-008: Scheduled and Event-Triggered Tasks

Define service-account ownership, source-event authentication, deduplication,
budgets, and approval routing for non-interactive tasks.

**Default:** Use the accepted Agent Invocation API for explicit application-
identity calls first. Scheduling and inbound event subscriptions remain later
trigger mechanisms over the same task contract.

**Decision gate:** Post-GA prioritization.

### DQ-009: Multi-Agent Orchestration Semantics

Define delegation authority, shared context, budgets, cancellation, approval
ownership, event hierarchy, and evaluation before adding a graph designer.

**Default:** A parent cannot delegate more authority than the intersection of
its own authority and the child agent's published maximum.

**Decision gate:** Post-GA design phase.

### DQ-010: SaaS Multi-Tenancy

Define organization isolation for compute, data, keys, networks, identity,
quotas, observability, incident response, and upgrades.

**Default:** Do not market the initial self-hosted workspace model as SaaS tenant
isolation.

**Decision gate:** Before any hosted external-customer offering.

## 3. Decision Process

New material decisions should be recorded as short ADRs containing context,
decision, consequences, rejected alternatives, owner, and date. A deferred
question becomes accepted only when implementation evidence and the responsible
product, engineering, security, or operations owners are identified.

Changes to an accepted decision require updating every affected design document
and the roadmap gate before implementation proceeds.
