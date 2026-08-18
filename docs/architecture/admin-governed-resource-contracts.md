# Admin Governed Resource Contracts

## 1. Scope and Status

This document is the target contract for the Admin resources that are currently
described only at product level: Policies, Evaluations, Integrations, and
Platform management. It defines the typed resource shapes, state transitions,
OpenAPI route surface, command preconditions, and authorization boundary.

These are design contracts, not claims that the routes already exist. A route
becomes implemented only when it is present in the Admin OpenAPI document, has
an owning handler and persistence path, generates client types, and has focused
authorization and transition tests. The checked-in capability baseline remains
in [Implementation Status](../delivery/implementation-status.md).

The contract uses the Admin audience `gantry-admin-api`. Copilot and Enterprise
Invocation receive projections of these resources where explicitly stated; they
never reuse Admin authorization or response types.

## 2. Common HTTP and Schema Rules

### 2.1 Common types

The following component schemas are shared by every resource. They are written
in OpenAPI-like YAML so they can be copied into the generated Admin contract
without inventing a second type system.

```yaml
ScopeRef:
  type: object
  required: [organization_id, scope]
  properties:
    organization_id: {type: string, format: uuid}
    workspace_id: {type: string, format: uuid}
    scope: {type: string, enum: [organization, workspace]}

ResourceState:
  type: string
  enum: [draft, validating, valid, active, published, disabled, retired, revoked]

PageInfo:
  type: object
  required: [next_cursor]
  properties:
    next_cursor: {type: string, nullable: true}

CommandMeta:
  type: object
  required: [correlation_id, audit_event_id]
  properties:
    correlation_id: {type: string}
    audit_event_id: {type: string}
    async_job_id: {type: string, nullable: true}

Problem:
  type: object
  required: [code, message, correlation_id]
  properties:
    code: {type: string}
    message: {type: string}
    correlation_id: {type: string}
    field_errors:
      type: array
      items: {type: object}

ValidationResult:
  type: object
  required: [state, findings]
  properties:
    state: {type: string, enum: [valid, invalid, pending]}
    findings: {type: array, items: {type: object}}

AdminCapability:
  type: string
  enum:
    - policies.read
    - policies.draft.read
    - policies.draft.edit
    - policies.validate
    - policies.publish
    - policies.bind
    - policies.simulate
    - policies.retire
    - evaluations.read
    - evaluations.suite.edit
    - evaluations.suite.publish
    - evaluations.run
    - evaluations.gate.override
    - evaluations.export
    - integrations.read
    - integrations.manage
    - integrations.clients.manage
    - integrations.credentials.rotate
    - integrations.publications.manage
    - integrations.webhooks.manage
    - integrations.webhooks.redeliver
    - integrations.usage.read
    - platform.read
    - platform.providers.manage
    - platform.runners.manage
    - platform.credentials.read
    - platform.credentials.rotate
    - platform.credentials.revoke
    - platform.classifications.manage
    - platform.limits.manage
    - platform.environments.manage
    - platform.settings.manage
    - platform.emergency.quarantine

AuditLink:
  type: object
  required: [event_id]
  properties:
    event_id: {type: string}
    correlation_id: {type: string}
```

Resource IDs are opaque. Every list is cursor-paginated with a documented
stable sort, every timestamp is UTC RFC 3339, and every response is filtered by
the caller's authorized scope. A forbidden resource is indistinguishable from
an absent resource.

### 2.2 Command requirements

- `POST` commands accept `Idempotency-Key`; the server stores the request digest
  and returns the original result for a matching retry.
- A reused key with a different digest returns `409 idempotency_conflict`.
- Mutable Drafts and projections that support concurrent edits return an `ETag`.
  Mutations require `If-Match`; a stale value returns `409 etag_conflict`.
- State-changing commands return the changed resource plus `CommandMeta`.
  Long-running work returns `202` and an accepted resource or job state.
- Every accepted, rejected, blocked, expired, and failed command emits one
  canonical Audit Event. Read endpoints never create audit records.

### 2.3 Common errors

```yaml
ErrorCode:
  type: string
  enum:
    - resource_forbidden
    - scope_forbidden
    - etag_conflict
    - idempotency_conflict
    - invalid_state
    - schema_invalid
    - binding_incompatible
    - publication_incompatible
    - gate_not_satisfied
    - fixture_miss
    - quarantine_active
    - secret_not_available
    - command_expired
```

