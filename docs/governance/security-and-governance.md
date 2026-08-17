# Security and Governance

## 1. Security Objectives

Gantry must prevent an agent, employee, compromised runner, or misconfigured
tool from exceeding the authority granted for one concrete action. Controls are
layered because model instructions, shell policies, and network policy can each
fail independently.

Primary assets include enterprise credentials, employee data, agent prompts,
tool results, artifacts, audit evidence, model inputs and outputs, evaluation
fixtures, and production systems reachable through tools.

## 2. Trust Boundaries

| Boundary | Trust assumption |
| --- | --- |
| Browser to public APIs | Browser input is untrusted; OAuth identity is verified server-side |
| Copilot API to Admin API | Separate audiences and permissions; Copilot cannot call Admin resources |
| Enterprise system to Agent Invocation API | Confidential client and required delegated subject are independently verified; only the subject becomes requester |
| Control plane to runner | Runner is potentially compromised and receives least privilege |
| Runner to LLM/API gateway | All requests are authenticated, authorized, limited, and audited |
| Sandbox to network | Egress is denied unless explicitly mediated or allowed |
| Evaluation to external systems | Writes are blocked or replayed; fixtures are untrusted inputs |
| Control plane to secret store | Only the credential broker can resolve credential references |

## 3. Identity

- Use OIDC authorization code flow with PKCE for both web applications.
- Configure separate OAuth clients and access-token audiences for Admin and
  Copilot.
- Give the server-to-server Agent Invocation API its own audience and register
  each enterprise system as a confidential client.
- Map stable provider subjects to internal principals; do not key users by
  mutable email address.
- Support group-to-role mapping and explicit workspace membership.
- Require recent authentication or stronger authentication context for
  high-risk administrative and approval actions when the identity provider can
  express it.
- Service accounts use scoped workload credentials, not shared user tokens.
- Enterprise clients use OAuth client credentials to authenticate transport and
  token exchange for required delegated-user authority. A client credential or
  user ID in request JSON never establishes Run requester identity.
- Runner workloads use short-lived workload identity and mutual TLS.

## 4. Authorization Model

Authorization combines role-based grants with action context.

### Resource Roles

Baseline roles:

- Organization Administrator
- Workspace Administrator
- Workspace Agent Editor (the Admin page role for Agent design; it does not
  imply Workspace Administrator authority)
- Security Reviewer
- Operator
- Auditor
- Employee

The complete resource-capability matrix for Policies, Evaluations,
Integrations, and Platform management is defined in
[Admin Governed Resource Contracts](../architecture/admin-governed-resource-contracts.md).
The matrix is additive to these baseline roles: a role preset never bypasses
organization/Workspace scope, resource state, ETag checks, or action-time
Policy.

Copilot does not reuse this Admin matrix. Its member-bound Session and
requester-bound Run/Approval
Attachment, Artifact, and event authorization rules are defined in
[Copilot Resource Contracts](../architecture/copilot-resource-contracts.md),
including non-leaking direct reads and authorization inside list queries.

### Action-Time Context

Tool execution considers:

- Current user or service principal.
- Organization and workspace.
- Agent and immutable version.
- Tool server, tool, operation, and normalized arguments.
- Target resource and destination.
- Credential binding and delegated identity.
- Data classification and attachment properties.
- Environment, time, risk, budget, and previous approvals.
- Current run state, assignment lease epoch, action revision, and cancellation
  state.

Publishing an agent does not pre-authorize every future call. The concrete
action is evaluated again immediately before execution.

### Permission Intersection

Effective authority is the intersection of:

1. The actor's current permissions.
2. The deployed Agent Revision's maximum permissions.
3. The workspace and organization policy.
4. The tool server's own authorization decision.
5. The runtime and network constraints.

No layer can broaden another layer's authority.

For enterprise invocation, effective authority also includes the registered
client's scopes and the explicit integration publication. Delegated calls add,
rather than replace, the subject user's current authorization intersection.

### Policy Lifecycle and Scope Composition

Each Policy has one mutable Draft and immutable published Versions. Drafts are
never evaluated for runtime authorization. Publishing records an exact Version
ID, content digest, schema, author, message, and validation evidence; activating
that Version is a separate authorized Binding action. Policies have no branch,
merge, or movable-latest runtime semantics.

