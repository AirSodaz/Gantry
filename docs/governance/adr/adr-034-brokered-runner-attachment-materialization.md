# ADR-034: Brokered Runner Attachment Materialization

**Status:** Accepted

## Context

Copilot Attachments are requester-owned objects that become eligible for a
Session/Run only after upload, digest, classification, and scan checks. The Runner is
a potentially compromised workload and must not receive object-store
credentials or a reusable URL merely because it was assigned a Run.

## Decision

The Control Plane owns an Attachment Broker. Before assignment it creates an
immutable Run Attachment Input snapshot with safe metadata, a generated
sandbox-relative path, and an opaque Run-bound reference. The Runner requests
materialization over the mutually authenticated Runner session.

The broker rechecks Runner session, Run, lease epoch, assignment expiry,
Attachment binding and scan state, classification, retention, cancellation,
quarantine, and Policy before streaming. The Runner writes to a temporary file,
verifies exact size and SHA-256 digest, and atomically exposes the file beneath
the input root. A stale lease, revoked authorization, missing object, or digest
mismatch fails closed and leaves no visible partial file.

The first protocol version streams from byte zero and does not support arbitrary
ranges or resume. A reconnect creates a new request and a newly authorized full
stream. Materialization attempts and terminal outcomes are durable and
auditable, but object keys, staging paths, bytes, and secrets are never exposed
in manifests, Runner events, logs, or browser projections.

## Consequences

- The Runner Session protocol gains additive read-request, start, chunk,
  completion, and failure messages only when implementation begins.
- Attachments and Artifacts own source-object lookup and broker policy facts;
  Sessions own binding and Run snapshots; Runner Sessions own transport fencing and
  flow control.
- A slow or disconnected Runner cannot force unbounded Control Plane buffering;
  bounded chunks and flow control apply backpressure.
- Replaying an old reference, session, or lease cannot read an Attachment from a
  different Run. Full replay after reconnect costs additional object reads but
  keeps the trust boundary and recovery semantics simple.
