# ADR-031: Protocol and Storage Boundaries

**Status:** Accepted
**Date:** 2026-08-13

## Decision

OpenAPI 3.1 is the primary source of truth for Admin, Copilot, and enterprise
HTTP APIs. Protobuf and Connect are limited to the mutually authenticated
runner gRPC session. The public HTTP listener does not mount Connect handlers.

Control-plane services depend on an `ObjectStore` port. The development
implementation is S3-compatible MinIO; provider SDK types and bucket details
remain inside the adapter. A later S3-compatible or managed object store can be
introduced through configuration and adapter replacement.

## Consequences

HTTP clients evolve from OpenAPI schemas and generated TypeScript clients.
Runner protocol compatibility evolves additively through Buf-checked protobuf.
The control plane can test storage behavior against a fake port without MinIO,
while Compose provides a realistic local object-storage service.
