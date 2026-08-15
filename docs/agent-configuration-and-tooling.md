# Agent Configuration, Skills, and Tools

## 1. Purpose and Scope

This document defines the target configuration model for Agent-owned prompts,
standalone Skills, installable Plugins, built-in tools, MCP servers, and governed
CLI commands. It covers registration or import, binding, validation,
publication, and run-manifest materialization.

The first implementation slice is configuration-only: Admin can register and
version assets, bind them to an Agent draft, validate the resulting authority,
and publish immutable references. General MCP, CLI, credential, and production
tool execution are separate runtime slices.

Detailed Admin and Copilot page behavior is intentionally deferred to the next
product-design pass.

## 2. Design Principles

- Published behavior is immutable and reproducible.
- Registration does not grant an Agent permission to use an asset.
- A binding may narrow a descriptor but never broaden it.
- Dynamic discovery may propose a new descriptor version but never mutate a
  published Agent version.
- Secrets are logical references and never enter prompts, descriptors, run
  manifests, events, checkpoints, or browser responses.
- Inputs are schema-validated and canonicalized before policy evaluation.
- Runtime authority is the intersection of actor, workspace, Agent version,
  Skill, Plugin, Tool Binding, policy, credential, destination, and current run
  state.
- The runner receives one compiled, signed run manifest. It does not query Admin
  catalogs during a run.

## 3. Configuration Assets

### Agent Prompt and Prompt Snapshot

The system prompt belongs to an Agent Draft working copy and is edited inside
the Agent designer. Gantry does not provide a separate Prompt catalog in the
initial product.

Committing an Agent Revision freezes the prompt text, declared variables,
instruction ordering, classification, provenance, compiler version, and content
digest as an immutable Prompt Snapshot. Reuse across Agents is provided through
Skills rather than shared mutable prompts.

### Skill and Imported Skill Artifact

A Skill is a reusable governed capability package, imported from an external
skills registry or package source such as an `npx skills` marketplace or a
Claude Code skills marketplace, or added manually as a complete package or
local directory. The package remains the source of its version. Gantry does not
create a Skill Draft, Revision, semantic version, or release pointer.

An imported Skill Artifact records the package identity, source type and source
reference, the version declared inside the package, its normalized instruction
and contract content, provenance, validation state, risk metadata, and a
content digest. A manual artifact accepts a complete package or local
directory, records the upload actor and uploaded package digest as its source
reference, and is never edited in Gantry. If the package declares no version, the UI shows
`未声明`; Gantry never invents a fallback version. The declared version is
display metadata; the source reference and content digest are the
reproducibility identity. If two artifacts declare the same version but have
different content, Gantry keeps both and shows the digest difference.

Multiple artifacts of the same Skill may coexist in a workspace. Test Agent
bindings may select different artifacts to compare upstream versions or package
revisions. A Production Agent Revision pins one exact source reference, declared
version, and content digest. Gantry only imports, validates, activates,
deprecates, or retires artifacts; it never increments or rewrites their package
versions.

A Skill does not execute independently and cannot grant access to a Tool. When
an Agent binds a Skill artifact, every required Tool Binding must also be present
and satisfy the declared operation and schema requirements. The Agent binding
may narrow optional behavior but cannot remove a capability required for the
Skill to remain valid.

Skills are workspace-scoped in the first release. Organization-wide reuse is an
explicit publication operation into selected workspaces, not implicit global
visibility.

### Plugin and Plugin Version

A Plugin is a reviewed installation and distribution container that combines
one or more business Skills with optional MCP Tool Servers, Tool Descriptor
Versions, configuration schemas, default binding templates, and presentation
metadata. A Plugin is not itself an executable tool and does not grant runtime
authority.

Plugin Versions are immutable and content-addressed. They declare their
contained assets, dependencies, minimum Gantry capabilities, publisher,
provenance, risk metadata, compatibility, and digest.

Plugin enablement has three explicit stages:

1. An organization administrator installs and reviews a Plugin Version.
2. A workspace administrator enables that version for a workspace.
3. An Agent designer explicitly selects the contained Skills and Tool Bindings
   needed by an Agent.

Installing or enabling a Plugin never binds all contained tools automatically.

### Tool Server

A Tool Server is an organization-owned registration for a tool provider. It
records:

