# ADR-023: Agent Runner Model Routing

## Status

Accepted for Agent Runner V1 development deployments.

This ADR does not select the production-supported provider set. That decision
remains DQ-002 in [Decisions and Open Questions](../decisions-and-open-questions.md).

## Decision

The Linux runner may call OpenAI-compatible Chat/Responses SSE endpoints or
Anthropic Messages SSE endpoints directly only when
`GANTRY_ALLOW_DIRECT_MODEL=1` is explicitly present in the runner process
environment. Provider credentials are read from `OPENAI_API_KEY` or
`ANTHROPIC_API_KEY` and are never part of a manifest, checkpoint, event, or
log payload.

Production configuration must leave direct-provider mode disabled. A trusted
LLM gateway will become the production routing boundary in a later ADR; this
V1 adapter contract keeps that migration additive by normalizing provider
stream events before they reach the agent loop.

## Consequences

Local mock servers and deterministic scripted models can exercise the full
runner loop without credentials. Deployments that omit the explicit opt-in
fail assignment validation for direct providers instead of silently routing
around the gateway policy.
