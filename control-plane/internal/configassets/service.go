// Package configassets owns the administrative catalog for reusable Agent
// configuration assets. It deliberately stops at registration and validation;
// runtime tool and package execution belongs to the gateway/runner boundary.
package configassets

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/AirSodaz/gantry/internal/authorization"
	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidInput = errors.New("invalid configuration asset input")
	ErrNotFound     = errors.New("configuration asset not found")
)

type Skill struct {
	ID              string `json:"id"`
	WorkspaceID     string `json:"workspace_id"`
	Slug            string `json:"slug"`
	DisplayName     string `json:"display_name"`
	Description     string `json:"description"`
	SourceType      string `json:"source_type"`
	SourceRef       string `json:"source_ref"`
	DeclaredVersion string `json:"declared_version"`
	ContentDigest   string `json:"content_digest"`
	Status          string `json:"status"`
}

type CreateSkillRequest struct {
	WorkspaceID     string `json:"workspace_id"`
	Slug            string `json:"slug"`
	DisplayName     string `json:"display_name"`
	Description     string `json:"description"`
	SourceType      string `json:"source_type"`
	SourceRef       string `json:"source_ref"`
	DeclaredVersion string `json:"declared_version"`
	ContentDigest   string `json:"content_digest"`
}

type Plugin struct {
	ID            string `json:"id"`
	Slug          string `json:"slug"`
	DisplayName   string `json:"display_name"`
	Description   string `json:"description"`
	Version       string `json:"version"`
	ContentDigest string `json:"content_digest"`
	Status        string `json:"status"`
}

type CreatePluginRequest struct {
	Slug          string `json:"slug"`
	DisplayName   string `json:"display_name"`
	Description   string `json:"description"`
	Version       string `json:"version"`
	ContentDigest string `json:"content_digest"`
}

type EnablePluginRequest struct {
	WorkspaceID string `json:"workspace_id"`
}

type Tool struct {
	ID                 string `json:"id"`
	ServerID           string `json:"server_id"`
	ServerName         string `json:"server_name"`
	ServerType         string `json:"server_type"`
	FullyQualifiedName string `json:"fully_qualified_name"`
	Version            string `json:"version"`
	Effect             string `json:"effect"`
	Idempotency        string `json:"idempotency"`
	ContentDigest      string `json:"content_digest"`
	Status             string `json:"status"`
}

type CreateToolRequest struct {
	ServerName         string `json:"server_name"`
	ServerType         string `json:"server_type"`
	EndpointRef        string `json:"endpoint_ref"`
	FullyQualifiedName string `json:"fully_qualified_name"`
	Version            string `json:"version"`
	Effect             string `json:"effect"`
	Idempotency        string `json:"idempotency"`
	ContentDigest      string `json:"content_digest"`
}

type Service struct {
	pool  *pgxpool.Pool
	authz *authorization.Service
}

func NewService(pool *pgxpool.Pool, authz *authorization.Service) *Service {
	return &Service{pool: pool, authz: authz}
}

