// Package adminaudit owns the scope-authorized Admin audit projection.
package adminaudit

import "encoding/json"

type ListOptions struct {
	WorkspaceID     string `json:"workspace_id,omitempty"`
	ResourceType    string `json:"resource_type,omitempty"`
	ResourceID      string `json:"resource_id,omitempty"`
	ActorID         string `json:"actor_id,omitempty"`
	EventType       string `json:"event_type,omitempty"`
	Outcome         string `json:"outcome,omitempty"`
	Risk            string `json:"risk,omitempty"`
	CorrelationID   string `json:"correlation_id,omitempty"`
	RunID           string `json:"run_id,omitempty"`
	RevisionHash    string `json:"revision_hash,omitempty"`
	PolicyVersionID string `json:"policy_version_id,omitempty"`
	Before          string `json:"before,omitempty"`
	After           string `json:"after,omitempty"`
	Limit           int    `json:"limit,omitempty"`
	Cursor          string `json:"cursor,omitempty"`
}

type Event struct {
	ID              int64  `json:"id"`
	ActorID         string `json:"actor_id"`
	ActorName       string `json:"actor_name"`
	ResourceType    string `json:"resource_type"`
	ResourceID      string `json:"resource_id"`
	EventType       string `json:"event_type"`
	Scope           string `json:"scope"`
	Outcome         string `json:"outcome"`
	Risk            string `json:"risk"`
	CorrelationID   string `json:"correlation_id"`
	RunID           string `json:"run_id"`
	RevisionHash    string `json:"revision_hash"`
	PolicyVersionID string `json:"policy_version_id"`
	CreatedAt       string `json:"created_at"`
}

type PageInfo struct {
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor,omitempty"`
}

type ListResult struct {
	Items    []Event  `json:"items"`
	PageInfo PageInfo `json:"page_info"`
}

type Evidence struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

type RedactionMetadata struct {
	Mode           string   `json:"mode"`
	RedactedFields []string `json:"redacted_fields"`
}

type Detail struct {
	Event
	Payload           json.RawMessage   `json:"payload"`
	Evidence          []Evidence        `json:"evidence"`
	RedactionMetadata RedactionMetadata `json:"redaction_metadata"`
}

type Export struct {
	ID            string `json:"id"`
	QueryDigest   string `json:"query_digest"`
	Scope         string `json:"scope"`
	State         string `json:"state"`
	PackageDigest string `json:"package_digest"`
	DownloadCount int    `json:"download_count"`
	ExpiresAt     string `json:"expires_at,omitempty"`
	FailureReason string `json:"failure_reason,omitempty"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type Download struct {
	URL       string `json:"url"`
	ExpiresAt string `json:"expires_at"`
}
