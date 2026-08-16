package adminaudit

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/AirSodaz/gantry/internal/authorization"
	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/AirSodaz/gantry/internal/objectstore"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound     = errors.New("admin audit event not found")
	ErrInvalidInput = errors.New("invalid admin audit query")
)

type Service struct {
	pool  *pgxpool.Pool
	authz *authorization.Service
	store objectstore.ArtifactStore
}

func NewService(pool *pgxpool.Pool, authz *authorization.Service) *Service {
	return NewServiceWithStore(pool, authz, nil)
}

func NewServiceWithStore(pool *pgxpool.Pool, authz *authorization.Service, store objectstore.ArtifactStore) *Service {
	return &Service{pool: pool, authz: authz, store: store}
}

const accessibleEvent = `
	e.organization_id=$1 AND (
		EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$2 AND rb.role='organization_admin' AND rb.workspace_id IS NULL)
		OR EXISTS (
			SELECT 1 FROM gantry.role_bindings rb
			WHERE rb.principal_id=$2 AND rb.role='workspace_agent_editor' AND rb.workspace_id = COALESCE(
				e.payload->>'workspace_id',
				CASE
					WHEN e.resource_type='agent' THEN (SELECT a.workspace_id FROM gantry.agents a WHERE a.id=e.resource_id)
					WHEN e.resource_type='agent_revision' THEN (SELECT a.workspace_id FROM gantry.agent_revisions v JOIN gantry.agents a ON a.id=v.agent_id WHERE v.id=e.resource_id)
					WHEN e.resource_type='agent_revision_review' THEN (SELECT a.workspace_id FROM gantry.agent_revision_reviews v JOIN gantry.agents a ON a.id=v.agent_id WHERE v.id=e.resource_id)
					WHEN e.resource_type='skill' THEN (SELECT s.workspace_id FROM gantry.skills s WHERE s.id=e.resource_id)
					WHEN e.resource_type='run' THEN (SELECT t.workspace_id FROM gantry.runs r JOIN gantry.tasks t ON t.id=r.task_id WHERE r.id=e.resource_id)
					WHEN e.resource_type='policy' THEN (SELECT COALESCE(p.workspace_id, '') FROM gantry.policies p WHERE p.id=e.resource_id)
					WHEN e.resource_type='evaluation_suite' THEN (SELECT s.workspace_id FROM gantry.evaluation_suites s WHERE s.id=e.resource_id)
					ELSE ''
				END
			)
		)
	)`

const eventProjection = `
	SELECT e.id, e.actor_principal_id, COALESCE(NULLIF(actor.display_name, ''), actor.external_subject, e.actor_principal_id),
		e.resource_type, e.resource_id, e.event_type,
		COALESCE(e.payload->>'workspace_id',
			CASE
				WHEN e.resource_type='agent' THEN (SELECT a.workspace_id FROM gantry.agents a WHERE a.id=e.resource_id)
				WHEN e.resource_type='agent_revision' THEN (SELECT a.workspace_id FROM gantry.agent_revisions v JOIN gantry.agents a ON a.id=v.agent_id WHERE v.id=e.resource_id)
				WHEN e.resource_type='agent_revision_review' THEN (SELECT a.workspace_id FROM gantry.agent_revision_reviews v JOIN gantry.agents a ON a.id=v.agent_id WHERE v.id=e.resource_id)
				WHEN e.resource_type='skill' THEN (SELECT s.workspace_id FROM gantry.skills s WHERE s.id=e.resource_id)
				WHEN e.resource_type='run' THEN (SELECT t.workspace_id FROM gantry.runs r JOIN gantry.tasks t ON t.id=r.task_id WHERE r.id=e.resource_id)
				WHEN e.resource_type='policy' THEN (SELECT COALESCE(p.workspace_id, '') FROM gantry.policies p WHERE p.id=e.resource_id)
				WHEN e.resource_type='evaluation_suite' THEN (SELECT s.workspace_id FROM gantry.evaluation_suites s WHERE s.id=e.resource_id)
				ELSE ''
			END, ''),
		COALESCE(e.payload->>'outcome', ''), COALESCE(e.payload->>'risk', ''),
		COALESCE(e.payload->>'correlation_id', ''),
		COALESCE(e.payload->>'run_id', CASE WHEN e.resource_type='run' THEN e.resource_id ELSE '' END),
		COALESCE(e.payload->>'revision_hash', ''), COALESCE(e.payload->>'policy_version_id', ''), e.created_at`