func (s *Service) ListSkills(ctx context.Context, actor identity.Principal, workspaceID string) ([]Skill, error) {
	if workspaceID != "" {
		if err := s.authz.RequireWorkspace(ctx, actor, workspaceID); err != nil {
			return nil, err
		}
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, workspace_id, slug, display_name, description, source_type, source_ref,
			declared_version, content_digest, status
		FROM gantry.skills
		WHERE organization_id=$1 AND ($2='' OR workspace_id=$2) AND status <> 'retired' AND (
			EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$3 AND rb.role='organization_admin' AND rb.workspace_id IS NULL)
			OR EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$3 AND rb.role='workspace_agent_editor' AND rb.workspace_id=gantry.skills.workspace_id)
		)
		ORDER BY display_name, slug, id`, actor.OrganizationID, workspaceID, actor.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Skill, 0)
	for rows.Next() {
		var item Skill
		if err := rows.Scan(&item.ID, &item.WorkspaceID, &item.Slug, &item.DisplayName, &item.Description, &item.SourceType, &item.SourceRef, &item.DeclaredVersion, &item.ContentDigest, &item.Status); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) CreateSkill(ctx context.Context, actor identity.Principal, request CreateSkillRequest) (Skill, error) {
	request.Slug = strings.TrimSpace(request.Slug)
	request.DisplayName = strings.TrimSpace(request.DisplayName)
	request.SourceType = strings.TrimSpace(request.SourceType)
	request.SourceRef = strings.TrimSpace(request.SourceRef)
	request.ContentDigest = strings.TrimSpace(request.ContentDigest)
	if request.WorkspaceID == "" || request.Slug == "" || request.DisplayName == "" || request.SourceRef == "" || request.ContentDigest == "" || !validSlug(request.Slug) || !validSkillSource(request.SourceType) {
		return Skill{}, ErrInvalidInput
	}
	if err := s.authz.RequireWorkspace(ctx, actor, request.WorkspaceID); err != nil {
		return Skill{}, err
	}
	var item Skill
	item.ID = newID("skill")
	item.WorkspaceID, item.Slug, item.DisplayName = request.WorkspaceID, request.Slug, request.DisplayName
	item.Description, item.SourceType, item.SourceRef = strings.TrimSpace(request.Description), request.SourceType, request.SourceRef
	item.DeclaredVersion, item.ContentDigest, item.Status = strings.TrimSpace(request.DeclaredVersion), request.ContentDigest, "available"
	err := s.pool.QueryRow(ctx, `INSERT INTO gantry.skills (id, organization_id, workspace_id, slug, display_name, description, source_type, source_ref, declared_version, content_digest, status, created_by_principal_id) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'available',$11) RETURNING id`, item.ID, actor.OrganizationID, item.WorkspaceID, item.Slug, item.DisplayName, item.Description, item.SourceType, item.SourceRef, item.DeclaredVersion, item.ContentDigest, actor.ID).Scan(&item.ID)
	if err != nil {
		return Skill{}, err
	}
	return item, nil
}

func (s *Service) ListPlugins(ctx context.Context, actor identity.Principal) ([]Plugin, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, slug, display_name, description, version, content_digest, status FROM gantry.plugins WHERE organization_id=$1 AND status <> 'retired' ORDER BY display_name, version DESC, id`, actor.OrganizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Plugin, 0)
	for rows.Next() {
		var item Plugin
		if err := rows.Scan(&item.ID, &item.Slug, &item.DisplayName, &item.Description, &item.Version, &item.ContentDigest, &item.Status); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) CreatePlugin(ctx context.Context, actor identity.Principal, request CreatePluginRequest) (Plugin, error) {
	if err := s.authz.RequireOrganizationAdmin(ctx, actor); err != nil {
		return Plugin{}, err
	}
	request.Slug = strings.TrimSpace(request.Slug)
	request.DisplayName = strings.TrimSpace(request.DisplayName)
	request.Version = strings.TrimSpace(request.Version)
	request.ContentDigest = strings.TrimSpace(request.ContentDigest)
	if request.Slug == "" || request.DisplayName == "" || request.Version == "" || request.ContentDigest == "" || !validSlug(request.Slug) {
		return Plugin{}, ErrInvalidInput
	}
	item := Plugin{ID: newID("plugin"), Slug: request.Slug, DisplayName: request.DisplayName, Description: strings.TrimSpace(request.Description), Version: request.Version, ContentDigest: request.ContentDigest, Status: "active"}
	err := s.pool.QueryRow(ctx, `INSERT INTO gantry.plugins (id, organization_id, slug, display_name, description, version, content_digest, status, created_by_principal_id) VALUES ($1,$2,$3,$4,$5,$6,$7,'active',$8) RETURNING id`, item.ID, actor.OrganizationID, item.Slug, item.DisplayName, item.Description, item.Version, item.ContentDigest, actor.ID).Scan(&item.ID)
	if err != nil {
		return Plugin{}, err
	}
	return item, nil
}

func (s *Service) EnablePlugin(ctx context.Context, actor identity.Principal, pluginID, workspaceID string) error {
	if err := s.authz.RequireOrganizationAdmin(ctx, actor); err != nil {
		return err
	}
	if strings.TrimSpace(pluginID) == "" || strings.TrimSpace(workspaceID) == "" {
		return ErrInvalidInput
	}
	var found bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM gantry.plugins WHERE id=$1 AND organization_id=$2 AND status='active') AND EXISTS (SELECT 1 FROM gantry.workspaces WHERE id=$3 AND organization_id=$2)`, pluginID, actor.OrganizationID, workspaceID).Scan(&found)
	if err != nil {
		return err
	}
	if !found {
		return ErrNotFound
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO gantry.workspace_plugin_enablements (workspace_id, plugin_id, enabled_by_principal_id) VALUES ($1,$2,$3) ON CONFLICT (workspace_id,plugin_id) DO NOTHING`, workspaceID, pluginID, actor.ID)
	return err
}

