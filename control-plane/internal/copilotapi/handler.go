package copilotapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/AirSodaz/gantry/internal/approvals"
	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/AirSodaz/gantry/internal/runs"
	"github.com/AirSodaz/gantry/internal/sessions"
)

type authenticator interface {
	Authenticate(context.Context, string) (identity.Principal, error)
}

// sessionService is the Copilot application's transport-facing use-case boundary.
// The concrete PostgreSQL service remains in the sessions package.
type sessionService interface {
	ListAgents(context.Context, identity.Principal, string, string, string, *sessions.AgentCursor, int) (sessions.AgentPage, error)
	SetAgentFavorite(context.Context, identity.Principal, string, string, sessions.SetAgentFavoriteRequest) (sessions.Agent, error)
	Submit(context.Context, identity.Principal, string, sessions.SubmitRequest) (sessions.Session, bool, error)
	List(context.Context, identity.Principal, sessions.ListFilter, *sessions.SessionCursor, int) (sessions.SessionPage, error)
	Get(context.Context, identity.Principal, string) (sessions.Session, error)
	AppendMessage(context.Context, identity.Principal, string, string, int64, sessions.AppendMessageRequest) (sessions.Session, bool, error)
	ListRuns(context.Context, identity.Principal, string, *sessions.RunCursor, int) (sessions.RunPage, error)
	Cancel(context.Context, identity.Principal, string, string, string) (sessions.CancelResult, error)
	Retry(context.Context, identity.Principal, string, bool, string, int64) (sessions.RetryResult, error)
	RetryRun(context.Context, identity.Principal, string, string, bool, string, int64) (sessions.RetryResult, error)
}

type eventReader interface {
	Events(context.Context, identity.Principal, string, uint64, int) (sessions.EventPage, error)
}

type memberService interface {
	ListMembers(context.Context, identity.Principal, string, *sessions.MemberCursor, int) (sessions.MemberPage, error)
	AddMember(context.Context, identity.Principal, string, string, int64, sessions.AddMemberRequest) (sessions.Session, error)
	UpdateMember(context.Context, identity.Principal, string, string, string, int64, sessions.UpdateMemberRequest) (sessions.Session, error)
	RemoveMember(context.Context, identity.Principal, string, string, string, int64) (sessions.Session, error)
	TransferOwner(context.Context, identity.Principal, string, string, int64, sessions.TransferOwnerRequest) (sessions.Session, error)
	Archive(context.Context, identity.Principal, string, string, int64) (sessions.Session, error)
}

type artifactService interface {
	GetArtifact(context.Context, identity.Principal, string) (runs.Artifact, error)
	ListMyArtifacts(context.Context, identity.Principal, string, string, string, *runs.ArtifactCursor, int) (runs.ArtifactPage, error)
	DownloadArtifact(context.Context, identity.Principal, string) (runs.ArtifactDownloadGrant, error)
}

type dispatcher interface {
	Dispatch(context.Context) error
	RequestCancel(string, uint64, string) bool
	ResolveApproval(string, string, string, string, string, string, string, uint64, time.Time) bool
}

type approvalService interface {
	List(context.Context, identity.Principal, string, *approvals.Cursor, int) (approvals.Page, error)
	Get(context.Context, identity.Principal, string) (approvals.Request, error)
	Expire(context.Context, identity.Principal) ([]approvals.Resolution, error)
	Decide(context.Context, identity.Principal, approvals.DecisionInput) (approvals.Resolution, error)
}

type Handler struct {
	auth       authenticator
	sessions   sessionService
	artifacts  artifactService
	approvals  approvalService
	dispatcher dispatcher
	logger     *slog.Logger
	eventKey   []byte
}

var problemSequence atomic.Uint64

type copilotProblem struct {
	Code              string `json:"code"`
	Message           string `json:"message"`
	CorrelationID     string `json:"correlation_id"`
	Retryable         bool   `json:"retryable"`
	RetryAfterSeconds *int   `json:"retry_after_seconds"`
	CurrentResource   any    `json:"current_resource"`
}

