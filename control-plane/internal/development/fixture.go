// Package development owns local-only records used by Compose and smoke tests.
package development

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	OrganizationID       = "org_development"
	WorkspaceID          = "wsp_development"
	DemoAgentID          = "agt_lifecycle_demo"
	DemoVersionID        = "agtv_lifecycle_demo_1"
	AwaitCancelAgentID   = "agt_lifecycle_await_cancel"
	AwaitCancelVersionID = "agtv_lifecycle_await_cancel_1"
	DemoPrincipalID      = "prn_copilot_demo"
	OtherPrincipalID     = "prn_copilot_other"
	AdminPrincipalID     = "prn_admin_demo"
	DemoSubject          = "11111111-1111-1111-1111-111111111111"
	OtherSubject         = "22222222-2222-2222-2222-222222222222"
	AdminSubject         = "33333333-3333-3333-3333-333333333333"
)

func Seed(ctx context.Context, pool *pgxpool.Pool) error {
	completeSpec := `{"kind":"gantry.phase0.demo/v1","mode":"complete"}`
	awaitCancelSpec := `{"kind":"gantry.phase0.demo/v1","mode":"await_cancel"}`
	completeDigest := sha256.Sum256([]byte(completeSpec))
	awaitCancelDigest := sha256.Sum256([]byte(awaitCancelSpec))
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	statements := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO gantry.organizations (id, slug, display_name) VALUES ($1, 'development', 'Gantry Development') ON CONFLICT (id) DO NOTHING`, []any{OrganizationID}},
		{`INSERT INTO gantry.workspaces (id, organization_id, slug, display_name) VALUES ($1, $2, 'development', 'Development') ON CONFLICT (id) DO NOTHING`, []any{WorkspaceID, OrganizationID}},
		{`INSERT INTO gantry.principals (id, organization_id, external_subject, display_name) VALUES ($1, $2, $3, 'Copilot Demo') ON CONFLICT (id) DO NOTHING`, []any{DemoPrincipalID, OrganizationID, DemoSubject}},
		{`INSERT INTO gantry.principals (id, organization_id, external_subject, display_name) VALUES ($1, $2, $3, 'Copilot Other') ON CONFLICT (id) DO NOTHING`, []any{OtherPrincipalID, OrganizationID, OtherSubject}},
		{`INSERT INTO gantry.principals (id, organization_id, external_subject, display_name) VALUES ($1, $2, $3, 'Admin Demo') ON CONFLICT (id) DO NOTHING`, []any{AdminPrincipalID, OrganizationID, AdminSubject}},
		{`INSERT INTO gantry.workspace_memberships (workspace_id, principal_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, []any{WorkspaceID, DemoPrincipalID}},
		{`INSERT INTO gantry.workspace_memberships (workspace_id, principal_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, []any{WorkspaceID, OtherPrincipalID}},
		{`INSERT INTO gantry.workspace_memberships (workspace_id, principal_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, []any{WorkspaceID, AdminPrincipalID}},
		{`INSERT INTO gantry.role_bindings (id, principal_id, role) VALUES ('rb_admin_demo', $1, 'organization_admin') ON CONFLICT DO NOTHING`, []any{AdminPrincipalID}},
		{`INSERT INTO gantry.agents (id, organization_id, workspace_id, owner_principal_id, slug, display_name, description, category) VALUES ($1, $2, $3, $4, 'lifecycle-demo', 'Lifecycle Demo', 'Deterministic completion lifecycle agent.', 'Development') ON CONFLICT (id) DO NOTHING`, []any{DemoAgentID, OrganizationID, WorkspaceID, AdminPrincipalID}},
		{`INSERT INTO gantry.agent_drafts (agent_id, revision, spec_json, validation_status, updated_by_principal_id) VALUES ($1, 1, $2::jsonb, 'valid', $3) ON CONFLICT (agent_id) DO NOTHING`, []any{DemoAgentID, completeSpec, AdminPrincipalID}},
		{`INSERT INTO gantry.agent_versions (id, agent_id, version, source_draft_revision, spec_json, spec_digest, created_by_principal_id) VALUES ($1, $2, 1, 1, $3::jsonb, $4, $5) ON CONFLICT (id) DO NOTHING`, []any{DemoVersionID, DemoAgentID, completeSpec, "sha256:" + hex.EncodeToString(completeDigest[:]), AdminPrincipalID}},
		{`INSERT INTO gantry.agent_publications (id, agent_id, agent_version_id, workspace_id, status, published_by_principal_id) VALUES ('pub_lifecycle_demo', $1, $2, $3, 'published', $4) ON CONFLICT (id) DO NOTHING`, []any{DemoAgentID, DemoVersionID, WorkspaceID, AdminPrincipalID}},
		{`INSERT INTO gantry.agents (id, organization_id, workspace_id, owner_principal_id, slug, display_name, description, category) VALUES ($1, $2, $3, $4, 'lifecycle-await-cancel', 'Lifecycle Await Cancel', 'Deterministic cancellation lifecycle agent.', 'Development') ON CONFLICT (id) DO NOTHING`, []any{AwaitCancelAgentID, OrganizationID, WorkspaceID, AdminPrincipalID}},
		{`INSERT INTO gantry.agent_drafts (agent_id, revision, spec_json, validation_status, updated_by_principal_id) VALUES ($1, 1, $2::jsonb, 'valid', $3) ON CONFLICT (agent_id) DO NOTHING`, []any{AwaitCancelAgentID, awaitCancelSpec, AdminPrincipalID}},
		{`INSERT INTO gantry.agent_versions (id, agent_id, version, source_draft_revision, spec_json, spec_digest, created_by_principal_id) VALUES ($1, $2, 1, 1, $3::jsonb, $4, $5) ON CONFLICT (id) DO NOTHING`, []any{AwaitCancelVersionID, AwaitCancelAgentID, awaitCancelSpec, "sha256:" + hex.EncodeToString(awaitCancelDigest[:]), AdminPrincipalID}},
		{`INSERT INTO gantry.agent_publications (id, agent_id, agent_version_id, workspace_id, status, published_by_principal_id) VALUES ('pub_lifecycle_await_cancel', $1, $2, $3, 'published', $4) ON CONFLICT (id) DO NOTHING`, []any{AwaitCancelAgentID, AwaitCancelVersionID, WorkspaceID, AdminPrincipalID}},
	}
	for _, statement := range statements {
		if _, err := tx.Exec(ctx, statement.query, statement.args...); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
