package adminapi

import (
	"net/http"

	"github.com/AirSodaz/gantry/internal/adminplatform"
	"github.com/AirSodaz/gantry/internal/identity"
)

func (h Handler) listLimitPolicies(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.platform.ListLimitPolicies(r.Context(), actor, r.URL.Query().Get("workspace_id"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handler) upsertLimitPolicy(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var req adminplatform.UpsertLimitPolicyRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, err := h.platform.UpsertLimitPolicy(r.Context(), actor, r.PathValue("policyID"), r.Header.Get("If-Match"), req)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h Handler) listEnvironmentProfiles(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.platform.ListEnvironmentProfiles(r.Context(), actor, r.URL.Query().Get("workspace_id"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handler) upsertEnvironmentProfile(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var req adminplatform.UpsertEnvironmentProfileRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, err := h.platform.UpsertEnvironmentProfile(r.Context(), actor, r.PathValue("profileID"), r.Header.Get("If-Match"), req)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h Handler) getPlatformSettings(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	scope := r.URL.Query().Get("scope")
	workspaceID := r.URL.Query().Get("workspace_id")
	if scope == "workspace" && workspaceID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "workspace_id is required for workspace scope.")
		return
	}
	if scope != "" && scope != "organization" && scope != "workspace" {
		writeError(w, http.StatusBadRequest, "invalid_request", "scope must be organization or workspace.")
		return
	}
	item, err := h.platform.GetSettings(r.Context(), actor, workspaceID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h Handler) validatePlatformSettings(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var req adminplatform.SettingsApplyRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, err := h.platform.ValidateSettings(r.Context(), actor, req)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h Handler) applyPlatformSettings(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var req adminplatform.SettingsApplyRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, err := h.platform.ApplySettings(r.Context(), actor, r.Header.Get("If-Match"), req)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
