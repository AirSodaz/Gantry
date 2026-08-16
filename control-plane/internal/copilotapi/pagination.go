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

type runListCursorClaims struct {
	Actor   string `json:"actor"`
	TaskID  string `json:"task_id"`
	Attempt int    `json:"attempt"`
	ID      string `json:"id"`
}

type approvalListCursorClaims struct {
	Actor     string `json:"actor"`
	Filter    string `json:"filter"`
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

func (h Handler) parseArtifactListCursor(raw string, actor identity.Principal, taskID, classification, state string) (*tasks.ArtifactCursor, bool) {
	if strings.TrimSpace(raw) == "" {
		return nil, true
	}
	claims, ok := verifyPayload[artifactListCursorClaims](h.eventKey, "artifact-page", raw)
	if !ok || claims.Actor != actor.ID || claims.Filter != artifactListFilterHash(taskID, classification, state) {
		return nil, false
	}
	createdAt, err := time.Parse(time.RFC3339Nano, claims.CreatedAt)
	if err != nil || claims.ID == "" {
		return nil, false
	}
	return &tasks.ArtifactCursor{CreatedAt: createdAt, ID: claims.ID}, true
}

func (h Handler) encodeArtifactListCursor(actor identity.Principal, taskID, classification, state string, cursor tasks.ArtifactCursor) string {
	return signPayload(h.eventKey, "artifact-page", artifactListCursorClaims{Actor: actor.ID, Filter: artifactListFilterHash(taskID, classification, state), CreatedAt: cursor.CreatedAt.UTC().Format(time.RFC3339Nano), ID: cursor.ID})
}

func artifactListFilterHash(taskID, classification, state string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{strings.TrimSpace(taskID), strings.TrimSpace(classification), strings.TrimSpace(state)}, "\x00")))
	return hex.EncodeToString(sum[:])
}

func (h Handler) parseApprovalListCursor(raw string, actor identity.Principal, state string) (*approvals.Cursor, bool) {
	if strings.TrimSpace(raw) == "" {
		return nil, true
	}
	claims, ok := verifyPayload[approvalListCursorClaims](h.eventKey, "approval-page", raw)
	if !ok || claims.Actor != actor.ID || claims.Filter != approvalListFilterHash(state) {
		return nil, false
	}
	createdAt, err := time.Parse(time.RFC3339Nano, claims.CreatedAt)
	if err != nil || claims.ID == "" {
		return nil, false
	}
	return &approvals.Cursor{CreatedAt: createdAt, ID: claims.ID}, true
}

func (h Handler) encodeApprovalListCursor(actor identity.Principal, state string, cursor approvals.Cursor) string {
	return signPayload(h.eventKey, "approval-page", approvalListCursorClaims{Actor: actor.ID, Filter: approvalListFilterHash(state), CreatedAt: cursor.CreatedAt.UTC().Format(time.RFC3339Nano), ID: cursor.ID})
}

func approvalListFilterHash(state string) string {
	if strings.TrimSpace(state) == "" {
		state = "pending"
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(state)))
	return hex.EncodeToString(sum[:])
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

func (h Handler) parseRunListCursor(raw string, actor identity.Principal, taskID string) (*tasks.RunCursor, bool) {
	if strings.TrimSpace(raw) == "" {
		return nil, true
	}
	claims, ok := verifyPayload[runListCursorClaims](h.eventKey, "run-page", raw)
	if !ok || claims.Actor != actor.ID || claims.TaskID != taskID || claims.Attempt < 1 || claims.ID == "" {
		return nil, false
	}
	return &tasks.RunCursor{Attempt: claims.Attempt, ID: claims.ID}, true
}

func (h Handler) encodeRunListCursor(actor identity.Principal, taskID string, cursor tasks.RunCursor) string {
	return signPayload(h.eventKey, "run-page", runListCursorClaims{Actor: actor.ID, TaskID: taskID, Attempt: cursor.Attempt, ID: cursor.ID})
}
