// Package configassets owns the administrative catalog for reusable Agent
// configuration assets. It deliberately stops at registration and validation;
// runtime tool and package execution belongs to the gateway/runner boundary.
package configassets

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"

	"github.com/AirSodaz/gantry/internal/authorization"
	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidInput = errors.New("invalid configuration asset input")
	ErrNotFound     = errors.New("configuration asset not found")
)

type Skill struct {
	ID              string          `json:"id"`
	WorkspaceID     string          `json:"workspace_id"`
	Slug            string          `json:"slug"`
	DisplayName     string          `json:"display_name"`
	Description     string          `json:"description"`
	SourceType      string          `json:"source_type"`
	SourceRef       string          `json:"source_ref"`
	DeclaredVersion string          `json:"declared_version"`
	ContentDigest   string          `json:"content_digest"`
	Status          string          `json:"status"`
	Metadata        json.RawMessage `json:"metadata_json"`
}

type AssetUsage struct {
	AgentID       string `json:"agent_id"`
	AgentName     string `json:"agent_name"`
	WorkspaceID   string `json:"workspace_id"`
	ReferenceKind string `json:"reference_kind"`
	ReferenceID   string `json:"reference_id"`
	ReferenceHash string `json:"reference_hash,omitempty"`
}

type PluginWorkspace struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type PluginDetail struct {
	Plugin
	Workspaces []PluginWorkspace `json:"workspaces"`
}

type CreateSkillRequest struct {
	WorkspaceID     string          `json:"workspace_id"`
	Slug            string          `json:"slug"`
	DisplayName     string          `json:"display_name"`
	Description     string          `json:"description"`
	SourceType      string          `json:"source_type"`
	SourceRef       string          `json:"source_ref"`
	DeclaredVersion string          `json:"declared_version"`
	ContentDigest   string          `json:"content_digest"`
	Metadata        json.RawMessage `json:"metadata_json"`
}

type Plugin struct {
	ID            string          `json:"id"`
	Slug          string          `json:"slug"`
	DisplayName   string          `json:"display_name"`
	Description   string          `json:"description"`
	Version       string          `json:"version"`
	ContentDigest string          `json:"content_digest"`
	Status        string          `json:"status"`
	Manifest      json.RawMessage `json:"manifest_json"`
}

type CreatePluginRequest struct {
	Slug          string          `json:"slug"`
	DisplayName   string          `json:"display_name"`
	Description   string          `json:"description"`
	Version       string          `json:"version"`
	ContentDigest string          `json:"content_digest"`
	Manifest      json.RawMessage `json:"manifest_json"`
}

type EnablePluginRequest struct {
	WorkspaceID string `json:"workspace_id"`
}

// AssetStatusRequest carries the audit reason for an explicit catalog
// lifecycle command. The target state is part of the route, not user input.
type AssetStatusRequest struct {
	Reason string `json:"reason"`
}

type ListOptions struct {
	WorkspaceID string
	Search      string
	Status      string
}

type Tool struct {
	ID                 string          `json:"id"`
	ServerID           string          `json:"server_id"`
	ServerName         string          `json:"server_name"`
	ServerType         string          `json:"server_type"`
	EndpointRef        string          `json:"endpoint_ref,omitempty"`
	FullyQualifiedName string          `json:"fully_qualified_name"`
	Version            string          `json:"version"`
	Effect             string          `json:"effect"`
	Idempotency        string          `json:"idempotency"`
	ContentDigest      string          `json:"content_digest"`
	Schema             json.RawMessage `json:"schema_json"`
	Status             string          `json:"status"`
}

type CreateToolRequest struct {
	ServerName         string          `json:"server_name"`
	ServerType         string          `json:"server_type"`
	EndpointRef        string          `json:"endpoint_ref"`
	FullyQualifiedName string          `json:"fully_qualified_name"`
	Version            string          `json:"version"`
	Effect             string          `json:"effect"`
	Idempotency        string          `json:"idempotency"`
	ContentDigest      string          `json:"content_digest"`
	Schema             json.RawMessage `json:"schema_json"`
}