The API never returns a secret value, raw chain-of-thought, private key,
provider token, or unrestricted internal prompt. Redaction is represented in
the typed response, not silently omitted.

## 3. Policy Contract

### 3.1 Typed schemas

```yaml
Policy:
  type: object
  required: [id, organization_id, type, name, state, schema_version, draft_etag]
  properties:
    id: {type: string}
    organization_id: {type: string, format: uuid}
    workspace_id: {type: string, format: uuid, nullable: true}
    type:
      type: string
      enum: [approval, model, tool, command, network, credential, data,
             budget, retention, evaluation, publication]
    name: {type: string}
    owner_principal_id: {type: string}
    state: {type: string, enum: [draft, published, retired]}
    schema_version: {type: string}
    draft_etag: {type: string}
    latest_version_id: {type: string, nullable: true}
    active_binding_count: {type: integer, minimum: 0}

PolicyDraft:
  type: object
  required: [policy_id, document, schema_version, etag, validation]
  properties:
    policy_id: {type: string}
    document: {type: object}
    schema_version: {type: string}
    etag: {type: string}
    validation: {$ref: '#/components/schemas/ValidationResult'}

PolicyDocument:
  oneOf:
    - {$ref: '#/components/schemas/ApprovalPolicyDocument'}
    - {$ref: '#/components/schemas/ModelPolicyDocument'}
    - {$ref: '#/components/schemas/ToolPolicyDocument'}
    - {$ref: '#/components/schemas/NetworkPolicyDocument'}
    - {$ref: '#/components/schemas/LimitPolicyDocument'}
  discriminator: {propertyName: kind}

ApprovalPolicyDocument:
  type: object
  required: [kind, rules]
  properties:
    kind: {type: string, enum: [approval]}
    rules: {type: array, items: {type: object}}
    default_effect: {type: string, enum: [allow, deny, require_requester_approval]}
    approval_ttl_seconds: {type: integer, minimum: 1}

ModelPolicyDocument:
  type: object
  required: [kind, allowed_routes]
  properties:
    kind: {type: string, enum: [model]}
    allowed_routes: {type: array, items: {type: string}}
    max_tokens: {type: integer, minimum: 1}
    data_classifications: {type: array, items: {type: string}}

ToolPolicyDocument:
  type: object
  required: [kind, allowed_effects]
  properties:
    kind: {type: string, enum: [tool, command]}
    allowed_effects: {type: array, items: {type: string}}
    allowed_operations: {type: array, items: {type: string}}
    approval_required_for: {type: array, items: {type: string}}

NetworkPolicyDocument:
  type: object
  required: [kind, egress]
  properties:
    kind: {type: string, enum: [network]}
    egress: {type: string, enum: [deny, allowlist]}
    destinations: {type: array, items: {type: object}}

LimitPolicyDocument:
  type: object
  required: [kind, limits]
  properties:
    kind: {type: string, enum: [budget, retention, evaluation, publication]}
    limits: {type: object}

PolicyVersion:
  type: object
  required: [id, policy_id, content_digest, message, document, created_at]
  properties:
    id: {type: string}
    policy_id: {type: string}
    content_digest: {type: string}
    schema_version: {type: string}
    message: {type: string, minLength: 1}
    document: {type: object}
    compiler_evidence: {type: object}
    created_by: {type: string}
    created_at: {type: string, format: date-time}

PolicyBinding:
  type: object
  required: [id, version_id, target, environment, state, effective_from]
  properties:
    id: {type: string}
    version_id: {type: string}
    target: {$ref: '#/components/schemas/ScopeRef'}
    target_resource_id: {type: string, nullable: true}
    environment: {type: string, enum: [development, staging, production]}
    state: {type: string, enum: [pending, active, expired, revoked]}
    effective_from: {type: string, format: date-time}
    effective_until: {type: string, format: date-time, nullable: true}
    reason: {type: string}

PolicySimulation:
  type: object
  required: [decision, contributing_versions, explanation]
  properties:
    decision: {type: string, enum: [allow, deny, require_requester_approval]}
    contributing_versions: {type: array, items: {type: object}}
    ineffective_rules: {type: array, items: {type: object}}
    explanation: {type: string}
```

