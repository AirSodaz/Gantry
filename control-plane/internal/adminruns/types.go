// Package adminruns owns the authorized operational projection of durable runs.
package adminruns

import (
	"encoding/json"
)

type ListOptions struct {
	WorkspaceID  string
	AgentID      string
	RevisionHash string
	Status       string
	Limit        int
}

type Run struct {
	ID             string `json:"id"`
	TaskID         string `json:"task_id"`
	WorkspaceID    string `json:"workspace_id"`
	WorkspaceName  string `json:"workspace_name"`
	AgentID        string `json:"agent_id"`
	AgentName      string `json:"agent_name"`
	RevisionHash   string `json:"revision_hash"`
	DeploymentID   string `json:"deployment_id,omitempty"`
	DeploymentName string `json:"deployment_name,omitempty"`
	RequesterID    string `json:"requester_id"`
	RequesterName  string `json:"requester_name"`
	Status         string `json:"status"`
	StatusReason   string `json:"status_reason,omitempty"`
	RunnerID       string `json:"runner_id,omitempty"`
	AttemptNumber  int    `json:"attempt_number"`
	ManifestDigest string `json:"manifest_digest,omitempty"`
	ActionCount    int    `json:"action_count"`
	ApprovalCount  int    `json:"approval_count"`
	CreatedAt      string `json:"created_at"`
	StartedAt      string `json:"started_at,omitempty"`
	CompletedAt    string `json:"completed_at,omitempty"`
	LastEventAt    string `json:"last_event_at,omitempty"`
}

type Event struct {
	Sequence  uint64          `json:"sequence"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	CreatedAt string          `json:"created_at"`
}

type Action struct {
	ID           string `json:"id"`
	ToolName     string `json:"tool_name"`
	Operation    string `json:"operation"`
	Target       string `json:"target,omitempty"`
	Effect       string `json:"effect"`
	State        string `json:"state"`
	ActionDigest string `json:"action_digest"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type Approval struct {
	ID           string `json:"id"`
	ActionID     string `json:"action_id"`
	ActionDigest string `json:"action_digest"`
	RiskClass    string `json:"risk_class"`
	Status       string `json:"status"`
	RequestedBy  string `json:"requested_by"`
	ExpiresAt    string `json:"expires_at"`
	CreatedAt    string `json:"created_at"`
	DecidedAt    string `json:"decided_at,omitempty"`
}

type Artifact struct {
	ID             string `json:"id"`
	Filename       string `json:"filename"`
	MediaType      string `json:"media_type"`
	SizeBytes      int64  `json:"size_bytes"`
	Digest         string `json:"digest"`
	Classification string `json:"classification"`
	ScanStatus     string `json:"scan_status"`
	State          string `json:"state"`
	CreatedAt      string `json:"created_at"`
}

type Detail struct {
	Run       Run        `json:"run"`
	Events    []Event    `json:"events"`
	Actions   []Action   `json:"actions"`
	Approvals []Approval `json:"approvals"`
	Artifacts []Artifact `json:"artifacts"`
}