func (s *Service) List(ctx context.Context, actor identity.Principal, options ListOptions) (ListResult, error) {
	options = normalizeOptions(options)
	if options.WorkspaceID != "" {
		if err := s.authz.RequireWorkspace(ctx, actor, options.WorkspaceID); err != nil {
			return ListResult{}, err
		}
	}
	if options.Cursor != "" && !validCursor(options.Cursor) {
		return ListResult{}, ErrInvalidInput
	}
	if !validTimeFilter(options.Before) || !validTimeFilter(options.After) {
		return ListResult{}, ErrInvalidInput
	}
	args := []any{actor.OrganizationID, actor.ID, options.WorkspaceID, options.ResourceType, options.ResourceID, options.ActorID, options.EventType, options.Outcome, options.Risk, options.CorrelationID, options.RunID, options.RevisionHash, options.PolicyVersionID, options.Before, options.After}
	query := eventProjection + `
	FROM gantry.audit_events e
	JOIN gantry.principals actor ON actor.id=e.actor_principal_id
	WHERE ` + accessibleEvent + `
	AND ($3='' OR COALESCE(e.payload->>'workspace_id', CASE
		WHEN e.resource_type='agent' THEN (SELECT a.workspace_id FROM gantry.agents a WHERE a.id=e.resource_id)
		WHEN e.resource_type='agent_revision' THEN (SELECT a.workspace_id FROM gantry.agent_revisions v JOIN gantry.agents a ON a.id=v.agent_id WHERE v.id=e.resource_id)
		WHEN e.resource_type='agent_revision_review' THEN (SELECT a.workspace_id FROM gantry.agent_revision_reviews v JOIN gantry.agents a ON a.id=v.agent_id WHERE v.id=e.resource_id)
		WHEN e.resource_type='skill' THEN (SELECT s.workspace_id FROM gantry.skills s WHERE s.id=e.resource_id)
		WHEN e.resource_type='run' THEN (SELECT t.workspace_id FROM gantry.runs r JOIN gantry.tasks t ON t.id=r.task_id WHERE r.id=e.resource_id)
		WHEN e.resource_type='policy' THEN (SELECT COALESCE(p.workspace_id, '') FROM gantry.policies p WHERE p.id=e.resource_id)
		WHEN e.resource_type='evaluation_suite' THEN (SELECT s.workspace_id FROM gantry.evaluation_suites s WHERE s.id=e.resource_id)
		ELSE '' END)=$3)
	AND ($4='' OR e.resource_type=$4) AND ($5='' OR e.resource_id=$5)
	AND ($6='' OR e.actor_principal_id=$6) AND ($7='' OR e.event_type=$7)
	AND ($8='' OR e.payload->>'outcome'=$8) AND ($9='' OR e.payload->>'risk'=$9)
	AND ($10='' OR e.payload->>'correlation_id'=$10)
	AND ($11='' OR COALESCE(e.payload->>'run_id', CASE WHEN e.resource_type='run' THEN e.resource_id ELSE '' END)=$11)
	AND ($12='' OR e.payload->>'revision_hash'=$12) AND ($13='' OR e.payload->>'policy_version_id'=$13)
	AND ($14='' OR e.created_at >= $14::timestamptz) AND ($15='' OR e.created_at <= $15::timestamptz)`
	if options.Cursor != "" {
		args = append(args, cursorID(options.Cursor))
		query += ` AND e.id < $16`
	}
	limitArg := len(args) + 1
	args = append(args, options.Limit+1)
	query += fmt.Sprintf(` ORDER BY e.created_at DESC, e.id DESC LIMIT $%d`, limitArg)
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return ListResult{}, err
	}
	defer rows.Close()
	items := make([]Event, 0, options.Limit)
	for rows.Next() {
		item, err := scanEvent(rows)
		if err != nil {
			return ListResult{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return ListResult{}, err
	}
	result := ListResult{Items: items, PageInfo: PageInfo{HasMore: len(items) > options.Limit}}
	if result.PageInfo.HasMore {
		result.Items = result.Items[:options.Limit]
		result.PageInfo.NextCursor = encodeCursor(result.Items[len(result.Items)-1].ID)
	}
	return result, nil
}

func (s *Service) Get(ctx context.Context, actor identity.Principal, eventID string) (Detail, error) {
	id, err := strconv.ParseInt(strings.TrimSpace(eventID), 10, 64)
	if err != nil || id < 1 {
		return Detail{}, ErrInvalidInput
	}
	row := s.pool.QueryRow(ctx, eventProjection+`, e.payload
		FROM gantry.audit_events e
		JOIN gantry.principals actor ON actor.id=e.actor_principal_id
		WHERE `+accessibleEvent+` AND e.id=$3`, actor.OrganizationID, actor.ID, id)
	item, payload, err := scanDetail(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Detail{}, ErrNotFound
	}
	if err != nil {
		return Detail{}, err
	}
	redacted, fields := redact(payload)
	return Detail{Event: item, Payload: redacted, Evidence: linkedEvidence(payload), RedactionMetadata: RedactionMetadata{Mode: "capability_scoped", RedactedFields: fields}}, nil
}

func scanEvent(row interface{ Scan(...any) error }) (Event, error) {
	var item Event
	var createdAt time.Time
	err := row.Scan(&item.ID, &item.ActorID, &item.ActorName, &item.ResourceType, &item.ResourceID, &item.EventType, &item.Scope, &item.Outcome, &item.Risk, &item.CorrelationID, &item.RunID, &item.RevisionHash, &item.PolicyVersionID, &createdAt)
	if err == nil {
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	}
	return item, err
}

func scanDetail(row interface{ Scan(...any) error }) (Event, json.RawMessage, error) {
	var item Event
	var payload []byte
	var createdAt time.Time
	err := row.Scan(&item.ID, &item.ActorID, &item.ActorName, &item.ResourceType, &item.ResourceID, &item.EventType, &item.Scope, &item.Outcome, &item.Risk, &item.CorrelationID, &item.RunID, &item.RevisionHash, &item.PolicyVersionID, &createdAt, &payload)
	if err == nil {
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	}
	return item, json.RawMessage(payload), err
}

func normalizeOptions(options ListOptions) ListOptions {
	options.WorkspaceID = strings.TrimSpace(options.WorkspaceID)
	options.ResourceType = strings.TrimSpace(options.ResourceType)
	options.ResourceID = strings.TrimSpace(options.ResourceID)
	options.ActorID = strings.TrimSpace(options.ActorID)
	options.EventType = strings.TrimSpace(options.EventType)
	options.Outcome = strings.TrimSpace(options.Outcome)
	options.Risk = strings.TrimSpace(options.Risk)
	options.CorrelationID = strings.TrimSpace(options.CorrelationID)
	options.RunID = strings.TrimSpace(options.RunID)
	options.RevisionHash = strings.TrimSpace(options.RevisionHash)
	options.PolicyVersionID = strings.TrimSpace(options.PolicyVersionID)
	options.Before = strings.TrimSpace(options.Before)
	options.After = strings.TrimSpace(options.After)
	if options.Limit < 1 {
		options.Limit = 50
	}
	if options.Limit > 100 {
		options.Limit = 100
	}
	return options
}

func validTimeFilter(value string) bool {
	if value == "" {
		return true
	}
	_, err := time.Parse(time.RFC3339, value)
	return err == nil
}

func encodeCursor(id int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(id, 10)))
}

