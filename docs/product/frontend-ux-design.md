# Frontend UX Design

This document fixes the shared experience principles and cross-product
information architecture. The complete page inventory, route map, field-level
functions, interactions, and page acceptance criteria are defined in the
[Admin Site Design](admin-site-design.md) and
[Copilot Site Design](copilot-site-design.md) documents. This document remains
the shared UX contract; implementation status and rendered acceptance evidence
are tracked separately.

## 1. Experience Boundary

Gantry Admin and Gantry Copilot are separate React applications. They may share
design tokens, accessible primitives, authentication utilities, event-stream
clients, and typed API contracts, but they must not share top-level navigation
or expose hidden administrator routes in the Copilot bundle.

Recommended deployment origins:

- `admin.gantry.example.com` for Gantry Admin
- `copilot.gantry.example.com` for Gantry Copilot

Separate OAuth clients and API audiences reduce accidental privilege mixing.

## 2. Shared Design Language

The visual language is quiet, dense, and operational. It favors legibility,
comparison, and repeated action over marketing-style presentation.

### Foundations

- Use a neutral base palette with distinct semantic colors for success (`#10a37f`),
  warning (`#d97706`), danger (`#e11d48`), information/brand (`#0284c7`), approval,
  and suspended states.
- Use one accessible sans-serif UI family and a monospace family (`--ds-font-mono`:
  `ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace`) for commands,
  logs, identifiers, JSON, and diffs.
- Use an 8 px spacing grid and consistent design token radii (`--ds-radius-sm: 8px`,
  `--ds-radius: 12px`, `--ds-radius-lg: 16px`, `--ds-radius-full: 9999px`).
- Use stable heights for toolbars, table rows, tabs, terminal headers, and run
  status bands to prevent layout movement during streaming.
- Use icons from a maintained icon library such as Lucide. Every unfamiliar
  icon-only control has a tooltip and accessible name.
- Avoid nested cards. Major page regions are unframed; cards are reserved for
  repeated catalog items, modals, and tightly scoped tools.

### State Vocabulary

Use the same state names, colors, and visual indicators everywhere across 6 standardized
categories:

1. **In-Flight & Execution** (Amber pulse dot): `Running`, `Provisioning`, `Canceling`,
   `Processing`, `Evaluating`, `Draining`, `Validating`, `Hashing`.
2. **Pending & Review** (Indigo/Purple static dot): `Queued`, `Pending`, `Requested`,
   `Awaiting approval`, `Awaiting requester input`, `In review`, `Draft`, `Proposed`.
3. **Success & Released** (Emerald green static dot `#10a37f`): `Completed`, `Published`,
   `Active`, `Ready`, `Released`, `Available`, `Valid`, `Passed`, `Approved`, `Standard`,
   `Low`, `Public`.
4. **Danger & Blocked** (Rose red static dot `#e11d48`): `Failed`, `Rejected`,
   `Quarantined`, `Blocked`, `Invalid`, `High`, `Confidential`.
5. **Warning & Suspended** (Warm orange static dot `#f59e0b`): `Suspended`, `Deprecated`,
   `Medium`, `Write`.
6. **Muted & Expired** (Zinc gray static dot `#71717a`): `Canceled`, `Disabled`,
   `Retired`, `Expired`, `Unknown outcome`, `Internal`, `Read`, `Not submitted`.

A page may expose a narrower subset, but it must not rename a shared state or silently
collapse a blocked or unknown outcome into success or ordinary failure.

## 3. Design Style and Philosophy

### Minimalism and Utility-First

Gantry's interface is a working tool, not a showcase. Every element must earn
its screen space. Prefer whitespace and typographic hierarchy over decorative
borders, gradients, or shadows. Default to hiding complexity: progressive
disclosure reveals detail when the user asks for it, not before.

Layout decisions prioritize scan speed. Align labels, values, and actions on a
predictable grid so operators can compare rows without hunting. Avoid
ornamental animation; motion exists only to communicate state transitions
(loading, expanding, collapsing) and respects `prefers-reduced-motion`.

Color is semantic, never cosmetic. The neutral palette recedes; accent colors
mark status, risk, and actionable affordances. If removing a visual element
does not reduce comprehension or usability, remove it.

### Bento Layout and Component Modularity

Page composition follows a Bento-grid approach: self-contained, clearly
bounded content blocks arranged on a spatial grid. Each block owns one concern
(a metric, a list, a form section, a status panel) and communicates its
boundaries through alignment and spacing rather than nested card chrome.

