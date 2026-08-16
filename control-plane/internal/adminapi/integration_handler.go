package adminapi

import (
	"net/http"
	"strings"

	"github.com/AirSodaz/gantry/internal/adminintegration"
	"github.com/AirSodaz/gantry/internal/identity"
)

func (h Handler) listIntegrations(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.integrations.List(r.Context(), actor, r.URL.Query().Get("state"), r.URL.Query().Get("search"), r.URL.Query().Get("environment"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "page_info": map[string]any{"next_cursor": nil}})
}
func (h Handler) createIntegration(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var req adminintegration.CreateIntegrationRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, err := h.integrations.Create(r.Context(), actor, req)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}
func (h Handler) getIntegration(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	item, err := h.integrations.Get(r.Context(), actor, r.PathValue("integrationID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (h Handler) patchIntegration(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var req adminintegration.PatchIntegrationRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, err := h.integrations.Patch(r.Context(), actor, r.PathValue("integrationID"), req)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (h Handler) listIntegrationClients(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.integrations.ListClients(r.Context(), actor, r.PathValue("integrationID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (h Handler) createIntegrationClient(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var req adminintegration.CreateClientRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, err := h.integrations.CreateClient(r.Context(), actor, r.PathValue("integrationID"), req)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}
func (h Handler) integrationClientCommand(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	id, op, ok := strings.Cut(r.PathValue("clientID"), ":")
	if !ok {
		id = r.PathValue("clientID")
	}
	switch op {
	case "rotate":
		var req struct {
			CredentialFingerprint string `json:"credential_fingerprint"`
		}
		if err := decodeJSON(w, r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
			return
		}
		item, err := h.integrations.RotateClient(r.Context(), actor, id, req.CredentialFingerprint)
		if err != nil {
			h.writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, item)
	case "disable":
		if err := h.integrations.DisableClient(r.Context(), actor, id); err != nil {
			h.writeServiceError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}
func (h Handler) listIntegrationPublications(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.integrations.ListPublications(r.Context(), actor, r.PathValue("integrationID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (h Handler) createIntegrationPublication(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var req adminintegration.CreatePublicationRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, err := h.integrations.CreatePublication(r.Context(), actor, r.PathValue("integrationID"), req)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}
func (h Handler) integrationPublicationCommand(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	id, op, ok := strings.Cut(r.PathValue("publicationID"), ":")
	if !ok || op != "revoke" {
		http.NotFound(w, r)
		return
	}
	if err := h.integrations.RevokePublication(r.Context(), actor, id); err != nil {
		h.writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (h Handler) listIntegrationWebhooks(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.integrations.ListWebhooks(r.Context(), actor, r.PathValue("integrationID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (h Handler) createIntegrationWebhook(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var req adminintegration.CreateWebhookRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, err := h.integrations.CreateWebhook(r.Context(), actor, r.PathValue("integrationID"), req)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}
func (h Handler) webhookCommand(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	id, op, ok := strings.Cut(r.PathValue("endpointID"), ":")
	if !ok || op != "redeliver" {
		http.NotFound(w, r)
		return
	}
	var req struct {
		DeliveryID string `json:"delivery_id"`
	}
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, err := h.integrations.Redeliver(r.Context(), actor, id, req.DeliveryID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, item)
}
