# Architecture Documentation

This layer defines the technical contracts and runtime boundaries. Product
behavior is authoritative in [Product](../product/README.md); implementation
status is authoritative in [Delivery](../delivery/README.md).

| Document | Use it for |
| --- | --- |
| [System Architecture](system-architecture.md) | Components, deployment, and lifecycle |
| [Data and Event Model](data-and-event-model.md) | Entities, state machines, events, retention, and recovery |
| [API and Protocols](api-and-protocols.md) | Public APIs, streaming, runner protocol, and compatibility |
| [Admin Governed Resource Contracts](admin-governed-resource-contracts.md) | Typed Admin OpenAPI target for Policies, Evaluations, Integrations, and Platform |
| [Enterprise Invocation API](enterprise-integration-api.md) | Server-to-server agent invocation |
| [Agent Runner V1](agent-runner-v1.md) | Current runner execution boundary |
