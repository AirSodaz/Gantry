package copilotapi

import (
	"errors"
	"net/http"

	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/AirSodaz/gantry/internal/runs"
)

func (h Handler) getArtifact(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	if h.artifacts == nil {
		writeInternal(w, errors.New("artifact service is unavailable"))
		return
	}
	item, err := h.artifacts.GetArtifact(r.Context(), actor, r.PathValue("artifactID"))
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}
func (h Handler) downloadArtifact(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	if h.artifacts == nil {
		writeInternal(w, errors.New("artifact service is unavailable"))
		return
	}
	id, ok := operationTarget(r.PathValue("operation"), ":download")
	if !ok {
		http.NotFound(w, r)
		return
	}
	grant, err := h.artifacts.DownloadArtifact(r.Context(), actor, id)
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, grant)
}
func (h Handler) listArtifacts(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	if h.artifacts == nil {
		writeInternal(w, errors.New("artifact service is unavailable"))
		return
	}
	sessionID, classification, state := r.URL.Query().Get("session_id"), r.URL.Query().Get("classification"), r.URL.Query().Get("state")
	after, ok := h.parseArtifactListCursor(r.URL.Query().Get("cursor"), actor, sessionID, classification, state)
	if !ok {
		writeError(w, http.StatusBadRequest, "cursor_invalid", "The artifact cursor is not valid for this Session member or filter.")
		return
	}
	page, err := h.artifacts.ListMyArtifacts(r.Context(), actor, sessionID, classification, state, after, limit(r))
	if err != nil {
		writeArtifactError(w, err)
		return
	}
	info := map[string]any{"has_more": page.HasMore}
	if page.HasMore {
		last := page.Items[len(page.Items)-1]
		info["next_cursor"] = h.encodeArtifactListCursor(actor, sessionID, classification, state, runs.ArtifactCursor{CreatedAt: last.CreatedAt, ID: last.ID})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": page.Items, "page_info": info})
}
func writeArtifactError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, runs.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Artifact was not found.")
	case errors.Is(err, runs.ErrInvalidState):
		writeError(w, http.StatusConflict, "artifact_unavailable", "The artifact is not available for download.")
	default:
		writeInternal(w, err)
	}
}