type Service struct {
	pool  *pgxpool.Pool
	authz *authorization.Service
}

func NewService(pool *pgxpool.Pool, authz *authorization.Service) *Service {
	return &Service{pool: pool, authz: authz}
}

func (s *Service) ListSkills(ctx context.Context, actor identity.Principal, options ListOptions) ([]Skill, error) {
	options.WorkspaceID = strings.TrimSpace(options.WorkspaceID)
	options.Search = strings.TrimSpace(options.Search)
	options.Status = normalizeSkillStatus(options.Status)
	if options.WorkspaceID != "" {
		if err := s.authz.RequireWorkspace(ctx, actor, options.WorkspaceID); err != nil {
			return nil, err
		}
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, workspace_id, slug, display_name, description, source_type, source_ref,
			declared_version, content_digest, status, metadata_json
		FROM gantry.skills
		WHERE organization_id=$1 AND ($2='' OR workspace_id=$2) AND ($3='' OR status=$3) AND
			($4='' OR display_name ILIKE '%' || $4 || '%' OR slug ILIKE '%' || $4 || '%') AND (
			EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$5 AND rb.role='organization_admin' AND rb.workspace_id IS NULL)
			OR EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$5 AND rb.role='workspace_agent_editor' AND rb.workspace_id=gantry.skills.workspace_id)
		)
		ORDER BY display_name, slug, id`, actor.OrganizationID, options.WorkspaceID, options.Status, options.Search, actor.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Skill, 0)
	for rows.Next() {
		var item Skill
		if err := rows.Scan(&item.ID, &item.WorkspaceID, &item.Slug, &item.DisplayName, &item.Description, &item.SourceType, &item.SourceRef, &item.DeclaredVersion, &item.ContentDigest, &item.Status, &item.Metadata); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) GetSkill(ctx context.Context, actor identity.Principal, skillID string) (Skill, error) {
	var item Skill
	err := s.pool.QueryRow(ctx, `
		SELECT id, workspace_id, slug, display_name, description, source_type, source_ref,
			declared_version, content_digest, status, metadata_json
		FROM gantry.skills
		WHERE id=$1 AND organization_id=$2`, skillID, actor.OrganizationID).Scan(
		&item.ID, &item.WorkspaceID, &item.Slug, &item.DisplayName, &item.Description,
		&item.SourceType, &item.SourceRef, &item.DeclaredVersion, &item.ContentDigest, &item.Status, &item.Metadata)
	if errors.Is(err, pgx.ErrNoRows) {
		return Skill{}, ErrNotFound
	}
	if err != nil {
		return Skill{}, err
	}
	if err := s.authz.RequireWorkspace(ctx, actor, item.WorkspaceID); err != nil {
		return Skill{}, err
	}
	return item, nil
}

func (s *Service) ListSkillUsage(ctx context.Context, actor identity.Principal, skillID string) ([]AssetUsage, error) {
	skill, err := s.GetSkill(ctx, actor, skillID)
	if err != nil {
		return nil, err
	}
	return s.listUsage(ctx, actor, skillID, skill.WorkspaceID, "skills", "artifact_id")
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
	if len(request.Metadata) == 0 {
		request.Metadata = json.RawMessage(`{}`)
	}
	var metadata map[string]any
	if err := json.Unmarshal(request.Metadata, &metadata); err != nil || metadata == nil {
		return Skill{}, ErrInvalidInput
	}
	var item Skill
	item.ID = newID("skill")
	item.WorkspaceID, item.Slug, item.DisplayName = request.WorkspaceID, request.Slug, request.DisplayName
	item.Description, item.SourceType, item.SourceRef = strings.TrimSpace(request.Description), request.SourceType, request.SourceRef
	item.DeclaredVersion, item.ContentDigest, item.Status = strings.TrimSpace(request.DeclaredVersion), request.ContentDigest, "available"
	item.Metadata = request.Metadata
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Skill{}, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `INSERT INTO gantry.skills (id, organization_id, workspace_id, slug, display_name, description, source_type, source_ref, declared_version, content_digest, metadata_json, status, created_by_principal_id) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb,'available',$12) RETURNING id`, item.ID, actor.OrganizationID, item.WorkspaceID, item.Slug, item.DisplayName, item.Description, item.SourceType, item.SourceRef, item.DeclaredVersion, item.ContentDigest, string(item.Metadata), actor.ID).Scan(&item.ID)
	if err != nil {
		return Skill{}, err
	}
	if err := appendCreatedAudit(ctx, tx, actor.OrganizationID, actor.ID, "skill", item.ID, map[string]string{"source_type": item.SourceType, "source_ref": item.SourceRef, "content_digest": item.ContentDigest}); err != nil {
		return Skill{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Skill{}, err
	}
	return item, nil
}

func (s *Service) ListPlugins(ctx context.Context, actor identity.Principal, options ListOptions) ([]Plugin, error) {
	options.Search = strings.TrimSpace(options.Search)
	options.Status = normalizePluginStatus(options.Status)
	rows, err := s.pool.Query(ctx, `SELECT id, slug, display_name, description, version, content_digest, status, manifest_json FROM gantry.plugins WHERE organization_id=$1 AND ($2='' OR status=$2) AND ($3='' OR display_name ILIKE '%' || $3 || '%' OR slug ILIKE '%' || $3 || '%' OR version ILIKE '%' || $3 || '%') ORDER BY display_name, version DESC, id`, actor.OrganizationID, options.Status, options.Search)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Plugin, 0)
	for rows.Next() {
		var item Plugin
		if err := rows.Scan(&item.ID, &item.Slug, &item.DisplayName, &item.Description, &item.Version, &item.ContentDigest, &item.Status, &item.Manifest); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) GetPlugin(ctx context.Context, actor identity.Principal, pluginID string) (PluginDetail, error) {
	var item Plugin
	err := s.pool.QueryRow(ctx, `SELECT id, slug, display_name, description, version, content_digest, status, manifest_json FROM gantry.plugins WHERE id=$1 AND organization_id=$2`, pluginID, actor.OrganizationID).Scan(
		&item.ID, &item.Slug, &item.DisplayName, &item.Description, &item.Version, &item.ContentDigest, &item.Status, &item.Manifest)
	if errors.Is(err, pgx.ErrNoRows) {
		return PluginDetail{}, ErrNotFound
	}
	if err != nil {
		return PluginDetail{}, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT w.id, w.display_name
		FROM gantry.workspace_plugin_enablements e
		JOIN gantry.workspaces w ON w.id=e.workspace_id
		WHERE e.plugin_id=$1 AND w.organization_id=$2 AND (
			EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$3 AND rb.role='organization_admin' AND rb.workspace_id IS NULL)
			OR EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$3 AND rb.role='workspace_agent_editor' AND rb.workspace_id=w.id)
		)
		ORDER BY w.display_name, w.id`, pluginID, actor.OrganizationID, actor.ID)
	if err != nil {
		return PluginDetail{}, err
	}
	defer rows.Close()
	detail := PluginDetail{Plugin: item, Workspaces: make([]PluginWorkspace, 0)}
	for rows.Next() {
		var workspace PluginWorkspace
		if err := rows.Scan(&workspace.ID, &workspace.DisplayName); err != nil {
			return PluginDetail{}, err
		}
		detail.Workspaces = append(detail.Workspaces, workspace)
	}
	if err := rows.Err(); err != nil {
		return PluginDetail{}, err
	}
	return detail, nil
}