The `document` field is typed by `Policy.type` and `schema_version`; arbitrary
JSON is rejected unless the selected type schema explicitly allows it. Approval
Policies describe one concrete Agent action and can only select `allow`, `deny`,
or the authenticated Run requester's approval. They cannot nominate an Admin
approver or represent a business workflow.

### 3.2 OpenAPI target routes

| Method and route | Contract and command boundary |
| --- | --- |
| `GET /api/admin/v1/policies` | Cursor list; filters type, scope, state, owner, binding target |
| `POST /api/admin/v1/policies` | Creates identity and one Draft; requires `policies.draft.edit` |
| `GET /api/admin/v1/policies/{id}` | Policy projection plus current Draft metadata |
| `GET /api/admin/v1/policies/{id}/draft` | Typed Draft; requires `policies.draft.read` |
| `PATCH /api/admin/v1/policies/{id}/draft` | ETag-protected typed update; no runtime effect |
| `POST /api/admin/v1/policies/{id}:validate` | Side-effect-free validation; returns findings and normalized digest |
| `GET /api/admin/v1/policies/{id}/versions` | Immutable versions, stable creation order |
| `POST /api/admin/v1/policies/{id}/versions` | Publishes exact Draft with message, digest, ETag, and idempotency key |
| `GET /api/admin/v1/policies/{id}/bindings` | Exact active and historical bindings in authorized scope |
| `POST /api/admin/v1/policies/{id}/bindings` | Binds one exact version; rejects scope broadening |
| `POST /api/admin/v1/policy-bindings/{id}:revoke` | Revokes one binding without deleting its Version |
| `POST /api/admin/v1/policies/{id}:simulate` | Evaluates Draft or exact Version; never creates an approval or permit |
| `POST /api/admin/v1/policies/{id}:retire` | Retires the identity after bindings are removed or expired |

Publishing never activates a version. Binding never mutates a version. A
lower-scope binding is rejected when it broadens an outer policy, and an
uncomposable rule is rejected rather than resolved by priority.

Policy endpoint capability mapping is explicit: catalog and Version reads use
`policies.read`; Draft reads/editing use `policies.draft.read` and
`policies.draft.edit`; validation uses `policies.validate`; publication, binding,
simulation, and retirement use `policies.publish`, `policies.bind`,
`policies.simulate`, and `policies.retire` respectively. A caller may hold
`policies.read` without holding any mutation capability.

Policy transitions are `draft -> validating -> valid -> published -> retired`;
an invalid Draft returns to `draft`, and a Version never returns to Draft.
Bindings transition `pending -> active -> expired|revoked`; revocation does not
rewrite the Version or historical Run Manifest.

## 4. Evaluation Contract

### 4.1 Typed schemas

```yaml
EvaluationSuite:
  type: object
  required: [id, organization_id, workspace_id, name, state, owner_principal_id]
  properties:
    id: {type: string}
    organization_id: {type: string, format: uuid}
    workspace_id: {type: string, format: uuid}
    name: {type: string}
    state: {type: string, enum: [draft, published, retired]}
    owner_principal_id: {type: string}
    latest_version_id: {type: string, nullable: true}
    gate_usage_count: {type: integer, minimum: 0}

EvaluationCase:
  type: object
  required: [id, suite_id, input, fixture_manifest, assertions, etag]
  properties:
    id: {type: string}
    suite_id: {type: string}
    input: {type: object}
    fixture_manifest: {type: object}
    assertions: {type: array, items: {type: object}}
    rubric: {type: object, nullable: true}
    compatibility: {type: object}
    etag: {type: string}

EvaluationSuiteVersion:
  type: object
  required: [id, suite_id, content_digest, case_manifest_digest, fixture_manifest_digest]
  properties:
    id: {type: string}
    suite_id: {type: string}
    content_digest: {type: string}
    case_manifest_digest: {type: string}
    fixture_manifest_digest: {type: string}
    evaluator_policy_version_id: {type: string}
    runtime_image_digest: {type: string}
    published_at: {type: string, format: date-time}

EvaluationRun:
  type: object
  required: [id, suite_version_id, candidate_revision_hash, environment_digest, state]
  properties:
    id: {type: string}
    suite_version_id: {type: string}
    candidate_revision_hash: {type: string}
    baseline_revision_hash: {type: string, nullable: true}
    environment_digest: {type: string}
    state: {type: string, enum: [requested, queued, provisioning, running, completed, failed, canceled, invalid]}
    gate_result: {type: string, enum: [not_applicable, passed, failed, blocked, invalid]}
    deterministic_summary: {type: object}
    probabilistic_summary: {type: object, nullable: true}
    evidence_manifest_digest: {type: string, nullable: true}

PublicationGate:
  type: object
  required: [id, agent_revision_hash, suite_version_id, requirement, state]
  properties:
    id: {type: string}
    agent_revision_hash: {type: string}
    suite_version_id: {type: string}
    requirement: {type: object}
    state: {type: string, enum: [required, passed, failed, overridden, expired]}
    override_id: {type: string, nullable: true}

GateOverride:
  type: object
  required: [id, gate_id, reason, expires_at, reviewer_principal_id]
  properties:
    id: {type: string}
    gate_id: {type: string}
    reason: {type: string, minLength: 1}
    reviewer_principal_id: {type: string}
    expires_at: {type: string, format: date-time}
```

