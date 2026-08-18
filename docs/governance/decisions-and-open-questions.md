# Decisions and Open Questions

## 1. Accepted Decisions

### ADR-001: Separate Admin and Copilot Products

**Status:** Accepted

Gantry Admin and Gantry Copilot are separate frontend applications and OAuth
clients over shared governed platform services. They have distinct navigation,
API audiences, response models, and deployment origins.

**Reason:** Administrator capabilities, information density, and risk differ
materially from employee Session use. A hidden-menu approach creates avoidable
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

The first release supports independent Agent Runs. Multi-agent delegation
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

**Status:** Superseded by ADR-036

The earlier design allowed confidential application identity or a verifiable
delegated-user token exchange. Direct application authority is no longer part
of the target contract.

**Reason:** A machine identity cannot receive requester-bound Agent-action
approval. Unattended work now uses a human-owner-bound Webhook or scheduled
trigger, while direct Enterprise invocation requires a verified delegated user.

### ADR-019: Asynchronous Canonical Invocation

**Status:** Accepted

Enterprise invocation creates a durable personal Session and first Run and
returns `202 Accepted`, with
polling or signed webhooks for progress and results. A bounded wait preference
may return a quick result from the same Run.

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

**Status:** Accepted. Gantry owns action-time authorization for effect-bearing
Agent operations; only the authenticated Run requester decides an exact action
digest. Business workflow approvals remain in the owning tool or enterprise
system. A pending Tool result suspends the same Run; a signed callback resumes
the next Agent loop without replaying the Tool call. Rejection or expiry leaves
the Copilot conversation available for a new instruction. See
[ADR-022](adr/adr-022-agent-action-approval-boundary.md).

### ADR-023: Development Runner Model Routing

**Status:** Accepted for development deployments. Runner V1 may use direct
OpenAI-compatible or Anthropic adapters only with explicit development opt-in;
production uses a trusted gateway and never stores provider credentials in
execution inputs or evidence. See [ADR-023](adr/adr-023-runner-model-routing.md).

### ADR-024: Versioned Agent Configuration Assets

**Status:** Accepted. Prompt Snapshots, external Skill Artifacts, Plugins, Tool
Servers, descriptor versions, CLI profiles, and explicit bindings are governed
assets. Revisions pin exact digests; Skills and Plugins cannot grant authority;
runtime discovery cannot mutate deployed configuration. See
[ADR-024](adr/adr-024-agent-configuration-assets.md).

### ADR-025: Flat Agent Revisions and Deployment Pointers

**Status:** Accepted. Agents use independent named Drafts and flat,
hash-identified Revisions with required messages. Multiple Test Deployments may
coexist while one Production pointer is active; there are no merge or rebase
semantics. See [ADR-025](adr/adr-025-flat-agent-revisions.md).

### ADR-026: Agent-Scoped Independent Permissions

**Status:** Accepted. Agent ACL capabilities independently govern metadata,
configuration, editing, review, deployment, run inspection, execution, and ACL
management. The initial model is Allow-only with default denial; outer
constraints explain ineffective grants. See
[ADR-026](adr/adr-026-agent-scoped-permissions.md).

### ADR-027: External Skill Package Versions

**Status:** Accepted. External packages own declared Skill versions. Gantry
stores source references and digests, displays the declared version or `未声明`,
and permits multiple immutable artifacts to coexist for testing. See
[ADR-027](adr/adr-027-external-skill-package-versions.md).

### ADR-028: Immutable Policy Versions and Narrowing Scope Intersection

**Status:** Accepted. Policies use one Draft, immutable Versions, and explicit
Bindings. Organization and Workspace policies intersect monotonically;
Revisions pin exact Policy Versions. Simulation is side-effect free and Approval
Policies never create an Admin approval inbox. See
[ADR-028](adr/adr-028-policy-lifecycle-and-scope-intersection.md).

### ADR-029: Retention Classes, Legal Holds, and Verifiable Deletion

**Status:** Accepted; exact durations deferred. Retention is class-based with
organization bounds, Workspace narrowing, Legal Hold protection, asynchronous
deletion, and digest-preserving Tombstones. See
[ADR-029](adr/adr-029-retention-and-legal-hold.md).

### ADR-030: Single Platform Settings Route and Bounded Workspace Overrides

