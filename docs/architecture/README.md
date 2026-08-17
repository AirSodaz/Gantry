# Architecture Documentation

This layer defines the technical contracts and runtime boundaries. Product
behavior is authoritative in [Product](../product/README.md); implementation
status is authoritative in [Delivery](../delivery/README.md).

| Document | Use it for |
| --- | --- |
| [System Architecture](system-architecture.md) | Components, deployment, and lifecycle |
| [Data and Event Model](data-and-event-model.md) | Entities, state machines, events, retention, and recovery |
| [API and Protocols](api-and-protocols.md) | Public APIs, streaming, runner protocol, and compatibility |
| [Copilot Resource Contracts](copilot-resource-contracts.md) | Typed employee resources, commands, authorization, events, and recovery |
| [Agent Access Contracts](agent-access-contracts.md) | Typed Agent ACL grants, owner transfer, recovery, and Admin/Copilot boundaries |
| [Runner Attachment Contracts](runner-attachment-contracts.md) | Lease-bound brokered Attachment reads and sandbox materialization |
| [External Business Approval Callback Contracts](external-business-approval-contracts.md) | Tool-owned business approval waits, signed callbacks, and same-Run resume |
| [Enterprise Agent API Contracts](enterprise-agent-api-contracts.md) | Typed enterprise invocation and user-owned inbound automation triggers |
| [Admin Governed Resource Contracts](admin-governed-resource-contracts.md) | Typed Admin OpenAPI target for Policies, Evaluations, Integrations, and Platform |
| [Control-Plane Design Contract](control-plane-design.md) | Module ownership, transactions, asynchronous work, and restart recovery |
| [Enterprise Invocation API](enterprise-integration-api.md) | Server-to-server agent invocation |
| [Agent Runner V1](agent-runner-v1.md) | Current runner execution boundary |
