// Package adminpolicy owns the typed, scope-authorized Admin Policy resource.
package adminpolicy

import "encoding/json"

type ListOptions struct {
	Type          string
	WorkspaceID   string
	State         string
	OwnerID       string
	BindingTarget string
	Cursor        string
	Limit         int
}

type PageInfo struct {
	NextCursor *string `json:"next_cursor"`
}

type ListResult struct {
	Items    []Policy `json:"items"`
	PageInfo PageInfo `json:"page_info"`
}

type Policy struct {
	ID                 string  `json:"id"`
	OrganizationID     string  `json:"organization_id"`
	WorkspaceID        *string `json:"workspace_id"`
	Type               string  `json:"type"`
	Name               string  `json:"name"`
	OwnerPrincipalID   string  `json:"owner_principal_id"`
	State              string  `json:"state"`
	SchemaVersion      string  `json:"schema_version"`
	DraftETag          string  `json:"draft_etag"`
	LatestVersionID    *string `json:"latest_version_id"`
	ActiveBindingCount int     `json:"active_binding_count"`
}

type Draft struct {
	PolicyID      string          `json:"policy_id"`
	Document      json.RawMessage `json:"document"`
	SchemaVersion string          `json:"schema_version"`
	ETag          string          `json:"etag"`
	Validation    Validation      `json:"validation"`
}

type Validation struct {
	State    string           `json:"state"`
	Findings []map[string]any `json:"findings"`
}

type Version struct {
	ID               string          `json:"id"`
	PolicyID         string          `json:"policy_id"`
	ContentDigest    string          `json:"content_digest"`
	SchemaVersion    string          `json:"schema_version"`
	Message          string          `json:"message"`
	Document         json.RawMessage `json:"document"`
	CompilerEvidence map[string]any  `json:"compiler_evidence"`
	CreatedBy        string          `json:"created_by"`
	CreatedAt        string          `json:"created_at"`
}

type Binding struct {
	ID               string   `json:"id"`
	VersionID        string   `json:"version_id"`
	Target           ScopeRef `json:"target"`
	TargetResourceID *string  `json:"target_resource_id"`
	Environment      string   `json:"environment"`
	State            string   `json:"state"`
	EffectiveFrom    string   `json:"effective_from"`
	EffectiveUntil   *string  `json:"effective_until"`
	Reason           string   `json:"reason"`
}

type ScopeRef struct {
	OrganizationID string  `json:"organization_id"`
	WorkspaceID    *string `json:"workspace_id,omitempty"`
	Scope          string  `json:"scope"`
}

type CreateRequest struct {
	WorkspaceID   string          `json:"workspace_id"`
	Type          string          `json:"type"`
	Name          string          `json:"name"`
	SchemaVersion string          `json:"schema_version"`
	Document      json.RawMessage `json:"document"`
}

type UpdateDraftRequest struct {
	Document      json.RawMessage `json:"document"`
	SchemaVersion string          `json:"schema_version"`
}

type PublishRequest struct {
	Message string `json:"message"`
}

type BindRequest struct {
	VersionID        string `json:"version_id"`
	Scope            string `json:"scope"`
	WorkspaceID      string `json:"workspace_id"`
	TargetResourceID string `json:"target_resource_id"`
	Environment      string `json:"environment"`
	Reason           string `json:"reason"`
}

type SimulationRequest struct {
	VersionID string         `json:"version_id"`
	Action    map[string]any `json:"action"`
}

type Simulation struct {
	Decision             string           `json:"decision"`
	ContributingVersions []map[string]any `json:"contributing_versions"`
	IneffectiveRules     []map[string]any `json:"ineffective_rules"`
	Explanation          string           `json:"explanation"`
}
