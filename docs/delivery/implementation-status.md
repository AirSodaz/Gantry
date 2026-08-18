# Implementation Status

This document separates the checked-in product from the target design. It is a
source-based snapshot of the current worktree on 2026-08-17. A capability is
not considered complete merely because a package,
table, route, or UI placeholder exists.

## Status Vocabulary

| Status | Meaning |
| --- | --- |
| Implemented | The primary path exists across its required layers and has focused automated verification. |
| Partial | A usable or testable slice exists, but important production behavior or scope remains. |
| Designed | The target behavior is documented, but no complete product slice exists. |
| Deferred | Intentionally outside the current release sequence or waiting for an explicit decision gate. |

## Current Product Slice

The repository currently proves a self-hosted, single-organization development
slice with two independently built web applications, a Go control plane, a Rust
runner, PostgreSQL, S3-compatible object storage, and Dex for local OIDC.

### Implemented

- Separate Admin and Copilot React applications with separate OAuth clients and
  API audiences.
- OpenAPI-owned public Admin and Copilot HTTP contracts and a private
  protobuf/Connect runner session.
- Workspace-scoped Admin Agent target lifecycle: create and edit named Drafts
  with optimistic concurrency, validate, commit hash-identified Revisions,
  review exact Revisions, create multiple Test Deployments, and move one
  Production Deployment pointer. Superseded integer-version Draft/Version/
  Publication routes and tables are not part of the active contract.
- Admin configuration catalog slice: register immutable Skill artifacts,
  Plugin versions, and Tool descriptor versions; enable Plugins per workspace;
  bind exact catalog IDs to Agent Drafts; and revalidate availability at draft
  write and publication time.
- Admin Agent workspace views: scope-authorized Agent Overview, named Draft
  Design, and immutable Revision Detail routes expose Production/Test
  Deployments, draft validation, audit activity, exact hashes, and frozen
  Prompt Snapshot projections.
- Admin Agent Versions workspace: production and active test pointers, Draft
  latest Revision pointers, and chronological immutable Revision history are
  available with links to exact Revision detail.
- Admin home Overview: scope-authorized aggregation of Agent lifecycle state,
  pending reviews, invalid Drafts, active and failed runs, requester approval
  waits, recent publications, and scoped Agent audit activity. Provider health
  and runner capacity remain explicitly unavailable until their owning platform
  services are implemented.
- Prompt Snapshot compilation: committed Agent Revisions persist compiler version,
  ordered prompt content, and a deterministic content digest. Tool descriptor
  schemas can be authored at registration and declared operation lists are
  enforced as the upper bound for Agent bindings.
- Asset inspection metadata: Skill artifact metadata and Plugin manifest JSON
  are persisted and rendered read-only in catalog detail views.
- Plugin workspace enablement now supports explicit, audited disablement; each
  exact Plugin Version remains independently enabled or disabled per workspace.
- Copilot catalog with requester-owned favorites/recent use, Session creation,
  fixed owner/contributor/viewer membership, owner transfer, requester follow-up
  messages after a rejected action, and idempotency for creation, follow-up,
  cancel, retry, favorite, attachment, and approval commands. Session history
  exposes first request, last activity, requester action, Artifact availability,
  compact paged Run history, server-authorized filters, cancellation, retry,
  and live Session events.
- The target Session visibility rule is owner-default history with an explicit
  accessible scope for current owner/contributor/viewer memberships. The
  audited `workspace_agent_editor` self-enrollment-as-viewer command is a
  documented target and remains pending until its Admin route, persistence
  transition, Audit evidence, and authorization tests are implemented.
- Requester-scoped approval list/detail/decision, Artifact browse/detail and
  audited download, requester-owned attachment upload and validation before
  Session binding, and URL-preserved Agent collection/search/category filters
  with owner projections.
- Schema-versioned Session event snapshots and requester-bound cursors carry
  complete Session conversations, Run history, membership, and approval history. Continuous
  frames expose only typed committed-message, content-segment, Run-state,
  Session-state, approval, and Artifact projections; internal Runner event payloads remain
  private to the control plane.
- Approval decisions now return the requester-authorized winning approval
  projection. Concurrent, stale, expired, and idempotency-conflicting decisions
  include that same projection in their error envelope, so the Copilot page
  replaces local controls with durable server evidence. Approval queue and Session
  stream projections include the Agent display name and Session context.
