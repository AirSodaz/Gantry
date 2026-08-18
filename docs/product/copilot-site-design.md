# Gantry Copilot Site Design

## 1. Scope and Status

This document defines the target page and function design for the employee-facing
Gantry Copilot application. It turns the directional Copilot information
architecture in [Frontend UX Design](frontend-ux-design.md) into an
implementation-ready route, state, permission, and acceptance contract.
Typed resources, HTTP preconditions, stream frames, and server-side recovery are
defined in
[Copilot Resource Contracts](../architecture/copilot-resource-contracts.md).

Delivery labels mean:

- **Current:** a working route or behavior exists in the repository.
- **Next:** the current runtime slice needs the behavior to complete the primary
  user workflow.
- **Later:** useful follow-on capability that does not block the first Session
  workflow.

Copilot exposes only the employee's authorized projection of approved Agents,
Sessions, Runs, owner-bound Triggers, action approvals, events, and Artifacts.
It never exposes Admin navigation, raw Agent specifications, durable credential
values outside an explicit one-time Trigger secret reveal, infrastructure
controls, unrestricted terminal access, or raw chain-of-thought.

## 2. Application Shell

### Primary navigation

The shell keeps the current Session workflow one click away and does not expose an
organization or Workspace administration selector.

| Group | Page | Route | Delivery |
| --- | --- | --- | --- |
| Workspace | New session | `/` | Current |
| Workspace | Agents | `/agents` | Current |
| Workspace | Sessions | `/sessions` | Current; owner-default scope is target behavior |
| Workspace | Triggers | `/triggers` | Later |
| Governance | Approvals | `/approvals` | Current |
| Workspace | Artifacts | `/artifacts` | Current |

The effective Workspace and employee identity come from the authenticated
projection. If an identity can access more than one Workspace, the API returns
the allowed Agent catalog and a clear context label; Copilot does not expose
Admin-style cross-Workspace aggregation.

### Shell behavior

- Desktop uses a collapsible sidebar; mobile uses a modal navigation drawer.
- `New session` remains a primary command and preserves an optional Agent query
  parameter when navigating from the catalog.
- The shell shows a compact pending-approval indicator when authorized pending
  approvals exist; it does not expose another employee's queue.
- Theme, sign-out, session expiry, and connection status are shared shell
  controls. Authentication errors never reveal resource existence.
- Navigation links are filtered by server-authorized capabilities, but direct
  navigation still receives a non-leaking authorization response.
- A persistent status treatment distinguishes `live`, `reconnecting`, and
  `offline` without implying that a server-side Run stopped.

## 3. Route and Resource Inventory

| Resource | Route | Responsibility | Delivery |
| --- | --- | --- | --- |
| Agent catalog | `/agents` | Browse approved Agents and select one | Current |
| New session | `/` | Create a personal Session and submit its first instruction | Current |
| Sessions | `/sessions` | Owner-default history with an explicit member-authorized accessible scope | Current; new scope default is target behavior |
| Session detail | `/sessions/:sessionId` | Conversation, members, queue, live Run, approvals, history, and Artifacts | Current |
| Trigger list and creation | `/triggers` | Owner-only Webhook and scheduled Trigger management | Later |
| Trigger detail | `/triggers/:triggerId` | Configuration, state, one-time secret rotation, and committed occurrence links | Later |
| Approval queue | `/approvals` | Requester-bound pending action approvals | Current |
| Approval detail | `/approvals/:approvalId` | Immutable action preview and decision evidence | Current |
| Artifact browser | `/artifacts` | Cross-Session Artifact discovery within member and classification scope | Current |
| Artifact detail | `/artifacts/:artifactId` | Metadata, scan, and download state | Current; retention projection remains Later |

The standalone browser uses the same Session membership, classification, scan,
and download authorization contract as embedded Session Detail access.

## 4. Shared State and Interaction Contract

### Session and Run states

| State | Employee meaning | Primary controls |
| --- | --- | --- |
| `queued`, `provisioning`, `running` | The Agent is working | Observe stream, cancel |
| `awaiting_approval` | One concrete Agent action needs that Run requester's decision | Requester opens approval; other members see read-only status |
| `canceling` | Cancellation was accepted and is being reconciled | Observe; no second cancel |
| `suspended` | Execution is durably paused, including a Tool-owned external business approval wait | Observe status and reason; business approval remains in the owning system |
| `completed` | The Run produced a terminal result | Read result, download Artifacts, continue the Session |
| `failed` | The Run ended without a successful result | Read reason, retry Run, or add a new instruction |
| `canceled`, `expired` | The Run ended without continuing execution | Read reason, retry Run, or continue the Session |