Organization and Workspace Policies compose by intersection. A lower scope may
add a restriction or select a narrower allowance, but cannot broaden an outer
Policy. Agent Revisions pin exact Policy Versions and remain constrained by the
active organization, Workspace, resource, Deployment, and Integration
Publication boundaries. Each Run Manifest captures all contributing Policy
Version identities and digests.

Decision composition is monotonic: deny dominates requester approval, and
requester approval dominates allow. Sets intersect, numeric limits take the
minimum, and time windows intersect. An uncomposable Binding is rejected rather
than resolved through an implicit priority.

Policy simulation is side-effect free. It may evaluate a Draft or Version and
explain contributing rules, but it cannot execute a Tool, resolve a secret,
create an Approval Request, change a Binding, or produce a reusable execution
permit. Approval Policies configure allow, deny, or requester-approval rules;
they do not create generic approver authority or move approval decisions into
Admin.

### Platform Settings Scope

`/platform/settings` is a single Admin route with an explicit Organization or
Workspace scope. Organization Administrators own organization defaults and
non-negotiable bounds. Workspace overrides are valid only when their effective
value is inside those bounds; they may narrow authority or capacity but never
broaden it. The Settings projection is composed from typed resources and does
not bypass the owning authorization contract for Policies, Integrations,
Providers, Runners, credentials, or Audit.

The UI uses role-oriented actions instead of exposing an Allow/Deny rule
builder. The server still evaluates explicit capabilities for each section and
re-checks the actor, scope, ETag, and action-time Policy on every mutation.
Validation is side-effect free. Accepted, rejected, blocked, and completed
changes are attributable Audit events with correlation IDs. Retention deletion
is asynchronous and re-checks active Legal Holds, minimum Audit retention,
classification, and key-destruction eligibility at execution time.

## 5. Agent Spec Governance

Agent specs are untrusted configuration until validated and published.

- Mutable Draft working copies cannot execute. Committed Revisions may execute
  through test Deployments restricted to development policies and credentials.
- Agent Revisions are immutable, hash-identified, and content-addressed.
- Permission expansion, new credential classes, broader egress, weaker approval
  rules, or new write-capable tools require security review.
- Review and publication bind the full Revision hash, content digest, exact
  reviewer set, comparison base, and evaluation evidence.
- Emergency quarantine prevents new runs and may cancel active runs according
  to incident policy.
- Rollback moves the Production Deployment pointer to an earlier approved
  Revision; it does not delete or rewrite the faulty Revision or its history.
- Agent Prompt Snapshots, imported Skill Artifacts, Plugin Versions, Tool
  Descriptor Versions, CLI Command Profiles, and Tool Bindings are immutable once
  referenced by an Agent Revision.
- A Skill may require an explicitly bound tool but cannot grant a permission,
  credential, destination, or operation by itself.

### Agent-Scoped Authorization

Agent access is default-deny and separately controls:

- safe metadata discovery;
- raw configuration and Prompt/Skill/Plugin/Tool visibility;
- Draft editing and Revision commits;
- Review decisions;
- test Deployment management;
- Production publication and rollback;
- run and artifact inspection;
- Copilot or integration execution;
- Agent ACL management.

The initial Agent ACL is Allow-only. No explicit Deny grant is stored or
evaluated. Absence of a capability is denial; revocation removes the Allow
grant. When a granted capability is blocked by organization/workspace
membership, Deployment, integration publication, quarantine, or Policy, the
decision explains that outer constraint instead of adding a conflicting Deny
record. Emergency execution stops use quarantine or a Deployment state.

An Agent ACL grant never replaces organization or workspace authorization. The
effective permission is the intersection of the current principal, organization
and workspace roles, Agent ACL, resource state, Deployment, integration
publication, and action-time policy. `execute` does not grant configuration
read, and configuration read does not grant execute.

Execution additionally requires an active authorized Deployment and the current
policy, credential, destination, model, approval, and data-classification
checks. Removing an execute grant blocks new work while preserving historical
run and audit evidence. Configuration responses are redacted or denied when
`configuration.read` is absent.

Agent ACL changes, effective-permission decisions, denied reads or executions,
and emergency revocations are attributable audit events. An access manager
cannot remove the last organization-authorized recovery path without a
break-glass or owner-transfer procedure.

The typed grant, owner-transfer, recovery, and persistence contract is defined
in [Agent Access Contracts](../architecture/agent-access-contracts.md).

## 6. Credentials

The runner and model never store durable enterprise or provider secrets.

