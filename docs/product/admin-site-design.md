# Gantry Admin Site Design

## 1. Scope and Status

This document defines the complete target page and function design for Gantry
Admin. It is implementation-ready product structure, not a statement that every
route exists today. Each page is labeled by delivery status:

- **Current:** a working product slice exists in the repository.
- **Next:** required for the next Admin configuration slice.
- **Later:** target product behavior after the next slice.

“Later” describes delivery order only. A page remains in scope when its
executable API, persistence, authorization, and verification contracts are
available; a target without those contracts is reported as a delivery gap
rather than implemented by inference.

The existing visual language and shared design-system components remain the
foundation. Detailed visual mockups and frontend implementation follow this
page/function specification.

## 2. Application Shell

### Primary Navigation

The left sidebar uses non-clickable group labels and direct page links. It does
not add empty section landing pages.

| Group | Page | Target route | Scope | Delivery |
| --- | --- | --- | --- | --- |
| Home | Overview | `/` | Current workspace or all workspaces | Current |
| Build | Agents | `/agents` | Workspace | Current |
| Build | Skills | `/skills` | Workspace | Current |
| Build | Plugins | `/plugins` | Organization catalog with workspace enablement | Current |
| Build | Tools | `/tools` | Organization inventory with workspace availability | Current |
| Operate | Runs | `/runs` | Workspace or all workspaces | Later |
| Operate | Evaluations | `/evaluations` | Workspace | Current (suite core) |
| Govern | Integrations | `/integrations` | Organization | Current (management slice) |
| Govern | Policies | `/policies` | Organization and workspace | Current (core lifecycle) |
| Govern | Audit | `/audit` | Authorized organization/workspace scope | Later |
| Platform | Runners | `/platform` | Organization | Current (pool metadata slice) |
| Platform | Model Providers | `/platform` | Organization | Current (provider metadata slice) |
| Platform | Credentials | `/platform/credentials` | Organization | Later |
| Platform | Settings | `/platform/settings` | Organization | Later |

Prompts are configured inside an Agent. There is no standalone Prompts page.

### Workspace Context

The workspace selector is persistent and visible near the top of the sidebar.
Its value is preserved across navigation and browser reloads.

- Workspace-scoped pages use the selected workspace automatically.
- Organization administrators may select `All workspaces` on pages that support
  aggregation.
- Organization-only pages ignore the workspace filter and display a visible
  `Organization` scope label.
- A page never silently changes scope. If the current selection is invalid for
  the destination, the page explains the applied organization scope.
- Deep links include enough route or query context to reproduce the intended
  scope without relying only on local browser state.

### Global Shell Functions

- Collapse or expand the sidebar without changing the current page.
- Switch workspace scope when authorized.
- Display current organization, identity, role summary, and environment.
- Switch theme using the shared design-system control.
- Open profile/session actions and sign out.
- Show a persistent environment marker outside production.
- Surface global incidents or emergency quarantine through a restrained status
  band that links to the owning operational page.

### Page Header Contract

Every page header contains:

- breadcrumb only when the page is nested below a resource;
- literal page or resource name;
- concise state or scope metadata;
- one primary command at most;
- secondary commands in a menu or adjacent compact command group;
- no explanatory marketing copy.

Destructive, irreversible, security-sensitive, or authority-broadening commands
require an explicit confirmation surface with the affected scope and result.

## 3. Roles and Page Visibility

| Role | Primary page access |
| --- | --- |
| Organization Administrator | All pages and all-workspace aggregation; installation, platform, identity, and emergency controls |
| Workspace Agent Editor | Agents, Skills, enabled Plugins, available Tools, workspace Runs and Evaluations; explicit self-view of Agent Sessions as viewer |
| Security Reviewer | Agent reviews, Plugin/Tool risk, Policies, Audit, evaluation evidence, and scoped Audit export |
| Operator | Overview, Runs, runtime artifacts, Runners, provider health, and read-only operational Audit |
| Auditor | Read-only Overview, Runs, Integrations, Policies, Audit, and scoped Audit export within assigned scope |

Navigation hides pages the actor cannot use. Direct navigation still performs
server authorization and returns a non-leaking denied state.

## 4. Complete Page Inventory

### Overview

| Page | Route | Responsibility | Delivery |
| --- | --- | --- | --- |
| Overview | `/` | Scope-aware operational and governance summary | Current |

### Build

| Page | Route | Responsibility | Delivery |
| --- | --- | --- | --- |
| Agent list | `/agents` | Search, filter, create, and inspect workspace Agents | Current |
| New Agent | `/agents/new` | Create Agent identity and initial draft | Current |
| Agent resource | `/agents/:agentId/*` | Own the Agent overview, design, versions, usage, access, and history | Current, expanding next |
| Agent review | `/agents/:agentId/review/:reviewId` | Review semantic changes and record a decision; current slice may embed this in Agent Design | Current, expanding next |
| Agent revision | `/agents/:agentId/revisions/:revisionHash` | Inspect one immutable revision and its review, test, and deployment evidence | Current |
| Skill list | `/skills` | Browse and filter standalone workspace Skills | Current |
| Import Skill | `/skills/import` | Import a package from a marketplace, direct locator, or manual upload and validate its declared metadata | Next |
| Skill resource | `/skills/:assetId` | Inspect imported artifacts, declared versions, source provenance, activation state, and Agent usage | Current |
| Skill artifact | `/skills/:skillId/artifacts/:artifactId` | Inspect one imported package artifact, content digest, requirements, and usage | Next |
| Plugin catalog | `/plugins` | Browse installed/available Plugins and workspace enablement | Current |
| Install Plugin | `/plugins/install` | Review source, contents, permissions, compatibility, and install | Next |
| Plugin resource | `/plugins/:assetId` | Inspect versions, contained assets, workspaces, health, and audit | Current |
| Plugin version | `/plugins/:pluginId/versions/:versionId` | Review immutable package contents and permission changes | Next |
| Tool inventory | `/tools` | Browse built-in, MCP, and CLI descriptors across providers | Current |
| Tool Server | `/tools/servers/:serverId/*` | Configure connection metadata, discovery, health, and descriptors | Next |
| Tool Descriptor | `/tools/:assetId` | Inspect schemas, effects, risk, compatibility, and Agent usage | Current |
| CLI Command Profile | `/tools/cli-profiles/:profileId` | Define and inspect governed structured command execution | Next |

### Operate

| Page | Route | Responsibility | Delivery |
| --- | --- | --- | --- |
| Run list | `/runs` | Cross-actor operational search, diagnosis, comparison, and authorized Run operations | Later |
| Run detail | `/runs/:runId` | Timeline, output, tools, approvals, artifacts, configuration, and diagnostics | Later |
| Evaluation suite list | `/evaluations` | Browse suites, coverage, regressions, and publication gates | Later |
| Evaluation suite | `/evaluations/suites/:suiteId/*` | Author cases, versions, assertions, and gate policy | Later |
| Evaluation run | `/evaluations/runs/:runId` | Compare candidate/baseline results and inspect failures | Later |

### Govern

| Page | Route | Responsibility | Delivery |
| --- | --- | --- | --- |
| Integration list | `/integrations` | Manage registered enterprise clients and publications | Later |
| Integration resource | `/integrations/:integrationId/*` | Clients, agent contracts, webhooks, quotas, credentials, and audit | Later |
| Agent publication | `/integrations/:integrationId/publications/:publicationId` | Inspect one exact Agent Revision contract exposed to the Integration | Later |
| Webhook endpoint | `/integrations/:integrationId/webhooks/:webhookId` | Configure endpoint metadata and inspect signed delivery history | Later |
| Policy list | `/policies` | Browse policy sets by type, scope, state, and usage | Later |
| Policy resource | `/policies/:policyId/*` | Edit drafts, review versions, simulate, publish, and inspect bindings | Later |
| Audit explorer | `/audit` | Search attributable configuration and runtime records | Later |
| Audit event detail | `/audit/events/:eventId` | Inspect one immutable event and linked evidence | Later |