- Copilot Session, Run, approval, Artifact, and Agent lists use requester- and
  filter-bound signed keyset cursors, with stable ordering and incremental
  loading in the Web client. Artifact and approval APIs support their current
  persisted-state filters; the approval queue remains pending-only by design.
- Persistent Sessions, Runs, ordered semantic events, bounded content segments,
  artifacts, durable action records, approval requests, and approval decisions.
- Runner V1 agent loop with deterministic and development provider adapters,
  native workspace tools, shell and PTY support, streamed rule interception,
  context compaction, encrypted checkpoints, cancellation, approval resume, and
  lease-fenced artifact upload.
- Local Compose and host smoke paths for runner, Copilot, Admin, approval, live
  event, and artifact lifecycle behavior.
- Admin Runs workbench: authorized cross-workspace Run list and filters, exact
  Run detail, immutable timeline, tool-action and requester-approval evidence,
  artifact metadata, Deployment/manifest identity, and rendered Admin routes.

### Partial

- **Identity:** Dex proves local OIDC and audience separation. Production IdP
  conventions, group mapping, lifecycle provisioning, and enterprise rollout
  guidance remain open.
- **Model routing:** Direct OpenAI-compatible and Anthropic adapters are allowed
  only for development. The production LLM gateway, credential mediation,
  budgets, route policy, and provider governance are not complete.
- **Authorization:** Workspace membership, initial Admin roles, direct-principal
  Agent grants, and `metadata.read`/`execute` checks on catalog and new Run paths
  exist. The complete ACL subject model, Admin management routes, group and
  service-identity resolution, recovery invariant, policy intersection, and
  complete revocation behavior remain incomplete.
- **Action approval:** One durable, digest-bound approval loop is implemented.
  Requester-visible reads expire elapsed approvals and keep Session follow-up input available;
  cancellation/retry and rejection/expiry continuation are idempotent. A
  background expiry worker reconciles all elapsed requests and delivers their
  durable resolution to an active Runner when present. Retained suspension,
  revocation, and general tool-gateway execution remain incomplete.
- **Artifacts, attachments, and streams:** Output upload, storage, download,
  requester attachment quarantine/binding, live events, and content segments
  exist. The target Runner Attachment contract now defines a scoped brokered
  materialization/read path, but its protobuf extension, broker, persistence,
  and sandbox implementation are not complete, so bound input files remain
  unavailable to an Agent. Production malware scanning, preview isolation,
  retention, compaction, backpressure evidence, and object-store failure
  handling remain.
- **Copilot Agent projection:** Search/category/collection filters, published
  owner projection, Workspace-scoped principal favorites, and the eight most
  recent successful Session creations are persisted and callable. Revision-frozen
  typical input/output metadata and disclosures are still represented by a
  schema-valid generic projection, and Deployment-bound temporary availability
  is not yet sourced from published Admin data.

- **Scheduling and isolation:** Runner registration, assignment, leases,
  cancellation, and runner-loss handling exist. Production runner pools,
  Kubernetes Jobs, gVisor, network enforcement, resource accounting, and
  cleanup controllers do not.
- **Admin configuration UX:** Skills, Plugins, and Tools can be registered,
  listed, explicitly activated/deprecated/retired, and an Agent Draft can
  select exact catalog references. Metadata detail, Agent usage, Plugin
  enablement visibility, Tool descriptor schema display, and catalog
  search/filtering are available. Agent Drafts also expose the Agent-owned
  prompt/model fields and explicit Tool operation constraints. Package upload materialization and content
  inspection, Tool discovery/health, and package-backed import remain
  incomplete. Skill metadata and Plugin manifest inspection is available, but
  package materialization and full contained-asset validation are not. Descriptor schema authoring and
  operation-level narrow binding constraints are implemented for the current
  catalog model; broader schema compatibility and authority checks remain.
