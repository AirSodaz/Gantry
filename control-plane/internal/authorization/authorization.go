// Package authorization owns organization and workspace access decisions.
package authorization

import (
	"context"
	"errors"

	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrForbidden = errors.New("forbidden")
	ErrNotFound  = errors.New("not found")
)

const (
	OrganizationAdmin    = "organization_admin"
	WorkspaceAgentEditor = "workspace_agent_editor"
)

type Workspace struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
}

type Service struct{ pool *pgxpool.Pool }

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

func (s *Service) RequireAdmin(ctx context.Context, actor identity.Principal) error {
	var allowed bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM gantry.role_bindings
			WHERE principal_id=$1 AND role IN ($2, $3)
		)`, actor.ID, OrganizationAdmin, WorkspaceAgentEditor).Scan(&allowed)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrForbidden
	}
	return nil
}

func (s *Service) RequireOrganizationAdmin(ctx context.Context, actor identity.Principal) error {
	var allowed bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM gantry.role_bindings
			WHERE principal_id=$1 AND role=$2 AND workspace_id IS NULL
		)`, actor.ID, OrganizationAdmin).Scan(&allowed)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrForbidden
	}
	return nil
}

func (s *Service) ListWorkspaces(ctx context.Context, actor identity.Principal) ([]Workspace, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT w.id, w.slug, w.display_name
		FROM gantry.workspaces w
		WHERE w.organization_id=$1 AND (
			EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$2 AND rb.role=$3 AND rb.workspace_id IS NULL)
			OR EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$2 AND rb.role=$4 AND rb.workspace_id=w.id)
		)
		ORDER BY w.display_name, w.id`, actor.OrganizationID, actor.ID, OrganizationAdmin, WorkspaceAgentEditor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Workspace, 0)
	for rows.Next() {
		var item Workspace
		if err := rows.Scan(&item.ID, &item.Slug, &item.DisplayName); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// RequireWorkspace deliberately treats an out-of-domain workspace as absent.
func (s *Service) RequireWorkspace(ctx context.Context, actor identity.Principal, workspaceID string) error {
	var organizationID string
	err := s.pool.QueryRow(ctx, `SELECT organization_id FROM gantry.workspaces WHERE id=$1`, workspaceID).Scan(&organizationID)
	if errors.Is(err, pgx.ErrNoRows) || organizationID != actor.OrganizationID {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	var allowed bool
	err = s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM gantry.role_bindings
			WHERE principal_id=$1 AND (
				(role=$2 AND workspace_id IS NULL) OR
				(role=$3 AND workspace_id=$4)
			)
		)`, actor.ID, OrganizationAdmin, WorkspaceAgentEditor, workspaceID).Scan(&allowed)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrNotFound
	}
	return nil
}
