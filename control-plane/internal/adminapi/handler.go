// Package adminapi owns the HTTP transport for the administrative agent lifecycle.
package adminapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/AirSodaz/gantry/internal/agentlifecycle"
	"github.com/AirSodaz/gantry/internal/authorization"
	"github.com/AirSodaz/gantry/internal/identity"
)

type authenticator interface {
	Authenticate(context.Context, string) (identity.Principal, error)
}

type authorizer interface {
	RequireAdmin(context.Context, identity.Principal) error
}

type lifecycleService interface {
	ListWorkspaces(context.Context, identity.Principal) ([]authorization.Workspace, error)
	ListAgents(context.Context, identity.Principal, string) ([]agentlifecycle.Agent, error)
	Create(context.Context, identity.Principal, agentlifecycle.CreateRequest) (agentlifecycle.Agent, error)
	Get(context.Context, identity.Principal, string) (agentlifecycle.Agent, error)
	GetDraft(context.Context, identity.Principal, string) (agentlifecycle.Draft, error)
	UpdateDraft(context.Context, identity.Principal, string, int, json.RawMessage) (agentlifecycle.Draft, error)
	ListVersions(context.Context, identity.Principal, string) ([]agentlifecycle.Version, error)
	GetReview(context.Context, identity.Principal, string) (agentlifecycle.Review, error)
	SubmitReview(context.Context, identity.Principal, string, int, string) (agentlifecycle.Review, error)
	DecideReview(context.Context, identity.Principal, string, string, string) (agentlifecycle.Review, error)
	Publish(context.Context, identity.Principal, string, int) (agentlifecycle.Version, bool, error)
	Retire(context.Context, identity.Principal, string) error
	Rollback(context.Context, identity.Principal, string, string) error
}

type Handler struct {
	auth      authenticator
	authorize authorizer
	service   lifecycleService
	logger    *slog.Logger
}

func New(auth authenticator, authorize authorizer, service lifecycleService, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	h := Handler{auth: auth, authorize: authorize, service: service, logger: logger}
	mux := http.NewServeMux()
	mux.Handle("GET /workspaces", h.withActor(h.listWorkspaces))
	mux.Handle("GET /agents", h.withActor(h.listAgents))
	mux.Handle("POST /agents", h.withActor(h.createAgent))
	mux.Handle("GET /agents/{agentID}", h.withActor(h.getAgent))
	mux.Handle("GET /agents/{agentID}/draft", h.withActor(h.getDraft))
	mux.Handle("PUT /agents/{agentID}/draft", h.withActor(h.updateDraft))
	mux.Handle("GET /agents/{agentID}/versions", h.withActor(h.listVersions))
	mux.Handle("GET /agents/{agentID}/review", h.withActor(h.getReview))
	mux.Handle("POST /agents/{operation...}", h.withActor(h.command))
	return mux
}

type actorHandler func(http.ResponseWriter, *http.Request, identity.Principal)

func (h Handler) withActor(next actorHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor, err := h.auth.Authenticate(r.Context(), r.Header.Get("Authorization"))
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "A valid Admin access token is required.")
			return
		}
		if err := h.authorize.RequireAdmin(r.Context(), actor); err != nil {
			if errors.Is(err, authorization.ErrForbidden) {
				writeError(w, http.StatusForbidden, "forbidden", "Administrative access is required.")
				return
			}
			h.writeInternal(w, err)
			return
		}
		next(w, r, actor)
	})
}

func (h Handler) listWorkspaces(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.service.ListWorkspaces(r.Context(), actor)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "page_info": map[string]bool{"has_more": false}})
}

func (h Handler) listAgents(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.service.ListAgents(r.Context(), actor, r.URL.Query().Get("workspace_id"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "page_info": map[string]bool{"has_more": false}})
}

func (h Handler) createAgent(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var request agentlifecycle.CreateRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	agent, err := h.service.Create(r.Context(), actor, request)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, agent)
}

