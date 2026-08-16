package adminapi

import (
	"net/http"
	"strings"

	"github.com/AirSodaz/gantry/internal/adminevaluation"
	"github.com/AirSodaz/gantry/internal/identity"
)

func (h Handler) listEvaluationSuites(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	limit, err := queryLimit(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "limit must be a positive integer no greater than 100.")
		return
	}
	item, err := h.evaluations.ListSuites(r.Context(), actor, adminevaluation.ListOptions{WorkspaceID: r.URL.Query().Get("workspace_id"), State: r.URL.Query().Get("state"), Search: r.URL.Query().Get("search"), Limit: limit})
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h Handler) createEvaluationSuite(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var request adminevaluation.CreateSuiteRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, err := h.evaluations.CreateSuite(r.Context(), actor, request)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h Handler) getEvaluationSuite(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	item, err := h.evaluations.GetSuite(r.Context(), actor, r.PathValue("suiteID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	w.Header().Set("ETag", `"`+item.ETag+`"`)
	writeJSON(w, http.StatusOK, item)
}

func (h Handler) patchEvaluationSuite(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var request adminevaluation.PatchSuiteRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, err := h.evaluations.PatchSuite(r.Context(), actor, r.PathValue("suiteID"), r.Header.Get("If-Match"), request)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	w.Header().Set("ETag", `"`+item.ETag+`"`)
	writeJSON(w, http.StatusOK, item)
}

func (h Handler) listEvaluationCases(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.evaluations.ListCases(r.Context(), actor, r.PathValue("suiteID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "page_info": map[string]any{"next_cursor": nil}})
}
func (h Handler) createEvaluationCase(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var request adminevaluation.CreateCaseRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, err := h.evaluations.CreateCase(r.Context(), actor, r.PathValue("suiteID"), request)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}
func (h Handler) patchEvaluationCase(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var request adminevaluation.PatchCaseRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, err := h.evaluations.PatchCase(r.Context(), actor, r.PathValue("suiteID"), r.PathValue("caseID"), r.Header.Get("If-Match"), request)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	w.Header().Set("ETag", `"`+item.ETag+`"`)
	writeJSON(w, http.StatusOK, item)
}

func (h Handler) evaluationSuiteCommand(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	suiteID, operation, ok := strings.Cut(r.PathValue("suiteID"), ":")
	if !ok || operation != "validate" {
		http.NotFound(w, r)
		return
	}
	item, err := h.evaluations.ValidateSuite(r.Context(), actor, suiteID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (h Handler) listEvaluationVersions(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.evaluations.ListVersions(r.Context(), actor, r.PathValue("suiteID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "page_info": map[string]any{"next_cursor": nil}})
}
func (h Handler) publishEvaluationVersion(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var request adminevaluation.PublishVersionRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, err := h.evaluations.PublishVersion(r.Context(), actor, r.PathValue("suiteID"), r.Header.Get("If-Match"), r.Header.Get("Idempotency-Key"), request)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}
func (h Handler) listEvaluationRuns(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.evaluations.ListRuns(r.Context(), actor, r.PathValue("suiteID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "page_info": map[string]any{"next_cursor": nil}})
}
func (h Handler) createEvaluationRun(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var request adminevaluation.CreateRunRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, err := h.evaluations.CreateRun(r.Context(), actor, r.PathValue("suiteID"), r.Header.Get("Idempotency-Key"), request)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, item)
}
func (h Handler) getEvaluationRun(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	item, err := h.evaluations.GetRun(r.Context(), actor, r.PathValue("runID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (h Handler) evaluationRunCommand(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	runID, operation, ok := strings.Cut(r.PathValue("runID"), ":")
	if !ok || operation != "cancel" {
		http.NotFound(w, r)
		return
	}
	item, err := h.evaluations.CancelRun(r.Context(), actor, runID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
