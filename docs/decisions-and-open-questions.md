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

## 2. Deferred Questions

These questions do not block the first vertical slice. Each has a required
decision point.

### DQ-001: First Enterprise Identity Provider

Choose the first fully supported OIDC provider and its group/claim conventions.

**Default:** Standards-compliant OIDC plus documented examples for Microsoft
Entra ID and Keycloak.

**Decision gate:** Before Phase 1 identity integration is declared complete.

### DQ-002: First Model Providers

Choose providers based on enterprise data terms, regions, tool-calling behavior,
streaming, and customer demand.

**Default:** Implement one provider adapter and one OpenAI-compatible adapter,
without treating compatibility as identical semantics.

**Decision gate:** Phase 1 planning.

### DQ-003: Secret Store Integration

Choose the first supported secret manager.

**Default:** Define an internal interface and implement the deployment team's
existing manager; keep a development-only encrypted local adapter.

**Decision gate:** Before Phase 3.

### DQ-004: Event Content Retention Defaults

Set default retention for prompts, outputs, terminal streams, artifacts, audit
metadata, and evaluation fixtures with legal and security stakeholders.

**Default:** Shorter retention for content than audit metadata, configurable by
workspace within organization limits.

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