func (h Handler) getAgent(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	agent, err := h.service.Get(r.Context(), actor, r.PathValue("agentID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, agent)
}

func (h Handler) getDraft(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	draft, err := h.service.GetDraft(r.Context(), actor, r.PathValue("agentID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, draft)
}

func (h Handler) updateDraft(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	revision, err := revisionHeader(r)
	if err != nil {
		writeError(w, http.StatusPreconditionRequired, "revision_required", "If-Match must contain the current draft revision.")
		return
	}
	var request struct {
		Spec json.RawMessage `json:"spec"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	draft, err := h.service.UpdateDraft(r.Context(), actor, r.PathValue("agentID"), revision, request.Spec)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, draft)
}

func (h Handler) listVersions(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.service.ListVersions(r.Context(), actor, r.PathValue("agentID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "page_info": map[string]bool{"has_more": false}})
}

func (h Handler) getReview(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	review, err := h.service.GetReview(r.Context(), actor, r.PathValue("agentID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, review)
}

func (h Handler) command(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	agentID, operation, ok := strings.Cut(r.PathValue("operation"), ":")
	if !ok || agentID == "" {
		http.NotFound(w, r)
		return
	}
	switch operation {
	case "review":
		revision, err := revisionHeader(r)
		if err != nil {
			writeError(w, http.StatusPreconditionRequired, "revision_required", "If-Match must contain the current draft revision.")
			return
		}
		var request struct {
			ReleaseNotes string `json:"release_notes"`
		}
		if err := decodeJSON(w, r, &request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
			return
		}
		review, err := h.service.SubmitReview(r.Context(), actor, agentID, revision, request.ReleaseNotes)
		if err != nil {
			h.writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, review)
	case "review-decision":
		var request agentlifecycle.ReviewDecisionRequest
		if err := decodeJSON(w, r, &request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
			return
		}
		review, err := h.service.DecideReview(r.Context(), actor, agentID, request.Decision, request.Reason)
		if err != nil {
			h.writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, review)
	case "publish":
		revision, err := revisionHeader(r)
		if err != nil {
			writeError(w, http.StatusPreconditionRequired, "revision_required", "If-Match must contain the current draft revision.")
			return
		}
		version, duplicate, err := h.service.Publish(r.Context(), actor, agentID, revision)
		if err != nil {
			h.writeServiceError(w, err)
			return
		}
		status := http.StatusCreated
		if duplicate {
			status = http.StatusOK
		}
		writeJSON(w, status, version)
	case "retire":
		if err := h.service.Retire(r.Context(), actor, agentID); err != nil {
			h.writeServiceError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case "rollback":
		var request agentlifecycle.RollbackRequest
		if err := decodeJSON(w, r, &request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
			return
		}
		if err := h.service.Rollback(r.Context(), actor, agentID, request.VersionID); err != nil {
			h.writeServiceError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

func revisionHeader(r *http.Request) (int, error) {
	value := strings.Trim(strings.TrimSpace(r.Header.Get("If-Match")), `"`)
	revision, err := strconv.Atoi(value)
	if err != nil || revision < 1 {
		return 0, errors.New("invalid revision")
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

func (h Handler) writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, agentlifecycle.ErrNotFound), errors.Is(err, authorization.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Resource was not found.")
	case errors.Is(err, agentlifecycle.ErrInvalidInput):
		writeError(w, http.StatusUnprocessableEntity, "invalid_input", "The agent request is not valid.")
	case errors.Is(err, agentlifecycle.ErrRevisionConflict):
		writeError(w, http.StatusPreconditionFailed, "revision_conflict", "The draft was changed by another administrator.")
	case errors.Is(err, agentlifecycle.ErrInvalidState):
		writeError(w, http.StatusConflict, "invalid_state", "The agent is not in a state that permits this operation.")
	case errors.Is(err, agentlifecycle.ErrReviewRequired):
		writeError(w, http.StatusConflict, "review_required", "An approved review for the current draft is required before publication.")
	case errors.Is(err, authorization.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "Administrative access is required.")
	default:
		h.writeInternal(w, err)
	}
}

func (h Handler) writeInternal(w http.ResponseWriter, err error) {
	h.logger.Error("admin API request failed", "error", err)
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
