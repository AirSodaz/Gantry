# ADR-029: Retention and Legal Hold

## Status

Accepted; exact durations deferred.

## Decision

Retention is classified by Audit metadata, operational metadata, prompts and
outputs, terminal streams, Artifacts, and Evaluation fixtures. The organization
defines permitted bounds for each class. A Workspace may choose a value within
those bounds, but the product does not hard-code universal day counts before
Legal and Security approve deployment defaults.

Audit metadata and signed integrity checkpoints remain through a configured
minimum retention floor. Prompts, outputs, terminal streams, Artifacts, and
Evaluation fixtures may have shorter retention. Server-side Audit export
packages are short-lived and follow their own configured expiration.

A Legal Hold identifies an owner, authority basis, scope or selector, affected
data classes, status, and set or release history. An active Hold blocks scheduled
deletion and key destruction for matching content and evidence. Hold creation,
release, and blocked deletion are attributable Audit events.

Hold selectors are bounded typed expressions over scope, resource identifiers,
Session/Run/Artifact identifiers, classification, and time range. They are frozen
when activated. A match preview is an evidence snapshot, while active Holds are
re-evaluated for newly matching data and again at deletion execution; arbitrary
SQL selectors are not supported.

Deletion is explicit and asynchronous. Gantry records the request, calculates
affected and protected records, enters pending state, re-checks active Holds,
minimum Audit retention, classification, and key-destruction eligibility, then
deletes only permitted content and keys. It retains a digest-preserving
Tombstone with scope, classification, reason, and verification state. Deletion,
failure, retry, and completion are all auditable.

Deletion Jobs use requested, evaluating, pending, running, completed, blocked,
and failed states. Only failed Jobs are retryable, and each retry creates a new
attempt while retaining the original evidence.

## Consequences

The product can support enterprise-specific legal and security requirements
without inventing a universal retention period. Audit remains verifiable after
eligible content deletion, while Legal Holds prevent premature destruction.
Workspace administrators receive bounded control rather than an independent
retention policy that can weaken organization requirements.
