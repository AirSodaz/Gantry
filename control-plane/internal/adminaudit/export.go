package adminaudit

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/jackc/pgx/v5"
)

var (
	ErrExportNotReady    = errors.New("audit export is not ready")
	ErrExportUnavailable = errors.New("audit export is unavailable")
)

type exportPackage struct {
	SchemaVersion string   `json:"schema_version"`
	ExportID      string   `json:"export_id"`
	QueryDigest   string   `json:"query_digest"`
	Scope         string   `json:"scope"`
	GeneratedAt   string   `json:"generated_at"`
	Events        []Detail `json:"events"`
}

func (s *Service) CreateExport(ctx context.Context, actor identity.Principal, options ListOptions) (Export, error) {
	if err := s.authz.RequireOrganizationAdmin(ctx, actor); err != nil {
		return Export{}, err
	}
	if s.store == nil {
		return Export{}, ErrExportUnavailable
	}
	options = normalizeOptions(options)
	if options.WorkspaceID != "" {
		if err := s.authz.RequireWorkspace(ctx, actor, options.WorkspaceID); err != nil {
			return Export{}, err
		}
	}
	if !validTimeFilter(options.Before) || !validTimeFilter(options.After) {
		return Export{}, ErrInvalidInput
	}
	query := exportQuery(options)
	queryJSON, err := json.Marshal(query)
	if err != nil {
		return Export{}, err
	}
	digest := digestBytes(queryJSON)
	scope := options.WorkspaceID
	if scope == "" {
		scope = "organization:" + actor.OrganizationID
	}
	exportID := newID("aex")
	_, err = s.pool.Exec(ctx, `
		INSERT INTO gantry.audit_exports
		(id, organization_id, requested_by_principal_id, query_json, query_digest, scope, state)
		VALUES ($1,$2,$3,$4::jsonb,$5,$6,'requested')`, exportID, actor.OrganizationID, actor.ID, string(queryJSON), digest, scope)
	if err != nil {
		return Export{}, err
	}
	_ = s.recordExportAudit(ctx, actor, exportID, "audit.export_requested", map[string]any{"query_digest": digest, "scope": scope})
	go s.processExport(exportID, actor, query)
	return s.GetExport(ctx, actor, exportID)
}

func (s *Service) GetExport(ctx context.Context, actor identity.Principal, exportID string) (Export, error) {
	if err := s.authz.RequireOrganizationAdmin(ctx, actor); err != nil {
		return Export{}, err
	}
	item, expired, err := s.loadExport(ctx, actor.OrganizationID, exportID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Export{}, ErrNotFound
	}
	if err != nil {
		return Export{}, err
	}
	if expired {
		_, _ = s.pool.Exec(ctx, `UPDATE gantry.audit_exports SET state='expired', updated_at=now() WHERE id=$1 AND state='ready'`, exportID)
		_ = s.recordExportAudit(ctx, actor, exportID, "audit.export_expired", map[string]any{"package_digest": item.PackageDigest})
		item.State = "expired"
	}
	return item, nil
}

func (s *Service) DownloadExport(ctx context.Context, actor identity.Principal, exportID string) (Download, error) {
	if s.store == nil {
		return Download{}, ErrExportUnavailable
	}
	item, err := s.GetExport(ctx, actor, exportID)
	if err != nil {
		return Download{}, err
	}
	if item.State != "ready" {
		return Download{}, ErrExportNotReady
	}
	var objectKey string
	if err := s.pool.QueryRow(ctx, `SELECT object_key FROM gantry.audit_exports WHERE id=$1 AND organization_id=$2`, exportID, actor.OrganizationID).Scan(&objectKey); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Download{}, ErrNotFound
		}
		return Download{}, err
	}
	url, expiresAt, err := s.store.PresignGet(ctx, objectKey, 5*time.Minute)
	if err != nil {
		return Download{}, err
	}
	_, _ = s.pool.Exec(ctx, `UPDATE gantry.audit_exports SET download_count=download_count+1, updated_at=now() WHERE id=$1 AND organization_id=$2`, exportID, actor.OrganizationID)
	_ = s.recordExportAudit(ctx, actor, exportID, "audit.export_downloaded", map[string]any{"package_digest": item.PackageDigest, "expires_at": expiresAt.UTC().Format(time.RFC3339)})
	return Download{URL: url, ExpiresAt: expiresAt.UTC().Format(time.RFC3339)}, nil
}

