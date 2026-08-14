# Gantry Engineering Constraints

## Code Structure

Keep the codebase clean by default and continuously. Do not add glue layers,
god files, transitional compatibility paths, or migration code when a clean
current-state design is available. Organize code by stable ownership and use
case, keep boundaries explicit, and remove superseded paths in the same change.

Before extending an existing module, split it when the new responsibility would
mix transport, persistence, domain transitions, infrastructure wiring, or
development-only fixtures. Prefer a clear replacement over a minimally scoped
patch that leaves avoidable complexity behind.
