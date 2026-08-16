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
	"time"

	"github.com/AirSodaz/gantry/internal/approvals"
	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/AirSodaz/gantry/internal/tasks"
)

type authenticator interface {
	Authenticate(context.Context, string) (identity.Principal, error)
}

// taskService is the Copilot application's transport-facing use-case boundary.
// The concrete PostgreSQL service remains in the tasks package.
type taskService interface {
	ListAgents(context.Context, identity.Principal, string, string, *tasks.AgentCursor, int) (tasks.AgentPage, error)
	Submit(context.Context, identity.Principal, string, tasks.SubmitRequest) (tasks.Task, bool, error)
	List(context.Context, identity.Principal, tasks.ListFilter, *tasks.TaskCursor, int) (tasks.TaskPage, error)
	Get(context.Context, identity.Principal, string) (tasks.Task, error)
	AppendMessage(context.Context, identity.Principal, string, string, int64, tasks.AppendMessageRequest) (tasks.Task, bool, error)
	ListRuns(context.Context, identity.Principal, string, *tasks.RunCursor, int) (tasks.RunPage, error)
	Cancel(context.Context, identity.Principal, string, string, string) (tasks.CancelResult, error)
	Retry(context.Context, identity.Principal, string, bool, string, int64) (tasks.Task, error)
}

type eventReader interface {
	Events(context.Context, identity.Principal, string, uint64, int) (tasks.EventPage, error)
}

type artifactReader interface {
	GetArtifact(context.Context, identity.Principal, string) (tasks.Artifact, error)
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
	tasks      taskService
	approvals  approvalService
	dispatcher dispatcher
	logger     *slog.Logger
	eventKey   []byte
}

