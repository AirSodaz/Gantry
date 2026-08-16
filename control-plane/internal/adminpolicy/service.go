package adminpolicy

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/AirSodaz/gantry/internal/authorization"
	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound            = errors.New("admin policy not found")
	ErrInvalidInput        = errors.New("invalid admin policy input")
	ErrInvalidState        = errors.New("invalid admin policy state")
	ErrETagConflict        = errors.New("admin policy etag conflict")
	ErrIdempotencyConflict = errors.New("admin policy idempotency conflict")
	ErrSchemaInvalid       = errors.New("admin policy schema is invalid")
)

type Service struct {
	pool  *pgxpool.Pool
	authz *authorization.Service
}

func NewService(pool *pgxpool.Pool, authz *authorization.Service) *Service {
	return &Service{pool: pool, authz: authz}
}

func (s *Service) List(ctx context.Context, actor identity.Principal, options ListOptions) (ListResult, error) {
	options = normalizeListOptions(options)
	if options.WorkspaceID != "" {
		if err := s.authz.RequireWorkspace(ctx, actor, options.WorkspaceID); err != nil {
			return ListResult{}, err
		}
	}
	args := []any{actor.OrganizationID, actor.ID, options.WorkspaceID, options.Type, options.State, options.OwnerID, options.BindingTarget}
	query := `
		SELECT p.id, p.organization_id, p.workspace_id, p.type, p.name, p.owner_principal_id,
			p.state, p.schema_version, p.draft_etag::text, p.latest_version_id,
			(SELECT count(*) FROM gantry.policy_bindings b WHERE b.policy_id=p.id AND b.state='active')
		FROM gantry.policies p
		WHERE p.organization_id=$1
		  AND ($3='' OR p.workspace_id=$3)
		  AND ($4='' OR p.type=$4)
		  AND ($5='' OR p.state=$5)
		  AND ($6='' OR p.owner_principal_id=$6)
		  AND ($7='' OR EXISTS (SELECT 1 FROM gantry.policy_bindings b WHERE b.policy_id=p.id AND (b.target_workspace_id=$7 OR b.target_scope=$7)))
		  AND (
			EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$2 AND rb.role='organization_admin' AND rb.workspace_id IS NULL)
			OR EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$2 AND rb.role='workspace_agent_editor' AND rb.workspace_id=p.workspace_id)
		  )`
	if options.Cursor != "" {
		cursor, err := decodeCursor(options.Cursor)
		if err != nil {
			return ListResult{}, ErrInvalidInput
		}
		args = append(args, cursor)
		query += fmt.Sprintf(" AND p.id < $%d", len(args))
	}
	args = append(args, options.Limit+1)
	query += fmt.Sprintf(" ORDER BY p.name, p.id LIMIT $%d", len(args))
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return ListResult{}, err
	}
	defer rows.Close()
	items := make([]Policy, 0, options.Limit)
	for rows.Next() {
		item, err := scanPolicy(rows)
		if err != nil {
			return ListResult{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return ListResult{}, err
	}
	result := ListResult{Items: items, PageInfo: PageInfo{NextCursor: nil}}
	if len(items) > options.Limit {
		result.Items = items[:options.Limit]
		cursor := encodeCursor(result.Items[len(result.Items)-1].ID)
		result.PageInfo.NextCursor = &cursor
	}
	return result, nil
}

func (s *Service) Create(ctx context.Context, actor identity.Principal, request CreateRequest) (Policy, Draft, error) {
	request.Type = strings.TrimSpace(request.Type)
	request.Name = strings.TrimSpace(request.Name)
	request.SchemaVersion = strings.TrimSpace(request.SchemaVersion)
	if request.SchemaVersion == "" {
		request.SchemaVersion = "gantry.policy/v1"
	}
	if request.Name == "" || !validPolicyType(request.Type) {
		return Policy{}, Draft{}, ErrInvalidInput
	}
	if err := s.requireScope(ctx, actor, request.WorkspaceID); err != nil {
		return Policy{}, Draft{}, err
	}
	validation, document, err := validateDocument(request.Type, request.Document)
	if err != nil {
		return Policy{}, Draft{}, err
	}
	policyID := newID("pol")
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Policy{}, Draft{}, err
	}
	defer tx.Rollback(ctx)
	var policy Policy
	var workspace any
	if request.WorkspaceID != "" {
		workspace = request.WorkspaceID
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO gantry.policies (id, organization_id, workspace_id, type, name, owner_principal_id, state, schema_version, draft_etag)
		VALUES ($1,$2,$3,$4,$5,$6,'draft',$7,1)
		RETURNING id, organization_id, workspace_id, type, name, owner_principal_id, state, schema_version, draft_etag::text, latest_version_id,
			0`, policyID, actor.OrganizationID, workspace, request.Type, request.Name, actor.ID, request.SchemaVersion).Scan(
		&policy.ID, &policy.OrganizationID, &policy.WorkspaceID, &policy.Type, &policy.Name, &policy.OwnerPrincipalID,
		&policy.State, &policy.SchemaVersion, &policy.DraftETag, &policy.LatestVersionID, &policy.ActiveBindingCount); err != nil {
		return Policy{}, Draft{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.policy_drafts (policy_id, document, schema_version, etag, validation_state, validation_findings, updated_by_principal_id) VALUES ($1,$2::jsonb,$3,1,$4,$5::jsonb,$6)`, policy.ID, string(document), request.SchemaVersion, validation.State, marshalFindings(validation.Findings), actor.ID); err != nil {
		return Policy{}, Draft{}, err
	}
	if err := appendAudit(ctx, tx, actor, policy.ID, policy.WorkspaceID, "policy.created", map[string]any{"type": policy.Type, "workspace_id": stringValue(policy.WorkspaceID), "outcome": "success", "risk": "low"}); err != nil {
		return Policy{}, Draft{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Policy{}, Draft{}, err
	}
	return policy, Draft{PolicyID: policy.ID, Document: document, SchemaVersion: request.SchemaVersion, ETag: "1", Validation: validation}, nil
}

func (s *Service) Get(ctx context.Context, actor identity.Principal, id string) (Policy, error) {
	policy, err := s.getPolicy(ctx, actor, id)
	if err != nil {
		return Policy{}, err
	}
	return policy, nil
}

func (s *Service) GetDraft(ctx context.Context, actor identity.Principal, id string) (Draft, error) {
	policy, err := s.getPolicy(ctx, actor, id)
	if err != nil {
		return Draft{}, err
	}
	var draft Draft
	var findings []byte
	if err := s.pool.QueryRow(ctx, `SELECT policy_id, document, schema_version, etag::text, validation_state, validation_findings FROM gantry.policy_drafts WHERE policy_id=$1`, policy.ID).Scan(&draft.PolicyID, &draft.Document, &draft.SchemaVersion, &draft.ETag, &draft.Validation.State, &findings); errors.Is(err, pgx.ErrNoRows) {
		return Draft{}, ErrNotFound
	} else if err != nil {
		return Draft{}, err
	}
	_ = json.Unmarshal(findings, &draft.Validation.Findings)
	if draft.Validation.Findings == nil {
		draft.Validation.Findings = []map[string]any{}
	}
	return draft, nil
}

func (s *Service) UpdateDraft(ctx context.Context, actor identity.Principal, id, expectedETag string, request UpdateDraftRequest) (Draft, error) {
	policy, err := s.getPolicy(ctx, actor, id)
	if err != nil {
		return Draft{}, err
	}
	if err := s.requireScope(ctx, actor, stringValue(policy.WorkspaceID)); err != nil {
		return Draft{}, err
	}
	expectedETag = strings.Trim(strings.TrimSpace(expectedETag), `"`)
	if expectedETag == "" {
		return Draft{}, ErrETagConflict
	}
	if request.SchemaVersion == "" {
		request.SchemaVersion = policy.SchemaVersion
	}
	validation, document, err := validateDocument(policy.Type, request.Document)
	if err != nil {
		return Draft{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Draft{}, err
	}
	defer tx.Rollback(ctx)
	var etag string
	err = tx.QueryRow(ctx, `UPDATE gantry.policy_drafts SET document=$3::jsonb, schema_version=$4, etag=etag+1, validation_state=$5, validation_findings=$6::jsonb, updated_by_principal_id=$7, updated_at=now() WHERE policy_id=$1 AND etag::text=$2 RETURNING etag::text`, id, expectedETag, string(document), request.SchemaVersion, validation.State, marshalFindings(validation.Findings), actor.ID).Scan(&etag)
	if errors.Is(err, pgx.ErrNoRows) {
		return Draft{}, ErrETagConflict
	}
	if err != nil {
		return Draft{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.policies SET draft_etag=$2, schema_version=$3, updated_at=now() WHERE id=$1`, id, etag, request.SchemaVersion); err != nil {
		return Draft{}, err
	}
	if err := appendAudit(ctx, tx, actor, id, policy.WorkspaceID, "policy.draft.updated", map[string]any{"workspace_id": stringValue(policy.WorkspaceID), "outcome": "success", "risk": "low"}); err != nil {
		return Draft{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Draft{}, err
	}
	return Draft{PolicyID: id, Document: document, SchemaVersion: request.SchemaVersion, ETag: etag, Validation: validation}, nil
}

func (s *Service) Validate(ctx context.Context, actor identity.Principal, id string) (Draft, error) {
	draft, err := s.GetDraft(ctx, actor, id)
	if err != nil {
		return Draft{}, err
	}
	policy, err := s.getPolicy(ctx, actor, id)
	if err != nil {
		return Draft{}, err
	}
	validation, document, err := validateDocument(policy.Type, draft.Document)
	if err != nil {
		return Draft{}, err
	}
	if _, err := s.pool.Exec(ctx, `UPDATE gantry.policy_drafts SET document=$2::jsonb, validation_state=$3, validation_findings=$4::jsonb, updated_at=now() WHERE policy_id=$1`, id, string(document), validation.State, marshalFindings(validation.Findings)); err != nil {
		return Draft{}, err
	}
	draft.Document, draft.Validation = document, validation
	return draft, nil
}

func (s *Service) ListVersions(ctx context.Context, actor identity.Principal, id string) ([]Version, error) {
	if _, err := s.getPolicy(ctx, actor, id); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT v.id, v.policy_id, v.content_digest, v.schema_version, v.message, v.document, v.compiler_evidence, v.created_by_principal_id, v.created_at FROM gantry.policy_versions v WHERE v.policy_id=$1 ORDER BY v.created_at, v.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Version, 0)
	for rows.Next() {
		item, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) Publish(ctx context.Context, actor identity.Principal, id, expectedETag, key string, request PublishRequest) (Version, error) {
	policy, err := s.getPolicy(ctx, actor, id)
	if err != nil {
		return Version{}, err
	}
	if err := s.requireScope(ctx, actor, stringValue(policy.WorkspaceID)); err != nil {
		return Version{}, err
	}
	draft, err := s.GetDraft(ctx, actor, id)
	if err != nil {
		return Version{}, err
	}
	expectedETag = strings.Trim(strings.TrimSpace(expectedETag), `"`)
	if expectedETag == "" || expectedETag != draft.ETag {
		return Version{}, ErrETagConflict
	}
	if draft.Validation.State != "valid" {
		return Version{}, ErrSchemaInvalid
	}
	request.Message = strings.TrimSpace(request.Message)
	if request.Message == "" || strings.TrimSpace(key) == "" {
		return Version{}, ErrInvalidInput
	}
	digest := digestJSON(draft.Document)
	version := Version{ID: newID("pver"), PolicyID: id, ContentDigest: digest, SchemaVersion: draft.SchemaVersion, Message: request.Message, Document: draft.Document, CompilerEvidence: map[string]any{"validator": "policy-compiler/v1"}, CreatedBy: actor.ID, CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Version{}, err
	}
	defer tx.Rollback(ctx)
	if cached, conflict, err := loadIdempotency(ctx, tx, actor.ID, "publish:"+id, key, digestJSON(map[string]any{"etag": expectedETag, "message": request.Message})); err != nil {
		return Version{}, err
	} else if conflict {
		return Version{}, ErrIdempotencyConflict
	} else if cached != nil {
		_ = json.Unmarshal(cached, &version)
		return version, tx.Commit(ctx)
	}
	var createdAt time.Time
	err = tx.QueryRow(ctx, `INSERT INTO gantry.policy_versions (id, policy_id, content_digest, schema_version, message, document, compiler_evidence, created_by_principal_id) VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7::jsonb,$8) ON CONFLICT (policy_id, content_digest) DO UPDATE SET id=gantry.policy_versions.id RETURNING id, created_at`, version.ID, id, version.ContentDigest, version.SchemaVersion, version.Message, string(version.Document), mustJSON(version.CompilerEvidence), actor.ID).Scan(&version.ID, &createdAt)
	if err != nil {
		return Version{}, err
	}
	version.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	var updated int64
	if err := tx.QueryRow(ctx, `UPDATE gantry.policies SET state='published', latest_version_id=$2, updated_at=now() WHERE id=$1 AND state <> 'retired' RETURNING 1`, id, version.ID).Scan(&updated); errors.Is(err, pgx.ErrNoRows) {
		return Version{}, ErrInvalidState
	} else if err != nil {
		return Version{}, err
	}
	if err := appendAudit(ctx, tx, actor, id, policy.WorkspaceID, "policy.version.published", map[string]any{"workspace_id": stringValue(policy.WorkspaceID), "policy_version_id": version.ID, "content_digest": version.ContentDigest, "outcome": "success", "risk": "medium"}); err != nil {
		return Version{}, err
	}
	if err := saveIdempotency(ctx, tx, actor.ID, "publish:"+id, key, digestJSON(map[string]any{"etag": expectedETag, "message": request.Message}), version); err != nil {
		return Version{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Version{}, err
	}
	return version, nil
}

func (s *Service) ListBindings(ctx context.Context, actor identity.Principal, id string) ([]Binding, error) {
	policy, err := s.getPolicy(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT b.id, b.version_id, b.target_scope, b.target_workspace_id, b.target_resource_id, b.environment, b.state, b.effective_from, b.effective_until, b.reason FROM gantry.policy_bindings b WHERE b.policy_id=$1 ORDER BY b.effective_from DESC, b.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Binding, 0)
	for rows.Next() {
		item, err := scanBinding(rows, policy.OrganizationID)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) Bind(ctx context.Context, actor identity.Principal, id, key string, request BindRequest) (Binding, error) {
	policy, err := s.getPolicy(ctx, actor, id)
	if err != nil {
		return Binding{}, err
	}
	if err := s.requireScope(ctx, actor, stringValue(policy.WorkspaceID)); err != nil {
		return Binding{}, err
	}
	request.Scope, request.Environment = strings.TrimSpace(request.Scope), strings.TrimSpace(request.Environment)
	if request.Scope != "organization" && request.Scope != "workspace" || request.Environment == "" || strings.TrimSpace(request.VersionID) == "" || strings.TrimSpace(key) == "" {
		return Binding{}, ErrInvalidInput
	}
	if request.Scope == "workspace" {
		if request.WorkspaceID == "" {
			return Binding{}, ErrInvalidInput
		}
		if err := s.authz.RequireWorkspace(ctx, actor, request.WorkspaceID); err != nil {
			return Binding{}, err
		}
	} else if err := s.authz.RequireOrganizationAdmin(ctx, actor); err != nil {
		return Binding{}, err
	}
	var versionExists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM gantry.policy_versions WHERE id=$1 AND policy_id=$2)`, request.VersionID, id).Scan(&versionExists); err != nil {
		return Binding{}, err
	}
	if !versionExists {
		return Binding{}, ErrNotFound
	}
	binding := Binding{ID: newID("pbind"), VersionID: request.VersionID, Target: ScopeRef{OrganizationID: actor.OrganizationID, Scope: request.Scope}, Environment: request.Environment, State: "active", EffectiveFrom: time.Now().UTC().Format(time.RFC3339), Reason: strings.TrimSpace(request.Reason)}
	if request.WorkspaceID != "" {
		binding.Target.WorkspaceID = &request.WorkspaceID
	}
	if request.TargetResourceID != "" {
		binding.TargetResourceID = &request.TargetResourceID
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Binding{}, err
	}
	defer tx.Rollback(ctx)
	digest := digestJSON(request)
	if cached, conflict, err := loadIdempotency(ctx, tx, actor.ID, "bind:"+id, key, digest); err != nil {
		return Binding{}, err
	} else if conflict {
		return Binding{}, ErrIdempotencyConflict
	} else if cached != nil {
		_ = json.Unmarshal(cached, &binding)
		return binding, tx.Commit(ctx)
	}
	var targetWorkspace any
	if request.WorkspaceID != "" {
		targetWorkspace = request.WorkspaceID
	}
	var effectiveFrom time.Time
	if err := tx.QueryRow(ctx, `INSERT INTO gantry.policy_bindings (id, policy_id, version_id, target_scope, target_workspace_id, target_resource_id, environment, state, reason, created_by_principal_id) VALUES ($1,$2,$3,$4,$5,$6,$7,'active',$8,$9) RETURNING effective_from`, binding.ID, id, binding.VersionID, binding.Target.Scope, targetWorkspace, request.TargetResourceID, binding.Environment, binding.Reason, actor.ID).Scan(&effectiveFrom); err != nil {
		return Binding{}, err
	}
	binding.EffectiveFrom = effectiveFrom.UTC().Format(time.RFC3339)
	if err := appendAudit(ctx, tx, actor, id, policy.WorkspaceID, "policy.binding.created", map[string]any{"workspace_id": request.WorkspaceID, "policy_version_id": binding.VersionID, "binding_id": binding.ID, "outcome": "success", "risk": "high"}); err != nil {
		return Binding{}, err
	}
	if err := saveIdempotency(ctx, tx, actor.ID, "bind:"+id, key, digest, binding); err != nil {
		return Binding{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Binding{}, err
	}
	return binding, nil
}

func (s *Service) RevokeBinding(ctx context.Context, actor identity.Principal, bindingID, key, reason string) (Binding, error) {
	if strings.TrimSpace(key) == "" || strings.TrimSpace(bindingID) == "" {
		return Binding{}, ErrInvalidInput
	}
	var policyID, workspaceID string
	if err := s.pool.QueryRow(ctx, `SELECT b.policy_id, COALESCE(p.workspace_id,'') FROM gantry.policy_bindings b JOIN gantry.policies p ON p.id=b.policy_id WHERE b.id=$1 AND p.organization_id=$2`, bindingID, actor.OrganizationID).Scan(&policyID, &workspaceID); errors.Is(err, pgx.ErrNoRows) {
		return Binding{}, ErrNotFound
	} else if err != nil {
		return Binding{}, err
	}
	if err := s.requireScope(ctx, actor, workspaceID); err != nil {
		return Binding{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Binding{}, err
	}
	defer tx.Rollback(ctx)
	digest := digestJSON(map[string]string{"binding_id": bindingID, "reason": reason})
	var binding Binding
	if cached, conflict, err := loadIdempotency(ctx, tx, actor.ID, "revoke:"+bindingID, key, digest); err != nil {
		return Binding{}, err
	} else if conflict {
		return Binding{}, ErrIdempotencyConflict
	} else if cached != nil {
		_ = json.Unmarshal(cached, &binding)
		return binding, tx.Commit(ctx)
	}
	var targetScope string
	var targetWorkspace *string
	var targetResource *string
	var effectiveFrom time.Time
	var effectiveUntil *time.Time
	if err := tx.QueryRow(ctx, `UPDATE gantry.policy_bindings SET state='revoked', revoked_at=now(), reason=$2 WHERE id=$1 AND state IN ('pending','active') RETURNING version_id, target_scope, target_workspace_id, target_resource_id, environment, effective_from, effective_until`, bindingID, strings.TrimSpace(reason)).Scan(&binding.VersionID, &targetScope, &targetWorkspace, &targetResource, &binding.Environment, &effectiveFrom, &effectiveUntil); errors.Is(err, pgx.ErrNoRows) {
		return Binding{}, ErrInvalidState
	} else if err != nil {
		return Binding{}, err
	}
	binding.ID, binding.State, binding.Reason = bindingID, "revoked", strings.TrimSpace(reason)
	binding.Target = ScopeRef{OrganizationID: actor.OrganizationID, Scope: targetScope, WorkspaceID: targetWorkspace}
	binding.TargetResourceID = targetResource
	binding.EffectiveFrom = effectiveFrom.UTC().Format(time.RFC3339)
	if effectiveUntil != nil {
		value := effectiveUntil.UTC().Format(time.RFC3339)
		binding.EffectiveUntil = &value
	}
	if err := appendAudit(ctx, tx, actor, policyID, nil, "policy.binding.revoked", map[string]any{"binding_id": bindingID, "policy_version_id": binding.VersionID, "outcome": "success", "risk": "high"}); err != nil {
		return Binding{}, err
	}
	if err := saveIdempotency(ctx, tx, actor.ID, "revoke:"+bindingID, key, digest, binding); err != nil {
		return Binding{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Binding{}, err
	}
	return binding, nil
}

func (s *Service) Simulate(ctx context.Context, actor identity.Principal, id string, request SimulationRequest) (Simulation, error) {
	policy, err := s.getPolicy(ctx, actor, id)
	if err != nil {
		return Simulation{}, err
	}
	var document json.RawMessage
	versionID := request.VersionID
	if versionID != "" {
		if err := s.pool.QueryRow(ctx, `SELECT document FROM gantry.policy_versions WHERE id=$1 AND policy_id=$2`, versionID, id).Scan(&document); errors.Is(err, pgx.ErrNoRows) {
			return Simulation{}, ErrNotFound
		} else if err != nil {
			return Simulation{}, err
		}
	} else {
		draft, err := s.GetDraft(ctx, actor, id)
		if err != nil {
			return Simulation{}, err
		}
		document = draft.Document
	}
	var object map[string]any
	if err := json.Unmarshal(document, &object); err != nil {
		return Simulation{}, ErrSchemaInvalid
	}
	decision, explanation := "allow", "No matching deny or approval rule was found."
	if value, ok := object["default_effect"].(string); ok && (value == "deny" || value == "require_requester_approval") {
		decision, explanation = value, "The typed policy default effect applies to this action."
	}
	if rules, ok := object["rules"].([]any); ok {
		for _, raw := range rules {
			rule, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if effect, ok := rule["effect"].(string); ok && (effect == "deny" || effect == "require_requester_approval" || effect == "allow") {
				decision, explanation = effect, "The first typed approval rule determines the simulation result."
				break
			}
		}
	}
	return Simulation{Decision: decision, ContributingVersions: []map[string]any{{"policy_id": policy.ID, "version_id": versionID, "state": policy.State}}, IneffectiveRules: []map[string]any{}, Explanation: explanation}, nil
}

func (s *Service) Retire(ctx context.Context, actor identity.Principal, id, key, reason string) (Policy, error) {
	policy, err := s.getPolicy(ctx, actor, id)
	if err != nil {
		return Policy{}, err
	}
	if err := s.requireScope(ctx, actor, stringValue(policy.WorkspaceID)); err != nil {
		return Policy{}, err
	}
	if strings.TrimSpace(key) == "" {
		return Policy{}, ErrInvalidInput
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Policy{}, err
	}
	defer tx.Rollback(ctx)
	digest := digestJSON(map[string]string{"policy_id": id, "reason": reason})
	if cached, conflict, err := loadIdempotency(ctx, tx, actor.ID, "retire:"+id, key, digest); err != nil {
		return Policy{}, err
	} else if conflict {
		return Policy{}, ErrIdempotencyConflict
	} else if cached != nil {
		_ = json.Unmarshal(cached, &policy)
		return policy, tx.Commit(ctx)
	}
	var updated int64
	if err := tx.QueryRow(ctx, `UPDATE gantry.policies SET state='retired', updated_at=now() WHERE id=$1 AND state <> 'retired' AND NOT EXISTS (SELECT 1 FROM gantry.policy_bindings WHERE policy_id=$1 AND state='active') RETURNING 1`, id).Scan(&updated); errors.Is(err, pgx.ErrNoRows) {
		return Policy{}, ErrInvalidState
	} else if err != nil {
		return Policy{}, err
	}
	policy.State = "retired"
	if err := appendAudit(ctx, tx, actor, id, policy.WorkspaceID, "policy.retired", map[string]any{"workspace_id": stringValue(policy.WorkspaceID), "reason": strings.TrimSpace(reason), "outcome": "success", "risk": "high"}); err != nil {
		return Policy{}, err
	}
	if err := saveIdempotency(ctx, tx, actor.ID, "retire:"+id, key, digest, policy); err != nil {
		return Policy{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Policy{}, err
	}
	return policy, nil
}

func (s *Service) getPolicy(ctx context.Context, actor identity.Principal, id string) (Policy, error) {
	var policy Policy
	err := s.pool.QueryRow(ctx, `
		SELECT p.id, p.organization_id, p.workspace_id, p.type, p.name, p.owner_principal_id, p.state, p.schema_version, p.draft_etag::text, p.latest_version_id,
			(SELECT count(*) FROM gantry.policy_bindings b WHERE b.policy_id=p.id AND b.state='active')
		FROM gantry.policies p WHERE p.id=$1 AND p.organization_id=$2 AND (
			EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$3 AND rb.role='organization_admin' AND rb.workspace_id IS NULL)
			OR EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$3 AND rb.role='workspace_agent_editor' AND rb.workspace_id=p.workspace_id)
		)`, id, actor.OrganizationID, actor.ID).Scan(&policy.ID, &policy.OrganizationID, &policy.WorkspaceID, &policy.Type, &policy.Name, &policy.OwnerPrincipalID, &policy.State, &policy.SchemaVersion, &policy.DraftETag, &policy.LatestVersionID, &policy.ActiveBindingCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return Policy{}, ErrNotFound
	}
	return policy, err
}

func (s *Service) requireScope(ctx context.Context, actor identity.Principal, workspaceID string) error {
	if workspaceID == "" {
		return s.authz.RequireOrganizationAdmin(ctx, actor)
	}
	return s.authz.RequireWorkspace(ctx, actor, workspaceID)
}

func normalizeListOptions(options ListOptions) ListOptions {
	options.Type, options.State, options.WorkspaceID, options.OwnerID, options.BindingTarget = strings.TrimSpace(options.Type), strings.TrimSpace(options.State), strings.TrimSpace(options.WorkspaceID), strings.TrimSpace(options.OwnerID), strings.TrimSpace(options.BindingTarget)
	if options.Limit < 1 {
		options.Limit = 50
	}
	if options.Limit > 100 {
		options.Limit = 100
	}
	return options
}

func validateDocument(policyType string, raw json.RawMessage) (Validation, json.RawMessage, error) {
	if len(raw) == 0 {
		return Validation{State: "invalid", Findings: []map[string]any{{"code": "document_required", "message": "document is required"}}}, nil, ErrSchemaInvalid
	}
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return Validation{}, nil, ErrSchemaInvalid
	}
	kind, _ := object["kind"].(string)
	if !validPolicyKind(policyType, kind) {
		return Validation{State: "invalid", Findings: []map[string]any{{"code": "kind_mismatch", "message": "document kind does not match policy type"}}}, nil, ErrSchemaInvalid
	}
	required := map[string]string{"approval": "rules", "model": "allowed_routes", "tool": "allowed_effects", "command": "allowed_effects", "network": "egress", "credential": "limits", "data": "limits", "budget": "limits", "retention": "limits", "evaluation": "limits", "publication": "limits"}
	if field := required[kind]; field != "" {
		if _, ok := object[field]; !ok {
			return Validation{State: "invalid", Findings: []map[string]any{{"code": "field_required", "field": field, "message": "typed policy field is required"}}}, nil, ErrSchemaInvalid
		}
	}
	canonical, err := json.Marshal(object)
	if err != nil {
		return Validation{}, nil, ErrSchemaInvalid
	}
	return Validation{State: "valid", Findings: []map[string]any{}}, canonical, nil
}

func validPolicyType(value string) bool {
	switch value {
	case "approval", "model", "tool", "command", "network", "credential", "data", "budget", "retention", "evaluation", "publication":
		return true
	default:
		return false
	}
}

func validPolicyKind(policyType, kind string) bool {
	if policyType == "tool" || policyType == "command" {
		return kind == "tool" || kind == "command"
	}
	return kind == policyType
}

func scanPolicy(row interface{ Scan(...any) error }) (Policy, error) {
	var item Policy
	err := row.Scan(&item.ID, &item.OrganizationID, &item.WorkspaceID, &item.Type, &item.Name, &item.OwnerPrincipalID, &item.State, &item.SchemaVersion, &item.DraftETag, &item.LatestVersionID, &item.ActiveBindingCount)
	return item, err
}

func scanVersion(row interface{ Scan(...any) error }) (Version, error) {
	var item Version
	var evidence []byte
	var created time.Time
	err := row.Scan(&item.ID, &item.PolicyID, &item.ContentDigest, &item.SchemaVersion, &item.Message, &item.Document, &evidence, &item.CreatedBy, &created)
	if err == nil {
		item.CreatedAt = created.UTC().Format(time.RFC3339)
		_ = json.Unmarshal(evidence, &item.CompilerEvidence)
	}
	return item, err
}

func scanBinding(row interface{ Scan(...any) error }, organizationID string) (Binding, error) {
	var item Binding
	var scope string
	var workspace *string
	var effectiveFrom time.Time
	var effectiveUntil *time.Time
	err := row.Scan(&item.ID, &item.VersionID, &scope, &workspace, &item.TargetResourceID, &item.Environment, &item.State, &effectiveFrom, &effectiveUntil, &item.Reason)
	item.Target = ScopeRef{OrganizationID: organizationID, Scope: scope, WorkspaceID: workspace}
	if err == nil {
		item.EffectiveFrom = effectiveFrom.UTC().Format(time.RFC3339)
		if effectiveUntil != nil {
			value := effectiveUntil.UTC().Format(time.RFC3339)
			item.EffectiveUntil = &value
		}
	}
	return item, err
}

func appendAudit(ctx context.Context, tx pgx.Tx, actor identity.Principal, resourceID string, workspaceID *string, eventType string, payload map[string]any) error {
	if workspaceID != nil && payload["workspace_id"] == nil {
		payload["workspace_id"] = *workspaceID
	}
	value, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO gantry.audit_events (organization_id, actor_principal_id, resource_type, resource_id, event_type, payload) VALUES ($1,$2,'policy',$3,$4,$5::jsonb)`, actor.OrganizationID, actor.ID, resourceID, eventType, string(value))
	return err
}

func loadIdempotency(ctx context.Context, tx pgx.Tx, principal, route, key, digest string) ([]byte, bool, error) {
	var storedDigest string
	var response []byte
	err := tx.QueryRow(ctx, `SELECT request_digest, response_json FROM gantry.policy_command_idempotency WHERE principal_id=$1 AND route=$2 AND idempotency_key=$3`, principal, route, key).Scan(&storedDigest, &response)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return response, storedDigest != digest, nil
}

func saveIdempotency(ctx context.Context, tx pgx.Tx, principal, route, key, digest string, response any) error {
	value, err := json.Marshal(response)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO gantry.policy_command_idempotency (principal_id, route, idempotency_key, request_digest, response_json, status_code) VALUES ($1,$2,$3,$4,$5::jsonb,200)`, principal, route, key, digest, string(value))
	return err
}

func marshalFindings(findings []map[string]any) string { return mustJSON(findings) }
func mustJSON(value any) string                        { raw, _ := json.Marshal(value); return string(raw) }
func digestJSON(value any) string {
	raw := []byte(mustJSON(value))
	digest := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(digest[:])
}
func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func newID(prefix string) string {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(value)
}

func encodeCursor(value string) string { return base64.RawURLEncoding.EncodeToString([]byte(value)) }
func decodeCursor(value string) (string, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(decoded) == 0 {
		return "", ErrInvalidInput
	}
	return string(decoded), nil
}