Suite Versions freeze cases, fixture digests, evaluator policy, runtime image,
and compatibility constraints. An Evaluation Run can never reference a mutable
Draft or Suite working copy. `invalid` means evidence is not comparable or the
fixture/environment contract was violated; it is not a passing or ordinary
failed result.

### 4.2 OpenAPI target routes

| Method and route | Contract and command boundary |
| --- | --- |
| `GET /api/admin/v1/evaluation-suites` | Workspace-scoped suite list and filters |
| `POST /api/admin/v1/evaluation-suites` | Creates a suite Draft; requires `evaluations.suite.edit` |
| `GET /api/admin/v1/evaluation-suites/{id}` | Suite projection, latest Version, gates, and recent activity |
| `PATCH /api/admin/v1/evaluation-suites/{id}` | ETag-protected metadata/working-copy update |
| `GET /api/admin/v1/evaluation-suites/{id}/cases` | Typed case working copies and immutable case references |
| `POST /api/admin/v1/evaluation-suites/{id}/cases` | Adds one case to the mutable Suite working copy |
| `PATCH /api/admin/v1/evaluation-suites/{id}/cases/{case_id}` | ETag-protected case edit |
| `POST /api/admin/v1/evaluation-suites/{id}:validate` | Side-effect-free case, fixture, and assertion validation |
| `GET /api/admin/v1/evaluation-suites/{id}/versions` | Immutable Suite Versions |
| `POST /api/admin/v1/evaluation-suites/{id}/versions` | Publishes a frozen case/fixture manifest |
| `GET /api/admin/v1/evaluation-suites/{id}/runs` | Cursor list of runs, filtered by candidate, baseline, state, and time |
| `POST /api/admin/v1/evaluation-suites/{id}/runs` | Starts a run from exact Revision + Suite Version; returns `202` |
| `GET /api/admin/v1/evaluation-runs/{id}` | Run status, summaries, evidence manifest, and redacted findings |
| `POST /api/admin/v1/evaluation-runs/{id}:cancel` | Cancels a queued or active run; preserves partial evidence |
| `GET /api/admin/v1/evaluation-runs/{id}/regressions` | Comparable candidate/baseline regressions only |
| `GET /api/admin/v1/evaluation-gates` | Authorized gate projections by Agent Revision |
| `POST /api/admin/v1/evaluation-gates/{id}:override` | Expiring, reasoned override; never changes run evidence |
| `POST /api/admin/v1/trajectory-exports` | Creates a redacted, reviewed fixture-export job |

Starting a run requires an exact candidate Revision, immutable Suite Version,
evaluation environment, and idempotency key. The server rejects production
credential references, real write destinations, unknown fixtures, and a missing
runtime/evaluator digest before provisioning a runner.

Evaluation endpoint capability mapping is explicit: suite and result reads use
`evaluations.read`; authoring and Version publication use
`evaluations.suite.edit` and `evaluations.suite.publish`; starting or canceling
a Run uses `evaluations.run`; Gate Overrides use `evaluations.gate.override`;
trajectory export uses `evaluations.export`. `evaluations.run` never implies
the ability to edit or publish a Suite.

