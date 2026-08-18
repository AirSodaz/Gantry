# Agent Access Contracts

## 1. Scope and Status

This document defines the target Admin contract for the Agent-scoped access
resource. It makes the existing product rules executable: independent
capabilities, Allow-only grants, principal/group/service-identity subjects,
owner transfer, optimistic concurrency, recovery protection, and canonical
Audit evidence.

This is a design contract, not a claim that the routes are implemented. The
current callable Admin OpenAPI and handlers do not yet expose Agent access
routes. Implementation requires adding the routes to the Admin OpenAPI source,
generating clients, adding persistence and handlers, and testing every
authorization and recovery transition.

The contract uses the Admin audience `gantry-admin-api`. Copilot receives only
effective Agent availability and the authenticated user's authorized Agent
projection. It never receives the Admin ACL, group membership, service
identity inventory, grant issuer, or access-management controls.

## 2. Authority Model

An Agent has one `owner_principal_id` stored on the Agent resource and an
independent collection of explicit `AgentAccessGrant` resources. Ownership is
not a capability and does not silently grant configuration, execution, or
other Agent access. The owner has a narrow owner-transfer authority, while
normal Agent operations still require their explicit grants and outer
organization/Workspace authorization.

The initial model is Allow-only. A grant contains an explicit set of allowed
capabilities; there is no stored Deny grant and no Deny precedence algorithm.
Roles and presets are client-side convenience bundles that materialize the
same explicit capability set. A later change to a preset never mutates an
existing grant.

### 2.1 Subjects

```yaml
AgentAccessSubject:
  type: object
  required: [type, id, display_name, state]
  properties:
    type:
      type: string
      enum: [principal, group, service_identity]
    id: {type: string}
    display_name: {type: string}
    state: {type: string, enum: [active, disabled, unknown]}
    directory_source: {type: string, nullable: true}
```

- `principal` is one normalized human or non-human identity from the
  organization directory.
- `group` is an organization directory group. Membership is evaluated from the
  current identity projection at authorization time; membership is not copied
  into the Agent ACL and nested groups are not expanded by Gantry.
- `service_identity` references an active registered service identity. It is
  not a human, cannot be an Agent owner, and carries no secret in the grant.

Subjects must belong to the Agent's organization. A human principal or group
also needs an applicable Workspace membership at action time. A service
identity must be registered for the organization and allowed in the target
Workspace or Integration publication.

### 2.2 Capabilities

```yaml
AgentCapability:
  type: string
  enum:
    - metadata.read
    - configuration.read
    - draft.edit
    - review.decide
    - deployment.test
    - deployment.production
    - runs.read
    - execute
    - access.manage
```

Capabilities have no inheritance. In particular, `access.manage` does not
grant configuration or execution, `execute` does not grant configuration read,
and `configuration.read` does not grant execution. Effective access is the
intersection of the matching grants, current organization/Workspace
authorization, Agent and Deployment state, Integration publication, and
action-time Policy.

## 3. Typed Resource Schemas

The schemas below reuse the common Admin `PageInfo`, `CommandMeta`, and
problem-envelope shapes defined in [Admin Governed Resource Contracts](admin-governed-resource-contracts.md).

