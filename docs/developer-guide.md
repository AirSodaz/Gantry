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

## Code Structure

Keep Gantry's code clean by default and continuously. Do not introduce glue
layers, god files, transitional compatibility paths, or migration code when a
clean current-state design is available. Organize code around stable ownership
and use cases, retain explicit transport/domain/persistence boundaries, and
remove superseded paths in the same change.

Split a module before a new responsibility would combine transport, persistence,
domain transitions, infrastructure wiring, or development-only fixtures.
Prefer a clear replacement over a narrowly scoped patch that preserves
avoidable complexity.

Keep reusable visual primitives such as buttons, icon buttons, text fields, and
status marks in `packages/design-system`. Keep authentication, API clients,
catalog controls, task controls, and page composition in the owning app under
`apps/copilot-web/src`; do not move business-specific components into the
shared package just to shorten an import path.

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
Keycloak on `8180`, public OpenAPI-owned HTTP on `8080`, and the private runner
gRPC endpoint on `8081`. Admin Vite runs on `3001`; Copilot Vite runs on `3002`. Both proxy
`/api` to `http://localhost:8080`.

The `minio-init` container creates the development buckets. Do not use MinIO
SDK types outside the storage adapter. Runner mTLS variables are intentionally
explicit (`GANTRY_RUNNER_CA_FILE`, `GANTRY_RUNNER_CERT_FILE`, and
`GANTRY_RUNNER_KEY_FILE`) and remain unset for the local plaintext h2 smoke
test.

### Native Phase 0 smoke test

For a fast non-Docker lifecycle check, run PostgreSQL and an S3-compatible
endpoint natively on the addresses configured in `.env.example`. The object
store only needs to accept TCP connections for this Phase 0 proof; Keycloak is
not required. Copy the example to `.env.local`, set the database and object
store values for those native services, then enable the developer probe:

```sh
GANTRY_DEVELOPMENT_MODE=true
GANTRY_PHASE0_DEV_API_TOKEN=local_phase0_token
```

From Git Bash, run:

```sh
moon run deploy-local:phase0-smoke
```

The task reads `.env.local` automatically, builds temporary control-plane and
runner binaries, waits for `/readyz`, and verifies completion, cancellation,
and runner-process loss. It always terminates the processes it started and
prints their logs on failure. It does not validate OIDC, Keycloak fixtures, or
the full Copilot API; use the Compose Copilot smoke test for those paths.

### Phase 0 lifecycle smoke test

The Compose stack enables a token-protected, development-only lifecycle probe.
It is not part of the public OpenAPI surface and must never be enabled in a
production deployment. Set `GANTRY_DEVELOPMENT_MODE=true` and provide
`GANTRY_PHASE0_DEV_API_TOKEN`; startup rejects development mode without a
token. The local Compose default is `gantry_phase0_dev_token`, which is for
local testing only.

The probe exposes three development routes with an `Authorization: Bearer`
token:

- `POST /internal/phase0/runs` with `{"mode":"complete"}` or
  `{"mode":"await_cancel"}` creates a deterministic run on the durable
  development task path.
- `GET /internal/phase0/runs/{runID}` returns its lifecycle status, lease epoch,
  and acknowledged event sequence.
- `POST /internal/phase0/runs/{runID}/cancel` requests asynchronous cancellation.

On a machine with Docker and Git Bash, run:

```sh
moon run deploy-compose:smoke
```

The task starts a disposable stack, proves completion, cancellation, and
runner-disconnect failure, prints relevant logs if it fails, then removes its
containers and volumes. It proves the deterministic runner lifecycle through
the durable development task path; it does not prove sandbox cleanup or durable
process recovery.

### Persistent Copilot slice

The initial schema is embedded as a clean bootstrap schema because Gantry has
no prior released database shape. Development fixtures are isolated in the
development package and are only seeded when `GANTRY_DEVELOPMENT_MODE=true`.
They create a development workspace, two local principals, and one published
Lifecycle Demo agent. They are not production data or a substitute for the
Admin publication workflow.

Compose imports the `gantry-dev` Keycloak realm. The control plane validates
issuer, expiry, signature, and the `gantry-copilot-api` audience before mapping
the stable OIDC subject to a local principal. The `gantry-copilot-smoke` client
uses password grant only for the disposable Compose smoke test; browser-facing
Copilot uses the public development issuer
`http://gantry-keycloak.localhost:8180/realms/gantry-dev` with the
`gantry-copilot-web` PKCE client. The hostname must resolve to `127.0.0.1` in
the browser; current Chromium-based browsers resolve `*.localhost`
automatically. Start the frontend with `pnpm dev:copilot` after the Compose
stack is ready and sign in with the `copilot-demo` fixture account. The
approvals and artifacts navigation entries remain intentionally disabled until
their APIs exist.

For a natively started control plane or a different browser origin, override
the Vite settings before starting the app:

```sh
VITE_COPILOT_OIDC_ISSUER=http://localhost:8180/realms/gantry-dev \
VITE_COPILOT_OIDC_CLIENT_ID=gantry-copilot-web \
VITE_COPILOT_API_BASE=http://localhost:8080/api/copilot/v1 \
pnpm dev:copilot
```

The browser flow uses Authorization Code + PKCE and keeps the OIDC session in
`sessionStorage`; it does not send placeholder requests for approvals or
artifacts. The workbench is desktop-first in this slice. Narrow breakpoints
keep the component structure usable for a later mobile pass, but mobile visual
parity is not a Phase 0 acceptance gate.

Run the full durable task flow on a Docker-capable machine with Git Bash:

```sh
moon run deploy-compose:copilot-smoke
```

It validates catalog visibility, OIDC authentication, header-based idempotency,
private task reads, completion, cancellation, runner loss, control-plane restart,
and safe retry. `POST /api/copilot/v1/tasks` requires `Idempotency-Key`; the
first request returns `201`, an identical retry returns `200`, and a changed
request using the same key returns `409`. Retry creates a new immutable run only
after the current run has failed or been canceled; it cannot replace active work.

## Contracts

`pnpm contracts:generate:openapi` generates TypeScript types from both OpenAPI
documents into `packages/api-client/src/generated`. `buf generate proto` emits
Go protobuf and Connect bindings into `control-plane/gen`; Rust generates tonic
bindings at build time from the same proto source. Generated files are never
hand-edited. `pnpm contracts:check` regenerates and fails if tracked generated
outputs differ.

The persistent Copilot slice includes the development Keycloak browser login,
catalog, task submission, polling, cancellation, and retry. It intentionally
excludes Admin agent lifecycle, approvals, artifacts, event streaming, real
models/tools, and sandboxing. It uses the deterministic fixture agent to validate task ownership,
runner leases, low-frequency event persistence, cancellation, failure, and
retry without executing external effects.
