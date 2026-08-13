# Developer Guide

## Source of truth

Gantry is a pnpm workspace coordinated by Moon. `apps/*` contains the separate
Admin and Copilot Vite applications; `control-plane` is the Go modular monolith;
`runner` is the Rust workspace; `packages/contracts` owns OpenAPI and protobuf
source schemas; `packages/api-client` owns generated TypeScript clients.

OpenAPI 3.1 is the primary contract for every public HTTP API. Protobuf is
reserved for the runner session. Connect Go is used only to serve that runner
gRPC endpoint; no public Admin, Copilot, or enterprise HTTP route is a Connect
endpoint.

Application code uses the `ObjectStore` port. MinIO is only the local
S3-compatible adapter and can be replaced by another S3-compatible service
without changing domain or service interfaces.

## Bootstrap

Install the pinned tools from `.prototools` (Moon, Node, pnpm, Go, Rust, and
Buf), then run:

```sh
pnpm install --frozen-lockfile
pnpm contracts:generate
pnpm verify
```

The repository expects Moon `>=1.39.0 <2.0.0`, Node `22.12.0`, pnpm `11.21.0`,
Go `1.26.5`, Rust `1.84.0`, and Buf `1.55.1`. `moon run :build` and
`moon run :test` are the canonical workspace task entry points; package scripts
are thin adapters for local IDE use.

## Local environment

Copy `.env.example` to `.env` when running services outside Compose. The
canonical stack is:

```sh
moon run deploy-compose:up
```

It starts PostgreSQL on `5432`, MinIO API on `9000`, MinIO console on `9001`,
public OpenAPI-owned HTTP on `8080`, and the private runner gRPC endpoint on
`8081`. Admin Vite runs on `3001`; Copilot Vite runs on `3002`. Both proxy
`/api` to `http://localhost:8080`.

The `minio-init` container creates the development buckets. Do not use MinIO
SDK types outside the storage adapter. Runner mTLS variables are intentionally
explicit (`GANTRY_RUNNER_CA_FILE`, `GANTRY_RUNNER_CERT_FILE`, and
`GANTRY_RUNNER_KEY_FILE`) and remain unset for the local plaintext h2 smoke
test.

## Contracts

`pnpm contracts:generate:openapi` generates TypeScript types from both OpenAPI
documents into `packages/api-client/src/generated`. `buf generate proto` emits
Go protobuf and Connect bindings into `control-plane/gen`; Rust generates tonic
bindings at build time from the same proto source. Generated files are never
hand-edited. `pnpm contracts:check` regenerates and fails if tracked generated
outputs differ.

Phase 0 intentionally excludes persistence, authentication, scheduling, task
execution, and UI workflows. Its runner smoke path only registers, heartbeats,
receives control-plane acknowledgements, and shuts down on cancellation.