Reference: OpenAI's dashboard and platform pages demonstrate the target
aesthetic — generous padding, flat surfaces, restrained type scale, and
content blocks that breathe without relying on drop shadows or thick borders.

Component boundaries map directly to React component boundaries. A Bento cell
is a self-contained, independently loadable unit with its own loading,
empty, and error states. This enables:

- Parallel data fetching per cell without page-level waterfalls.
- Independent skeleton placeholders that match final dimensions.
- Lazy mounting of below-fold or permission-gated cells.
- Isolated failure: one cell's error does not blank the page.

Grid breakpoints collapse gracefully: desktop Bento grids stack into
single-column layouts on narrow viewports without losing information or
requiring horizontal scrolling.

### Maintainability and Dependency Discipline

Use the platform first. Prefer native browser capabilities, Headless UI primitives,
and type-safe token architectures over monolithic UI frameworks or hand-crafted low-level
overlays:

- **Layout**: CSS Grid, Flexbox, and Tailwind CSS utility composition. No custom grid frameworks.
- **Component Primitives**: Radix UI headless primitives (`@radix-ui/react-dialog`,
  `@radix-ui/react-select`, `@radix-ui/react-dropdown-menu`, `@radix-ui/react-tabs`)
  for accessible dialogs, portals, comboboxes, dropdowns, and tabs. These primitives provide
  built-in focus trapping, collision-aware portal positioning, keyboard roaming (`ArrowDown`,
  `ArrowUp`, `Home`, `End`, `Escape`), and complete WAI-ARIA semantics without styling constraints.
- **Component Encapsulation**: All visual primitives are strictly encapsulated in `@gantry/design-system`
  using Class Variance Authority (`cva`) and `cn()` (`clsx` + `tailwind-merge`). Business
  surfaces consume high-level components with typed variants rather than duplicating raw styling utility strings.
- **Design Tokens & Theme Binding**: Semantic CSS variables (`--ds-bg`, `--ds-surface`,
  `--ds-border`, `--ds-primary`, `--ds-brand-cyan`, `--ds-success`, `--ds-danger`) define light
  and dark themes at the `:root` and `[data-theme]` boundary.
- **Build Toolchain**: Vite 8 with Rolldown bundler for sub-second build, HMR, and test execution.
- **Scrolling and virtualization**: native `overflow` and `content-visibility`
  before reaching for a virtualization library; add one only when profiling
  proves native performance is insufficient for measured data volumes.
- **Forms**: native validation attributes and `FormData`; a form library is
  acceptable only for complex dynamic schemas.
- **State**: React server-state libraries (React Query / TanStack Query) for
  async data; `useReducer` or lightweight stores for local UI state. Global
  state management libraries are introduced only when prop drilling is
  demonstrably impractical across multiple unrelated subtrees.
- **Animation**: CSS transitions and `@keyframes` for UI motion; a JS
  animation library only for orchestrated, physics-based, or scroll-driven
  sequences that CSS cannot express.
- **Date and time**: `Intl.DateTimeFormat` and `Intl.RelativeTimeFormat`
  before adding a date library.
- **Copy and formatting**: `navigator.clipboard`, `Intl.NumberFormat`,
  `Intl.ListFormat` before custom utilities.

When a library is genuinely needed, prefer small, composable, single-purpose
packages over monolithic UI frameworks. Pin major versions and audit bundle
impact before adoption. A dependency added for one feature must not impose
its conventions across unrelated areas of the codebase.

Custom components are justified only when no maintained option meets
accessibility, performance, or domain requirements. Document the reason in
the component's module header when overriding this default.

## 4. Gantry Admin Information Architecture

The authoritative Admin page inventory, routes, roles, delivery status, and
page-level specifications are maintained in
[Gantry Admin Site Design](admin-site-design.md). This section summarizes the
experience direction. The authoritative Copilot route, state, permission, and
acceptance specifications are maintained in
[Gantry Copilot Site Design](copilot-site-design.md).

Primary navigation uses labeled groups with direct page entries. The labels are
not empty landing pages.

1. Overview
2. Build: Agents, Skills, Plugins, Tools
3. Operate: Runs, Evaluations
4. Govern: Integrations, Policies, Audit
5. Platform: Runners, Model Providers, Credentials, Settings

Prompts are configured inside an Agent and do not have a standalone page.
Standalone Skills contain lightweight reusable instructions. Plugins package
business Skills with optional MCP tools and configuration, while Tools remains
the independent inventory for runtime descriptors, health, risk, and policy.
Skills are imported from external package sources or added manually as complete
package artifacts or local directories; the UI displays each package's declared
version and exact content digest. Multiple imported artifacts of one Skill may
coexist for test bindings, without a Gantry-owned version editor or inline
content editing workflow.

