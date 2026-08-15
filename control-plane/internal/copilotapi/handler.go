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
	ListAgents(context.Context, identity.Principal, string, string, int) ([]tasks.Agent, error)
	Submit(context.Context, identity.Principal, string, tasks.SubmitRequest) (tasks.Task, bool, error)
	List(context.Context, identity.Principal, string, int) ([]tasks.Task, error)
	Get(context.Context, identity.Principal, string) (tasks.Task, error)
	Cancel(context.Context, identity.Principal, string, string) (tasks.CancelResult, error)
	Retry(context.Context, identity.Principal, string, bool) (tasks.Task, error)
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
	List(context.Context, identity.Principal, int) ([]approvals.Request, error)
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
	mux.Handle("POST /tasks/{taskID}/events:ticket", h.withActor(h.issueEventTicket))
	mux.HandleFunc("GET /tasks/{taskID}/events", h.events)
	mux.Handle("GET /artifacts/{artifactID}", h.withActor(h.getArtifact))
	mux.Handle("GET /approvals", h.withActor(h.listApprovals))
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
	agents, err := h.tasks.ListAgents(r.Context(), actor, r.URL.Query().Get("category"), r.URL.Query().Get("search"), limit(r))
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": agents, "page_info": map[string]any{"has_more": false}})
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
	writeJSON(w, status, task)
}
func (h Handler) listTasks(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.tasks.List(r.Context(), actor, r.URL.Query().Get("status"), limit(r))
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "page_info": map[string]any{"has_more": false}})
}
func (h Handler) getTask(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	task, err := h.tasks.Get(r.Context(), actor, r.PathValue("taskID"))
	if err != nil {
		writeTaskError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}
func (h Handler) listApprovals(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	if h.approvals == nil {
		writeInternal(w, errors.New("approval service is unavailable"))
		return
	}
	items, err := h.approvals.List(r.Context(), actor, limit(r))
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "page_info": map[string]any{"has_more": false}})
}
func (h Handler) decideApproval(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	if h.approvals == nil {
		writeInternal(w, errors.New("approval service is unavailable"))
		return
	}
	var request struct {
		Decision     string `json:"decision"`
		Reason       string `json:"reason"`
		ActionDigest string `json:"action_digest"`
		Idempotency  string `json:"idempotency_key"`
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
	resolution, err := h.approvals.Decide(r.Context(), actor, approvals.DecisionInput{ID: approvalID, Decision: request.Decision, Reason: request.Reason, ActionDigest: request.ActionDigest, Idempotency: request.Idempotency})
	if err != nil {
		writeApprovalError(w, err)
		return
	}
	if !h.dispatcher.ResolveApproval(resolution.RunID, resolution.ApprovalID, resolution.Decision, resolution.Reason, resolution.ActionID, resolution.CallID, resolution.PermitID, resolution.PermitLeaseEpoch, resolution.PermitExpiresAt) {
		h.logger.Warn("approval persisted without an active runner session", "approval_id", resolution.ApprovalID, "run_id", resolution.RunID)
	}
	status := "rejected"
	if resolution.Decision == "approve" {
		status = "satisfied"
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": status, "approval_id": resolution.ApprovalID, "run_id": resolution.RunID, "decision": resolution.Decision})
}
func (h Handler) cancelOperation(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	runID, ok := operationTarget(r.PathValue("operation"), ":cancel")
	if !ok {
		http.NotFound(w, r)
		return
	}
	result, err := h.tasks.Cancel(r.Context(), actor, r.PathValue("taskID"), runID)
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
		UseLatestVersion bool `json:"use_latest_version"`
	}
	if err := decodeJSON(w, r, &request); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	task, err := h.tasks.Retry(r.Context(), actor, taskID, request.UseLatestVersion)
	if err != nil {
		writeTaskError(w, err)
		return
	}
	if err := h.dispatcher.Dispatch(r.Context()); err != nil {
		h.logger.Error("retried task dispatch failed", "error", err, "task_id", task.ID)
	}
	writeJSON(w, http.StatusCreated, task)
}

func operationTarget(operation, suffix string) (string, bool) {
	target, ok := strings.CutSuffix(operation, suffix)
	return target, ok && target != "" && !strings.Contains(target, "/")
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
		writeError(w, http.StatusPreconditionFailed, "stale_action", "The action changed and requires a new approval.")
	case errors.Is(err, approvals.ErrNotEligible):
		writeError(w, http.StatusForbidden, "forbidden", "You are not eligible to decide this approval.")
	case errors.Is(err, approvals.ErrAlreadyDecided):
		writeError(w, http.StatusConflict, "already_decided", "The approval has already been decided.")
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