Suite transitions are `draft -> validating -> published -> retired`. Runs
transition `requested -> queued -> provisioning -> running -> completed|failed|
canceled|invalid`; a terminal result is immutable. Gate state is derived from
the exact run evidence and can become `overridden` only through an expiring
Gate Override.

## 5. Integration Contract

### 5.1 Typed schemas

```yaml
Integration:
  type: object
  required: [id, organization_id, slug, display_name, state, owner_principal_id]
  properties:
    id: {type: string}
    organization_id: {type: string, format: uuid}
    slug: {type: string}
    display_name: {type: string}
    state: {type: string, enum: [active, disabled, retired]}
    owner_principal_id: {type: string}
    environments: {type: array, items: {type: string}}

DelegatedSubjectPolicy:
  type: object
  required: [token_exchange_required, allowed_issuers, required_claims]
  properties:
    token_exchange_required: {type: boolean, enum: [true]}
    allowed_issuers: {type: array, minItems: 1, items: {type: string}}
    required_claims: {type: array, items: {type: string}, uniqueItems: true}
    minimum_authentication_strength: {type: string, nullable: true}
    maximum_token_age_seconds: {type: integer, minimum: 1}

IntegrationClient:
  type: object
  required: [id, integration_id, environment, token_exchange_policy, status,
             credential_fingerprint]
  properties:
    id: {type: string}
    integration_id: {type: string}
    environment: {type: string, enum: [development, staging, production]}
    token_exchange_policy: {$ref: '#/components/schemas/DelegatedSubjectPolicy'}
    audience: {type: string}
    status: {type: string, enum: [active, disabled, expired, revoked]}
    credential_fingerprint: {type: string}
    expires_at: {type: string, format: date-time, nullable: true}

AgentPublication:
  type: object
  required: [id, integration_id, revision_hash, input_contract_digest,
             output_contract_digest, delegated_subject_policy, state]
  properties:
    id: {type: string}
    integration_id: {type: string}
    client_id: {type: string}
    workspace_id: {type: string, format: uuid}
    environment: {type: string}
    revision_hash: {type: string}
    input_contract_digest: {type: string}
    output_contract_digest: {type: string}
    delegated_subject_policy: {$ref: '#/components/schemas/DelegatedSubjectPolicy'}
    state: {type: string, enum: [draft, active, expired, revoked]}
    effective_until: {type: string, format: date-time, nullable: true}

WebhookEndpoint:
  type: object
  required: [id, integration_id, environment, destination, status, signing_key_fingerprint]
  properties:
    id: {type: string}
    integration_id: {type: string}
    environment: {type: string}
    destination: {type: string, format: uri}
    status: {type: string, enum: [active, disabled, quarantined, retired]}
    signing_key_fingerprint: {type: string}
    subscribed_events: {type: array, items: {type: string}}
    retry_policy: {type: object}

WebhookDelivery:
  type: object
  required: [id, endpoint_id, event_id, delivery_id, attempt, state]
  properties:
    id: {type: string}
    endpoint_id: {type: string}
    event_id: {type: string}
    delivery_id: {type: string}
    attempt: {type: integer, minimum: 1}
    state: {type: string, enum: [queued, delivered, retrying, failed, canceled]}
    response_class: {type: string, nullable: true}
    next_attempt_at: {type: string, format: date-time, nullable: true}
```

An Integration identity does not issue credentials or grant Agent authority.
Every invocation resolves one active Client, one active Publication, one exact
Revision, one input/output contract pair, and one verified delegated subject.
The client identity and subject are preserved independently in policy and Audit
context, but only the subject becomes Session owner, Run requester, or
Agent-action approver.
A client without a subject must use a pre-configured owner-bound Webhook or
scheduled trigger instead of direct invocation.

### 5.2 OpenAPI target routes

