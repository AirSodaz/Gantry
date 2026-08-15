# Construction Roadmap

Roadmap phases describe dependency and exit gates, not a claim that work happens
strictly in one phase at a time. Narrow governance and Runner V1 slices were
built early to retire cross-process risk. Current evidence is tracked in
[Implementation Status](implementation-status.md).

## 1. Roadmap Strategy

Build Gantry through vertical slices. Each milestone must produce a usable flow
across identity, APIs, persistence, runner execution, events, and the relevant
frontend. Avoid completing all control-plane services before proving a real
employee task.

The roadmap separates platform foundations, the employee Copilot path, the
administrator design path, governance, and evaluation while preserving one
integrated release train.

Estimated duration assumes a core team of 8-10 engineers with product, design,
security, and infrastructure support. A smaller team should preserve milestone
order and reduce parallelism rather than remove verification gates.

## 2. Team Shape

Recommended core ownership:

| Area | Suggested ownership |
| --- | --- |
| Product and UX | 1 product lead, 1 product designer |
| Admin and Copilot web | 2 frontend engineers |
| Go control plane | 3 backend/platform engineers |
| Rust runner and sandbox | 2 systems engineers |
| Infrastructure and reliability | 1 platform/SRE engineer |
| Security and evaluation | 1 security/quality engineer, initially shared |

One technical lead owns contracts and cross-component decisions. Every module
still has a named operational owner.

### Current Assessment

| Phase | Status | Interpretation |
| --- | --- | --- |
| Phase 0 | Partial | Core repository, contract, Compose, runner-session, and lifecycle prototypes exist; isolation, egress, telemetry, load, and artifact-signing gates remain. |
| Phase 1 | Partial | Core Copilot, persistence, live events, artifacts, and Runner V1 exist; production gateway, sandbox, enterprise API, and complete authorization remain. |
| Phase 2 | Partial | Admin agent lifecycle exists; configuration catalogs, operations, audit, evaluations, integrations, and platform management remain. |
| Phase 3 | Partial prototype | One durable requester approval loop exists; credential mediation, general tool execution, thresholds, suspension, and audit integrity remain. |
| Phase 4 | Designed | Evaluation is documented but has no complete product slice. |
| Phase 5 | Not started | Pilot hardening has not begun. |
| Phase 6 | Not started | General-availability gates have not begun. |

No phase is complete until every exit-gate item has evidence on the relevant
commit and environment.

### Immediate Design Work Package

Before expanding frontend implementation, complete the page and function design
for both Admin and Copilot. This work consumes the platform documents and must
produce an approved sitemap, route/permission matrix, page specifications,
cross-page workflows, responsive requirements, and acceptance criteria. It does
not change the phase dependency rules below.

## 3. Phase 0: Inception and Risk Prototypes

**Target:** 2-3 weeks

### Objectives

- Establish repository, contracts, engineering standards, and development
  environment.
- Retire the highest-risk assumptions before product construction begins.

### Deliverables

- Monorepo layout and ownership rules, Moon project graph, pinned toolchain,
  deterministic bootstrap, and generated-contract freshness check.
- OpenAPI-first public API boundary; Connect/protobuf limited to the private
  runner gRPC listener; S3-compatible `ObjectStore` port with MinIO in Compose.
- Go, Rust, and TypeScript build, lint, unit-test, and dependency policies.
- Docker Compose environment with PostgreSQL, object storage, control plane,
  runner, Admin shell, and Copilot shell.
- Contract-generation pipeline for OpenAPI/JSON Schema and protobuf.
- OIDC development provider and separate Admin/Copilot clients.
- Contract spike for a confidential HR integration client, application token,
  delegated token exchange, asynchronous task, and signed webhook.
- OpenTelemetry trace from browser request through control plane to runner.
- Spike: outbound runner gRPC stream with heartbeat, assignment, event
  acknowledgement, and cancellation.
- Spike: lease-epoch fencing across LLM, tool, credential, and artifact gateways,
  including a partitioned stale runner.
- Spike: sustained model and PTY output using bounded segment batching, object
  storage, durable cursors, backpressure, and PostgreSQL write measurements.
- Spike: PTY lifecycle with descendant-process termination.
- Spike: gVisor Pod startup, resource limits, network deny, and cleanup.
- Threat-model workshop focused on credentials, tool calls, approvals, prompt
  injection, and runner compromise.
- Initial UI wireframes for both applications and usability review with one
  admin cohort and one employee cohort.

### Exit Gate

