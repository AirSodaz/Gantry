package agentlifecycle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/jackc/pgx/v5"
)

func (s *Service) ListNamedDrafts(ctx context.Context, actor identity.Principal, agentID string) ([]NamedDraft, error) {
	if _, err := s.getTargetAgent(ctx, actor, agentID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, agent_id, name, status, derived_from_revision_hash, latest_revision_hash,
			spec_json, schema_version, working_copy_etag, validation_status, validation_findings,
			created_by_principal_id, updated_by_principal_id, created_at, updated_at
		FROM gantry.agent_draft_workspaces
		WHERE agent_id=$1 ORDER BY CASE WHEN name='Main' THEN 0 ELSE 1 END, updated_at DESC, id`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]NamedDraft, 0)
	for rows.Next() {
		item, err := scanNamedDraft(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) GetNamedDraft(ctx context.Context, actor identity.Principal, agentID, draftID string) (NamedDraft, error) {
	if _, err := s.getTargetAgent(ctx, actor, agentID); err != nil {
		return NamedDraft{}, err
	}
	return loadNamedDraft(ctx, s.pool, agentID, draftID)
}

func (s *Service) GetMainDraft(ctx context.Context, actor identity.Principal, agentID string) (NamedDraft, error) {
	if _, err := s.getTargetAgent(ctx, actor, agentID); err != nil {
		return NamedDraft{}, err
	}
	return loadMainDraft(ctx, s.pool, agentID)
}

func (s *Service) CreateNamedDraft(ctx context.Context, actor identity.Principal, agentID string, request CreateDraftRequest) (NamedDraft, error) {
	request.Name = strings.TrimSpace(request.Name)
	request.FromRevisionHash = strings.TrimSpace(request.FromRevisionHash)
	if request.Name == "" || len(request.Name) > 96 || request.Name == "Main" {
		return NamedDraft{}, ErrInvalidInput
	}
	agent, err := s.getTargetAgent(ctx, actor, agentID)
	if err != nil {
		return NamedDraft{}, err
	}
	spec := defaultSpec()
	derivedFrom := ""
	if request.FromRevisionHash != "" {
		var revisionAgentID string
		if err := s.pool.QueryRow(ctx, `SELECT agent_id, spec_json FROM gantry.agent_revisions WHERE revision_hash=$1`, request.FromRevisionHash).Scan(&revisionAgentID, &spec); errors.Is(err, pgx.ErrNoRows) {
			return NamedDraft{}, ErrNotFound
		} else if err != nil {
			return NamedDraft{}, err
		}
		if revisionAgentID != agent.ID {
			return NamedDraft{}, ErrNotFound
		}
		derivedFrom = request.FromRevisionHash
	}
	canonical, findings := ValidateSpec(spec)
	if len(findings) != 0 {
		return NamedDraft{}, ErrInvalidState
	}
	id := newID("drf")
	findingsJSON, err := json.Marshal([]Finding{})
	if err != nil {
		return NamedDraft{}, err
	}
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO gantry.agent_draft_workspaces
		(id, agent_id, name, status, derived_from_revision_hash, spec_json, working_copy_etag, validation_status, validation_findings, created_by_principal_id, updated_by_principal_id)
		VALUES ($1,$2,$3,'active',$4,$5::jsonb,1,'valid',$6::jsonb,$7,$7)`,
		id, agent.ID, request.Name, derivedFrom, string(canonical), string(findingsJSON), actor.ID); err != nil {
		return NamedDraft{}, err
	}
	if err := s.appendTargetAudit(ctx, actor, "agent", agent.ID, "agent.draft_created", map[string]any{"draft_id": id, "name": request.Name, "derived_from_revision_hash": derivedFrom}); err != nil {
		return NamedDraft{}, err
	}
	return loadNamedDraft(ctx, s.pool, agent.ID, id)
}