- type: `builtin`, `mcp`, or `cli`;
- owner, environment, status, trust tier, and operational contacts;
- transport and endpoint metadata where applicable;
- supported authentication modes and credential capability references;
- data classifications, destination class, and health metadata;
- discovery policy and the latest observed descriptor set.

Endpoint credentials and private connection material remain behind trusted
control-plane adapters.

### Tool Descriptor Version

A Tool Descriptor Version is immutable and content-addressed. It contains:

- fully qualified name: `builtin::<server>/<tool>` or
  `app::<server>/<tool>`;
- semantic version and descriptor digest;
- input and output schemas;
- effect classification: `read`, `write`, `external_side_effect`, or
  `administrative`;
- idempotency classification: `read_only`, `idempotent`, `compensatable`, or
  `non_repeatable`;
- data input/output classification, credential capability, destination class,
  timeout, and result-size limits;
- deprecation, replacement, and compatibility metadata.

MCP discovery creates a proposed descriptor version. An administrator must
review and activate it before an Agent draft can bind it.

### CLI Command Profile

A CLI Command Profile is a specialized Tool Descriptor for an approved command,
not unrestricted shell access. It pins:

- executable identity and image/runtime requirements;
- argument schema and deterministic argument rendering;
- allowed working-directory modes and filesystem scopes;
- environment-variable allowlist and credential references;
- timeout, output, process, and artifact limits;
- effect and idempotency classification;
- interceptor and denied-pattern policy used as defense in depth.

Arbitrary command text is never accepted as a substitute for a registered
profile in production. Development-only shell access remains a separate Runner
capability guarded by explicit manifest policy.

### Tool Binding

A Tool Binding is Agent-owned configuration that selects one Tool Descriptor
Version and narrows its authority. It defines:

- allowed operations and argument constraints;
- credential reference and permitted credential mode;
- destination and network constraints;
- data classification limits;
- approval policy reference;
- per-call timeout, output, concurrency, and budget limits;
- optional aliases used only for display or prompt compilation.

The canonical fully qualified tool name remains the runtime identity.

## 4. Ownership and Lifecycle

| Asset | Owner | Mutable identity | Immutable execution unit |
| --- | --- | --- | --- |
| Agent Prompt | Agent | Agent Draft working-copy field | Prompt Snapshot in Agent Revision |
| Skill | Workspace | Catalog metadata and imported artifacts | External Skill Artifact (source, declared version, digest) |
| Plugin | Organization, enabled per workspace | Installation and enablement state | Plugin Version |
| Tool Server | Organization | Connection and operational state | Tool Descriptor Version |
| CLI Command Profile | Organization or workspace | Draft policy | Descriptor Version |
| Agent | Workspace | Named Agent Drafts | Agent Revision |

Lifecycle states are `draft`, `active`, `deprecated`, and `retired` for assets
that Gantry authors or publishes. Imported Skill Artifacts do not have a
Gantry-authored draft or release lifecycle; their catalog status is
`available`, `deprecated`, or `retired`. Retiring an artifact prevents new
Draft bindings but does not invalidate historical Agent Revisions or runs.
Emergency quarantine is a separate operational state that can block new
execution of otherwise immutable Revisions.

## 5. Draft and Publication Semantics

An Agent may have multiple named Drafts. Each Draft has a mutable working copy,
an optional source Revision, and a latest Revision reference. The Main Draft is
created with the Agent; additional Drafts may be created from the content of any
Revision. Drafts and Revisions form a flat history without parent, branch,
merge, or rebase semantics.

An Agent Draft working copy contains:

- identity and employee-facing catalog metadata;
- an Agent-owned system prompt and declared variables;
- ordered Skill artifact bindings;
- selected Plugin Version references, Tool Bindings, and CLI Command Profiles;
- model, command, filesystem, network, credential, approval, runtime, artifact,
  and evaluation policies;
- input/output contracts.

Draft references may be edited explicitly. They never float automatically to a
new imported Skill artifact or Tool descriptor version.

Committing a valid working copy creates an immutable Agent Revision with a
cryptographic revision hash, required message, author, timestamp,
canonical content digest, and all resolved configuration references. Test runs,
evaluations, reviews, and Deployments always bind a committed Revision; mutable
working copies cannot execute.

Publication performs one transactionally consistent compilation:

1. Resolve every selected Skill artifact, Plugin, tool, and policy asset to its
   source identity and immutable content digest.
