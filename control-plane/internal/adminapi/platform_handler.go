package adminapi

import (
	"net/http"
	"strings"

	"github.com/AirSodaz/gantry/internal/adminplatform"
	"github.com/AirSodaz/gantry/internal/identity"
)

func (h Handler) listPlatformProviders(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.platform.ListProviders(r.Context(), actor)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (h Handler) createPlatformProvider(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var req adminplatform.CreateProviderRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, err := h.platform.CreateProvider(r.Context(), actor, req)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}
func (h Handler) listProviderRoutes(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.platform.ListRoutes(r.Context(), actor, r.PathValue("providerID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (h Handler) putProviderRoute(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var req adminplatform.PutRouteRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, err := h.platform.PutRoute(r.Context(), actor, r.PathValue("providerID"), r.PathValue("routeID"), r.Header.Get("If-Match"), req)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	w.Header().Set("ETag", `"`+item.ETag+`"`)
	writeJSON(w, http.StatusOK, item)
}
func (h Handler) platformProviderCommand(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	id, op, ok := strings.Cut(r.PathValue("providerID"), ":")
	if !ok || op != "quarantine" {
		http.NotFound(w, r)
		return
	}
	item, err := h.platform.QuarantineProvider(r.Context(), actor, id)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (h Handler) listRunnerPools(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.platform.ListRunnerPools(r.Context(), actor)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (h Handler) createRunnerPool(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var req adminplatform.CreateRunnerPoolRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, err := h.platform.CreateRunnerPool(r.Context(), actor, req)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}
func (h Handler) listRunners(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.platform.ListRunners(r.Context(), actor, r.PathValue("poolID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (h Handler) runnerPoolCommand(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	id, op, ok := strings.Cut(r.PathValue("poolID"), ":")
	if !ok || (op != "drain" && op != "quarantine") {
		http.NotFound(w, r)
		return
	}
	item, err := h.platform.SetPoolState(r.Context(), actor, id, op)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