### Platform

| Page | Route | Responsibility | Delivery |
| --- | --- | --- | --- |
| Runner pools | `/platform/runners` | Capacity, health, drain, quarantine, compatibility, and incidents | Later |
| Runner pool detail | `/platform/runners/:poolId` | Pool configuration, active runners, workload, versions, and audit | Later |
| Model Providers | `/platform/model-providers` | Provider routes, models, health, budgets, data policy, and failover | Later |
| Model Provider detail | `/platform/model-providers/:providerId` | Route configuration, allowed models, usage, incidents, and audit | Later |
| Credentials | `/platform/credentials` | Credential references, modes, owners, expiry, rotation, and usage | Later |
| Credential detail | `/platform/credentials/:credentialId` | Metadata and policy without revealing secret values | Later |
| Settings | `/platform/settings` | Scope-aware organization defaults, Workspace overrides, retention, Legal Holds, classifications, limits, and environments | Later |

## 5. Overview Page Specification

### Purpose

Overview is the default Admin landing page. It answers what needs attention in
the selected scope and links directly to the owning working view. It is not a
welcome page and does not duplicate complete operational tables.

### Audience and Scope

- Route: `/`
- Default scope: selected workspace.
- Organization administrators may select `All workspaces` for aggregate data.
- Organization-only platform signals retain an `Organization` label even when a
  workspace is selected.

### Primary Functions

1. Show an attention queue ordered by urgency: incidents, blocked runs, runs
   awaiting requester action, failed publication gates, quarantined assets, expiring
   credentials, and unhealthy providers or runner pools.
2. Summarize active runs, queue age, failure rate, requester wait, artifact
   failures, and evaluation regressions for the selected scope.
3. Show recent Agent publications and permission-broadening reviews.
4. Show runtime capacity and provider health as organization-scoped signals.
5. Link every value to a filtered owning page; metrics are not dead display
   cards.

### Layout

- Compact status band for active incidents or emergency quarantine.
- Attention list occupies the first working region.
- A stable metric row supports comparison without oversized cards.
- Active work and recent governance changes use dense tables or lists.
- Capacity/provider health is a lower organization-scoped band.
- The first viewport exposes attention items and active work at desktop sizes.

### Filters and Commands

- Persistent workspace selector from the shell.
- Time range: `24 hours`, `7 days`, `30 days`.
- Optional Agent and status filters where aggregate data is large.
- Refresh command and last-updated indicator.
- No destructive command executes directly from Overview. Linked pages own
  cancellation, quarantine, approval, or policy changes.

### States

- Loading uses stable row and metric skeletons.
- Empty state says that no attention is required and still shows recent work.
- Partial failure identifies the unavailable signal without blanking other
  regions.
- Permission-limited actors see only allowed metrics and no indication of
  hidden resource counts.
- Disconnected realtime state preserves the last snapshot and displays its age.

### Acceptance Criteria

- Scope changes update every workspace-aware region consistently.
- `All workspaces` is unavailable to non-organization administrators.
- Organization-scoped signals are visibly labeled and never imply workspace
  ownership.
- Every attention item and metric has an authorized filtered destination.
- The page remains useful when one metrics source or realtime stream fails.
- Keyboard and screen-reader order follows urgency, then current work, then
  historical summaries.

## 6. Agent Page Specifications

### Agent Configuration Model

The Agent workspace separates configuration history from mutable editing and
deployment state:

| Concept | Responsibility |
| --- | --- |
| Agent | Stable workspace-owned identity, catalog metadata, ownership, and lifecycle |
| Draft | Named mutable editing space with one working copy and its latest committed Revision |
| Revision | Immutable configuration snapshot identified by a hash and required message |
| Deployment | Named pointer from a test or production environment to one exact Revision |
| Review | Policy-driven decision bound to one Revision, its content digest, and its comparison base |

### Agent-Scoped Access Control

Every Agent has an explicit ACL. Organization and workspace roles determine
whether an actor may manage the ACL, but they do not silently grant access to
every Agent in the workspace. ACL subjects may be principals, groups, or
registered service identities.

The ACL exposes independent capabilities:

| Capability | Allows | Does not imply |
| --- | --- | --- |
| `metadata.read` | See Agent name, owner, lifecycle, and safe catalog metadata | Prompt/configuration read or execution |
| `configuration.read` | See Draft configuration, immutable Revisions, Prompt Snapshots, Skills, Plugins, Tools, and policies allowed by redaction rules | Edit, review decision, publish, or execute |
| `draft.edit` | Create, rename, archive, and edit the Agent's Drafts; commit Revisions | Review approval, Production publish, or execution |
| `review.decide` | Inspect the bound diff/evidence and approve or reject a Review | Edit the Draft or publish without policy authorization |
| `deployment.test` | Create, update, or stop named test Deployments using approved test policies | Production publish or production credentials |
| `deployment.production` | Move the default Production pointer or rollback to an approved Revision | Review decision or ACL management |
| `runs.read` | Inspect runs, artifacts, and operational evidence within allowed redaction | Configuration read or execution |
| `execute` | Discover and invoke the Agent through an authorized Copilot, integration, or owner-managed webhook surface | Configuration read, Draft access, or unrestricted tool authority |
| `access.manage` | Grant, remove, and review Agent ACL entries | Bypass of organization/workspace authorization or policy |

`configuration.read` is intentionally separate from `metadata.read`, and
`execute` is intentionally separate from both. A service identity may execute
an Agent without receiving its Prompt or internal Tool Bindings. A reviewer may
read one Revision and decide its Review without editing the Draft. Preset role
names are convenience bundles over these capabilities; the ACL remains the
source of truth.

The effective decision is default-deny and evaluates in this order:

1. Organization and workspace membership, role, environment, and session
   strength.
2. Agent ACL capability for the subject.
3. Resource state: Draft, Revision, Deployment, Review, quarantine, retirement,
   and expiry.
4. Policy intersection for the requested action, including credentials,
   destinations, model route, approval, and data classification.
5. For `execute`, an active Deployment and the caller's Copilot or integration
   publication.

The initial ACL model is Allow-only. No user-configurable Deny grant exists:
absence of an applicable capability means denied, and revocation removes the
Allow grant. If an Allow is present but an outer membership, Deployment,
integration, quarantine, or Policy check blocks the action, the UI shows
`Granted, blocked by <constraint>` rather than creating a competing Deny
record. Emergency execution stops use Agent or Deployment quarantine.

Removing `execute` prevents new tasks without revoking historical run evidence.
Removing `configuration.read` redacts raw configuration from the Admin response
even when the actor can still see safe metadata or run status. Every ACL change,
denied access, and emergency revocation is audited.

Every Agent starts with a `Main draft`. Editors may create multiple named Drafts
from the content of any Revision for experiments, debugging, or parallel
changes. Drafts are persisted governed resources rather than private browser
state. Draft read and edit access inherit the owning Agent's
`configuration.read` and `draft.edit` grants; Drafts do not introduce a
second private-sharing ACL.

Drafts and Revisions form a flat history, not a branch tree. There are no parent
Revision, fork, merge, rebase, cherry-pick, or ancestry semantics. A Draft may
record an optional `derived from` Revision for provenance, but this does not
create a graph or constrain later comparisons. Editors can compare any two
Revisions and deliberately copy or recreate selected changes.

### Revision Identity and Integrity

- Committing a Draft creates an immutable Revision with a full cryptographic
  revision hash, a required human-authored message, author, and timestamp.
- The UI displays the shortest unique hash prefix, with at least seven
  characters, and provides the full hash for copying and audit use.
- A separate content digest covers the canonical compiled configuration. The
  revision hash identifies the immutable snapshot and commit metadata; the content digest
  identifies executable behavior.
- Two Revisions may have different hashes but the same content digest when their
  author, timestamp, or message differs without changing behavior.