func (s *Service) ListPluginUsage(ctx context.Context, actor identity.Principal, pluginID string) ([]AssetUsage, error) {
	if _, err := s.GetPlugin(ctx, actor, pluginID); err != nil {
		return nil, err
	}
	return s.listUsage(ctx, actor, pluginID, "", "plugins", "plugin_version_id")
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
	if len(request.Manifest) == 0 {
		request.Manifest = json.RawMessage(`{}`)
	}
	var manifest map[string]any
	if err := json.Unmarshal(request.Manifest, &manifest); err != nil || manifest == nil {
		return Plugin{}, ErrInvalidInput
	}
	item := Plugin{ID: newID("plugin"), Slug: request.Slug, DisplayName: request.DisplayName, Description: strings.TrimSpace(request.Description), Version: request.Version, ContentDigest: request.ContentDigest, Manifest: request.Manifest, Status: "active"}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Plugin{}, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `INSERT INTO gantry.plugins (id, organization_id, slug, display_name, description, version, content_digest, manifest_json, status, created_by_principal_id) VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,'active',$9) RETURNING id`, item.ID, actor.OrganizationID, item.Slug, item.DisplayName, item.Description, item.Version, item.ContentDigest, string(item.Manifest), actor.ID).Scan(&item.ID)
	if err != nil {
		return Plugin{}, err
	}
	if err := appendCreatedAudit(ctx, tx, actor.OrganizationID, actor.ID, "plugin", item.ID, map[string]string{"version": item.Version, "content_digest": item.ContentDigest}); err != nil {
		return Plugin{}, err
	}
	if err := tx.Commit(ctx); err != nil {
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
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var found bool
	err = tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM gantry.plugins WHERE id=$1 AND organization_id=$2 AND status='active') AND EXISTS (SELECT 1 FROM gantry.workspaces WHERE id=$3 AND organization_id=$2)`, pluginID, actor.OrganizationID, workspaceID).Scan(&found)
	if err != nil {
		return err
	}
	if !found {
		return ErrNotFound
	}
	_, err = tx.Exec(ctx, `INSERT INTO gantry.workspace_plugin_enablements (workspace_id, plugin_id, enabled_by_principal_id) VALUES ($1,$2,$3) ON CONFLICT (workspace_id,plugin_id) DO NOTHING`, workspaceID, pluginID, actor.ID)
	if err != nil {
		return err
	}
	if err := appendAssetEvent(ctx, tx, actor.OrganizationID, actor.ID, "plugin", pluginID, "configuration_asset.workspace_enabled", map[string]string{"workspace_id": workspaceID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) DisablePlugin(ctx context.Context, actor identity.Principal, pluginID, workspaceID string) error {
	if err := s.authz.RequireOrganizationAdmin(ctx, actor); err != nil {
		return err
	}
	pluginID = strings.TrimSpace(pluginID)
	workspaceID = strings.TrimSpace(workspaceID)
	if pluginID == "" || workspaceID == "" {
		return ErrInvalidInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var organizationID string
	if err := tx.QueryRow(ctx, `SELECT organization_id FROM gantry.plugins WHERE id=$1`, pluginID).Scan(&organizationID); errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if organizationID != actor.OrganizationID {
		return ErrNotFound
	}
	command, err := tx.Exec(ctx, `DELETE FROM gantry.workspace_plugin_enablements WHERE plugin_id=$1 AND workspace_id=$2`, pluginID, workspaceID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrNotFound
	}
	payload, err := json.Marshal(map[string]string{"workspace_id": workspaceID, "plugin_id": pluginID})
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.audit_events (organization_id, actor_principal_id, resource_type, resource_id, event_type, payload) VALUES ($1,$2,'plugin',$3,'configuration_asset.workspace_disabled',$4::jsonb)`, actor.OrganizationID, actor.ID, pluginID, string(payload)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) ActivateSkill(ctx context.Context, actor identity.Principal, skillID, reason string) error {
	return s.setSkillStatus(ctx, actor, skillID, "available", reason)
}