2. Reject missing, inactive, quarantined, incompatible, or unauthorized assets.
3. Verify that Skill tool requirements are satisfied by Agent Tool Bindings.
4. Compute the effective permission intersection and semantic risk diff.
5. Freeze the Agent prompt snapshot and validate its variables, input/output
   schemas, aliases, namespace
   collisions, command policies, credential references, and evaluation gates.
6. Verify the immutable Agent Revision and all resolved references.
7. Record review evidence, warnings acknowledged, compiler version, and the
   resulting specification digest.
8. Move the requested test or Production Deployment pointer to the exact
   Revision.

Publication cannot silently ingest a newly discovered MCP tool, broaden a CLI
profile, update a Skill, or absorb newly added Plugin assets.

## 6. Run-Manifest Materialization

At task assignment, the control plane materializes a signed, expiring manifest
from the immutable Agent Revision selected by a Deployment and the current
authorized runtime context.

- The Agent prompt and selected Skill text are compiled into ordered,
  provenance-labeled rule and instruction snapshots.
- Tool Bindings become pinned runtime tool descriptors and policy references.
- Credential references become gateway capabilities, never plaintext secrets.
- Actor, workspace, publication, policy, budget, runner capability, and
  `lease_epoch` constraints are attached.
- Only the configuration needed for that run is included.

The manifest digest, compiler version, Agent Revision, Prompt Snapshot digest,
Skill package identities, declared package versions, source references, content
digests, Plugin Versions, Tool Descriptor Versions, Tool Bindings, policy
versions, and runtime image digest are recorded for reproducibility.

## 7. Validation

Static validation returns structured findings with path, severity, code,
message, and remediation. Required checks include:

- unresolved or duplicate asset references;
- prompt variables not supplied by input, context, or a declared Skill;
- Plugin not installed by the organization or enabled for the workspace;
- Agent selection of a Plugin tool or Skill not explicitly bound to the Agent;
- Skill requirements missing from Tool Bindings;
- descriptor schema incompatibility or namespace collision;
- binding constraints broader than the selected descriptor;
- write or non-repeatable actions without an applicable approval policy;
- credential capability incompatible with the Tool Server or destination;
- CLI executable, argument, filesystem, environment, or image-policy mismatch;
- classification flow from task input through tool output and artifacts;
- unsupported runner or protocol capability;
- permission broadening relative to the current Production Revision.

Warnings may be acknowledged only when policy allows. Errors always block
publication.

## 8. Runtime Security Invariants

- Tool output, Skill content imported from external sources, and MCP metadata
  are treated as untrusted model context and labeled by provenance.
- The runtime revalidates descriptor digest, action digest, current policy,
  approval, cancellation, and lease epoch immediately before an effect.
- CLI argument rendering uses structured values. It does not concatenate a
  model-generated shell command.
- A Skill cannot request an unbound tool or override system, policy, sandbox,
  credential, or network rules.
- A Plugin cannot auto-bind contained assets or broaden an Agent when its
  installation, enablement, or discovered descriptors change.
- Revocation or quarantine affects new claims and assignments without rewriting
  historical Agent versions.
- Browser APIs return redacted configuration projections appropriate to their
  audience. Copilot never receives Prompt or internal Tool Binding content.

## 9. Planned Admin Contract Surface

The configuration slice adds OpenAPI-owned Admin resources for:

- Skill import, artifact activation, coexistence, and usage inspection;
- Plugin installation, version review, workspace enablement, and contained
  asset inspection;
- Tool Server registration, health, discovery, and descriptor activation;
- CLI Command Profile lifecycle;
- Agent-owned Prompt, Skill, Plugin, and Tool Binding configuration;
- static validation and effective-authority preview.

The first slice does not add public tool invocation endpoints, credential
values, production MCP proxying, or arbitrary CLI execution. Generated clients,
database ownership, domain services, and Admin UI follow the public contract.

## 10. Audit Requirements

Audit records cover asset creation, draft changes, descriptor discovery,
activation, deprecation, retirement, quarantine, Agent binding changes,
validation overrides, review, publication, and rollback. Each record identifies
the actor, resource, old and new digests, workspace or organization scope,
reason, correlation ID, and result.

Runtime events record only the immutable references and execution facts needed
for diagnosis and compliance; they do not duplicate secret or full prompt
content into general-purpose logs.
