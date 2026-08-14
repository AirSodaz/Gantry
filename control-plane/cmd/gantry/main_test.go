package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AirSodaz/gantry/internal/config"
)

type readyStore struct{}

func (readyStore) Ready(_ context.Context) error { return nil }

func TestPublicServerDoesNotExposeRunnerConnectRoute(t *testing.T) {
	server := publicServer(config.Config{}, readyStore{}, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	response := httptest.NewRecorder()
	server.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/gantry.runner.v1.RunnerSession/Session", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
}

func TestPublicServerDoesNotExposePhase0DevelopmentRoutesByDefault(t *testing.T) {
	server := publicServer(config.Config{}, readyStore{}, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	response := httptest.NewRecorder()
	server.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/internal/phase0/runs", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
}