func New(auth authenticator, taskService taskService, approvalService approvalService, dispatcher dispatcher, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	h := Handler{auth: auth, tasks: taskService, approvals: approvalService, dispatcher: dispatcher, logger: logger, eventKey: loadEventTicketKey()}
	mux := http.NewServeMux()
	mux.Handle("GET /agents", h.withActor(h.listAgents))
	mux.Handle("POST /tasks", h.withActor(h.submitTask))
	mux.Handle("GET /tasks", h.withActor(h.listTasks))
	mux.Handle("GET /tasks/{taskID}", h.withActor(h.getTask))
	mux.Handle("POST /tasks/{taskID}/messages", h.withActor(h.appendMessage))
	mux.Handle("GET /tasks/{taskID}/runs", h.withActor(h.listRuns))
	mux.Handle("POST /tasks/{taskID}/events:ticket", h.withActor(h.issueEventTicket))
	mux.HandleFunc("GET /tasks/{taskID}/events", h.events)
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
	mux.Handle("POST /tasks/{taskID}/runs/{operation...}", h.withActor(h.cancelOperation))
	mux.Handle("POST /tasks/{operation...}", h.withActor(h.retryOperation))
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
	category, search := r.URL.Query().Get("category"), r.URL.Query().Get("search")
	after, ok := h.parseAgentListCursor(r.URL.Query().Get("cursor"), actor, category, search)
	if !ok {
		writeError(w, http.StatusBadRequest, "cursor_invalid", "The agent cursor is not valid for this requester or filter.")
		return
	}
	page, err := h.tasks.ListAgents(r.Context(), actor, category, search, after, limit(r))
	if err != nil {
		writeInternal(w, err)
		return
	}
	info := map[string]any{"has_more": page.HasMore}
	if page.HasMore {
		last := page.Items[len(page.Items)-1]
		info["next_cursor"] = h.encodeAgentListCursor(actor, category, search, tasks.AgentCursor{DisplayName: last.DisplayName, ID: last.ID})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": page.Items, "page_info": info})
}
func (h Handler) submitTask(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var request tasks.SubmitRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	task, duplicate, err := h.tasks.Submit(r.Context(), actor, r.Header.Get("Idempotency-Key"), request)
	if err != nil {
		writeTaskError(w, err)
		return
	}
	if err := h.dispatcher.Dispatch(r.Context()); err != nil {
		h.logger.Error("queued task dispatch failed", "error", err, "task_id", task.ID)
	}
	status := http.StatusCreated
	if duplicate {
		status = http.StatusOK
	}
	w.Header().Set("ETag", task.ConversationETag())
	writeJSON(w, status, task)
}
func (h Handler) listTasks(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	filter := tasks.ListFilter{Status: r.URL.Query().Get("status"), AgentID: r.URL.Query().Get("agent_id"), RequesterAction: r.URL.Query().Get("requester_action")}
	if value := r.URL.Query().Get("created_after"); value != "" {
		createdAfter, err := time.Parse(time.RFC3339, value)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "created_after must be an RFC 3339 timestamp.")
			return
		}
		filter.CreatedAfter = &createdAfter
	}
	after, ok := h.parseTaskListCursor(r.URL.Query().Get("cursor"), actor, filter)
	if !ok {
		writeError(w, http.StatusBadRequest, "cursor_invalid", "The task cursor is not valid for this requester or filter.")
		return
	}
	page, err := h.tasks.List(r.Context(), actor, filter, after, limit(r))
	if err != nil {
		writeInternal(w, err)
		return
	}
	pageInfo := map[string]any{"has_more": page.HasMore}
	if page.HasMore {
		last := page.Items[len(page.Items)-1]
		pageInfo["next_cursor"] = h.encodeTaskListCursor(actor, filter, tasks.TaskCursor{CreatedAt: last.CreatedAt, ID: last.ID})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": page.Items, "page_info": pageInfo})
}
func (h Handler) getTask(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	task, err := h.tasks.Get(r.Context(), actor, r.PathValue("taskID"))
	if err != nil {
		writeTaskError(w, err)
		return
	}
	w.Header().Set("ETag", task.ConversationETag())
	writeJSON(w, http.StatusOK, task)
}
func (h Handler) appendMessage(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var request tasks.AppendMessageRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	expectedRevision, err := parseConversationETag(r.Header.Get("If-Match"))
	if err != nil {
		writeError(w, http.StatusPreconditionRequired, "conversation_etag_required", "If-Match must contain the current conversation ETag.")
		return
	}
	task, duplicate, err := h.tasks.AppendMessage(r.Context(), actor, r.PathValue("taskID"), r.Header.Get("Idempotency-Key"), expectedRevision, request)
	if err != nil {
		if errors.Is(err, tasks.ErrConversationChanged) {
			h.writeConversationChanged(w, r, actor, r.PathValue("taskID"))
			return
		}
		writeTaskError(w, err)
		return
	}
	if err := h.dispatcher.Dispatch(r.Context()); err != nil {
		h.logger.Error("follow-up task dispatch failed", "error", err, "task_id", task.ID)
	}
	status := http.StatusCreated
	if duplicate {
		status = http.StatusOK
	}
	w.Header().Set("ETag", task.ConversationETag())
	writeJSON(w, status, task)
}
func (h Handler) listRuns(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	taskID := r.PathValue("taskID")
	after, ok := h.parseRunListCursor(r.URL.Query().Get("cursor"), actor, taskID)
	if !ok {
		writeError(w, http.StatusBadRequest, "cursor_invalid", "The run cursor is not valid for this requester or task.")
		return
	}
	page, err := h.tasks.ListRuns(r.Context(), actor, taskID, after, limit(r))
	if err != nil {
		writeTaskError(w, err)
		return
	}
	pageInfo := map[string]any{"has_more": page.HasMore}
	if page.HasMore {
		last := page.Items[len(page.Items)-1]
		pageInfo["next_cursor"] = h.encodeRunListCursor(actor, taskID, tasks.RunCursor{Attempt: last.Attempt, ID: last.ID})
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
	writeJSON(w, status, map[string]any{"error": map[string]any{"code": code, "message": message, "current_resource": item}})
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
	runID, ok := operationTarget(r.PathValue("operation"), ":cancel")
	if !ok {
		http.NotFound(w, r)
		return
	}
	result, err := h.tasks.Cancel(r.Context(), actor, r.PathValue("taskID"), runID, r.Header.Get("Idempotency-Key"))
	if err != nil {
		writeTaskError(w, err)
		return
	}
	if result.Deliver && !h.dispatcher.RequestCancel(result.Run.ID, result.Run.LeaseEpoch, "requested by Copilot user") {
		h.logger.Warn("cancel persisted without an active runner session", "run_id", result.Run.ID)
	}
	writeJSON(w, http.StatusOK, result.Run)
}
func (h Handler) retryOperation(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	taskID, ok := operationTarget(r.PathValue("operation"), ":retry")
	if !ok {
		http.NotFound(w, r)
		return
	}
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
	task, err := h.tasks.Retry(r.Context(), actor, taskID, useLatest, r.Header.Get("Idempotency-Key"), expectedRevision)
	if err != nil {
		if errors.Is(err, tasks.ErrConversationChanged) {
			h.writeConversationChanged(w, r, actor, taskID)
			return
		}
		writeTaskError(w, err)
		return
	}
	if err := h.dispatcher.Dispatch(r.Context()); err != nil {
		h.logger.Error("retried task dispatch failed", "error", err, "task_id", task.ID)
	}
	w.Header().Set("ETag", task.ConversationETag())
	writeJSON(w, http.StatusCreated, task)
}

func (h Handler) writeConversationChanged(w http.ResponseWriter, r *http.Request, actor identity.Principal, taskID string) {
	current, err := h.tasks.Get(r.Context(), actor, taskID)
	if err != nil {
		writeTaskError(w, err)
		return
	}
	w.Header().Set("ETag", current.ConversationETag())
	writeJSON(w, http.StatusConflict, map[string]any{"error": map[string]any{"code": "conversation_changed", "message": "The task changed; review the latest conversation before continuing.", "current_resource": current}})
}

func operationTarget(operation, suffix string) (string, bool) {
	target, ok := strings.CutSuffix(operation, suffix)
	return target, ok && target != "" && !strings.Contains(target, "/")
}

func parseConversationETag(raw string) (int64, error) {
	value := strings.TrimSpace(raw)
	if len(value) < 3 || value[0] != '"' || value[len(value)-1] != '"' {
		return 0, tasks.ErrPreconditionRequired
	}
	revision, err := strconv.ParseInt(value[1:len(value)-1], 10, 64)
	if err != nil || revision < 1 {
		return 0, tasks.ErrPreconditionRequired
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
func writeTaskError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, tasks.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Resource was not found.")
	case errors.Is(err, tasks.ErrInvalidInput):
		writeError(w, http.StatusUnprocessableEntity, "invalid_input", "Task input is not supported.")
	case errors.Is(err, tasks.ErrInvalidState):
		writeError(w, http.StatusConflict, "invalid_state", "The task is not in a state that permits this operation.")
	case errors.Is(err, tasks.ErrIdempotencyConflict):
		writeError(w, http.StatusConflict, "idempotency_key_reused", "The idempotency key was used for a different request.")
	case errors.Is(err, tasks.ErrPreconditionRequired):
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
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