```yaml
AgentAccessGrant:
  type: object
  required: [id, agent_id, subject, capabilities, state, valid_from,
             created_by_principal_id, created_at, updated_at, etag]
  properties:
    id: {type: string}
    agent_id: {type: string}
    subject: {$ref: '#/components/schemas/AgentAccessSubject'}
    capabilities:
      type: array
      minItems: 1
      uniqueItems: true
      items: {$ref: '#/components/schemas/AgentCapability'}
    state: {type: string, enum: [scheduled, active, expired, revoked]}
    valid_from: {type: string, format: date-time}
    expires_at: {type: string, format: date-time, nullable: true}
    grant_batch_id: {type: string, nullable: true}
    source_preset: {type: string, nullable: true}
    reason: {type: string, nullable: true}
    created_by_principal_id: {type: string}
    updated_by_principal_id: {type: string}
    revoked_by_principal_id: {type: string, nullable: true}
    revoked_at: {type: string, format: date-time, nullable: true}
    revocation_reason: {type: string, nullable: true}
    created_at: {type: string, format: date-time}
    updated_at: {type: string, format: date-time}
    etag: {type: string}

AgentAccessList:
  type: object
  required: [agent_id, owner_principal_id, items, page_info, acl_etag,
             recovery]
  properties:
    agent_id: {type: string}
    owner_principal_id: {type: string}
    items: {type: array, items: {$ref: '#/components/schemas/AgentAccessGrant'}}
    page_info: {$ref: '#/components/schemas/PageInfo'}
    acl_etag: {type: string}
    recovery: {$ref: '#/components/schemas/AccessRecoveryState'}

AgentAccessGrantResult:
  type: object
  required: [grant, command]
  properties:
    grant: {$ref: '#/components/schemas/AgentAccessGrant'}
    command: {$ref: '#/components/schemas/CommandMeta'}

AccessRecoveryState:
  type: object
  required: [has_recovery_path, active_access_manager_count]
  properties:
    has_recovery_path: {type: boolean}
    active_access_manager_count: {type: integer, minimum: 0}
    blocking_reason: {type: string, nullable: true}

EffectiveAgentAccess:
  type: object
  required: [agent_id, subject, capabilities, constraints]
  properties:
    agent_id: {type: string}
    subject: {$ref: '#/components/schemas/AgentAccessSubject'}
    capabilities:
      type: array
      items:
        type: object
        required: [capability, state]
        properties:
          capability: {$ref: '#/components/schemas/AgentCapability'}
          state: {type: string, enum: [granted, blocked, absent]}
          grant_ids: {type: array, items: {type: string}}
    constraints: {type: array, items: {type: object}}

CreateAgentAccessGrantRequest:
  type: object
  required: [subject_type, subject_id, capabilities]
  properties:
    subject_type: {type: string, enum: [principal, group, service_identity]}
    subject_id: {type: string}
    capabilities:
      type: array
      minItems: 1
      uniqueItems: true
      items: {$ref: '#/components/schemas/AgentCapability'}
    valid_from: {type: string, format: date-time, nullable: true}
    expires_at: {type: string, format: date-time, nullable: true}
    source_preset: {type: string, nullable: true}
    reason: {type: string, nullable: true}

PatchAgentAccessGrantRequest:
  type: object
  minProperties: 1
  properties:
    capabilities:
      type: array
      minItems: 1
      uniqueItems: true
      items: {$ref: '#/components/schemas/AgentCapability'}
    valid_from: {type: string, format: date-time, nullable: true}
    expires_at: {type: string, format: date-time, nullable: true}
    reason: {type: string, nullable: true}

RevokeAgentAccessGrantRequest:
  type: object
  required: [reason]
  properties:
    reason: {type: string, minLength: 1}

TransferAgentOwnerRequest:
  type: object
  required: [target_principal_id, reason]
  properties:
    target_principal_id: {type: string}
    reason: {type: string, minLength: 1}

AgentOwnerTransfer:
  type: object
  required: [agent_id, previous_owner_principal_id, new_owner_principal_id,
             access_manage_grant, acl_etag, command]
  properties:
    agent_id: {type: string}
    previous_owner_principal_id: {type: string}
    new_owner_principal_id: {type: string}
    access_manage_grant: {$ref: '#/components/schemas/AgentAccessGrant'}
    acl_etag: {type: string}
    command: {$ref: '#/components/schemas/CommandMeta'}
```

There is at most one non-terminal grant for a subject on an Agent. A revoked
or expired grant remains immutable history; a later regrant creates a new
resource. Direct and group grants may both match one principal and their
capabilities union before outer constraints are applied.

## 4. Admin Route Surface