| Method and route | Contract and command boundary |
| --- | --- |
| `GET /api/admin/v1/integrations` | Organization-scoped directory and filters |
| `POST /api/admin/v1/integrations` | Creates identity/owner only; no credential issuance |
| `GET /api/admin/v1/integrations/{id}` | Integration projection and recent activity |
| `PATCH /api/admin/v1/integrations/{id}` | ETag-protected metadata update |
| `GET /api/admin/v1/integrations/{id}/clients` | Environment-bound client projections, never secrets |
| `POST /api/admin/v1/integrations/{id}/clients` | Registers client metadata and credential reference |
| `POST /api/admin/v1/integration-clients/{id}:rotate` | Rotates reference; bounded overlap; idempotent |
| `POST /api/admin/v1/integration-clients/{id}:disable` | Blocks new calls; preserves history |
| `GET /api/admin/v1/integrations/{id}/publications` | Exact Agent Publication list |
| `POST /api/admin/v1/integrations/{id}/publications` | Publishes exact Revision after compatibility/security checks |
| `POST /api/admin/v1/integration-publications/{id}:revoke` | Blocks new invocations; preserves Sessions/Runs |
| `GET /api/admin/v1/integrations/{id}/webhooks` | Endpoint metadata and health |
| `POST /api/admin/v1/integrations/{id}/webhooks` | Registers validated HTTPS endpoint metadata |
| `POST /api/admin/v1/webhook-endpoints/{id}:redeliver` | Reuses event ID, creates new delivery attempt |
| `GET /api/admin/v1/integrations/{id}/usage` | Aggregated usage linked to Runs and deliveries |

Publication changes require a semantic diff, exact Revision hash, contract
digests, environment, delegated-subject policy, quotas, and idempotency key. Destination
validation rejects private-network/SSRF targets. Webhook redelivery never
changes the Run result.

Integration endpoint capability mapping is explicit: directory and usage reads
use `integrations.read` and `integrations.usage.read`; identity and client
metadata use `integrations.manage` and `integrations.clients.manage`; credential
rotation uses `integrations.credentials.rotate`; Publication changes use
`integrations.publications.manage`; endpoint changes and redelivery use
`integrations.webhooks.manage` and `integrations.webhooks.redeliver`.
No Integration capability grants business approval authority.

## 6. Platform Management Contract

### 6.1 Typed schemas

```yaml
ModelProvider:
  type: object
  required: [id, organization_id, name, state, data_classes, credential_reference_id]
  properties:
    id: {type: string}
    organization_id: {type: string, format: uuid}
    name: {type: string}
    state: {type: string, enum: [active, degraded, disabled, quarantined]}
    data_classes: {type: array, items: {type: string}}
    credential_reference_id: {type: string}
    health: {type: object}
    routes: {type: array, items: {$ref: '#/components/schemas/ProviderRoute'}}

ProviderRoute:
  type: object
  required: [id, provider_id, allowed_models, state]
  properties:
    id: {type: string}
    provider_id: {type: string}
    allowed_models: {type: array, items: {type: string}}
    fallback_route_ids: {type: array, items: {type: string}}
    state: {type: string, enum: [active, degraded, disabled]}
    budget_policy_id: {type: string, nullable: true}
    classification_constraints: {type: object}

RunnerPool:
  type: object
  required: [id, organization_id, isolation_tier, state, capacity]
  properties:
    id: {type: string}
    organization_id: {type: string, format: uuid}
    isolation_tier: {type: string, enum: [development, gvisor, microvm]}
    state: {type: string, enum: [active, draining, quarantined, disabled]}
    compatible_protocols: {type: array, items: {type: string}}
    capacity: {type: object}

Runner:
  type: object
  required: [id, pool_id, state, protocol_version, lease_epoch]
  properties:
    id: {type: string}
    pool_id: {type: string}
    state: {type: string, enum: [ready, assigned, draining, quarantined, offline]}
    protocol_version: {type: string}
    lease_epoch: {type: integer, minimum: 0}
    last_heartbeat_at: {type: string, format: date-time, nullable: true}

CredentialReference:
  type: object
  required: [id, organization_id, target_service, state, classification]
  properties:
    id: {type: string}
    organization_id: {type: string, format: uuid}
    target_service: {type: string}
    state: {type: string, enum: [active, rotating, expired, revoked, disabled]}
    classification: {type: string}
    allowed_modes: {type: array, items: {type: string}}
    secret_version: {type: string, nullable: true}
    expires_at: {type: string, format: date-time, nullable: true}

DataClassification:
  type: object
  required: [id, organization_id, label, handling, retention_class]
  properties:
    id: {type: string}
    organization_id: {type: string, format: uuid}
    label: {type: string}
    handling: {type: string, enum: [public, internal, confidential, restricted]}
    retention_class: {type: string}
    allowed_provider_ids: {type: array, items: {type: string}}
    allowed_tool_classes: {type: array, items: {type: string}}

LimitPolicy:
  type: object
  required: [id, scope, concurrency, duration_seconds, budget]
  properties:
    id: {type: string}
    scope: {$ref: '#/components/schemas/ScopeRef'}
    concurrency: {type: integer, minimum: 0}
    duration_seconds: {type: integer, minimum: 1}
    output_bytes: {type: integer, minimum: 0}
    artifact_bytes: {type: integer, minimum: 0}
    budget: {type: object}

EnvironmentProfile:
  type: object
  required: [id, organization_id, name, publication_posture, state]
  properties:
    id: {type: string}
    organization_id: {type: string, format: uuid}
    name: {type: string, enum: [development, staging, production]}
    publication_posture: {type: string, enum: [test_only, review_required, production]}
    state: {type: string, enum: [active, emergency, disabled]}
    data_classification_id: {type: string}
    allowed_target_controls: {type: object}

PlatformSettingsProjection:
  type: object
  required: [scope, values, etag]
  properties:
    scope: {$ref: '#/components/schemas/ScopeRef'}
    values: {type: object}
    etag: {type: string}
    validation_state: {type: string, enum: [valid, conflict, pending]}
```

