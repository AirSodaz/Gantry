# Runner Attachment Materialization Contract

## 1. Scope and Status

This document defines the target private contract for reading requester-owned
Attachments inside a Runner sandbox after an Attachment has passed validation,
scanning, and Session/Run binding.

The contract is not a public Copilot API and is not yet implemented in the
Runner protobuf or session handler. It complements
[Copilot Resource Contracts](copilot-resource-contracts.md), which define
upload, scan, binding, and requester projections. The checked-in Runner
protocol remains authoritative until the target messages described here are
implemented and generated.

## 2. Trust and Authority Boundary

The Runner is a potentially compromised workload. It never receives an object
store credential, object key, presigned URL, upload token, requester token, or
unbounded filesystem path. The Control Plane's Attachment Broker reads bytes
through the ObjectStore adapter and streams them over the mutually
authenticated Runner session.

The broker authorizes every materialization against:

- the authenticated Runner identity and current Runner session;
- the exact Run, current lease epoch, and assignment expiry;
- an immutable Run Attachment Input snapshot;
- Attachment state `available` and scan state `passed`;
- Workspace, Agent Revision, data classification, retention, Legal Hold, and
  action-time Policy constraints; and
- cancellation, quarantine, suspension, and revocation state.

A materialization reference is an opaque capability for one Run and one bound
Attachment. Possessing an attachment ID or a reference from another Run is not
authorization. The reference is never accepted on a different Runner session,
lease epoch, or Run.

## 3. Typed Target Protocol

These OpenAPI-like shapes describe the payload carried by future additive
private protobuf messages. They are not browser-visible JSON resources.

```yaml
RunAttachmentInput:
  type: object
  required: [reference_id, run_id, attachment_id, filename, relative_path,
             media_type, size_bytes, digest, classification, expires_at]
  properties:
    reference_id: {type: string, description: Opaque Run-bound reference}
    run_id: {type: string}
    attachment_id: {type: string}
    filename: {type: string}
    relative_path: {type: string, description: Generated beneath the input root}
    media_type: {type: string}
    size_bytes: {type: integer, format: int64, minimum: 0}
    digest: {type: string, pattern: '^sha256:'}
    classification: {type: string}
    expires_at: {type: string, format: date-time}

AttachmentReadRequest:
  type: object
  required: [request_id, run_id, lease_epoch, reference_id]
  properties:
    request_id: {type: string}
    run_id: {type: string}
    lease_epoch: {type: integer, format: int64}
    reference_id: {type: string}

AttachmentReadStarted:
  type: object
  required: [request_id, run_id, lease_epoch, size_bytes, digest, chunk_bytes]
  properties:
    request_id: {type: string}
    run_id: {type: string}
    lease_epoch: {type: integer, format: int64}
    size_bytes: {type: integer, format: int64}
    digest: {type: string, pattern: '^sha256:'}
    chunk_bytes: {type: integer, minimum: 1, maximum: 262144}

AttachmentReadChunk:
  type: object
  required: [request_id, sequence, offset, bytes]
  properties:
    request_id: {type: string}
    sequence: {type: integer, format: int64, minimum: 0}
    offset: {type: integer, format: int64, minimum: 0}
    bytes: {type: string, format: byte}

AttachmentReadCompleted:
  type: object
  required: [request_id, size_bytes, digest]
  properties:
    request_id: {type: string}
    size_bytes: {type: integer, format: int64, minimum: 0}
    digest: {type: string, pattern: '^sha256:'}

AttachmentReadFailed:
  type: object
  required: [request_id, code, retryable]
  properties:
    request_id: {type: string}
    code:
      type: string
      enum: [reference_invalid, lease_stale, attachment_unavailable,
             policy_denied, classification_denied, quota_exceeded,
             stream_canceled, object_missing, digest_mismatch, expired]
    retryable: {type: boolean}
    message: {type: string}
```

`AssignRun` carries a list of `RunAttachmentInput` records as part of the
future run-input section. It contains safe metadata and a generated relative
path, never an object key or secret. The Runner requests one reference at a
time over the existing bidirectional session. The Control Plane replies with
`AttachmentReadStarted`, zero or more ordered chunks, and exactly one
`AttachmentReadCompleted` or `AttachmentReadFailed`.

The initial protocol does not support arbitrary ranges or resumable offsets.
A retry creates a new `request_id` and restreams from byte zero. This keeps
lease and digest validation simple; range reads require a later decision about
encrypted object ranges, billing, and partial-file cleanup.

## 4. Materialization Flow

1. Copilot Session creation or instruction submission binds uploader-owned,
   `available` Attachments in the same transaction as the Message and Run. A
   bound Attachment cannot be rebound.
2. Run scheduling creates an immutable `RunAttachmentInput` snapshot containing
   the Attachment identity, digest, classification, and generated input path.
3. The assignment includes only the snapshot metadata and opaque references.
4. The Runner creates a temporary file below its input root and sends an
   `AttachmentReadRequest` over the assigned mTLS session.
5. The Broker locks or rechecks the Run lease facts, Attachment state, Policy,
   and reference expiry, then streams bytes through the ObjectStore adapter.
