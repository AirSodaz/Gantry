# ADR-028: Policy Lifecycle and Scope Intersection

## Status

Accepted.

## Decision

Gantry presents all typed Policy resources in one Admin catalog. A Policy has
one mutable Draft, immutable published Versions, and explicit Bindings. It does
not have named Drafts, branches, merge, rebase, or a movable latest-Version
pointer used at run time.

Publishing validates the Draft and creates an immutable Policy Version with an
exact ID, content digest, schema version, author, required message, and evidence.
Publishing does not activate the Version. A separate authorized Binding applies
one exact Version to an organization, Workspace, governed resource, Deployment,
or Integration Publication. Agent Revisions directly pin exact Policy Versions,
and Run Manifests preserve every contributing Version identity and digest.

Organization and Workspace Policies compose by intersection. A lower scope may
add restrictions or select a narrower allowance, but it cannot broaden an outer
Policy. Other authority boundaries, including Agent access, Tool Descriptors,
Tool Bindings, credentials, destinations, runtime constraints, Deployments, and
Integration Publications, remain part of the effective intersection.

For a concrete action, decision composition is monotonic:

- any applicable deny produces deny;
- otherwise any applicable requester approval produces require approval;
- only when all applicable layers allow does the result become allow.

Set-valued permissions use set intersection. Numeric limits use the minimum
permitted value, time windows use the intersection of windows, and boolean
restrictions use the more restrictive value. A policy document that cannot be
composed under these rules is rejected at Binding time; the server never resolves
the conflict by broadening authority or by choosing a hidden priority.

Policy simulation may compare a Draft or exact Version with current effective
Bindings and explain `allow`, `deny`, or `require_requester_approval`. It is
side-effect free: it cannot execute a Tool, resolve a secret, create an Approval
Request, change a Binding, or produce an execution permit.

Approval Policy is one Policy type. It configures whether a concrete Agent
action is allowed, denied, or requires approval from the authenticated task
requester. Pending Agent action approvals remain in Copilot. Admin exposes only
configuration and read-only approval evidence, and business workflow approvals
remain in the owning tool or enterprise system.

## Consequences

The UI can use one predictable Overview, Draft, Versions, Bindings, Simulation,
and Audit structure across Policy types. Runtime decisions remain reproducible
because no Draft or floating Version reference can affect execution. Workspace
administrators can tailor stricter local rules without weakening organization
guardrails, and Policy configuration remains separate from both approval
decision handling and external business processes.
