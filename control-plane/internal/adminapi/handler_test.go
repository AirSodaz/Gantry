package adminapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/AirSodaz/gantry/internal/agentlifecycle"
	"github.com/AirSodaz/gantry/internal/authorization"
	"github.com/AirSodaz/gantry/internal/identity"
)

type fakeAuthenticator struct {
	actor identity.Principal
	err   error
}

func (a fakeAuthenticator) Authenticate(context.Context, string) (identity.Principal, error) {
	return a.actor, a.err
}

type fakeAuthorizer struct{ err error }

func (a fakeAuthorizer) RequireAdmin(context.Context, identity.Principal) error { return a.err }

type fakeLifecycleService struct {
	publish func(context.Context, identity.Principal, string, int) (agentlifecycle.Version, bool, error)
}

func (fakeLifecycleService) ListWorkspaces(context.Context, identity.Principal) ([]authorization.Workspace, error) {
	return nil, nil
}
func (fakeLifecycleService) ListAgents(context.Context, identity.Principal, string) ([]agentlifecycle.Agent, error) {
	return nil, nil
}
func (fakeLifecycleService) Create(context.Context, identity.Principal, agentlifecycle.CreateRequest) (agentlifecycle.Agent, error) {
	return agentlifecycle.Agent{}, agentlifecycle.ErrNotFound
}
func (fakeLifecycleService) Get(context.Context, identity.Principal, string) (agentlifecycle.Agent, error) {
	return agentlifecycle.Agent{}, agentlifecycle.ErrNotFound
}
func (fakeLifecycleService) GetDraft(context.Context, identity.Principal, string) (agentlifecycle.Draft, error) {
	return agentlifecycle.Draft{}, agentlifecycle.ErrNotFound
}
func (fakeLifecycleService) UpdateDraft(context.Context, identity.Principal, string, int, json.RawMessage) (agentlifecycle.Draft, error) {
	return agentlifecycle.Draft{}, agentlifecycle.ErrNotFound
}
func (fakeLifecycleService) ListVersions(context.Context, identity.Principal, string) ([]agentlifecycle.Version, error) {
	return nil, agentlifecycle.ErrNotFound
}
func (s fakeLifecycleService) Publish(ctx context.Context, actor identity.Principal, id string, revision int) (agentlifecycle.Version, bool, error) {
	if s.publish == nil {
		return agentlifecycle.Version{}, false, agentlifecycle.ErrNotFound
	}
	return s.publish(ctx, actor, id, revision)
}
func (fakeLifecycleService) Retire(context.Context, identity.Principal, string) error {
	return agentlifecycle.ErrNotFound
}

func TestAdminRoutesRequireAdminRole(t *testing.T) {
	handler := New(fakeAuthenticator{actor: identity.Principal{ID: "prn_1"}}, fakeAuthorizer{err: authorization.ErrForbidden}, fakeLifecycleService{}, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/workspaces", nil))
	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d", response.Code)
	}
}

func TestPublishUsesDraftRevisionAndReturnsIdempotentResponse(t *testing.T) {
	called := false
	handler := New(fakeAuthenticator{actor: identity.Principal{ID: "prn_1"}}, fakeAuthorizer{}, fakeLifecycleService{publish: func(_ context.Context, _ identity.Principal, id string, revision int) (agentlifecycle.Version, bool, error) {
		called = id == "agt_1" && revision == 3
		return agentlifecycle.Version{ID: "agtv_1"}, true, nil
	}}, nil)
	request := httptest.NewRequest(http.MethodPost, "/agents/agt_1:publish", nil)
	request.Header.Set("Authorization", "Bearer token")
	request.Header.Set("If-Match", `"3"`)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !called {
		t.Fatalf("status=%d called=%t", response.Code, called)
	}
}

func TestOutOfDomainResourcesAreNotEnumerated(t *testing.T) {
	handler := New(fakeAuthenticator{actor: identity.Principal{ID: "prn_1"}}, fakeAuthorizer{}, fakeLifecycleService{}, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/agents/agt_other", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("status=%d", response.Code)
	}
}

func TestRevisionConflictMapsToPreconditionFailed(t *testing.T) {
	handler := New(fakeAuthenticator{actor: identity.Principal{ID: "prn_1"}}, fakeAuthorizer{}, fakeLifecycleService{publish: func(context.Context, identity.Principal, string, int) (agentlifecycle.Version, bool, error) {
		return agentlifecycle.Version{}, false, agentlifecycle.ErrRevisionConflict
	}}, nil)
	request := httptest.NewRequest(http.MethodPost, "/agents/agt_1:publish", nil)
	request.Header.Set("If-Match", "2")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusPreconditionFailed {
		t.Fatalf("status=%d", response.Code)
	}
}

func TestAuthenticationFailureIsUnauthorized(t *testing.T) {
	handler := New(fakeAuthenticator{err: errors.New("invalid token")}, fakeAuthorizer{}, fakeLifecycleService{}, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/workspaces", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", response.Code)
	}
}