func New(auth authenticator, sessionService sessionService, approvalService approvalService, dispatcher dispatcher, logger *slog.Logger, artifactServices ...artifactService) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	h := Handler{auth: auth, sessions: sessionService, approvals: approvalService, dispatcher: dispatcher, logger: logger, eventKey: loadEventTicketKey()}
	if len(artifactServices) > 0 {
		h.artifacts = artifactServices[0]
	}
	mux := http.NewServeMux()
	mux.Handle("GET /agents", h.withActor(h.listAgents))
	mux.Handle("PUT /agents/{agentID}/favorite", h.withActor(h.setAgentFavorite))
	mux.Handle("POST /sessions", h.withActor(h.submitSession))
	mux.Handle("GET /sessions", h.withActor(h.listSessions))
	mux.Handle("GET /sessions/{sessionID}", h.withActor(h.getSession))
	mux.Handle("POST /sessions/{sessionID}/messages", h.withActor(h.appendMessage))
	mux.Handle("GET /sessions/{sessionID}/members", h.withActor(h.listMembers))
	mux.Handle("POST /sessions/{sessionID}/members", h.withActor(h.addMember))
	mux.Handle("PATCH /sessions/{sessionID}/members/{principalID}", h.withActor(h.updateMember))
	mux.Handle("DELETE /sessions/{sessionID}/members/{principalID}", h.withActor(h.removeMember))
	mux.Handle("GET /sessions/{sessionID}/runs", h.withActor(h.listRuns))
	mux.Handle("POST /sessions/{sessionID}/events:ticket", h.withActor(h.issueEventTicket))
	mux.HandleFunc("GET /sessions/{sessionID}/events", h.events)
	mux.Handle("GET /artifacts/{artifactID}", h.withActor(h.getArtifact))
	mux.Handle("POST /artifacts/{operation...}", h.withActor(h.downloadArtifact))
	mux.Handle("GET /artifacts", h.withActor(h.listArtifacts))
	mux.Handle("POST /attachments", h.withActor(h.createAttachment))
	mux.Handle("GET /attachments/{attachmentID}", h.withActor(h.getAttachment))
	mux.Handle("PUT /attachments/{attachmentID}/content", h.withActor(h.uploadAttachment))
	mux.Handle("POST /attachments/{operation...}", h.withActor(h.completeAttachment))
	mux.Handle("GET /approvals", h.withActor(h.listApprovals))
	mux.Handle("GET /approvals/{approvalID}", h.withActor(h.getApproval))
	mux.Handle("POST /approvals/{operation...}", h.withActor(h.decideApproval))
	mux.Handle("POST /sessions/{sessionID}/runs/{operation...}", h.withActor(h.cancelOperation))
	mux.Handle("POST /sessions/{operation...}", h.withActor(h.retryOperation))
	return mux
}

type actorHandler func(http.ResponseWriter, *http.Request, identity.Principal)

func (h Handler) withActor(next actorHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor, err := h.auth.Authenticate(r.Context(), r.Header.Get("Authorization"))
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "A valid Copilot access token is required.")
			return
		}
		next(w, r, actor)
	})
}