func (s *Service) UpdateNamedDraft(ctx context.Context, actor identity.Principal, agentID, draftID string, expectedETag int, spec json.RawMessage) (NamedDraft, error) {
	if expectedETag < 1 || len(spec) == 0 {
		return NamedDraft{}, ErrInvalidInput
	}
	agent, err := s.getTargetAgent(ctx, actor, agentID)
	if err != nil {
		return NamedDraft{}, err
	}
	draft, err := loadNamedDraft(ctx, s.pool, agent.ID, draftID)
	if err != nil {
		return NamedDraft{}, err
	}
	if draft.Status != "active" {
		return NamedDraft{}, ErrInvalidState
	}
	canonical, findings := ValidateSpec(spec)
	stored := spec
	if len(findings) == 0 {
		assetFindings, err := validateAssetBindings(ctx, s.pool, agent.WorkspaceID, spec)
		if err != nil {
			return NamedDraft{}, err
		}
		findings = append(findings, assetFindings...)
	}
	status := "invalid"
	if len(findings) == 0 {
		stored, status = canonical, "valid"
	}
	findingsJSON, err := json.Marshal(findings)
	if err != nil {
		return NamedDraft{}, err
	}
	command, err := s.pool.Exec(ctx, `
		UPDATE gantry.agent_draft_workspaces
		SET working_copy_etag=working_copy_etag+1, spec_json=$3::jsonb,
			validation_status=$4, validation_findings=$5::jsonb,
			updated_by_principal_id=$6, updated_at=now()
		WHERE id=$1 AND agent_id=$2 AND working_copy_etag=$7`,
		draftID, agent.ID, string(stored), status, string(findingsJSON), actor.ID, expectedETag)
	if err != nil {
		return NamedDraft{}, err
	}
	if command.RowsAffected() == 0 {
		return NamedDraft{}, ErrRevisionConflict
	}
	if _, err := s.pool.Exec(ctx, `UPDATE gantry.agents SET updated_at=now() WHERE id=$1`, agent.ID); err != nil {
		return NamedDraft{}, err
	}
	if err := s.appendTargetAudit(ctx, actor, "agent", agent.ID, "agent.draft_updated", map[string]any{"draft_id": draftID, "working_copy_etag": expectedETag + 1}); err != nil {
		return NamedDraft{}, err
	}
	return loadNamedDraft(ctx, s.pool, agent.ID, draftID)
}

func (s *Service) ArchiveNamedDraft(ctx context.Context, actor identity.Principal, agentID, draftID string) error {
	if _, err := s.getTargetAgent(ctx, actor, agentID); err != nil {
		return err
	}
	command, err := s.pool.Exec(ctx, `UPDATE gantry.agent_draft_workspaces SET status='archived', updated_by_principal_id=$3, updated_at=now() WHERE id=$1 AND agent_id=$2 AND status='active' AND name <> 'Main'`, draftID, agentID, actor.ID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrInvalidState
	}
	return s.appendTargetAudit(ctx, actor, "agent", agentID, "agent.draft_archived", map[string]any{"draft_id": draftID})
}