func (s *Service) processExport(exportID string, actor identity.Principal, options ListOptions) {
	ctx := context.Background()
	_, _ = s.pool.Exec(ctx, `UPDATE gantry.audit_exports SET state='processing', updated_at=now() WHERE id=$1 AND state='requested'`, exportID)
	if err := s.generateExport(ctx, exportID, actor, options); err != nil {
		_, _ = s.pool.Exec(ctx, `UPDATE gantry.audit_exports SET state='failed', failure_reason=$2, updated_at=now() WHERE id=$1`, exportID, truncateError(err))
		_ = s.recordExportAudit(ctx, actor, exportID, "audit.export_failed", map[string]any{"reason": truncateError(err)})
	}
}

func (s *Service) generateExport(ctx context.Context, exportID string, actor identity.Principal, options ListOptions) error {
	items := make([]Detail, 0)
	options.Limit = 100
	options.Cursor = ""
	for {
		page, err := s.List(ctx, actor, options)
		if err != nil {
			return err
		}
		for _, event := range page.Items {
			detail, err := s.Get(ctx, actor, fmt.Sprintf("%d", event.ID))
			if err != nil {
				return err
			}
			items = append(items, detail)
		}
		if !page.PageInfo.HasMore {
			break
		}
		options.Cursor = page.PageInfo.NextCursor
	}
	queryJSON, err := json.Marshal(exportQuery(options))
	if err != nil {
		return err
	}
	queryDigest := digestBytes(queryJSON)
	scope := options.WorkspaceID
	if scope == "" {
		scope = "organization:" + actor.OrganizationID
	}
	payload, err := json.Marshal(exportPackage{SchemaVersion: "gantry.audit-export/v1", ExportID: exportID, QueryDigest: queryDigest, Scope: scope, GeneratedAt: time.Now().UTC().Format(time.RFC3339), Events: items})
	if err != nil {
		return err
	}
	packageDigest := digestBytes(payload)
	objectKey := "audit-exports/" + exportID + ".json"
	if err := s.store.Put(ctx, objectKey, bytes.NewReader(payload), int64(len(payload)), "application/json"); err != nil {
		return err
	}
	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	_, err = s.pool.Exec(ctx, `UPDATE gantry.audit_exports SET state='ready', package_digest=$2, object_key=$3, expires_at=$4, updated_at=now() WHERE id=$1`, exportID, packageDigest, objectKey, expiresAt)
	if err != nil {
		return err
	}
	_ = s.recordExportAudit(ctx, actor, exportID, "audit.export_ready", map[string]any{"package_digest": packageDigest, "expires_at": expiresAt.Format(time.RFC3339)})
	return nil
}

func (s *Service) loadExport(ctx context.Context, organizationID, exportID string) (Export, bool, error) {
	var item Export
	var createdAt, updatedAt time.Time
	var expiresAt *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT id, query_digest, scope, state, package_digest, download_count, expires_at, failure_reason, created_at, updated_at
		FROM gantry.audit_exports WHERE id=$1 AND organization_id=$2`, exportID, organizationID).
		Scan(&item.ID, &item.QueryDigest, &item.Scope, &item.State, &item.PackageDigest, &item.DownloadCount, &expiresAt, &item.FailureReason, &createdAt, &updatedAt)
	if err != nil {
		return Export{}, false, err
	}
	item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	item.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	if expiresAt != nil {
		item.ExpiresAt = expiresAt.UTC().Format(time.RFC3339)
	}
	return item, expiresAt != nil && expiresAt.Before(time.Now().UTC()) && item.State == "ready", nil
}

func (s *Service) recordExportAudit(ctx context.Context, actor identity.Principal, exportID, eventType string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO gantry.audit_events (organization_id, actor_principal_id, resource_type, resource_id, event_type, payload) VALUES ($1,$2,'audit_export',$3,$4,$5::jsonb)`, actor.OrganizationID, actor.ID, exportID, eventType, string(data))
	return err
}

func exportQuery(options ListOptions) ListOptions {
	options.Limit = 0
	options.Cursor = ""
	return options
}

func digestBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func truncateError(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	if len(message) > 500 {
		return message[:500]
	}
	return message
}

func newID(prefix string) string {
	value := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, value); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(value)
}
