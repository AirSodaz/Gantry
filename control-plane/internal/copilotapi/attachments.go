package copilotapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/AirSodaz/gantry/internal/sessions"
)

type attachmentService interface {
	CreateAttachment(context.Context, identity.Principal, string, sessions.CreateAttachmentRequest) (sessions.Attachment, bool, error)
	GetAttachment(context.Context, identity.Principal, string) (sessions.Attachment, error)
	UploadAttachment(context.Context, identity.Principal, string, string, io.Reader) error
	CompleteAttachment(context.Context, identity.Principal, string, string) (sessions.Attachment, bool, error)
}

func (h Handler) attachments() (attachmentService, bool) {
	service, ok := h.sessions.(attachmentService)
	return service, ok
}

func (h Handler) createAttachment(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	service, ok := h.attachments()
	if !ok {
		writeInternal(w, errors.New("attachment service is unavailable"))
		return
	}
	var request sessions.CreateAttachmentRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, duplicate, err := service.CreateAttachment(r.Context(), actor, r.Header.Get("Idempotency-Key"), request)
	if err != nil {
		writeAttachmentError(w, err)
		return
	}
	status := http.StatusCreated
	if duplicate {
		status = http.StatusOK
	}
	writeJSON(w, status, sessions.AttachmentUploadGrant{Attachment: item, UploadPath: "/attachments/" + item.ID + "/content", UploadToken: item.UploadToken, ExpiresAt: item.UploadExpires})
}

func (h Handler) getAttachment(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	service, ok := h.attachments()
	if !ok {
		writeInternal(w, errors.New("attachment service is unavailable"))
		return
	}
	item, err := service.GetAttachment(r.Context(), actor, r.PathValue("attachmentID"))
	if err != nil {
		writeAttachmentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h Handler) uploadAttachment(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	service, ok := h.attachments()
	if !ok {
		writeInternal(w, errors.New("attachment service is unavailable"))
		return
	}
	if r.Body == nil || r.ContentLength > 64<<20 {
		writeError(w, http.StatusBadRequest, "invalid_attachment", "Attachment content is invalid.")
		return
	}
	err := service.UploadAttachment(r.Context(), actor, r.PathValue("attachmentID"), strings.TrimSpace(r.Header.Get("X-Gantry-Upload-Token")), r.Body)
	if err != nil {
		writeAttachmentError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handler) completeAttachment(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	service, ok := h.attachments()
	if !ok {
		writeInternal(w, errors.New("attachment service is unavailable"))
		return
	}
	attachmentID, ok := operationTarget(r.PathValue("operation"), ":complete")
	if !ok {
		http.NotFound(w, r)
		return
	}
	item, duplicate, err := service.CompleteAttachment(r.Context(), actor, attachmentID, r.Header.Get("Idempotency-Key"))
	if err != nil {
		writeAttachmentError(w, err)
		return
	}
	status := http.StatusAccepted
	if duplicate {
		status = http.StatusOK
	}
	writeJSON(w, status, item)
}

func writeAttachmentError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, sessions.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Attachment was not found.")
	case errors.Is(err, sessions.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "invalid_attachment", "Attachment metadata or content is invalid.")
	case errors.Is(err, sessions.ErrInvalidState):
		writeError(w, http.StatusConflict, "attachment_not_ready", "Attachment is not ready for this operation.")
	case errors.Is(err, sessions.ErrIdempotencyConflict):
		writeError(w, http.StatusConflict, "idempotency_conflict", "The idempotency key was used for a different attachment command.")
	default:
		writeInternal(w, err)
	}
}
