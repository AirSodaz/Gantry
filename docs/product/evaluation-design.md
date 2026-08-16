# Evaluation Design

The page-level Admin workflow is in [Admin Site Design](admin-site-design.md).
The typed resource, OpenAPI, and authorization contract is in
[Admin Governed Resource Contracts](../architecture/admin-governed-resource-contracts.md).

## 1. Purpose

Gantry evaluation determines whether an Agent Revision completes representative
tasks correctly, remains within policy, and avoids unintended external effects.
It combines deterministic environment checks with carefully labeled
probabilistic quality scoring.

Evaluation is not production traffic replay against production services.

## 2. Evaluation Modes

### Authored Golden Case

A designer creates inputs, fixtures, assertions, and rubrics explicitly. This is
the preferred source for security, permission, and failure-path tests.

### Sanitized Production Export

An authorized operator selects an eligible successful or failed run. Gantry
creates a derivative candidate, removes or transforms sensitive content, strips
credentials, replaces unstable identifiers, and requires review when policy
demands it. The original run remains unchanged.

### Shadow Evaluation

A later capability may execute a candidate version against copied inputs with
all writes blocked. Shadow results never affect the employee's production task.

## 3. Golden Case Contents

An immutable golden-case version includes:

- Human-readable purpose and owner.
- Input messages, structured fields, and sanitized attachments.
- Agent version compatibility constraints.
- Initial filesystem and database fixtures.
- HTTP replay cassette and matching rules.
- Expected tool and policy events.
- Output schema and content assertions.
- Filesystem and database delta assertions.
- Latency, token, and normalized cost budgets.
- Optional rubric and evaluator configuration.
- Redaction and provenance metadata.

Fixtures and large expected outputs are content-addressed in object storage.

## 4. Evaluation Environment

Each case receives a clean isolated sandbox using the same runner protocol and a
pinned runtime image. Evaluation mode adds:

- No production credentials.
- Default-deny network policy.
- A VCR proxy for supported HTTP/HTTPS traffic.
- Evaluation-aware LLM, tool, credential, and artifact gateways that accept only
  the signed fixture manifest and evaluation lease identity.
- Clean filesystem fixtures on a copy-on-write layer.
- A case-specific fixture database or schema created from a template.
- Deterministic clock, locale, and random seed where tools support them.
- Strict TTL and resource limits.

The environment is destroyed after evidence collection. Cleanup is verified and
reported separately from the case outcome.

Default-deny sandbox networking is insufficient by itself because mediated
tools execute from trusted gateways outside the sandbox. Evaluation identity is
therefore propagated end to end. A gateway must route every mediated operation
to a fixture-backed evaluation adapter and reject an unknown request before
credential resolution, DNS lookup, or connection to a real target. Production
credential references are invalid in an evaluation manifest.

## 5. VCR Proxy Design

### Recording

Recording is an explicit, privileged operation against an approved non-
production environment. The proxy captures request and response metadata and
bodies according to policy, removes credentials, applies redaction, and stores a
canonical cassette.

### Replay

- Requests are matched on method, normalized URL, selected headers, and a
  canonical body matcher.
- Volatile values use declared placeholders and extraction rules.
- Read requests return recorded responses.
- Write requests validate the expected payload and return a recorded or
  synthetic response without reaching the target.
- Unknown requests fail closed with a clear fixture-miss result.
- Redirects and nested calls remain inside the proxy policy.

The initial release supports HTTP/HTTPS. Generic gRPC and arbitrary RPC replay
are deferred until concrete enterprise integrations establish matching and
serialization requirements.

## 6. Filesystem Delta Assertions

The harness captures a normalized initial and final manifest containing paths,
types, permissions where relevant, sizes, and content digests. Assertions may
require:

- File creation, modification, or deletion.
- Exact or pattern-matched text content.
- Structured JSON, YAML, CSV, or archive contents.
- Absence of unexpected files.
- Maximum output size and allowed path roots.

Temporary files and nondeterministic metadata are ignored only through explicit
case rules.

## 7. Database Delta Assertions