func (s *Service) ListTools(ctx context.Context, actor identity.Principal) ([]Tool, error) {
	rows, err := s.pool.Query(ctx, `SELECT d.id, s.id, s.name, s.server_type, d.fully_qualified_name, d.version, d.effect, d.idempotency, d.content_digest, d.status FROM gantry.tool_descriptors d JOIN gantry.tool_servers s ON s.id=d.server_id WHERE s.organization_id=$1 AND s.status <> 'retired' AND d.status IN ('active','proposed') ORDER BY s.name, d.fully_qualified_name, d.version DESC`, actor.OrganizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Tool, 0)
	for rows.Next() {
		var item Tool
		if err := rows.Scan(&item.ID, &item.ServerID, &item.ServerName, &item.ServerType, &item.FullyQualifiedName, &item.Version, &item.Effect, &item.Idempotency, &item.ContentDigest, &item.Status); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) CreateTool(ctx context.Context, actor identity.Principal, request CreateToolRequest) (Tool, error) {
	if err := s.authz.RequireOrganizationAdmin(ctx, actor); err != nil {
		return Tool{}, err
	}
	request.ServerName = strings.TrimSpace(request.ServerName)
	request.ServerType = strings.TrimSpace(request.ServerType)
	request.FullyQualifiedName = strings.TrimSpace(request.FullyQualifiedName)
	request.Version = strings.TrimSpace(request.Version)
	request.Effect = strings.TrimSpace(request.Effect)
	request.Idempotency = strings.TrimSpace(request.Idempotency)
	request.ContentDigest = strings.TrimSpace(request.ContentDigest)
	if request.ServerName == "" || request.FullyQualifiedName == "" || request.Version == "" || request.ContentDigest == "" || !validServerType(request.ServerType) || !validEffect(request.Effect) || !validIdempotency(request.Idempotency) {
		return Tool{}, ErrInvalidInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Tool{}, err
	}
	defer tx.Rollback(ctx)
	var serverID, serverType string
	err = tx.QueryRow(ctx, `INSERT INTO gantry.tool_servers (id, organization_id, name, server_type, endpoint_ref, status, created_by_principal_id) VALUES ($1,$2,$3,$4,$5,'active',$6) ON CONFLICT (organization_id,name) DO UPDATE SET endpoint_ref=EXCLUDED.endpoint_ref, status='active' RETURNING id, server_type`, newID("toolserver"), actor.OrganizationID, request.ServerName, request.ServerType, strings.TrimSpace(request.EndpointRef), actor.ID).Scan(&serverID, &serverType)
	if err != nil {
		return Tool{}, err
	}
	item := Tool{ID: newID("tool"), ServerID: serverID, ServerName: request.ServerName, ServerType: serverType, FullyQualifiedName: request.FullyQualifiedName, Version: request.Version, Effect: request.Effect, Idempotency: request.Idempotency, ContentDigest: request.ContentDigest, Status: "active"}
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.tool_descriptors (id, server_id, fully_qualified_name, version, effect, idempotency, content_digest, status) VALUES ($1,$2,$3,$4,$5,$6,$7,'active')`, item.ID, serverID, item.FullyQualifiedName, item.Version, item.Effect, item.Idempotency, item.ContentDigest); err != nil {
		return Tool{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Tool{}, err
	}
	return item, nil
}

func validSlug(value string) bool {
	if len(value) < 2 || len(value) > 80 {
		return false
	}
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return false
		}
	}
	return true
}

func validSkillSource(value string) bool {
	return value == "marketplace" || value == "locator" || value == "upload" || value == "local"
}
func validServerType(value string) bool {
	return value == "builtin" || value == "mcp" || value == "cli"
}
func validEffect(value string) bool {
	return value == "read" || value == "write" || value == "external_side_effect" || value == "administrative"
}
func validIdempotency(value string) bool {
	return value == "read_only" || value == "idempotent" || value == "compensatable" || value == "non_repeatable"
}

func newID(prefix string) string {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(value)
}
