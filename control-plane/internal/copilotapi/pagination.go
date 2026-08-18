package copilotapi

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/AirSodaz/gantry/internal/approvals"
	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/AirSodaz/gantry/internal/runs"
	"github.com/AirSodaz/gantry/internal/sessions"
)

type sessionListCursorClaims struct {
	Actor     string `json:"actor"`
	Filter    string `json:"filter"`
	CreatedAt string `json:"created_at"`
	ID        string `json:"id"`
}

type runListCursorClaims struct {
	Actor           string `json:"actor"`
	SessionID       string `json:"session_id"`
	SessionSequence int    `json:"attempt"`
	ID              string `json:"id"`
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
	LastUsedAt  string `json:"last_used_at"`
	ID          string `json:"id"`
}
type memberListCursorClaims struct {
	Actor       string `json:"actor"`
	SessionID   string `json:"session_id"`
	JoinedAt    string `json:"joined_at"`
	PrincipalID string `json:"principal_id"`
}

func (h Handler) parseMemberListCursor(raw string, actor identity.Principal, sessionID string) (*sessions.MemberCursor, bool) {
	if strings.TrimSpace(raw) == "" {
		return nil, true
	}
	claims, ok := verifyPayload[memberListCursorClaims](h.eventKey, "session-member-page", raw)
	if !ok || claims.Actor != actor.ID || claims.SessionID != sessionID || claims.PrincipalID == "" {
		return nil, false
	}
	joinedAt, err := time.Parse(time.RFC3339Nano, claims.JoinedAt)
	if err != nil {
		return nil, false
	}
	return &sessions.MemberCursor{JoinedAt: joinedAt, PrincipalID: claims.PrincipalID}, true
}
func (h Handler) encodeMemberListCursor(actor identity.Principal, sessionID string, cursor sessions.MemberCursor) string {
	return signPayload(h.eventKey, "session-member-page", memberListCursorClaims{Actor: actor.ID, SessionID: sessionID, JoinedAt: cursor.JoinedAt.UTC().Format(time.RFC3339Nano), PrincipalID: cursor.PrincipalID})
}

func (h Handler) parseAgentListCursor(raw string, actor identity.Principal, category, search, collection string) (*sessions.AgentCursor, bool) {
	if strings.TrimSpace(raw) == "" {
		return nil, true
	}
	c, ok := verifyPayload[agentListCursorClaims](h.eventKey, "agent-page", raw)
	if !ok || c.Actor != actor.ID || c.Filter != agentListFilterHash(category, search, collection) || c.ID == "" {
		return nil, false
	}
	cursor := &sessions.AgentCursor{DisplayName: c.DisplayName, ID: c.ID}
	if collection == "recent" {
		lastUsedAt, err := time.Parse(time.RFC3339Nano, c.LastUsedAt)
		if err != nil {
			return nil, false
		}
		cursor.LastUsedAt = &lastUsedAt
	}
	return cursor, true
}
func (h Handler) encodeAgentListCursor(actor identity.Principal, category, search, collection string, cursor sessions.AgentCursor) string {
	lastUsedAt := ""
	if cursor.LastUsedAt != nil {
		lastUsedAt = cursor.LastUsedAt.UTC().Format(time.RFC3339Nano)
	}
	return signPayload(h.eventKey, "agent-page", agentListCursorClaims{Actor: actor.ID, Filter: agentListFilterHash(category, search, collection), DisplayName: cursor.DisplayName, LastUsedAt: lastUsedAt, ID: cursor.ID})
}
func agentListFilterHash(category, search, collection string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{strings.TrimSpace(category), strings.TrimSpace(search), strings.TrimSpace(collection)}, "\x00")))
	return hex.EncodeToString(sum[:])
}

func (h Handler) parseArtifactListCursor(raw string, actor identity.Principal, sessionID, classification, state string) (*runs.ArtifactCursor, bool) {
	if strings.TrimSpace(raw) == "" {
		return nil, true
	}
	claims, ok := verifyPayload[artifactListCursorClaims](h.eventKey, "artifact-page", raw)
	if !ok || claims.Actor != actor.ID || claims.Filter != artifactListFilterHash(sessionID, classification, state) {
		return nil, false
	}
	createdAt, err := time.Parse(time.RFC3339Nano, claims.CreatedAt)
	if err != nil || claims.ID == "" {
		return nil, false
	}
	return &runs.ArtifactCursor{CreatedAt: createdAt, ID: claims.ID}, true
}

func (h Handler) encodeArtifactListCursor(actor identity.Principal, sessionID, classification, state string, cursor runs.ArtifactCursor) string {
	return signPayload(h.eventKey, "artifact-page", artifactListCursorClaims{Actor: actor.ID, Filter: artifactListFilterHash(sessionID, classification, state), CreatedAt: cursor.CreatedAt.UTC().Format(time.RFC3339Nano), ID: cursor.ID})
}

func artifactListFilterHash(sessionID, classification, state string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{strings.TrimSpace(sessionID), strings.TrimSpace(classification), strings.TrimSpace(state)}, "\x00")))
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

func (h Handler) parseSessionListCursor(raw string, actor identity.Principal, filter sessions.ListFilter) (*sessions.SessionCursor, bool) {
	if strings.TrimSpace(raw) == "" {
		return nil, true
	}
	claims, ok := verifyPayload[sessionListCursorClaims](h.eventKey, "session-page", raw)
	if !ok || claims.Actor != actor.ID || claims.Filter != sessionListFilterHash(filter) {
		return nil, false
	}
	createdAt, err := time.Parse(time.RFC3339Nano, claims.CreatedAt)
	if err != nil || claims.ID == "" {
		return nil, false
	}
	return &sessions.SessionCursor{CreatedAt: createdAt, ID: claims.ID}, true
}

func (h Handler) encodeSessionListCursor(actor identity.Principal, filter sessions.ListFilter, cursor sessions.SessionCursor) string {
	return signPayload(h.eventKey, "session-page", sessionListCursorClaims{Actor: actor.ID, Filter: sessionListFilterHash(filter), CreatedAt: cursor.CreatedAt.UTC().Format(time.RFC3339Nano), ID: cursor.ID})
}

func sessionListFilterHash(filter sessions.ListFilter) string {
	updatedAfter := ""
	if filter.UpdatedAfter != nil {
		updatedAfter = filter.UpdatedAfter.UTC().Format(time.RFC3339Nano)
	}
	sum := sha256.Sum256([]byte(strings.Join([]string{filter.State, filter.Mode, filter.AgentID, filter.MyAction, updatedAfter}, "\x00")))
	return hex.EncodeToString(sum[:])
}

func (h Handler) parseRunListCursor(raw string, actor identity.Principal, sessionID string) (*sessions.RunCursor, bool) {
	if strings.TrimSpace(raw) == "" {
		return nil, true
	}
	claims, ok := verifyPayload[runListCursorClaims](h.eventKey, "run-page", raw)
	if !ok || claims.Actor != actor.ID || claims.SessionID != sessionID || claims.SessionSequence < 1 || claims.ID == "" {
		return nil, false
	}
	return &sessions.RunCursor{SessionSequence: claims.SessionSequence, ID: claims.ID}, true
}

func (h Handler) encodeRunListCursor(actor identity.Principal, sessionID string, cursor sessions.RunCursor) string {
	return signPayload(h.eventKey, "run-page", runListCursorClaims{Actor: actor.ID, SessionID: sessionID, SessionSequence: cursor.SessionSequence, ID: cursor.ID})
}
