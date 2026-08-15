# Frontend UX Design

This document fixes the shared experience principles and directional
information architecture. It does not yet approve the complete page inventory,
route map, field-level functions, interactions, or page acceptance criteria.
Those Admin and Copilot decisions are the next product-design work package.

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

- Use a neutral base palette with distinct semantic colors for success,
  warning, danger, information, approval, and suspended states.
- Use one accessible sans-serif UI family and a monospace family for commands,
  logs, identifiers, JSON, and diffs.
- Use an 8 px spacing grid and radii no larger than 8 px.
- Use stable heights for toolbars, table rows, tabs, terminal headers, and run
  status bands to prevent layout movement during streaming.
- Use icons from a maintained icon library such as Lucide. Every unfamiliar
  icon-only control has a tooltip and accessible name.
- Avoid nested cards. Major page regions are unframed; cards are reserved for
  repeated catalog items, modals, and tightly scoped tools.

### State Vocabulary

Use the same state names and icons everywhere:

`Draft`, `In review`, `Published`, `Deprecated`, `Queued`, `Provisioning`,
`Running`, `Awaiting approval`, `Suspended`, `Canceling`, `Completed`, `Failed`,
`Canceled`, and `Expired`.

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

Use the platform first. Prefer native browser capabilities and mature,
well-maintained libraries over custom implementations:

- **Layout**: CSS Grid and Flexbox. No custom grid frameworks.
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
experience direction.

Primary navigation uses labeled groups with direct page entries. The labels are
not empty landing pages.

1. Overview
2. Build: Agents, Skills, Plugins, Tools
3. Operate: Runs, Approvals, Evaluations
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
contains active runs, approval backlog, failure rate, runner capacity, recent
policy denials, quarantined agents, and evaluation regressions. Each metric
links to a filtered working view.

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
- Audit

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

### Approval Queue

The queue is optimized for comparison and fast triage. Each row shows age,
requester, agent, action class, risk, target, and expiry. The detail view shows
the exact proposed action, redacted sensitive fields, relevant diffs, policy
reason, prior approvals, and consequences of approval or rejection.

Bulk approval is prohibited in the first release.

### Evaluation Workspace

The suite page compares baseline and candidate results using aligned tables and
diffs. It separates deterministic assertion failures from probabilistic score
changes. Cost, latency, policy, VCR mismatches, filesystem deltas, and database
deltas are independently filterable.

### Integrations

The Integrations area manages enterprise systems that invoke agents through the
server-to-server API. It includes:

- Registered clients, owners, environment, status, credential metadata, and
  recent activity.
- Agent publications with input/output contract versions, application or
  delegated-user authority, scopes, quotas, artifacts, event projection, and
  retention.
- Webhook endpoints, signature-key rotation, subscribed events, delivery
  attempts, response classes, and explicit redelivery.
- Access revocation, publication expiry, and audit history.

Client credentials and private keys are never displayed after provisioning.
The Admin UI shows identifiers, fingerprints, expiry, and rotation state. Test
invocation uses non-production data and an explicit development publication.

## 5. Directional Gantry Copilot Information Architecture

Primary navigation:

1. New task
2. Agents
3. My tasks
4. Approvals
5. Artifacts

Administration concepts are absent. The employee chooses an approved capability
and states an intent; they do not configure model providers, MCP servers,
runners, or credentials.

### New Task

The default screen provides a focused composer plus recently used and favorite
agents. An employee can choose an agent explicitly or, if enabled later, allow
an approved routing policy to recommend one. The first release should not use
an opaque autonomous router.

Agent-specific structured fields appear above the composer when required.
Attachments show upload progress, validation, classification warnings, and
removal controls.

### Agent Catalog

Catalog items show name, description, owner, category, typical inputs,
capability summary, and relevant data/action disclosure. Catalog cards do not
show system prompts, internal tools, model names, or raw permission rules.

### Active Task

The task screen prioritizes outcome and current status:

- Conversation or structured result in the main region.
- Compact activity stream for plans, tool activity summaries, and waiting
  states.
- Generated files and links in an artifact region.
- Cancel, retry, and feedback actions when permitted.

Terminal output is not exposed by default. Agents designed for developer use
may expose a sanitized command-log component, but never an interactive shell.

### Approval Detail

The employee-facing approval page uses plain business language and includes an
expandable technical payload for qualified users. Approve and reject actions
require a reason when policy specifies one. Expired or superseded requests are
read-only.

## 6. Realtime and Reconnection Behavior

- Initial page data is fetched over HTTPS.
- Live run events use a reconnectable WebSocket stream with an event cursor.
- The UI acknowledges the last rendered cursor and requests missed events after
  reconnect.
- Frames beyond the last durable cursor are provisional. The client tracks
  stream byte offsets, discards uncommitted provisional output on reconnect,
  and replaces it with committed segment content without duplication.
- On `cursor_expired`, the client replaces local task/run state from the server
  snapshot, clearly indicates that older content has expired, and resumes from
  the earliest available cursor.
- Streaming text is batched into short render intervals to avoid excessive DOM
  updates.
- A disconnected banner shows whether the run continues server-side.
- User commands are idempotent and disable only after the server acknowledges
  them, not merely after a click.
- Browser refresh never creates a second task submission.

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
- Admin and employee approval flows, including expiry and supersession.
- Approval races with cancellation, expiry, duplicate decisions, policy
  revocation, and runner recovery, proving that the UI renders the server's
  winning state and never implies that a stale approval executed an action.
- Copilot agent discovery, task submission, streaming, artifacts, and retry.
- Permission boundaries proving Copilot cannot access administrator resources.
- Long names, long commands, large payload summaries, localization expansion,
  and reduced-motion behavior.

Gantry Admin is desktop-first but must remain usable on a tablet. Gantry
Copilot is fully responsive for desktop and mobile use.

## 11. Next Page and Function Design Package

The next design pass must produce, for both applications:

- approved sitemap, route inventory, navigation ownership, and permission map;
- one specification per page covering purpose, actors, entry points, data,
  commands, filters, states, validation, errors, destructive actions, and
  responsive behavior;
- cross-page workflows for Agent configuration/publication, task execution,
  approvals, artifacts, operations, and recovery;
- capability-to-page and API-to-page traceability;
- desktop, tablet, and mobile requirements appropriate to each application;
- accessibility, localization, empty/loading/error, realtime, and browser-test
  acceptance criteria.

Existing page descriptions above are inputs to that pass and may be merged,
renamed, split, or removed before implementation commitments are made.
