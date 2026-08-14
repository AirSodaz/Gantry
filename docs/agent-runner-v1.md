# Agent Runner V1

The runner now executes `gantry.agent/v1` manifests. A manifest pins the model
adapter, workspace root, enabled tools, execution limits, checkpoint policy and
local command policy. The control plane still owns assignment, lease fencing,
approval state and durable semantic events.

## Model providers

`scripted` is the deterministic local adapter used by smoke tests. OpenAI
compatible Chat Completions and Anthropic Messages adapters consume streaming
SSE and normalize text, thinking, tool-call and usage events.

Direct provider mode is deliberately development-only. Set
`GANTRY_ALLOW_DIRECT_MODEL=1` and provide the provider key through the process
environment. Keys never enter a manifest, checkpoint, event payload or log.
Production gateway routing remains a follow-up boundary.

## Native tools

`read`, `grep`, `glob`, `write` and `edit` execute in-process. `edit` uses a
content digest tag and line hashes; stale or overlapping edits fail before any
write. Read results include the snapshot artifact needed by a later hashline
edit. All paths are resolved beneath `workspace_root`, and writes use a
temporary file followed by an atomic rename.

Shell execution is disabled unless the manifest enables it. Deny and
interceptor regexes run before a process is created. Linux command execution is
bounded by the manifest timeout, runs in a dedicated process group, and returns
bounded output. The pty-handler also exposes a real portable-pty path for
interactive session owners; non-PTY shell execution is kept separate.

## Rules and context

The runner discovers `AGENTS.md` and `.omp/rules/*.md`, applies deterministic
first-wins precedence, and injects always-apply rules into the system prompt.
Assistant text, thinking and tool arguments are inspected as they stream.
External file, web, shell and MCP results are untrusted data blocks; instruction
override attempts produce `security.untrusted_context` events and high-risk
matches can interrupt a tool call.

Context compaction keeps policy, recent messages, pending actions and artifact
references, then records a digest of the archived summary. When enabled,
checkpoints are AES-GCM encrypted and written atomically. The key comes from
`GANTRY_RUNNER_CHECKPOINT_KEY`; persistence is rejected when no key is present.

## Verification

```text
cargo fmt --all -- --check
cargo test --workspace
go test ./...
pnpm typecheck
pnpm -r build
```