- **Admin lifecycle UX:** The Draft/Revision/Deployment core lifecycle is usable.
  The read-only Run evidence workbench and scope-authorized Audit Explorer
  list/detail and signed object-storage Audit export lifecycle are implemented.
  Typed Policy catalog, Draft ETag editing/validation, immutable Versions,
  exact Bindings, side-effect-free Simulation, and Retire are implemented as
  the first governed-resource slice. Evaluation Suite Draft/Case authoring,
  fixture validation, immutable Suite Versions, and exact Run requests are
  implemented as a second slice. Required Gate projections bind the exact Run,
  Revision, and Suite Version; authorized, expiring overrides are audited, and
  comparable deterministic regressions have a read-only projection. The
  evaluator worker, evidence collection, and complete gate derivation are
  still partial. Integrations now have an organization directory, client
  metadata lifecycle, exact Revision publication, and webhook endpoint
  metadata/redelivery routes with an Admin list/detail view; invocation
  execution, signed delivery workers, usage projections, and full capability
  authorization remain incomplete. Platform now has provider/route and runner
  pool/runner metadata management as a partial organization-level slice;
  credentials and classifications expose non-secret metadata. Limit policies,
  Environment Profiles, and the scope-aware Settings composition are now
  implemented as typed metadata with ETag-protected updates and validation;
  production health integration remains incomplete.

### Designed, Not Yet Implemented

- **Automation and remaining Session edges:** Trigger configuration may create a
  new Session or bind one exact owner-owned Session, with occurrence idempotency
  committed before queueing. Trigger persistence/runtime, schedule delivery,
  webhook occurrence admission, channel binding, and disabling bound Triggers
  on owner transfer are not implemented; Trigger ownership is never transferred
  implicitly. External business-approval callbacks and same-Run resume remain
  pending. Agent action rejection and expiry already end the current Run as
  `completed` with `requester_input_required`; the Session stays active for a
  new requester instruction and the denied approval is not reused.
- Package-content ingestion and inspection, Plugin asset expansion, Tool Server
  health/discovery, descriptor schema compatibility, broader Tool Binding
  constraints, and CLI Command Profile catalogs described in
  [Agent Configuration, Skills, and Tools](../product/agent-configuration-and-tooling.md).
- Detailed deployment history, Run mutations, richer Policy projections,
  evaluations, and platform administration remain designed but are not complete
  product areas. Audit list/detail/export currently projects the
  existing append-only event envelope; outcome, risk, correlation, policy
  links, and full authentication context are empty until their durable event
  fields are defined and populated. The current role model does not yet map
  Security Reviewer or Auditor export capabilities, and export processing is
  currently an in-process development worker without restart recovery or a
  durable retry scheduler.
- Per-Agent ACLs with independent metadata read, configuration read, Draft edit,
  Review, deployment, run-inspection, execution, and access-management
  capabilities. Current authorization is workspace-role based.
- Production credential broker, LLM gateway, tool gateway, egress gateway, and
  secret-store integration.
- Credential Reference/Lease, full settings idempotency/audit correlation, and
  production authorization/health projections. Model Provider/Route, Runner Pool/Runner, Limit Policy,
  Environment Profile, and composed Settings management are implemented only as
  a partial Admin metadata slice.
- The scoped Runner attachment-read/materialization contract is documented in
  [Runner Attachment Contracts](../architecture/runner-attachment-contracts.md):
  immutable Run input snapshots, opaque lease-bound references, brokered
  streaming, digest verification, failure semantics, and recovery. The target
  protobuf extension, broker implementation, persistence, and sandbox
  materialization remain unimplemented.
- Revision-frozen Copilot Agent catalog metadata still needs Admin Draft
  authoring, immutable Revision storage/projection, and Deployment-bound
  availability. Favorite/recent-use OpenAPI, generated clients, storage, HTTP
  routes, and Copilot controls are implemented; Admin catalog-metadata authoring
  remains incomplete.
- Enterprise Agent Invocation API execution, signed webhook delivery workers,
  usage projections, required delegated-user token exchange, and owner-bound
  scheduled/Webhook triggers. Integration registration, client metadata,
  publication, and endpoint metadata are implemented as a partial Admin
  management slice; the current stored auth-mode model still permits the
  superseded `application` value and is not evidence of target authority.
