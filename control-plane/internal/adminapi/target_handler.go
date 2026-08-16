package adminapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/AirSodaz/gantry/internal/agentlifecycle"
	"github.com/AirSodaz/gantry/internal/identity"
)

func (h Handler) registerTargetRoutes(mux *http.ServeMux) {
	mux.Handle("GET /agents/{agentID}/lifecycle", h.withActor(h.getTargetOverview))
	mux.Handle("GET /agents/{agentID}/drafts", h.withActor(h.listNamedDrafts))
	mux.Handle("POST /agents/{agentID}/drafts", h.withActor(h.createNamedDraft))
	mux.Handle("GET /agents/{agentID}/drafts/{draftID}", h.withActor(h.getNamedDraft))
	mux.Handle("PUT /agents/{agentID}/drafts/{draftID}", h.withActor(h.updateNamedDraft))
	mux.Handle("POST /agents/{agentID}/drafts/{operation...}", h.withActor(h.draftCommand))
	mux.Handle("GET /agents/{agentID}/revisions", h.withActor(h.listRevisions))
	mux.Handle("GET /agents/{agentID}/revisions/{revisionHash}", h.withActor(h.getRevision))
	mux.Handle("GET /agents/{agentID}/revisions/{revisionHash}/review", h.withActor(h.getRevisionReview))
	mux.Handle("POST /agents/{agentID}/revisions/{revisionHash}/review", h.withActor(h.submitRevisionReview))
	mux.Handle("POST /agents/{agentID}/revisions/{operation...}", h.withActor(h.revisionCommand))
	mux.Handle("GET /agents/{agentID}/deployments", h.withActor(h.listDeployments))
	mux.Handle("POST /agents/{agentID}/deployments", h.withActor(h.createTestDeployment))
	mux.Handle("POST /agents/{agentID}/deployments/{operation...}", h.withActor(h.deploymentCommand))
}

func splitTargetOperation(r *http.Request) (string, string, bool) {
	value := r.PathValue("operation")
	separator := strings.LastIndexByte(value, ':')
	if separator <= 0 || separator == len(value)-1 {
		return "", "", false
	}
	return value[:separator], value[separator+1:], true
}

func (h Handler) draftCommand(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	_, operation, ok := splitTargetOperation(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	switch operation {
	case "archive":
		h.archiveNamedDraft(w, r, actor)
	case "commit":
		h.commitNamedDraft(w, r, actor)
	default:
		http.NotFound(w, r)
	}
}

func (h Handler) revisionCommand(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	_, operation, ok := splitTargetOperation(r)
	if !ok {
		http.NotFound(w, r)
		return
	}
	switch operation {
	case "review-decision":
		h.decideRevisionReview(w, r, actor)
	case "publish":
		h.publishRevision(w, r, actor)
	default:
		http.NotFound(w, r)
	}
}

func (h Handler) deploymentCommand(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	_, operation, ok := splitTargetOperation(r)
	if !ok || operation != "stop" {
		http.NotFound(w, r)
		return
	}
	h.stopTestDeployment(w, r, actor)
}

func (h Handler) getTargetOverview(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	overview, err := h.target.GetTargetOverview(r.Context(), actor, r.PathValue("agentID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (h Handler) listNamedDrafts(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.target.ListNamedDrafts(r.Context(), actor, r.PathValue("agentID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "page_info": map[string]bool{"has_more": false}})
}

func (h Handler) createNamedDraft(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var request agentlifecycle.CreateDraftRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	draft, err := h.target.CreateNamedDraft(r.Context(), actor, r.PathValue("agentID"), request)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, draft)
}

func (h Handler) getNamedDraft(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	draft, err := h.target.GetNamedDraft(r.Context(), actor, r.PathValue("agentID"), r.PathValue("draftID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, draft)
}

func (h Handler) updateNamedDraft(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	etag, err := revisionHeader(r)
	if err != nil {
		writeError(w, http.StatusPreconditionRequired, "working_copy_etag_required", "If-Match must contain the current working-copy ETag.")
		return
	}
	var request struct {
		Spec json.RawMessage `json:"spec"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	draft, err := h.target.UpdateNamedDraft(r.Context(), actor, r.PathValue("agentID"), r.PathValue("draftID"), etag, request.Spec)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, draft)
}

func (h Handler) archiveNamedDraft(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	draftID, operation, ok := splitTargetOperation(r)
	if !ok || operation != "archive" {
		http.NotFound(w, r)
		return
	}
	if err := h.target.ArchiveNamedDraft(r.Context(), actor, r.PathValue("agentID"), draftID); err != nil {
		h.writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handler) commitNamedDraft(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	draftID, operation, ok := splitTargetOperation(r)
	if !ok || operation != "commit" {
		http.NotFound(w, r)
		return
	}
	var request agentlifecycle.CommitDraftRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	revision, err := h.target.CommitNamedDraft(r.Context(), actor, r.PathValue("agentID"), draftID, request)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, revision)
}

func (h Handler) listRevisions(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.target.ListRevisions(r.Context(), actor, r.PathValue("agentID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "page_info": map[string]bool{"has_more": false}})
}

func (h Handler) getRevision(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	revision, err := h.target.GetRevision(r.Context(), actor, r.PathValue("agentID"), r.PathValue("revisionHash"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, revision)
}

func (h Handler) getRevisionReview(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	review, err := h.target.GetRevisionReview(r.Context(), actor, r.PathValue("agentID"), r.PathValue("revisionHash"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, review)
}

func (h Handler) submitRevisionReview(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var request struct {
		ReleaseNotes string `json:"release_notes"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	review, err := h.target.SubmitRevisionReview(r.Context(), actor, r.PathValue("agentID"), r.PathValue("revisionHash"), request.ReleaseNotes)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, review)
}

func (h Handler) decideRevisionReview(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	revisionHash, operation, ok := splitTargetOperation(r)
	if !ok || operation != "review-decision" {
		http.NotFound(w, r)
		return
	}
	var request agentlifecycle.ReviewDecisionRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	review, err := h.target.DecideRevisionReview(r.Context(), actor, r.PathValue("agentID"), revisionHash, request.Decision, request.Reason)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, review)
}

func (h Handler) publishRevision(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	revisionHash, operation, ok := splitTargetOperation(r)
	if !ok || operation != "publish" {
		http.NotFound(w, r)
		return
	}
	var request agentlifecycle.PublishRevisionRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	request.RevisionHash = revisionHash
	deployment, err := h.target.PublishRevision(r.Context(), actor, r.PathValue("agentID"), request)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, deployment)
}

func (h Handler) listDeployments(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.target.ListDeployments(r.Context(), actor, r.PathValue("agentID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "page_info": map[string]bool{"has_more": false}})
}

func (h Handler) createTestDeployment(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var request agentlifecycle.CreateDeploymentRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	deployment, err := h.target.CreateTestDeployment(r.Context(), actor, r.PathValue("agentID"), request)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, deployment)
}

func (h Handler) stopTestDeployment(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	deploymentID, operation, ok := splitTargetOperation(r)
	if !ok || operation != "stop" {
		http.NotFound(w, r)
		return
	}
	if err := h.target.StopTestDeployment(r.Context(), actor, r.PathValue("agentID"), deploymentID); err != nil {
		h.writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
