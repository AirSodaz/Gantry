# Gantry Design Documentation

This directory defines the product and engineering design for Gantry, an
enterprise platform for creating, governing, running, and evaluating AI agents.

Gantry exposes two separate user-facing products over one governed platform:

- **Gantry Admin** is the administrator console used to design, publish,
  operate, audit, and evaluate agents.
- **Gantry Copilot** is the employee-facing application used to discover and
  run approved agents within the employee's permissions.

Registered enterprise systems can also consume approved agents through the
server-to-server Agent Invocation API while retaining their own user interface.

## Document Map

| Document | Purpose |
| --- | --- |
| [Implementation Status](implementation-status.md) | Checked-in capabilities, partial slices, design-only areas, and roadmap evidence |
| [Product Design](product-design.md) | Product boundaries, actors, use cases, requirements, and success measures |
| [Agent Configuration, Skills, and Tools](agent-configuration-and-tooling.md) | Agent Prompt, standalone Skill, Plugin, MCP, built-in tool, CLI, binding, validation, and publication model |
| [Gantry Admin Site Design](admin-site-design.md) | Admin navigation, complete page inventory, scope, roles, delivery status, and page specifications |
| [Frontend UX Design](frontend-ux-design.md) | Shared design language, directional information architecture, states, and next page-design contract |
| [System Architecture](system-architecture.md) | Components, deployment topology, runtime lifecycle, and repository structure |
| [Security and Governance](security-and-governance.md) | Identity, authorization, credentials, isolation, approvals, and audit controls |
| [Data and Event Model](data-and-event-model.md) | Core entities, state machines, event schema, retention, and recovery semantics |
| [API and Protocols](api-and-protocols.md) | External APIs, streaming interfaces, runner protocol, and compatibility rules |
| [Enterprise Agent Invocation API](enterprise-integration-api.md) | Server-to-server invocation from HR and other enterprise systems |
| [Evaluation Design](evaluation-design.md) | Golden cases, VCR replay, state assertions, scoring, and evaluation safety |
| [Construction Roadmap](roadmap.md) | Phases, deliverables, dependencies, verification gates, and staffing assumptions |
| [Decisions and Open Questions](decisions-and-open-questions.md) | Accepted architectural decisions and questions deferred until evidence exists |
| [Developer Guide](developer-guide.md) | Monorepo bootstrap, Moon tasks, local services, contracts, and verification |
| [Agent Runner V1](agent-runner-v1.md) | Current runner execution model and verified development boundary |
| [ADR-022](adr-022-protocol-and-storage-boundaries.md) | Public/private protocol and object-storage adapter boundaries |
| [ADR-023](adr-023-runner-model-routing.md) | Development-only direct model routing boundary |
| [ADR-024](adr-024-agent-configuration-assets.md) | Agent Prompt, Skill, Plugin, Tool, CLI, and Agent binding decision |
| [ADR-025](adr-025-flat-agent-revisions.md) | Flat Agent Draft, Revision, test Deployment, Production, and rollback model |
| [ADR-026](adr-026-agent-scoped-permissions.md) | Independent per-Agent metadata, configuration, edit, and execution permissions |
| [ADR-027](adr-027-external-skill-package-versions.md) | External Skill package versions, imported artifacts, and coexistence for testing |

## Design Status

The documentation has three distinct authority levels:

- **Target design:** product, architecture, security, data, API, evaluation, and
  frontend documents define the intended end state.
- **Accepted decisions:** ADRs and accepted decisions are implementation
  constraints until superseded by another recorded decision.
- **Checked-in reality:** [Implementation Status](implementation-status.md)
  states what is implemented, partial, designed, or deferred at the named
  repository baseline.

Roadmap placement does not prove completion, and an implemented narrow slice
does not imply that its whole phase has passed the exit gate.

The current frontend document fixes the product boundary, shared design
language, and directional information architecture. Detailed page inventories,
field-level behavior, interaction specifications, and per-page acceptance
criteria for Admin and Copilot are intentionally the next design phase.

## Product Principles

1. Administration and employee use are separate experiences.
2. Every consequential action is attributable, authorized, and auditable.
3. Agents never receive durable provider or enterprise credentials.
4. Execution is isolated and network access is denied by default.
5. Human approval is a durable workflow state, not an in-memory pause.
6. Evaluation must prevent real external side effects.
7. Gantry records structured execution rationale, not private chain-of-thought.
8. Agent definitions and policies are versioned and reproducible.
