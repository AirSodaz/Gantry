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

var _ interface {
	GetArtifact(context.Context, identity.Principal, string) (tasks.Artifact, error)
} = (*tasks.Service)(nil)