All routes are under `/api/admin/v1` and use the Admin audience. Full grant
rows, subjects, issuer/reason metadata, and recovery details require
`access.manage`. A caller with `configuration.read` may receive a redacted
read-only access projection; it never receives mutation controls or protected
identity details.

| Method and route | Success | Authorization and concurrency |
| --- | --- | --- |
| `GET /agents/{agent_id}/access` | `200 AgentAccessList` | `access.manage` full view or `configuration.read` redacted view; cursor and filters |
| `GET /agents/{agent_id}/access/grants/{grant_id}` | `200 AgentAccessGrant` | `access.manage`; `configuration.read` receives redacted projection |
| `GET /agents/{agent_id}/access/effective` | `200 EffectiveAgentAccess` | `access.manage`; requires `subject_type` and `subject_id` query parameters |
| `POST /agents/{agent_id}/access/grants` | `201 AgentAccessGrantResult` | `access.manage`; `Idempotency-Key` and current `If-Match` ACL ETag |
| `PATCH /agents/{agent_id}/access/grants/{grant_id}` | `200 AgentAccessGrantResult` | `access.manage`; grant `If-Match` and `Idempotency-Key` |
| `POST /agents/{agent_id}/access/grants/{grant_id}:revoke` | `200 AgentAccessGrantResult` | `access.manage`; grant `If-Match` and `Idempotency-Key` |
| `POST /agents/{agent_id}/access:transfer-owner` | `200 AgentOwnerTransfer` | Current owner or `access.manage`; current ACL `If-Match` and `Idempotency-Key` |

`AgentOwnerTransfer` returns the Agent owner projection, the affected
`access.manage` grant, the new `acl_etag`, and `CommandMeta`. If the target
already has a grant, the command adds `access.manage` to that explicit grant;
otherwise it creates a new grant for the target. Existing grants for the old
owner are unchanged.

The list uses stable ordering `(subject_type, subject_id, grant_id)` and
supports `subject_type`, `subject_id`, `capability`, `state`, and `cursor`
filters. A forbidden Agent is returned as `404` rather than revealing its
existence. All mutations use the common problem envelope and return
`etag_conflict`, `idempotency_conflict`, `invalid_state`, `subject_unavailable`,
`recovery_path_required`, or `scope_forbidden` when applicable.

## 5. Command Preconditions and Lifecycle

The server evaluates every command in this order:

1. Authenticate the Admin audience and normalized principal.
2. Resolve the Agent and requested organization/Workspace scope without
   leaking unauthorized existence.
3. Require the route capability or the current-owner exception for owner
   transfer.
4. Resolve the subject from the organization directory or registered service
   identity and verify it is eligible for the Agent's scope.
5. Validate capability names, validity interval, reason requirements, and
   current Agent lifecycle.
6. Lock the Agent access revision and target grant, verify `If-Match` and
   idempotency, and enforce the recovery invariant.
7. Write the grant/owner transition, append canonical Audit evidence, and
   commit the transaction.

Grant lifecycle is `scheduled -> active -> expired|revoked`. A scheduled grant
may be revoked before activation. Expiry is a server transition and may not be
reversed; a new grant is required. A patch cannot change the subject or restore
a revoked/expired grant. Any mutation that includes `execute`,
`deployment.production`, or `access.manage` requires a non-empty reason; a
mutation that adds or removes a capability is never optimistic in the UI.

The Agent access collection has a monotonic `acl_etag`. Grant mutations and
owner transfer increment it. Grant `etag` values change only for that grant.
The transaction that changes access also appends the Audit event, so a
successful response cannot be returned without durable evidence.

### 5.1 Recovery invariant

The server refuses to revoke, expire, or narrow the last active recovery path.
An Agent must retain either an active direct principal or group grant with
`access.manage`, or an organization-authorized break-glass path. A service
identity grant never satisfies this invariant. Owner transfer is allowed only
when the target is an active principal in the same organization and the
resulting target grant has `access.manage`.

