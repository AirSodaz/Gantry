package copilotapi

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/AirSodaz/gantry/internal/approvals"
	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/AirSodaz/gantry/internal/tasks"
)

type taskListCursorClaims struct {
	Actor     string `json:"actor"`
	Filter    string `json:"filter"`
	CreatedAt string `json:"created_at"`
	ID        string `json:"id"`
}

type approvalListCursorClaims struct {
	Actor     string `json:"actor"`
	CreatedAt string `json:"created_at"`
	ID        string `json:"id"`
}

type artifactListCursorClaims struct {
	Actor     string `json:"actor"`
	Filter    string `json:"filter"`
	CreatedAt string `json:"created_at"`
	ID        string `json:"id"`
}
type agentListCursorClaims struct {
	Actor       string `json:"actor"`
	Filter      string `json:"filter"`
	DisplayName string `json:"display_name"`
	ID          string `json:"id"`
}

func (h Handler) parseAgentListCursor(raw string, actor identity.Principal, category, search string) (*tasks.AgentCursor, bool) {
	if strings.TrimSpace(raw) == "" {
		return nil, true
	}
	c, ok := verifyPayload[agentListCursorClaims](h.eventKey, "agent-page", raw)
	if !ok || c.Actor != actor.ID || c.Filter != agentListFilterHash(category, search) || c.ID == "" {
		return nil, false
	}
	return &tasks.AgentCursor{DisplayName: c.DisplayName, ID: c.ID}, true
}
func (h Handler) encodeAgentListCursor(actor identity.Principal, category, search string, cursor tasks.AgentCursor) string {
	return signPayload(h.eventKey, "agent-page", agentListCursorClaims{Actor: actor.ID, Filter: agentListFilterHash(category, search), DisplayName: cursor.DisplayName, ID: cursor.ID})
}
func agentListFilterHash(category, search string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{strings.TrimSpace(category), strings.TrimSpace(search)}, "\x00")))
	return hex.EncodeToString(sum[:])
}

func (h Handler) parseArtifactListCursor(raw string, actor identity.Principal, taskID, classification string) (*tasks.ArtifactCursor, bool) {
	if strings.TrimSpace(raw) == "" {
		return nil, true
	}
	claims, ok := verifyPayload[artifactListCursorClaims](h.eventKey, "artifact-page", raw)
	if !ok || claims.Actor != actor.ID || claims.Filter != artifactListFilterHash(taskID, classification) {
		return nil, false
	}
	createdAt, err := time.Parse(time.RFC3339Nano, claims.CreatedAt)
	if err != nil || claims.ID == "" {
		return nil, false
	}
	return &tasks.ArtifactCursor{CreatedAt: createdAt, ID: claims.ID}, true
}

func (h Handler) encodeArtifactListCursor(actor identity.Principal, taskID, classification string, cursor tasks.ArtifactCursor) string {
	return signPayload(h.eventKey, "artifact-page", artifactListCursorClaims{Actor: actor.ID, Filter: artifactListFilterHash(taskID, classification), CreatedAt: cursor.CreatedAt.UTC().Format(time.RFC3339Nano), ID: cursor.ID})
}

func artifactListFilterHash(taskID, classification string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{strings.TrimSpace(taskID), strings.TrimSpace(classification)}, "\x00")))
	return hex.EncodeToString(sum[:])
}

func (h Handler) parseApprovalListCursor(raw string, actor identity.Principal) (*approvals.Cursor, bool) {
	if strings.TrimSpace(raw) == "" {
		return nil, true
	}
	claims, ok := verifyPayload[approvalListCursorClaims](h.eventKey, "approval-page", raw)
	if !ok || claims.Actor != actor.ID {
		return nil, false
	}
	createdAt, err := time.Parse(time.RFC3339Nano, claims.CreatedAt)
	if err != nil || claims.ID == "" {
		return nil, false
	}
	return &approvals.Cursor{CreatedAt: createdAt, ID: claims.ID}, true
}

func (h Handler) encodeApprovalListCursor(actor identity.Principal, cursor approvals.Cursor) string {
	return signPayload(h.eventKey, "approval-page", approvalListCursorClaims{Actor: actor.ID, CreatedAt: cursor.CreatedAt.UTC().Format(time.RFC3339Nano), ID: cursor.ID})
}

func (h Handler) parseTaskListCursor(raw string, actor identity.Principal, filter tasks.ListFilter) (*tasks.TaskCursor, bool) {
	if strings.TrimSpace(raw) == "" {
		return nil, true
	}
	claims, ok := verifyPayload[taskListCursorClaims](h.eventKey, "task-page", raw)
	if !ok || claims.Actor != actor.ID || claims.Filter != taskListFilterHash(filter) {
		return nil, false
	}
	createdAt, err := time.Parse(time.RFC3339Nano, claims.CreatedAt)
	if err != nil || claims.ID == "" {
		return nil, false
	}
	return &tasks.TaskCursor{CreatedAt: createdAt, ID: claims.ID}, true
}

func (h Handler) encodeTaskListCursor(actor identity.Principal, filter tasks.ListFilter, cursor tasks.TaskCursor) string {
	return signPayload(h.eventKey, "task-page", taskListCursorClaims{Actor: actor.ID, Filter: taskListFilterHash(filter), CreatedAt: cursor.CreatedAt.UTC().Format(time.RFC3339Nano), ID: cursor.ID})
}

func taskListFilterHash(filter tasks.ListFilter) string {
	createdAfter := ""
	if filter.CreatedAfter != nil {
		createdAfter = filter.CreatedAfter.UTC().Format(time.RFC3339Nano)
	}
	sum := sha256.Sum256([]byte(strings.Join([]string{filter.Status, filter.AgentID, filter.RequesterAction, createdAfter}, "\x00")))
	return hex.EncodeToString(sum[:])
}
