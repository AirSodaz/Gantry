package adminplatform

import (
	"context"
	"crypto/rand"
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
	ErrNotFound     = errors.New("admin platform resource not found")
	ErrInvalidInput = errors.New("invalid admin platform input")
	ErrInvalidState = errors.New("invalid admin platform state")
	ErrETagConflict = errors.New("admin platform etag conflict")
)

type Service struct {
	pool  *pgxpool.Pool
	authz *authorization.Service
}

func NewService(pool *pgxpool.Pool, authz *authorization.Service) *Service {
	return &Service{pool: pool, authz: authz}
}

func (s *Service) ListProviders(ctx context.Context, actor identity.Principal) ([]ModelProvider, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,organization_id,name,state,data_classes,credential_reference_id,health FROM gantry.platform_model_providers WHERE organization_id=$1 ORDER BY name,id`, actor.OrganizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ModelProvider, 0)
	for rows.Next() {
		var item ModelProvider
		var classes []byte
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.Name, &item.State, &classes, &item.CredentialReferenceID, &item.Health); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(classes, &item.DataClasses)
		if item.DataClasses == nil {
			item.DataClasses = []string{}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) CreateProvider(ctx context.Context, actor identity.Principal, req CreateProviderRequest) (ModelProvider, error) {
	if err := s.authz.RequireOrganizationAdmin(ctx, actor); err != nil {
		return ModelProvider{}, err
	}
	req.Name = strings.TrimSpace(req.Name)
	req.CredentialReferenceID = strings.TrimSpace(req.CredentialReferenceID)
	if req.Name == "" || req.CredentialReferenceID == "" {
		return ModelProvider{}, ErrInvalidInput
	}
	if len(req.DataClasses) == 0 {
		return ModelProvider{}, ErrInvalidInput
	}
	classes, _ := json.Marshal(req.DataClasses)
	item := ModelProvider{ID: newID("prov"), OrganizationID: actor.OrganizationID, Name: req.Name, State: "active", DataClasses: req.DataClasses, CredentialReferenceID: req.CredentialReferenceID, Health: json.RawMessage(`{"status":"unknown"}`)}
	err := s.pool.QueryRow(ctx, `INSERT INTO gantry.platform_model_providers(id,organization_id,name,state,data_classes,credential_reference_id,health) VALUES($1,$2,$3,'active',$4::jsonb,$5,$6::jsonb) RETURNING id`, item.ID, item.OrganizationID, item.Name, string(classes), item.CredentialReferenceID, string(item.Health)).Scan(&item.ID)
	return item, err
}

func (s *Service) ListRoutes(ctx context.Context, actor identity.Principal, providerID string) ([]ProviderRoute, error) {
	if err := s.providerVisible(ctx, actor, providerID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id,provider_id,allowed_models,fallback_route_ids,state,budget_policy_id,classification_constraints,etag::text FROM gantry.platform_provider_routes WHERE provider_id=$1 ORDER BY id`, providerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ProviderRoute, 0)
	for rows.Next() {
		var item ProviderRoute
		var models, fallback []byte
		if err := rows.Scan(&item.ID, &item.ProviderID, &models, &fallback, &item.State, &item.BudgetPolicyID, &item.ClassificationConstraints, &item.ETag); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(models, &item.AllowedModels)
		_ = json.Unmarshal(fallback, &item.FallbackRouteIDs)
		if item.AllowedModels == nil {
			item.AllowedModels = []string{}
		}
		if item.FallbackRouteIDs == nil {
			item.FallbackRouteIDs = []string{}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) PutRoute(ctx context.Context, actor identity.Principal, providerID, routeID, expectedETag string, req PutRouteRequest) (ProviderRoute, error) {
	if err := s.authz.RequireOrganizationAdmin(ctx, actor); err != nil {
		return ProviderRoute{}, err
	}
	if err := s.providerVisible(ctx, actor, providerID); err != nil {
		return ProviderRoute{}, err
	}
	if len(req.AllowedModels) == 0 || !validRouteState(req.State) {
		return ProviderRoute{}, ErrInvalidInput
	}
	expectedETag = strings.Trim(expectedETag, `"`)
	if expectedETag == "" {
		return ProviderRoute{}, ErrETagConflict
	}
	models, _ := json.Marshal(req.AllowedModels)
	fallback, _ := json.Marshal(req.FallbackRouteIDs)
	constraints := req.ClassificationConstraints
	if len(constraints) == 0 {
		constraints = json.RawMessage(`{}`)
	}
	var item ProviderRoute
	err := s.pool.QueryRow(ctx, `UPDATE gantry.platform_provider_routes SET allowed_models=$4::jsonb,fallback_route_ids=$5::jsonb,state=$6,budget_policy_id=$7,classification_constraints=$8::jsonb,etag=etag+1,updated_at=now() WHERE id=$1 AND provider_id=$2 AND etag::text=$3 RETURNING id,provider_id,allowed_models,fallback_route_ids,state,budget_policy_id,classification_constraints,etag::text`, routeID, providerID, expectedETag, string(models), string(fallback), req.State, req.BudgetPolicyID, string(constraints)).Scan(&item.ID, &item.ProviderID, &models, &fallback, &item.State, &item.BudgetPolicyID, &item.ClassificationConstraints, &item.ETag)
	if errors.Is(err, pgx.ErrNoRows) {
		return ProviderRoute{}, ErrETagConflict
	}
	if err != nil {
		return ProviderRoute{}, err
	}
	_ = json.Unmarshal(models, &item.AllowedModels)
	_ = json.Unmarshal(fallback, &item.FallbackRouteIDs)
	return item, nil
}

func (s *Service) QuarantineProvider(ctx context.Context, actor identity.Principal, providerID string) (ModelProvider, error) {
	if err := s.authz.RequireOrganizationAdmin(ctx, actor); err != nil {
		return ModelProvider{}, err
	}
	if err := s.providerVisible(ctx, actor, providerID); err != nil {
		return ModelProvider{}, err
	}
	var item ModelProvider
	var classes []byte
	err := s.pool.QueryRow(ctx, `UPDATE gantry.platform_model_providers SET state='quarantined',updated_at=now() WHERE id=$1 AND organization_id=$2 RETURNING id,organization_id,name,state,data_classes,credential_reference_id,health`, providerID, actor.OrganizationID).Scan(&item.ID, &item.OrganizationID, &item.Name, &item.State, &classes, &item.CredentialReferenceID, &item.Health)
	if err != nil {
		return ModelProvider{}, err
	}
	_ = json.Unmarshal(classes, &item.DataClasses)
	return item, nil
}

func (s *Service) ListRunnerPools(ctx context.Context, actor identity.Principal) ([]RunnerPool, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,organization_id,isolation_tier,state,compatible_protocols,capacity FROM gantry.platform_runner_pools WHERE organization_id=$1 ORDER BY id`, actor.OrganizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]RunnerPool, 0)
	for rows.Next() {
		var item RunnerPool
		var protocols []byte
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.IsolationTier, &item.State, &protocols, &item.Capacity); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(protocols, &item.CompatibleProtocols)
		if item.CompatibleProtocols == nil {
			item.CompatibleProtocols = []string{}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s *Service) CreateRunnerPool(ctx context.Context, actor identity.Principal, req CreateRunnerPoolRequest) (RunnerPool, error) {
	if err := s.authz.RequireOrganizationAdmin(ctx, actor); err != nil {
		return RunnerPool{}, err
	}
	if req.IsolationTier != "development" && req.IsolationTier != "gvisor" && req.IsolationTier != "microvm" || len(req.CompatibleProtocols) == 0 {
		return RunnerPool{}, ErrInvalidInput
	}
	protocols, _ := json.Marshal(req.CompatibleProtocols)
	capacity := req.Capacity
	if len(capacity) == 0 {
		capacity = json.RawMessage(`{}`)
	}
	item := RunnerPool{ID: newID("rpool"), OrganizationID: actor.OrganizationID, IsolationTier: req.IsolationTier, State: "active", CompatibleProtocols: req.CompatibleProtocols, Capacity: capacity}
	_, err := s.pool.Exec(ctx, `INSERT INTO gantry.platform_runner_pools(id,organization_id,isolation_tier,state,compatible_protocols,capacity) VALUES($1,$2,$3,'active',$4::jsonb,$5::jsonb)`, item.ID, item.OrganizationID, item.IsolationTier, string(protocols), string(capacity))
	return item, err
}
func (s *Service) ListRunners(ctx context.Context, actor identity.Principal, poolID string) ([]Runner, error) {
	if err := s.pool.QueryRow(ctx, `SELECT 1 FROM gantry.platform_runner_pools WHERE id=$1 AND organization_id=$2`, poolID, actor.OrganizationID).Scan(new(int)); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id,pool_id,state,protocol_version,lease_epoch,last_heartbeat_at FROM gantry.platform_runners WHERE pool_id=$1 ORDER BY id`, poolID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Runner, 0)
	for rows.Next() {
		var item Runner
		var t *time.Time
		if err := rows.Scan(&item.ID, &item.PoolID, &item.State, &item.ProtocolVersion, &item.LeaseEpoch, &t); err != nil {
			return nil, err
		}
		if t != nil {
			v := t.UTC().Format(time.RFC3339)
			item.LastHeartbeatAt = &v
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s *Service) SetPoolState(ctx context.Context, actor identity.Principal, poolID, state string) (RunnerPool, error) {
	if err := s.authz.RequireOrganizationAdmin(ctx, actor); err != nil {
		return RunnerPool{}, err
	}
	if state != "draining" && state != "quarantined" {
		return RunnerPool{}, ErrInvalidInput
	}
	var item RunnerPool
	var protocols []byte
	err := s.pool.QueryRow(ctx, `UPDATE gantry.platform_runner_pools SET state=$3,updated_at=now() WHERE id=$1 AND organization_id=$2 RETURNING id,organization_id,isolation_tier,state,compatible_protocols,capacity`, poolID, actor.OrganizationID, state).Scan(&item.ID, &item.OrganizationID, &item.IsolationTier, &item.State, &protocols, &item.Capacity)
	if errors.Is(err, pgx.ErrNoRows) {
		return RunnerPool{}, ErrNotFound
	}
	if err != nil {
		return RunnerPool{}, err
	}
	_ = json.Unmarshal(protocols, &item.CompatibleProtocols)
	return item, nil
}

func (s *Service) ListCredentials(ctx context.Context, actor identity.Principal) ([]CredentialReference, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,organization_id,target_service,state,classification,allowed_modes,secret_version,expires_at FROM gantry.platform_credential_references WHERE organization_id=$1 ORDER BY target_service,id`, actor.OrganizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]CredentialReference, 0)
	for rows.Next() {
		var item CredentialReference
		var modes []byte
		var expires *time.Time
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.TargetService, &item.State, &item.Classification, &modes, &item.SecretVersion, &expires); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(modes, &item.AllowedModes)
		if item.AllowedModes == nil {
			item.AllowedModes = []string{}
		}
		if expires != nil {
			value := expires.UTC().Format(time.RFC3339)
			item.ExpiresAt = &value
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) RotateCredential(ctx context.Context, actor identity.Principal, id string) (CredentialReference, error) {
	if err := s.authz.RequireOrganizationAdmin(ctx, actor); err != nil {
		return CredentialReference{}, err
	}
	return s.updateCredentialState(ctx, actor, id, "rotating")
}

func (s *Service) RevokeCredential(ctx context.Context, actor identity.Principal, id string) (CredentialReference, error) {
	if err := s.authz.RequireOrganizationAdmin(ctx, actor); err != nil {
		return CredentialReference{}, err
	}
	return s.updateCredentialState(ctx, actor, id, "revoked")
}

func (s *Service) updateCredentialState(ctx context.Context, actor identity.Principal, id, state string) (CredentialReference, error) {
	var item CredentialReference
	var modes []byte
	var expires *time.Time
	err := s.pool.QueryRow(ctx, `UPDATE gantry.platform_credential_references SET state=$3,updated_at=now() WHERE id=$1 AND organization_id=$2 RETURNING id,organization_id,target_service,state,classification,allowed_modes,secret_version,expires_at`, id, actor.OrganizationID, state).Scan(&item.ID, &item.OrganizationID, &item.TargetService, &item.State, &item.Classification, &modes, &item.SecretVersion, &expires)
	if errors.Is(err, pgx.ErrNoRows) {
		return CredentialReference{}, ErrNotFound
	}
	if err != nil {
		return CredentialReference{}, err
	}
	_ = json.Unmarshal(modes, &item.AllowedModes)
	if expires != nil {
		value := expires.UTC().Format(time.RFC3339)
		item.ExpiresAt = &value
	}
	return item, nil
}

func (s *Service) ListClassifications(ctx context.Context, actor identity.Principal) ([]DataClassification, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,organization_id,label,handling,retention_class,allowed_provider_ids,allowed_tool_classes,etag::text FROM gantry.platform_data_classifications WHERE organization_id=$1 ORDER BY label,id`, actor.OrganizationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]DataClassification, 0)
	for rows.Next() {
		var item DataClassification
		var providers, tools []byte
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.Label, &item.Handling, &item.RetentionClass, &providers, &tools, &item.ETag); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(providers, &item.AllowedProviderIDs)
		_ = json.Unmarshal(tools, &item.AllowedToolClasses)
		if item.AllowedProviderIDs == nil {
			item.AllowedProviderIDs = []string{}
		}
		if item.AllowedToolClasses == nil {
			item.AllowedToolClasses = []string{}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) CreateClassification(ctx context.Context, actor identity.Principal, req CreateDataClassificationRequest) (DataClassification, error) {
	if err := s.authz.RequireOrganizationAdmin(ctx, actor); err != nil {
		return DataClassification{}, err
	}
	req.Label, req.RetentionClass = strings.TrimSpace(req.Label), strings.TrimSpace(req.RetentionClass)
	if req.Label == "" || req.RetentionClass == "" || !validHandling(req.Handling) {
		return DataClassification{}, ErrInvalidInput
	}
	providers, _ := json.Marshal(req.AllowedProviderIDs)
	tools, _ := json.Marshal(req.AllowedToolClasses)
	item := DataClassification{ID: newID("class"), OrganizationID: actor.OrganizationID, Label: req.Label, Handling: req.Handling, RetentionClass: req.RetentionClass, AllowedProviderIDs: req.AllowedProviderIDs, AllowedToolClasses: req.AllowedToolClasses, ETag: "1"}
	_, err := s.pool.Exec(ctx, `INSERT INTO gantry.platform_data_classifications(id,organization_id,label,handling,retention_class,allowed_provider_ids,allowed_tool_classes,etag) VALUES($1,$2,$3,$4,$5,$6::jsonb,$7::jsonb,1)`, item.ID, item.OrganizationID, item.Label, item.Handling, item.RetentionClass, string(providers), string(tools))
	return item, err
}

func (s *Service) ListLimitPolicies(ctx context.Context, actor identity.Principal, workspaceID string) ([]LimitPolicy, error) {
	if err := s.requireScopeRead(ctx, actor, workspaceID); err != nil {
		return nil, err
	}
	query := `SELECT id,organization_id,workspace_id,concurrency,duration_seconds,output_bytes,artifact_bytes,budget,etag::text FROM gantry.platform_limit_policies WHERE organization_id=$1`
	args := []any{actor.OrganizationID}
	if workspaceID != "" {
		query += ` AND (workspace_id IS NULL OR workspace_id=$2)`
		args = append(args, workspaceID)
	}
	query += ` ORDER BY workspace_id NULLS FIRST,id`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]LimitPolicy, 0)
	for rows.Next() {
		var item LimitPolicy
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.WorkspaceID, &item.Concurrency, &item.DurationSeconds, &item.OutputBytes, &item.ArtifactBytes, &item.Budget, &item.ETag); err != nil {
			return nil, err
		}
		if len(item.Budget) == 0 {
			item.Budget = json.RawMessage(`{}`)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) UpsertLimitPolicy(ctx context.Context, actor identity.Principal, id, expectedETag string, req UpsertLimitPolicyRequest) (LimitPolicy, error) {
	if err := s.authz.RequireOrganizationAdmin(ctx, actor); err != nil {
		return LimitPolicy{}, err
	}
	if req.WorkspaceID != nil && strings.TrimSpace(*req.WorkspaceID) == "" {
		return LimitPolicy{}, ErrInvalidInput
	}
	if req.WorkspaceID != nil {
		if err := s.workspaceInOrganization(ctx, actor, *req.WorkspaceID); err != nil {
			return LimitPolicy{}, err
		}
	}
	if req.Concurrency < 0 || req.DurationSeconds <= 0 || req.OutputBytes < 0 || req.ArtifactBytes < 0 || !jsonObject(req.Budget) {
		return LimitPolicy{}, ErrInvalidInput
	}
	if req.WorkspaceID != nil {
		if err := s.validateLimitBound(ctx, actor.OrganizationID, *req.WorkspaceID, req); err != nil {
			return LimitPolicy{}, err
		}
	}
	expectedETag = strings.Trim(expectedETag, `"`)
	if expectedETag == "" {
		return LimitPolicy{}, ErrETagConflict
	}
	if len(req.Budget) == 0 {
		req.Budget = json.RawMessage(`{}`)
	}
	if id == "" {
		id = newID("limit")
	}
	var item LimitPolicy
	err := s.pool.QueryRow(ctx, `
		INSERT INTO gantry.platform_limit_policies(id,organization_id,workspace_id,concurrency,duration_seconds,output_bytes,artifact_bytes,budget,etag)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8::jsonb,1)
		ON CONFLICT (id) DO UPDATE SET workspace_id=$3,concurrency=$4,duration_seconds=$5,output_bytes=$6,artifact_bytes=$7,budget=$8::jsonb,etag=gantry.platform_limit_policies.etag+1,updated_at=now()
		WHERE gantry.platform_limit_policies.organization_id=$2 AND gantry.platform_limit_policies.etag::text=$9
		RETURNING id,organization_id,workspace_id,concurrency,duration_seconds,output_bytes,artifact_bytes,budget,etag::text`, id, actor.OrganizationID, req.WorkspaceID, req.Concurrency, req.DurationSeconds, req.OutputBytes, req.ArtifactBytes, string(req.Budget), expectedETag).Scan(&item.ID, &item.OrganizationID, &item.WorkspaceID, &item.Concurrency, &item.DurationSeconds, &item.OutputBytes, &item.ArtifactBytes, &item.Budget, &item.ETag)
	if errors.Is(err, pgx.ErrNoRows) {
		return LimitPolicy{}, ErrETagConflict
	}
	return item, err
}