- CI runs on a clean machine and produces signed development artifacts.
- A task can be assigned to a runner, emit ordered events, and be canceled.
- A stale runner cannot call any effect-capable gateway after reassignment, even
  while its previous workload credential remains unexpired.
- Sustained stream load meets an explicit throughput target without unbounded
  memory, lifecycle-message starvation, or per-token PostgreSQL writes.
  Evidence includes rows and WAL bytes per output GiB, segment/object request
  rate, commit and reconnect latency, and behavior during slow object storage.
- A sandbox cannot reach an unapproved network destination.
- The team has measured sandbox startup and runner overhead rather than relying
  on binary-size or millisecond-start assumptions.
- Security and UX risks are captured as owned backlog items.

## 4. Phase 1: Governed Copilot Vertical Slice

**Target:** 4-6 weeks

### Objectives

Deliver the smallest employee-facing task flow with durable execution and no
administrative agent editor yet.

### Deliverables

- Organization, workspace, principal, role-binding, Agent Revision, Deployment,
  task, run, and event schemas.
- Seeded immutable Agent Revisions managed through migration fixtures or an
  internal bootstrap command.
- Copilot agent catalog, new-task composer, task view, history, cancellation,
  and artifact download.
- Task submission idempotency and immutable version binding.
- Scheduler, runner leases, sandbox provisioning, heartbeats, and cleanup.
- Rust runner agent loop with one model adapter, PTY, and at least one read-only
  built-in tool.
- LLM gateway with credential injection, streaming normalization, budgets, and
  usage recording.
- Cursor-based browser event stream with committed content segments,
  provisional offsets, reconnect, expired-cursor snapshot, and backpressure
  tests.
- Basic policy intersection for user, workspace, agent, tool, and destination.
- Enterprise Agent Invocation API with one registered HR client, application-
  identity task submission, polling, versioned result schema, idempotency, and
  signed terminal webhooks.
- Structured plans and rationale summaries; no raw chain-of-thought.
- Operational dashboard for queue, active runs, failures, and cleanup.

### Exit Gate

- An authorized employee can discover an agent, run a task, disconnect,
  reconnect, receive an artifact, and inspect history.
- An unauthorized employee cannot discover or access the agent or task.
- Duplicate task submissions do not create duplicate tasks.
- Control-plane restart does not lose accepted tasks.
- Runner loss reaches a clear failed state and triggers sandbox cleanup.
- Duplicate task submissions remain suppressed through terminal completion and
  the documented retry interval, including after task content deletion.
- End-to-end tests cover success, provider failure, cancellation, reconnect, and
  unauthorized access.
- An HR backend can invoke only its published HR agent, receive a validated
  result asynchronously, suppress duplicate submissions, and verify a signed
  webhook.

## 5. Phase 2: Gantry Admin Agent Lifecycle

**Target:** 5-7 weeks

### Objectives

Replace seeded configurations with a complete administrator workflow for agent
creation, review, publication, and operations.

### Deliverables

- Separate Admin application and API audience.
- Agent list, detail, draft designer, source view, validation inspector, version
  history, semantic diff, review, publication, rollback, deprecation, and
  retirement.
- Versioned schemas for model, tool, command, network, resource, approval, and
  evaluation policy references.
- Agent-owned Prompt settings with immutable publication snapshots; standalone
  external Skill package import and artifact catalogs with test coexistence;
  Plugin installation and workspace enablement; explicit Agent
  bindings; static validation, semantic diff, and publication pinning.
- MCP server registry, descriptor ingestion, namespace collision prevention,
  and schema pinning.
- Governed CLI Command Profiles with structured arguments and explicit command,
  filesystem, environment, image, effect, and idempotency policy.
- Admin run explorer with timeline, configuration references, artifacts,
  policy decisions, resource use, and authorized read-only Xterm.js view.
- Operator cancellation, safe retry, agent quarantine, and runner-pool drain.
- Audit search for configuration and operational actions.
- Catalog assignment to workspaces and employee groups.
- Integration client registry and explicit agent publication to enterprise
  systems, including contract, authority, event, artifact, and quota settings.

### Exit Gate

- An agent designer can create, validate, review, publish, use, roll back, and
  retire an agent without database or configuration-file access.
- Published versions remain immutable and every run proves its bound versions.
- Permission-broadening diffs are identified and routed to security review.
- Copilot cannot retrieve draft specs or call Admin endpoints.
- Operator terminal attachment is read-only by default and fully audited.
- Browser tests cover the full Admin publication flow and its Copilot result.
- Admin tests cover registering an HR client and publishing a compatible Agent
  API contract without exposing draft or internal configuration.