Provider, Runner Pool, Credential Reference, Classification, Limit, and
Environment are separate resources. `/platform/settings` composes their
effective organization and Workspace projection; it does not become a mutable
catch-all record. Credential values, private keys, and provider tokens are
never part of these schemas.

### 6.2 OpenAPI target routes

| Method and route | Contract and command boundary |
| --- | --- |
| `GET /api/admin/v1/platform/model-providers` | Organization-scoped provider metadata and health |
| `POST /api/admin/v1/platform/model-providers` | Registers provider metadata and credential reference |
| `GET /api/admin/v1/platform/model-providers/{id}/routes` | Provider routes and allowed model metadata |
| `PUT /api/admin/v1/platform/model-providers/{id}/routes/{route_id}` | ETag-protected route policy change |
| `POST /api/admin/v1/platform/model-providers/{id}:quarantine` | Stops new route selection; preserves evidence |
| `GET /api/admin/v1/platform/runner-pools` | Pool capacity, compatibility, health, and state |
| `POST /api/admin/v1/platform/runner-pools` | Creates a typed pool definition |
| `GET /api/admin/v1/platform/runner-pools/{id}/runners` | Runner health and lease-free operational metadata |
| `POST /api/admin/v1/platform/runner-pools/{id}:drain` | Prevents new scheduling and returns drain state |
| `POST /api/admin/v1/platform/runner-pools/{id}:quarantine` | Emergency stop for new assignments |
| `GET /api/admin/v1/platform/credentials` | Non-secret references, expiry, rotation, and usage |
| `POST /api/admin/v1/platform/credentials/{id}:rotate` | Starts broker-owned rotation; never returns secret material |
| `POST /api/admin/v1/platform/credentials/{id}:revoke` | Blocks new leases and records reason |
| `GET /api/admin/v1/platform/data-classifications` | Typed classifications and provider/tool constraints |
| `POST /api/admin/v1/platform/data-classifications` | Creates or updates definition with ETag |
| `GET /api/admin/v1/platform/limit-policies` | Organization bounds and Workspace allocations |
| `PUT /api/admin/v1/platform/limit-policies/{id}` | ETag-protected bounded change |
| `GET /api/admin/v1/platform/environment-profiles` | Environment posture and emergency state |
| `PUT /api/admin/v1/platform/environment-profiles/{id}` | ETag-protected posture change |
| `GET /api/admin/v1/platform/settings` | Composed scope-aware projection |
| `POST /api/admin/v1/platform/settings:validate` | Side-effect-free cross-resource validation |
| `POST /api/admin/v1/platform/settings:apply` | Atomic section command with ETag and idempotency key |

Organization scope is required for Providers, Runner Pools, Credentials, and
organization bounds. Workspace scope may only narrow classifications, limits,
environment posture, and retention within organization constraints. Drain,
quarantine, revoke, and emergency settings are explicit auditable commands and
never rewrite historical Runs or Audit evidence.