func (s *Service) ListEnvironmentProfiles(ctx context.Context, actor identity.Principal, workspaceID string) ([]EnvironmentProfile, error) {
	if err := s.requireScopeRead(ctx, actor, workspaceID); err != nil {
		return nil, err
	}
	query := `SELECT id,organization_id,workspace_id,name,publication_posture,state,data_classification_id,allowed_target_controls,etag::text FROM gantry.platform_environment_profiles WHERE organization_id=$1`
	args := []any{actor.OrganizationID}
	if workspaceID != "" {
		query += ` AND (workspace_id IS NULL OR workspace_id=$2)`
		args = append(args, workspaceID)
	}
	query += ` ORDER BY workspace_id NULLS FIRST,name,id`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]EnvironmentProfile, 0)
	for rows.Next() {
		var item EnvironmentProfile
		if err := rows.Scan(&item.ID, &item.OrganizationID, &item.WorkspaceID, &item.Name, &item.PublicationPosture, &item.State, &item.DataClassificationID, &item.AllowedTargetControls, &item.ETag); err != nil {
			return nil, err
		}
		if len(item.AllowedTargetControls) == 0 {
			item.AllowedTargetControls = json.RawMessage(`{}`)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) UpsertEnvironmentProfile(ctx context.Context, actor identity.Principal, id, expectedETag string, req UpsertEnvironmentProfileRequest) (EnvironmentProfile, error) {
	if err := s.authz.RequireOrganizationAdmin(ctx, actor); err != nil {
		return EnvironmentProfile{}, err
	}
	req.Name, req.PublicationPosture, req.State = strings.TrimSpace(req.Name), strings.TrimSpace(req.PublicationPosture), strings.TrimSpace(req.State)
	if !validEnvironmentName(req.Name) || !validPublicationPosture(req.PublicationPosture) || !validEnvironmentState(req.State) || !jsonObject(req.AllowedTargetControls) {
		return EnvironmentProfile{}, ErrInvalidInput
	}
	if req.WorkspaceID != nil {
		if strings.TrimSpace(*req.WorkspaceID) == "" {
			return EnvironmentProfile{}, ErrInvalidInput
		}
		if err := s.workspaceInOrganization(ctx, actor, *req.WorkspaceID); err != nil {
			return EnvironmentProfile{}, err
		}
		if err := s.validateEnvironmentBound(ctx, actor.OrganizationID, *req.WorkspaceID, req); err != nil {
			return EnvironmentProfile{}, err
		}
	}
	if req.DataClassificationID != nil {
		var exists bool
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gantry.platform_data_classifications WHERE id=$1 AND organization_id=$2)`, *req.DataClassificationID, actor.OrganizationID).Scan(&exists); err != nil {
			return EnvironmentProfile{}, err
		}
		if !exists {
			return EnvironmentProfile{}, ErrInvalidInput
		}
	}
	expectedETag = strings.Trim(expectedETag, `"`)
	if expectedETag == "" {
		return EnvironmentProfile{}, ErrETagConflict
	}
	if len(req.AllowedTargetControls) == 0 {
		req.AllowedTargetControls = json.RawMessage(`{}`)
	}
	if id == "" {
		id = newID("env")
	}
	var item EnvironmentProfile
	err := s.pool.QueryRow(ctx, `
		INSERT INTO gantry.platform_environment_profiles(id,organization_id,workspace_id,name,publication_posture,state,data_classification_id,allowed_target_controls,etag)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8::jsonb,1)
		ON CONFLICT (id) DO UPDATE SET workspace_id=$3,name=$4,publication_posture=$5,state=$6,data_classification_id=$7,allowed_target_controls=$8::jsonb,etag=gantry.platform_environment_profiles.etag+1,updated_at=now()
		WHERE gantry.platform_environment_profiles.organization_id=$2 AND gantry.platform_environment_profiles.etag::text=$9
		RETURNING id,organization_id,workspace_id,name,publication_posture,state,data_classification_id,allowed_target_controls,etag::text`, id, actor.OrganizationID, req.WorkspaceID, req.Name, req.PublicationPosture, req.State, req.DataClassificationID, string(req.AllowedTargetControls), expectedETag).Scan(&item.ID, &item.OrganizationID, &item.WorkspaceID, &item.Name, &item.PublicationPosture, &item.State, &item.DataClassificationID, &item.AllowedTargetControls, &item.ETag)
	if errors.Is(err, pgx.ErrNoRows) {
		return EnvironmentProfile{}, ErrETagConflict
	}
	return item, err
}

func (s *Service) GetSettings(ctx context.Context, actor identity.Principal, workspaceID string) (PlatformSettingsProjection, error) {
	if err := s.requireScopeRead(ctx, actor, workspaceID); err != nil {
		return PlatformSettingsProjection{}, err
	}
	limits, err := s.ListLimitPolicies(ctx, actor, workspaceID)
	if err != nil {
		return PlatformSettingsProjection{}, err
	}
	environments, err := s.ListEnvironmentProfiles(ctx, actor, workspaceID)
	if err != nil {
		return PlatformSettingsProjection{}, err
	}
	classifications, err := s.ListClassifications(ctx, actor)
	if err != nil {
		return PlatformSettingsProjection{}, err
	}
	overrides := json.RawMessage(`{}`)
	etag := "1"
	var storedETag string
	if err := s.pool.QueryRow(ctx, `SELECT values,etag::text FROM gantry.platform_settings WHERE organization_id=$1 AND workspace_id IS NOT DISTINCT FROM $2 ORDER BY updated_at DESC,id DESC LIMIT 1`, actor.OrganizationID, nullableString(workspaceID)).Scan(&overrides, &storedETag); err == nil {
		etag = storedETag
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return PlatformSettingsProjection{}, err
	}
	values, _ := json.Marshal(map[string]any{"limit_policies": limits, "environment_profiles": environments, "data_classifications": classifications, "overrides": json.RawMessage(overrides)})
	scope := map[string]any{"type": "organization"}
	if workspaceID != "" {
		scope["type"] = "workspace"
		scope["workspace_id"] = workspaceID
	}
	return PlatformSettingsProjection{Scope: scope, Values: values, ETag: etag, ValidationState: "valid"}, nil
}

func (s *Service) ValidateSettings(ctx context.Context, actor identity.Principal, req SettingsApplyRequest) (SettingsValidation, error) {
	if req.WorkspaceID != nil {
		if err := s.authz.RequireWorkspace(ctx, actor, *req.WorkspaceID); err != nil {
			return SettingsValidation{}, err
		}
	} else if err := s.authz.RequireOrganizationAdmin(ctx, actor); err != nil {
		return SettingsValidation{}, err
	}
	if !jsonObject(req.Values) {
		return SettingsValidation{State: "invalid", Findings: []map[string]any{{"code": "values_object_required", "message": "values must be a JSON object"}}}, nil
	}
	return SettingsValidation{State: "valid", Findings: []map[string]any{}, SemanticDiff: []map[string]any{{"path": "overrides", "change": "replace"}}, RequiredCapabilities: []string{"platform.settings.manage"}}, nil
}

func (s *Service) ApplySettings(ctx context.Context, actor identity.Principal, expectedETag string, req SettingsApplyRequest) (PlatformSettingsProjection, error) {
	if err := s.authz.RequireOrganizationAdmin(ctx, actor); err != nil {
		return PlatformSettingsProjection{}, err
	}
	if !jsonObject(req.Values) {
		return PlatformSettingsProjection{}, ErrInvalidInput
	}
	expectedETag = strings.Trim(expectedETag, `"`)
	if expectedETag == "" {
		return PlatformSettingsProjection{}, ErrETagConflict
	}
	var workspaceID any
	if req.WorkspaceID != nil {
		workspaceID = *req.WorkspaceID
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PlatformSettingsProjection{}, err
	}
	defer tx.Rollback(ctx)
	var currentETag string
	lookupErr := tx.QueryRow(ctx, `SELECT etag::text FROM gantry.platform_settings WHERE organization_id=$1 AND workspace_id IS NOT DISTINCT FROM $2 ORDER BY updated_at DESC,id DESC LIMIT 1 FOR UPDATE`, actor.OrganizationID, workspaceID).Scan(&currentETag)
	switch {
	case lookupErr == nil:
		if currentETag != expectedETag {
			return PlatformSettingsProjection{}, ErrETagConflict
		}
		if _, err = tx.Exec(ctx, `UPDATE gantry.platform_settings SET values=$1::jsonb,etag=etag+1,updated_at=now() WHERE organization_id=$2 AND workspace_id IS NOT DISTINCT FROM $3 AND etag::text=$4`, string(req.Values), actor.OrganizationID, workspaceID, expectedETag); err != nil {
			return PlatformSettingsProjection{}, err
		}
	case errors.Is(lookupErr, pgx.ErrNoRows):
		if expectedETag != "0" && expectedETag != "1" {
			return PlatformSettingsProjection{}, ErrETagConflict
		}
		if _, err = tx.Exec(ctx, `INSERT INTO gantry.platform_settings(id,organization_id,workspace_id,values,etag) VALUES($1,$2,$3,$4::jsonb,1)`, newID("settings"), actor.OrganizationID, workspaceID, string(req.Values)); err != nil {
			return PlatformSettingsProjection{}, err
		}
	default:
		return PlatformSettingsProjection{}, lookupErr
	}
	if err = tx.Commit(ctx); err != nil {
		return PlatformSettingsProjection{}, err
	}
	return s.GetSettings(ctx, actor, nullableStringValue(req.WorkspaceID))
}

func (s *Service) requireScopeRead(ctx context.Context, actor identity.Principal, workspaceID string) error {
	if workspaceID != "" {
		return s.authz.RequireWorkspace(ctx, actor, workspaceID)
	}
	return s.authz.RequireOrganizationAdmin(ctx, actor)
}

func (s *Service) workspaceInOrganization(ctx context.Context, actor identity.Principal, workspaceID string) error {
	var organizationID string
	if err := s.pool.QueryRow(ctx, `SELECT organization_id FROM gantry.workspaces WHERE id=$1`, workspaceID).Scan(&organizationID); errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	} else if organizationID != actor.OrganizationID {
		return ErrNotFound
	}
	return nil
}

