# Gantry Copilot Site Design

## 1. Scope and Status

This document defines the target page and function design for the employee-facing
Gantry Copilot application. It turns the directional Copilot information
architecture in [Frontend UX Design](frontend-ux-design.md) into an
implementation-ready route, state, permission, and acceptance contract.

Delivery labels mean:

- **Current:** a working route or behavior exists in the repository.
- **Next:** the current runtime slice needs the behavior to complete the primary
  user workflow.
- **Later:** useful follow-on capability that does not block the first task
  workflow.

Copilot exposes only the employee's authorized projection of approved Agents,
Tasks, action approvals, events, and Artifacts. It never exposes Admin
navigation, raw Agent specifications, credentials, infrastructure controls,
unrestricted terminal access, or raw chain-of-thought.

## 2. Application Shell

### Primary navigation

The shell keeps the current task workflow one click away and does not expose an
organization or Workspace administration selector.

| Group | Page | Route | Delivery |
| --- | --- | --- | --- |
| Workspace | New task | `/` | Current |
| Workspace | Agents | `/agents` | Current |
| Workspace | My tasks | `/tasks` | Current |
| Governance | Approvals | `/approvals` | Current |
| Workspace | Artifacts | `/artifacts` | Current |

The effective Workspace and employee identity come from the authenticated
projection. If an identity can access more than one Workspace, the API returns
the allowed Agent catalog and a clear context label; Copilot does not expose
Admin-style cross-Workspace aggregation.

### Shell behavior

- Desktop uses a collapsible sidebar; mobile uses a modal navigation drawer.
- `New task` remains a primary command and preserves an optional Agent query
  parameter when navigating from the catalog.
- The shell shows a compact pending-approval indicator when authorized pending
  approvals exist; it does not expose another employee's queue.
- Theme, sign-out, session expiry, and connection status are shared shell
  controls. Authentication errors never reveal resource existence.
- Navigation links are filtered by server-authorized capabilities, but direct
  navigation still receives a non-leaking authorization response.
- A persistent status treatment distinguishes `live`, `reconnecting`, and
  `offline` without implying that a server-side Task stopped.

## 3. Route and Resource Inventory

| Resource | Route | Responsibility | Delivery |
| --- | --- | --- | --- |
| Agent catalog | `/agents` | Browse approved Agents and select one | Current |
| New task | `/` | Compose and submit an idempotent Task | Current |
| My tasks | `/tasks` | Requester-scoped Task history and filters | Current |
| Task detail | `/tasks/:taskId` | Conversation, live state, actions, follow-up input, Run history, and Artifacts | Current |
| Approval queue | `/approvals` | Requester-bound pending action approvals | Current |
| Approval detail | `/approvals/:approvalId` | Immutable action preview and decision evidence | Current |
| Artifact browser | `/artifacts` | Cross-Task Artifact discovery within scope | Current |
| Artifact detail | `/artifacts/:artifactId` | Metadata, scan, and download state | Current; retention projection remains Later |

The standalone browser uses the same Task, classification, scan, and download
authorization contract as embedded Task Detail access.

## 4. Shared State and Interaction Contract

### Task states

| State | Employee meaning | Primary controls |
| --- | --- | --- |
| `queued`, `provisioning`, `running` | The Agent is working | Observe stream, cancel |
| `awaiting_approval` | One concrete Agent action needs the requester's decision | Open approval, cancel; no duplicate decision control in Task detail |
| `awaiting_requester_input` | The Agent needs direction after rejection, expiry, or an explicit question | Continue conversation, cancel if a Run is active |
| `canceling` | Cancellation was accepted and is being reconciled | Observe; no second cancel |
| `suspended` | Execution is paused and resumable by runtime policy | Observe status and reason |
| `completed` | The Task produced a terminal result | Read result, download Artifacts, start a new Task |
| `failed` | The attempt ended without a successful result | Read user-facing reason, retry Task |
| `canceled`, `expired` | The attempt ended without continuing execution | Read reason, retry Task or start a new Task |

The server state is authoritative. The UI never marks an action completed from
an optimistic click, and a stale response cannot move a Task backward.

### Conversation contract

Task Detail renders employee-visible messages and concise observable summaries:

- employee request and follow-up messages;
- Agent result and structured output;
- concise plan or rationale summaries when policy permits;
- tool/action summaries, approval boundaries, and user-facing errors;
- generated Artifact references.

Raw prompts, hidden instructions, raw chain-of-thought, secret values, and
unredacted tool payloads are never rendered. Provisional stream output is
replaced by durable committed content after reconnect without duplication.

### Requester input after rejection or expiry

When an approval is rejected or expires, the Task remains open and transitions
to `awaiting_requester_input` with a visible event explaining the outcome. The
composer stays enabled so the requester can tell the Agent what to change or
what to do next. Sending a follow-up creates a new user message and a new Run
attempt under the same Task; it does not mutate the rejected action or silently
retry it.

While an approval is pending, the composer is read-only by default to avoid
competing instructions. The employee may cancel the active Run or open the
approval detail. Allowing non-conflicting messages during this state requires a
separate concurrency contract.

## 5. New Task Page

### Purpose and layout

`/` helps an employee state an outcome to an approved Agent with the minimum
configuration needed to start a governed Task.

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
- `POST /api/copilot/v1/tasks` receives an idempotency key. A retry after a
  network failure reuses the same key and does not create a duplicate Task.
