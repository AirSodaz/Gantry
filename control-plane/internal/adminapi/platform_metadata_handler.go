package adminapi

import (
	"net/http"
	"strings"

	"github.com/AirSodaz/gantry/internal/adminplatform"
	"github.com/AirSodaz/gantry/internal/identity"
)

func (h Handler) listPlatformCredentials(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.platform.ListCredentials(r.Context(), actor)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (h Handler) platformCredentialCommand(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	id, op, ok := strings.Cut(r.PathValue("credentialID"), ":")
	if !ok || (op != "rotate" && op != "revoke") {
		http.NotFound(w, r)
		return
	}
	var item adminplatform.CredentialReference
	var err error
	if op == "rotate" {
		item, err = h.platform.RotateCredential(r.Context(), actor, id)
	} else {
		item, err = h.platform.RevokeCredential(r.Context(), actor, id)
	}
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (h Handler) listDataClassifications(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.platform.ListClassifications(r.Context(), actor)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
func (h Handler) createDataClassification(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var req adminplatform.CreateDataClassificationRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, err := h.platform.CreateClassification(r.Context(), actor, req)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}
