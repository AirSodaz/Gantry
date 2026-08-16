# Implementation Status

This document separates the checked-in product from the target design. It is a
source-based snapshot of the current worktree on 2026-08-16. A capability is
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
- Workspace-scoped Admin agent lifecycle: create, edit with optimistic
  concurrency, validate, review, publish immutable versions, retire, and roll
  back.
- Admin configuration catalog slice: register immutable Skill artifacts,
  Plugin versions, and Tool descriptor versions; enable Plugins per workspace;
  bind exact catalog IDs to Agent Drafts; and revalidate availability at draft
  write and publication time.
- Copilot catalog, task submission, idempotency, task history, cancellation,
  retry, live task events, action approvals assigned to the requester, and
  artifact download.
- Persistent tasks, runs, ordered semantic events, bounded content segments,
  artifacts, durable action records, approval requests, and approval decisions.
- Runner V1 agent loop with deterministic and development provider adapters,
  native workspace tools, shell and PTY support, streamed rule interception,
  context compaction, encrypted checkpoints, cancellation, approval resume, and
  lease-fenced artifact upload.
- Local Compose and host smoke paths for runner, Copilot, Admin, approval, live
  event, and artifact lifecycle behavior.

### Partial

- **Identity:** Dex proves local OIDC and audience separation. Production IdP
  conventions, group mapping, lifecycle provisioning, and enterprise rollout
  guidance remain open.
- **Model routing:** Direct OpenAI-compatible and Anthropic adapters are allowed
  only for development. The production LLM gateway, credential mediation,
  budgets, route policy, and provider governance are not complete.
- **Authorization:** Workspace membership and initial Admin roles exist. The
  full resource-role model, group assignment, policy intersection, and
  revocation behavior remain incomplete.
- **Action approval:** One durable, digest-bound approval loop is implemented.
  Expiry processing, retained/durable suspension, revocation, rejection resume,
  and general tool-gateway execution remain incomplete.
- **Artifacts and streams:** Upload, storage, download, live events, and content
  segments exist. Production malware scanning, preview isolation, retention,
  compaction, backpressure evidence, and object-store failure handling remain.
- **Scheduling and isolation:** Runner registration, assignment, leases,
  cancellation, and runner-loss handling exist. Production runner pools,
  Kubernetes Jobs, gVisor, network enforcement, resource accounting, and
  cleanup controllers do not.
- **Admin configuration UX:** Skills, Plugins, and Tools can be registered,
  listed, explicitly activated/deprecated/retired, and an Agent Draft can
  select exact catalog references. Metadata detail, Agent usage, Plugin
  enablement visibility, and Tool descriptor schema display are available.
  Package upload materialization and content inspection, catalog
  search/filtering, Plugin contained-asset inspection, Tool discovery/health,
  descriptor schema authoring, and narrow binding constraints are not complete.
- **Admin lifecycle UX:** The core lifecycle is usable. Policies, evaluations,
  run operations, audit search, integrations, and platform administration are
  not complete product areas.

### Designed, Not Yet Implemented

- Agent-owned Prompt Snapshot compilation, package-content ingestion and
  inspection, Plugin asset expansion, Tool Server health/discovery, descriptor
  schema authoring/compatibility, narrow Tool Binding constraints, and CLI
  Command Profile catalogs described in
  [Agent Configuration, Skills, and Tools](agent-configuration-and-tooling.md).
- Hash-identified Agent Revisions with commit messages, multiple independent named
  Drafts, multiple test Deployments, and one default Production Deployment. The
  current implementation has one integer-revision Draft and a monotonic list of
  published versions instead.
- Per-Agent ACLs with independent metadata read, configuration read, Draft edit,
  Review, deployment, run-inspection, execution, and access-management
  capabilities. Current authorization is workspace-role based.
- Production credential broker, LLM gateway, tool gateway, egress gateway, and
  secret-store integration.
- Enterprise Agent Invocation API, registered integration clients, signed
  webhooks, and delegated-user authority.
- Evaluation suites, VCR replay, filesystem/database assertions, publication
  gates, and trajectory export.
- Admin run explorer, audit search, runner-pool operations, emergency controls,
  integration management, and policy administration.
- Production deployment, gVisor isolation, Helm, scaling, SLOs, retention jobs,
  hash-chained audit export, backup/restore, SBOM, signing, and release gates.

### Deferred

- Multi-agent orchestration and visual DAG authoring.
- General scheduled or event-triggered tasks.
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
| Phase 3 | Partial prototype | A requester-bound action approval loop exists; credential mediation, general tool execution, expiry processing, suspension, rejection resume, and audit integrity remain. |
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