The server state is authoritative. The UI never marks an action completed from
an optimistic click, and a stale response cannot move a Run backward. Session
lifecycle is only `active` or `archived`; execution status is always a Run fact.

### Conversation contract

Session Detail renders employee-visible messages and concise observable summaries:

- employee request and follow-up messages;
- Agent result and structured output;
- concise plan or rationale summaries when policy permits;
- tool/action summaries, approval boundaries, and user-facing errors;
- generated Artifact references.

Raw prompts, hidden instructions, raw chain-of-thought, secret values, and
unredacted tool payloads are never rendered. Provisional stream output is
replaced by durable committed content after reconnect without duplication.

### Continued input and queueing

When an approval is rejected or expires, its Run records the structured outcome
and the Session stays active. The composer remains enabled for owners and
contributors so they can tell the Agent what to change or do next. Every
accepted instruction creates a queued Run and never mutates or silently retries
the denied action.

A Session executes at most one Run. Instructions submitted while another Run is
running, awaiting approval, or suspended are shown in Session order as queued.
They cannot overtake the active context. The UI identifies each Run requester;
only that person receives approval controls.

## 5. New Session Page

### Purpose and layout

`/` helps an employee state an outcome to an approved Agent with the minimum
configuration needed to start a governed personal Session.

- Main region: Agent selector, request composer, attachment controls, and
  submit command.
- Secondary region: recently used/favorite Agents and concise capability/data
  disclosures; it is not marketing copy.
- Structured Agent inputs appear only when the published Agent contract requires
  them. The UI validates shape and classification before submission.
- The composer shows the selected Agent, privacy context, and policy warnings
  that affect submission.

### Submission behavior

- Submit is disabled until an Agent and valid request are present.
- `POST /api/copilot/v1/sessions` receives an idempotency key. A retry after a
  network failure reuses the same key and does not create a duplicate Session or
  first Run.
- Successful submission navigates to `/sessions/:sessionId` using the returned
  opaque Session identity.
- Validation, authorization, classification, rate-limit, and unavailable
  Agent errors remain in the composer with a retryable message.
- The page never lets an employee choose a model, Tool, runner, credential, or
  Policy Version.

## 6. Agent Catalog

Catalog rows or cards show only employee-relevant metadata:

- display name, description, owner or support contact, and category;
- typical inputs and expected output type;
- capability summary, data classification, and action disclosure;
- availability and publication-specific use restriction.

Search and category filters serialize to the URL. Selecting an Agent preserves
the catalog context while opening a preselected New Session composer. The current
projection combines stable Agent identity, Revision-frozen employee metadata,
Deployment-bound availability, and the current requester's Workspace-scoped
favorite/recent-use preference. Favorites are changed from the catalog and
recent use is recorded only after successful Session creation; neither changes
authorization or availability. Catalog data is a server-authorized projection
and does not reveal internal Tools, prompts, model names, raw permission rules,
or unpublished Revisions.

The catalog supports `all`, `favorites`, and `recent` collections. The recent
collection is limited to the eight most recently submitted Agents in the
current Workspace and intersects with search/category filters. A temporary
availability message may explain why an already-linked Agent cannot currently
be submitted, but submission always rechecks the active Deployment.

## 7. Session Detail Page

### Layout

`/sessions/:sessionId` uses a conversation-first workbench:

- Main column: request, employee-visible output, follow-up composer when
  allowed, activity summaries, status reason, and Artifact list.
- Side panel: Session identity and mode, member list, executing and queued Runs,
  timestamps, and connection state. It is a collaboration and compact runtime
  summary, not Admin Run detail.
- On narrow screens the side panel moves below the conversation without
  requiring horizontal scrolling.

### Actions and recovery

- **Cancel Run:** available to the Run requester or Session owner. A queued Run
  is removed from scheduling; an active Run remains observable until the server
  reaches a terminal or unknown outcome. Cancel never grants approval authority.
- **Retry Run:** available to an authorized contributor for a failed or canceled
  Run; it creates a queued Run with the retrying contributor as requester.
- **Continue:** appends an instruction and creates a queued Run whenever the
  Session is active and the member retains Agent `execute` authority.
- **Open approval:** links to the requester approval without duplicating the
  decision command in the Session view.
- **Manage members:** available to the Session owner for inviting, changing, or
  removing non-owner members. A `workspace_agent_editor` has a separate
  self-only command to add itself as a viewer to a Session for an Agent in its
  managed Workspace; it cannot invite, change, or remove another principal.
- **Transfer ownership:** available only to the owner and targets an eligible
  current contributor; bound Triggers owned by the prior owner are disabled.
- **Archive Session:** available only to the owner and blocks later human,
  Trigger, and channel instructions without deleting evidence.
- **Download Artifact:** requests an authorized download reference only after
  scan and availability checks.

