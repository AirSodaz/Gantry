CREATE SCHEMA IF NOT EXISTS gantry;

CREATE TABLE IF NOT EXISTS gantry.organizations (
  id text PRIMARY KEY,
  slug text NOT NULL UNIQUE,
  display_name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS gantry.workspaces (
  id text PRIMARY KEY,
  organization_id text NOT NULL REFERENCES gantry.organizations(id),
  slug text NOT NULL,
  display_name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (organization_id, slug)
);

CREATE TABLE IF NOT EXISTS gantry.principals (
  id text PRIMARY KEY,
  organization_id text NOT NULL REFERENCES gantry.organizations(id),
  external_subject text NOT NULL UNIQUE,
  display_name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS gantry.workspace_memberships (
  workspace_id text NOT NULL REFERENCES gantry.workspaces(id),
  principal_id text NOT NULL REFERENCES gantry.principals(id),
  PRIMARY KEY (workspace_id, principal_id)
);

CREATE TABLE IF NOT EXISTS gantry.role_bindings (
  id text PRIMARY KEY,
  principal_id text NOT NULL REFERENCES gantry.principals(id),
  workspace_id text REFERENCES gantry.workspaces(id),
  role text NOT NULL CHECK (role IN ('organization_admin', 'workspace_agent_editor')),
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK (
    (role = 'organization_admin' AND workspace_id IS NULL) OR
    (role = 'workspace_agent_editor' AND workspace_id IS NOT NULL)
  )
);
CREATE UNIQUE INDEX IF NOT EXISTS role_bindings_organization_admin_idx
  ON gantry.role_bindings (principal_id, role) WHERE workspace_id IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS role_bindings_workspace_role_idx
  ON gantry.role_bindings (principal_id, workspace_id, role) WHERE workspace_id IS NOT NULL;

-- Configuration assets are immutable execution inputs.  The catalog records
-- metadata and digests only; package contents and credentials stay behind
-- trusted adapters or object storage.
CREATE TABLE IF NOT EXISTS gantry.skills (
  id text PRIMARY KEY,
  organization_id text NOT NULL REFERENCES gantry.organizations(id),
  workspace_id text NOT NULL REFERENCES gantry.workspaces(id),
  slug text NOT NULL,
  display_name text NOT NULL,
  description text NOT NULL DEFAULT '',
  source_type text NOT NULL CHECK (source_type IN ('marketplace', 'locator', 'upload', 'local')),
  source_ref text NOT NULL,
  declared_version text NOT NULL DEFAULT '',
  content_digest text NOT NULL,
  status text NOT NULL CHECK (status IN ('available', 'deprecated', 'retired')),
  created_by_principal_id text NOT NULL REFERENCES gantry.principals(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (workspace_id, slug, content_digest)
);
ALTER TABLE gantry.skills
  ADD COLUMN IF NOT EXISTS metadata_json jsonb NOT NULL DEFAULT '{}'::jsonb;
CREATE INDEX IF NOT EXISTS skills_workspace_idx ON gantry.skills (workspace_id, status, display_name);

CREATE TABLE IF NOT EXISTS gantry.plugins (
  id text PRIMARY KEY,
  organization_id text NOT NULL REFERENCES gantry.organizations(id),
  slug text NOT NULL,
  display_name text NOT NULL,
  description text NOT NULL DEFAULT '',
  version text NOT NULL,
  content_digest text NOT NULL,
  status text NOT NULL CHECK (status IN ('active', 'deprecated', 'retired')),
  created_by_principal_id text NOT NULL REFERENCES gantry.principals(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (organization_id, slug, version, content_digest)
);
ALTER TABLE gantry.plugins
  ADD COLUMN IF NOT EXISTS manifest_json jsonb NOT NULL DEFAULT '{}'::jsonb;
CREATE INDEX IF NOT EXISTS plugins_org_idx ON gantry.plugins (organization_id, status, display_name);

CREATE TABLE IF NOT EXISTS gantry.workspace_plugin_enablements (
  workspace_id text NOT NULL REFERENCES gantry.workspaces(id),
  plugin_id text NOT NULL REFERENCES gantry.plugins(id),
  enabled_by_principal_id text NOT NULL REFERENCES gantry.principals(id),
  enabled_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (workspace_id, plugin_id)
);

CREATE TABLE IF NOT EXISTS gantry.tool_servers (
  id text PRIMARY KEY,
  organization_id text NOT NULL REFERENCES gantry.organizations(id),
  name text NOT NULL,
  server_type text NOT NULL CHECK (server_type IN ('builtin', 'mcp', 'cli')),
  endpoint_ref text NOT NULL DEFAULT '',
  status text NOT NULL CHECK (status IN ('active', 'degraded', 'retired')),
  created_by_principal_id text NOT NULL REFERENCES gantry.principals(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (organization_id, name)
);

CREATE TABLE IF NOT EXISTS gantry.tool_descriptors (
  id text PRIMARY KEY,
  server_id text NOT NULL REFERENCES gantry.tool_servers(id),
  fully_qualified_name text NOT NULL,
  version text NOT NULL,
  effect text NOT NULL CHECK (effect IN ('read', 'write', 'external_side_effect', 'administrative')),
  idempotency text NOT NULL CHECK (idempotency IN ('read_only', 'idempotent', 'compensatable', 'non_repeatable')),
  schema_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  content_digest text NOT NULL,
  status text NOT NULL CHECK (status IN ('active', 'proposed', 'deprecated', 'retired')),
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (server_id, fully_qualified_name, version, content_digest)
);
CREATE INDEX IF NOT EXISTS tool_descriptors_server_idx ON gantry.tool_descriptors (server_id, status, fully_qualified_name);

CREATE TABLE IF NOT EXISTS gantry.agents (
  id text PRIMARY KEY,
  organization_id text NOT NULL REFERENCES gantry.organizations(id),
  workspace_id text NOT NULL REFERENCES gantry.workspaces(id),
  owner_principal_id text NOT NULL REFERENCES gantry.principals(id),
  slug text NOT NULL,
  display_name text NOT NULL,
  description text NOT NULL,
  category text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (workspace_id, slug)
);

-- The target Agent lifecycle is intentionally flat. Named Draft working copies,
-- immutable hash-addressed Revisions, and Deployments are separate resources.
CREATE TABLE IF NOT EXISTS gantry.agent_draft_workspaces (
  id text PRIMARY KEY,
  agent_id text NOT NULL REFERENCES gantry.agents(id),
  name text NOT NULL,
  status text NOT NULL CHECK (status IN ('active', 'archived')),
  derived_from_revision_hash text NOT NULL DEFAULT '',
  latest_revision_hash text NOT NULL DEFAULT '',
  spec_json jsonb NOT NULL,
  schema_version text NOT NULL DEFAULT 'gantry.agent/v1',
  working_copy_etag integer NOT NULL CHECK (working_copy_etag > 0),
  validation_status text NOT NULL CHECK (validation_status IN ('valid', 'invalid')),
  validation_findings jsonb NOT NULL DEFAULT '[]'::jsonb,
  created_by_principal_id text NOT NULL REFERENCES gantry.principals(id),
  updated_by_principal_id text NOT NULL REFERENCES gantry.principals(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (agent_id, name)
);
CREATE INDEX IF NOT EXISTS agent_draft_workspaces_agent_idx
  ON gantry.agent_draft_workspaces (agent_id, status, updated_at DESC);

CREATE TABLE IF NOT EXISTS gantry.agent_revisions (
  id text PRIMARY KEY,
  agent_id text NOT NULL REFERENCES gantry.agents(id),
  revision_hash text NOT NULL UNIQUE,
  source_draft_id text NOT NULL REFERENCES gantry.agent_draft_workspaces(id),
  message text NOT NULL,
  spec_json jsonb NOT NULL,
  spec_digest text NOT NULL,
  runtime_image_digest text NOT NULL DEFAULT '',
  created_by_principal_id text NOT NULL REFERENCES gantry.principals(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  prompt_snapshot_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  prompt_snapshot_digest text NOT NULL DEFAULT '',
  prompt_compiler_version text NOT NULL DEFAULT 'prompt-compiler/v1',
  UNIQUE (id, agent_id)
);
CREATE INDEX IF NOT EXISTS agent_revisions_agent_idx
  ON gantry.agent_revisions (agent_id, created_at DESC);

CREATE TABLE IF NOT EXISTS gantry.agent_revision_reviews (
  id text PRIMARY KEY,
  agent_id text NOT NULL REFERENCES gantry.agents(id),
  revision_id text NOT NULL,
  revision_hash text NOT NULL,
  base_revision_hash text NOT NULL DEFAULT '',
  release_notes text NOT NULL DEFAULT '',
  diff_json jsonb NOT NULL DEFAULT '[]'::jsonb,
  risk_summary jsonb NOT NULL DEFAULT '{}'::jsonb,
  status text NOT NULL CHECK (status IN ('pending', 'approved', 'rejected', 'superseded')),
  submitted_by_principal_id text NOT NULL REFERENCES gantry.principals(id),
  reviewed_by_principal_id text REFERENCES gantry.principals(id),
  review_reason text NOT NULL DEFAULT '',
  submitted_at timestamptz NOT NULL DEFAULT now(),
  reviewed_at timestamptz,
  UNIQUE (revision_id),
  FOREIGN KEY (revision_id, agent_id) REFERENCES gantry.agent_revisions(id, agent_id)
);
CREATE INDEX IF NOT EXISTS agent_revision_reviews_agent_idx
  ON gantry.agent_revision_reviews (agent_id, status, submitted_at DESC);

CREATE TABLE IF NOT EXISTS gantry.agent_deployments (
  id text PRIMARY KEY,
  agent_id text NOT NULL REFERENCES gantry.agents(id),
  workspace_id text NOT NULL REFERENCES gantry.workspaces(id),
  name text NOT NULL,
  environment_kind text NOT NULL CHECK (environment_kind IN ('test', 'production')),
  revision_id text NOT NULL,
  revision_hash text NOT NULL,
  spec_digest text NOT NULL,
  status text NOT NULL CHECK (status IN ('active', 'stopped', 'quarantined')),
  owner_principal_id text REFERENCES gantry.principals(id),
  purpose text NOT NULL DEFAULT '',
  expires_at timestamptz,
  environment_policy jsonb NOT NULL DEFAULT '{}'::jsonb,
  changed_by_principal_id text NOT NULL REFERENCES gantry.principals(id),
  review_id text REFERENCES gantry.agent_revision_reviews(id),
  previous_revision_hash text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (agent_id, name),
  FOREIGN KEY (revision_id, agent_id) REFERENCES gantry.agent_revisions(id, agent_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS agent_deployments_one_production_idx
  ON gantry.agent_deployments (agent_id) WHERE environment_kind='production' AND status='active';
CREATE INDEX IF NOT EXISTS agent_deployments_agent_idx
  ON gantry.agent_deployments (agent_id, environment_kind, status, updated_at DESC);

CREATE TABLE IF NOT EXISTS gantry.audit_events (
  id bigserial PRIMARY KEY,
  organization_id text NOT NULL REFERENCES gantry.organizations(id),
  actor_principal_id text NOT NULL REFERENCES gantry.principals(id),
  resource_type text NOT NULL,
  resource_id text NOT NULL,
  event_type text NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS audit_events_resource_idx ON gantry.audit_events (resource_type, resource_id, created_at DESC);

CREATE TABLE IF NOT EXISTS gantry.audit_exports (
  id text PRIMARY KEY,
  organization_id text NOT NULL REFERENCES gantry.organizations(id),
  requested_by_principal_id text NOT NULL REFERENCES gantry.principals(id),
  query_json jsonb NOT NULL,
  query_digest text NOT NULL,
  scope text NOT NULL,
  state text NOT NULL CHECK (state IN ('requested', 'processing', 'ready', 'expired', 'failed')),
  package_digest text NOT NULL DEFAULT '',
  object_key text NOT NULL DEFAULT '',
  download_count integer NOT NULL DEFAULT 0,
  expires_at timestamptz,
  failure_reason text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE gantry.audit_exports
  ADD COLUMN IF NOT EXISTS download_count integer NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS audit_exports_owner_idx ON gantry.audit_exports (organization_id, requested_by_principal_id, created_at DESC);

CREATE TABLE IF NOT EXISTS gantry.tasks (
  id text PRIMARY KEY,
  organization_id text NOT NULL REFERENCES gantry.organizations(id),
  workspace_id text NOT NULL REFERENCES gantry.workspaces(id),
  requester_principal_id text NOT NULL REFERENCES gantry.principals(id),
  agent_id text NOT NULL REFERENCES gantry.agents(id),
  input_json jsonb NOT NULL,
  current_run_id text,
  status text NOT NULL CHECK (status IN ('queued', 'running', 'awaiting_approval', 'canceling', 'completed', 'failed', 'canceled')),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS gantry.runs (
  id text PRIMARY KEY,
  task_id text NOT NULL REFERENCES gantry.tasks(id),
  agent_revision_id text NOT NULL REFERENCES gantry.agent_revisions(id),
  deployment_id text REFERENCES gantry.agent_deployments(id),
  manifest_digest text NOT NULL DEFAULT '',
  attempt_number integer NOT NULL,
  status text NOT NULL CHECK (status IN ('queued', 'assigned', 'accepted', 'awaiting_approval', 'canceling', 'completed', 'failed', 'canceled')),
  status_reason text NOT NULL DEFAULT '',
  runner_id text,
  lease_epoch bigint NOT NULL DEFAULT 0,
  event_sequence bigint NOT NULL DEFAULT 0,
  runner_event_sequence bigint NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  started_at timestamptz,
  completed_at timestamptz,
  UNIQUE (task_id, attempt_number)
);
ALTER TABLE gantry.runs
  ADD COLUMN IF NOT EXISTS deployment_id text REFERENCES gantry.agent_deployments(id);
ALTER TABLE gantry.runs
  ADD COLUMN IF NOT EXISTS manifest_digest text NOT NULL DEFAULT '';
CREATE TABLE IF NOT EXISTS gantry.run_events (
  run_id text NOT NULL REFERENCES gantry.runs(id),
  sequence bigint NOT NULL,
  event_type text NOT NULL,
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (run_id, sequence)
);

CREATE TABLE IF NOT EXISTS gantry.run_content_segments (
  id text PRIMARY KEY,
  run_id text NOT NULL REFERENCES gantry.runs(id),
  stream_id text NOT NULL,
  start_offset bigint NOT NULL CHECK (start_offset >= 0),
  end_offset bigint NOT NULL CHECK (end_offset > start_offset),
  object_key text NOT NULL UNIQUE,
  digest text NOT NULL,
  size_bytes bigint NOT NULL CHECK (size_bytes > 0),
  media_type text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (run_id, stream_id, start_offset),
  CHECK (end_offset > start_offset)
);
CREATE INDEX IF NOT EXISTS run_content_segments_stream_idx ON gantry.run_content_segments (run_id, stream_id, start_offset);

CREATE TABLE IF NOT EXISTS gantry.artifacts (
  id text PRIMARY KEY,
  task_id text NOT NULL REFERENCES gantry.tasks(id),
  run_id text NOT NULL REFERENCES gantry.runs(id),
  object_key text NOT NULL UNIQUE,
  filename text NOT NULL,
  media_type text NOT NULL,
  size_bytes bigint NOT NULL CHECK (size_bytes >= 0),
  digest text NOT NULL,
  classification text NOT NULL DEFAULT 'internal',
  scan_status text NOT NULL CHECK (scan_status IN ('pending', 'passed', 'failed')),
  visibility text NOT NULL CHECK (visibility IN ('requester', 'workspace')),
  state text NOT NULL CHECK (state IN ('declared', 'uploaded', 'available', 'rejected')),
  upload_token_hash text NOT NULL DEFAULT '',
  upload_lease_epoch bigint NOT NULL DEFAULT 0,
  upload_expires_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  uploaded_at timestamptz,
  UNIQUE (run_id, filename)
);
CREATE INDEX IF NOT EXISTS artifacts_task_idx ON gantry.artifacts (task_id, created_at);

CREATE TABLE IF NOT EXISTS gantry.idempotency_tombstones (
  principal_id text NOT NULL REFERENCES gantry.principals(id),
  route text NOT NULL,
  idempotency_key text NOT NULL,
  request_digest text NOT NULL,
  task_id text NOT NULL REFERENCES gantry.tasks(id) DEFERRABLE INITIALLY DEFERRED,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (principal_id, route, idempotency_key)
);

CREATE INDEX IF NOT EXISTS tasks_requester_created_idx ON gantry.tasks (requester_principal_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS runs_queue_idx ON gantry.runs (status, created_at);

CREATE TABLE IF NOT EXISTS gantry.actions (
  id text PRIMARY KEY,
  run_id text NOT NULL REFERENCES gantry.runs(id),
  runner_call_id text NOT NULL DEFAULT '',
  tool_name text NOT NULL,
  operation text NOT NULL,
  arguments_json jsonb NOT NULL,
  target text NOT NULL DEFAULT '',
  effect text NOT NULL CHECK (effect IN ('read', 'write', 'destructive')),
  credential_ref text NOT NULL DEFAULT '',
  credential_mode text NOT NULL DEFAULT '',
  policy_version text NOT NULL,
  action_digest text NOT NULL UNIQUE,
  state text NOT NULL CHECK (state IN ('proposed', 'awaiting_approval', 'ready', 'executing', 'succeeded', 'failed', 'rejected', 'unknown_outcome')),
  revision integer NOT NULL CHECK (revision > 0),
  requested_by_principal_id text NOT NULL REFERENCES gantry.principals(id),
  execution_permit_id text NOT NULL DEFAULT '',
  execution_permit_lease_epoch bigint NOT NULL DEFAULT 0,
  execution_permit_expires_at timestamptz,
  execution_claimed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS actions_run_call_idx ON gantry.actions (run_id, runner_call_id);
CREATE UNIQUE INDEX IF NOT EXISTS actions_execution_permit_idx ON gantry.actions (execution_permit_id) WHERE execution_permit_id <> '';


CREATE TABLE IF NOT EXISTS gantry.approval_requests (
  id text PRIMARY KEY,
  action_id text NOT NULL UNIQUE REFERENCES gantry.actions(id),
  run_id text NOT NULL REFERENCES gantry.runs(id),
  action_digest text NOT NULL,
  action_preview jsonb NOT NULL,
  risk_class text NOT NULL,
  status text NOT NULL CHECK (status IN ('pending', 'satisfied', 'rejected', 'expired', 'superseded')),
  requested_by_principal_id text NOT NULL REFERENCES gantry.principals(id),
  assigned_principal_id text REFERENCES gantry.principals(id),
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  decided_at timestamptz
);

CREATE TABLE IF NOT EXISTS gantry.approval_decisions (
  approval_id text NOT NULL REFERENCES gantry.approval_requests(id),
  principal_id text NOT NULL REFERENCES gantry.principals(id),
  decision text NOT NULL CHECK (decision IN ('approve', 'reject')),
  reason text NOT NULL DEFAULT '',
  action_digest text NOT NULL,
  idempotency_key text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (approval_id, principal_id),
  UNIQUE (approval_id, principal_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS approval_requests_assignee_idx ON gantry.approval_requests (assigned_principal_id, status, created_at);

-- Policies own typed governance documents independently from runtime action
-- evaluation. Drafts are mutable working copies; versions and bindings are
-- immutable evidence projections.
CREATE TABLE IF NOT EXISTS gantry.policies (
  id text PRIMARY KEY,
  organization_id text NOT NULL REFERENCES gantry.organizations(id),
  workspace_id text REFERENCES gantry.workspaces(id),
  type text NOT NULL CHECK (type IN ('approval', 'model', 'tool', 'command', 'network', 'credential', 'data', 'budget', 'retention', 'evaluation', 'publication')),
  name text NOT NULL,
  owner_principal_id text NOT NULL REFERENCES gantry.principals(id),
  state text NOT NULL CHECK (state IN ('draft', 'published', 'retired')),
  schema_version text NOT NULL,
  draft_etag integer NOT NULL CHECK (draft_etag > 0),
  latest_version_id text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (organization_id, workspace_id, name)
);
CREATE INDEX IF NOT EXISTS policies_scope_idx ON gantry.policies (organization_id, workspace_id, state, name);

CREATE TABLE IF NOT EXISTS gantry.policy_drafts (
  policy_id text PRIMARY KEY REFERENCES gantry.policies(id),
  document jsonb NOT NULL,
  schema_version text NOT NULL,
  etag integer NOT NULL CHECK (etag > 0),
  validation_state text NOT NULL CHECK (validation_state IN ('valid', 'invalid', 'pending')),
  validation_findings jsonb NOT NULL DEFAULT '[]'::jsonb,
  updated_by_principal_id text NOT NULL REFERENCES gantry.principals(id),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS gantry.policy_versions (
  id text PRIMARY KEY,
  policy_id text NOT NULL REFERENCES gantry.policies(id),
  content_digest text NOT NULL,
  schema_version text NOT NULL,
  message text NOT NULL,
  document jsonb NOT NULL,
  compiler_evidence jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_by_principal_id text NOT NULL REFERENCES gantry.principals(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (policy_id, content_digest)
);
CREATE INDEX IF NOT EXISTS policy_versions_policy_idx ON gantry.policy_versions (policy_id, created_at, id);

CREATE TABLE IF NOT EXISTS gantry.policy_bindings (
  id text PRIMARY KEY,
  policy_id text NOT NULL REFERENCES gantry.policies(id),
  version_id text NOT NULL REFERENCES gantry.policy_versions(id),
  target_scope text NOT NULL CHECK (target_scope IN ('organization', 'workspace')),
  target_workspace_id text REFERENCES gantry.workspaces(id),
  target_resource_id text,
  environment text NOT NULL CHECK (environment IN ('development', 'staging', 'production')),
  state text NOT NULL CHECK (state IN ('pending', 'active', 'expired', 'revoked')),
  effective_from timestamptz NOT NULL DEFAULT now(),
  effective_until timestamptz,
  reason text NOT NULL DEFAULT '',
  created_by_principal_id text NOT NULL REFERENCES gantry.principals(id),
  revoked_at timestamptz,
  CHECK ((target_scope = 'organization' AND target_workspace_id IS NULL) OR (target_scope = 'workspace' AND target_workspace_id IS NOT NULL))
);
CREATE INDEX IF NOT EXISTS policy_bindings_policy_idx ON gantry.policy_bindings (policy_id, state, effective_from DESC);

CREATE TABLE IF NOT EXISTS gantry.policy_command_idempotency (
  principal_id text NOT NULL REFERENCES gantry.principals(id),
  route text NOT NULL,
  idempotency_key text NOT NULL,
  request_digest text NOT NULL,
  response_json jsonb NOT NULL,
  status_code integer NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (principal_id, route, idempotency_key)
);

CREATE TABLE IF NOT EXISTS gantry.evaluation_suites (
  id text PRIMARY KEY,
  organization_id text NOT NULL REFERENCES gantry.organizations(id),
  workspace_id text NOT NULL REFERENCES gantry.workspaces(id),
  name text NOT NULL,
  state text NOT NULL CHECK (state IN ('draft', 'published', 'retired')),
  owner_principal_id text NOT NULL REFERENCES gantry.principals(id),
  latest_version_id text,
  etag integer NOT NULL CHECK (etag > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (workspace_id, name)
);
CREATE INDEX IF NOT EXISTS evaluation_suites_workspace_idx ON gantry.evaluation_suites (workspace_id, state, name);

CREATE TABLE IF NOT EXISTS gantry.evaluation_cases (
  id text PRIMARY KEY,
  suite_id text NOT NULL REFERENCES gantry.evaluation_suites(id),
  input_json jsonb NOT NULL,
  fixture_manifest_json jsonb NOT NULL,
  assertions_json jsonb NOT NULL,
  rubric_json jsonb,
  compatibility_json jsonb NOT NULL DEFAULT '{}'::jsonb,
  etag integer NOT NULL CHECK (etag > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS evaluation_cases_suite_idx ON gantry.evaluation_cases (suite_id, id);

CREATE TABLE IF NOT EXISTS gantry.evaluation_suite_versions (
  id text PRIMARY KEY,
  suite_id text NOT NULL REFERENCES gantry.evaluation_suites(id),
  content_digest text NOT NULL,
  case_manifest_digest text NOT NULL,
  fixture_manifest_digest text NOT NULL,
  evaluator_policy_version_id text NOT NULL DEFAULT '',
  runtime_image_digest text NOT NULL,
  published_at timestamptz NOT NULL DEFAULT now(),
  created_by_principal_id text NOT NULL REFERENCES gantry.principals(id),
  UNIQUE (suite_id, content_digest)
);
CREATE INDEX IF NOT EXISTS evaluation_suite_versions_suite_idx ON gantry.evaluation_suite_versions (suite_id, published_at, id);

CREATE TABLE IF NOT EXISTS gantry.evaluation_runs (
  id text PRIMARY KEY,
  suite_version_id text NOT NULL REFERENCES gantry.evaluation_suite_versions(id),
  candidate_revision_hash text NOT NULL,
  baseline_revision_hash text,
  environment_digest text NOT NULL,
  state text NOT NULL CHECK (state IN ('requested', 'queued', 'provisioning', 'running', 'completed', 'failed', 'canceled', 'invalid')),
  gate_result text NOT NULL CHECK (gate_result IN ('not_applicable', 'passed', 'failed', 'blocked', 'invalid')),
  deterministic_summary jsonb NOT NULL DEFAULT '{}'::jsonb,
  probabilistic_summary jsonb,
  evidence_manifest_digest text,
  requested_by_principal_id text NOT NULL REFERENCES gantry.principals(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS evaluation_runs_suite_idx ON gantry.evaluation_runs (suite_version_id, created_at DESC);

CREATE TABLE IF NOT EXISTS gantry.evaluation_gates (
  id text PRIMARY KEY,
  evaluation_run_id text NOT NULL UNIQUE REFERENCES gantry.evaluation_runs(id),
  agent_revision_hash text NOT NULL,
  suite_version_id text NOT NULL REFERENCES gantry.evaluation_suite_versions(id),
  requirement jsonb NOT NULL DEFAULT '{}'::jsonb,
  state text NOT NULL CHECK (state IN ('required', 'passed', 'failed', 'overridden', 'expired')),
  override_id text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS evaluation_gates_revision_idx ON gantry.evaluation_gates (agent_revision_hash, state, created_at DESC);

CREATE TABLE IF NOT EXISTS gantry.evaluation_gate_overrides (
  id text PRIMARY KEY,
  gate_id text NOT NULL REFERENCES gantry.evaluation_gates(id),
  reason text NOT NULL,
  reviewer_principal_id text NOT NULL REFERENCES gantry.principals(id),
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS evaluation_gate_overrides_gate_idx ON gantry.evaluation_gate_overrides (gate_id, expires_at DESC);

CREATE TABLE IF NOT EXISTS gantry.evaluation_command_idempotency (
  principal_id text NOT NULL REFERENCES gantry.principals(id),
  route text NOT NULL,
  idempotency_key text NOT NULL,
  request_digest text NOT NULL,
  response_json jsonb NOT NULL,
  PRIMARY KEY (principal_id, route, idempotency_key)
);

CREATE TABLE IF NOT EXISTS gantry.integrations (
  id text PRIMARY KEY,
  organization_id text NOT NULL REFERENCES gantry.organizations(id),
  slug text NOT NULL,
  display_name text NOT NULL,
  state text NOT NULL CHECK (state IN ('active', 'disabled', 'retired')),
  owner_principal_id text NOT NULL REFERENCES gantry.principals(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (organization_id, slug)
);
CREATE INDEX IF NOT EXISTS integrations_org_idx ON gantry.integrations (organization_id, state, display_name);

CREATE TABLE IF NOT EXISTS gantry.integration_clients (
  id text PRIMARY KEY,
  integration_id text NOT NULL REFERENCES gantry.integrations(id),
  environment text NOT NULL CHECK (environment IN ('development', 'staging', 'production')),
  auth_modes jsonb NOT NULL DEFAULT '[]'::jsonb,
  audience text NOT NULL DEFAULT '',
  status text NOT NULL CHECK (status IN ('active', 'disabled', 'expired', 'revoked')),
  credential_fingerprint text NOT NULL,
  expires_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (integration_id, environment)
);
CREATE INDEX IF NOT EXISTS integration_clients_integration_idx ON gantry.integration_clients (integration_id, environment);

CREATE TABLE IF NOT EXISTS gantry.integration_publications (
  id text PRIMARY KEY,
  integration_id text NOT NULL REFERENCES gantry.integrations(id),
  client_id text NOT NULL REFERENCES gantry.integration_clients(id),
  workspace_id text NOT NULL REFERENCES gantry.workspaces(id),
  environment text NOT NULL CHECK (environment IN ('development', 'staging', 'production')),
  revision_hash text NOT NULL,
  input_contract_digest text NOT NULL,
  output_contract_digest text NOT NULL,
  authority_modes jsonb NOT NULL DEFAULT '[]'::jsonb,
  state text NOT NULL CHECK (state IN ('draft', 'active', 'expired', 'revoked')),
  effective_until timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS integration_publications_integration_idx ON gantry.integration_publications (integration_id, state, created_at DESC);

CREATE TABLE IF NOT EXISTS gantry.webhook_endpoints (
  id text PRIMARY KEY,
  integration_id text NOT NULL REFERENCES gantry.integrations(id),
  environment text NOT NULL,
  destination text NOT NULL,
  status text NOT NULL CHECK (status IN ('active', 'disabled', 'quarantined', 'retired')),
  signing_key_fingerprint text NOT NULL,
  subscribed_events jsonb NOT NULL DEFAULT '[]'::jsonb,
  retry_policy jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS webhook_endpoints_integration_idx ON gantry.webhook_endpoints (integration_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS gantry.webhook_deliveries (
  id text PRIMARY KEY,
  endpoint_id text NOT NULL REFERENCES gantry.webhook_endpoints(id),
  event_id text NOT NULL,
  delivery_id text NOT NULL,
  attempt integer NOT NULL CHECK (attempt > 0),
  state text NOT NULL CHECK (state IN ('queued', 'delivered', 'retrying', 'failed', 'canceled')),
  response_class text,
  next_attempt_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (endpoint_id, delivery_id, attempt)
);

CREATE TABLE IF NOT EXISTS gantry.platform_model_providers (
  id text PRIMARY KEY,
  organization_id text NOT NULL REFERENCES gantry.organizations(id),
  name text NOT NULL,
  state text NOT NULL CHECK (state IN ('active', 'degraded', 'disabled', 'quarantined')),
  data_classes jsonb NOT NULL DEFAULT '[]'::jsonb,
  credential_reference_id text NOT NULL,
  health jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (organization_id, name)
);
CREATE INDEX IF NOT EXISTS platform_model_providers_org_idx ON gantry.platform_model_providers (organization_id, state, name);

CREATE TABLE IF NOT EXISTS gantry.platform_provider_routes (
  id text PRIMARY KEY,
  provider_id text NOT NULL REFERENCES gantry.platform_model_providers(id),
  allowed_models jsonb NOT NULL DEFAULT '[]'::jsonb,
  fallback_route_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
  state text NOT NULL CHECK (state IN ('active', 'degraded', 'disabled')),
  budget_policy_id text,
  classification_constraints jsonb NOT NULL DEFAULT '{}'::jsonb,
  etag bigint NOT NULL DEFAULT 1 CHECK (etag > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS platform_provider_routes_provider_idx ON gantry.platform_provider_routes (provider_id, state, id);

CREATE TABLE IF NOT EXISTS gantry.platform_runner_pools (
  id text PRIMARY KEY,
  organization_id text NOT NULL REFERENCES gantry.organizations(id),
  isolation_tier text NOT NULL CHECK (isolation_tier IN ('development', 'gvisor', 'microvm')),
  state text NOT NULL CHECK (state IN ('active', 'draining', 'quarantined', 'disabled')),
  compatible_protocols jsonb NOT NULL DEFAULT '[]'::jsonb,
  capacity jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS platform_runner_pools_org_idx ON gantry.platform_runner_pools (organization_id, state, id);

CREATE TABLE IF NOT EXISTS gantry.platform_runners (
  id text PRIMARY KEY,
  pool_id text NOT NULL REFERENCES gantry.platform_runner_pools(id),
  state text NOT NULL CHECK (state IN ('ready', 'assigned', 'draining', 'quarantined', 'offline')),
  protocol_version text NOT NULL,
  lease_epoch bigint NOT NULL DEFAULT 0 CHECK (lease_epoch >= 0),
  last_heartbeat_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS platform_runners_pool_idx ON gantry.platform_runners (pool_id, state, id);

CREATE TABLE IF NOT EXISTS gantry.platform_credential_references (
  id text PRIMARY KEY,
  organization_id text NOT NULL REFERENCES gantry.organizations(id),
  target_service text NOT NULL,
  state text NOT NULL CHECK (state IN ('active', 'rotating', 'expired', 'revoked', 'disabled')),
  classification text NOT NULL,
  allowed_modes jsonb NOT NULL DEFAULT '[]'::jsonb,
  secret_version text,
  expires_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS platform_credential_references_org_idx ON gantry.platform_credential_references (organization_id, state, target_service);

CREATE TABLE IF NOT EXISTS gantry.platform_data_classifications (
  id text PRIMARY KEY,
  organization_id text NOT NULL REFERENCES gantry.organizations(id),
  label text NOT NULL,
  handling text NOT NULL CHECK (handling IN ('public', 'internal', 'confidential', 'restricted')),
  retention_class text NOT NULL,
  allowed_provider_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
  allowed_tool_classes jsonb NOT NULL DEFAULT '[]'::jsonb,
  etag bigint NOT NULL DEFAULT 1 CHECK (etag > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (organization_id, label)
);
CREATE INDEX IF NOT EXISTS platform_data_classifications_org_idx ON gantry.platform_data_classifications (organization_id, label);

CREATE TABLE IF NOT EXISTS gantry.platform_limit_policies (
  id text PRIMARY KEY,
  organization_id text NOT NULL REFERENCES gantry.organizations(id),
  workspace_id text REFERENCES gantry.workspaces(id),
  concurrency integer NOT NULL CHECK (concurrency >= 0),
  duration_seconds integer NOT NULL CHECK (duration_seconds > 0),
  output_bytes bigint NOT NULL DEFAULT 0 CHECK (output_bytes >= 0),
  artifact_bytes bigint NOT NULL DEFAULT 0 CHECK (artifact_bytes >= 0),
  budget jsonb NOT NULL DEFAULT '{}'::jsonb,
  etag bigint NOT NULL DEFAULT 1 CHECK (etag > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE NULLS NOT DISTINCT (organization_id, workspace_id)
);
CREATE INDEX IF NOT EXISTS platform_limit_policies_scope_idx ON gantry.platform_limit_policies (organization_id, workspace_id);

CREATE TABLE IF NOT EXISTS gantry.platform_environment_profiles (
  id text PRIMARY KEY,
  organization_id text NOT NULL REFERENCES gantry.organizations(id),
  workspace_id text REFERENCES gantry.workspaces(id),
  name text NOT NULL CHECK (name IN ('development', 'staging', 'production')),
  publication_posture text NOT NULL CHECK (publication_posture IN ('test_only', 'review_required', 'production')),
  state text NOT NULL CHECK (state IN ('active', 'emergency', 'disabled')),
  data_classification_id text,
  allowed_target_controls jsonb NOT NULL DEFAULT '{}'::jsonb,
  etag bigint NOT NULL DEFAULT 1 CHECK (etag > 0),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE NULLS NOT DISTINCT (organization_id, workspace_id, name)
);
CREATE INDEX IF NOT EXISTS platform_environment_profiles_scope_idx ON gantry.platform_environment_profiles (organization_id, workspace_id, name);

CREATE TABLE IF NOT EXISTS gantry.platform_settings (
  id text PRIMARY KEY,
  organization_id text NOT NULL REFERENCES gantry.organizations(id),
  workspace_id text REFERENCES gantry.workspaces(id),
  values jsonb NOT NULL DEFAULT '{}'::jsonb,
  etag bigint NOT NULL DEFAULT 1 CHECK (etag > 0),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE NULLS NOT DISTINCT (organization_id, workspace_id)
);
