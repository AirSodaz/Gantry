// Package adminevaluation owns the typed Admin Evaluation Suite boundary.
package adminevaluation

import "encoding/json"

type ListOptions struct {
	WorkspaceID string
	State       string
	Search      string
	Limit       int
}

type PageInfo struct {
	NextCursor *string `json:"next_cursor"`
}

type SuiteList struct {
	Items    []Suite  `json:"items"`
	PageInfo PageInfo `json:"page_info"`
}

type Suite struct {
	ID               string  `json:"id"`
	OrganizationID   string  `json:"organization_id"`
	WorkspaceID      string  `json:"workspace_id"`
	Name             string  `json:"name"`
	State            string  `json:"state"`
	OwnerPrincipalID string  `json:"owner_principal_id"`
	LatestVersionID  *string `json:"latest_version_id"`
	GateUsageCount   int     `json:"gate_usage_count"`
	ETag             string  `json:"etag"`
}

type Case struct {
	ID              string          `json:"id"`
	SuiteID         string          `json:"suite_id"`
	Input           json.RawMessage `json:"input"`
	FixtureManifest json.RawMessage `json:"fixture_manifest"`
	Assertions      json.RawMessage `json:"assertions"`
	Rubric          json.RawMessage `json:"rubric,omitempty"`
	Compatibility   json.RawMessage `json:"compatibility"`
	ETag            string          `json:"etag"`
}

type Validation struct {
	State    string           `json:"state"`
	Findings []map[string]any `json:"findings"`
}

type Version struct {
	ID                       string `json:"id"`
	SuiteID                  string `json:"suite_id"`
	ContentDigest            string `json:"content_digest"`
	CaseManifestDigest       string `json:"case_manifest_digest"`
	FixtureManifestDigest    string `json:"fixture_manifest_digest"`
	EvaluatorPolicyVersionID string `json:"evaluator_policy_version_id,omitempty"`
	RuntimeImageDigest       string `json:"runtime_image_digest"`
	PublishedAt              string `json:"published_at"`
}

type Run struct {
	ID                     string          `json:"id"`
	SuiteVersionID         string          `json:"suite_version_id"`
	CandidateRevisionHash  string          `json:"candidate_revision_hash"`
	BaselineRevisionHash   *string         `json:"baseline_revision_hash"`
	EnvironmentDigest      string          `json:"environment_digest"`
	State                  string          `json:"state"`
	GateResult             string          `json:"gate_result"`
	DeterministicSummary   json.RawMessage `json:"deterministic_summary"`
	ProbabilisticSummary   json.RawMessage `json:"probabilistic_summary,omitempty"`
	EvidenceManifestDigest *string         `json:"evidence_manifest_digest"`
	CreatedAt              string          `json:"created_at"`
}

type CreateSuiteRequest struct {
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
}

type PatchSuiteRequest struct {
	Name string `json:"name"`
}

type CreateCaseRequest struct {
	Input           json.RawMessage `json:"input"`
	FixtureManifest json.RawMessage `json:"fixture_manifest"`
	Assertions      json.RawMessage `json:"assertions"`
	Rubric          json.RawMessage `json:"rubric,omitempty"`
	Compatibility   json.RawMessage `json:"compatibility"`
}

type PatchCaseRequest = CreateCaseRequest

type PublishVersionRequest struct {
	EvaluatorPolicyVersionID string `json:"evaluator_policy_version_id"`
	RuntimeImageDigest       string `json:"runtime_image_digest"`
}

type CreateRunRequest struct {
	SuiteVersionID        string  `json:"suite_version_id"`
	CandidateRevisionHash string  `json:"candidate_revision_hash"`
	BaselineRevisionHash  *string `json:"baseline_revision_hash"`
	EnvironmentDigest     string  `json:"environment_digest"`
}

type Gate struct {
	ID                string          `json:"id"`
	AgentRevisionHash string          `json:"agent_revision_hash"`
	SuiteVersionID    string          `json:"suite_version_id"`
	Requirement       json.RawMessage `json:"requirement"`
	State             string          `json:"state"`
	OverrideID        *string         `json:"override_id"`
}

type GateOverride struct {
	ID                  string `json:"id"`
	GateID              string `json:"gate_id"`
	Reason              string `json:"reason"`
	ReviewerPrincipalID string `json:"reviewer_principal_id"`
	ExpiresAt           string `json:"expires_at"`
}
