// Package development owns local-only records used by Compose and smoke tests.
package development

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	OrganizationID         = "org_development"
	WorkspaceID            = "wsp_development"
	CompleteAgentID        = "agt_lifecycle_complete"
	CompleteVersionID      = "agtv_lifecycle_complete_1"
	AwaitCancelAgentID     = "agt_lifecycle_await_cancel"
	AwaitCancelVersionID   = "agtv_lifecycle_await_cancel_1"
	AwaitApprovalAgentID   = "agt_lifecycle_await_approval"
	AwaitApprovalVersionID = "agtv_lifecycle_await_approval_1"
	DevelopmentPrincipalID = "prn_copilot_development"
	OtherPrincipalID       = "prn_copilot_other"
	AdminPrincipalID       = "prn_admin_demo"
	// Dex encodes the local user ID and connector ID into the OIDC subject.
	DevelopmentSubject = "CiQxMTExMTExMS0xMTExLTExMTEtMTExMS0xMTExMTExMTExMTESBWxvY2Fs"
	OtherSubject       = "CiQyMjIyMjIyMi0yMjIyLTIyMjItMjIyMi0yMjIyMjIyMjIyMjISBWxvY2Fs"
	AdminSubject       = "CiQzMzMzMzMzMy0zMzMzLTMzMzMtMzMzMy0zMzMzMzMzMzMzMzMSBWxvY2Fs"
)