func (h Handler) listAgents(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	category, search, collection := r.URL.Query().Get("category"), r.URL.Query().Get("search"), r.URL.Query().Get("collection")
	if collection == "" {
		collection = "all"
	}
	if collection != "all" && collection != "favorites" && collection != "recent" {
		writeError(w, http.StatusBadRequest, "invalid_request", "collection must be all, favorites, or recent.")
		return
	}
	after, ok := h.parseAgentListCursor(r.URL.Query().Get("cursor"), actor, category, search, collection)
	if !ok {
		writeError(w, http.StatusBadRequest, "cursor_invalid", "The agent cursor is not valid for this requester or filter.")
		return
	}
	page, err := h.sessions.ListAgents(r.Context(), actor, category, search, collection, after, limit(r))
	if err != nil {
		writeInternal(w, err)
		return
	}
	info := map[string]any{"has_more": page.HasMore}
	if page.HasMore {
		last := page.Items[len(page.Items)-1]
		info["next_cursor"] = h.encodeAgentListCursor(actor, category, search, collection, sessions.AgentCursor{DisplayName: last.DisplayName, LastUsedAt: last.LastUsedAt, ID: last.ID})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": page.Items, "page_info": info})
}

func (h Handler) setAgentFavorite(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var request sessions.SetAgentFavoriteRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, err := h.sessions.SetAgentFavorite(r.Context(), actor, r.PathValue("agentID"), r.Header.Get("Idempotency-Key"), request)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h Handler) submitSession(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var request sessions.SubmitRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	session, duplicate, err := h.sessions.Submit(r.Context(), actor, r.Header.Get("Idempotency-Key"), request)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	if err := h.dispatcher.Dispatch(r.Context()); err != nil {
		h.logger.Error("queued session dispatch failed", "error", err, "session_id", session.ID)
	}
	status := http.StatusCreated
	if duplicate {
		status = http.StatusOK
	}
	w.Header().Set("ETag", session.ConversationETag())
	writeJSON(w, status, session)
}
func (h Handler) listSessions(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	filter := sessions.ListFilter{State: r.URL.Query().Get("state"), Mode: r.URL.Query().Get("mode"), AgentID: r.URL.Query().Get("agent_id"), MyAction: r.URL.Query().Get("my_action")}
	if value := r.URL.Query().Get("updated_after"); value != "" {
		updatedAfter, err := time.Parse(time.RFC3339, value)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "updated_after must be an RFC 3339 timestamp.")
			return
		}
		filter.UpdatedAfter = &updatedAfter
	}
	after, ok := h.parseSessionListCursor(r.URL.Query().Get("cursor"), actor, filter)
	if !ok {
		writeError(w, http.StatusBadRequest, "cursor_invalid", "The Session cursor is not valid for this member or filter.")
		return
	}
	page, err := h.sessions.List(r.Context(), actor, filter, after, limit(r))
	if err != nil {
		writeInternal(w, err)
		return
	}
	pageInfo := map[string]any{"has_more": page.HasMore}
	if page.HasMore {
		last := page.Items[len(page.Items)-1]
		pageInfo["next_cursor"] = h.encodeSessionListCursor(actor, filter, sessions.SessionCursor{UpdatedAt: last.UpdatedAt, ID: last.ID})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": page.Items, "page_info": pageInfo})
}
func (h Handler) getSession(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	session, err := h.sessions.Get(r.Context(), actor, r.PathValue("sessionID"))
	if err != nil {
		writeSessionError(w, err)
		return
	}
	w.Header().Set("ETag", session.ConversationETag())
	writeJSON(w, http.StatusOK, session)
}
func (h Handler) appendMessage(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var request sessions.AppendMessageRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	expectedRevision, err := parseConversationETag(r.Header.Get("If-Match"))
	if err != nil {
		writeError(w, http.StatusPreconditionRequired, "conversation_etag_required", "If-Match must contain the current conversation ETag.")
		return
	}
	session, duplicate, err := h.sessions.AppendMessage(r.Context(), actor, r.PathValue("sessionID"), r.Header.Get("Idempotency-Key"), expectedRevision, request)
	if err != nil {
		if errors.Is(err, sessions.ErrConversationChanged) {
			h.writeConversationChanged(w, r, actor, r.PathValue("sessionID"))
			return
		}
		writeSessionError(w, err)
		return
	}
	if err := h.dispatcher.Dispatch(r.Context()); err != nil {
		h.logger.Error("follow-up session dispatch failed", "error", err, "session_id", session.ID)
	}
	status := http.StatusCreated
	if duplicate {
		status = http.StatusOK
	}
	w.Header().Set("ETag", session.ConversationETag())
	writeJSON(w, status, session)
}

