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
| [Product Design](product-design.md) | Product boundaries, actors, use cases, requirements, and success measures |
| [Frontend UX Design](frontend-ux-design.md) | Information architecture, workflows, screens, states, and design system |
| [System Architecture](system-architecture.md) | Components, deployment topology, runtime lifecycle, and repository structure |
| [Security and Governance](security-and-governance.md) | Identity, authorization, credentials, isolation, approvals, and audit controls |
| [Data and Event Model](data-and-event-model.md) | Core entities, state machines, event schema, retention, and recovery semantics |
| [API and Protocols](api-and-protocols.md) | External APIs, streaming interfaces, runner protocol, and compatibility rules |
| [Enterprise Agent Invocation API](enterprise-integration-api.md) | Server-to-server invocation from HR and other enterprise systems |
| [Evaluation Design](evaluation-design.md) | Golden cases, VCR replay, state assertions, scoring, and evaluation safety |
| [Construction Roadmap](roadmap.md) | Phases, deliverables, dependencies, verification gates, and staffing assumptions |
| [Decisions and Open Questions](decisions-and-open-questions.md) | Accepted architectural decisions and questions deferred until evidence exists |
| [Developer Guide](developer-guide.md) | Monorepo bootstrap, Moon tasks, local services, contracts, and verification |

## Design Status

This is the baseline design for implementation. Decisions marked **Accepted**
are implementation constraints. Items under **Deferred Questions** do not block
the first milestone and must be resolved before their named decision gate.

## Product Principles

1. Administration and employee use are separate experiences.
2. Every consequential action is attributable, authorized, and auditable.
3. Agents never receive durable provider or enterprise credentials.
4. Execution is isolated and network access is denied by default.
5. Human approval is a durable workflow state, not an in-memory pause.
6. Evaluation must prevent real external side effects.
7. Gantry records structured execution rationale, not private chain-of-thought.
8. Agent definitions and policies are versioned and reproducible.