6. The Runner enforces the advertised size limit while writing, computes the
   digest, and atomically renames the temporary file to the offered relative
   path only after `size_bytes` and `digest` match.
7. The Runner reports `AttachmentReadCompleted`. The Control Plane records the
   materialization evidence and the Agent may read only the generated input
   path.

The Agent cannot request a different path, another Attachment, a different
classification, or a second object-store read by changing manifest content.
The manifest and input snapshot are immutable for the Run.

## 5. Stream Safety and Failure Semantics

- Each chunk is at most 256 KiB. The session applies bounded flow control with
  a configured maximum in-flight byte window; a slow Runner applies backpressure
  to the Broker without unbounded buffering.
- `request_id` is unique within a Run. Duplicate requests return the durable
  attempt result when safe; a different reference or expected lease returns a
  conflict and never starts a second stream.
- The Broker checks cancellation, lease epoch, assignment expiry, quarantine,
  and revocation before the stream and between bounded chunk reads. It closes
  the stream with `lease_stale` or `stream_canceled` when any check fails.
- A partial file is never visible under the offered path. The Runner deletes
  the temporary file on failure, disconnect, cancellation, or process restart.
- A missing object, changed object size, or digest mismatch fails closed and
  emits durable evidence; the Agent never receives unverified bytes.
- A Runner disconnect leaves a materialization attempt in `streaming` until a
  short recovery deadline, then marks it `abandoned`. A reconnect may retry
  the full stream under the current lease, but cannot resume the old stream.
- Attachment expiry, deletion, Legal Hold changes, scan revocation, or Policy
  changes block new reads. Historical Session, Run, and Audit evidence remains.

Materialization failures are classified as input/runtime failures, not user
approval outcomes. The Run may fail or suspend according to its normal
recovery policy; it must not silently continue with a missing input.

## 6. Persistence and Recovery

The Attachments and Artifacts module owns the broker and source-object lookup.
The Sessions module owns binding and Run input snapshots. The Runner Session
module owns transport framing, flow control, and session fencing.

The target PostgreSQL records are:

```text
gantry.run_attachment_inputs
  run_id, attachment_id, reference_id
  filename, relative_path, media_type, classification
  size_bytes, digest, expires_at, created_at

gantry.attachment_materialization_attempts
  id, run_id, attachment_id, reference_id, request_id
  runner_id, session_id, lease_epoch, state
  bytes_streamed, result_digest, error_code
  started_at, completed_at, abandoned_at
```

Required invariants:

- `run_attachment_inputs` is immutable once the Run is assigned and has one
  row per bound Attachment; its digest and classification are copied for
  reproducibility, not authorization.
- `reference_id` is unique within a Run and is not globally meaningful.
- A materialization attempt must reference the exact Run input and current or
  superseded lease epoch; a superseded epoch cannot start a new stream.
- Attempt state is `requested -> streaming -> materialized`, or
  `requested|streaming -> failed|abandoned|revoked|expired`.
- Source object keys remain inside the ObjectStore adapter and never appear in
  these records, the manifest, Runner events, logs, or browser responses.
- Reconciliation marks abandoned attempts and removes any staging references;
  it never marks a digest as materialized without a completed Runner report.

The Run assignment transaction records the input snapshot and scheduling
outbox. Materialization does not hold a database transaction open while bytes
are streamed. Start, completion, failure, stale-lease rejection, and object
missing events are committed separately with idempotent request handling.

## 7. Authorization and Audit Boundary

The broker uses typed facts supplied by Sessions, Authorization, Retention, and
Policy services. It does not fetch arbitrary domain tables or trust filenames
or IDs supplied by the Runner. A successful Session instruction or Run assignment
does not grant indefinite object access; every materialization rechecks current
state.

The canonical Audit projection records materialization requested, started,
completed, failed, canceled, stale-lease rejected, digest mismatch, and object
missing outcomes. It records Run, Attachment, Runner/session, lease epoch,
classification, byte count, digest, correlation ID, and error code, but never
bytes, object keys, upload tokens, or secret material. Per-chunk telemetry is
bounded operational metrics, not a second Audit stream.

Copilot sees only the existing Attachment and Session projections. It never sees
Runner session IDs, materialization references, staging paths, object keys, or
lease epochs. Admin Run detail may show redacted materialization status and
Audit evidence when the actor has the required run/configuration visibility.

## 8. Acceptance Contract

- A Runner cannot read an Attachment that is not bound to its exact Run and
  current lease, even when it knows the Attachment ID.
- No object-store credential, object key, presigned URL, requester token, or
  unrestricted path crosses the Runner boundary.
- Scan failure, classification denial, Policy revocation, quarantine,
  cancellation, lease loss, expiry, missing object, and digest mismatch all
  fail closed and leave no visible partial file.
- Duplicate and conflicting materialization requests are idempotent and cannot
  create two successful materializations for one Run input.
- Runner restart and session reconnect require a full, newly authorized stream;
  an old reference or lease cannot be replayed.
- Materialization evidence is durable, redacted, auditable, and linked to the
  exact immutable Run input digest.
- The future protobuf extension is additive and keeps existing Runner event
  sequence, lease fencing, and artifact upload semantics unchanged.