### Credential References

Agent specs contain logical references such as `crm-read-as-user` or
`llm-standard-confidential`, not secret material. The credential broker resolves
these references only after authorization.

### Credential Modes

- **Platform identity:** a narrowly scoped service identity for shared actions.
- **Delegated user identity:** an on-behalf-of token bound to the current user
  and target service.
- **Ephemeral generated credential:** a short-lived credential created for one
  run or action.

The selected mode is visible in approval and audit records without revealing
the credential.

### Injection

The preferred design is a trusted egress gateway that injects credentials after
receiving an authenticated action envelope. When a protocol cannot be proxied,
the broker may issue a very short-lived, audience-bound credential to a tool
sidecar, never to the model prompt or terminal environment.

Secrets are redacted before events, logs, VCR fixtures, or artifacts are
persisted. Redaction is a defense layer, not justification to log secrets first.

## 7. Tool and MCP Security

- MCP servers are registered with owner, transport, version, schema digest,
  trust level, data classification, supported auth modes, and effect metadata.
- Agent Revisions pin the server and tool descriptor digest.
- Tool input is schema-validated and canonicalized before policy evaluation.
- Tool output has size, type, and content limits and is treated as untrusted
  model input.
- Tool descriptions and results may contain prompt injection; system policy
  remains authoritative and tool output is labeled by provenance.
- Remote MCP servers are reached through controlled gateways where possible.
- Dynamic tool discovery cannot silently add tools to a published agent.
- Discovery creates a proposed immutable descriptor version that requires
  review and activation before it can be selected by a draft.
- An Agent Tool Binding may narrow a descriptor's operations, schemas,
  credentials, destinations, classifications, approvals, and limits, but cannot
  broaden any of them.
- Tool Server connection secrets remain behind trusted adapters and are never
  returned by Admin APIs.

### Skill Security

- Skill prompt fragments are content-addressed, provenance-labeled, and treated
  as configuration requiring review.
- Imported or generated Skill content is untrusted until validated and
  activated.
- A Skill cannot override system policy, sandbox rules, Tool Bindings,
  credentials, network policy, approval requirements, or runtime limits.
- Agent Revisions pin the exact Skill source reference, package-declared version,
  and content digest; importing another artifact or updating the external
  package never changes existing Agents.

### Plugin Security

- Organization installation requires publisher, provenance, dependency,
  descriptor, permission, and compatibility review.
- Workspaces may enable multiple exact Plugin Versions explicitly; upgrades
  create a new reviewable enablement choice and never replace an existing
  enablement implicitly.
- Agents select contained Skills and Tools individually. Install, enablement, or
  upgrade never auto-binds a newly introduced capability.
- Plugin configuration and MCP connection secrets remain behind trusted
  adapters and are not included in Agent prompts or browser responses.

### CLI Security

- Production CLI access uses registered Command Profiles with executable
  identity, structured argument schemas, filesystem scope, environment
  allowlists, image requirements, and execution limits.
- Model-generated free-form shell text is not a production Tool Descriptor.
- Structured arguments are rendered without shell concatenation. When a shell
  grammar is explicitly required, the profile names the shell and policy parses
  that grammar before execution.
- Development shell access remains an explicit Runner capability and does not
  imply that a production Agent may bind arbitrary commands.

## 8. Shell and Filesystem Security

Command controls are defense in depth, not a complete sandbox boundary.

- Prefer structured built-in operations over shell commands.
- Command policy supports executable allowlists, denied executables, argument
  constraints, working-directory constraints, environment allowlists, and
  maximum duration and output.
- Shell parsing is performed using the actual shell grammar where policy must
  inspect pipelines or redirection; substring matching is insufficient.
- Published specs choose the shell and image explicitly.
- The runner uses a non-root account and cannot access the host container
  runtime socket.
- Filesystem mounts are explicit, minimal, and read-only by default.
- Artifact export follows path, size, type, malware-scan, and data-loss policy.

## 9. Network Security

- Sandbox ingress is denied.
- Sandbox egress is denied by default.
- Control-plane, LLM, credential, artifact, and tool gateways are addressed
  through explicit service identities and network policy.
- Direct egress exceptions require destination allowlists, port and protocol
  constraints, DNS-rebinding protection, private-address controls, and an
  agent-version review.
- HTTP redirects are re-evaluated against destination policy.
- Metadata services, cluster administration endpoints, loopback side channels,
  and internal control-plane endpoints are unreachable from the sandbox unless
  explicitly required.