func validCursor(value string) bool {
	return cursorID(value) > 0
}

func cursorID(value string) int64 {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return 0
	}
	id, err := strconv.ParseInt(string(decoded), 10, 64)
	if err != nil {
		return 0
	}
	return id
}

func redact(payload []byte) (json.RawMessage, []string) {
	var value any
	if json.Unmarshal(payload, &value) != nil {
		return json.RawMessage(`{}`), nil
	}
	fields := make([]string, 0)
	redactValue(value, "", &fields)
	result, err := json.Marshal(value)
	if err != nil {
		return json.RawMessage(`{}`), fields
	}
	return result, fields
}

func redactValue(value any, path string, fields *[]string) {
	switch current := value.(type) {
	case map[string]any:
		for key, child := range current {
			lower := strings.ToLower(key)
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			if strings.Contains(lower, "secret") || strings.Contains(lower, "token") || strings.Contains(lower, "password") || strings.Contains(lower, "credential") || lower == "authorization" {
				current[key] = "[REDACTED]"
				*fields = append(*fields, childPath)
				continue
			}
			redactValue(child, childPath, fields)
		}
	case []any:
		for index, child := range current {
			redactValue(child, fmt.Sprintf("%s[%d]", path, index), fields)
		}
	}
}

func linkedEvidence(payload []byte) []Evidence {
	var object map[string]any
	if json.Unmarshal(payload, &object) != nil {
		return []Evidence{}
	}
	keys := []struct{ key, kind string }{{"run_id", "run"}, {"revision_id", "revision"}, {"revision_hash", "revision"}, {"policy_version_id", "policy_version"}, {"deployment_id", "deployment"}, {"artifact_id", "artifact"}, {"approval_id", "approval"}}
	result := make([]Evidence, 0, len(keys))
	seen := map[string]bool{}
	for _, entry := range keys {
		value, ok := object[entry.key].(string)
		if !ok || value == "" || seen[entry.kind+":"+value] {
			continue
		}
		seen[entry.kind+":"+value] = true
		result = append(result, Evidence{Kind: entry.kind, ID: value})
	}
	return result
}