Platform endpoint capability mapping is explicit: metadata and health reads use
`platform.read`; provider, Runner Pool, classification, limit, and environment
mutations use their corresponding `*.manage` capability; credential metadata,
rotation, and revocation use `platform.credentials.read`,
`platform.credentials.rotate`, and `platform.credentials.revoke`; composed
Settings mutation uses `platform.settings.manage`; quarantine uses
`platform.emergency.quarantine`. No capability returns secret material.

Provider states are `active -> degraded|disabled|quarantined`; a quarantined
provider cannot receive new route selection. Runner Pools transition
`active -> draining -> disabled` or `active -> quarantined`; draining waits for
existing leases according to the operational timeout. Credential References
transition `active -> rotating -> active` or `expired|revoked|disabled` and
only the broker can access the underlying secret version.

## 7. Authorization Matrix

The capability names below are resource capabilities, not roles. Roles are
presets that create explicit grants; they are not evaluated as an implicit
exception to scope or resource state.

| Capability family | Organization Administrator | Workspace Administrator | Workspace Agent Editor | Security Reviewer | Operator | Auditor |
| --- | --- | --- | --- | --- | --- | --- |
| Policy read/simulate | Org + all Workspaces | Assigned Workspaces | No by default; explicit grant only | Authorized scope | Effective runtime view | Read-only assigned scope |
| Policy Draft edit/validate | Org policies | Workspace policies | No by default | No by default | No | No |
| Policy publish/bind/retire | Org + assigned targets | Assigned Workspace targets | No | Review/decision only unless separately granted | No | No |
| Evaluation suite edit/publish | All scopes | Assigned Workspaces | Assigned Workspaces | Review and gate evidence | No | Read-only |
| Evaluation run | All authorized scopes | Assigned Workspaces | Assigned Workspaces | No by default | Operational read | Read-only |
| Gate override/export | Org scope | Assigned scope if granted | No | Yes with reason and expiry | No | Export only if `evaluations.export` granted |
| Integration/client management | Organization | No by default | No | Read/review only | Health/read only | Read-only |
| Integration Publication | Organization | No by default | No | Review only | No | Read-only |
| Webhook management/redelivery | Organization | No by default | No | No by default | Delivery operations only | Read-only |
| Provider/credential management | Organization | No | No | Review/read only | Health/quarantine only | Read-only metadata |
| Runner pool drain/quarantine | Organization | No | No | Emergency review | Operational command if granted | Read-only |
| Classification/limits/environments | Org bounds | Workspace narrowing | Read-only effective view | Review/simulation | Effective runtime view | Read-only |
| Platform Settings apply | Organization | Bounded Workspace overrides | No | Legal Hold/retention simulation only | No | No |

The server evaluates access in this order:

1. Authenticate the Admin audience and principal.
2. Require the route capability and the requested organization/Workspace scope.
3. Check resource ownership, publication relation, environment, and lifecycle
   state.
4. Check ETag/idempotency and command-specific transition preconditions.
5. Apply outer Policy and action-time constraints; a lower scope cannot broaden
   authority.
6. Record the decision and resulting state in the canonical Audit projection.

Agent ACL capabilities do not grant any of these Admin management capabilities.
Conversely, an Admin role does not grant Copilot requester approval or business
workflow approval authority. `workspace_agent_editor` has only the explicit,
audited self-enrollment path described above; it does not automatically expose
another employee's conversation or grant contribution or execution authority.
This Session command is a scoped Admin workflow rather than an Agent ACL
capability, so it is intentionally outside the capability-family matrix above.

## 8. Audit, Redaction, and Verification Boundary

The four resource families use the canonical `/api/admin/v1/audit-events`
projection. Resource pages may show Recent activity and pre-filtered links but
do not create alternate Audit stores. Audit payloads include resource ID,
version/digest, actor, scope, command, outcome, correlation ID, and redaction
metadata.

The following actions are always attributable: Draft validation, publish, bind,
retire, suite/version publication, evaluation start/cancel, gate override,
trajectory export, Integration/client/publication/webhook changes, provider or
Runner quarantine, credential rotate/revoke, classification/limit/environment
changes, Settings apply, and failed or blocked transitions.

The contract is complete only when each route has:

- an OpenAPI path and component schema;
- generated client types and request/response compatibility tests;
- scope and capability tests for allow, deny, and outer-constraint cases;
- ETag and idempotency race tests for every mutable command;
- state-transition and audit-event tests;
- redaction tests proving secrets and protected content never appear.