## 10. Human Approval

The approval model distinguishes agent action approval from business workflow
approval. An agent action approval binds to an immutable action digest containing the run, tool,
operation, canonical arguments, target, credential mode, policy version, and
expiry. Changing any bound field invalidates the approval.

When a human confirmation is required, the authenticated Run requester is the
only eligible approver. Gantry does not route Agent action approvals to generic
approver roles, groups, thresholds, or administrators. A published policy may
allow or deny an action without human confirmation, but it cannot nominate a
different human approver. Approval requests have risk-based expiry and may
require a reason.

Business workflow approvals remain in the tool or enterprise system that owns
the business process. Gantry stores an external approval reference and accepts
only a signed, idempotent callback that is bound to the same action digest; the
Gantry Admin application is not a universal business-approval inbox.

The external wait begins only after the requester's exact Gantry action has
been approved and its single-use execution permit has been consumed by the Tool
gateway. A pending Tool result suspends the same Run with
`external_business_approval`; a valid callback records the business outcome and
resumes the next Agent loop. The callback never approves a Gantry action,
replays the original Tool invocation, or reuses its permit. New effects require
new action digests and current authorization. The full contract is in
[External Business Approval Callback Contracts](../architecture/external-business-approval-contracts.md).

Before executing an approved action, Gantry verifies that the run is still at
the same pending action, the approval is unused and unexpired, the deciding
identity is still the authenticated Run requester, and current policy still
permits execution.

An approval decision changes only the durable action state; it never calls the
tool directly. The tool gateway atomically claims the exact action digest using
its expected revision and current lease epoch, then issues a single-use
execution permit. Approval, rejection, expiry, cancellation, policy revocation,
lease loss, and resume transitions race through the same compare-and-swap state
machine. Only one transition wins, and losing requests return the resulting
state without creating an effect.

Decision append, approval projection revision, and the bound action transition
are one transaction. A uniqueness constraint makes a repeated requester
decision idempotent, and only one terminal decision may make the action ready or
reject it.

Rejecting an Agent action or allowing its approval to expire denies only that
immutable action digest. The Agent loop resumes with a structured
`action_denied` or `approval_expired` result, and the Session conversation remains
available for new requester input. The requester may tell the Agent how to
revise the action or what to do next. Any later consequential action has a new
digest and requires its own authorization and, when required, approval.

The first agent loop serializes consequential actions and allows only one
pending or executing action. This product restriction reduces ambiguity but is
not treated as a concurrency primitive: multiple control-plane replicas,
duplicate requests, delayed runner messages, and failover still require the
atomic protocol. Future parallel agent loops must use independent action records
and explicit conflict domains, resource locks or optimistic preconditions, and
approval scopes; one approval may never authorize a different concurrent
action.

Permit consumption is the execution linearization point. Cancellation or
revocation that commits before it prevents execution; after it, Gantry can
request interruption but must preserve an observed or unknown external outcome.
Audit and UI language must not label that action safely canceled without proof.
Unknown outcomes have a bounded reconciliation deadline and fail the run with a
distinct reason if proof cannot be obtained.

Within Copilot, either the immutable Run requester or the current Session owner
may issue cancellation. Session-owner cancellation is a restrictive control: it
does not grant approval, Agent `execute`, retry, or Tool authority. Only the Run
requester may approve the exact action digest.

## 11. Audit and Event Integrity

Audit records cover authentication, authorization decisions, configuration
changes, publication, Session/Run commands, model routes, tool calls, approvals,
operator terminal attachment, artifact access, exports, and emergency controls.

The canonical Audit projection is cross-resource and append-only. Resource
pages may show a bounded Recent activity slice, but they do not own separate
Audit stores or alternate evidence semantics. Run timelines, Policy Version
history, and Webhook delivery attempts remain domain projections that link to
the canonical Audit event and preserve their operational detail.

- Events are append-only through application APIs.
- Each event contains actor, subject, action, resource, outcome, timestamp,
  request ID, and policy/version references.
- Per-run events form a hash chain. Periodic signed checkpoints anchor batches
  so later modification is detectable.
- High-frequency model and PTY content is stored in immutable segments. Segment
  events hash their offset range and content digest. Compaction uses a manifest
  of source digests and never rewrites signed chain entries; retained-content
  deletion leaves hash-preserving tombstones.
