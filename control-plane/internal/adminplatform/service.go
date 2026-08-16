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