func (h Handler) members() (memberService, bool) {
	value, ok := h.sessions.(memberService)
	return value, ok
}
func (h Handler) listMembers(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	service, ok := h.members()
	if !ok {
		writeInternal(w, errors.New("session membership service is unavailable"))
		return
	}
	sessionID := r.PathValue("sessionID")
	after, ok := h.parseMemberListCursor(r.URL.Query().Get("cursor"), actor, sessionID)
	if !ok {
		writeError(w, http.StatusBadRequest, "cursor_invalid", "The Session member cursor is not valid for this member.")
		return
	}
	page, err := service.ListMembers(r.Context(), actor, sessionID, after, limit(r))
	if err != nil {
		writeSessionError(w, err)
		return
	}
	pageInfo := map[string]any{"has_more": page.HasMore}
	if page.HasMore {
		last := page.Items[len(page.Items)-1]
		pageInfo["next_cursor"] = h.encodeMemberListCursor(actor, sessionID, sessions.MemberCursor{JoinedAt: last.JoinedAt, PrincipalID: last.PrincipalID})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": page.Items, "page_info": pageInfo})
}
func memberRevision(w http.ResponseWriter, r *http.Request) (int64, bool) {
	revision, err := parseConversationETag(r.Header.Get("If-Match"))
	if err != nil {
		writeError(w, http.StatusPreconditionRequired, "conversation_etag_required", "If-Match must contain the current conversation ETag.")
		return 0, false
	}
	return revision, true
}
func writeMemberSession(w http.ResponseWriter, r *http.Request, session sessions.Session, err error, status int) {
	if err != nil {
		writeSessionError(w, err)
		return
	}
	w.Header().Set("ETag", session.ConversationETag())
	writeJSON(w, status, session)
}
func (h Handler) addMember(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	service, ok := h.members()
	if !ok {
		writeInternal(w, errors.New("session membership service is unavailable"))
		return
	}
	var request sessions.AddMemberRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	revision, ok := memberRevision(w, r)
	if !ok {
		return
	}
	session, err := service.AddMember(r.Context(), actor, r.PathValue("sessionID"), r.Header.Get("Idempotency-Key"), revision, request)
	writeMemberSession(w, r, session, err, http.StatusCreated)
}
func (h Handler) updateMember(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	service, ok := h.members()
	if !ok {
		writeInternal(w, errors.New("session membership service is unavailable"))
		return
	}
	var request sessions.UpdateMemberRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	revision, ok := memberRevision(w, r)
	if !ok {
		return
	}
	session, err := service.UpdateMember(r.Context(), actor, r.PathValue("sessionID"), r.PathValue("principalID"), r.Header.Get("Idempotency-Key"), revision, request)
	writeMemberSession(w, r, session, err, http.StatusOK)
}
func (h Handler) removeMember(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	service, ok := h.members()
	if !ok {
		writeInternal(w, errors.New("session membership service is unavailable"))
		return
	}
	revision, ok := memberRevision(w, r)
	if !ok {
		return
	}
	session, err := service.RemoveMember(r.Context(), actor, r.PathValue("sessionID"), r.PathValue("principalID"), r.Header.Get("Idempotency-Key"), revision)
	writeMemberSession(w, r, session, err, http.StatusOK)
}
func (h Handler) transferOwner(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	service, ok := h.members()
	if !ok {
		writeInternal(w, errors.New("session membership service is unavailable"))
		return
	}
	var request sessions.TransferOwnerRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	revision, ok := memberRevision(w, r)
	if !ok {
		return
	}
	session, err := service.TransferOwner(r.Context(), actor, r.PathValue("sessionID"), r.Header.Get("Idempotency-Key"), revision, request)
	writeMemberSession(w, r, session, err, http.StatusOK)
}
func (h Handler) archiveSession(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	service, ok := h.members()
	if !ok {
		writeInternal(w, errors.New("session membership service is unavailable"))
		return
	}
	revision, ok := memberRevision(w, r)
	if !ok {
		return
	}
	session, err := service.Archive(r.Context(), actor, r.PathValue("sessionID"), r.Header.Get("Idempotency-Key"), revision)
	writeMemberSession(w, r, session, err, http.StatusOK)
}
func (h Handler) listRuns(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	sessionID := r.PathValue("sessionID")
	after, ok := h.parseRunListCursor(r.URL.Query().Get("cursor"), actor, sessionID)
	if !ok {
		writeError(w, http.StatusBadRequest, "cursor_invalid", "The Run cursor is not valid for this Session member.")
		return
	}
	page, err := h.sessions.ListRuns(r.Context(), actor, sessionID, after, limit(r))
	if err != nil {
		writeSessionError(w, err)
		return
	}
	pageInfo := map[string]any{"has_more": page.HasMore}
	if page.HasMore {
		last := page.Items[len(page.Items)-1]
		pageInfo["next_cursor"] = h.encodeRunListCursor(actor, sessionID, sessions.RunCursor{SessionSequence: int(last.SessionSequence), ID: last.ID})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": page.Items, "page_info": pageInfo})
}
func (h Handler) listApprovals(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	if h.approvals == nil {
		writeInternal(w, errors.New("approval service is unavailable"))
		return
	}
	if !h.expireApprovals(r.Context(), actor) {
		writeInternal(w, errors.New("approval expiry processing failed"))
		return
	}
	state := r.URL.Query().Get("state")
	after, ok := h.parseApprovalListCursor(r.URL.Query().Get("cursor"), actor, state)
	if !ok {
		writeError(w, http.StatusBadRequest, "cursor_invalid", "The approval cursor is not valid for this requester.")
		return
	}
	page, err := h.approvals.List(r.Context(), actor, state, after, limit(r))
	if err != nil {
		writeInternal(w, err)
		return
	}
	pageInfo := map[string]any{"has_more": page.HasMore}
	if page.HasMore {
		last := page.Items[len(page.Items)-1]
		pageInfo["next_cursor"] = h.encodeApprovalListCursor(actor, state, approvals.Cursor{CreatedAt: last.CreatedAt, ID: last.ID})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": page.Items, "page_info": pageInfo})
}
func (h Handler) getApproval(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	if h.approvals == nil {
		writeInternal(w, errors.New("approval service is unavailable"))
		return
	}
	if !h.expireApprovals(r.Context(), actor) {
		writeInternal(w, errors.New("approval expiry processing failed"))
		return
	}
	item, err := h.approvals.Get(r.Context(), actor, r.PathValue("approvalID"))
	if err != nil {
		writeApprovalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (h Handler) decideApproval(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	if h.approvals == nil {
		writeInternal(w, errors.New("approval service is unavailable"))
		return
	}
	if !h.expireApprovals(r.Context(), actor) {
		writeInternal(w, errors.New("approval expiry processing failed"))
		return
	}
	var request struct {
		Decision     string `json:"decision"`
		Reason       string `json:"reason"`
		ActionDigest string `json:"action_digest"`
		Revision     int64  `json:"approval_revision"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	approvalID, ok := operationTarget(r.PathValue("operation"), ":decide")
	if !ok {
		http.NotFound(w, r)
		return
	}
	resolution, err := h.approvals.Decide(r.Context(), actor, approvals.DecisionInput{ID: approvalID, Decision: request.Decision, Reason: request.Reason, ActionDigest: request.ActionDigest, Idempotency: r.Header.Get("Idempotency-Key"), Revision: request.Revision})
	if err != nil {
		if h.writeApprovalCurrentState(w, r, actor, approvalID, err) {
			return
		}
		writeApprovalError(w, err)
		return
	}
	if !h.dispatcher.ResolveApproval(resolution.RunID, resolution.ApprovalID, resolution.Decision, resolution.Reason, resolution.ActionID, resolution.CallID, resolution.PermitID, resolution.PermitLeaseEpoch, resolution.PermitExpiresAt) {
		h.logger.Warn("approval persisted without an active runner session", "approval_id", resolution.ApprovalID, "run_id", resolution.RunID)
	}
	item, err := h.approvals.Get(r.Context(), actor, approvalID)
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// writeApprovalCurrentState returns the requester-visible winning projection
// for command races. It keeps the UI from inferring approval execution from a
// stale local decision attempt.
func (h Handler) writeApprovalCurrentState(w http.ResponseWriter, r *http.Request, actor identity.Principal, approvalID string, err error) bool {
	var status int
	var code, message string
	switch {
	case errors.Is(err, approvals.ErrInvalidDigest):
		status, code, message = http.StatusPreconditionFailed, "action_changed", "The action changed and requires a new approval."
	case errors.Is(err, approvals.ErrAlreadyDecided):
		status, code, message = http.StatusConflict, "already_decided", "The approval has already been decided."
	case errors.Is(err, approvals.ErrChanged):
		status, code, message = http.StatusConflict, "approval_changed", "The approval changed and must be reviewed again."
	case errors.Is(err, approvals.ErrExpired):
		status, code, message = http.StatusConflict, "approval_expired", "The approval request has expired."
	case errors.Is(err, approvals.ErrIdempotency):
		status, code, message = http.StatusConflict, "idempotency_key_reused", "The idempotency key was used for another decision."
	default:
		return false
	}
	item, getErr := h.approvals.Get(r.Context(), actor, approvalID)
	if getErr != nil {
		return false
	}
	writeProblem(w, status, code, message, item)
	return true
}

func (h Handler) expireApprovals(ctx context.Context, actor identity.Principal) bool {
	expired, err := h.approvals.Expire(ctx, actor)
	if err != nil {
		h.logger.Error("approval expiry processing failed", "error", err, "principal_id", actor.ID)
		return false
	}
	for _, resolution := range expired {
		if !h.dispatcher.ResolveApproval(resolution.RunID, resolution.ApprovalID, resolution.Decision, resolution.Reason, resolution.ActionID, resolution.CallID, "", 0, time.Time{}) {
			h.logger.Warn("expired approval persisted without an active runner session", "approval_id", resolution.ApprovalID, "run_id", resolution.RunID)
		}
	}
	return true
}
func (h Handler) cancelOperation(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	if runID, ok := operationTarget(r.PathValue("operation"), ":retry"); ok {
		h.retryRunOperation(w, r, actor, r.PathValue("sessionID"), runID)
		return
	}
	runID, ok := operationTarget(r.PathValue("operation"), ":cancel")
	if !ok {
		http.NotFound(w, r)
		return
	}
	result, err := h.sessions.Cancel(r.Context(), actor, r.PathValue("sessionID"), runID, r.Header.Get("Idempotency-Key"))
	if err != nil {
		writeSessionError(w, err)
		return
	}
	if result.Deliver && !h.dispatcher.RequestCancel(result.Run.ID, result.Run.LeaseEpoch, "requested by Copilot user") {
		h.logger.Warn("cancel persisted without an active runner session", "run_id", result.Run.ID)
	}
	status := http.StatusOK
	if result.Deliver {
		status = http.StatusAccepted
	}
	writeJSON(w, status, result.Run)
}
func (h Handler) retryOperation(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	if sessionID, ok := operationTarget(r.PathValue("operation"), ":transfer-owner"); ok {
		h.transferOwnerForSession(w, r, actor, sessionID)
		return
	}
	if sessionID, ok := operationTarget(r.PathValue("operation"), ":archive"); ok {
		h.archiveForSession(w, r, actor, sessionID)
		return
	}
	http.NotFound(w, r)
}

func (h Handler) retryRunOperation(w http.ResponseWriter, r *http.Request, actor identity.Principal, sessionID, runID string) {
	h.retryForSession(w, r, actor, sessionID, runID)
}

func (h Handler) retryForSession(w http.ResponseWriter, r *http.Request, actor identity.Principal, sessionID, runID string) {
	var request struct {
		RevisionSelection string `json:"revision_selection"`
	}
	if err := decodeJSON(w, r, &request); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	useLatest := request.RevisionSelection == "current_production_revision"
	if request.RevisionSelection != "original_revision" && !useLatest {
		writeError(w, http.StatusBadRequest, "invalid_request", "revision_selection must select the original or current production revision.")
		return
	}
	expectedRevision, err := parseConversationETag(r.Header.Get("If-Match"))
	if err != nil {
		writeError(w, http.StatusPreconditionRequired, "conversation_etag_required", "If-Match must contain the current conversation ETag.")
		return
	}
	result, err := h.sessions.RetryRun(r.Context(), actor, sessionID, runID, useLatest, r.Header.Get("Idempotency-Key"), expectedRevision)
	if err != nil {
		if errors.Is(err, sessions.ErrConversationChanged) {
			h.writeConversationChanged(w, r, actor, sessionID)
			return
		}
		writeSessionError(w, err)
		return
	}
	if !result.Duplicate {
		if err := h.dispatcher.Dispatch(r.Context()); err != nil {
			h.logger.Error("retried session dispatch failed", "error", err, "session_id", sessionID)
		}
	}
	status := http.StatusCreated
	if result.Duplicate {
		status = http.StatusOK
	}
	writeJSON(w, status, result.Run)
}

func (h Handler) transferOwnerForSession(w http.ResponseWriter, r *http.Request, actor identity.Principal, sessionID string) {
	service, ok := h.members()
	if !ok {
		writeInternal(w, errors.New("session membership service is unavailable"))
		return
	}
	var request sessions.TransferOwnerRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	revision, ok := memberRevision(w, r)
	if !ok {
		return
	}
	session, err := service.TransferOwner(r.Context(), actor, sessionID, r.Header.Get("Idempotency-Key"), revision, request)
	writeMemberSession(w, r, session, err, http.StatusOK)
}
func (h Handler) archiveForSession(w http.ResponseWriter, r *http.Request, actor identity.Principal, sessionID string) {
	service, ok := h.members()
	if !ok {
		writeInternal(w, errors.New("session membership service is unavailable"))
		return
	}
	revision, ok := memberRevision(w, r)
	if !ok {
		return
	}
	session, err := service.Archive(r.Context(), actor, sessionID, r.Header.Get("Idempotency-Key"), revision)
	writeMemberSession(w, r, session, err, http.StatusOK)
}

func (h Handler) writeConversationChanged(w http.ResponseWriter, r *http.Request, actor identity.Principal, sessionID string) {
	current, err := h.sessions.Get(r.Context(), actor, sessionID)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	w.Header().Set("ETag", current.ConversationETag())
	writeProblem(w, http.StatusConflict, "conversation_changed", "The session changed; review the latest conversation before continuing.", current)
}

func operationTarget(operation, suffix string) (string, bool) {
	target, ok := strings.CutSuffix(operation, suffix)
	return target, ok && target != "" && !strings.Contains(target, "/")
}

func parseConversationETag(raw string) (int64, error) {
	value := strings.TrimSpace(raw)
	if len(value) < 3 || value[0] != '"' || value[len(value)-1] != '"' {
		return 0, sessions.ErrPreconditionRequired
	}
	revision, err := strconv.ParseInt(value[1:len(value)-1], 10, 64)
	if err != nil || revision < 1 {
		return 0, sessions.ErrPreconditionRequired
	}
	return revision, nil
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("multiple JSON values")
	}
	return nil
}
func limit(r *http.Request) int { value, _ := strconv.Atoi(r.URL.Query().Get("limit")); return value }
func writeSessionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, sessions.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Resource was not found.")
	case errors.Is(err, sessions.ErrInvalidInput):
		writeError(w, http.StatusUnprocessableEntity, "invalid_input", "Session input is not supported.")
	case errors.Is(err, sessions.ErrInvalidState):
		writeError(w, http.StatusConflict, "invalid_state", "The Session is not in a state that permits this operation.")
	case errors.Is(err, sessions.ErrIdempotencyConflict):
		writeError(w, http.StatusConflict, "idempotency_key_reused", "The idempotency key was used for a different request.")
	case errors.Is(err, sessions.ErrPreconditionRequired):
		writeError(w, http.StatusPreconditionRequired, "conversation_etag_required", "If-Match must contain the current conversation ETag.")
	default:
		writeInternal(w, err)
	}
}
func writeApprovalError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, approvals.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Approval request was not found.")
	case errors.Is(err, approvals.ErrInvalidInput):
		writeError(w, http.StatusUnprocessableEntity, "invalid_input", "The approval decision is not valid.")
	case errors.Is(err, approvals.ErrInvalidDigest):
		writeError(w, http.StatusPreconditionFailed, "action_changed", "The action changed and requires a new approval.")
	case errors.Is(err, approvals.ErrNotEligible):
		writeError(w, http.StatusForbidden, "forbidden", "You are not eligible to decide this approval.")
	case errors.Is(err, approvals.ErrAlreadyDecided):
		writeError(w, http.StatusConflict, "already_decided", "The approval has already been decided.")
	case errors.Is(err, approvals.ErrChanged):
		writeError(w, http.StatusConflict, "approval_changed", "The approval changed and must be reviewed again.")
	case errors.Is(err, approvals.ErrExpired):
		writeError(w, http.StatusConflict, "approval_expired", "The approval request has expired.")
	case errors.Is(err, approvals.ErrIdempotency):
		writeError(w, http.StatusConflict, "idempotency_key_reused", "The idempotency key was used for another decision.")
	default:
		writeInternal(w, err)
	}
}
func writeInternal(w http.ResponseWriter, err error) {
	writeError(w, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
}
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeProblem(w, status, code, message, nil)
}
func writeProblem(w http.ResponseWriter, status int, code, message string, current any) {
	writeJSON(w, status, copilotProblem{Code: code, Message: message, CorrelationID: "cor_" + strconv.FormatUint(problemSequence.Add(1), 10), Retryable: status >= 500, CurrentResource: current})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
