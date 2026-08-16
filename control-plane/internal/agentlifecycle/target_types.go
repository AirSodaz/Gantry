package agentlifecycle

import "encoding/json"

// NamedDraft is one mutable, independently locked Agent working copy.
type NamedDraft struct {
	ID                      string          `json:"id"`
	AgentID                 string          `json:"agent_id"`
	Name                    string          `json:"name"`
	Status                  string          `json:"status"`
	DerivedFromRevisionHash string          `json:"derived_from_revision_hash,omitempty"`
	LatestRevisionHash      string          `json:"latest_revision_hash,omitempty"`
	Spec                    json.RawMessage `json:"spec"`
	SchemaVersion           string          `json:"schema_version"`
	WorkingCopyETag         int             `json:"working_copy_etag"`
	ValidationStatus        string          `json:"validation_status"`
	ValidationFindings      []Finding       `json:"validation_findings"`
	CreatedBy               string          `json:"created_by"`
	UpdatedBy               string          `json:"updated_by"`
	CreatedAt               string          `json:"created_at"`
	UpdatedAt               string          `json:"updated_at"`
}

type Revision struct {
	ID                 string          `json:"id"`
	AgentID            string          `json:"agent_id"`
	RevisionHash       string          `json:"revision_hash"`
	SourceDraftID      string          `json:"source_draft_id"`
	Message            string          `json:"message"`
	Spec               json.RawMessage `json:"spec"`
	SpecDigest         string          `json:"spec_digest"`
	RuntimeImageDigest string          `json:"runtime_image_digest,omitempty"`
	CreatedAt          string          `json:"created_at"`
	CreatedBy          string          `json:"created_by"`
	PromptSnapshot     PromptSnapshot  `json:"prompt_snapshot"`
}

type Deployment struct {
	ID                   string          `json:"id"`
	AgentID              string          `json:"agent_id"`
	WorkspaceID          string          `json:"workspace_id"`
	Name                 string          `json:"name"`
	EnvironmentKind      string          `json:"environment_kind"`
	RevisionID           string          `json:"revision_id"`
	RevisionHash         string          `json:"revision_hash"`
	SpecDigest           string          `json:"spec_digest"`
	Status               string          `json:"status"`
	Owner                string          `json:"owner,omitempty"`
	Purpose              string          `json:"purpose,omitempty"`
	ExpiresAt            string          `json:"expires_at,omitempty"`
	EnvironmentPolicy    json.RawMessage `json:"environment_policy"`
	ChangedBy            string          `json:"changed_by"`
	ReviewID             string          `json:"review_id,omitempty"`
	PreviousRevisionHash string          `json:"previous_revision_hash,omitempty"`
	CreatedAt            string          `json:"created_at"`
	UpdatedAt            string          `json:"updated_at"`
}

type RevisionReview struct {
	ID               string      `json:"id,omitempty"`
	AgentID          string      `json:"agent_id"`
	RevisionHash     string      `json:"revision_hash"`
	BaseRevisionHash string      `json:"base_revision_hash,omitempty"`
	ReleaseNotes     string      `json:"release_notes"`
	Diff             []DiffEntry `json:"diff"`
	RiskSummary      DiffSummary `json:"risk_summary"`
	Status           string      `json:"status"`
	SubmittedBy      string      `json:"submitted_by,omitempty"`
	ReviewedBy       string      `json:"reviewed_by,omitempty"`
	ReviewReason     string      `json:"review_reason,omitempty"`
	SubmittedAt      string      `json:"submitted_at,omitempty"`
	ReviewedAt       string      `json:"reviewed_at,omitempty"`
}

type CreateDraftRequest struct {
	Name             string `json:"name"`
	FromRevisionHash string `json:"from_revision_hash,omitempty"`
}

type CommitDraftRequest struct {
	Message string `json:"message"`
}

type CreateDeploymentRequest struct {
	Name              string          `json:"name"`
	RevisionHash      string          `json:"revision_hash"`
	Purpose           string          `json:"purpose,omitempty"`
	ExpiresAt         string          `json:"expires_at,omitempty"`
	EnvironmentPolicy json.RawMessage `json:"environment_policy,omitempty"`
}

type PublishRevisionRequest struct {
	RevisionHash                   string `json:"revision_hash"`
	ExpectedProductionRevisionHash string `json:"expected_production_revision_hash,omitempty"`
}

type AgentTargetOverview struct {
	Agent                Agent          `json:"agent"`
	MainDraft            NamedDraft     `json:"main_draft"`
	Drafts               []NamedDraft   `json:"drafts"`
	ProductionDeployment *Deployment    `json:"production_deployment,omitempty"`
	TestDeployments      []Deployment   `json:"test_deployments"`
	RevisionCount        int            `json:"revision_count"`
	RecentActivity       []ActivityItem `json:"recent_activity"`
}
