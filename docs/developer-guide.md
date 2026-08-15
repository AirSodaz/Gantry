# Developer Guide

## Source of truth

Gantry is a pnpm workspace coordinated by Moon. `apps/*` contains the separate
Admin and Copilot Vite applications; `control-plane` is the Go modular monolith;
`runner` is the Rust workspace; `packages/contracts/openapi` owns public HTTP
schemas; top-level `proto` owns the private runner protocol; and
`packages/api-client` owns generated TypeScript clients.

OpenAPI 3.1 is the primary contract for every public HTTP API. Protobuf is
reserved for the runner session. Connect Go is used only to serve that runner
gRPC endpoint; no public Admin, Copilot, or enterprise HTTP route is a Connect
endpoint.

Application code uses the `ObjectStore` port. MinIO is only the local
S3-compatible adapter and can be replaced by another S3-compatible service
without changing domain or service interfaces.

[Implementation Status](implementation-status.md) is the capability truth for a
named repository baseline. Target behavior lives in the product and engineering
design documents; accepted ADRs constrain implementation. The configuration
ownership and compilation model is defined in
[Agent Configuration, Skills, and Tools](agent-configuration-and-tooling.md).

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
status marks in `packages/design-system`. Keep authentication, API clients, and
feature workflows in their owning application under `apps/copilot-web/src` or
`apps/admin-web/src`; do not move business-specific components into the shared
package just to shorten an import path.

## Bootstrap

Install the pinned tools from `.prototools` (Moon, Node, pnpm, Go, Rust, and
Buf), then run:

```sh
pnpm install --frozen-lockfile
pnpm contracts:generate
pnpm verify
```

The repository expects Moon `>=2.4.6, <3.0.0`, Node `22.13.0`, pnpm `11.21.0`,
Go `1.26.6`, Rust `1.97.0`, and Buf `1.72.0`. `moon run :build` and
`moon run :test` are the canonical workspace task entry points; package scripts
are thin adapters for local IDE use.

## Local environment

Copy `.env.example` to `.env` when running services outside Compose. The
canonical stack is:

```sh
moon run deploy-compose:up
```

It starts PostgreSQL on `5432`, MinIO API on `9000`, MinIO console on `9001`,
Dex on `5556`, public OpenAPI-owned HTTP on `8080`, and the private runner
gRPC endpoint on `8081`. Admin Vite runs on `3001`; Copilot Vite runs on `3002`. Both proxy
`/api` to `http://localhost:8080`.

Do not use MinIO SDK types outside the storage adapter. Runner mTLS variables
are intentionally explicit (`GANTRY_RUNNER_CA_FILE`, `GANTRY_RUNNER_CERT_FILE`,
and `GANTRY_RUNNER_KEY_FILE`) and remain unset for the local plaintext h2 smoke
test.

### Runner development smoke test

For a fast non-Docker lifecycle check, run PostgreSQL and an S3-compatible
endpoint natively on the addresses configured in `.env.example`. The object
store only needs to accept TCP connections for this runner proof; Dex is
not required. Copy the example to `.env.local`, set the database and object
store values for those native services, then enable the developer probe:

```sh
GANTRY_DEVELOPMENT_MODE=true
GANTRY_DEVELOPMENT_API_TOKEN=local_development_token
```

From Git Bash, run:

```sh
moon run deploy-local:runner-smoke
```

The task reads `.env.local` automatically, builds temporary control-plane and
runner binaries, waits for `/readyz`, and verifies completion, cancellation,
and runner-process loss. It always terminates the processes it started and
prints their logs on failure. It does not validate OIDC, Dex fixtures, or
the full Copilot API; use the Compose Copilot smoke test for those paths.

### Agent runner lifecycle smoke test

The Compose stack enables a token-protected, development-only lifecycle probe.
It is not part of the public OpenAPI surface and must never be enabled in a
production deployment. Set `GANTRY_DEVELOPMENT_MODE=true` and provide
`GANTRY_DEVELOPMENT_API_TOKEN`; startup rejects development mode without a
token. The local Compose default is `gantry_development_token`, which is for
local testing only.

The probe exposes three development routes with an `Authorization: Bearer`
token:

- `POST /internal/development/runs` with `{"mode":"complete"}` or
  `{"mode":"await_cancel"}` creates a deterministic run on the durable
  development task path.
- `GET /internal/development/runs/{runID}` returns its lifecycle status, lease epoch,
  and acknowledged event sequence.
- `POST /internal/development/runs/{runID}/cancel` requests asynchronous cancellation.

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
They create a development workspace, two Copilot principals, one Admin
principal with an organization-level role binding, and published deterministic
completion and cancellation agents. They are not production data or a
substitute for the Admin publication workflow.

Compose starts Dex with a SQLite-backed local password database. The control
plane validates issuer, expiry, signature, and the `gantry-copilot-api`
audience before mapping the stable OIDC subject to a local principal. The
`gantry-copilot-smoke` client uses password grant only for the disposable
Compose smoke test; browser-facing Copilot uses the public development issuer
`http://gantry-dex.localhost:5556/dex` with the `gantry-copilot-web` PKCE client.
The Vite development server proxies OIDC endpoints through the frontend origin
so browser login does not depend on cross-origin discovery. The requested Dex
cross-client audience scope preserves the API audience boundary. Start the
frontend with `pnpm dev:copilot` after the Compose stack is ready and sign in
with `copilot-demo@example.test` and password `gantry_demo_password`. The
Approvals page is available for action-time approvals; artifacts remain disabled
until their API exists.