func (s *Service) CommitNamedDraft(ctx context.Context, actor identity.Principal, agentID, draftID string, request CommitDraftRequest) (Revision, error) {
	request.Message = strings.TrimSpace(request.Message)
	if request.Message == "" || len(request.Message) > 4000 {
		return Revision{}, ErrInvalidInput
	}
	agent, err := s.getTargetAgent(ctx, actor, agentID)
	if err != nil {
		return Revision{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Revision{}, err
	}
	defer tx.Rollback(ctx)
	draft, err := loadNamedDraft(ctx, tx, agent.ID, draftID)
	if err != nil {
		return Revision{}, err
	}
	if draft.Status != "active" || draft.ValidationStatus != "valid" {
		return Revision{}, ErrInvalidState
	}
	canonical, specDigest, err := canonicalDigest(draft.Spec)
	if err != nil {
		return Revision{}, err
	}
	assetFindings, err := validateAssetBindings(ctx, s.pool, agent.WorkspaceID, canonical)
	if err != nil {
		return Revision{}, err
	}
	if len(assetFindings) != 0 {
		return Revision{}, ErrInvalidState
	}
	prompt, err := CompilePromptSnapshot(canonical)
	if err != nil {
		return Revision{}, err
	}
	createdAt := time.Now().UTC()
	revisionID := newID("arv")
	revisionHash, err := revisionHash(agent.ID, draft.ID, request.Message, actor.ID, createdAt, specDigest)
	if err != nil {
		return Revision{}, err
	}
	promptJSON, err := json.Marshal(prompt)
	if err != nil {
		return Revision{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO gantry.agent_revisions
		(id, agent_id, revision_hash, source_draft_id, message, spec_json, spec_digest, created_by_principal_id, created_at, prompt_snapshot_json, prompt_snapshot_digest, prompt_compiler_version)
		VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7,$8,$9,$10::jsonb,$11,$12)`,
		revisionID, agent.ID, revisionHash, draft.ID, request.Message, string(canonical), specDigest, actor.ID, createdAt, string(promptJSON), prompt.ContentDigest, prompt.CompilerVersion); err != nil {
		return Revision{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.agent_draft_workspaces SET latest_revision_hash=$2, updated_at=now() WHERE id=$1`, draft.ID, revisionHash); err != nil {
		return Revision{}, err
	}
	if err := appendAudit(ctx, tx, actor.OrganizationID, actor.ID, "agent", agent.ID, "agent.revision_committed", map[string]any{"draft_id": draft.ID, "revision_hash": revisionHash, "spec_digest": specDigest}); err != nil {
		return Revision{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Revision{}, err
	}
	return s.GetRevision(ctx, actor, agent.ID, revisionHash)
}

func (s *Service) ListRevisions(ctx context.Context, actor identity.Principal, agentID string) ([]Revision, error) {
	if _, err := s.getTargetAgent(ctx, actor, agentID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, revisionSelect+` WHERE r.agent_id=$1 ORDER BY r.created_at DESC, r.id DESC`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Revision, 0)
	for rows.Next() {
		item, err := scanRevision(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) GetRevision(ctx context.Context, actor identity.Principal, agentID, hash string) (Revision, error) {
	if _, err := s.getTargetAgent(ctx, actor, agentID); err != nil {
		return Revision{}, err
	}
	item, err := scanRevision(s.pool.QueryRow(ctx, revisionSelect+` WHERE r.agent_id=$1 AND r.revision_hash=$2`, agentID, strings.TrimSpace(hash)))
	if errors.Is(err, pgx.ErrNoRows) {
		return Revision{}, ErrNotFound
	}
	return item, err
}

func (s *Service) GetTargetOverview(ctx context.Context, actor identity.Principal, agentID string) (AgentTargetOverview, error) {
	agent, err := s.getTargetAgent(ctx, actor, agentID)
	if err != nil {
		return AgentTargetOverview{}, err
	}
	drafts, err := s.ListNamedDrafts(ctx, actor, agent.ID)
	if err != nil {
		return AgentTargetOverview{}, err
	}
	main, err := loadMainDraft(ctx, s.pool, agent.ID)
	if err != nil {
		return AgentTargetOverview{}, err
	}
	deployments, err := s.ListDeployments(ctx, actor, agent.ID)
	if err != nil {
		return AgentTargetOverview{}, err
	}
	overview := AgentTargetOverview{Agent: agent, MainDraft: main, Drafts: drafts, TestDeployments: make([]Deployment, 0), RecentActivity: make([]ActivityItem, 0)}
	for _, deployment := range deployments {
		if deployment.EnvironmentKind == "production" && deployment.Status == "active" {
			copy := deployment
			overview.ProductionDeployment = &copy
		} else if deployment.EnvironmentKind == "test" {
			overview.TestDeployments = append(overview.TestDeployments, deployment)
		}
	}
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM gantry.agent_revisions WHERE agent_id=$1`, agent.ID).Scan(&overview.RevisionCount); err != nil {
		return AgentTargetOverview{}, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id, event_type, payload, created_at FROM gantry.audit_events WHERE resource_type IN ('agent', 'agent_revision', 'agent_revision_review') AND (resource_id=$1 OR payload->>'agent_id'=$1) ORDER BY created_at DESC, id DESC LIMIT 12`, agent.ID)
	if err != nil {
		return AgentTargetOverview{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var item ActivityItem
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.EventType, &item.Payload, &createdAt); err != nil {
			return AgentTargetOverview{}, err
		}
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		overview.RecentActivity = append(overview.RecentActivity, item)
	}
	return overview, rows.Err()
}

const revisionSelect = `
	SELECT r.id, r.agent_id, r.revision_hash, r.source_draft_id, r.message, r.spec_json, r.spec_digest,
		r.runtime_image_digest, r.created_at, COALESCE(p.display_name, ''), r.prompt_snapshot_json
	FROM gantry.agent_revisions r
	JOIN gantry.principals p ON p.id=r.created_by_principal_id`

func revisionHash(agentID, draftID, message, actorID string, createdAt time.Time, specDigest string) (string, error) {
	envelope, err := json.Marshal(struct {
		AgentID    string `json:"agent_id"`
		DraftID    string `json:"source_draft_id"`
		Message    string `json:"message"`
		AuthorID   string `json:"author_id"`
		CreatedAt  string `json:"created_at"`
		SpecDigest string `json:"spec_digest"`
	}{agentID, draftID, message, actorID, createdAt.Format(time.RFC3339Nano), specDigest})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(envelope)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func (s *Service) getTargetAgent(ctx context.Context, actor identity.Principal, agentID string) (Agent, error) {
	var agent Agent
	err := s.pool.QueryRow(ctx, `
		SELECT a.id, a.organization_id, a.workspace_id, a.slug, a.display_name, a.description, a.category,
			COALESCE(production.status, 'draft'), COALESCE(production.revision_hash, '')
		FROM gantry.agents a
		LEFT JOIN gantry.agent_deployments production ON production.agent_id=a.id AND production.environment_kind='production' AND production.status='active'
		WHERE a.id=$1`, agentID).Scan(&agent.ID, &agent.OrganizationID, &agent.WorkspaceID, &agent.Slug, &agent.DisplayName, &agent.Description, &agent.Category, &agent.LifecycleStatus, &agent.CurrentProductionRevisionHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return Agent{}, ErrNotFound
	}
	if err != nil {
		return Agent{}, err
	}
	if err := s.authz.RequireWorkspace(ctx, actor, agent.WorkspaceID); err != nil {
		return Agent{}, err
	}
	return agent, nil
}

func (s *Service) appendTargetAudit(ctx context.Context, actor identity.Principal, resourceType, resourceID, eventType string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `INSERT INTO gantry.audit_events (organization_id, actor_principal_id, resource_type, resource_id, event_type, payload) VALUES ($1,$2,$3,$4,$5,$6::jsonb)`, actor.OrganizationID, actor.ID, resourceType, resourceID, eventType, string(data))
	return err
}

type rowScanner interface{ Scan(...any) error }

func scanNamedDraft(row rowScanner) (NamedDraft, error) {
	var item NamedDraft
	var findings []byte
	var createdAt, updatedAt time.Time
	err := row.Scan(&item.ID, &item.AgentID, &item.Name, &item.Status, &item.DerivedFromRevisionHash, &item.LatestRevisionHash, &item.Spec, &item.SchemaVersion, &item.WorkingCopyETag, &item.ValidationStatus, &findings, &item.CreatedBy, &item.UpdatedBy, &createdAt, &updatedAt)
	if err != nil {
		return NamedDraft{}, err
	}
	if err := json.Unmarshal(findings, &item.ValidationFindings); err != nil {
		return NamedDraft{}, err
	}
	item.CreatedAt, item.UpdatedAt = createdAt.UTC().Format(time.RFC3339), updatedAt.UTC().Format(time.RFC3339)
	return item, nil
}

func loadNamedDraft(ctx context.Context, querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, agentID, draftID string) (NamedDraft, error) {
	item, err := scanNamedDraft(querier.QueryRow(ctx, `
		SELECT id, agent_id, name, status, derived_from_revision_hash, latest_revision_hash,
			spec_json, schema_version, working_copy_etag, validation_status, validation_findings,
			created_by_principal_id, updated_by_principal_id, created_at, updated_at
		FROM gantry.agent_draft_workspaces WHERE agent_id=$1 AND id=$2`, agentID, draftID))
	if errors.Is(err, pgx.ErrNoRows) {
		return NamedDraft{}, ErrNotFound
	}
	return item, err
}

func loadMainDraft(ctx context.Context, querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, agentID string) (NamedDraft, error) {
	item, err := scanNamedDraft(querier.QueryRow(ctx, `
		SELECT id, agent_id, name, status, derived_from_revision_hash, latest_revision_hash,
			spec_json, schema_version, working_copy_etag, validation_status, validation_findings,
			created_by_principal_id, updated_by_principal_id, created_at, updated_at
		FROM gantry.agent_draft_workspaces WHERE agent_id=$1 AND name='Main'`, agentID))
	if errors.Is(err, pgx.ErrNoRows) {
		return NamedDraft{}, ErrNotFound
	}
	return item, err
}

func scanRevision(row rowScanner) (Revision, error) {
	var item Revision
	var createdAt time.Time
	var prompt []byte
	err := row.Scan(&item.ID, &item.AgentID, &item.RevisionHash, &item.SourceDraftID, &item.Message, &item.Spec, &item.SpecDigest, &item.RuntimeImageDigest, &createdAt, &item.CreatedBy, &prompt)
	if err != nil {
		return Revision{}, err
	}
	if err := json.Unmarshal(prompt, &item.PromptSnapshot); err != nil {
		return Revision{}, err
	}
	item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	return item, nil
}

func (s *Service) GetRevisionReview(ctx context.Context, actor identity.Principal, agentID, revisionHash string) (RevisionReview, error) {
	if _, err := s.GetRevision(ctx, actor, agentID, revisionHash); err != nil {
		return RevisionReview{}, err
	}
	return loadRevisionReview(ctx, s.pool, agentID, revisionHash)
}

func (s *Service) SubmitRevisionReview(ctx context.Context, actor identity.Principal, agentID, revisionHash, releaseNotes string) (RevisionReview, error) {
	revision, err := s.GetRevision(ctx, actor, agentID, revisionHash)
	if err != nil {
		return RevisionReview{}, err
	}
	releaseNotes = strings.TrimSpace(releaseNotes)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return RevisionReview{}, err
	}
	defer tx.Rollback(ctx)
	base, err := loadProductionRevision(ctx, tx, agentID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return RevisionReview{}, err
	}
	var baseSpec json.RawMessage
	if err == nil {
		baseSpec = base.Spec
	}
	diff, summary, err := buildDiff(baseSpec, revision.Spec)
	if err != nil {
		return RevisionReview{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO gantry.agent_revision_reviews
		(id, agent_id, revision_id, revision_hash, base_revision_hash, release_notes, diff_json, risk_summary, status, submitted_by_principal_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8::jsonb,'pending',$9)
		ON CONFLICT (revision_id) DO UPDATE SET base_revision_hash=EXCLUDED.base_revision_hash, release_notes=EXCLUDED.release_notes, diff_json=EXCLUDED.diff_json, risk_summary=EXCLUDED.risk_summary, status='pending', submitted_by_principal_id=EXCLUDED.submitted_by_principal_id, reviewed_by_principal_id=NULL, review_reason='', submitted_at=now(), reviewed_at=NULL`,
		newID("rrv"), agentID, revision.ID, revision.RevisionHash, base.RevisionHash, releaseNotes, string(mustJSON(diff)), string(mustJSON(summary)), actor.ID); err != nil {
		return RevisionReview{}, err
	}
	if err := appendAudit(ctx, tx, actor.OrganizationID, actor.ID, "agent_revision", revision.ID, "agent.revision_review_submitted", map[string]any{"revision_hash": revision.RevisionHash}); err != nil {
		return RevisionReview{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return RevisionReview{}, err
	}
	return s.GetRevisionReview(ctx, actor, agentID, revisionHash)
}

func (s *Service) DecideRevisionReview(ctx context.Context, actor identity.Principal, agentID, revisionHash, decision, reason string) (RevisionReview, error) {
	if decision != "approve" && decision != "reject" {
		return RevisionReview{}, ErrInvalidInput
	}
	if _, err := s.GetRevision(ctx, actor, agentID, revisionHash); err != nil {
		return RevisionReview{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return RevisionReview{}, err
	}
	defer tx.Rollback(ctx)
	var reviewID, status string
	err = tx.QueryRow(ctx, `SELECT id, status FROM gantry.agent_revision_reviews WHERE agent_id=$1 AND revision_hash=$2 FOR UPDATE`, agentID, revisionHash).Scan(&reviewID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return RevisionReview{}, ErrReviewRequired
	}
	if err != nil {
		return RevisionReview{}, err
	}
	if status != "pending" {
		return RevisionReview{}, ErrInvalidState
	}
	if _, err := tx.Exec(ctx, `UPDATE gantry.agent_revision_reviews SET status=$2, reviewed_by_principal_id=$3, review_reason=$4, reviewed_at=now() WHERE id=$1`, reviewID, decision+"d", actor.ID, strings.TrimSpace(reason)); err != nil {
		return RevisionReview{}, err
	}
	if err := appendAudit(ctx, tx, actor.OrganizationID, actor.ID, "agent_revision_review", reviewID, "agent.revision_review_"+decision+"d", map[string]any{"revision_hash": revisionHash, "reason": strings.TrimSpace(reason)}); err != nil {
		return RevisionReview{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return RevisionReview{}, err
	}
	return s.GetRevisionReview(ctx, actor, agentID, revisionHash)
}

func (s *Service) ListDeployments(ctx context.Context, actor identity.Principal, agentID string) ([]Deployment, error) {
	if _, err := s.getTargetAgent(ctx, actor, agentID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, deploymentSelect+` WHERE d.agent_id=$1 ORDER BY d.environment_kind, d.updated_at DESC, d.id`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Deployment, 0)
	for rows.Next() {
		item, err := scanDeployment(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) CreateTestDeployment(ctx context.Context, actor identity.Principal, agentID string, request CreateDeploymentRequest) (Deployment, error) {
	request.Name = strings.TrimSpace(request.Name)
	request.RevisionHash = strings.TrimSpace(request.RevisionHash)
	request.Purpose = strings.TrimSpace(request.Purpose)
	if request.Name == "" || request.Name == "Production" || request.RevisionHash == "" || len(request.Name) > 96 {
		return Deployment{}, ErrInvalidInput
	}
	agent, err := s.getTargetAgent(ctx, actor, agentID)
	if err != nil {
		return Deployment{}, err
	}
	revision, err := s.GetRevision(ctx, actor, agent.ID, request.RevisionHash)
	if err != nil {
		return Deployment{}, err
	}
	policy := request.EnvironmentPolicy
	if len(policy) == 0 {
		policy = json.RawMessage(`{}`)
	}
	var policyObject map[string]any
	if err := json.Unmarshal(policy, &policyObject); err != nil || policyObject == nil {
		return Deployment{}, ErrInvalidInput
	}
	var expiresAt *time.Time
	if request.ExpiresAt != "" {
		value, err := time.Parse(time.RFC3339, request.ExpiresAt)
		if err != nil || !value.After(time.Now()) {
			return Deployment{}, ErrInvalidInput
		}
		expiresAt = &value
	}
	id := newID("dpl")
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO gantry.agent_deployments
		(id, agent_id, workspace_id, name, environment_kind, revision_id, revision_hash, spec_digest, status, owner_principal_id, purpose, expires_at, environment_policy, changed_by_principal_id)
		VALUES ($1,$2,$3,$4,'test',$5,$6,$7,'active',$8,$9,$10,$11::jsonb,$8)`,
		id, agent.ID, agent.WorkspaceID, request.Name, revision.ID, revision.RevisionHash, revision.SpecDigest, actor.ID, request.Purpose, expiresAt, string(policy)); err != nil {
		return Deployment{}, err
	}
	if err := s.appendTargetAudit(ctx, actor, "agent", agent.ID, "agent.test_deployment_created", map[string]any{"deployment_id": id, "revision_hash": revision.RevisionHash}); err != nil {
		return Deployment{}, err
	}
	return loadDeployment(ctx, s.pool, agent.ID, id)
}

func (s *Service) PublishRevision(ctx context.Context, actor identity.Principal, agentID string, request PublishRevisionRequest) (Deployment, error) {
	request.RevisionHash = strings.TrimSpace(request.RevisionHash)
	request.ExpectedProductionRevisionHash = strings.TrimSpace(request.ExpectedProductionRevisionHash)
	if request.RevisionHash == "" {
		return Deployment{}, ErrInvalidInput
	}
	agent, err := s.getTargetAgent(ctx, actor, agentID)
	if err != nil {
		return Deployment{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Deployment{}, err
	}
	defer tx.Rollback(ctx)
	revision, err := loadRevisionByHash(ctx, tx, agent.ID, request.RevisionHash)
	if err != nil {
		return Deployment{}, err
	}
	var reviewID, reviewStatus string
	err = tx.QueryRow(ctx, `SELECT id, status FROM gantry.agent_revision_reviews WHERE revision_id=$1`, revision.ID).Scan(&reviewID, &reviewStatus)
	if errors.Is(err, pgx.ErrNoRows) || reviewStatus != "approved" {
		return Deployment{}, ErrReviewRequired
	}
	if err != nil {
		return Deployment{}, err
	}
	current, currentErr := loadProductionDeployment(ctx, tx, agent.ID)
	if currentErr != nil && !errors.Is(currentErr, ErrNotFound) {
		return Deployment{}, currentErr
	}
	if request.ExpectedProductionRevisionHash != "" && (errors.Is(currentErr, ErrNotFound) || current.RevisionHash != request.ExpectedProductionRevisionHash) {
		return Deployment{}, ErrRevisionConflict
	}
	if currentErr == nil && current.RevisionHash == revision.RevisionHash {
		return current, tx.Commit(ctx)
	}
	previousHash := ""
	if currentErr == nil {
		previousHash = current.RevisionHash
		if _, err := tx.Exec(ctx, `UPDATE gantry.agent_deployments SET revision_id=$2, revision_hash=$3, spec_digest=$4, review_id=$5, previous_revision_hash=$6, changed_by_principal_id=$7, updated_at=now() WHERE id=$1`, current.ID, revision.ID, revision.RevisionHash, revision.SpecDigest, reviewID, previousHash, actor.ID); err != nil {
			return Deployment{}, err
		}
	} else if _, err := tx.Exec(ctx, `
		INSERT INTO gantry.agent_deployments
		(id, agent_id, workspace_id, name, environment_kind, revision_id, revision_hash, spec_digest, status, owner_principal_id, changed_by_principal_id, review_id)
		VALUES ($1,$2,$3,'Production','production',$4,$5,$6,'active',$7,$7,$8)`, newID("dpl"), agent.ID, agent.WorkspaceID, revision.ID, revision.RevisionHash, revision.SpecDigest, actor.ID, reviewID); err != nil {
		return Deployment{}, err
	}
	if err := appendAudit(ctx, tx, actor.OrganizationID, actor.ID, "agent", agent.ID, "agent.production_deployment_moved", map[string]any{"from_revision_hash": previousHash, "to_revision_hash": revision.RevisionHash}); err != nil {
		return Deployment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Deployment{}, err
	}
	return loadProductionDeployment(ctx, s.pool, agent.ID)
}

func (s *Service) StopTestDeployment(ctx context.Context, actor identity.Principal, agentID, deploymentID string) error {
	if _, err := s.getTargetAgent(ctx, actor, agentID); err != nil {
		return err
	}
	command, err := s.pool.Exec(ctx, `UPDATE gantry.agent_deployments SET status='stopped', changed_by_principal_id=$3, updated_at=now() WHERE id=$1 AND agent_id=$2 AND environment_kind='test' AND status='active'`, deploymentID, agentID, actor.ID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrInvalidState
	}
	return s.appendTargetAudit(ctx, actor, "agent", agentID, "agent.test_deployment_stopped", map[string]any{"deployment_id": deploymentID})
}

func loadRevisionReview(ctx context.Context, querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, agentID, revisionHash string) (RevisionReview, error) {
	var item RevisionReview
	var diff, summary []byte
	var submittedAt time.Time
	var reviewedAt *time.Time
	err := querier.QueryRow(ctx, `SELECT id, agent_id, revision_hash, base_revision_hash, release_notes, diff_json, risk_summary, status, submitted_by_principal_id, COALESCE(reviewed_by_principal_id,''), review_reason, submitted_at, reviewed_at FROM gantry.agent_revision_reviews WHERE agent_id=$1 AND revision_hash=$2`, agentID, revisionHash).Scan(&item.ID, &item.AgentID, &item.RevisionHash, &item.BaseRevisionHash, &item.ReleaseNotes, &diff, &summary, &item.Status, &item.SubmittedBy, &item.ReviewedBy, &item.ReviewReason, &submittedAt, &reviewedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return RevisionReview{AgentID: agentID, RevisionHash: revisionHash, Diff: make([]DiffEntry, 0), Status: "not_submitted"}, nil
	}
	if err != nil {
		return RevisionReview{}, err
	}
	if err := json.Unmarshal(diff, &item.Diff); err != nil {
		return RevisionReview{}, err
	}
	if err := json.Unmarshal(summary, &item.RiskSummary); err != nil {
		return RevisionReview{}, err
	}
	item.SubmittedAt = submittedAt.UTC().Format(time.RFC3339)
	if reviewedAt != nil {
		item.ReviewedAt = reviewedAt.UTC().Format(time.RFC3339)
	}
	return item, nil
}

const deploymentSelect = `
	SELECT d.id, d.agent_id, d.workspace_id, d.name, d.environment_kind, d.revision_id, d.revision_hash, d.spec_digest,
		d.status, COALESCE(owner.display_name,''), d.purpose, d.expires_at, d.environment_policy, COALESCE(changer.display_name,''),
		COALESCE(d.review_id,''), d.previous_revision_hash, d.created_at, d.updated_at
	FROM gantry.agent_deployments d
	LEFT JOIN gantry.principals owner ON owner.id=d.owner_principal_id
	JOIN gantry.principals changer ON changer.id=d.changed_by_principal_id`

func scanDeployment(row rowScanner) (Deployment, error) {
	var item Deployment
	var expiresAt *time.Time
	var createdAt, updatedAt time.Time
	err := row.Scan(&item.ID, &item.AgentID, &item.WorkspaceID, &item.Name, &item.EnvironmentKind, &item.RevisionID, &item.RevisionHash, &item.SpecDigest, &item.Status, &item.Owner, &item.Purpose, &expiresAt, &item.EnvironmentPolicy, &item.ChangedBy, &item.ReviewID, &item.PreviousRevisionHash, &createdAt, &updatedAt)
	if err != nil {
		return Deployment{}, err
	}
	if expiresAt != nil {
		item.ExpiresAt = expiresAt.UTC().Format(time.RFC3339)
	}
	item.CreatedAt, item.UpdatedAt = createdAt.UTC().Format(time.RFC3339), updatedAt.UTC().Format(time.RFC3339)
	return item, nil
}

func loadDeployment(ctx context.Context, querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, agentID, deploymentID string) (Deployment, error) {
	item, err := scanDeployment(querier.QueryRow(ctx, deploymentSelect+` WHERE d.agent_id=$1 AND d.id=$2`, agentID, deploymentID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Deployment{}, ErrNotFound
	}
	return item, err
}

func loadProductionDeployment(ctx context.Context, querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, agentID string) (Deployment, error) {
	item, err := scanDeployment(querier.QueryRow(ctx, deploymentSelect+` WHERE d.agent_id=$1 AND d.environment_kind='production' AND d.status='active'`, agentID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Deployment{}, ErrNotFound
	}
	return item, err
}

func loadProductionRevision(ctx context.Context, querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, agentID string) (Revision, error) {
	item, err := scanRevision(querier.QueryRow(ctx, revisionSelect+` JOIN gantry.agent_deployments d ON d.revision_id=r.id WHERE d.agent_id=$1 AND d.environment_kind='production' AND d.status='active'`, agentID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Revision{}, ErrNotFound
	}
	return item, err
}

func loadRevisionByHash(ctx context.Context, querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, agentID, hash string) (Revision, error) {
	item, err := scanRevision(querier.QueryRow(ctx, revisionSelect+` WHERE r.agent_id=$1 AND r.revision_hash=$2`, agentID, hash))
	if errors.Is(err, pgx.ErrNoRows) {
		return Revision{}, ErrNotFound
	}
	return item, err
}