Initial load has stable placeholders and a retry action. A stream disconnect
banner identifies whether the server-side Run continues. Reconnect resumes from
the last opaque cursor; `cursor_expired` replaces the local projection with the
server snapshot and clearly marks expired history. Terminal errors include a
user-facing reason, correlation identifier, and next action, never runner
leases, credentials, or private infrastructure details.

## 8. Approval Queue and Detail

### Ownership boundary

Approvals contains only Agent action approvals for Runs where the authenticated
employee is the requester. Leave, expense, purchase, HR, or other business
workflow approvals remain in the owning tool or enterprise system. Admin Runs
and Audit can show immutable evidence but cannot decide the approval.

### Queue

`/approvals` is a focused list with pending count, age, Agent, action class,
risk, target, expiry, and Session/Run link. Bulk approval is prohibited. Empty,
loading, refresh, authorization, and expired-item states are explicit.

### Detail and decision

The detail route shows the exact action digest and immutable request identity,
plain-language action summary, target, risk, policy reason, redacted technical
details behind a disclosure control, expiry, supersession, prior decision
evidence, and linked Session/Run context.

Approve and Reject include the action digest, an idempotency key, and the latest
request version. A stale or duplicate decision renders the server's winning
state and never implies that the action executed. An approved request remains
subject to action-time policy and runtime failure; approval is not execution.

After rejection or expiry, the linked Session shows the outcome and keeps the
authorized contributor composer usable. The rejection reason is visible according to policy, and
the new message is a new instruction rather than an automatic replay of the
denied action.

## 9. Sessions and History

`/sessions` is organized around employee intent and collaboration rather than
infrastructure. Rows show Agent, Session title, personal/shared/channel mode,
members, current Run state, queued count, last activity, the current employee's
pending action, and Artifact availability. Filters cover lifecycle, mode, Agent,
time, and `my action`; they remain within current Session membership.

Selecting a row opens the same Session Detail projection used for live work. A
compact Run history shows requester, Session sequence, status, start/completion
time, Trigger source where applicable, and user-facing failure or retry reason.
It never opens Admin runner, lease, credential, raw prompt, or unrestricted
diagnostic data.

## 10. Triggers

`/triggers` is a lightweight owner-scoped resource page, not an automation
builder or a new authorization domain. It manages only Webhook and scheduled
entry points that create ordinary Runs under the existing Session, requester,
approval, and Policy contracts. Admin users may inspect canonical Audit evidence
within their scope but cannot manage or approve an employee's Trigger from
Admin.

### List and creation

The list shows name, Agent, kind, state, Session target, next scheduled instant
when applicable, last committed occurrence, and last update. Filters cover
kind, state, and Agent. It never aggregates another owner's Triggers.

Creation uses one form with a Webhook/Schedule segmented control:

- The employee enters a name and selects an Agent they can currently execute.
  Copilot never asks for a Deployment, Revision, Policy, Tool, credential, or
  Service Principal.
- Session mode selects either a new personal Session for every occurrence or one
  exact bound Session. The bound picker contains only active Sessions owned by
  the employee that already use the selected Agent.
- Webhook configuration displays the Agent's published input contract as a
  read-only payload shape. The employee does not author a JSON Schema.
- Schedule configuration accepts a validated five-field cron expression, an
  IANA time-zone selector, and fixed input rendered from the Agent's published
  input contract. The form shows the resulting next planned instant before
  submission.
- Submit uses an idempotency key. The server resolves the current executable
  Production Deployment and rechecks Agent `execute`, Session ownership,
  lifecycle, Policy, quota, and input validity in one command.

Successful Webhook creation opens a one-time credential result containing the
endpoint and secret. The secret has a copy command but is never written to URL,
browser storage, logs, analytics, or later API responses. Closing or refreshing
the result discards the reveal. If the creation response was lost, idempotent
replay opens the already-created Trigger without the secret and offers explicit
rotation. Schedule creation returns no secret and opens Trigger Detail.

### Detail and lifecycle

`/triggers/:triggerId` is available only to the Trigger owner and returns a
non-leaking `404` otherwise. It shows employee-safe configuration, state and
reason, Agent, Session target, created/updated time, and a paged list of
committed occurrences linking to their Session and Run. Replayed Webhook calls
do not add another occurrence row, and raw request payloads, failed signatures,
security telemetry, secrets, Policy internals, and Service Principal details
are absent. Webhook detail may show the current key-version number and last
rotation time, but never the key material.

- **Edit:** changes name, Session target, expiry, or kind-specific configuration
  using `If-Match`. Trigger kind is immutable. A scheduled execution-affecting
  edit increments `schedule_revision` and replaces the displayed next instant.
