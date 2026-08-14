CREATE SCHEMA IF NOT EXISTS gantry;

CREATE TABLE IF NOT EXISTS gantry.organizations (
  id text PRIMARY KEY, slug text NOT NULL UNIQUE, display_name text NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS gantry.workspaces (
  id text PRIMARY KEY, organization_id text NOT NULL REFERENCES gantry.organizations(id), slug text NOT NULL, display_name text NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), UNIQUE (organization_id, slug)
);
CREATE TABLE IF NOT EXISTS gantry.principals (
  id text PRIMARY KEY, organization_id text NOT NULL REFERENCES gantry.organizations(id), external_subject text NOT NULL UNIQUE, display_name text NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS gantry.workspace_memberships (
  workspace_id text NOT NULL REFERENCES gantry.workspaces(id), principal_id text NOT NULL REFERENCES gantry.principals(id), PRIMARY KEY (workspace_id, principal_id)
);
CREATE TABLE IF NOT EXISTS gantry.role_bindings (
  id text PRIMARY KEY, principal_id text NOT NULL REFERENCES gantry.principals(id), workspace_id text REFERENCES gantry.workspaces(id), role text NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS gantry.agents (
  id text PRIMARY KEY, organization_id text NOT NULL REFERENCES gantry.organizations(id), workspace_id text NOT NULL REFERENCES gantry.workspaces(id), slug text NOT NULL, display_name text NOT NULL, description text NOT NULL, category text NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), UNIQUE (workspace_id, slug)
);
CREATE TABLE IF NOT EXISTS gantry.agent_versions (
  id text PRIMARY KEY, agent_id text NOT NULL REFERENCES gantry.agents(id), version integer NOT NULL, spec_json jsonb NOT NULL, spec_digest text NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), UNIQUE (agent_id, version), UNIQUE (id, agent_id)
);
CREATE TABLE IF NOT EXISTS gantry.agent_publications (
  id text PRIMARY KEY, agent_id text NOT NULL REFERENCES gantry.agents(id), agent_version_id text NOT NULL, workspace_id text NOT NULL REFERENCES gantry.workspaces(id), status text NOT NULL CHECK (status IN ('published', 'retired')), created_at timestamptz NOT NULL DEFAULT now(), UNIQUE (agent_version_id, workspace_id), FOREIGN KEY (agent_version_id, agent_id) REFERENCES gantry.agent_versions(id, agent_id)
);
CREATE TABLE IF NOT EXISTS gantry.tasks (
  id text PRIMARY KEY, organization_id text NOT NULL REFERENCES gantry.organizations(id), workspace_id text NOT NULL REFERENCES gantry.workspaces(id), requester_principal_id text NOT NULL REFERENCES gantry.principals(id), agent_id text NOT NULL REFERENCES gantry.agents(id), input_json jsonb NOT NULL, current_run_id text, status text NOT NULL CHECK (status IN ('queued', 'running', 'canceling', 'completed', 'failed', 'canceled')), created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS gantry.runs (
  id text PRIMARY KEY, task_id text NOT NULL REFERENCES gantry.tasks(id), agent_version_id text NOT NULL REFERENCES gantry.agent_versions(id), attempt_number integer NOT NULL, demo_mode text NOT NULL CHECK (demo_mode IN ('complete', 'await_cancel')), status text NOT NULL CHECK (status IN ('queued', 'assigned', 'accepted', 'canceling', 'completed', 'failed', 'canceled')), status_reason text NOT NULL DEFAULT '', runner_id text, lease_epoch bigint NOT NULL DEFAULT 0, event_sequence bigint NOT NULL DEFAULT 0, runner_event_sequence bigint NOT NULL DEFAULT 0, created_at timestamptz NOT NULL DEFAULT now(), started_at timestamptz, completed_at timestamptz, UNIQUE (task_id, attempt_number)
);
CREATE TABLE IF NOT EXISTS gantry.run_events (
  run_id text NOT NULL REFERENCES gantry.runs(id), sequence bigint NOT NULL, event_type text NOT NULL, payload jsonb NOT NULL DEFAULT '{}'::jsonb, created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY (run_id, sequence)
);
CREATE TABLE IF NOT EXISTS gantry.idempotency_tombstones (
  principal_id text NOT NULL REFERENCES gantry.principals(id), route text NOT NULL, idempotency_key text NOT NULL, request_digest text NOT NULL, task_id text NOT NULL REFERENCES gantry.tasks(id) DEFERRABLE INITIALLY DEFERRED, created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY (principal_id, route, idempotency_key)
);
CREATE INDEX IF NOT EXISTS tasks_requester_created_idx ON gantry.tasks (requester_principal_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS runs_queue_idx ON gantry.runs (status, created_at);
CREATE UNIQUE INDEX IF NOT EXISTS agent_publications_one_current_idx ON gantry.agent_publications (agent_id, workspace_id) WHERE status = 'published';