- Reviews, evaluations, runs, and deployments always bind the full Revision and
  content digest. A short hash is presentation only.
- Revision messages summarize intent and behavior. Release notes belong to the
  review and production-deployment workflow rather than replacing commit
  messages.

### Draft and Deployment Rules

- Static validation may run against the mutable working copy, but a test run or
  evaluation suite always executes a committed Revision.
- A Draft records its optional source Revision, latest Revision, working-copy
  ETag, owner, collaborators, validation state, and last activity.
- Creating a Draft from a Revision does not copy review approval or deployment
  state.
- Multiple named test Deployments may coexist and point to different Revisions,
  such as a personal sandbox, shared staging environment, or focused debugging
  slot.
- One Agent has one default `Production` Deployment in its workspace. Moving
  this pointer is the production publication or rollback operation.
- Integration publications may pin a different exact Revision for a registered
  client without changing the workspace default Production Deployment.
- Test Deployments use designated development or test credentials and policies.
  They cannot acquire production authority merely because their Revision later
  becomes production-approved.
- Rollback moves the Production pointer to an earlier approved Revision and
  records a new deployment event. It never creates, mutates, or deletes a
  Revision.

### Agent Resource Workspace

The Agent resource uses stable nested routes and the following tabs:

| Tab | Route | Purpose | Delivery |
| --- | --- | --- | --- |
| Overview | `/agents/:agentId` | Current state, ownership, production/test deployments, attention items, and recent activity | Current |
| Design | `/agents/:agentId/design` | Edit one named Draft, validate it, commit Revisions, and start test/review workflows | Current, expanding next |
| Versions | `/agents/:agentId/versions` | Browse Draft latest Revisions, immutable snapshots, test Deployments, and Production history | Current |
| Runs | `/agents/:agentId/runs` | Filter runtime attempts for this Agent and exact Revisions | Later |
| Evaluations | `/agents/:agentId/evaluations` | Compare suite results and publication-gate evidence by Revision | Later |
| Access | `/agents/:agentId/access` | Owners, editors, reviewers, publishers, consumers, and integration visibility | Later |
| Recent activity | `/audit?resource_type=agent&resource_id=:agentId` | Recent immutable activity with a link to the global Audit explorer | Later |

`Review` and `Publish` are not permanent tabs. They are Revision-bound
workflows launched from Design, Versions, or a Revision detail and use
`/agents/:agentId/review/:reviewId`. This route remains independently
authorizable because reviewers may not have Agent editing permission.

### Agent List

- Route: `/agents`.
- Default scope: selected workspace; organization administrators may use
  `All workspaces`.
- Primary command: `New Agent`.
- Search matches display name, slug, description, owner, and Revision hash.
- Filters cover lifecycle, owner, validation state, production state, test
  deployment, review state, and recent activity.
- Default columns are Agent, owner, lifecycle, Production Revision, active Draft
  latest Revision, validation/review status, recent runs, and last changed time.
- A missing Production Deployment is distinct from retired, quarantined, or
  invalid configuration.
- Row selection opens Agent Overview. Opening a specific Draft or Revision uses
  an explicit secondary action so the destination is predictable.

Empty state creates an Agent from a blank configuration or an authorized
template. Permission-limited users receive a read-only list without creation or
hidden-scope counts.

### New Agent

- Route: `/agents/new`.
- Required fields: workspace, display name, unique slug, owner, category, and
  concise employee-facing description.
- Starting point: blank configuration, authorized template, or an existing
  Revision the actor may reuse.
- Creation produces the Agent identity and `Main draft`; it does not create a
  Revision, Review, test Deployment, or Production Deployment implicitly.
- Slug availability is validated before submission and rechecked atomically on
  creation.
- After creation, the user lands in the Main Draft Design route with a visible
  uncommitted state.

### Agent Header and Overview

The persistent resource header shows display name, lifecycle, workspace, owner,
Production Revision short hash, and quarantine state. The primary command is
contextual: `Open Main draft`, `Resolve issue`, or `View production`, never a
direct unconditional publish action.

Agent Overview contains:

- Production Deployment and last deployment result;
- active test Deployments with owner, Revision, purpose, and expiry;
- active Drafts with latest Revision, working-copy state, validation, and owner;
- pending or failed review/evaluation gates;
- recent runs split by Production and test context;
- recent configuration and deployment activity;
- linked owner and access summary.

Overview does not duplicate the full editor, Revision history, or run table.

### Design

Design opens one named Draft. `/agents/:agentId/design` redirects to the actor's
last used accessible Draft or `Main draft`; the concrete Draft route is retained
in the URL for sharing and recovery.

The page provides:

- Draft selector with create, rename, archive, and create-from-Revision commands;
- optional source and latest Revision hashes, working-copy state, author, and
  last activity;
- form-led sections for identity and contract, Agent Prompt, Skills, Plugin
  assets, Tool Bindings, model policy, runtime and filesystem, network and
  credentials, approvals, artifacts, and evaluation gates;
- effective instruction order and effective-authority preview;
- structured validation findings linked to the owning section;
- semantic comparison against the Draft's latest or source Revision, Production,
  or another selected Revision;
- `Commit Revision` as the primary persistence command when the working copy has
  changes;
- `Run test`, `Run evaluation`, and `Submit for review` only after a valid
  Revision exists.

`Commit Revision` requires a message and shows summarized configuration
changes. Committing clears the working-copy dirty state and updates only that
Draft's latest Revision reference. Saving browser form state is not presented as
a version.

Concurrent edits use ETag conflict handling. On conflict, the page preserves the
actor's unsaved input and offers comparison against the newer working copy; it
does not overwrite automatically.

### Versions

Versions is an operational history workspace, not a list of integer release
numbers. Its first region shows:

- the one default Production Deployment;
- all active test Deployments;
- active Drafts and their latest Revisions;
- pending review and evaluation candidates.

The Revision table contains:

- short hash and commit message;
- source Draft and optional `derived from` Revision provenance;
- author and committed time;
- validation, review, and evaluation status;
- Production, test, integration-publication, deprecated, or quarantined labels;
- content digest and configuration-schema version;
- run count and most recent execution result.

Search matches hash, message, author, and referenced asset names. Filters cover
Draft, deployment, review, evaluation, author, risk, and time. The default order
is newest committed time. A Draft or Deployment filter produces a simple
chronological history without implying ancestry.

Available commands depend on permission and state: compare, create Draft from
Revision, run test, deploy to test, submit for review, publish to Production,
rollback Production, quarantine, deprecate, and copy full hash.

### Revision Detail

- Route: `/agents/:agentId/revisions/:revisionHash`.
- Header: short hash, commit message, author, time, source Draft, optional
  provenance Revision, validation, review, evaluation, and deployment labels.
- Configuration view: immutable effective Prompt Snapshot, selected Skill
  package identities with declared versions and content digests, selected Plugin
  assets, Tool Bindings, policies, contracts, artifacts, runtime requirements,
  compiler version, and content digest.
- Compare view: semantic diff against Production, the Draft's optional source or
  latest Revision, or any other selected Revision.
- Evidence view: reviews, evaluation results, test runs, production runs,
  deployment events, acknowledgements, and audit records.
- The page never offers inline editing. `Create Draft from Revision` is the edit
  path.

### Review and Publication

A review submission selects one valid Revision and records release notes,
comparison base, semantic diff, risk classification, required reviewer policy,
evaluation evidence, and requested deployment target.

- Review approval binds the full Revision hash and content digest.
- Editing or committing a Draft after submission does not mutate the submitted
  Revision or its Review.
- Publishing a different Revision requires a different approved Review.
- If the Production pointer changes before decision or publication, a Review
  whose risk assessment used the old Production base becomes superseded and
  must be regenerated.
- Reviewer sets and thresholds are policy-driven. The UI does not assume one
  reviewer or permit an editor to bypass separation-of-duty policy.