- **Disable/Enable:** controls only future occurrence acceptance. It does not
  cancel an already-created Run. Enable rechecks current Agent and Session
  authority; a schedule starts at its first future matching instant and never
  catches up missed time.
- **Rotate secret:** Webhook only. It requires confirmation that the previous
  key becomes invalid immediately with no overlap window, then presents the new
  secret once through the same non-persistent reveal surface. If the response is
  lost, replay remains redacted and the owner must rotate again with a new
  command key.
- **Open Session/Run:** follows the ordinary member-authorized Copilot route.
  Trigger Detail never expands into Admin runtime diagnostics.

The initial page has no manual `Run now`, shared ownership, transfer, arbitrary
payload mapping, branching workflow, or separate Trigger permission controls.
An employee tests the Agent through an ordinary Session and tests a Webhook by
calling its signed endpoint with a new event ID.

## 11. Artifacts

Session Detail lists Artifacts with filename, media type, size, scan state,
classification, availability, and retention notice. Download is enabled only
when the Artifact is available, scan policy passes, the member is authorized,
and a short-lived download reference is issued.

The later standalone browser preserves Session and classification filters, never
becomes a cross-user file search, and retains the same download and redaction
contract. Deletion or expiry renders a digest-preserving status when policy
permits evidence display, not the removed content.

## 12. Realtime and Command Semantics

- Initial Session and approval data use HTTPS JSON APIs.
- Session streams use a short-lived ticket and an opaque Session-level cursor
  that continues across Runs. `cursor_expired` is reserved for retained
  history or projection replacement, not a normal Run transition.
- Events are deduplicated by durable sequence and rendered within the
  employee's authorized projection.
- Heartbeats, reconnect backoff, cursor expiry, and ticket expiry are visible
  states rather than silent polling failures.
- Commands use idempotency keys and server reconciliation. Session creation,
  instructions, membership changes, cancellation, retries, approval decisions,
  and Trigger creation/lifecycle commands use durable key mappings. None of
  these commands may use optimistic terminal states.
- When a session expires, the page preserves unsent local text where safe but
  does not replay a mutation until the user re-authenticates and submits again.

## 13. Accessibility and Responsive Requirements

- Every Session, Run, approval, status, and stream state has a text equivalent; color is
  never the only status signal.
- Live output uses a polite announcement region and does not steal focus while
  the employee types.
- Approval actions have distinct labels, keyboard order, and confirmation
  state identifying the exact action digest.
- Lists keep stable columns or switch to labeled stacked rows on narrow screens.
  Long IDs and targets wrap or offer a copy action without resizing controls.
- Mobile Session Detail keeps composer, current status, and primary action visible
  without horizontal scrolling.
- Trigger forms preserve labels and validation associations for cron, time zone,
  Session target, and generated Agent input fields. One-time secrets are
  keyboard reachable and never announced outside their focused result region.
- Reduced-motion settings disable stream and drawer animation without removing
  state changes.

## 14. Acceptance Criteria

- Copilot exposes only employee-authorized Agents, Sessions, Runs, owner-bound
  Triggers, approvals, events, and Artifacts; no Admin route is reachable
  through the Copilot bundle.
- A created Session is idempotent, navigates to Session Detail, and renders live
  events with reconnect and cursor-expiry recovery.
- Cancel and retry preserve server authority and create no duplicate attempts.
- Only the authenticated Run requester can decide an Agent action approval;
  business workflow approvals remain external.
- Rejected or expired approvals leave the Session active with a usable
  contributor composer. Follow-up input creates a new queued Run and never replays the
  denied action automatically.
- Approval races, duplicate decisions, expiry, supersession, revocation, and
  runtime failure render the server's winning state.
- A Tool-owned business approval wait renders as `suspended` with a safe
  external-system reason and no Gantry decision controls; a signed callback
  resumes the same Run's next Agent loop without replaying the original Tool
  action.
- Personal, shared, and channel Sessions enforce fixed owner, contributor, and
  viewer roles; membership never grants Agent execution or configuration access.
- Bound Trigger occurrences are visibly queued in Session order, and a replay of
  the same occurrence never creates a second Message or Run.
- Trigger list and detail never expose another owner's resource. Creation asks
  for an Agent rather than Admin Deployment details, Webhook secrets are shown
  only once, and schedule edits display the server-authoritative next instant.
- Trigger disable blocks only future occurrences; occurrence links preserve the
  normal Session/Run authorization boundary and do not become another Audit
  explorer.
- Webhook rotation invalidates the previous key immediately, never reveals a
  secret on command replay, and allows an event rejected under the retired key
  to retry with the same unclaimed event ID after the caller switches keys.
- Artifact download requires authorization, scan success, availability, and
  retention/classification checks.
- Empty, loading, error, offline, expired, and unauthorized states are designed
  for every primary route.
