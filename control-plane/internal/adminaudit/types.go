// Package adminaudit owns the scope-authorized Admin audit projection.
package adminaudit

import "encoding/json"

type ListOptions struct {
	WorkspaceID     string
	ResourceType    string
	ResourceID      string
	ActorID         string
	EventType       string
	Outcome         string
	Risk            string
	CorrelationID   string
	RunID           string
	RevisionHash    string
	PolicyVersionID string
	Before          string
	After           string
	Limit           int
	Cursor          string
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