func (s *Service) DeprecateSkill(ctx context.Context, actor identity.Principal, skillID, reason string) error {
	return s.setSkillStatus(ctx, actor, skillID, "deprecated", reason)
}

func (s *Service) RetireSkill(ctx context.Context, actor identity.Principal, skillID, reason string) error {
	return s.setSkillStatus(ctx, actor, skillID, "retired", reason)
}

func (s *Service) setSkillStatus(ctx context.Context, actor identity.Principal, skillID, target, reason string) error {
	var workspaceID string
	if err := s.pool.QueryRow(ctx, `SELECT workspace_id FROM gantry.skills WHERE id=$1 AND organization_id=$2`, skillID, actor.OrganizationID).Scan(&workspaceID); errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if err := s.authz.RequireWorkspace(ctx, actor, workspaceID); err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var current string
	if err := tx.QueryRow(ctx, `SELECT status FROM gantry.skills WHERE id=$1 AND organization_id=$2 FOR UPDATE`, skillID, actor.OrganizationID).Scan(&current); errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if current == target {
		return tx.Commit(ctx)
	}
	if !validSkillTransition(current, target) {
		return ErrInvalidInput
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.skills SET status=$3 WHERE id=$1 AND organization_id=$2`, skillID, actor.OrganizationID, target); err != nil {
		return err
	}
	if err := appendAudit(ctx, tx, actor.OrganizationID, actor.ID, "skill", skillID, current, target, reason); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) ActivatePlugin(ctx context.Context, actor identity.Principal, pluginID, reason string) error {
	return s.setPluginStatus(ctx, actor, pluginID, "active", reason)
}

func (s *Service) DeprecatePlugin(ctx context.Context, actor identity.Principal, pluginID, reason string) error {
	return s.setPluginStatus(ctx, actor, pluginID, "deprecated", reason)
}

func (s *Service) RetirePlugin(ctx context.Context, actor identity.Principal, pluginID, reason string) error {
	return s.setPluginStatus(ctx, actor, pluginID, "retired", reason)
}

func (s *Service) setPluginStatus(ctx context.Context, actor identity.Principal, pluginID, target, reason string) error {
	if err := s.authz.RequireOrganizationAdmin(ctx, actor); err != nil {
		return err
	}
	return s.setOrganizationAssetStatus(ctx, actor, "plugin", pluginID, target, reason, `SELECT status FROM gantry.plugins WHERE id=$1 AND organization_id=$2 FOR UPDATE`, `UPDATE gantry.plugins SET status=$3 WHERE id=$1 AND organization_id=$2`, validPluginTransition)
}

func (s *Service) ActivateTool(ctx context.Context, actor identity.Principal, toolID, reason string) error {
	return s.setToolStatus(ctx, actor, toolID, "active", reason)
}

func (s *Service) DeprecateTool(ctx context.Context, actor identity.Principal, toolID, reason string) error {
	return s.setToolStatus(ctx, actor, toolID, "deprecated", reason)
}

func (s *Service) RetireTool(ctx context.Context, actor identity.Principal, toolID, reason string) error {
	return s.setToolStatus(ctx, actor, toolID, "retired", reason)
}

func (s *Service) setToolStatus(ctx context.Context, actor identity.Principal, toolID, target, reason string) error {
	if err := s.authz.RequireOrganizationAdmin(ctx, actor); err != nil {
		return err
	}
	return s.setOrganizationAssetStatus(ctx, actor, "tool_descriptor", toolID, target, reason, `SELECT d.status FROM gantry.tool_descriptors d JOIN gantry.tool_servers s ON s.id=d.server_id WHERE d.id=$1 AND s.organization_id=$2 FOR UPDATE`, `UPDATE gantry.tool_descriptors d SET status=$3 FROM gantry.tool_servers s WHERE d.id=$1 AND s.id=d.server_id AND s.organization_id=$2`, validToolTransition)
}

type statusTransition func(string, string) bool

func (s *Service) setOrganizationAssetStatus(ctx context.Context, actor identity.Principal, resourceType, assetID, target, reason, selectSQL, updateSQL string, transition statusTransition) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var current string
	if err := tx.QueryRow(ctx, selectSQL, assetID, actor.OrganizationID).Scan(&current); errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if current == target {
		return tx.Commit(ctx)
	}
	if !transition(current, target) {
		return ErrInvalidInput
	}
	if _, err := tx.Exec(ctx, updateSQL, assetID, actor.OrganizationID, target); err != nil {
		return err
	}
	if err := appendAudit(ctx, tx, actor.OrganizationID, actor.ID, resourceType, assetID, current, target, reason); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func validSkillTransition(current, target string) bool {
	return (current == "available" && (target == "deprecated" || target == "retired")) ||
		(current == "deprecated" && (target == "available" || target == "retired"))
}

func validPluginTransition(current, target string) bool {
	return (current == "active" && (target == "deprecated" || target == "retired")) ||
		(current == "deprecated" && (target == "active" || target == "retired"))
}

func validToolTransition(current, target string) bool {
	return (current == "proposed" && target == "active") ||
		(current == "active" && (target == "deprecated" || target == "retired")) ||
		(current == "deprecated" && (target == "active" || target == "retired"))
}

func appendAudit(ctx context.Context, tx pgx.Tx, organizationID, actorID, resourceType, resourceID, previous, next, reason string) error {
	payload, err := json.Marshal(map[string]string{"previous_status": previous, "status": next, "reason": strings.TrimSpace(reason)})
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO gantry.audit_events (organization_id, actor_principal_id, resource_type, resource_id, event_type, payload) VALUES ($1,$2,$3,$4,'configuration_asset.status_changed',$5::jsonb)`, organizationID, actorID, resourceType, resourceID, string(payload))
	return err
}

