// Package development owns local-only records used by Compose and smoke tests.
package development

import (
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	OrganizationID   = "org_development"
	WorkspaceID      = "wsp_development"
	DemoAgentID      = "agt_lifecycle_demo"
	DemoVersionID    = "agtv_lifecycle_demo_1"
	DemoPrincipalID  = "prn_copilot_demo"
	OtherPrincipalID = "prn_copilot_other"
	DemoSubject      = "11111111-1111-1111-1111-111111111111"
	OtherSubject     = "22222222-2222-2222-2222-222222222222"
)

func Seed(ctx context.Context, pool *pgxpool.Pool) error {
	spec := `{"kind":"gantry.phase0.demo/v1","modes":["complete","await_cancel"]}`
	digest := sha256.Sum256([]byte(spec))
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
		{`INSERT INTO gantry.workspace_memberships (workspace_id, principal_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, []any{WorkspaceID, DemoPrincipalID}},
		{`INSERT INTO gantry.workspace_memberships (workspace_id, principal_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, []any{WorkspaceID, OtherPrincipalID}},
		{`INSERT INTO gantry.agents (id, organization_id, workspace_id, slug, display_name, description, category) VALUES ($1, $2, $3, 'lifecycle-demo', 'Lifecycle Demo', 'Deterministic local lifecycle agent.', 'Development') ON CONFLICT (id) DO NOTHING`, []any{DemoAgentID, OrganizationID, WorkspaceID}},
		{`INSERT INTO gantry.agent_versions (id, agent_id, version, spec_json, spec_digest) VALUES ($1, $2, 1, $3::jsonb, $4) ON CONFLICT (id) DO NOTHING`, []any{DemoVersionID, DemoAgentID, spec, "sha256:" + hex.EncodeToString(digest[:])}},
		{`INSERT INTO gantry.agent_publications (id, agent_id, agent_version_id, workspace_id, status) VALUES ('pub_lifecycle_demo', $1, $2, $3, 'published') ON CONFLICT (id) DO NOTHING`, []any{DemoAgentID, DemoVersionID, WorkspaceID}},
	}
	for _, statement := range statements {
		if _, err := tx.Exec(ctx, statement.query, statement.args...); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
