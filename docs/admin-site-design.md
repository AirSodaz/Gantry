# Gantry Admin Site Design

## 1. Scope and Status

This document defines the complete target page and function design for Gantry
Admin. It is implementation-ready product structure, not a statement that every
route exists today. Each page is labeled by delivery status:

- **Current:** a working product slice exists in the repository.
- **Next:** required for the next Admin configuration slice.
- **Later:** target product behavior after the next slice.

The existing visual language and shared design-system components remain the
foundation. Detailed visual mockups and frontend implementation follow this
page/function specification.

## 2. Application Shell

### Primary Navigation

The left sidebar uses non-clickable group labels and direct page links. It does
not add empty section landing pages.

| Group | Page | Target route | Scope | Delivery |
| --- | --- | --- | --- | --- |
| Home | Overview | `/` | Current workspace or all workspaces | Next |
| Build | Agents | `/agents` | Workspace | Current |
| Build | Skills | `/skills` | Workspace | Next |
| Build | Plugins | `/plugins` | Organization catalog with workspace enablement | Next |
| Build | Tools | `/tools` | Organization inventory with workspace availability | Next |
| Operate | Runs | `/runs` | Workspace or all workspaces | Later |
| Operate | Approvals | `/approvals` | Assigned and administratively visible requests | Later |
| Operate | Evaluations | `/evaluations` | Workspace | Later |
| Govern | Integrations | `/integrations` | Organization | Later |
| Govern | Policies | `/policies` | Organization and workspace | Later |
| Govern | Audit | `/audit` | Authorized organization/workspace scope | Later |
| Platform | Runners | `/platform/runners` | Organization | Later |
| Platform | Model Providers | `/platform/model-providers` | Organization | Later |
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
| Workspace Agent Editor | Agents, Skills, enabled Plugins, available Tools, workspace Runs and Evaluations |
| Security Reviewer | Agent reviews, Plugin/Tool risk, Policies, relevant Approvals, Audit, and evaluation evidence |
| Operator | Overview, Runs, Approvals, runtime artifacts, Runners, provider health, and operational Audit |
| Auditor | Read-only Overview, Runs, Approvals, Integrations, Policies, and Audit within assigned scope |
| Approver | Assigned Approvals and the minimum linked run/action context needed to decide safely |

Navigation hides pages the actor cannot use. Direct navigation still performs
server authorization and returns a non-leaking denied state.

## 4. Complete Page Inventory

### Overview

| Page | Route | Responsibility | Delivery |
| --- | --- | --- | --- |
| Overview | `/` | Scope-aware operational and governance summary | Next |

### Build

| Page | Route | Responsibility | Delivery |
| --- | --- | --- | --- |
| Agent list | `/agents` | Search, filter, create, and inspect workspace Agents | Current |
| New Agent | `/agents/new` | Create Agent identity and initial draft | Current |
| Agent resource | `/agents/:agentId/*` | Own the Agent overview, design, versions, usage, access, and history | Current, expanding next |
| Agent review | `/agents/:agentId/review/:reviewId` | Review semantic changes and record a decision | Current, expanding next |
| Agent revision | `/agents/:agentId/versions/:revisionHash` | Inspect one immutable revision and its review, test, and deployment evidence | Next |
| Skill list | `/skills` | Browse and filter standalone workspace Skills | Next |
| Import Skill | `/skills/import` | Import a package from a marketplace, direct locator, or manual upload and validate its declared metadata | Next |
| Skill resource | `/skills/:skillId/*` | Inspect imported artifacts, declared versions, source provenance, activation state, and Agent usage | Next |
| Skill artifact | `/skills/:skillId/artifacts/:artifactId` | Inspect one imported package artifact, content digest, requirements, and usage | Next |
| Plugin catalog | `/plugins` | Browse installed/available Plugins and workspace enablement | Next |
| Install Plugin | `/plugins/install` | Review source, contents, permissions, compatibility, and install | Next |
| Plugin resource | `/plugins/:pluginId/*` | Inspect versions, contained assets, workspaces, health, and audit | Next |
| Plugin version | `/plugins/:pluginId/versions/:versionId` | Review immutable package contents and permission changes | Next |
| Tool inventory | `/tools` | Browse built-in, MCP, and CLI descriptors across providers | Next |
| Tool Server | `/tools/servers/:serverId/*` | Configure connection metadata, discovery, health, and descriptors | Next |
| Tool Descriptor | `/tools/descriptors/:descriptorId` | Inspect schemas, effects, risk, compatibility, and Agent usage | Next |
| CLI Command Profile | `/tools/cli-profiles/:profileId` | Define and inspect governed structured command execution | Next |

### Operate

| Page | Route | Responsibility | Delivery |
| --- | --- | --- | --- |
| Run list | `/runs` | Search, filter, compare, and operate task attempts | Later |
| Run detail | `/runs/:runId` | Timeline, output, tools, approvals, artifacts, configuration, and diagnostics | Later |
| Approval queue | `/approvals` | Triage actionable, waiting, expired, and resolved approvals | Later |
| Approval detail | `/approvals/:approvalId` | Review one exact action digest and decide when authorized | Later |
| Evaluation suite list | `/evaluations` | Browse suites, coverage, regressions, and publication gates | Later |
| Evaluation suite | `/evaluations/suites/:suiteId/*` | Author cases, versions, assertions, and gate policy | Later |
| Evaluation run | `/evaluations/runs/:runId` | Compare candidate/baseline results and inspect failures | Later |

### Govern

| Page | Route | Responsibility | Delivery |
| --- | --- | --- | --- |
| Integration list | `/integrations` | Manage registered enterprise clients and publications | Later |
| Integration resource | `/integrations/:integrationId/*` | Clients, agent contracts, webhooks, quotas, credentials, and audit | Later |
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
| Settings | `/platform/settings` | Organization identity, retention, classifications, limits, and environments | Later |

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

1. Show an attention queue ordered by urgency: incidents, blocked runs, aging
   approvals, failed publication gates, quarantined assets, expiring
   credentials, and unhealthy providers or runner pools.
2. Summarize active runs, queue age, failure rate, approval wait, artifact
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
| `execute` | Discover and invoke the Agent through an authorized Copilot or integration surface | Configuration read, Draft access, or unrestricted tool authority |
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
| Overview | `/agents/:agentId` | Current state, ownership, production/test deployments, attention items, and recent activity | Next |
| Design | `/agents/:agentId/design/:draftId` | Edit one named Draft, validate it, commit Revisions, and start test/review workflows | Current, expanding next |
| Versions | `/agents/:agentId/versions` | Browse Draft latest Revisions, immutable snapshots, test Deployments, and Production history | Next |
| Runs | `/agents/:agentId/runs` | Filter runtime attempts for this Agent and exact Revisions | Later |
| Evaluations | `/agents/:agentId/evaluations` | Compare suite results and publication-gate evidence by Revision | Later |
| Access | `/agents/:agentId/access` | Owners, editors, reviewers, publishers, consumers, and integration visibility | Later |
| Audit | `/agents/:agentId/audit` | Immutable configuration, review, deployment, access, and emergency events | Later |

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

- Route: `/agents/:agentId/versions/:revisionHash`.
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

### Audit

Audit includes Draft creation/archive, Revision commits, validation, comparison,
review decisions, test/Production deployment changes, rollback, quarantine,
access changes, effective-access denials, and denied consequential commands.

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

## 8. Cross-Page Conventions

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