- Successful submission navigates to `/tasks/:taskId` using the returned opaque
  Task identity.
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
the catalog context while opening a preselected New Task composer. The current
projection includes the published owner display name; typical input/output,
data classification, action disclosure, favorites, and availability restrictions
remain dependent on their missing published metadata fields. Catalog data is a
server-authorized projection and does not reveal internal Tools, prompts, model
names, raw permission rules, or unpublished Revisions.

## 7. Task Detail Page

### Layout

`/tasks/:taskId` uses a conversation-first workbench:

- Main column: request, employee-visible output, follow-up composer when
  allowed, activity summaries, status reason, and Artifact list.
- Side panel: Task identity, current Run attempt, status, timestamps, and
  connection state. It is a compact diagnostic summary, not Admin Run detail.
- On narrow screens the side panel moves below the conversation without
  requiring horizontal scrolling.

### Actions and recovery

- **Cancel run:** available only for an active, cancelable Run; the response
  remains observable until the server reaches a terminal or unknown outcome.
- **Retry Task:** available for failed or canceled attempts; it creates a new
  Run attempt and preserves prior evidence.
- **Continue:** available in `awaiting_requester_input`; appends a follow-up
  message and creates the next Run under the same Task.
- **Open approval:** links to the requester approval without duplicating the
  decision command in the Task view.
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

Approvals contains only Agent action approvals for Tasks where the authenticated
employee is the requester. Leave, expense, purchase, HR, or other business
workflow approvals remain in the owning tool or enterprise system. Admin Runs
and Audit can show immutable evidence but cannot decide the approval.

### Queue

`/approvals` is a focused list with pending count, age, Agent, action class,
risk, target, expiry, and Task link. Bulk approval is prohibited. Empty,
loading, refresh, authorization, and expired-item states are explicit.

### Detail and decision

The detail route shows the exact action digest and immutable request identity,
plain-language action summary, target, risk, policy reason, redacted technical
details behind a disclosure control, expiry, supersession, prior decision
evidence, and linked Task context.

Approve and Reject include the action digest, an idempotency key, and the latest
request version. A stale or duplicate decision renders the server's winning
state and never implies that the action executed. An approved request remains
subject to action-time policy and runtime failure; approval is not execution.

After rejection or expiry, the linked Task shows the outcome and enables the
requester composer. The rejection reason is visible according to policy, and
the new message is a new instruction rather than an automatic replay of the
denied action.

## 9. My Tasks and History

`/tasks` is organized around employee intent rather than infrastructure. Rows
show Agent, task title or first request, current outcome, last activity, pending
requester action, and Artifact availability. Filters cover status, Agent, time,
and requester action; they remain within the employee's authorized visibility.

Selecting a row opens the same Task Detail projection used for live work. The
status, Agent, time, and requester-action filters serialize to the URL and are
applied by the requester-authorized API. A compact Run attempts section can show
attempt number, status, start/completion time, and user-facing failure or retry
reason. It never opens Admin runner, lease, credential, raw prompt, or cross-user
diagnostic data.

## 10. Artifacts

Task Detail lists Artifacts with filename, media type, size, scan state,
classification, availability, and retention notice. Download is enabled only
when the Artifact is available, scan policy passes, the requester is authorized,
and a short-lived download reference is issued.

The later standalone browser preserves Task and classification filters, never
becomes a cross-user file search, and retains the same download and redaction
contract. Deletion or expiry renders a digest-preserving status when policy
permits evidence display, not the removed content.

## 11. Realtime and Command Semantics

- Initial Task and approval data use HTTPS JSON APIs.
- Task streams use a short-lived ticket and, in the target multi-Run contract, an
  opaque Task-level cursor. The current implementation's cursor is Run-bound and
  signals `cursor_expired` when the current Run changes.
- Events are deduplicated by durable sequence and rendered within the
  employee's authorized projection.
- Heartbeats, reconnect backoff, cursor expiry, and ticket expiry are visible
  states rather than silent polling failures.
- Commands use idempotency keys and server reconciliation. Task submission,
  follow-up messages, cancellation, retries, and approval decisions use durable
  key mappings. None of these commands may use optimistic terminal states.
- When a session expires, the page preserves unsent local text where safe but
  does not replay a mutation until the user re-authenticates and submits again.

## 12. Accessibility and Responsive Requirements

- Every Task, approval, status, and stream state has a text equivalent; color is
  never the only status signal.
- Live output uses a polite announcement region and does not steal focus while
  the employee types.
- Approval actions have distinct labels, keyboard order, and confirmation
  state identifying the exact action digest.
- Lists keep stable columns or switch to labeled stacked rows on narrow screens.
  Long IDs and targets wrap or offer a copy action without resizing controls.
- Mobile Task Detail keeps composer, current status, and primary action visible
  without horizontal scrolling.
- Reduced-motion settings disable stream and drawer animation without removing
  state changes.

## 13. Acceptance Criteria

- Copilot exposes only employee-authorized Agents, Tasks, approvals, events, and
  Artifacts; no Admin route is reachable through the Copilot bundle.
- A submitted Task is idempotent, navigates to Task Detail, and renders live
  events with reconnect and cursor-expiry recovery.
- Cancel and retry preserve server authority and create no duplicate attempts.
- Only the authenticated Task requester can decide an Agent action approval;
  business workflow approvals remain external.
- Rejected or expired approvals leave the Task open with a usable requester
  composer. Follow-up input creates a new Run attempt and never replays the
  denied action automatically.
- Approval races, duplicate decisions, expiry, supersession, revocation, and
  runtime failure render the server's winning state.
- Artifact download requires authorization, scan success, availability, and
  retention/classification checks.
- Empty, loading, error, offline, expired, and unauthorized states are designed
  for every primary route.