## 6. Phase 3: Approvals, Credentials, and Strong Governance

**Target:** 5-7 weeks

### Objectives

Enable write-capable enterprise agents with explicit action-time authorization,
credential mediation, and durable human approval.

### Deliverables

- Typed policy decision service returning allow, deny, or require approval.
- Tool action canonicalization, effect metadata, idempotency classification, and
  exact action digests.
- Durable action execution records, compare-and-swap revisions, lease fencing,
  and single-use execution permits.
- Approval queue and detail views in Admin and Copilot.
- Eligible approver rules, separation of duties, thresholds, expiry,
  supersession, revocation, and reason capture.
- Retained suspension with TTL and resource accounting.
- Durable checkpoint contract and resume in a new sandbox.
- Credential broker integrated with one enterprise secret store.
- Delegated user and platform credential modes for one representative
  integration.
- OAuth token exchange for delegated enterprise calls, with both application
  and subject principals preserved in policy and audit records.
- Trusted tool/egress gateway and enforced destination policy.
- Hash-chained run events, signed checkpoints, audit export, retention jobs, and
  legal-hold foundation.
- Emergency quarantine for agents, tools, providers, credentials, and runner
  pools.

### Exit Gate

- A write action cannot execute without current policy authorization and any
  required valid approval.
- Approval substitution, replay, expiry, revocation, and concurrent-decision
  tests fail closed.
- Approval versus cancellation, policy change, lease loss, durable resume, and
  execution-claim races consume at most one permit and use the documented
  rejection/expiry outcome mapping.
- Concurrent threshold votes produce one satisfaction transition; duplicate
  votes, late votes, and terminal-rejection rules are deterministic.
- Cancellation tests cover both sides of the execution linearization point and
  never report an in-flight unknown external effect as safely canceled.
- Runner filesystem, environment, terminal, and logs contain no durable
  enterprise or model credentials.
- An HR client cannot invent a delegated subject, invoke an unpublished agent,
  or broaden the Agent's tools, credentials, version, or destination policy.
- Default-deny egress and redirect tests block unauthorized destinations.
- Durable resume succeeds from a declared checkpoint and explicitly rejects
  unsupported arbitrary process restoration.
- Audit verification detects modified or removed run events.
- Stream compaction and retention preserve verifiable segment digests and
  hash-chain tombstones.

## 7. Phase 4: Evaluation and Golden Cases

**Target:** 5-7 weeks

### Objectives

Provide safe, reproducible regression testing and make evaluation evidence part
of publication.

### Deliverables

- Golden-case editor, versioning, suites, suite runs, and baseline comparison.
- Explicit production-trajectory export with classification, redaction,
  provenance, and review.
- HTTP/HTTPS VCR recording against approved non-production systems.
- Replay sidecar with fail-closed matching and synthetic write responses.
- Evaluation-aware tool, LLM, credential, and artifact gateway routing so
  mediated traffic outside the sandbox also uses the signed fixture manifest.
- Copy-on-write workspace fixtures and filesystem delta assertions.
- PostgreSQL fixture provisioning and logical database delta assertions.
- Output, event, tool-call, command, artifact, policy, latency, token, and cost
  assertions.
- Optional rubric scoring with evaluator identity and repeated-run reporting.
- Publication gates and audited override workflow.
- One end-to-end reference agent exercising read replay, intercepted write,
  shell execution, artifact creation, and database changes.

### Exit Gate

- Evaluation produces no external write and proves its network isolation.
- Unknown HTTP requests fail as fixture misses rather than reaching the network.
- Fixture misses from every mediated gateway fail before credential resolution,
  DNS, or target connection; tests prove that no production route is selected.
- Candidate and baseline comparisons use identical immutable suite manifests.
- A deliberate request, file, database, policy, and output regression is
  independently detected.
- A published agent can require the evaluation suite and block a failing
  candidate.
- Exported golden cases retain provenance and pass privacy review requirements.

## 8. Phase 5: Enterprise Pilot Hardening

**Target:** 4-6 weeks

### Objectives

Prepare for a limited production pilot with representative teams and agents.

### Deliverables

- Helm deployment, environment overlays, upgrade jobs, backup and restore, and
  disaster-recovery runbook.
- Horizontal control-plane scaling and tested scheduler concurrency.
- Quotas, rate limits, budgets, queue fairness, and backpressure.
- Capacity evidence for semantic-event rows, content-segment upload/index rate,
  object-storage latency, reconnect fan-out, and PostgreSQL saturation.
