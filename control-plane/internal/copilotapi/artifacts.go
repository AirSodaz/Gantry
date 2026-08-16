package copilotapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/AirSodaz/gantry/internal/tasks"
)

func (h Handler) getArtifact(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	reader, ok := h.tasks.(artifactReader)
	if !ok {
		writeInternal(w, errors.New("artifact service is unavailable"))
		return
	}
	artifact, err := reader.GetArtifact(r.Context(), actor, r.PathValue("artifactID"))
	if err != nil {
		if errors.Is(err, tasks.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Artifact was not found.")
			return
		}
		writeInternal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, artifact)
}

func (h Handler) downloadArtifact(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	reader, ok := h.tasks.(artifactDownloadService)
	if !ok {
		writeInternal(w, errors.New("artifact service is unavailable"))
		return
	}
	artifactID, ok := operationTarget(r.PathValue("operation"), ":download")
	if !ok {
		http.NotFound(w, r)
		return
	}
	grant, err := reader.DownloadArtifact(r.Context(), actor, artifactID)
	if errors.Is(err, tasks.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "Artifact was not found.")
		return
	}
	if errors.Is(err, tasks.ErrInvalidState) {
		writeError(w, http.StatusConflict, "artifact_unavailable", "The artifact is not available for download.")
		return
	}
	if err != nil {
		writeInternal(w, err)
		return
	}
	writeJSON(w, http.StatusOK, grant)
}

func (h Handler) listArtifacts(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	reader, ok := h.tasks.(artifactListReader)
	if !ok {
		writeInternal(w, errors.New("artifact service is unavailable"))
		return
	}
	taskID, classification, state := r.URL.Query().Get("task_id"), r.URL.Query().Get("classification"), r.URL.Query().Get("state")
	after, ok := h.parseArtifactListCursor(r.URL.Query().Get("cursor"), actor, taskID, classification, state)
	if !ok {
		writeError(w, http.StatusBadRequest, "cursor_invalid", "The artifact cursor is not valid for this requester or filter.")
		return
	}
	page, err := reader.ListMyArtifacts(r.Context(), actor, taskID, classification, state, after, limit(r))
	if err != nil {
		writeInternal(w, err)
		return
	}
	pageInfo := map[string]any{"has_more": page.HasMore}
	if page.HasMore {
		last := page.Items[len(page.Items)-1]
		pageInfo["next_cursor"] = h.encodeArtifactListCursor(actor, taskID, classification, state, tasks.ArtifactCursor{CreatedAt: last.CreatedAt, ID: last.ID})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": page.Items, "page_info": pageInfo})
}

var _ interface {
	GetArtifact(context.Context, identity.Principal, string) (tasks.Artifact, error)
} = (*tasks.Service)(nil)

type artifactListReader interface {
	ListMyArtifacts(context.Context, identity.Principal, string, string, string, *tasks.ArtifactCursor, int) (tasks.ArtifactPage, error)
}

type artifactDownloadService interface {
	DownloadArtifact(context.Context, identity.Principal, string) (tasks.ArtifactDownloadGrant, error)
}

var _ artifactListReader = (*tasks.Service)(nil)
var _ artifactDownloadService = (*tasks.Service)(nil)