func (s *Service) validateLimitBound(ctx context.Context, organizationID, workspaceID string, req UpsertLimitPolicyRequest) error {
	var bound UpsertLimitPolicyRequest
	err := s.pool.QueryRow(ctx, `SELECT concurrency,duration_seconds,output_bytes,artifact_bytes FROM gantry.platform_limit_policies WHERE organization_id=$1 AND workspace_id IS NULL`, organizationID).Scan(&bound.Concurrency, &bound.DurationSeconds, &bound.OutputBytes, &bound.ArtifactBytes)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if req.Concurrency > bound.Concurrency || req.DurationSeconds > bound.DurationSeconds || req.OutputBytes > bound.OutputBytes || req.ArtifactBytes > bound.ArtifactBytes {
		return ErrInvalidInput
	}
	return nil
}

func (s *Service) validateEnvironmentBound(ctx context.Context, organizationID, workspaceID string, req UpsertEnvironmentProfileRequest) error {
	var posture string
	err := s.pool.QueryRow(ctx, `SELECT publication_posture FROM gantry.platform_environment_profiles WHERE organization_id=$1 AND workspace_id IS NULL AND name=$2`, organizationID, req.Name).Scan(&posture)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if postureRank(req.PublicationPosture) > postureRank(posture) {
		return ErrInvalidInput
	}
	return nil
}

func validEnvironmentName(v string) bool {
	return v == "development" || v == "staging" || v == "production"
}
func validPublicationPosture(v string) bool {
	return v == "test_only" || v == "review_required" || v == "production"
}
func validEnvironmentState(v string) bool {
	return v == "active" || v == "emergency" || v == "disabled"
}
func postureRank(v string) int {
	switch v {
	case "test_only":
		return 0
	case "review_required":
		return 1
	case "production":
		return 2
	default:
		return -1
	}
}
func jsonObject(raw json.RawMessage) bool {
	var value map[string]any
	return len(raw) > 0 && json.Unmarshal(raw, &value) == nil && value != nil
}
func nullableString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
func nullableStringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func (s *Service) providerVisible(ctx context.Context, actor identity.Principal, providerID string) error {
	var ok bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gantry.platform_model_providers WHERE id=$1 AND organization_id=$2)`, providerID, actor.OrganizationID).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}
func validRouteState(v string) bool { return v == "active" || v == "degraded" || v == "disabled" }
func validHandling(v string) bool {
	return v == "public" || v == "internal" || v == "confidential" || v == "restricted"
}
func newID(prefix string) string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(b)
}