Workspace selection is persistent and visible in the application header.
Organization-wide pages are clearly labeled and never silently mixed with a
workspace-scoped list.

### Overview

The first viewport is an operational summary rather than a welcome page. The
default scope is the selected workspace; organization administrators may choose
`All workspaces`. It
contains active runs, runs awaiting requester action, failure rate, runner
capacity, recent policy denials, quarantined agents, and evaluation regressions.
Each metric links to a filtered working view.

### Agent List

The list supports search, owner, lifecycle state, risk class, workspace,
published version, last run, and health filters. Batch actions are limited to
low-risk metadata changes; publication and retirement remain explicit actions.

### Agent Detail

Tabs:

- Overview
- Versions
- Runs
- Evaluations
- Access
- Recent activity (links to the global Audit explorer)

The overview presents employee-facing description, ownership, current release,
health, usage, and risk summary. It does not make the editable draft look like
the published production configuration.

### Agent Designer

The designer uses a stable left section index and a central form workspace:

1. Identity and catalog metadata
2. Agent prompt, instructions, and variables
3. Input and output schemas
4. Model policy
5. Skills, Plugins, and explicit Tool Bindings
6. Shell and filesystem permissions
7. Network and credential policy
8. Approval rules
9. Runtime limits
10. Evaluation gates

A right-side inspector shows validation, effective permissions, unresolved
references, and change risk. Advanced JSON/YAML source view is available, but
it uses the same schema and validation as the form.

React Flow is reserved for an optional policy-flow view and future multi-agent
workflows. A single-agent definition must remain understandable and editable
without manipulating a graph.

### Review and Publication

The review screen groups differences by behavior and risk:

- Employee-visible changes
- Instructions and model routing
- Added or removed tools
- Broadened or narrowed permissions
- Approval changes
- Resource and network changes
- Evaluation results

Security-sensitive expansions are visually prominent and require explicit
review. The publish action opens a confirmation dialog containing target
workspaces, employee groups, version, unresolved warnings, and rollback target.

### Run Explorer

The run page uses a two-column operational layout:

- Main: chronological event timeline with streamed assistant output, structured
  rationale, tool calls, approval boundaries, errors, and artifacts.
- Side panel: immutable configuration references, actor, resource use, model
  route, policy decisions, sandbox identity, and timestamps.

An Xterm.js panel is available to authorized administrators. It displays the
actual PTY stream and starts read-only. Interactive input requires a separate
permission, a visible session indicator, and an audit event for every attached
operator session.

Raw chain-of-thought is never requested or displayed. "Reasoning" in the UI
means model-provided concise rationale summaries, plans, and observable actions.

### Evaluation Workspace

The workspace uses `Suites`, `Runs`, and `Regressions` views. Suite and run pages
compare baseline and candidate results using aligned tables and diffs. They
separate deterministic assertion failures from probabilistic score changes.
Cost, latency, policy, VCR mismatches, filesystem deltas, and database deltas
are independently filterable. Agent Evaluations reuses the same result
projection with a fixed Agent filter.

### Integrations

The Integrations area manages enterprise systems that invoke agents through the
server-to-server API. It includes:

- Registered clients, owners, environment, status, credential metadata, and
  recent activity.
- Agent publications with input/output contract versions, delegated-subject
  requirements, scopes, quotas, artifacts, event projection, and retention.
- Webhook endpoints, signature-key rotation, subscribed events, delivery
  attempts, response classes, and explicit redelivery.
- Access revocation, publication expiry, and audit history.

Client credentials and private keys are never displayed after provisioning.
The Admin UI shows identifiers, fingerprints, expiry, and rotation state. Test
invocation uses non-production data and an explicit development publication.

### Audit Explorer

Audit is one cross-resource Admin explorer, not a repeated tab on every
resource. It searches immutable events by resource, actor, scope, outcome,
correlation ID, linked Run/Revision/Policy Version, and time. Resource pages
show only a compact Recent activity slice and link to the same explorer with
resource filters encoded in the URL.

Audit detail is read-only and shows the event envelope, actor, action, outcome,
linked evidence, redaction state, and authorized export controls. Run timelines,
Policy Version history, and Webhook delivery history remain domain-specific
views; they link to Audit but do not create another audit table or export
system. Export controls are visible only to Organization Administrators,
Security Reviewers, and Auditors within their assigned scope; Operators can
inspect and locate evidence but cannot export it.

### Platform Settings