- The typed target contract for the Enterprise Agent API and user-owned inbound
  automation triggers is documented in
  [Enterprise Agent API Contracts](../architecture/enterprise-agent-api-contracts.md).
  Direct invocation now requires a verified delegated subject; unattended work
  uses a human-owner-bound trigger. The OpenAPI source, generated client,
  invocation handlers, trigger storage, schedule contract, and signed hook
  worker are not yet implemented.
- Tool-owned external business approval waits and callbacks are defined in
  [External Business Approval Callback Contracts](../architecture/external-business-approval-contracts.md):
  post-requester-approval Tool invocation, `suspended` same-Run wait, signed
  callback, idempotency, expiry/cancellation races, and next-loop resume. The
  Tool result schema, callback route, persistence, outbox worker, and runner
  resume behavior are not implemented.
- VCR replay, filesystem/database assertions, publication gates, trajectory
  export, and the durable evaluator worker. Evaluation Suite authoring and
  exact Run request persistence are implemented as a partial slice.
- Audit search, the remaining runner-pool operational controls, emergency
  controls, and the remaining integration management/usage capabilities.
- The target schemas, routes, state machines, and authorization matrix for
  Evaluations, Integrations, and Platform management are documented in
  [Admin Governed Resource Contracts](../architecture/admin-governed-resource-contracts.md),
  Integration management routes are now checked in for the partial slice;
  remaining target routes are not yet capabilities. The Platform provider and
  runner-pool routes are checked in only for the partial metadata slice. Policy
  routes are implemented only for the core Draft/Version/Binding slice above;
  capability-specific authorization and outer-policy composition remain.
- Production deployment, gVisor isolation, Helm, scaling, SLOs, retention
  deletion jobs, Legal Hold matching, Audit Event/Export Package integrity,
  backup/restore, SBOM, signing, and release gates.
- Control-plane target ownership, atomic command groups, idempotency, outbox/job
  semantics, fencing, object-store reconciliation, and restart recovery are
  documented in
  [Control-Plane Design Contract](../architecture/control-plane-design.md).
  Existing packages implement only part of this boundary; the document is not
  evidence of durable workers or production recovery.

### Deferred

- Multi-agent orchestration and visual DAG authoring.
- Shared/multi-owner event subscriptions and arbitrary trigger delegation.
  Owner-bound Webhook and scheduled Trigger contracts are designed, including
  cron, time-zone, misfire, Session targeting, and idempotency semantics, but
  neither Trigger execution path nor the Copilot `/triggers` management pages
  are implemented.
- SaaS multi-tenancy, SCIM, microVM runner pools, and a public marketplace.
- Arbitrary process-memory restoration and unrestricted interactive remote
  shell access.

## Roadmap Assessment

The implementation crosses phase boundaries because risk prototypes and narrow
governance loops were built early. This does not mean later phases are complete.

| Roadmap phase | Assessment | Evidence and remaining boundary |
| --- | --- | --- |
| Phase 0 | Partial | Repository, contracts, Compose, runner session, PTY, and core lifecycle spikes exist; isolation, egress, telemetry, load evidence, and signed artifacts remain. |
| Phase 1 | Partial | The core Copilot path, persistence, live events, artifacts, and runner loop exist; production model gateway, sandboxing, enterprise API, and full authorization remain. |
| Phase 2 | Partial | Admin agent lifecycle and review-gated publication exist; configuration catalogs, operations, audit, evaluations, integrations, and access management remain. |
| Phase 3 | Partial prototype | A requester-bound action approval loop with background expiry reconciliation exists; credential mediation, general tool execution, suspension, and audit integrity remain. |
| Phase 4 | Designed | Evaluation architecture exists only in documentation and scaffolding. |
| Phase 5 | Not started | No production-pilot hardening claim. |
| Phase 6 | Not started | No general-availability claim. |

## Evidence Rules

- Public HTTP capability is evidenced by checked-in OpenAPI plus an owning
  handler and tests.
- Runner protocol capability is evidenced by protobuf, both generated
  consumers, scheduler/executor behavior, and tests.
- A UI route without a working authorized backend flow is not an implemented
  product capability.
- A database table or package name alone is scaffolding, not completion.
- Compose smoke evidence and host unit/build evidence are reported separately.
- Release readiness requires verification on the exact release candidate and
  artifacts; a successful local build is insufficient.

Update this document whenever a capability changes status or a roadmap exit
gate gains evidence.