func appendCreatedAudit(ctx context.Context, tx pgx.Tx, organizationID, actorID, resourceType, resourceID string, payload any) error {
	return appendAssetEvent(ctx, tx, organizationID, actorID, resourceType, resourceID, "configuration_asset.created", payload)
}

func appendAssetEvent(ctx context.Context, tx pgx.Tx, organizationID, actorID, resourceType, resourceID, eventType string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO gantry.audit_events (organization_id, actor_principal_id, resource_type, resource_id, event_type, payload) VALUES ($1,$2,$3,$4,$5,$6::jsonb)`, organizationID, actorID, resourceType, resourceID, eventType, string(data))
	return err
}

func (s *Service) ListTools(ctx context.Context, actor identity.Principal, options ListOptions) ([]Tool, error) {
	options.Search = strings.TrimSpace(options.Search)
	options.Status = normalizeToolStatus(options.Status)
	rows, err := s.pool.Query(ctx, `SELECT d.id, s.id, s.name, s.server_type, s.endpoint_ref, d.fully_qualified_name, d.version, d.effect, d.idempotency, d.content_digest, d.schema_json, d.status FROM gantry.tool_descriptors d JOIN gantry.tool_servers s ON s.id=d.server_id WHERE s.organization_id=$1 AND s.status <> 'retired' AND ($2='' OR d.status=$2) AND ($3='' OR s.name ILIKE '%' || $3 || '%' OR d.fully_qualified_name ILIKE '%' || $3 || '%' OR d.version ILIKE '%' || $3 || '%') ORDER BY s.name, d.fully_qualified_name, d.version DESC`, actor.OrganizationID, options.Status, options.Search)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Tool, 0)
	for rows.Next() {
		var item Tool
		if err := rows.Scan(&item.ID, &item.ServerID, &item.ServerName, &item.ServerType, &item.EndpointRef, &item.FullyQualifiedName, &item.Version, &item.Effect, &item.Idempotency, &item.ContentDigest, &item.Schema, &item.Status); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) GetTool(ctx context.Context, actor identity.Principal, toolID string) (Tool, error) {
	var item Tool
	err := s.pool.QueryRow(ctx, `
		SELECT d.id, s.id, s.name, s.server_type, s.endpoint_ref, d.fully_qualified_name,
			d.version, d.effect, d.idempotency, d.content_digest, d.schema_json, d.status
		FROM gantry.tool_descriptors d
		JOIN gantry.tool_servers s ON s.id=d.server_id
		WHERE d.id=$1 AND s.organization_id=$2`, toolID, actor.OrganizationID).Scan(
		&item.ID, &item.ServerID, &item.ServerName, &item.ServerType, &item.EndpointRef,
		&item.FullyQualifiedName, &item.Version, &item.Effect, &item.Idempotency,
		&item.ContentDigest, &item.Schema, &item.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Tool{}, ErrNotFound
	}
	if err != nil {
		return Tool{}, err
	}
	return item, nil
}

func (s *Service) ListToolUsage(ctx context.Context, actor identity.Principal, toolID string) ([]AssetUsage, error) {
	if _, err := s.GetTool(ctx, actor, toolID); err != nil {
		return nil, err
	}
	return s.listUsage(ctx, actor, toolID, "", "tool_bindings", "descriptor_id")
}

func (s *Service) listUsage(ctx context.Context, actor identity.Principal, assetID, workspaceID, path, bindingField string) ([]AssetUsage, error) {
	query := `
		SELECT a.id, a.display_name, a.workspace_id, 'draft' AS reference_kind, d.id AS reference_id, COALESCE(d.latest_revision_hash, '') AS reference_hash
		FROM gantry.agent_draft_workspaces d
		JOIN gantry.agents a ON a.id=d.agent_id
		WHERE a.organization_id=$1 AND ($2='' OR a.workspace_id=$2)
		  AND d.spec_json->'` + path + `' @> jsonb_build_array(jsonb_build_object('` + bindingField + `', $3::text))
		  AND (
			EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$4 AND rb.role='organization_admin' AND rb.workspace_id IS NULL)
			OR EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$4 AND rb.role='workspace_agent_editor' AND rb.workspace_id=a.workspace_id)
		  )
		UNION ALL
		SELECT a.id, a.display_name, a.workspace_id, 'revision' AS reference_kind, v.id AS reference_id, v.revision_hash AS reference_hash
		FROM gantry.agent_revisions v
		JOIN gantry.agents a ON a.id=v.agent_id
		WHERE a.organization_id=$1 AND ($2='' OR a.workspace_id=$2)
		  AND v.spec_json->'` + path + `' @> jsonb_build_array(jsonb_build_object('` + bindingField + `', $3::text))
		  AND (
			EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$4 AND rb.role='organization_admin' AND rb.workspace_id IS NULL)
			OR EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$4 AND rb.role='workspace_agent_editor' AND rb.workspace_id=a.workspace_id)
		  )
		ORDER BY display_name, reference_kind, reference_id`
	rows, err := s.pool.Query(ctx, query, actor.OrganizationID, workspaceID, assetID, actor.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]AssetUsage, 0)
	for rows.Next() {
		var item AssetUsage
		if err := rows.Scan(&item.AgentID, &item.AgentName, &item.WorkspaceID, &item.ReferenceKind, &item.ReferenceID, &item.ReferenceHash); err != nil {
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
	if len(request.Schema) == 0 {
		request.Schema = json.RawMessage(`{}`)
	}
	if request.ServerName == "" || request.FullyQualifiedName == "" || request.Version == "" || request.ContentDigest == "" || !validServerType(request.ServerType) || !validEffect(request.Effect) || !validIdempotency(request.Idempotency) {
		return Tool{}, ErrInvalidInput
	}
	var schema map[string]any
	if err := json.Unmarshal(request.Schema, &schema); err != nil || schema == nil {
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
	item := Tool{ID: newID("tool"), ServerID: serverID, ServerName: request.ServerName, ServerType: serverType, EndpointRef: strings.TrimSpace(request.EndpointRef), FullyQualifiedName: request.FullyQualifiedName, Version: request.Version, Effect: request.Effect, Idempotency: request.Idempotency, ContentDigest: request.ContentDigest, Schema: request.Schema, Status: "active"}
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.tool_descriptors (id, server_id, fully_qualified_name, version, effect, idempotency, schema_json, content_digest, status) VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8,'active')`, item.ID, serverID, item.FullyQualifiedName, item.Version, item.Effect, item.Idempotency, string(item.Schema), item.ContentDigest); err != nil {
		return Tool{}, err
	}
	if err := appendCreatedAudit(ctx, tx, actor.OrganizationID, actor.ID, "tool_descriptor", item.ID, map[string]string{"fully_qualified_name": item.FullyQualifiedName, "version": item.Version, "content_digest": item.ContentDigest}); err != nil {
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

func normalizeSkillStatus(value string) string {
	if value == "available" || value == "deprecated" || value == "retired" {
		return value
	}
	return ""
}

func normalizePluginStatus(value string) string {
	if value == "active" || value == "deprecated" || value == "retired" {
		return value
	}
	return ""
}

func normalizeToolStatus(value string) string {
	if value == "active" || value == "proposed" || value == "deprecated" || value == "retired" {
		return value
	}
	return ""
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
