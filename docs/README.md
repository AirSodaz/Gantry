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

Read the layers in this order:

1. [Product](product/README.md) defines the users, workflows, and two web
   applications.
2. [Architecture](architecture/README.md) defines runtime boundaries, data,
   APIs, and execution protocols.
3. [Governance](governance/README.md) defines security controls and accepted
   decisions.
4. [Delivery](delivery/README.md) tracks implementation reality and execution
   planning.

## Design Status

The documentation has three distinct authority levels:

- **Target design:** product, architecture, security, data, API, evaluation, and
  frontend documents define the intended end state.
- **Accepted decisions:** ADRs and accepted decisions are implementation
  constraints until superseded by another recorded decision.
- **Checked-in reality:** [Implementation Status](delivery/implementation-status.md)
  states what is implemented, partial, designed, or deferred at the named
  repository baseline.

Roadmap placement does not prove completion, and an implemented narrow slice
does not imply that its whole phase has passed the exit gate.

The Admin and Copilot site-design documents now define the target page
inventories, field-level behavior, interaction specifications, and per-page
acceptance criteria. The remaining work is implementation alignment: each target
route must be backed by an authoritative contract, handler, tests, and an updated
implementation-status entry.

## Product Principles

1. Administration and employee use are separate experiences.
2. Every consequential action is attributable, authorized, and auditable.
3. Agents never receive durable provider or enterprise credentials.
4. Execution is isolated and network access is denied by default.
5. Human approval is a durable workflow state, not an in-memory pause.
6. Evaluation must prevent real external side effects.
7. Gantry records structured execution rationale, not private chain-of-thought.
8. Agent definitions and policies are versioned and reproducible.