`/platform/settings` is one route with an explicit `Organization | Workspace`
scope switcher. It does not create a parallel Workspace Settings page. The
selected scope is visible in the page header, and Workspace scope additionally
shows the selected Workspace and the organization bounds that constrain it.

The first viewport is an effective-settings summary: inherited versus overridden
values, validation conflicts, pending deletion or Legal Hold warnings, and a
compact Settings-filtered Recent activity list. A persistent section index leads
to Organization, Retention, Legal Holds, Data Classifications, Limits and
Quotas, and Environments.

Each value displays its source, effective value, organization bound, last actor,
and validation state. Workspace users can reset an override to inheritance, but
cannot widen a bound. Provider budgets, runner capacity, Integration quotas,
credential secrets, and Policy Bindings remain on their owning pages; Settings
links to those pages instead of duplicating their editors.

Settings mutations use section-scoped forms, semantic diffs, explicit save
confirmation, expected-ETag conflict handling, and visible correlation IDs.
`Validate` is side-effect free. Retention deletion is asynchronous and shows
estimated scope, next eligible window, active Holds, blocked records, retries,
and tombstone outcomes. No control is optimistic for security-sensitive or
destructive changes, and all mutation outcomes link to the global Audit explorer.

## 5. Directional Gantry Copilot Information Architecture

Primary navigation:

1. New session
2. Agents
3. Sessions
4. Triggers
5. Approvals
6. Artifacts

Administration concepts are absent. The employee chooses an approved capability
and states an intent; they do not configure model providers, MCP servers,
runners, or credentials.

### New Session

The default screen provides a focused composer plus recently used and favorite
agents. These collections are scoped to the current employee and Workspace;
recent use means a successfully created Session and is limited to eight Agents.
An employee can choose an agent explicitly or, if enabled later, allow
an approved routing policy to recommend one. The first release should not use
an opaque autonomous router.

Agent-specific structured fields appear above the composer when required.
Attachments show upload progress, validation, classification warnings, and
removal controls.

### Agent Catalog

Catalog items show name, description, owner, category, typical inputs,
capability summary, and relevant data/action disclosure. Catalog cards do not
show system prompts, internal tools, model names, or raw permission rules.

### Active Session

The Session screen prioritizes conversation, collaboration, and current Run:

- Conversation or structured result in the main region.
- Compact activity stream for plans, tool activity summaries, and waiting
  states.
- Generated files and links in an artifact region.
- Cancel, retry, and feedback actions when permitted.
- A compact member list, queued Run count, and fixed owner/contributor/viewer
  role controls for the Session owner.

Terminal output is not exposed by default. Agents designed for developer use
may expose a sanitized command-log component, but never an interactive shell.

### Sessions and History

`Sessions` defaults to Sessions owned by the employee. A clear scope control
can switch to `Accessible`, which includes only Sessions where the employee is
the current owner, contributor, or viewer. The server applies the scope before
pagination. Rows show Agent, Session title, mode, members, current Run, queued
count, last activity, the employee's pending action, and Artifact availability.
Filters cover Session scope, lifecycle, mode, Agent, time, and `my action`; they
never expose unrestricted operational Runs.

The Admin Run workbench may offer `View conversation` for a
`workspace_agent_editor`. The action first records the editor's explicit
self-enrollment as a Session `viewer`, then opens the ordinary member-scoped
Copilot view. It cannot target another principal and remains visible in Audit.

Session detail expands a compact Run history showing requester, sequence,
status, start and completion time, and a user-facing failure or retry reason.
Selecting a Run keeps the user in the Session conversation and activity
stream. It does not open Admin runner, lease, credential, raw prompt, or
unrestricted diagnostic data. Retry is a Run command that creates a new queued Run and
returns to the same conversation.

The history and active views share the same reconnectable Session cursor and
Artifact authorization rules. A Session remains active after a Run completes,
fails, or consumes a rejection/expiry result, so the composer remains available
to authorized contributors. At most one Run executes; later instructions are
shown in a stable Session-order queue.

### Triggers

`Triggers` is an owner-only Copilot page for Webhook and scheduled entry points,
not an Admin integration console or workflow designer. Creation selects an
employee-visible Agent and a new or exact owner-bound Session target. Webhook
payload shape and scheduled fixed-input controls come from the Agent's published
input contract; employees do not select Deployments or author JSON Schema.

Webhook endpoint and secret are revealed together only after creation, with the
secret never displayed again outside an explicit rotation result. Scheduled
Triggers show cron, IANA time zone, and the next planned instant. Detail shows
only committed occurrence links to authorized Sessions and Runs; security
telemetry and canonical Audit remain on their owning surfaces.

