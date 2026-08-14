package copilotapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/AirSodaz/gantry/internal/tasks"
)

type fakeAuthenticator struct {
	principal identity.Principal
	err       error
}

func (a fakeAuthenticator) Authenticate(context.Context, string) (identity.Principal, error) {
	return a.principal, a.err
}

type fakeTaskService struct {
	submit func(context.Context, identity.Principal, string, tasks.SubmitRequest) (tasks.Task, bool, error)
	get    func(context.Context, identity.Principal, string) (tasks.Task, error)
	cancel func(context.Context, identity.Principal, string, string) (tasks.CancelResult, error)
	retry  func(context.Context, identity.Principal, string, bool) (tasks.Task, error)
}

func (s fakeTaskService) ListAgents(context.Context, identity.Principal, string, string, int) ([]tasks.Agent, error) {
	return nil, nil
}

func (s fakeTaskService) Submit(ctx context.Context, actor identity.Principal, key string, request tasks.SubmitRequest) (tasks.Task, bool, error) {
	if s.submit == nil {
		return tasks.Task{}, false, errors.New("unexpected submit")
	}
	return s.submit(ctx, actor, key, request)
}

func (s fakeTaskService) List(context.Context, identity.Principal, string, int) ([]tasks.Task, error) {
	return nil, nil
}

func (s fakeTaskService) Get(ctx context.Context, actor identity.Principal, taskID string) (tasks.Task, error) {
	if s.get == nil {
		return tasks.Task{}, tasks.ErrNotFound
	}
	return s.get(ctx, actor, taskID)
}

func (s fakeTaskService) Cancel(ctx context.Context, actor identity.Principal, taskID, runID string) (tasks.CancelResult, error) {
	if s.cancel != nil {
		return s.cancel(ctx, actor, taskID, runID)
	}
	return tasks.CancelResult{}, tasks.ErrNotFound
}

func (s fakeTaskService) Retry(ctx context.Context, actor identity.Principal, taskID string, useLatest bool) (tasks.Task, error) {
	if s.retry != nil {
		return s.retry(ctx, actor, taskID, useLatest)
	}
	return tasks.Task{}, tasks.ErrNotFound
}

type fakeDispatcher struct {
	dispatches    int
	canceledRun   string
	canceledEpoch uint64
}

func (d *fakeDispatcher) Dispatch(context.Context) error {
	d.dispatches++
	return nil
}

func (d *fakeDispatcher) RequestCancel(runID string, epoch uint64, _ string) bool {
	d.canceledRun = runID
	d.canceledEpoch = epoch
	return true
}

func TestSubmitTaskUsesHeaderIdempotencyKey(t *testing.T) {
	dispatcher := &fakeDispatcher{}
	var receivedKey string
	var receivedRequest tasks.SubmitRequest
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeTaskService{submit: func(_ context.Context, actor identity.Principal, key string, request tasks.SubmitRequest) (tasks.Task, bool, error) {
			if actor.ID != "principal-1" {
				t.Fatalf("actor ID = %q", actor.ID)
			}
			receivedKey = key
			receivedRequest = request
			return tasks.Task{ID: "tsk_1", Status: "queued", CurrentRun: tasks.Run{ID: "run_1", Status: "queued"}}, false, nil
		}},
		dispatcher,
		nil,
	)

	request := httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(`{"agent_id":"agt_lifecycle_demo","message":"hello"}`))
	request.Header.Set("Authorization", "Bearer access-token")
	request.Header.Set("Idempotency-Key", "retry-safe-key")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if receivedKey != "retry-safe-key" || receivedRequest.Message != "hello" {
		t.Fatalf("received key=%q request=%#v", receivedKey, receivedRequest)
	}
	if dispatcher.dispatches != 1 {
		t.Fatalf("dispatches = %d", dispatcher.dispatches)
	}
}

func TestSubmitTaskRejectsBodyIdempotencyKey(t *testing.T) {
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeTaskService{},
		&fakeDispatcher{},
		nil,
	)
	request := httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(`{"agent_id":"agt_lifecycle_demo","message":"hello","idempotency_key":"not-supported"}`))
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestSubmitTaskReturnsOKForIdempotentRetry(t *testing.T) {
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeTaskService{submit: func(context.Context, identity.Principal, string, tasks.SubmitRequest) (tasks.Task, bool, error) {
			return tasks.Task{ID: "tsk_1"}, true, nil
		}},
		&fakeDispatcher{},
		nil,
	)
	request := httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(`{"agent_id":"agt_lifecycle_demo","message":"hello"}`))
	request.Header.Set("Authorization", "Bearer access-token")
	request.Header.Set("Idempotency-Key", "retry-safe-key")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestTaskLookupDoesNotExposeOtherUsersResources(t *testing.T) {
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-2"}},
		fakeTaskService{get: func(context.Context, identity.Principal, string) (tasks.Task, error) {
			return tasks.Task{}, tasks.ErrNotFound
		}},
		&fakeDispatcher{},
		nil,
	)
	request := httptest.NewRequest(http.MethodGet, "/tasks/tsk_other_user", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestCancelOperationParsesColonSuffix(t *testing.T) {
	dispatcher := &fakeDispatcher{}
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeTaskService{cancel: func(_ context.Context, _ identity.Principal, taskID, runID string) (tasks.CancelResult, error) {
			if taskID != "tsk_1" || runID != "run_1" {
				t.Fatalf("task=%q run=%q", taskID, runID)
			}
			return tasks.CancelResult{Run: tasks.Run{ID: runID, Status: "canceling", LeaseEpoch: 7}, Deliver: true}, nil
		}},
		dispatcher,
		nil,
	)
	request := httptest.NewRequest(http.MethodPost, "/tasks/tsk_1/runs/run_1:cancel", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if dispatcher.canceledRun != "run_1" || dispatcher.canceledEpoch != 7 {
		t.Fatalf("cancel delivery = %#v", dispatcher)
	}
}

func TestRetryOperationMapsInvalidStateToConflict(t *testing.T) {
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeTaskService{retry: func(_ context.Context, _ identity.Principal, taskID string, useLatest bool) (tasks.Task, error) {
			if taskID != "tsk_1" || !useLatest {
				t.Fatalf("task=%q useLatest=%t", taskID, useLatest)
			}
			return tasks.Task{}, tasks.ErrInvalidState
		}},
		&fakeDispatcher{},
		nil,
	)
	request := httptest.NewRequest(http.MethodPost, "/tasks/tsk_1:retry", strings.NewReader(`{"use_latest_version":true}`))
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestCopilotRoutesRequireBearerAuthentication(t *testing.T) {
	handler := New(fakeAuthenticator{err: identity.ErrUnauthorized}, fakeTaskService{}, &fakeDispatcher{}, nil)
	request := httptest.NewRequest(http.MethodGet, "/agents", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", response.Code)
	}
}