The error includes `recovery_path_required` and the current recovery summary.
An explicit owner transfer or separately authorized break-glass command is the
only way to remove the last direct recovery grant. Recovery protection never
creates a Deny grant and never rewrites historical grants.

## 6. Persistence Contract

PostgreSQL is the source of truth. The initial normalized shape is:

```text
gantry.agents
  owner_principal_id
  access_revision bigint not null

gantry.agent_access_grants
  id, agent_id, subject_type, subject_id
  state, valid_from, expires_at
  source_preset, reason
  created_by_principal_id, updated_by_principal_id
  revoked_by_principal_id, revoked_at, revocation_reason
  revision bigint not null
  created_at, updated_at

gantry.agent_access_grant_capabilities
  grant_id, capability
```

Required constraints and indexes:

- `subject_type` is one of `principal`, `group`, or `service_identity`.
- `capability` is one of the `AgentCapability` enum values.
- A partial unique constraint permits at most one `scheduled` or `active` grant
  for `(agent_id, subject_type, subject_id)`.
- Grant capabilities have a composite primary key `(grant_id, capability)`.
- `expires_at`, when present, is later than `valid_from`; the expiry worker
  transitions due grants to `expired` under row lock.
- Foreign keys bind the Agent and grant capability rows; subject references are
  resolved through the Identity/Service Identity registry and are not copied
  into the ACL tables.
- Queries index `(agent_id, state, subject_type, subject_id)` and
  `(agent_id, capability)` for list and effective-access evaluation.

ACL mutations lock the Agent row, verify `access_revision`, update the grant
and capability rows, increment `access_revision`, append Audit evidence, and
commit as one transaction. The idempotency record is in the same transaction.
Workers recheck current grants at action time; a queued Run or runner lease is
not an access grant.

## 7. Projection and Audit Boundary

Admin receives the full ACL resource within its authorized organization or
Workspace scope, including why a grant is currently blocked by membership,
Deployment, quarantine, Integration publication, or Policy. Admin does not
receive secrets or group credentials.

Copilot receives only the effective Agent catalog and member-authorized Session
projection. It may show that an Agent is unavailable or that the user lacks a
capability, but it never exposes ACL rows, other subjects, group membership,
service identities, grant reasons, or owner-transfer commands. Enterprise
Invocation intersects its Integration Publication with the effective Agent
grant and does not use Admin roles as a shortcut.

`workspace_agent_editor` is a Workspace-scoped Admin role, not an Agent ACL
capability. Its only bridge into Session conversation access is the separate,
audited command that adds the caller itself as a Session `viewer`; the resulting
read remains member-authorized and exposes no Agent ACL data.

The canonical Audit projection records:

- grant creation, capability change, validity change, revocation, and expiry;
- owner transfer and the affected old/new owner identities;
- blocked or denied access commands, recovery-path protection, and subject
  resolution failures;
- actor, Agent, subject reference, before/after capabilities, reason, scope,
  correlation ID, idempotency key digest, and resulting `acl_etag`.

Audit does not store service credentials, group membership snapshots, or secret
material. Resource pages show Recent activity and link to the global Audit
explorer rather than creating an ACL-specific history store.

## 8. Acceptance Contract

- A grant can express only the listed Allow capabilities; no Deny or preset
  evaluation is persisted.
- Direct, group, and service-identity subjects are validated against the
  organization and evaluated at action time.
- Full and redacted Admin projections enforce their distinct authorization
  boundaries; Copilot cannot enumerate ACLs.
- Duplicate mutation requests with the same idempotency key return the original
  result; a changed request returns `409 idempotency_conflict`.
- Stale collection or grant ETags return `409 etag_conflict` without partial
  changes.
- Revocation, expiry, Policy changes, membership changes, quarantine, and
  Deployment changes block new work without deleting historical Runs or Audit.
- The final recovery path cannot be removed without owner transfer or the
  separately authorized break-glass path.
- Owner transfer is atomic, idempotent, auditable, gives the new owner explicit
  `access.manage`, and leaves the old owner's other grants unchanged.
