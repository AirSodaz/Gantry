package adminplatform

import "encoding/json"

type ModelProvider struct {
	ID                    string          `json:"id"`
	OrganizationID        string          `json:"organization_id"`
	Name                  string          `json:"name"`
	State                 string          `json:"state"`
	DataClasses           []string        `json:"data_classes"`
	CredentialReferenceID string          `json:"credential_reference_id"`
	Health                json.RawMessage `json:"health"`
}

type ProviderRoute struct {
	ID                        string          `json:"id"`
	ProviderID                string          `json:"provider_id"`
	AllowedModels             []string        `json:"allowed_models"`
	FallbackRouteIDs          []string        `json:"fallback_route_ids"`
	State                     string          `json:"state"`
	BudgetPolicyID            *string         `json:"budget_policy_id"`
	ClassificationConstraints json.RawMessage `json:"classification_constraints"`
	ETag                      string          `json:"etag"`
}

type RunnerPool struct {
	ID                  string          `json:"id"`
	OrganizationID      string          `json:"organization_id"`
	IsolationTier       string          `json:"isolation_tier"`
	State               string          `json:"state"`
	CompatibleProtocols []string        `json:"compatible_protocols"`
	Capacity            json.RawMessage `json:"capacity"`
}

type Runner struct {
	ID              string  `json:"id"`
	PoolID          string  `json:"pool_id"`
	State           string  `json:"state"`
	ProtocolVersion string  `json:"protocol_version"`
	LeaseEpoch      int64   `json:"lease_epoch"`
	LastHeartbeatAt *string `json:"last_heartbeat_at"`
}

type CreateProviderRequest struct {
	Name                  string   `json:"name"`
	DataClasses           []string `json:"data_classes"`
	CredentialReferenceID string   `json:"credential_reference_id"`
}
type PutRouteRequest struct {
	AllowedModels             []string        `json:"allowed_models"`
	FallbackRouteIDs          []string        `json:"fallback_route_ids"`
	State                     string          `json:"state"`
	BudgetPolicyID            *string         `json:"budget_policy_id"`
	ClassificationConstraints json.RawMessage `json:"classification_constraints"`
}
type CreateRunnerPoolRequest struct {
	IsolationTier       string          `json:"isolation_tier"`
	CompatibleProtocols []string        `json:"compatible_protocols"`
	Capacity            json.RawMessage `json:"capacity"`
}