- Approval does not deploy automatically unless an explicit policy-owned
  auto-deploy rule exists. Otherwise an authorized publisher executes the
  Production deployment command.
- Publication rechecks Revision integrity, asset availability, current policy,
  evaluation gates, review validity, and Production base immediately before the
  pointer moves.
- A failed publication leaves Production unchanged and exposes the exact failed
  gate, correlation ID, and retry conditions.

The review route presents the semantic diff before raw configuration, groups
changes by behavior and authority, highlights permission broadening, and links
to exact evaluation and test evidence. Approve and reject require a recorded
reason when policy or risk level requires it.

### Runs and Evaluations

- Runs defaults to all Revisions and can isolate Production, a test Deployment,
  a Draft's latest Revision, or one exact Revision. Retry always states whether
  it reuses the original Revision or targets the current Production Revision.
- Evaluations shows required gates, candidate-versus-baseline comparisons, suite
  history, regressions, and evidence attached to Reviews.

### Access

- Route: `/agents/:agentId/access`.
- Required capability: `access.manage` for mutation; authorized configuration
  readers may receive a read-only effective-access view.
- Primary command: `Add access`.

The typed resource, Admin route contract, persistence invariants, and
owner-transfer behavior are defined in
[Agent Access Contracts](../architecture/agent-access-contracts.md). The page
does not expose a separate Deny model or a second ACL history; Recent activity
links to the canonical Audit explorer.

The page uses role presets only to populate an explicit capability matrix:

| Preset | Initial capabilities | Intended use |
| --- | --- | --- |
| Configuration Viewer | `metadata.read`, `configuration.read` | Inspect Prompt, Skills, Tools, policies, and Revisions without mutation |
| Editor | `metadata.read`, `configuration.read`, `draft.edit` | Author Drafts and commit Revisions |
| Reviewer | `metadata.read`, `configuration.read`, `review.decide`, `runs.read` | Decide Reviews using configuration and evidence |
| Test Operator | `metadata.read`, `deployment.test`, `runs.read` | Manage test Deployments and inspect results |
| Publisher | `metadata.read`, `configuration.read`, `deployment.production`, `runs.read` | Publish or roll back approved Production Revisions |
| Executor | `metadata.read`, `execute` | Discover and invoke the deployed Agent without internal configuration access |
| Access Manager | `metadata.read`, `access.manage` | Administer this Agent's ACL without implied edit or execution |

Selecting a preset checks its capabilities in the same visible matrix used for
custom access. Changing any checkbox marks the assignment `Custom`. The saved
authority is the explicit capability set, not the preset name. Changing a preset
definition later never alters existing assignments.

The first region summarizes:

- people, groups, and service identities with any Agent access;
- configuration readers, editors, reviewers, publishers, and executors;
- active and expiring grants;
- Copilot visibility and registered integration execution exposure;
- warnings for missing recovery access, stale identities, or blocked effective
  permissions.

The assignment table shows subject, subject type, preset or `Custom`,
expanded capabilities, grant source, issuer, effective interval, last changed
time, and effective state. Rows reveal why a checked capability is ineffective,
such as missing workspace membership, insufficient session strength, inactive
Deployment, revoked integration publication, quarantine, or policy denial.

`Add access` and `Edit access` use a routed dialog or modal containing:

1. Searchable principal, group, or registered service-identity selector.
2. Preset selector.
3. Expanded capability checkboxes grouped as `View`, `Build`, `Govern`,
   `Operate`, and `Use`.
4. Optional effective start and expiry.
5. Required reason for privileged or execution-bearing access.
6. Effective-permission preview after organization/workspace and current policy
   intersection.
7. Before/after diff and confirmation.

Adding `execute` previews the exact available surfaces: employee Copilot,
specific integration publications, or neither. Adding
`deployment.production`, `access.manage`, or broader configuration access
shows a high-impact confirmation. Access mutations are never optimistic.

The page prevents removing the final organization-authorized recovery path,
requires explicit confirmation before removing the actor's own
`access.manage` capability, and provides an owner-transfer or audited
break-glass route instead of leaving the Agent unmanaged.

Access acceptance criteria:

- Every stored assignment exposes its exact capabilities without opening a role
  definition elsewhere.
- Preset selection and manual checkbox selection produce the same authorization
  data.
- Preset updates cannot mutate existing Agent ACLs.
- Effective previews explain organization/workspace, Agent, Deployment,
  integration, and policy restrictions separately.
- The initial page has no user-configurable Deny state; blocked states explain
  the outer constraint that caused the denial.
- An Executor cannot retrieve internal configuration unless separately granted
  `configuration.read`.
- A Configuration Viewer cannot execute unless separately granted `execute`.
- Direct URLs and API commands enforce the same effective capability result as
  the page.

### Recent Activity

The Agent Overview and Versions views show a compact Recent activity slice for
Draft creation/archive, Revision commits, validation, comparison, review
decisions, test/Production deployment changes, rollback, quarantine, access
changes, effective-access denials, and denied consequential commands. The slice
links to `/audit?resource_type=agent&resource_id=:agentId`; the canonical Audit
explorer owns full filtering, event detail, and export.

### Agent Acceptance Criteria

- Every run, evaluation, review, and deployment identifies one immutable
  Revision and full content digest.
- Multiple Drafts and test Deployments can coexist without changing Production.
- Production has one unambiguous default pointer per workspace-owned Agent.
- No mutable working copy can execute, enter review, or be deployed without
  first becoming a valid Revision.
- Revision history remains intact across Draft archive, rollback, deprecation,
  retirement, and quarantine.
- Review and publication permissions are independently enforced on direct
  routes and commands.
- Agent metadata read, configuration read, Draft edit, and execution are
  independently grantable and independently revocable.
- Current implementation limitations remain labeled `Current`; target Draft,
  Revision, and Deployment behavior is not presented as already shipped.

## 7. Skill Page Specifications

### Skill Catalog

The Skill catalog is an import and inspection surface, not a second package
registry. Each row shows the Skill package identity, source marketplace or
locator, package-declared version, artifact digest, validation state, catalog
status, number of Agent bindings, and last import time. The catalog may contain
several artifacts for the same Skill so test Agents can compare package
versions.

### Import Skill

Import accepts a supported marketplace/package locator, a complete manual
package upload, or a local directory selection. The flow displays the package
manifest, declared version, provenance,
required Tool Bindings, content digest, dependencies, and validation findings
before activation. A manual upload records the uploader and uploaded package
digest as its source reference. Gantry does not provide fields for incrementing
a version, writing a release message, or moving a Skill release pointer.
Re-importing a package creates another catalog artifact when its source or
digest differs. The page never provides an inline Skill editor; changes require
importing another complete package or directory.

### Skill Resource and Artifact Detail

The Skill resource groups imported artifacts, Agent usage, and audit history.
An artifact detail route shows the exact source reference, package-declared
version, digest, normalized instructions and contracts, requirements,
validation, activation or quarantine state, and the Agent Revisions that pin it.
Artifact actions are limited to validate, activate, deprecate, quarantine,
retire, and import another artifact. The page never edits package content.

Agent designers select an exact imported artifact when binding a Skill. A test
Deployment may bind a different artifact of the same Skill, but no binding
floats automatically when a new package is imported. Historical Agent
Revisions and run manifests retain the selected source and digest.

### Skill Acceptance Criteria

- The declared package version is displayed as read-only source metadata; when
  absent, the UI shows `未声明` and does not generate a version.
- Import supports marketplace, direct-locator, complete-package upload, and
  local-directory sources, with source provenance visible on every artifact.
- Multiple artifacts of one Skill can coexist and be selected independently by
  test Agent Revisions.
- Artifact identity always includes source reference and content digest; duplicate
  declared versions with different content are visibly distinct.
- Import, activation, deprecation, quarantine, and retirement are auditable.
- No Skill page exposes Gantry-owned version editing, commit messages, release
  pointers, or a Skill merge/history graph.
