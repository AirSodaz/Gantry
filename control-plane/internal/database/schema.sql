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

CREATE TABLE IF NOT EXISTS gantry.agent_drafts (
  agent_id text PRIMARY KEY REFERENCES gantry.agents(id),
  revision integer NOT NULL CHECK (revision > 0),
  spec_json jsonb NOT NULL,
  validation_status text NOT NULL CHECK (validation_status IN ('valid', 'invalid')),
  validation_findings jsonb NOT NULL DEFAULT '[]'::jsonb,
  updated_by_principal_id text NOT NULL REFERENCES gantry.principals(id),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS gantry.agent_versions (
  id text PRIMARY KEY,
  agent_id text NOT NULL REFERENCES gantry.agents(id),
  version integer NOT NULL CHECK (version > 0),
  source_draft_revision integer NOT NULL CHECK (source_draft_revision > 0),
  spec_json jsonb NOT NULL,
  spec_digest text NOT NULL,
  created_by_principal_id text NOT NULL REFERENCES gantry.principals(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (agent_id, version),
  UNIQUE (agent_id, source_draft_revision),
  UNIQUE (id, agent_id)
);

CREATE TABLE IF NOT EXISTS gantry.agent_publications (
  id text PRIMARY KEY,
  agent_id text NOT NULL REFERENCES gantry.agents(id),
  agent_version_id text NOT NULL,
  workspace_id text NOT NULL REFERENCES gantry.workspaces(id),
  status text NOT NULL CHECK (status IN ('published', 'retired')),
  published_by_principal_id text NOT NULL REFERENCES gantry.principals(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  retired_at timestamptz,
  UNIQUE (agent_version_id, workspace_id),
  FOREIGN KEY (agent_version_id, agent_id) REFERENCES gantry.agent_versions(id, agent_id)
);
CREATE UNIQUE INDEX IF NOT EXISTS agent_publications_one_current_idx
  ON gantry.agent_publications (agent_id, workspace_id) WHERE status = 'published';

CREATE TABLE IF NOT EXISTS gantry.agent_reviews (
  id text PRIMARY KEY,
  agent_id text NOT NULL REFERENCES gantry.agents(id),
  draft_revision integer NOT NULL CHECK (draft_revision > 0),
  draft_digest text NOT NULL,
  base_version_id text REFERENCES gantry.agent_versions(id),
  release_notes text NOT NULL DEFAULT '',
  diff_json jsonb NOT NULL DEFAULT '[]'::jsonb,
  risk_summary jsonb NOT NULL DEFAULT '{}'::jsonb,
  status text NOT NULL CHECK (status IN ('pending', 'approved', 'rejected', 'superseded')),
  submitted_by_principal_id text NOT NULL REFERENCES gantry.principals(id),
  reviewed_by_principal_id text REFERENCES gantry.principals(id),
  review_reason text NOT NULL DEFAULT '',
  submitted_at timestamptz NOT NULL DEFAULT now(),
  reviewed_at timestamptz,
  UNIQUE (agent_id, draft_revision)
);
CREATE INDEX IF NOT EXISTS agent_reviews_current_idx ON gantry.agent_reviews (agent_id, status, draft_revision DESC);

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
  agent_version_id text NOT NULL REFERENCES gantry.agent_versions(id),
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
