# ADR-025: Flat Agent Revisions and Deployment Pointers

## Status

Accepted.

## Decision

An Agent owns multiple independent named Drafts. Each Draft contains one mutable
working copy, an optional `derived_from_revision_hash` for provenance, and a
reference to its latest committed Revision.

Committing creates an immutable Agent Revision identified by a cryptographic
revision hash and a required human-authored message. It also records the author,
timestamp, source Draft, canonical configuration, and a separate content digest.
The revision hash identifies the snapshot and commit metadata; the content
digest identifies executable behavior.

Drafts and Revisions form a flat history. Gantry does not model parent
Revisions, branches, forks, ancestry, merges, rebases, or cherry-picks. Creating
a Draft from a Revision copies its configuration and records optional
provenance, but does not create a graph relationship. Any two Revisions may be
compared directly.

Mutable working copies may be validated but cannot execute, enter review, or be
deployed. Test runs, evaluation runs, reviews, and Deployments bind one exact
Revision and its content digest.

An Agent may have multiple named test Deployments pointing to different
Revisions. It has one default Production Deployment in its workspace.
Publication and rollback move Deployment pointers without creating, changing,
or deleting Revisions. Integration publications may separately pin an exact
Revision without changing the default Production Deployment.

## Reason

Parallel Drafts and multiple test Deployments provide the debugging and
experimentation value the product needs. A tree-shaped history would create an
expectation of complete source-control behavior and require conflict resolution
for prompts, permissions, tools, policies, and schemas where automatic merging
is difficult to explain and unsafe to imply.

A flat immutable history keeps review, evaluation, execution, rollback, and
audit evidence exact while making the Admin workflow understandable to users
who do not need Git concepts.

## Consequences

- Editors can create independent Drafts from any Revision and compare arbitrary
  Revisions.
- Combining work from multiple Drafts is deliberate: an editor compares content
  and recreates or copies selected changes into one Draft.
- Review risk is normally computed against the current Production Revision, not
  an inferred ancestor.
- Two Revisions may have different revision hashes but the same content digest
  when their message, author, or timestamp differs without changing behavior.
- The current single integer-revision Draft and monotonic published-version
  implementation is a limited slice. The target implementation should replace
  that model cleanly rather than introduce a compatibility layer into the new
  domain.