- Manual import never exposes inline content editing; a changed Skill is a new
  imported artifact.
- Agent bindings and direct API calls enforce the same exact-artifact behavior.

## 8. Plugin Page Specifications

### Plugin Catalog

The Plugin catalog is organization-scoped and shows installed packages, source
provenance, publisher, available versions, review status, compatibility, risk,
and workspace enablement counts. Workspace context filters the enablement view
but never hides organization-level installation state.

### Install Plugin

The install flow reviews one immutable Plugin Version before installation. It
shows publisher and provenance, contained Skills, Tool Servers and Tool
Descriptors, dependencies, configuration schemas, permission or effect
changes, compatibility requirements, risk findings, and content digest.
Installation is an organization action and does not enable the Plugin for any
workspace or bind it to an Agent.

### Plugin Resource

The Plugin resource uses stable sections for Overview, Versions, Workspace
Enablement, Assets, and Health. Versions are read-only immutable package
records. A version detail shows its manifest, contained assets, dependency and
compatibility results, digest, review evidence, and Agent usage; it never edits
package contents. Overview includes Recent activity and links to the global
Audit explorer with the Plugin pre-filtered.

### Workspace Enablement

Workspace Enablement lists every exact Plugin Version available in the selected
workspace. Administrators may enable or disable versions independently, and
multiple versions of one Plugin may coexist for testing or migration. The page
does not offer a default version or automatic upgrade. Enablement checks asset
compatibility, namespace collisions, policy requirements, and workspace
capacity before committing an auditable change.

Plugin pages can link to Agents that bind a version, but Agent-specific Skill
and Tool selection remains in the Agent designer. Enabling a Plugin never
auto-binds contained assets.

### Plugin Acceptance Criteria

- Organization installation, review, Workspace enablement, and Agent binding
  are separate actions with separate authorization checks.
- Multiple exact Plugin Versions can be enabled in one Workspace without an
  implicit default or silent replacement.
- Version detail is immutable and exposes contained assets, digest, provenance,
  compatibility, risk, and review evidence.
- Namespace, dependency, and policy conflicts block enablement with actionable
  findings.
- Installation or enablement never grants an Agent contained Skill or Tool
  authority automatically.
- Enablement, disablement, review, conflict, and quarantine events are auditable.

## 9. Tools Page Specifications

### Tool Inventory

`/tools` is one organization-scoped inventory with segmented views for
Built-in, MCP, and CLI tools. The default table shows fully qualified name,
provider or Tool Server, descriptor version, effect and idempotency class,
data classification, trust tier, health, activation state, and Agent usage.
Workspace context filters availability and enablement without hiding the
organization-owned descriptor source.

Tool inventory search covers name, server, provider type, descriptor version,
effect, data class, lifecycle state, and Agent usage. Tool inventory commands
are limited to opening the owning Server or Descriptor route and reviewing
activation or deprecation state; Agent-specific authority is configured only
in Agent Designer.

### Tool Server

The Tool Server resource uses sections for Overview, Connection, Discovery,
Descriptors, Health, and Credentials. Connection and credential fields show
references and capabilities, never secret values. Health includes last check,
latency or availability evidence, discovery time, and degraded or quarantined
state. Overview includes Recent activity and links to the global Audit explorer
with the Tool Server pre-filtered.

MCP discovery is an explicit review workflow. A discovery result is shown as a
proposed Descriptor Version with schema, effect, idempotency, classification,
credential, destination, and compatibility changes. Discovery cannot add or
replace an active descriptor silently. Multiple compatible Descriptor Versions
for one fully qualified tool name may remain active at once; no version is
treated as a default.

### Tool Descriptor

Descriptor detail is an immutable, read-only route. It shows the fully
qualified name, version and digest, input/output schemas, effect and
idempotency, data classification, destination and credential requirements,
limits, deprecation or replacement metadata, activation evidence, and Agent
usage. The route links to the owning Tool Server and to Agent bindings without
editing those bindings inline.

Descriptor activation or deprecation requires the applicable organization or
workspace permission, compatibility validation, namespace checks within the
selected version, and an audit event. A new version is not rejected merely
because another version of the same fully qualified name is active. Existing
Agent Revisions remain pinned to their exact descriptor even when a newer
descriptor is discovered or activated.

### CLI Command Profile

CLI profiles use the same inventory and descriptor evidence model but expose
structured executable identity, argument schema, filesystem scope, environment
allowlist, image/runtime requirement, effect, idempotency, and interceptor
policy. The page never accepts arbitrary command text as a production profile.

### Tools Acceptance Criteria

- Built-in, MCP, and CLI assets are discoverable from one inventory with clear
  provider-type segmentation.
- Tool Server connection metadata and secrets remain separate from immutable
  Tool Descriptor content.
- MCP discovery creates a proposed descriptor and never silently changes an
  active descriptor or Agent Revision.
- Descriptor activation blocks unresolved schema, per-version namespace,
  compatibility, credential, destination, and policy conflicts with actionable
  findings; same-name versions may coexist when each binding selects an exact
  version.
- Descriptor routes are read-only; Agent-specific narrowing and approvals are
  configured in Agent Designer.
- Descriptor activation, deprecation, quarantine, discovery, and health changes
  are attributable and auditable.

## 10. Operate Page Specifications

### Runs Workbench

`/runs` is the global operational workbench. Its table supports filters for
workspace, Agent, exact Revision hash, Deployment, actor, integration,
status, risk, runner, model route, start time, and failure class. The default
view prioritizes active, blocked, failed, and recently completed runs without
losing access to historical runs.

The Agent route `/agents/:agentId/runs` is the same run projection with a fixed
Agent filter and preserves the global query controls. A Run row shows Session and
run identifiers, Agent and Revision, Deployment, actor, state, elapsed time,
tool or approval activity, failure summary, and last event time. It never
silently substitutes the current Production Revision for the recorded one.

### Run Detail

`/runs/:runId` presents a durable timeline with assignment, lease, model,
prompt/Skill provenance summary, tool calls, approvals, policy decisions,
artifacts, checkpoints, resource usage, runner diagnostics, and terminal state.
Sensitive prompt, credential, and internal Tool Binding content follows the
actor's configuration-read and run-read permissions; redaction is visible and
auditable.

The header identifies the exact Agent Revision, Deployment, actor, integration,
policy snapshot, and run-manifest digest. Configuration links open immutable
evidence views rather than mutable Agent drafts. Timeline filters can isolate
semantic events, model output, tool effects, approvals, artifacts, or errors.

### Admin and Copilot Run Boundary

Admin Runs are an organization or Workspace-scoped operational projection. The
workbench supports cross-actor search, failure diagnosis, runner and lease
evidence, model and Tool details, policy decisions, resource usage, and
authorized operational commands. It is not a second approval inbox: approval
evidence is read-only and only the authenticated Run requester can decide.

Copilot does not expose this global operational table. Its Sessions and Session
detail views are member-scoped, conversation-first projections that show Run
history, user-visible activity, requester approvals, and Artifacts. Copilot
does not expose runner internals, raw prompts, credentials, unrestricted
terminal output, or cross-user operational data. Both projections link the same
immutable Session, Run, event, and Artifact identities when the actor is allowed
to see them.

### Run Actions

Available commands depend on state and independent permissions:

- `Cancel` requests cooperative cancellation and shows lease, runner, and
  cleanup progress.
- `Retry` creates a new Run and requires an explicit choice to reuse the
  original Revision or target the current authorized Production Deployment;
  it never rewrites the original Run.
- `Quarantine Agent` or `Quarantine Deployment` opens the owning emergency
  workflow and blocks new claims without changing historical evidence.
- `Inspect approval evidence` opens the read-only action digest, requester
  decision, expiry, and outcome in Run Detail; `Open artifact` navigates to the
  exact linked resource.
- `Inspect tool call` shows the action digest, approval, policy decision,
  result classification, and redacted output permitted to the actor.
