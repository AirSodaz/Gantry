package adminapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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
	publish      func(context.Context, identity.Principal, string, int) (agentlifecycle.Version, bool, error)
	submitReview func(context.Context, identity.Principal, string, int, string) (agentlifecycle.Review, error)
	decideReview func(context.Context, identity.Principal, string, string, string) (agentlifecycle.Review, error)
	rollback     func(context.Context, identity.Principal, string, string) error
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
func (fakeLifecycleService) GetReview(context.Context, identity.Principal, string) (agentlifecycle.Review, error) {
	return agentlifecycle.Review{}, agentlifecycle.ErrNotFound
}
func (s fakeLifecycleService) SubmitReview(ctx context.Context, actor identity.Principal, id string, revision int, notes string) (agentlifecycle.Review, error) {
	if s.submitReview == nil {
		return agentlifecycle.Review{}, agentlifecycle.ErrNotFound
	}
	return s.submitReview(ctx, actor, id, revision, notes)
}
func (s fakeLifecycleService) DecideReview(ctx context.Context, actor identity.Principal, id, decision, reason string) (agentlifecycle.Review, error) {
	if s.decideReview == nil {
		return agentlifecycle.Review{}, agentlifecycle.ErrNotFound
	}
	return s.decideReview(ctx, actor, id, decision, reason)
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
func (s fakeLifecycleService) Rollback(ctx context.Context, actor identity.Principal, id, versionID string) error {
	if s.rollback == nil {
		return agentlifecycle.ErrNotFound
	}
	return s.rollback(ctx, actor, id, versionID)
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

func TestSubmitReviewUsesDraftRevisionAndNotes(t *testing.T) {
	called := false
	handler := New(fakeAuthenticator{actor: identity.Principal{ID: "prn_1"}}, fakeAuthorizer{}, fakeLifecycleService{submitReview: func(_ context.Context, _ identity.Principal, id string, revision int, notes string) (agentlifecycle.Review, error) {
		called = id == "agt_1" && revision == 3 && notes == "release"
		return agentlifecycle.Review{AgentID: id, DraftRevision: revision, Status: "pending"}, nil
	}}, nil)
	request := httptest.NewRequest(http.MethodPost, "/agents/agt_1:review", strings.NewReader(`{"release_notes":"release"}`))
	request.Header.Set("If-Match", `"3"`)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated || !called {
		t.Fatalf("status=%d called=%t", response.Code, called)
	}
}

func TestReviewDecisionAndRollbackRoutesForwardBodies(t *testing.T) {
	decisionCalled, rollbackCalled := false, false
	handler := New(fakeAuthenticator{actor: identity.Principal{ID: "prn_1"}}, fakeAuthorizer{}, fakeLifecycleService{
		decideReview: func(_ context.Context, _ identity.Principal, id, decision, reason string) (agentlifecycle.Review, error) {
			decisionCalled = id == "agt_1" && decision == "approve" && reason == "looks good"
			return agentlifecycle.Review{AgentID: id, Status: "approved"}, nil
		},
		rollback: func(_ context.Context, _ identity.Principal, id, versionID string) error {
			rollbackCalled = id == "agt_1" && versionID == "agtv_1"
			return nil
		},
	}, nil)
	decision := httptest.NewRequest(http.MethodPost, "/agents/agt_1:review-decision", strings.NewReader(`{"decision":"approve","reason":"looks good"}`))
	decisionResponse := httptest.NewRecorder()
	handler.ServeHTTP(decisionResponse, decision)
	rollback := httptest.NewRequest(http.MethodPost, "/agents/agt_1:rollback", strings.NewReader(`{"version_id":"agtv_1"}`))
	rollbackResponse := httptest.NewRecorder()
	handler.ServeHTTP(rollbackResponse, rollback)
	if decisionResponse.Code != http.StatusOK || rollbackResponse.Code != http.StatusNoContent || !decisionCalled || !rollbackCalled {
		t.Fatalf("decision=%d rollback=%d decisionCalled=%t rollbackCalled=%t", decisionResponse.Code, rollbackResponse.Code, decisionCalled, rollbackCalled)
	}
}