func Seed(ctx context.Context, pool *pgxpool.Pool) error {
	completeSpec := `{"kind":"gantry.agent/v1","model":{"provider":"scripted","model":"deterministic"},"user_input":"hello from development","workspace_root":"/workspace","limits":{"max_turns":12,"max_output_bytes":131072},"checkpoint":{"enabled":false},"command_policy":{"allow_shell":false},"artifacts":[{"path":"result.txt","filename":"gantry-result.txt","media_type":"text/plain"}]}`
	awaitCancelSpec := `{"kind":"gantry.agent/v1","model":{"provider":"scripted","model":"deterministic"},"user_input":"wait for cancellation","workspace_root":".","limits":{"max_turns":12,"max_output_bytes":131072},"checkpoint":{"enabled":false},"command_policy":{"allow_shell":false}}`
	awaitApprovalSpec := `{"kind":"gantry.agent/v1","model":{"provider":"scripted","model":"deterministic"},"user_input":"shell printf approval","workspace_root":".","tools":["shell"],"limits":{"max_turns":12,"max_output_bytes":131072},"checkpoint":{"enabled":false},"command_policy":{"allow_shell":true}}`
	completeDigest := sha256.Sum256([]byte(completeSpec))
	awaitCancelDigest := sha256.Sum256([]byte(awaitCancelSpec))
	awaitApprovalDigest := sha256.Sum256([]byte(awaitApprovalSpec))
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
		{`INSERT INTO gantry.principals (id, organization_id, external_subject, display_name) VALUES ($1, $2, $3, 'Copilot Development') ON CONFLICT (id) DO NOTHING`, []any{DevelopmentPrincipalID, OrganizationID, DevelopmentSubject}},
		{`INSERT INTO gantry.principals (id, organization_id, external_subject, display_name) VALUES ($1, $2, $3, 'Copilot Other') ON CONFLICT (id) DO NOTHING`, []any{OtherPrincipalID, OrganizationID, OtherSubject}},
		{`INSERT INTO gantry.principals (id, organization_id, external_subject, display_name) VALUES ($1, $2, $3, 'Admin Demo') ON CONFLICT (id) DO NOTHING`, []any{AdminPrincipalID, OrganizationID, AdminSubject}},
		{`INSERT INTO gantry.workspace_memberships (workspace_id, principal_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, []any{WorkspaceID, DevelopmentPrincipalID}},
		{`INSERT INTO gantry.workspace_memberships (workspace_id, principal_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, []any{WorkspaceID, OtherPrincipalID}},
		{`INSERT INTO gantry.workspace_memberships (workspace_id, principal_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, []any{WorkspaceID, AdminPrincipalID}},
		{`INSERT INTO gantry.role_bindings (id, principal_id, role) VALUES ('rb_admin_demo', $1, 'organization_admin') ON CONFLICT DO NOTHING`, []any{AdminPrincipalID}},
		{`INSERT INTO gantry.agents (id, organization_id, workspace_id, owner_principal_id, slug, display_name, description, category) VALUES ($1, $2, $3, $4, 'lifecycle-complete', 'Lifecycle Complete', 'Deterministic completion lifecycle agent.', 'Development') ON CONFLICT (id) DO NOTHING`, []any{CompleteAgentID, OrganizationID, WorkspaceID, AdminPrincipalID}},
		{`INSERT INTO gantry.agent_drafts (agent_id, revision, spec_json, validation_status, updated_by_principal_id) VALUES ($1, 1, $2::jsonb, 'valid', $3) ON CONFLICT (agent_id) DO NOTHING`, []any{CompleteAgentID, completeSpec, AdminPrincipalID}},
		{`INSERT INTO gantry.agent_versions (id, agent_id, version, source_draft_revision, spec_json, spec_digest, created_by_principal_id) VALUES ($1, $2, 1, 1, $3::jsonb, $4, $5) ON CONFLICT (id) DO NOTHING`, []any{CompleteVersionID, CompleteAgentID, completeSpec, "sha256:" + hex.EncodeToString(completeDigest[:]), AdminPrincipalID}},
		{`INSERT INTO gantry.agent_publications (id, agent_id, agent_version_id, workspace_id, status, published_by_principal_id) VALUES ('pub_lifecycle_complete', $1, $2, $3, 'published', $4) ON CONFLICT (id) DO NOTHING`, []any{CompleteAgentID, CompleteVersionID, WorkspaceID, AdminPrincipalID}},
		{`INSERT INTO gantry.agents (id, organization_id, workspace_id, owner_principal_id, slug, display_name, description, category) VALUES ($1, $2, $3, $4, 'lifecycle-await-cancel', 'Lifecycle Await Cancel', 'Deterministic cancellation lifecycle agent.', 'Development') ON CONFLICT (id) DO NOTHING`, []any{AwaitCancelAgentID, OrganizationID, WorkspaceID, AdminPrincipalID}},
		{`INSERT INTO gantry.agent_drafts (agent_id, revision, spec_json, validation_status, updated_by_principal_id) VALUES ($1, 1, $2::jsonb, 'valid', $3) ON CONFLICT (agent_id) DO NOTHING`, []any{AwaitCancelAgentID, awaitCancelSpec, AdminPrincipalID}},
		{`INSERT INTO gantry.agent_versions (id, agent_id, version, source_draft_revision, spec_json, spec_digest, created_by_principal_id) VALUES ($1, $2, 1, 1, $3::jsonb, $4, $5) ON CONFLICT (id) DO NOTHING`, []any{AwaitCancelVersionID, AwaitCancelAgentID, awaitCancelSpec, "sha256:" + hex.EncodeToString(awaitCancelDigest[:]), AdminPrincipalID}},
		{`INSERT INTO gantry.agent_publications (id, agent_id, agent_version_id, workspace_id, status, published_by_principal_id) VALUES ('pub_lifecycle_await_cancel', $1, $2, $3, 'published', $4) ON CONFLICT (id) DO NOTHING`, []any{AwaitCancelAgentID, AwaitCancelVersionID, WorkspaceID, AdminPrincipalID}},
		{`INSERT INTO gantry.agents (id, organization_id, workspace_id, owner_principal_id, slug, display_name, description, category) VALUES ($1, $2, $3, $4, 'lifecycle-await-approval', 'Lifecycle Await Approval', 'Deterministic action approval lifecycle agent.', 'Development') ON CONFLICT (id) DO NOTHING`, []any{AwaitApprovalAgentID, OrganizationID, WorkspaceID, AdminPrincipalID}},
		{`INSERT INTO gantry.agent_drafts (agent_id, revision, spec_json, validation_status, updated_by_principal_id) VALUES ($1, 1, $2::jsonb, 'valid', $3) ON CONFLICT (agent_id) DO NOTHING`, []any{AwaitApprovalAgentID, awaitApprovalSpec, AdminPrincipalID}},
		{`INSERT INTO gantry.agent_versions (id, agent_id, version, source_draft_revision, spec_json, spec_digest, created_by_principal_id) VALUES ($1, $2, 1, 1, $3::jsonb, $4, $5) ON CONFLICT (id) DO NOTHING`, []any{AwaitApprovalVersionID, AwaitApprovalAgentID, awaitApprovalSpec, "sha256:" + hex.EncodeToString(awaitApprovalDigest[:]), AdminPrincipalID}},
		{`INSERT INTO gantry.agent_publications (id, agent_id, agent_version_id, workspace_id, status, published_by_principal_id) VALUES ('pub_lifecycle_await_approval', $1, $2, $3, 'published', $4) ON CONFLICT (id) DO NOTHING`, []any{AwaitApprovalAgentID, AwaitApprovalVersionID, WorkspaceID, AdminPrincipalID}},
	}
	for _, statement := range statements {
		if _, err := tx.Exec(ctx, statement.query, statement.args...); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