### Approval Queue

The Copilot queue contains only Agent action approvals from Runs initiated by
the current employee. Each row shows age, agent, action class, risk, target, and
expiry. Business workflow approvals remain in the owning tool or enterprise
system. Bulk approval is prohibited.

### Approval Detail

The employee-facing approval page uses plain business language and includes an
expandable technical payload for qualified users. Approve and reject actions
require a reason when policy specifies one. Only the authenticated Run
requester can decide. Expired or superseded requests are read-only. After
rejection or expiry, the Session shows the outcome and keeps its contributor
composer enabled for a new instruction.

Copilot approval pages are the only decision surface for Agent action approvals.
Admin Run and Audit pages may show the same approval request, decision, expiry,
and outcome as immutable evidence, but never expose an approve or reject
command.

## 6. Realtime and Reconnection Behavior

- Initial page data is fetched over HTTPS.
- Live run events use a reconnectable WebSocket stream with an event cursor.
- The UI acknowledges the last rendered cursor and requests missed events after
  reconnect.
- Frames beyond the last durable cursor are provisional. The client tracks
  stream byte offsets, discards uncommitted provisional output on reconnect,
  and replaces it with committed segment content without duplication.
- On `cursor_expired`, the client replaces local Session/Run state from the server
  snapshot, clearly indicates that older content has expired, and resumes from
  the earliest available cursor.
- Streaming text is batched into short render intervals to avoid excessive DOM
  updates.
- A disconnected banner shows whether the run continues server-side.
- User commands are idempotent and disable only after the server acknowledges
  them, not merely after a click.
- Browser refresh never creates a second Session instruction.

## 7. Empty, Loading, and Error States

Every working view defines:

- Initial loading with stable skeleton dimensions.
- Empty state with the next permitted action.
- Permission-denied state without leaking resource existence.
- Partial-data state when streaming or metrics are unavailable.
- Retriable and terminal errors with a correlation ID.
- Stale-data indicator when the event stream is disconnected.

Errors use specific language. For example, distinguish policy denial, approval
expiry, runner loss, provider failure, tool failure, and validation failure.

## 8. Accessibility and Keyboard Model

- All core workflows are keyboard accessible.
- Tables support logical focus order and do not require horizontal pointer
  scrolling for primary actions.
- Terminal focus capture is explicit and escapable.
- Live updates use restrained ARIA announcements; token-by-token text is not
  announced.
- Status always includes text and an icon, not color alone.
- Motion is reduced or removed under `prefers-reduced-motion`.
- Destructive and irreversible operations require clear confirmation and do
  not rely on icon meaning alone.

## 9. Frontend Technical Direction

- React and TypeScript for both applications.
- A shared package for tokens and accessible primitives, not whole product
  screens.
- React Query or an equivalent server-state library for request caching and
  invalidation.
- Generated API clients and schema validators from versioned contracts.
- React Flow for graph-specific views.
- Xterm.js for authorized Admin PTY inspection.
- Component tests for states and permissions; browser tests for critical flows.
- Localization-ready message keys from the first implementation, even if the
  initial documentation and UI ship in English.

## 10. Required Prototype and Validation Coverage

Before frontend implementation is considered complete, validate at desktop and
mobile widths where relevant:

- Admin agent creation through publication review.
- Admin live run inspection, reconnect, cancellation, and failure diagnosis.
- Copilot requester approval flows plus read-only Admin Run/Audit evidence,
  including expiry and supersession.
- Approval races with cancellation, expiry, duplicate decisions, policy
  revocation, and runner recovery, proving that the UI renders the server's
  winning state and never implies that a stale approval executed an action.
- Copilot Agent discovery, Session creation/instructions, streaming, Artifacts,
  and Run retry.
- Copilot Trigger creation, one-time Webhook secret handling, schedule
  validation, bound-Session selection, lifecycle commands, and occurrence links.
- Permission boundaries proving Copilot cannot access administrator resources.
- Long names, long commands, large payload summaries, localization expansion,
  and reduced-motion behavior.

Gantry Admin is desktop-first but must remain usable on a tablet. Gantry
Copilot is fully responsive for desktop and mobile use.

## 11. Page Design Handoff

Page-level behavior lives in the [Admin Site Design](admin-site-design.md) and
[Copilot Site Design](copilot-site-design.md). The remaining shared work is
traceability and rendered acceptance: a target route is implementation-ready
only when its contract, authorization, handler, tests, responsive behavior, and
[Implementation Status](../delivery/implementation-status.md) entry agree.
