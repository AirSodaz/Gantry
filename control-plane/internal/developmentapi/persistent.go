package developmentapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/AirSodaz/gantry/internal/development"
	"github.com/AirSodaz/gantry/internal/tasks"
)

type persistentDispatcher interface {
	Dispatch(context.Context) error
	RequestCancel(string, uint64, string) bool
}

// NewHandler exposes the development-only lifecycle route while sending its
// deterministic runs through the durable task and runner path.
func NewHandler(token string, lifecycle *development.Lifecycle, dispatcher persistentDispatcher, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	h := handler{token: token, lifecycle: lifecycle, dispatcher: dispatcher, logger: logger}
	mux := http.NewServeMux()
	mux.Handle("POST /internal/development/runs", h.authorize(http.HandlerFunc(h.create)))
	mux.Handle("GET /internal/development/runs/{runID}", h.authorize(http.HandlerFunc(h.get)))
	mux.Handle("POST /internal/development/runs/{runID}/cancel", h.authorize(http.HandlerFunc(h.cancel)))
	return mux
}

type handler struct {
	token      string
	lifecycle  *development.Lifecycle
	dispatcher persistentDispatcher
	logger     *slog.Logger
}

type createRunRequest struct {
	Mode string `json:"mode"`
}
type runResponse struct {
	RunID                     string `json:"run_id"`
	Status                    string `json:"status"`
	LeaseEpoch                uint64 `json:"lease_epoch"`
	AcknowledgedEventSequence uint64 `json:"acknowledged_event_sequence"`
}

func (h handler) create(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	decoder.DisallowUnknownFields()
	var request createRunRequest
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request")
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid JSON request")
		return
	}
	run, err := h.lifecycle.Start(r.Context(), request.Mode)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid development scenario")
		return
	}
	if err := h.dispatcher.Dispatch(r.Context()); err != nil {
		h.logger.Error("development durable dispatch failed", "error", err, "run_id", run.Run.ID)
	}
	writeJSON(w, http.StatusCreated, runResponse{RunID: run.Run.ID, Status: run.Run.Status, LeaseEpoch: run.Run.LeaseEpoch, AcknowledgedEventSequence: run.Run.AcknowledgedEventSequence})
}
func (h handler) get(w http.ResponseWriter, r *http.Request) {
	run, err := h.lifecycle.Get(r.Context(), r.PathValue("runID"))
	if errors.Is(err, tasks.ErrNotFound) {
		writeError(w, http.StatusNotFound, "development run not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not read development run")
		return
	}
	writeJSON(w, http.StatusOK, runResponse{RunID: run.Run.ID, Status: run.Run.Status, LeaseEpoch: run.Run.LeaseEpoch, AcknowledgedEventSequence: run.Run.AcknowledgedEventSequence})
}
func (h handler) cancel(w http.ResponseWriter, r *http.Request) {
	result, err := h.lifecycle.Cancel(r.Context(), r.PathValue("runID"))
	if errors.Is(err, tasks.ErrNotFound) {
		writeError(w, http.StatusNotFound, "development run not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if result.Deliver {
		h.dispatcher.RequestCancel(result.Run.ID, result.Run.LeaseEpoch, "development API request")
	}
	writeJSON(w, http.StatusAccepted, runResponse{RunID: result.Run.ID, Status: result.Run.Status, LeaseEpoch: result.Run.LeaseEpoch, AcknowledgedEventSequence: result.Run.AcknowledgedEventSequence})
}
func (h handler) authorize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if provided == r.Header.Get("Authorization") || provided != h.token {
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeError(w, http.StatusUnauthorized, "missing or invalid bearer token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
