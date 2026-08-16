# ADR-024: Versioned Agent Configuration Assets

## Status

Accepted.

## Decision

Gantry models standalone Skills, installable Plugins, Tool Servers, Tool
Descriptor Versions, CLI Command Profiles, and Agent Tool Bindings as explicit
governed assets. The system prompt remains Agent-owned and is frozen as a Prompt
Snapshot inside each Agent Revision rather than managed in a separate
Prompt catalog.

Agent Revisions pin immutable content digests for every material
configuration asset. A Skill is an externally versioned prompt package with
declared input/output constraints and required Tool Bindings; Gantry displays
the package's declared version but does not create a separate Skill version
history. Multiple imported artifacts of one Skill may coexist for testing, and
each artifact is identified for execution by its source reference and content
digest. A Skill does not grant tool authority by itself. A Plugin packages one
or more Skills with optional MCP
servers, tool descriptors, schemas, and metadata for reviewed distribution. A
Tool Binding selects an immutable descriptor and may narrow operations,
arguments, credentials, destinations, approvals, and limits, but may never
broaden the descriptor.

Plugins are installed and reviewed at organization scope, enabled explicitly
per workspace, and bound selectively by each Agent. A workspace may enable
multiple exact Plugin Versions for testing or migration, with no implicit
default. Installation or enablement never grants all contained tool permissions
automatically.

MCP discovery may propose new descriptor versions but cannot mutate a deployed
Agent Revision. Multiple descriptor versions for one fully qualified tool name
may be active simultaneously, with no implicit default; each Agent Tool Binding
selects one exact version and digest. CLI integration uses structured, registered Command Profiles rather than
unrestricted model-generated shell commands. The control plane compiles the
selected assets into a signed run manifest; the runner does not query Admin
catalogs during execution.

The first delivery slice covers import, artifact activation, binding,
validation, and publication. General MCP, CLI, credential, and production tool
execution remain separate runtime work.

## Consequences

Committing an Agent Revision becomes the boundary where configuration references
are resolved and frozen. Semantic diffs can identify Agent Prompt, Skill artifact,
Plugin,
Tool, permission, and schema changes precisely. Runtime behavior remains reproducible
without allowing dynamic discovery or catalog changes to broaden a running or
deployed Agent Revision.

The model adds more explicit resources and validation, but avoids placing
connection state, reusable business workflows, command policy, and
Agent-specific authorization into one mutable specification blob.