Supported fixture databases expose a structured before/after diff. Assertions
target rows and fields rather than raw storage pages. Each case declares tables
and fields to include, stable ordering, ignored generated fields, and expected
inserts, updates, or deletes.

The first implementation should support PostgreSQL fixture databases. Additional
engines are adapters with the same logical delta contract.

## 8. Assertion Types

### Deterministic

- Run terminal status.
- Output schema validity.
- Exact or normalized content.
- Tool calls and canonical arguments.
- Required and forbidden policy decisions.
- Approval boundary occurrence.
- HTTP fixture matching.
- Command exit status and selected output.
- Filesystem and database state delta.
- Artifact existence, type, and digest rules.
- Resource and budget ceilings.

### Probabilistic

- Rubric-based usefulness, correctness, completeness, or style.
- Semantic similarity where exact text is inappropriate.
- Repeated-run success distribution.

Probabilistic results always identify evaluator model and version, rubric
version, repetition count, individual scores, aggregate method, and threshold.
They are not presented as deterministic proof.

## 9. Baseline Comparison

An evaluation run may compare a candidate with the currently published
baseline. The report distinguishes:

- Newly passing and newly failing deterministic cases.
- Policy and side-effect regressions.
- Quality score distribution changes.
- Latency, token, and cost changes.
- New fixture misses and unexpected tool use.
- Cases affected by environment or evaluator changes.

A comparison is invalid if candidate and baseline use materially different
fixtures or evaluator configurations without explicit normalization.

## 10. Publication Gates

Agent policy may require:

- All critical deterministic assertions to pass.
- No new policy violations or unexpected writes.
- A minimum pass rate for designated suites.
- Quality regression within an allowed threshold.
- Cost and latency ceilings.
- Security review when tools or permissions change.

Overrides require a reason, authorized reviewer, expiry or version scope, and an
audit event. An override never edits the evaluation result.

## 11. Production Trajectory Export

Export eligibility is policy-controlled. The pipeline:

1. Freezes the selected run and source references.
2. Classifies event and artifact content.
3. Replaces credentials, personal data, customer identifiers, and environment-
   specific values using deterministic rules where possible.
4. Removes unneeded terminal noise and private content.
5. Generates fixture candidates and explicit unresolved-redaction findings.
6. Requires reviewer acceptance when classification or policy demands it.
7. Creates a new golden-case version with provenance back to the source run.

The exporter must never claim irreversible anonymization merely because strings
were masked. High-risk exports require human review or synthetic replacement.

## 12. Contamination and Safety Checks

- Fixture credentials are non-production and scoped to the case.
- DNS and egress logs prove no unexpected external connection occurred.
- Write-capable requests not present in the cassette fail closed.
- Mediated tool and LLM gateway fixture misses fail before production credential
  resolution or target connection; direct real-target execution is forbidden.
- Evaluation databases are uniquely labeled and inaccessible from production
  applications.
- Cleanup verifies sandbox, volume, database, and temporary credential removal.
- Evaluation outputs cannot be promoted directly to production without the
  normal publication process.

## 13. Evaluation Service Interfaces

The evaluation service owns suite manifests and results. The runner receives a
signed evaluation manifest. The VCR sidecar exposes only its local proxy port
inside the sandbox and sends match results and evidence to the control plane.

The same fixture manifest is enforced by trusted gateways for traffic that does
not originate in the sandbox. Replay evidence identifies the adapter, matched
fixture, request digest, and the fact that no production credential or route was
resolved.

Runner, proxy, fixture database, runtime image, Agent Revision, model policy, and
evaluator versions are recorded so a result can be interpreted later.

## 14. First Implementation Slice

The first complete evaluation slice should demonstrate one realistic agent that:

1. Reads recorded HTTP data.
2. Proposes a write that is intercepted and payload-checked.
3. Runs shell commands in a fixture workspace.
4. Produces a file artifact.
5. Updates a PostgreSQL fixture.
6. Passes output, event, file, database, and policy assertions.
7. Fails clearly when the HTTP request or final state differs.

This vertical slice validates the architecture more effectively than building
many isolated assertion types without one end-to-end case.