For a natively started control plane or a different browser origin, override
the Vite settings before starting the app:

```sh
VITE_COPILOT_OIDC_ISSUER=http://localhost:5556/dex \
VITE_COPILOT_OIDC_CLIENT_ID=gantry-copilot-web \
VITE_COPILOT_OIDC_SCOPE='openid profile email audience:server:client_id:gantry-copilot-api' \
VITE_COPILOT_API_BASE=http://localhost:8080/api/copilot/v1 \
pnpm dev:copilot
```

The browser flow uses Authorization Code + PKCE and keeps the OIDC session in
`sessionStorage`. Approval requests are bound to an action digest and are
decided by the authenticated Copilot user; the page never receives credentials
or raw agent specs. The workbench is desktop-first in this slice. Narrow breakpoints
keep the component structure usable for a later mobile pass, but mobile visual
parity is not a runner V1 acceptance gate.

### External identity providers

Dex is the local development identity provider, not a production dependency.
It can also broker LDAP, SAML, and OIDC providers when a shared development
issuer is useful. For production, configure both Gantry API issuers and their
corresponding API audiences to the enterprise provider. The browser clients
must use the scopes required by that provider; remove the Dex-specific
`audience:server:client_id:*` scope only when the external provider already
issues the configured API audience in its access token. Dex local users are
declared in `deploy/compose/dex/config.yaml`; use its gRPC API instead of adding
application-level user management when development users must be changed at
runtime.

Run the full durable task flow on a Docker-capable machine with Git Bash:

```sh
moon run deploy-compose:copilot-smoke
```

It validates catalog visibility, OIDC authentication, header-based idempotency,
private task reads, completion, cancellation, action-time approval, runner loss,
control-plane restart, and safe retry. `POST /api/copilot/v1/tasks` requires `Idempotency-Key`; the
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

The persistent Copilot slice includes the development Dex browser login,
catalog, task submission, WebSocket event replay, cancellation, retry, artifact
download, and action-time
approval. The `lifecycle-await-approval` fixture proposes a policy-controlled
shell action, pauses in `awaiting_approval`, and resumes only after a matching
action digest is approved or rejected. The deterministic completion fixture
also produces a runner artifact; production malware scanning and gateway
integration remain outside this slice. The runner emits normalized
model/tool events and keeps sandboxing as a future boundary. Business workflow approvals such
as leave or expense approval remain owned by the external tool and are not
represented by the Copilot approval list.

Development mode also validates `GANTRY_DEV_CREDENTIAL_FILE` and
`GANTRY_DEV_CREDENTIAL_KEY` for the encrypted local credential broker. The
broker is a trusted-gateway adapter: it never puts credential values in a run
manifest, runner message, event payload, or browser response. The sample key
in `.env.example` is disposable and must be replaced outside local testing.
Actual enterprise tool-gateway vending is intentionally not part of this
deterministic slice; this adapter is the development seam for that later
integration.

### Admin agent lifecycle

The Admin API is a separate audience boundary at `/api/admin/v1`. Dex uses the
same development issuer and principal mapping as Copilot, but browser and smoke
tokens request the `gantry-admin-api` audience. Gantry then authorizes the
principal from database-backed role bindings: `organization_admin` may manage
every workspace in its organization, while `workspace_agent_editor` is limited
to its bound workspace. A Copilot token cannot call Admin routes, and an agent
outside an administrator's managed workspace is returned as `404`.

The implemented Admin contract is intentionally limited to workspace listing,
agent creation, draft read/update, review and semantic diff, version history,
publication, rollback, and retirement. Draft updates and publication require
an `If-Match` draft revision; publication also requires an approved review for
that exact revision. Publication validates, freezes, and digests the canonical
`gantry.agent/v1` manifest before it becomes visible to Copilot. The
The immutable version owns the complete execution manifest. Retirement hides the
agent from the catalog without deleting historical versions, tasks, or runs.

The Admin workbench is desktop-first. Start it after Compose is ready:

```sh
pnpm dev:admin
```

Open `http://localhost:3001` and sign in as `admin-demo@example.test` with password
`gantry_admin_password`. The corresponding PKCE client is `gantry-admin-web`.
For a native control plane or different browser origin, override:

```sh
VITE_ADMIN_OIDC_ISSUER=http://localhost:5556/dex \
VITE_ADMIN_OIDC_CLIENT_ID=gantry-admin-web \
VITE_ADMIN_OIDC_SCOPE='openid profile email audience:server:client_id:gantry-admin-api' \
VITE_ADMIN_API_BASE=http://localhost:8080/api/admin/v1 \
pnpm dev:admin
```

Run the direct API lifecycle smoke on a Docker-capable machine with Git Bash:

```sh
moon run deploy-compose:admin-smoke
```

It obtains real Admin and Copilot Dex tokens, creates a draft, verifies
revision preconditions, publishes an immutable version, observes catalog
visibility and successful execution, then retires the agent. It prints relevant
service logs and removes its Compose project on failure.

The Admin schema is an intentionally clean pre-release replacement, not a
forward migration. Reset a local Compose database created by an older build
before starting this version:

```sh
docker compose -f deploy/compose/docker-compose.yml down --volumes
```

Do not use this reset against data that needs to be retained. Versioned
migrations begin only once Gantry has an externally released schema contract.