- High-cardinality telemetry controls and operational dashboards.
- Alerting and runbooks for queue stalls, runner loss, provider outage, event
  lag, credential failure, and sandbox cleanup failure.
- Retention and deletion verification.
- Accessibility audit and remediation for both applications.
- Load, endurance, fault-injection, and upgrade testing.
- SBOM, signed images and binaries, vulnerability response process, and release
  provenance.
- External penetration test and remediation.
- Administrator, approver, operator, and employee documentation.

### Exit Gate

- Backup restoration and version upgrade are rehearsed on production-scale test
  data.
- Defined pilot SLOs pass under expected concurrency with failure injection.
- No unresolved critical or high security findings exist.
- Cleanup, retention, and audit export work under partial infrastructure
  failures.
- Pilot users complete the primary Admin and Copilot workflows without engineer
  assistance at an agreed success rate.
- Operations and incident owners accept the runbooks and on-call responsibilities.

## 9. Phase 6: General Availability

**Target:** 3-5 weeks after pilot evidence

### Objectives

Resolve pilot findings and establish supported product operations.

### Deliverables

- Compatibility and upgrade policy.
- Supported Kubernetes, PostgreSQL, object-storage, browser, and identity-
  provider matrix.
- Release channels, rollback process, and security advisory process.
- Capacity planning guidance and default quotas.
- Product analytics with privacy controls.
- Formal service objectives and support escalation paths.
- Final data retention, legal hold, and audit evidence documentation.

### Exit Gate

- Pilot exit criteria and representative workload objectives remain satisfied
  on the exact release candidate commit and images.
- Release artifacts are reproducible, signed, scanned, and upgrade-tested.
- Known limitations and deferred capabilities are documented.
- Product, security, operations, and engineering owners approve release.

## 10. Post-GA Capability Tracks

These tracks are intentionally outside the first release and should be ordered
by customer evidence:

- Multi-agent workflows and visual DAG orchestration.
- Stronger microVM runner pools for selected risk classes.
- Additional database and RPC evaluation adapters.
- SCIM lifecycle provisioning.
- Policy-as-code integration if typed policy modules become insufficient.
- Cross-organization hosted control plane with proven isolation.
- Agent templates and curated internal marketplace.
- Scheduled and event-triggered tasks.
- Approved agent routing and recommendation in Copilot.
- Advanced cost allocation, forecasting, and provider optimization.

## 11. Continuous Verification Matrix

| Layer | Required verification |
| --- | --- |
| Contracts | Schema compatibility, generated-client checks, fuzz/property tests |
| Go control plane | Unit, transaction, authorization, migration, and integration tests |
| Rust runner | Unit, PTY lifecycle, cancellation, parser, resource-limit, and protocol tests |
| Frontends | Component, accessibility, permission-state, and browser workflow tests |
| Sandbox | Escape probes, egress tests, cleanup, TTL, and resource exhaustion tests |
| Credentials | Secret-leak scans, audience, expiry, revocation, and delegated-auth tests |
| Events | Ordering, deduplication, segment batching, offset replay, cursor expiry, backpressure, projection rebuild, and tamper detection |
| Actions and approvals | CAS transitions, lease fencing, single-use permits, cancellation and expiry races, unknown-outcome reconciliation |
| Evaluation | Fixture isolation, fail-closed VCR, delta correctness, and contamination tests |
| Release | SBOM, signing, vulnerability scans, upgrade, rollback, backup restore |

## 12. Milestone Dependency Rules

- Do not build multi-agent orchestration before single-agent lifecycle,
  authorization, cancellation, and evaluation are reliable.
- Do not enable production write tools before credential mediation and approvals
  pass adversarial tests.
- Do not export production trajectories before classification, redaction,
  retention, and review workflows exist.
- Do not claim durable suspension before a supported checkpoint contract is
  demonstrated end to end.
- Do not claim formal release readiness from packaging or CI alone; all release
  gates must be verified on the final release candidate artifacts.

## 13. Initial Backlog Order

The first engineering backlog should begin in this order:

1. Contract schemas and resource identifiers.
2. Development environment and observability spine.
3. Runner session, lease, ordered events, cancellation, and cleanup.
4. Identity audiences and resource authorization skeleton.
5. Immutable Agent Revision bootstrap and task persistence.
6. LLM gateway and one read-only tool.
7. Copilot catalog and end-to-end task view.
8. Admin agent lifecycle.
9. Action-time policy, credentials, and approvals.
10. Evaluation vertical slice.

This order proves the hardest cross-process lifecycle early and avoids building
an extensive editor around an unverified execution model.