- `View conversation` is available to a `workspace_agent_editor` only after an
  explicit self-enrollment command for the Session's Agent Workspace. It adds
  the caller as `viewer`, records Audit evidence, and opens the ordinary
  member-scoped Copilot view; it cannot add another principal or grant write,
  execution, approval, or membership-management authority.

Destructive, authority-broadening, or external-effect commands require a
confirmation surface with scope, expected result, and correlation ID. Run
actions are server-authorized and remain available through direct routes only
when the same effective permission is present.

### Runs Acceptance Criteria

- Global and Agent-scoped run lists use the same data contract and differ only
  by fixed filtering.
- Every run identifies one immutable Agent Revision, Deployment, run-manifest
  digest, actor, and effective policy context.
- Admin Run detail is operational and cross-actor within the authorized scope;
  Copilot Session detail is member-scoped and conversation-first. Neither
  projection can approve an action from the other surface.
- Retry creates a new attributable Run with an explicit Revision target.
- Cancellation, quarantine, and external-effect controls fail closed when
  authorization or runner state is unavailable.
- Run detail preserves event ordering, redaction, artifact authorization, and
  audit links across reloads and reconnects.
- Direct URLs, table actions, and API commands enforce identical effective
  permissions.

### Evaluations Workspace

`/evaluations` is one workspace-scoped evaluation workbench with `Suites`,
`Runs`, and `Regressions` views. Filters for Agent, candidate Revision,
Production baseline, suite, owner, gate, status, risk, environment, and time are
stored in the URL. `/agents/:agentId/evaluations` is the same result projection
with a fixed Agent filter.

The executable resource, schema, state, and authorization target is defined in
[Admin Governed Resource Contracts](../architecture/admin-governed-resource-contracts.md).

### Suites

Suites lists purpose, owner, immutable latest version, case count, deterministic
and probabilistic coverage, publication-gate usage, recent pass rate, and last
run. A Suite resource uses Overview, Cases, Versions, Gates, and Runs sections.
Overview includes Recent activity and links to the global Audit explorer with
the Suite pre-filtered. Editable suite and case working copies are distinct
from immutable Golden Case and Suite Versions used by evaluation runs.

Suite authoring supports authored cases and reviewed sanitized production-run
exports. Each case exposes inputs, fixtures, assertions, rubrics, provenance,
redaction findings, compatibility constraints, and effect-safety checks. Large
fixtures and expected outputs are represented by content-addressed references.

### Evaluation Runs

Starting an Evaluation Run requires an exact candidate Agent Revision, an
immutable Suite Version, an evaluation environment, and an optional exact
baseline Revision. Mutable Agent Drafts and mutable suite working copies cannot
execute. The start confirmation summarizes case count, model/evaluator policy,
fixture environment, expected cost, write interception, and absence of
production credentials.

The Runs view shows candidate and baseline, suite version, environment, status,
deterministic pass rate, probabilistic score summary, policy violations,
fixture misses, cost, latency, and gate result. Evaluation Run detail separates
deterministic assertions from probabilistic scores and exposes aligned evidence
for tool calls, policy decisions, VCR matches, filesystem/database deltas,
artifacts, cleanup, and environment integrity.

### Regressions

Regressions groups newly failing deterministic cases, policy or side-effect
violations, quality-distribution changes, latency/cost movement, unexpected
tool use, fixture misses, and environment/evaluator drift. Comparisons are
marked invalid when candidate and baseline evidence is not materially
comparable. Every regression links to the exact case result and both immutable
Revision/evaluation manifests.

### Publication Gates

Gate status is evidence attached to an Agent Revision and Review; it does not
publish the Agent automatically. Overrides require authorized review, reason,
scope, and expiry, and never modify the underlying Evaluation Run. Publication
rechecks that required Suite Versions, results, and overrides still apply to the
exact candidate Revision.

### Evaluations Acceptance Criteria

- Suites, Runs, and Regressions share one workspace and URL-preserved filters;
  Agent Evaluations differs only by its fixed Agent scope.
- Every Evaluation Run binds an exact Agent Revision, Suite Version, fixture
  manifest, runtime image, model/evaluator policy, and environment digest.
- Evaluation mode cannot resolve production credentials or reach real write
  targets through sandbox or trusted gateways.
- Deterministic evidence and probabilistic scoring are visually and
  semantically distinct.
- Invalid baseline comparisons are labeled and cannot satisfy a publication
  gate silently.
- Gate overrides are scoped, expiring, attributable, and auditable without
  rewriting results.

## 11. Integrations Page Specifications

### Integration Directory

`/integrations` is an organization-scoped directory of external enterprise
systems. Each row shows Integration name, owner, environments, client status,
published Agents, delegated-subject policy, webhook health, recent requests, quota
state, and last activity. Search and filters cover owner, environment, status,
delegated-subject requirement, published Agent, webhook health, and recent
failures.

Creating an Integration establishes its identity and ownership only. It does
not issue a client credential, publish an Agent, or grant invocation authority
implicitly.

### Integration Resource

The resource uses Overview, Clients, Agent Publications, Webhooks, and Usage
sections. Overview summarizes environments, owners, active publications,
credential expiry, webhook incidents, quota pressure, and recent invocation
outcomes without exposing secret values or internal Agent configuration. It
also shows Recent activity and links to the global Audit explorer with the
Integration pre-filtered.

### Clients

Clients are environment-bound OAuth registrations with audience,
delegated-user token-exchange policy, owner, status, credential fingerprint,
issue and expiry time, and rotation history. A generated secret or private key
is shown only once. Rotation can overlap old and new credentials for a bounded
interval and is independently auditable. Disabling a client blocks new calls
without deleting historical tasks, runs, deliveries, or evidence. A client
credential authenticates transport only and never becomes a Run requester.

### Agent Publications

An Agent Publication is nested under one Integration and pins an exact Agent
Revision, input and output contract versions, workspace, environment,
verified delegated-user requirements, allowed scopes,
visible artifacts and event projection, quotas, budgets, retention, and
effective interval. Publication cannot broaden the selected Agent Revision's
Tools, credentials, destinations, or policy authority.

Creating or changing a Publication uses a semantic diff and compatibility
check. The detail route shows the exact Revision and contract digests, effective
authority, client/environment bindings, usage, recent failures, review evidence,
and audit history. Expiry or revocation blocks new invocations while preserving
existing Session and Run evidence. There is no global Publications page; Agent
Overview and Versions may link back to consuming Integration Publications.

### Webhooks

Webhook endpoints are nested under an Integration and record environment,
approved HTTPS destination, authentication mode, subscribed event projection,
status, signing-key fingerprint, rotation state, and delivery policy. Private
keys and secrets are never shown after creation.

Webhook detail shows ordered delivery attempts, event and delivery IDs,
signature-key reference, response class, latency, retry schedule, and terminal
state. Explicit redelivery reuses the same immutable event and creates an
auditable delivery attempt; it never changes the Run result. Destination
changes require SSRF/private-network validation and a confirmation of affected
subscriptions.

### Usage

Usage aggregates requests, concurrency, latency, errors, model/tool cost,
artifact transfer, quota consumption, webhook delivery, and delegated-user
failures by environment, client, Publication, Agent, and time. Metrics link to
filtered Runs and delivery evidence rather than becoming an alternate run
explorer.

### Integrations Acceptance Criteria

- Integration identity, client credential issuance, Agent Publication, and
  webhook registration are separate authorized actions.
- Every invocation resolves one active client, one exact Agent Publication, one
  Agent Revision, one input/output contract pair, and one verified delegated
  subject.
- A client without a delegated subject cannot invoke directly; client and
  subject identities remain independent in Policy and Audit evidence, while
  only the subject becomes Session owner and Run requester.
- Publication never broadens Agent authority and fails closed on incompatible
  contracts, expired clients, revoked scope, or invalid environment binding.