- Database administrators remain a privileged threat; high-assurance deployments
  may replicate checkpoints to external write-once storage.
- Audit export is a separate capability from Audit read. Organization
  Administrators, Security Reviewers, and Auditors may export within their
  authorized scope; Operators and other read-only actors cannot export merely
  because they can inspect events. Export creation and download reapply scope,
  classification, and redaction checks, never include secret values or raw
  chain-of-thought, and are themselves append-only Audit events.
- Export packages are asynchronous, digest-bound, independently expiring
  resources. A failed package may be retried without changing its query scope;
  changing the scope creates a new export request.

## 12. Data Protection and Retention

Data is classified at workspace, agent, attachment, tool, and artifact levels.
Retention policies distinguish operational metadata, prompts and outputs,
terminal streams, artifacts, evaluation fixtures, and audit evidence.

Organization retention policy defines the permitted bounds for each class;
Workspace settings may select values within those bounds. Exact durations remain
deployment configuration pending Legal and Security approval. Audit metadata and
integrity checkpoints have a minimum retention floor, while prompts, outputs,
terminal streams, Artifacts, and Evaluation fixtures may have shorter periods.

- Encrypt transport and storage using managed keys where available.
- Support workspace-specific retention within organization limits.
- Legal hold prevents scheduled deletion of selected evidence.
- Deletion creates tombstone events and verifiable cleanup jobs; immutable
  compliance records retain only the minimum required metadata.
- Deletion re-checks active Legal Holds, minimum Audit retention, classification,
  and key-destruction eligibility immediately before execution. Blocked records
  remain pending and expose the Hold or retention reason.
- Production-to-evaluation export runs deterministic and configurable redaction,
  followed by a human review when classification requires it.

## 13. Model Data Governance

Model policies declare permitted providers, regions, model classes, retention
terms, data classifications, maximum context, tool-use support, budget, and
fallback order.

- Data classification may prohibit a provider or fallback.
- Provider fallback never silently crosses a data-governance boundary.
- Prompt and response content is excluded from ordinary infrastructure logs.
- Gantry stores structured rationale summaries only when enabled by policy.
- Private chain-of-thought is neither requested as a product feature nor
  presented as an audit source.

## 14. Operational Security Controls

- Quarantine an Agent Revision, tool server, credential binding, provider, or
  runner pool.
- Revoke workload identities and rotate signing keys.
- Set global and workspace concurrency and budget limits.
- Detect anomalous tool denial, egress denial, credential failure, output size,
  approval, and Run-volume patterns.
- Run dependency, container, secret, and infrastructure-as-code scans in CI.
- Produce an SBOM and sign release images and binaries.
- Require tested backup restoration and key-recovery procedures before general
  availability.

## 15. Security Verification Gates

Before production pilot:

- Prove Copilot tokens cannot call Admin APIs.
- Prove enterprise client tokens cannot call Admin or browser Copilot APIs and
  cannot invoke agents not published to that client.
- Prove an enterprise client cannot impersonate an employee through Session input
  or switch a delegated token's subject.
- Verify webhook signature, replay, destination, redelivery, and payload-
  minimization controls.
- Verify scheduled Trigger cron and IANA time-zone validation, daylight-saving
  gap/overlap behavior, skip-only misfire recovery, schedule-revision fencing,
  and duplicate occurrence suppression.
- Verify Webhook Trigger creation and rotation reveal each plaintext secret only
  once, command replay returns no secret, rotation immediately invalidates the
  previous key version, and no browser storage, logs, Audit, Session, Run, or
  event payload retains it.
- Prove invalid signatures, retired keys, schema failures, and other rejected
  Webhook requests cannot claim or poison a `(trigger_id, event_id)` occurrence;
  the same unaccepted event may succeed after correction with a current key.
- Prove runner compromise cannot resolve stored credentials directly.
- Prove default-deny egress blocks unauthorized destinations and redirects.
- Exercise command-policy bypass attempts and filesystem escape attempts.
- Test approval replay, substitution, expiry, revocation, and race conditions.
- Test approval versus cancellation, policy revocation, lease loss, durable
  resume, duplicate delivery, and concurrent execution claims; exactly one
  action permit may be consumed.
- Verify cancellation terminates descendant processes and cleanup completes.
- Verify event tampering is detected by hash-chain validation.
- Perform a scoped threat model and external penetration test of public and
  runner-facing interfaces.
