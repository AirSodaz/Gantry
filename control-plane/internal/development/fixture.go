// Package development owns local-only records used by Compose and smoke tests.
package development

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	OrganizationID          = "org_development"
	WorkspaceID             = "wsp_development"
	CompleteAgentID         = "agt_lifecycle_complete"
	CompleteDraftID         = "drf_lifecycle_complete_main"
	CompleteRevisionID      = "arv_lifecycle_complete_1"
	AwaitCancelAgentID      = "agt_lifecycle_await_cancel"
	AwaitCancelDraftID      = "drf_lifecycle_await_cancel_main"
	AwaitCancelRevisionID   = "arv_lifecycle_await_cancel_1"
	AwaitApprovalAgentID    = "agt_lifecycle_await_approval"
	AwaitApprovalDraftID    = "drf_lifecycle_await_approval_main"
	AwaitApprovalRevisionID = "arv_lifecycle_await_approval_1"
	DevelopmentPrincipalID  = "prn_copilot_development"
	OtherPrincipalID        = "prn_copilot_other"
	AdminPrincipalID        = "prn_admin_demo"
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
	completeRevisionIdentity := sha256.Sum256([]byte("seed revision\n" + completeSpec))
	awaitCancelRevisionIdentity := sha256.Sum256([]byte("seed revision\n" + awaitCancelSpec))
	awaitApprovalRevisionIdentity := sha256.Sum256([]byte("seed revision\n" + awaitApprovalSpec))
	completeRevisionHash := "sha256:" + hex.EncodeToString(completeRevisionIdentity[:])
	awaitCancelRevisionHash := "sha256:" + hex.EncodeToString(awaitCancelRevisionIdentity[:])
	awaitApprovalRevisionHash := "sha256:" + hex.EncodeToString(awaitApprovalRevisionIdentity[:])
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
		{`INSERT INTO gantry.agent_draft_workspaces (id, agent_id, name, status, latest_revision_hash, spec_json, working_copy_etag, validation_status, validation_findings, created_by_principal_id, updated_by_principal_id) VALUES ($1, $2, 'Main', 'active', $3, $4::jsonb, 1, 'valid', '[]'::jsonb, $5, $5) ON CONFLICT (id) DO NOTHING`, []any{CompleteDraftID, CompleteAgentID, completeRevisionHash, completeSpec, AdminPrincipalID}},
		{`INSERT INTO gantry.agent_revisions (id, agent_id, revision_hash, source_draft_id, message, spec_json, spec_digest, created_by_principal_id) VALUES ($1, $2, $3, $4, 'Seeded development revision', $5::jsonb, $6, $7) ON CONFLICT (id) DO NOTHING`, []any{CompleteRevisionID, CompleteAgentID, completeRevisionHash, CompleteDraftID, completeSpec, "sha256:" + hex.EncodeToString(completeDigest[:]), AdminPrincipalID}},
		{`INSERT INTO gantry.agent_deployments (id, agent_id, workspace_id, name, environment_kind, revision_id, revision_hash, spec_digest, status, owner_principal_id, changed_by_principal_id) VALUES ('dpl_lifecycle_complete_production', $1, $2, 'Production', 'production', $3, $4, $5, 'active', $6, $6) ON CONFLICT (id) DO NOTHING`, []any{CompleteAgentID, WorkspaceID, CompleteRevisionID, completeRevisionHash, "sha256:" + hex.EncodeToString(completeDigest[:]), AdminPrincipalID}},
		{`INSERT INTO gantry.agents (id, organization_id, workspace_id, owner_principal_id, slug, display_name, description, category) VALUES ($1, $2, $3, $4, 'lifecycle-await-cancel', 'Lifecycle Await Cancel', 'Deterministic cancellation lifecycle agent.', 'Development') ON CONFLICT (id) DO NOTHING`, []any{AwaitCancelAgentID, OrganizationID, WorkspaceID, AdminPrincipalID}},
		{`INSERT INTO gantry.agent_draft_workspaces (id, agent_id, name, status, latest_revision_hash, spec_json, working_copy_etag, validation_status, validation_findings, created_by_principal_id, updated_by_principal_id) VALUES ($1, $2, 'Main', 'active', $3, $4::jsonb, 1, 'valid', '[]'::jsonb, $5, $5) ON CONFLICT (id) DO NOTHING`, []any{AwaitCancelDraftID, AwaitCancelAgentID, awaitCancelRevisionHash, awaitCancelSpec, AdminPrincipalID}},
		{`INSERT INTO gantry.agent_revisions (id, agent_id, revision_hash, source_draft_id, message, spec_json, spec_digest, created_by_principal_id) VALUES ($1, $2, $3, $4, 'Seeded development revision', $5::jsonb, $6, $7) ON CONFLICT (id) DO NOTHING`, []any{AwaitCancelRevisionID, AwaitCancelAgentID, awaitCancelRevisionHash, AwaitCancelDraftID, awaitCancelSpec, "sha256:" + hex.EncodeToString(awaitCancelDigest[:]), AdminPrincipalID}},
		{`INSERT INTO gantry.agent_deployments (id, agent_id, workspace_id, name, environment_kind, revision_id, revision_hash, spec_digest, status, owner_principal_id, changed_by_principal_id) VALUES ('dpl_lifecycle_await_cancel_production', $1, $2, 'Production', 'production', $3, $4, $5, 'active', $6, $6) ON CONFLICT (id) DO NOTHING`, []any{AwaitCancelAgentID, WorkspaceID, AwaitCancelRevisionID, awaitCancelRevisionHash, "sha256:" + hex.EncodeToString(awaitCancelDigest[:]), AdminPrincipalID}},
		{`INSERT INTO gantry.agents (id, organization_id, workspace_id, owner_principal_id, slug, display_name, description, category) VALUES ($1, $2, $3, $4, 'lifecycle-await-approval', 'Lifecycle Await Approval', 'Deterministic action approval lifecycle agent.', 'Development') ON CONFLICT (id) DO NOTHING`, []any{AwaitApprovalAgentID, OrganizationID, WorkspaceID, AdminPrincipalID}},
		{`INSERT INTO gantry.agent_draft_workspaces (id, agent_id, name, status, latest_revision_hash, spec_json, working_copy_etag, validation_status, validation_findings, created_by_principal_id, updated_by_principal_id) VALUES ($1, $2, 'Main', 'active', $3, $4::jsonb, 1, 'valid', '[]'::jsonb, $5, $5) ON CONFLICT (id) DO NOTHING`, []any{AwaitApprovalDraftID, AwaitApprovalAgentID, awaitApprovalRevisionHash, awaitApprovalSpec, AdminPrincipalID}},
		{`INSERT INTO gantry.agent_revisions (id, agent_id, revision_hash, source_draft_id, message, spec_json, spec_digest, created_by_principal_id) VALUES ($1, $2, $3, $4, 'Seeded development revision', $5::jsonb, $6, $7) ON CONFLICT (id) DO NOTHING`, []any{AwaitApprovalRevisionID, AwaitApprovalAgentID, awaitApprovalRevisionHash, AwaitApprovalDraftID, awaitApprovalSpec, "sha256:" + hex.EncodeToString(awaitApprovalDigest[:]), AdminPrincipalID}},
		{`INSERT INTO gantry.agent_deployments (id, agent_id, workspace_id, name, environment_kind, revision_id, revision_hash, spec_digest, status, owner_principal_id, changed_by_principal_id) VALUES ('dpl_lifecycle_await_approval_production', $1, $2, 'Production', 'production', $3, $4, $5, 'active', $6, $6) ON CONFLICT (id) DO NOTHING`, []any{AwaitApprovalAgentID, WorkspaceID, AwaitApprovalRevisionID, awaitApprovalRevisionHash, "sha256:" + hex.EncodeToString(awaitApprovalDigest[:]), AdminPrincipalID}},
		{`INSERT INTO gantry.agent_access_grants (id, agent_id, subject_type, subject_id, state, created_by_principal_id, updated_by_principal_id) VALUES ('aag_complete_dev_execute', $1, 'principal', $2, 'active', $3, $3) ON CONFLICT (id) DO NOTHING`, []any{CompleteAgentID, DevelopmentPrincipalID, AdminPrincipalID}},
		{`INSERT INTO gantry.agent_access_grants (id, agent_id, subject_type, subject_id, state, created_by_principal_id, updated_by_principal_id) VALUES ('aag_cancel_dev_execute', $1, 'principal', $2, 'active', $3, $3) ON CONFLICT (id) DO NOTHING`, []any{AwaitCancelAgentID, DevelopmentPrincipalID, AdminPrincipalID}},
		{`INSERT INTO gantry.agent_access_grants (id, agent_id, subject_type, subject_id, state, created_by_principal_id, updated_by_principal_id) VALUES ('aag_approval_dev_execute', $1, 'principal', $2, 'active', $3, $3) ON CONFLICT (id) DO NOTHING`, []any{AwaitApprovalAgentID, DevelopmentPrincipalID, AdminPrincipalID}},
		{`INSERT INTO gantry.agent_access_grant_capabilities (grant_id, capability) VALUES ('aag_complete_dev_execute', 'execute') ON CONFLICT DO NOTHING`, nil},
		{`INSERT INTO gantry.agent_access_grant_capabilities (grant_id, capability) VALUES ('aag_complete_dev_execute', 'metadata.read') ON CONFLICT DO NOTHING`, nil},
		{`INSERT INTO gantry.agent_access_grant_capabilities (grant_id, capability) VALUES ('aag_cancel_dev_execute', 'execute') ON CONFLICT DO NOTHING`, nil},
		{`INSERT INTO gantry.agent_access_grant_capabilities (grant_id, capability) VALUES ('aag_cancel_dev_execute', 'metadata.read') ON CONFLICT DO NOTHING`, nil},
		{`INSERT INTO gantry.agent_access_grant_capabilities (grant_id, capability) VALUES ('aag_approval_dev_execute', 'execute') ON CONFLICT DO NOTHING`, nil},
		{`INSERT INTO gantry.agent_access_grant_capabilities (grant_id, capability) VALUES ('aag_approval_dev_execute', 'metadata.read') ON CONFLICT DO NOTHING`, nil},
	}
	for _, statement := range statements {
		if _, err := tx.Exec(ctx, statement.query, statement.args...); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