- Secret values are one-time or never displayed; normal pages expose only
  references, fingerprints, status, and rotation metadata.
- Outbound Webhook delivery is signed, idempotent, auditable, and independent of Run
  outcome.
- Revocation and expiry block new work without rewriting historical evidence.

## 12. Policies Page Specifications

### Policy Catalog

`/policies` is the unified organization and workspace policy catalog. It does
not split approval, model, network, command, credential, data, budget,
retention, or evaluation policies into separate top-level pages. Each row shows
the Policy name, type, owning scope, owner, Draft validation state, latest
immutable Version, active Bindings, affected Agents, and last change. Filters
cover type, organization or Workspace scope, lifecycle state, validation state,
binding target, affected Agent, owner, and recent changes.

The catalog distinguishes a Policy's authored state from its effective use. A
valid Draft is not active, publishing a Version does not bind it, and removing
a Binding does not delete the immutable Version or its evidence.

### Policy Resource

`/policies/:policyId/*` uses Overview, Draft, Versions, Bindings, and Simulation
sections. The header always shows Policy type, owning scope, owner,
Draft state, latest Version, active Binding count, and whether a stricter
organization Policy also constrains the selected Workspace or Agent.

Overview summarizes purpose, schema, scope, owners, current Draft, latest
Version, active Bindings, affected resources, recent simulations, and recent
changes. It includes Recent activity and links to the global Audit explorer with
the Policy pre-filtered. It links to effective-policy explanations rather than
presenting a single editable "effective policy" document.

### Draft

Each Policy has one mutable Draft. Editing uses schema-aware forms for the
Policy type, with a structured source view only where the typed form cannot
express an advanced supported field. The editor exposes validation findings,
field provenance, unsaved state, optimistic-concurrency conflicts, and a
semantic comparison with one selected immutable Version.

Saving a Draft does not change runtime authorization. Publishing requires a
valid Draft, a required change message, the exact Draft ETag, and confirmation
of affected Bindings and Agents. A Policy has no named Drafts, branches,
merge, or rebase workflow.

### Versions

Publishing creates an immutable Policy Version with an exact Version ID,
content digest, schema version, author, message, creation time, validation
evidence, and canonical policy document. Versions can be compared semantically
and inspected independently, but never edited or deleted while referenced by a
Binding, Agent Revision, Run Manifest, review, simulation, or audit event.

Version history is linear by creation time and does not imply a branch graph.
Publishing a Version does not automatically replace any active Binding. An
operator changes runtime behavior only by selecting that exact Version in a
Binding or by publishing an Agent Revision that pins it.

### Bindings

Bindings show which exact Policy Version applies to an Organization,
Workspace, Agent Revision, Deployment, Integration Publication, or governed
platform resource. Each row shows target, environment, effective interval,
binding status, actor, reason, and the stricter outer Policies that also apply.

Organization and Workspace Policies compose by intersection. A Workspace
Binding may add restrictions or choose a narrower allowance, but cannot
broaden an Organization Policy. Agent Revisions pin exact Policy Versions and
remain constrained by both scopes; no movable "latest" or default Version is
resolved at run time. If a proposed Binding would be ineffective or broader
than an outer Policy, the UI explains the conflict and blocks activation rather
than silently weakening or ignoring the outer rule.

Binding changes show affected Agents, Deployments, Publications, and new-run
behavior before confirmation. They do not rewrite existing Agent Revisions or
historical Run Manifests. Emergency restriction or revocation uses an explicit
auditable Binding or resource-state command, not mutation of a Version.

### Simulation

Simulation evaluates a selected Draft or immutable Version against a
user-supplied scenario containing principal or client identity, Workspace,
Agent Revision, environment, tool or model operation, normalized arguments,
destination, credential mode, data classification, budget, and relevant prior
approval state. It returns `allow`, `deny`, or `require_requester_approval`,
the matched rules, contributing Policy Versions, ineffective lower-scope
rules, and a concise explanation.

A comparison mode evaluates the same scenario against the currently effective
Bindings and a candidate Draft or Version. Simulation never executes a Tool,
resolves a credential secret, creates an Approval Request, or changes a
Binding. Results are evidence for review and troubleshooting, not reusable
authorization decisions.

### Approval Policy Boundary

An Approval Policy configures whether a concrete Agent action is allowed,
denied, or requires approval from its authenticated Run requester, including
risk criteria and expiry. It does not nominate generic approvers, create an
Admin approval queue, or represent business workflow approvals. Pending Agent
action approvals remain in Copilot; Admin Run Detail and Audit expose
read-only evidence only.

### Policies Acceptance Criteria

- All Policy types share one catalog and the same Overview, Draft, Versions,
  Bindings, and Simulation resource structure; Recent activity links to the
  canonical global Audit explorer.
- A Policy has one mutable Draft; runtime behavior references only immutable
  exact Policy Versions and never a Draft or a movable latest pointer.
- Publishing and Binding are separate authorized, attributable actions.
- Organization and Workspace rules intersect, and no lower scope can broaden
  an outer Policy or another effective authority boundary.
- Agent Revisions and Run Manifests preserve exact Policy Version identities
  and content digests for reproduction and audit.
- Simulation is side-effect free and clearly distinguishes a candidate result
  from current effective behavior.
- Approval Policy configuration never creates an Admin approval inbox or takes
  ownership of business approvals.

## 13. Audit Page Specifications

### Audit Explorer

`/audit` is the only complete Admin audit experience. It searches immutable,
attributable events across Agents, Skills, Plugins, Tools, Runs, Evaluations,
Integrations, Policies, platform resources, and security controls. Filters
include resource type and ID, scope, actor, event type, outcome, risk,
correlation ID, linked Run/Revision/Policy Version, and time. URL state is
shareable and direct navigation remains subject to the actor's scope.

The default table shows time, actor, action, resource, scope, outcome, risk,
correlation ID, and linked evidence. It is an evidence explorer, not a second
configuration or operations workbench. It does not approve actions, edit
resources, retry Runs, or change Policy Bindings.

Resource pages expose only a compact Recent activity component. It shows a
small, pre-filtered slice and links to `/audit` with the resource query encoded
in the URL. Recent activity is not a second table contract, export workflow, or
resource-specific audit store.

### Audit Event Detail

`/audit/events/:eventId` shows the immutable event envelope, actor and
authentication context, resource and scope, action, outcome, timestamp,
correlation and request IDs, policy/version references, and linked Run,
Revision, Binding, Publication, Tool, Artifact, or Approval evidence. Sensitive
payloads follow the same redaction and capability checks as the owning resource;
redacted fields remain visibly marked.

The detail page can open the owning resource or a filtered Run/Policy/Integration
view, but those links never grant additional authority. Security Reviewers,
Auditors, and Organization Administrators may export the selected evidence or a
filtered result set as a signed, scoped package. Operators and other read-only
actors can inspect and locate events but cannot export them. Export creation,
download, and failure are themselves audit events; the export applies the
caller's current scope and redaction rules and never includes secret values or
raw chain-of-thought.

Export is asynchronous. The page shows requested, processing, ready, expired,
and failed states, the query/scope digest, package digest, expiry time, and a
short-lived download command only after readiness. A failed export can be
retried without changing its query digest; changing filters creates a new export
request.

### Audit Acceptance Criteria

- `/audit` and `/audit/events/:eventId` are the only full Audit routes.
- Agent, Plugin, Tool, Evaluation, Integration, and Policy resources use Recent
  activity plus a pre-filtered link instead of separate Audit tables.
- Run timelines, Policy Version history, Webhook delivery history, and similar
  domain views remain available as operational evidence, but do not reimplement
  the global Audit query or export contract.
- Audit events are append-only, attributable, redacted by capability, and
  linked to immutable resource identities and correlation IDs.
- Audit export is a separate `audit.export` capability. It is granted to
  Organization Administrators, Security Reviewers, and Auditors within scope;
  Operator read access does not imply export.
