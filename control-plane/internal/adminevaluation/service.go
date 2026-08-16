package adminevaluation

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/AirSodaz/gantry/internal/authorization"
	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound            = errors.New("admin evaluation resource not found")
	ErrInvalidInput        = errors.New("invalid admin evaluation input")
	ErrInvalidState        = errors.New("invalid admin evaluation state")
	ErrETagConflict        = errors.New("admin evaluation etag conflict")
	ErrIdempotencyConflict = errors.New("admin evaluation idempotency conflict")
	ErrFixtureInvalid      = errors.New("evaluation fixture is invalid")
)

type Service struct {
	pool  *pgxpool.Pool
	authz *authorization.Service
}

func NewService(pool *pgxpool.Pool, authz *authorization.Service) *Service {
	return &Service{pool: pool, authz: authz}
}

func (s *Service) ListSuites(ctx context.Context, actor identity.Principal, options ListOptions) (SuiteList, error) {
	options = normalizeListOptions(options)
	if options.WorkspaceID != "" {
		if err := s.authz.RequireWorkspace(ctx, actor, options.WorkspaceID); err != nil {
			return SuiteList{}, err
		}
	}
	args := []any{actor.OrganizationID, actor.ID, options.WorkspaceID, options.State, options.Search, options.Limit + 1}
	rows, err := s.pool.Query(ctx, `SELECT id, organization_id, workspace_id, name, state, owner_principal_id, latest_version_id, etag::text, (SELECT count(*) FROM gantry.evaluation_suite_versions v WHERE v.suite_id=s.id) FROM gantry.evaluation_suites s WHERE organization_id=$1 AND ($3='' OR workspace_id=$3) AND ($4='' OR state=$4) AND ($5='' OR name ILIKE '%' || $5 || '%') AND (EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$2 AND rb.role='organization_admin' AND rb.workspace_id IS NULL) OR EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$2 AND rb.role='workspace_agent_editor' AND rb.workspace_id=workspace_id)) ORDER BY name, id LIMIT $6`, args...)
	if err != nil {
		return SuiteList{}, err
	}
	defer rows.Close()
	items := make([]Suite, 0, options.Limit)
	for rows.Next() {
		item, err := scanSuite(rows)
		if err != nil {
			return SuiteList{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return SuiteList{}, err
	}
	result := SuiteList{Items: items, PageInfo: PageInfo{}}
	if len(items) > options.Limit {
		result.Items = items[:options.Limit]
		cursor := encodeCursor(result.Items[len(result.Items)-1].ID)
		result.PageInfo.NextCursor = &cursor
	}
	return result, nil
}

func (s *Service) CreateSuite(ctx context.Context, actor identity.Principal, request CreateSuiteRequest) (Suite, error) {
	request.WorkspaceID, request.Name = strings.TrimSpace(request.WorkspaceID), strings.TrimSpace(request.Name)
	if request.WorkspaceID == "" || request.Name == "" {
		return Suite{}, ErrInvalidInput
	}
	if err := s.authz.RequireWorkspace(ctx, actor, request.WorkspaceID); err != nil {
		return Suite{}, err
	}
	item := Suite{ID: newID("esuite"), OrganizationID: actor.OrganizationID, WorkspaceID: request.WorkspaceID, Name: request.Name, State: "draft", OwnerPrincipalID: actor.ID, ETag: "1"}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Suite{}, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `INSERT INTO gantry.evaluation_suites (id, organization_id, workspace_id, name, state, owner_principal_id, etag) VALUES ($1,$2,$3,$4,'draft',$5,1) RETURNING id`, item.ID, item.OrganizationID, item.WorkspaceID, item.Name, actor.ID).Scan(&item.ID)
	if err != nil {
		return Suite{}, err
	}
	if err := appendAudit(ctx, tx, actor, item.ID, item.WorkspaceID, "evaluation_suite.created", map[string]any{"workspace_id": item.WorkspaceID, "outcome": "success", "risk": "low"}); err != nil {
		return Suite{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Suite{}, err
	}
	return item, nil
}

func (s *Service) GetSuite(ctx context.Context, actor identity.Principal, id string) (Suite, error) {
	return s.getSuite(ctx, actor, id)
}

func (s *Service) PatchSuite(ctx context.Context, actor identity.Principal, id, expectedETag string, request PatchSuiteRequest) (Suite, error) {
	item, err := s.getSuite(ctx, actor, id)
	if err != nil {
		return Suite{}, err
	}
	if expectedETag == "" {
		return Suite{}, ErrETagConflict
	}
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" {
		return Suite{}, ErrInvalidInput
	}
	if err := s.authz.RequireWorkspace(ctx, actor, item.WorkspaceID); err != nil {
		return Suite{}, err
	}
	var updated Suite
	err = s.pool.QueryRow(ctx, `UPDATE gantry.evaluation_suites SET name=$3, etag=etag+1, updated_at=now() WHERE id=$1 AND etag::text=$2 RETURNING id, organization_id, workspace_id, name, state, owner_principal_id, latest_version_id, etag::text, 0`, id, strings.Trim(expectedETag, `"`), request.Name).Scan(&updated.ID, &updated.OrganizationID, &updated.WorkspaceID, &updated.Name, &updated.State, &updated.OwnerPrincipalID, &updated.LatestVersionID, &updated.ETag, &updated.GateUsageCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return Suite{}, ErrETagConflict
	}
	if err != nil {
		return Suite{}, err
	}
	return updated, nil
}

func (s *Service) ListCases(ctx context.Context, actor identity.Principal, suiteID string) ([]Case, error) {
	if _, err := s.getSuite(ctx, actor, suiteID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id, suite_id, input_json, fixture_manifest_json, assertions_json, rubric_json, compatibility_json, etag::text FROM gantry.evaluation_cases WHERE suite_id=$1 ORDER BY id`, suiteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Case, 0)
	for rows.Next() {
		item, err := scanCase(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) CreateCase(ctx context.Context, actor identity.Principal, suiteID string, request CreateCaseRequest) (Case, error) {
	suite, err := s.getSuite(ctx, actor, suiteID)
	if err != nil {
		return Case{}, err
	}
	if suite.State != "draft" {
		return Case{}, ErrInvalidState
	}
	if err := s.authz.RequireWorkspace(ctx, actor, suite.WorkspaceID); err != nil {
		return Case{}, err
	}
	item, err := normalizeCase(request)
	if err != nil {
		return Case{}, err
	}
	item.ID, item.SuiteID, item.ETag = newID("ecase"), suiteID, "1"
	_, err = s.pool.Exec(ctx, `INSERT INTO gantry.evaluation_cases (id, suite_id, input_json, fixture_manifest_json, assertions_json, rubric_json, compatibility_json, etag) VALUES ($1,$2,$3::jsonb,$4::jsonb,$5::jsonb,$6::jsonb,$7::jsonb,1)`, item.ID, suiteID, string(item.Input), string(item.FixtureManifest), string(item.Assertions), nullableJSON(item.Rubric), string(item.Compatibility))
	if err != nil {
		return Case{}, err
	}
	return item, nil
}

func (s *Service) PatchCase(ctx context.Context, actor identity.Principal, suiteID, caseID, expectedETag string, request PatchCaseRequest) (Case, error) {
	suite, err := s.getSuite(ctx, actor, suiteID)
	if err != nil {
		return Case{}, err
	}
	if suite.State != "draft" {
		return Case{}, ErrInvalidState
	}
	if err := s.authz.RequireWorkspace(ctx, actor, suite.WorkspaceID); err != nil {
		return Case{}, err
	}
	item, err := normalizeCase(request)
	if err != nil {
		return Case{}, err
	}
	expectedETag = strings.Trim(expectedETag, `"`)
	var result Case
	err = s.pool.QueryRow(ctx, `UPDATE gantry.evaluation_cases SET input_json=$4::jsonb, fixture_manifest_json=$5::jsonb, assertions_json=$6::jsonb, rubric_json=$7::jsonb, compatibility_json=$8::jsonb, etag=etag+1, updated_at=now() WHERE id=$1 AND suite_id=$2 AND etag::text=$3 RETURNING id, suite_id, input_json, fixture_manifest_json, assertions_json, rubric_json, compatibility_json, etag::text`, caseID, suiteID, expectedETag, string(item.Input), string(item.FixtureManifest), string(item.Assertions), nullableJSON(item.Rubric), string(item.Compatibility)).Scan(&result.ID, &result.SuiteID, &result.Input, &result.FixtureManifest, &result.Assertions, &result.Rubric, &result.Compatibility, &result.ETag)
	if errors.Is(err, pgx.ErrNoRows) {
		return Case{}, ErrETagConflict
	}
	if err != nil {
		return Case{}, err
	}
	return result, nil
}

func (s *Service) ValidateSuite(ctx context.Context, actor identity.Principal, suiteID string) (Validation, error) {
	if _, err := s.getSuite(ctx, actor, suiteID); err != nil {
		return Validation{}, err
	}
	cases, err := s.ListCases(ctx, actor, suiteID)
	if err != nil {
		return Validation{}, err
	}
	findings := make([]map[string]any, 0)
	for _, item := range cases {
		if len(item.FixtureManifest) == 0 || string(item.FixtureManifest) == "null" {
			findings = append(findings, map[string]any{"case_id": item.ID, "code": "fixture_manifest_required"})
		}
	}
	if len(cases) == 0 {
		findings = append(findings, map[string]any{"code": "case_required", "message": "at least one case is required"})
	}
	state := "valid"
	if len(findings) > 0 {
		state = "invalid"
	}
	return Validation{State: state, Findings: findings}, nil
}

func (s *Service) ListVersions(ctx context.Context, actor identity.Principal, suiteID string) ([]Version, error) {
	if _, err := s.getSuite(ctx, actor, suiteID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id, suite_id, content_digest, case_manifest_digest, fixture_manifest_digest, evaluator_policy_version_id, runtime_image_digest, published_at FROM gantry.evaluation_suite_versions WHERE suite_id=$1 ORDER BY published_at, id`, suiteID)
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

func (s *Service) PublishVersion(ctx context.Context, actor identity.Principal, suiteID, expectedETag, key string, request PublishVersionRequest) (Version, error) {
	suite, err := s.getSuite(ctx, actor, suiteID)
	if err != nil {
		return Version{}, err
	}
	if suite.State != "draft" {
		return Version{}, ErrInvalidState
	}
	if strings.TrimSpace(key) == "" {
		return Version{}, ErrInvalidInput
	}
	validation, err := s.ValidateSuite(ctx, actor, suiteID)
	if err != nil {
		return Version{}, err
	}
	if validation.State != "valid" {
		return Version{}, ErrFixtureInvalid
	}
	if strings.TrimSpace(request.RuntimeImageDigest) == "" {
		return Version{}, ErrInvalidInput
	}
	cases, err := s.ListCases(ctx, actor, suiteID)
	if err != nil {
		return Version{}, err
	}
	contentDigest := digestJSON(cases)
	caseDigest := digestJSON(caseManifest(cases))
	fixtureDigest := digestJSON(fixtureManifest(cases))
	version := Version{ID: newID("esver"), SuiteID: suiteID, ContentDigest: contentDigest, CaseManifestDigest: caseDigest, FixtureManifestDigest: fixtureDigest, EvaluatorPolicyVersionID: strings.TrimSpace(request.EvaluatorPolicyVersionID), RuntimeImageDigest: strings.TrimSpace(request.RuntimeImageDigest), PublishedAt: time.Now().UTC().Format(time.RFC3339)}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Version{}, err
	}
	defer tx.Rollback(ctx)
	digest := digestJSON(map[string]any{"etag": expectedETag, "content_digest": contentDigest, "runtime_image_digest": version.RuntimeImageDigest})
	if cached, conflict, err := loadIdempotency(ctx, tx, actor.ID, "publish:"+suiteID, key, digest); err != nil {
		return Version{}, err
	} else if conflict {
		return Version{}, ErrIdempotencyConflict
	} else if cached != nil {
		_ = json.Unmarshal(cached, &version)
		return version, tx.Commit(ctx)
	}
	var published time.Time
	err = tx.QueryRow(ctx, `INSERT INTO gantry.evaluation_suite_versions (id, suite_id, content_digest, case_manifest_digest, fixture_manifest_digest, evaluator_policy_version_id, runtime_image_digest, created_by_principal_id) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT (suite_id, content_digest) DO UPDATE SET id=gantry.evaluation_suite_versions.id RETURNING id, published_at`, version.ID, suiteID, contentDigest, caseDigest, fixtureDigest, version.EvaluatorPolicyVersionID, version.RuntimeImageDigest, actor.ID).Scan(&version.ID, &published)
	if err != nil {
		return Version{}, err
	}
	version.PublishedAt = published.UTC().Format(time.RFC3339)
	if err := tx.QueryRow(ctx, `UPDATE gantry.evaluation_suites SET state='published', latest_version_id=$2, etag=etag+1, updated_at=now() WHERE id=$1 AND etag::text=$3 AND state='draft' RETURNING 1`, suiteID, version.ID, strings.Trim(expectedETag, `"`)).Scan(new(int)); errors.Is(err, pgx.ErrNoRows) {
		return Version{}, ErrETagConflict
	} else if err != nil {
		return Version{}, err
	}
	if err := appendAudit(ctx, tx, actor, suiteID, suite.WorkspaceID, "evaluation_suite.version.published", map[string]any{"workspace_id": suite.WorkspaceID, "suite_version_id": version.ID, "outcome": "success", "risk": "medium"}); err != nil {
		return Version{}, err
	}
	if err := saveIdempotency(ctx, tx, actor.ID, "publish:"+suiteID, key, digest, version); err != nil {
		return Version{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Version{}, err
	}
	return version, nil
}

func (s *Service) ListRuns(ctx context.Context, actor identity.Principal, suiteID string) ([]Run, error) {
	if _, err := s.getSuite(ctx, actor, suiteID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT r.id, r.suite_version_id, r.candidate_revision_hash, r.baseline_revision_hash, r.environment_digest, r.state, r.gate_result, r.deterministic_summary, r.probabilistic_summary, r.evidence_manifest_digest, r.created_at FROM gantry.evaluation_runs r JOIN gantry.evaluation_suite_versions v ON v.id=r.suite_version_id WHERE v.suite_id=$1 ORDER BY r.created_at DESC, r.id`, suiteID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Run, 0)
	for rows.Next() {
		item, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) CreateRun(ctx context.Context, actor identity.Principal, suiteID, key string, request CreateRunRequest) (Run, error) {
	suite, err := s.getSuite(ctx, actor, suiteID)
	if err != nil {
		return Run{}, err
	}
	if strings.TrimSpace(key) == "" || request.SuiteVersionID == "" || request.CandidateRevisionHash == "" || request.EnvironmentDigest == "" {
		return Run{}, ErrInvalidInput
	}
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM gantry.evaluation_suite_versions WHERE id=$1 AND suite_id=$2)`, request.SuiteVersionID, suiteID).Scan(&exists); err != nil {
		return Run{}, err
	}
	if !exists {
		return Run{}, ErrNotFound
	}
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM gantry.agent_revisions r JOIN gantry.agents a ON a.id=r.agent_id WHERE r.revision_hash=$1 AND a.organization_id=$2)`, request.CandidateRevisionHash, actor.OrganizationID).Scan(&exists); err != nil {
		return Run{}, err
	}
	if !exists {
		return Run{}, ErrNotFound
	}
	item := Run{ID: newID("erun"), SuiteVersionID: request.SuiteVersionID, CandidateRevisionHash: request.CandidateRevisionHash, BaselineRevisionHash: request.BaselineRevisionHash, EnvironmentDigest: request.EnvironmentDigest, State: "requested", GateResult: "not_applicable", DeterministicSummary: json.RawMessage(`{}`), CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Run{}, err
	}
	defer tx.Rollback(ctx)
	digest := digestJSON(request)
	if cached, conflict, err := loadIdempotency(ctx, tx, actor.ID, "run:"+suiteID, key, digest); err != nil {
		return Run{}, err
	} else if conflict {
		return Run{}, ErrIdempotencyConflict
	} else if cached != nil {
		_ = json.Unmarshal(cached, &item)
		return item, tx.Commit(ctx)
	}
	var created time.Time
	err = tx.QueryRow(ctx, `INSERT INTO gantry.evaluation_runs (id,suite_version_id,candidate_revision_hash,baseline_revision_hash,environment_digest,state,gate_result,requested_by_principal_id) VALUES ($1,$2,$3,$4,$5,'requested','not_applicable',$6) RETURNING created_at`, item.ID, item.SuiteVersionID, item.CandidateRevisionHash, item.BaselineRevisionHash, item.EnvironmentDigest, actor.ID).Scan(&created)
	if err != nil {
		return Run{}, err
	}
	requirement := canonical(map[string]any{"kind": "evaluation_run_required", "evaluation_run_id": item.ID})
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.evaluation_gates(id,evaluation_run_id,agent_revision_hash,suite_version_id,requirement,state) VALUES($1,$2,$3,$4,$5::jsonb,'required')`, newID("egate"), item.ID, item.CandidateRevisionHash, item.SuiteVersionID, string(requirement)); err != nil {
		return Run{}, err
	}
	item.CreatedAt = created.UTC().Format(time.RFC3339)
	if err := appendAudit(ctx, tx, actor, suiteID, suite.WorkspaceID, "evaluation_run.requested", map[string]any{"workspace_id": suite.WorkspaceID, "evaluation_run_id": item.ID, "outcome": "accepted", "risk": "medium"}); err != nil {
		return Run{}, err
	}
	if err := saveIdempotency(ctx, tx, actor.ID, "run:"+suiteID, key, digest, item); err != nil {
		return Run{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Run{}, err
	}
	return item, nil
}

func (s *Service) GetRun(ctx context.Context, actor identity.Principal, id string) (Run, error) {
	var item Run
	var created time.Time
	err := s.pool.QueryRow(ctx, `SELECT r.id,r.suite_version_id,r.candidate_revision_hash,r.baseline_revision_hash,r.environment_digest,r.state,r.gate_result,r.deterministic_summary,r.probabilistic_summary,r.evidence_manifest_digest,r.created_at FROM gantry.evaluation_runs r JOIN gantry.evaluation_suite_versions v ON v.id=r.suite_version_id JOIN gantry.evaluation_suites s ON s.id=v.suite_id WHERE r.id=$1 AND s.organization_id=$2 AND (EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$3 AND rb.role='organization_admin' AND rb.workspace_id IS NULL) OR EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$3 AND rb.role='workspace_agent_editor' AND rb.workspace_id=s.workspace_id))`, id, actor.OrganizationID, actor.ID).Scan(&item.ID, &item.SuiteVersionID, &item.CandidateRevisionHash, &item.BaselineRevisionHash, &item.EnvironmentDigest, &item.State, &item.GateResult, &item.DeterministicSummary, &item.ProbabilisticSummary, &item.EvidenceManifestDigest, &created)
	if errors.Is(err, pgx.ErrNoRows) {
		return Run{}, ErrNotFound
	}
	if err != nil {
		return Run{}, err
	}
	item.CreatedAt = created.UTC().Format(time.RFC3339)
	return item, nil
}

func (s *Service) CancelRun(ctx context.Context, actor identity.Principal, id string) (Run, error) {
	item, err := s.GetRun(ctx, actor, id)
	if err != nil {
		return Run{}, err
	}
	if item.State != "requested" && item.State != "queued" && item.State != "provisioning" && item.State != "running" {
		return Run{}, ErrInvalidState
	}
	if _, err := s.pool.Exec(ctx, `UPDATE gantry.evaluation_runs SET state='canceled',updated_at=now() WHERE id=$1`, id); err != nil {
		return Run{}, err
	}
	item.State = "canceled"
	return item, nil
}

func (s *Service) ListGates(ctx context.Context, actor identity.Principal, workspaceID, agentRevisionHash string) (GateList, error) {
	workspaceID, agentRevisionHash = strings.TrimSpace(workspaceID), strings.TrimSpace(agentRevisionHash)
	if workspaceID != "" {
		if err := s.authz.RequireWorkspace(ctx, actor, workspaceID); err != nil {
			return GateList{}, err
		}
	}
	rows, err := s.pool.Query(ctx, `
		SELECT g.id,g.agent_revision_hash,g.suite_version_id,g.requirement,
			CASE WHEN o.id IS NOT NULL AND o.expires_at <= now() THEN 'expired' ELSE g.state END,
			g.override_id
		FROM gantry.evaluation_gates g
		JOIN gantry.evaluation_suite_versions v ON v.id=g.suite_version_id
		JOIN gantry.evaluation_suites s ON s.id=v.suite_id
		LEFT JOIN LATERAL (SELECT id,expires_at FROM gantry.evaluation_gate_overrides WHERE gate_id=g.id ORDER BY created_at DESC,id DESC LIMIT 1) o ON true
		WHERE s.organization_id=$1 AND ($2='' OR s.workspace_id=$2) AND ($3='' OR g.agent_revision_hash=$3)
		AND (EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$4 AND rb.role='organization_admin' AND rb.workspace_id IS NULL)
			OR EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$4 AND rb.role='workspace_agent_editor' AND rb.workspace_id=s.workspace_id))
		ORDER BY g.created_at DESC,g.id`, actor.OrganizationID, workspaceID, agentRevisionHash, actor.ID)
	if err != nil {
		return GateList{}, err
	}
	defer rows.Close()
	items := make([]Gate, 0)
	for rows.Next() {
		var item Gate
		if err := rows.Scan(&item.ID, &item.AgentRevisionHash, &item.SuiteVersionID, &item.Requirement, &item.State, &item.OverrideID); err != nil {
			return GateList{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return GateList{}, err
	}
	return GateList{Items: items}, nil
}

func (s *Service) OverrideGate(ctx context.Context, actor identity.Principal, gateID string, request OverrideGateRequest) (Gate, error) {
	request.Reason, request.ExpiresAt = strings.TrimSpace(request.Reason), strings.TrimSpace(request.ExpiresAt)
	if request.Reason == "" || request.ExpiresAt == "" {
		return Gate{}, ErrInvalidInput
	}
	expiresAt, err := time.Parse(time.RFC3339, request.ExpiresAt)
	if err != nil || !expiresAt.After(time.Now()) {
		return Gate{}, ErrInvalidInput
	}
	var gate Gate
	var workspaceID string
	err = s.pool.QueryRow(ctx, `
		SELECT g.id,g.agent_revision_hash,g.suite_version_id,g.requirement,g.state,g.override_id,s.workspace_id
		FROM gantry.evaluation_gates g
		JOIN gantry.evaluation_suite_versions v ON v.id=g.suite_version_id
		JOIN gantry.evaluation_suites s ON s.id=v.suite_id
		WHERE g.id=$1 AND s.organization_id=$2`, gateID, actor.OrganizationID).Scan(&gate.ID, &gate.AgentRevisionHash, &gate.SuiteVersionID, &gate.Requirement, &gate.State, &gate.OverrideID, &workspaceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Gate{}, ErrNotFound
	}
	if err != nil {
		return Gate{}, err
	}
	if err := s.authz.RequireWorkspace(ctx, actor, workspaceID); err != nil {
		return Gate{}, err
	}
	if gate.State == "passed" || gate.State == "expired" {
		return Gate{}, ErrInvalidState
	}
	override := GateOverride{ID: newID("egateovr"), GateID: gate.ID, Reason: request.Reason, ReviewerPrincipalID: actor.ID, ExpiresAt: expiresAt.UTC().Format(time.RFC3339)}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Gate{}, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `INSERT INTO gantry.evaluation_gate_overrides(id,gate_id,reason,reviewer_principal_id,expires_at) VALUES($1,$2,$3,$4,$5)`, override.ID, override.GateID, override.Reason, override.ReviewerPrincipalID, expiresAt); err != nil {
		return Gate{}, err
	}
	if err := tx.QueryRow(ctx, `UPDATE gantry.evaluation_gates SET state='overridden',override_id=$2,updated_at=now() WHERE id=$1 RETURNING id,agent_revision_hash,suite_version_id,requirement,state,override_id`, gate.ID, override.ID).Scan(&gate.ID, &gate.AgentRevisionHash, &gate.SuiteVersionID, &gate.Requirement, &gate.State, &gate.OverrideID); err != nil {
		return Gate{}, err
	}
	if err := appendAudit(ctx, tx, actor, gate.ID, workspaceID, "evaluation_gate.overridden", map[string]any{"gate_id": gate.ID, "override_id": override.ID, "expires_at": override.ExpiresAt, "outcome": "success", "risk": "high"}); err != nil {
		return Gate{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Gate{}, err
	}
	return gate, nil
}

func (s *Service) ListRunRegressions(ctx context.Context, actor identity.Principal, runID string) (RegressionList, error) {
	run, err := s.GetRun(ctx, actor, runID)
	if err != nil {
		return RegressionList{}, err
	}
	if run.BaselineRevisionHash == nil || run.State != "completed" {
		state := "pending"
		if run.State == "invalid" {
			state = "invalid"
		}
		return RegressionList{ComparisonState: state, Items: []Regression{}}, nil
	}
	var summary map[string]any
	if err := json.Unmarshal(run.DeterministicSummary, &summary); err != nil {
		return RegressionList{ComparisonState: "invalid", Items: []Regression{}}, nil
	}
	candidate, candidateOK := summary["candidate"].(map[string]any)
	baseline, baselineOK := summary["baseline"].(map[string]any)
	if !candidateOK || !baselineOK {
		return RegressionList{ComparisonState: "invalid", Items: []Regression{}}, nil
	}
	items := make([]Regression, 0)
	for key, candidateValue := range candidate {
		baselineValue, exists := baseline[key]
		if !exists || string(canonical(candidateValue)) == string(canonical(baselineValue)) {
			continue
		}
		if becameFailure(candidateValue, baselineValue) {
			items = append(items, Regression{Kind: "deterministic", Severity: "high", CaseID: key, Message: "Candidate result regressed from the baseline.", CandidateEvidence: canonical(candidateValue), BaselineEvidence: canonical(baselineValue)})
		}
	}
	return RegressionList{ComparisonState: "comparable", Items: items}, nil
}

func (s *Service) getSuite(ctx context.Context, actor identity.Principal, id string) (Suite, error) {
	var item Suite
	err := s.pool.QueryRow(ctx, `SELECT s.id,s.organization_id,s.workspace_id,s.name,s.state,s.owner_principal_id,s.latest_version_id,s.etag::text,(SELECT count(*) FROM gantry.evaluation_suite_versions v WHERE v.suite_id=s.id) FROM gantry.evaluation_suites s WHERE s.id=$1 AND s.organization_id=$2 AND (EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$3 AND rb.role='organization_admin' AND rb.workspace_id IS NULL) OR EXISTS (SELECT 1 FROM gantry.role_bindings rb WHERE rb.principal_id=$3 AND rb.role='workspace_agent_editor' AND rb.workspace_id=s.workspace_id))`, id, actor.OrganizationID, actor.ID).Scan(&item.ID, &item.OrganizationID, &item.WorkspaceID, &item.Name, &item.State, &item.OwnerPrincipalID, &item.LatestVersionID, &item.ETag, &item.GateUsageCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return Suite{}, ErrNotFound
	}
	return item, err
}

func normalizeCase(request CreateCaseRequest) (Case, error) {
	if len(request.Input) == 0 || len(request.FixtureManifest) == 0 || len(request.Assertions) == 0 {
		return Case{}, ErrInvalidInput
	}
	var input, fixture, compat map[string]any
	var assertions []any
	if json.Unmarshal(request.Input, &input) != nil || json.Unmarshal(request.FixtureManifest, &fixture) != nil || json.Unmarshal(request.Assertions, &assertions) != nil {
		return Case{}, ErrInvalidInput
	}
	if request.Compatibility == nil {
		request.Compatibility = json.RawMessage(`{}`)
	}
	if json.Unmarshal(request.Compatibility, &compat) != nil {
		return Case{}, ErrInvalidInput
	}
	return Case{Input: canonical(input), FixtureManifest: canonical(fixture), Assertions: canonical(assertions), Rubric: request.Rubric, Compatibility: canonical(compat)}, nil
}
func scanSuite(row interface{ Scan(...any) error }) (Suite, error) {
	var item Suite
	var count int
	err := row.Scan(&item.ID, &item.OrganizationID, &item.WorkspaceID, &item.Name, &item.State, &item.OwnerPrincipalID, &item.LatestVersionID, &item.ETag, &count)
	item.GateUsageCount = count
	return item, err
}
func scanCase(row interface{ Scan(...any) error }) (Case, error) {
	var item Case
	return item, row.Scan(&item.ID, &item.SuiteID, &item.Input, &item.FixtureManifest, &item.Assertions, &item.Rubric, &item.Compatibility, &item.ETag)
}
func scanVersion(row interface{ Scan(...any) error }) (Version, error) {
	var item Version
	var t time.Time
	err := row.Scan(&item.ID, &item.SuiteID, &item.ContentDigest, &item.CaseManifestDigest, &item.FixtureManifestDigest, &item.EvaluatorPolicyVersionID, &item.RuntimeImageDigest, &t)
	item.PublishedAt = t.UTC().Format(time.RFC3339)
	return item, err
}
func scanRun(row interface{ Scan(...any) error }) (Run, error) {
	var item Run
	var t time.Time
	err := row.Scan(&item.ID, &item.SuiteVersionID, &item.CandidateRevisionHash, &item.BaselineRevisionHash, &item.EnvironmentDigest, &item.State, &item.GateResult, &item.DeterministicSummary, &item.ProbabilisticSummary, &item.EvidenceManifestDigest, &t)
	item.CreatedAt = t.UTC().Format(time.RFC3339)
	return item, err
}
func caseManifest(items []Case) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{"id": item.ID, "input": json.RawMessage(item.Input), "assertions": json.RawMessage(item.Assertions)})
	}
	return result
}
func fixtureManifest(items []Case) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{"id": item.ID, "fixture_manifest": json.RawMessage(item.FixtureManifest)})
	}
	return result
}
func appendAudit(ctx context.Context, tx pgx.Tx, actor identity.Principal, resourceID, workspaceID, eventType string, payload map[string]any) error {
	value, _ := json.Marshal(payload)
	_, err := tx.Exec(ctx, `INSERT INTO gantry.audit_events (organization_id,actor_principal_id,resource_type,resource_id,event_type,payload) VALUES ($1,$2,'evaluation_suite',$3,$4,$5::jsonb)`, actor.OrganizationID, actor.ID, resourceID, eventType, string(value))
	return err
}
func loadIdempotency(ctx context.Context, tx pgx.Tx, principal, route, key, digest string) ([]byte, bool, error) {
	var stored string
	var response []byte
	err := tx.QueryRow(ctx, `SELECT request_digest,response_json FROM gantry.evaluation_command_idempotency WHERE principal_id=$1 AND route=$2 AND idempotency_key=$3`, principal, route, key).Scan(&stored, &response)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return response, stored != digest, nil
}
func saveIdempotency(ctx context.Context, tx pgx.Tx, principal, route, key, digest string, response any) error {
	value, _ := json.Marshal(response)
	_, err := tx.Exec(ctx, `INSERT INTO gantry.evaluation_command_idempotency (principal_id,route,idempotency_key,request_digest,response_json) VALUES ($1,$2,$3,$4,$5::jsonb)`, principal, route, key, digest, string(value))
	return err
}
func normalizeListOptions(options ListOptions) ListOptions {
	options.WorkspaceID, options.State, options.Search = strings.TrimSpace(options.WorkspaceID), strings.TrimSpace(options.State), strings.TrimSpace(options.Search)
	if options.Limit < 1 {
		options.Limit = 50
	}
	if options.Limit > 100 {
		options.Limit = 100
	}
	return options
}
func canonical(value any) json.RawMessage { raw, _ := json.Marshal(value); return raw }
func becameFailure(candidate, baseline any) bool {
	candidateState, candidateOK := candidate.(string)
	baselineState, baselineOK := baseline.(string)
	return candidateOK && baselineOK && candidateState == "failed" && baselineState != "failed"
}
func nullableJSON(value json.RawMessage) any {
	if len(value) == 0 {
		return nil
	}
	return string(value)
}
func digestJSON(value any) string {
	raw, _ := json.Marshal(value)
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}
func newID(prefix string) string {
	value := make([]byte, 12)
	if _, err := rand.Read(value); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(value)
}
func encodeCursor(value string) string { return base64.RawURLEncoding.EncodeToString([]byte(value)) }