**Status:** Accepted. `/platform/settings` is one scope-aware projection over
typed platform resources. Workspace values may inherit or narrow organization
bounds; Settings does not duplicate Policy, Provider, Runner, Integration, or
Audit ownership. See [ADR-030](adr/adr-030-platform-settings-scope-and-composition.md).

### ADR-031: Protocol and Storage Boundaries

**Status:** Accepted. OpenAPI is authoritative for public HTTP APIs, protobuf
and Connect are private to the mutually authenticated runner session, and the
control plane accesses object storage through an adapter port. See
[ADR-031](adr/adr-031-protocol-and-storage-boundaries.md).

### ADR-032: Historical Inbound Webhook Task Contract

**Status:** Superseded by ADR-037. Its human-owner, Service Principal,
transport-authentication, and occurrence-idempotency boundaries remain valid,
but occurrences now create Runs in new or bound Sessions. See
[ADR-032](adr/adr-032-inbound-webhook-task-contract.md).

### ADR-033: Agent ACL Grants and Explicit Owner Transfer

**Status:** Accepted. Agent access is a collection of explicit Allow-only
`AgentAccessGrant` resources over principal, group, or registered service-
identity subjects. Capabilities never inherit from one another. Agent owner
identity is stored separately from grants; owner transfer is atomic, gives the
new principal an explicit `access.manage` grant, and leaves the old owner's
other grants unchanged. The last recovery path is protected by transaction-
time validation. See [ADR-033](adr/adr-033-agent-acl-grants-and-owner-transfer.md).

### ADR-034: Brokered Runner Attachment Materialization

**Status:** Accepted. Runners read bound requester Attachments through a Control
Plane brokered stream over the mTLS Runner session. Materialization is bound to
the exact Run, lease epoch, opaque reference, digest, classification, and
current Policy; object-store credentials, keys, URLs, and unrestricted paths
never cross the Runner boundary. See
[ADR-034](adr/adr-034-brokered-runner-attachment-materialization.md).

### ADR-035: Copilot Published Metadata and Workspace-Scoped Preferences

**Status:** Accepted. Employee-facing input/output metadata and disclosures
are authored in Drafts and frozen with immutable Revisions. Deployment owns
only publication-specific temporary availability. Favorites and recent use are
requester-owned by `(principal_id, workspace_id)`, with recent use recorded
only after successful Session creation and limited to eight Agents. See
[ADR-035](adr/adr-035-copilot-published-metadata-and-preferences.md) and the
[Copilot Resource Contracts](../architecture/copilot-resource-contracts.md).

### ADR-036: Human Requesters and Owner-Bound Automation

**Status:** Accepted. Direct Enterprise invocation requires a verified
delegated user; the integration client authenticates transport but never
becomes Run requester. Unattended Webhook and scheduled Triggers have a human
owner, create ordinary Runs in new or bound Sessions, and use an execution-only
Service Principal. They do not bypass requester approval or create another Run
model. See
[ADR-036](adr/adr-036-human-requester-and-owner-bound-automation.md).

### ADR-037: Agent Sessions and Run Requesters

**Status:** Accepted. The existing Agent remains the company-governed
definition; Gantry does not add Template or per-user Instance resources in the
initial model. Personal, shared, and channel-bound Sessions own conversation and
membership. Each instruction creates a Run whose human initiator is the only
eligible Agent-action approver. Sessions serialize execution while permitting
ordered queued Runs. The Run requester or current Session owner may cancel a
Run, but Session ownership never grants approval or Agent execution authority.
Copilot history defaults to owner-owned Sessions, while the accessible scope
contains all current memberships. A Workspace Agent Editor may explicitly and
auditably add only itself as a viewer to a Session for an Agent in its assigned
Workspace; the role is not an implicit conversation-read bypass.
Webhook and scheduled Triggers choose a new or bound Session at configuration
time, and stable occurrence IDs prevent the same delivery from entering the
queue twice. See
[ADR-037](adr/adr-037-agent-sessions-and-run-requesters.md).

### ADR-038: Scheduled Trigger Time Semantics

**Status:** Accepted. Scheduled Triggers use five-field POSIX-style cron with
minute granularity and an explicit IANA time zone. Missed, uncommitted instants
are skipped rather than replayed after recovery. A nonexistent daylight-saving
local time is skipped, while a repeated local time runs once at its first UTC
instant. Configuration changes increment `schedule_revision`; stable occurrence
IDs and atomic creation prevent duplicate Runs. See
[ADR-038](adr/adr-038-scheduled-trigger-time-semantics.md).

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