- Audit is read-only with respect to the owning resource; consequential actions
  stay on their owning pages and are independently authorized.

### Retention and Legal Hold

Retention configuration lives under `/platform/settings`, not in the Audit event
table. Organization Administrators define retention bounds for Audit metadata,
operational metadata, prompts and outputs, terminal streams, Artifacts, and
Evaluation fixtures. Workspace settings may choose values within those
organization bounds; the product does not prescribe universal day counts before
Legal and Security review.

The settings view shows the effective value, organization bound, affected data
classes, pending deletion jobs, and the next eligible deletion window. A Legal
Hold names its owner, authority basis, scope or selector, affected data classes,
status, and set or release history. Active Holds block scheduled deletion and
key destruction for matching content and evidence.

Hold selectors use bounded fields for scope, resource identifiers,
Session/Run/Artifact identifiers, classification, and time range. They are frozen
when activated; the match preview is evidence, while deletion re-evaluates active
Holds against newly matching data. Arbitrary SQL selectors are not exposed.

Deletion is an explicit, asynchronous workflow. The confirmation surface shows
the estimated scope, matching Holds, protected records, and resulting
tombstone behavior. Deletion requests, pending state, blocked records,
completion, failure, retry, Hold creation, and Hold release are all visible in
the global Audit explorer. Audit metadata and signed integrity checkpoints stay
through the configured minimum even when content is removed; deleted content is
represented by a digest-preserving Tombstone.

Deletion Jobs use requested, evaluating, pending, running, completed, blocked,
and failed states. Only failed jobs may be retried, and every execution attempt
re-checks active Holds, minimum Audit retention, classification, and key
destruction eligibility.

## 14. Platform Settings Page Specification

### Purpose and scope model

`/platform/settings` is one Settings experience with an explicit scope switcher:

```text
Organization | Workspace
```

The typed resource, OpenAPI command, and authorization target for Platform
management is defined in
[Admin Governed Resource Contracts](../architecture/admin-governed-resource-contracts.md).

There is no separate Workspace Settings route. Organization Administrators
maintain organization defaults and non-negotiable bounds. A Workspace can set
an override only when it remains inside those bounds; a lower scope can narrow
authority or capacity but never broaden organization policy. The header always
shows the active scope and, for Workspace scope, the selected Workspace.

Settings is a composed view over typed platform resources. It is not a mutable
catch-all `PlatformSettings` record and does not take ownership of Integrations,
Model Providers, Runner Pools, Policies, or the global Audit explorer.

### Audience and authorization

The page evaluates explicit capabilities server-side and presents them through
simple role-oriented actions rather than an Allow/Deny rule builder:

| Actor | Read | Mutate |
| --- | --- | --- |
| Organization Administrator | Organization and all Workspace effective settings | Organization defaults/bounds, Workspace overrides, classifications, limits, environments, retention, and Legal Holds within organization scope |
| Security Reviewer | All settings and validation/deletion impact | Legal Holds and retention simulations; no identity, environment, or quota administration unless separately granted |
| Workspace Agent Editor | Effective settings for assigned Workspace | No Platform Settings mutation |
| Operator | Runtime-effective limits, environments, retention status, and pending deletion impact | No Platform Settings mutation |
| Auditor | Authorized settings and change evidence | No mutation; scoped Audit export remains governed by `audit.export` |

Every mutation is checked against organization authorization, Workspace scope,
the owning resource capability, and action-time policy. Hidden controls are not
security boundaries; direct API calls return a non-leaking authorization error.

### Sections and layout

The page uses a persistent left section index and a central settings workspace.
The first viewport contains scope, an effective-settings summary, unresolved
conflicts, pending deletion or Legal Hold warnings, and Recent activity. The
sections are:

1. **Overview**: effective values, inherited organization bounds, override
   count, validation state, and links to the owning section.
2. **Organization**: display name, slug, status, support/contact metadata, and
   organization-level defaults. Identity changes show a before/after preview
   and never alter authentication subjects.
3. **Retention**: bounds at Organization scope and selectable values at
   Workspace scope for Audit metadata, operational metadata, prompts and
   outputs, terminal streams, Artifacts, and Evaluation fixtures. Exact day
   counts are deployment configuration pending Legal and Security approval.
4. **Legal Holds**: active and released Holds, owner, authority basis, scope or
   selector, affected data classes, protected deletion jobs, and set/release
   history. Create and release use dedicated confirmation flows.
5. **Data Classifications**: definitions, allowed handling, default class,
   inheritance, and affected resources. Classification changes validate against
   provider, Tool, Integration, and retention constraints.
6. **Limits and Quotas**: organization ceilings and Workspace allocations for
   concurrency, Run duration, output/Artifact size, instruction volume, and budget.
   Provider budgets, runner capacity, and Integration quotas remain on their
   owning pages; Settings only supplies global bounds and defaults.
7. **Environments**: named `development`, `staging`, and `production` profiles,
   data handling posture, allowed publication targets, emergency state, and
   visible environment markers. Provider credentials and webhook secrets stay
   on their owning resources.
8. **Recent activity**: a compact Settings-filtered slice linking to the global
   `/audit` explorer. It is not a second audit table or export workflow.

Each editable section shows the current effective value, source (`Organization`
or `Workspace override`), organization bound, last change actor/time, and
validation status. A Workspace override can be reset to inheritance without
deleting the organization value.

### Edit, validation, and conflict behavior

- Edits are section-scoped and saved as an explicit command with an expected
  settings ETag; optimistic UI is not used.
- `Validate` is side-effect free and reports narrowed/broadened authority,
  affected resources, retention/deletion impact, and cross-section conflicts.
- `Save` presents a semantic diff, target scope, effective result, and pending
  asynchronous work before confirmation.
- A stale ETag returns a conflict with the current effective projection and a
  rebase-free choice to reload or discard local edits. The page never silently
  overwrites another administrator's change.
- Organization bound changes are rejected when they would invalidate existing
  Workspace overrides, active Policy Bindings, published Agent Revisions, or
  protected retention evidence. The response names the blocking resource or
  asks the administrator to narrow the change first.
- Retention deletion is asynchronous. The page shows an estimate, next eligible
  window, active Holds, blocked records, retry state, and tombstone outcome;
  deletion commands re-check Holds and minimum Audit retention at execution.
- All accepted, rejected, blocked, and completed mutations expose a correlation
  ID and a link to the canonical Audit event.

### Acceptance criteria

- One route and one explicit scope switcher are used for Organization and
  Workspace settings; there is no parallel Workspace Settings page.
- Organization values are authoritative bounds. Workspace values can only
  inherit or narrow within those bounds.
- Settings does not duplicate Integration, Provider, Runner, Policy, or Audit
  configuration; it links to the owning page when a value is managed there.
- Read-only actors see effective values and evidence appropriate to scope, but
  never secret values or protected content.
- Legal Hold creation/release and deletion impact are attributable, auditable,
  and visible without becoming an Admin approval inbox.

## 15. Cross-Page Conventions

- List filters serialize to the URL and survive reload and sharing.
- Resource detail pages use stable nested routes rather than modal-only deep
  content.
- Drawers are limited to previews and quick comparisons; editable or auditable
  resources receive routes.
- Tables preserve column layout during loading and support an accessible compact
  density.
- Status, risk, scope, version, and lifecycle vocabulary is shared across all
  pages.
- Mutations expose pending, success, conflict, rejected, and retryable states.
- Optimistic UI is not used for publication, approval, policy, quarantine,
  credential, runner, or destructive operations.
- Correlation IDs and audit links are available on terminal errors and
  consequential mutations.
- Resource pages use `Recent activity` for contextual history and link to the
  canonical `/audit` explorer; they do not create resource-specific Audit
  stores or full duplicate tables.
- Retention and Legal Hold settings are organization-governed and exposed from
  Platform Settings; Audit provides evidence and status, not an alternate
  deletion control surface.
