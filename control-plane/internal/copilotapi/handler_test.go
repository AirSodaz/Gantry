package copilotapi

import (
	"context"
	"errors"
	"github.com/AirSodaz/gantry/internal/approvals"
	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/AirSodaz/gantry/internal/tasks"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeAuthenticator struct {
	principal identity.Principal
	err       error
}

func (a fakeAuthenticator) Authenticate(context.Context, string) (identity.Principal, error) {
	return a.principal, a.err
}

type fakeTaskService struct {
	submit             func(context.Context, identity.Principal, string, tasks.SubmitRequest) (tasks.Task, bool, error)
	list               func(context.Context, identity.Principal, tasks.ListFilter, *tasks.TaskCursor, int) (tasks.TaskPage, error)
	get                func(context.Context, identity.Principal, string) (tasks.Task, error)
	append             func(context.Context, identity.Principal, string, string, int64, tasks.AppendMessageRequest) (tasks.Task, bool, error)
	runs               func(context.Context, identity.Principal, string, int) ([]tasks.RunAttempt, error)
	artifacts          func(context.Context, identity.Principal, string, string, *tasks.ArtifactCursor, int) (tasks.ArtifactPage, error)
	downloadArtifact   func(context.Context, identity.Principal, string) (tasks.ArtifactDownloadGrant, error)
	createAttachment   func(context.Context, identity.Principal, tasks.CreateAttachmentRequest) (tasks.Attachment, error)
	getAttachment      func(context.Context, identity.Principal, string) (tasks.Attachment, error)
	uploadAttachment   func(context.Context, identity.Principal, string, string, io.Reader) error
	completeAttachment func(context.Context, identity.Principal, string) (tasks.Attachment, error)
	cancel             func(context.Context, identity.Principal, string, string, string) (tasks.CancelResult, error)
	retry              func(context.Context, identity.Principal, string, bool, string, int64) (tasks.Task, error)
}

type fakeApprovalService struct {
	list   func(context.Context, identity.Principal, *approvals.Cursor, int) (approvals.Page, error)
	get    func(context.Context, identity.Principal, string) (approvals.Request, error)
	expire func(context.Context, identity.Principal) ([]approvals.Resolution, error)
	decide func(context.Context, identity.Principal, approvals.DecisionInput) (approvals.Resolution, error)
}

func (s fakeApprovalService) Get(ctx context.Context, actor identity.Principal, approvalID string) (approvals.Request, error) {
	if s.get != nil {
		return s.get(ctx, actor, approvalID)
	}
	return approvals.Request{}, approvals.ErrNotFound
}
func (s fakeApprovalService) Expire(ctx context.Context, actor identity.Principal) ([]approvals.Resolution, error) {
	if s.expire != nil {
		return s.expire(ctx, actor)
	}
	return nil, nil
}

func (s fakeApprovalService) List(ctx context.Context, actor identity.Principal, after *approvals.Cursor, limit int) (approvals.Page, error) {
	if s.list != nil {
		return s.list(ctx, actor, after, limit)
	}
	return approvals.Page{}, nil
}
func (s fakeApprovalService) Decide(ctx context.Context, actor identity.Principal, input approvals.DecisionInput) (approvals.Resolution, error) {
	if s.decide != nil {
		return s.decide(ctx, actor, input)
	}
	return approvals.Resolution{}, approvals.ErrNotFound
}

func (s fakeTaskService) ListAgents(context.Context, identity.Principal, string, string, *tasks.AgentCursor, int) (tasks.AgentPage, error) {
	return tasks.AgentPage{}, nil
}

func (s fakeTaskService) Submit(ctx context.Context, actor identity.Principal, key string, request tasks.SubmitRequest) (tasks.Task, bool, error) {
	if s.submit == nil {
		return tasks.Task{}, false, errors.New("unexpected submit")
	}
	return s.submit(ctx, actor, key, request)
}

func (s fakeTaskService) List(ctx context.Context, actor identity.Principal, filter tasks.ListFilter, after *tasks.TaskCursor, limit int) (tasks.TaskPage, error) {
	if s.list != nil {
		return s.list(ctx, actor, filter, after, limit)
	}
	return tasks.TaskPage{}, nil
}

func (s fakeTaskService) Get(ctx context.Context, actor identity.Principal, taskID string) (tasks.Task, error) {
	if s.get == nil {
		return tasks.Task{}, tasks.ErrNotFound
	}
	return s.get(ctx, actor, taskID)
}

func (s fakeTaskService) AppendMessage(ctx context.Context, actor identity.Principal, taskID, key string, revision int64, request tasks.AppendMessageRequest) (tasks.Task, bool, error) {
	if s.append != nil {
		return s.append(ctx, actor, taskID, key, revision, request)
	}
	return tasks.Task{}, false, tasks.ErrNotFound
}

func (s fakeTaskService) ListRuns(ctx context.Context, actor identity.Principal, taskID string, limit int) ([]tasks.RunAttempt, error) {
	if s.runs != nil {
		return s.runs(ctx, actor, taskID, limit)
	}
	return nil, tasks.ErrNotFound
}

func (s fakeTaskService) ListMyArtifacts(ctx context.Context, actor identity.Principal, taskID, classification string, after *tasks.ArtifactCursor, limit int) (tasks.ArtifactPage, error) {
	if s.artifacts != nil {
		return s.artifacts(ctx, actor, taskID, classification, after, limit)
	}
	return tasks.ArtifactPage{}, nil
}

func (s fakeTaskService) DownloadArtifact(ctx context.Context, actor identity.Principal, artifactID string) (tasks.ArtifactDownloadGrant, error) {
	if s.downloadArtifact != nil {
		return s.downloadArtifact(ctx, actor, artifactID)
	}
	return tasks.ArtifactDownloadGrant{}, tasks.ErrNotFound
}

func (s fakeTaskService) CreateAttachment(ctx context.Context, actor identity.Principal, request tasks.CreateAttachmentRequest) (tasks.Attachment, error) {
	if s.createAttachment != nil {
		return s.createAttachment(ctx, actor, request)
	}
	return tasks.Attachment{}, tasks.ErrNotFound
}

func (s fakeTaskService) GetAttachment(ctx context.Context, actor identity.Principal, attachmentID string) (tasks.Attachment, error) {
	if s.getAttachment != nil {
		return s.getAttachment(ctx, actor, attachmentID)
	}
	return tasks.Attachment{}, tasks.ErrNotFound
}

func (s fakeTaskService) UploadAttachment(ctx context.Context, actor identity.Principal, attachmentID, token string, body io.Reader) error {
	if s.uploadAttachment != nil {
		return s.uploadAttachment(ctx, actor, attachmentID, token, body)
	}
	return tasks.ErrNotFound
}

func (s fakeTaskService) CompleteAttachment(ctx context.Context, actor identity.Principal, attachmentID string) (tasks.Attachment, error) {
	if s.completeAttachment != nil {
		return s.completeAttachment(ctx, actor, attachmentID)
	}
	return tasks.Attachment{}, tasks.ErrNotFound
}

func (s fakeTaskService) Cancel(ctx context.Context, actor identity.Principal, taskID, runID, key string) (tasks.CancelResult, error) {
	if s.cancel != nil {
		return s.cancel(ctx, actor, taskID, runID, key)
	}
	return tasks.CancelResult{}, tasks.ErrNotFound
}

func (s fakeTaskService) Retry(ctx context.Context, actor identity.Principal, taskID string, useLatest bool, key string, revision int64) (tasks.Task, error) {
	if s.retry != nil {
		return s.retry(ctx, actor, taskID, useLatest, key, revision)
	}
	return tasks.Task{}, tasks.ErrNotFound
}

type fakeDispatcher struct {
	dispatches       int
	canceledRun      string
	canceledEpoch    uint64
	resolvedRun      string
	resolvedDecision string
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
func (d *fakeDispatcher) ResolveApproval(runID, _ string, decision, _ string, _ string, _ string, _ string, _ uint64, _ time.Time) bool {
	d.resolvedRun = runID
	d.resolvedDecision = decision
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
		nil,
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
		nil,
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
		nil,
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

func TestGetTaskReturnsConversationETag(t *testing.T) {
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeTaskService{get: func(context.Context, identity.Principal, string) (tasks.Task, error) {
			return tasks.Task{ID: "tsk_1", ConversationRevision: 7}, nil
		}},
		nil,
		&fakeDispatcher{},
		nil,
	)
	request := httptest.NewRequest(http.MethodGet, "/tasks/tsk_1", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("ETag") != `"7"` {
		t.Fatalf("status=%d etag=%q body=%s", response.Code, response.Header().Get("ETag"), response.Body.String())
	}
}

func TestAppendMessageRequiresConversationETag(t *testing.T) {
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeTaskService{},
		nil,
		&fakeDispatcher{},
		nil,
	)
	request := httptest.NewRequest(http.MethodPost, "/tasks/tsk_1/messages", strings.NewReader(`{"message":"Use a different target"}`))
	request.Header.Set("Authorization", "Bearer access-token")
	request.Header.Set("Idempotency-Key", "follow-up-1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusPreconditionRequired || !strings.Contains(response.Body.String(), "conversation_etag_required") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestAppendMessageReturnsCurrentTaskOnConversationConflict(t *testing.T) {
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeTaskService{
			append: func(context.Context, identity.Principal, string, string, int64, tasks.AppendMessageRequest) (tasks.Task, bool, error) {
				return tasks.Task{}, false, tasks.ErrConversationChanged
			},
			get: func(context.Context, identity.Principal, string) (tasks.Task, error) {
				return tasks.Task{ID: "tsk_1", Status: "awaiting_requester_input", ConversationRevision: 8}, nil
			},
		},
		nil,
		&fakeDispatcher{},
		nil,
	)
	request := httptest.NewRequest(http.MethodPost, "/tasks/tsk_1/messages", strings.NewReader(`{"message":"Use a different target"}`))
	request.Header.Set("Authorization", "Bearer access-token")
	request.Header.Set("Idempotency-Key", "follow-up-1")
	request.Header.Set("If-Match", `"7"`)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusConflict || response.Header().Get("ETag") != `"8"` || !strings.Contains(response.Body.String(), "conversation_changed") || !strings.Contains(response.Body.String(), `"current_resource"`) {
		t.Fatalf("status=%d etag=%q body=%s", response.Code, response.Header().Get("ETag"), response.Body.String())
	}
}

func TestCreateAttachmentReturnsOnlyTheRequestersUploadGrant(t *testing.T) {
	var received tasks.CreateAttachmentRequest
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1", OrganizationID: "org-1"}},
		fakeTaskService{createAttachment: func(_ context.Context, actor identity.Principal, request tasks.CreateAttachmentRequest) (tasks.Attachment, error) {
			if actor.ID != "principal-1" {
				t.Fatalf("actor=%q", actor.ID)
			}
			received = request
			return tasks.Attachment{ID: "att_1", Filename: request.Filename, State: "declared", ScanStatus: "pending", UploadURL: "/api/copilot/v1/attachments/att_1/content", UploadToken: "short-lived"}, nil
		}},
		nil,
		&fakeDispatcher{},
		nil,
	)
	request := httptest.NewRequest(http.MethodPost, "/attachments", strings.NewReader(`{"filename":"brief.txt","media_type":"text/plain","size_bytes":5,"digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`))
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if received.Filename != "brief.txt" || received.SizeBytes != 5 {
		t.Fatalf("request=%#v", received)
	}
	if !strings.Contains(response.Body.String(), `"upload_token":"short-lived"`) {
		t.Fatalf("body=%s", response.Body.String())
	}
}

func TestCompleteAttachmentParsesColonSuffix(t *testing.T) {
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeTaskService{completeAttachment: func(_ context.Context, _ identity.Principal, attachmentID string) (tasks.Attachment, error) {
			if attachmentID != "att_1" {
				t.Fatalf("attachment=%q", attachmentID)
			}
			return tasks.Attachment{ID: attachmentID, State: "available", ScanStatus: "passed"}, nil
		}},
		nil,
		&fakeDispatcher{},
		nil,
	)
	request := httptest.NewRequest(http.MethodPost, "/attachments/att_1:complete", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestTaskLookupDoesNotExposeOtherUsersResources(t *testing.T) {
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-2"}},
		fakeTaskService{get: func(context.Context, identity.Principal, string) (tasks.Task, error) {
			return tasks.Task{}, tasks.ErrNotFound
		}},
		nil,
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

func TestListTasksParsesRequesterFilters(t *testing.T) {
	var received tasks.ListFilter
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeTaskService{list: func(_ context.Context, _ identity.Principal, filter tasks.ListFilter, _ *tasks.TaskCursor, _ int) (tasks.TaskPage, error) {
			received = filter
			return tasks.TaskPage{}, nil
		}},
		nil,
		&fakeDispatcher{},
		nil,
	)
	request := httptest.NewRequest(http.MethodGet, "/tasks?status=completed&agent_id=agt_1&requester_action=input&created_after=2026-08-16T00:00:00Z", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	if received.Status != "completed" || received.AgentID != "agt_1" || received.RequesterAction != "input" || received.CreatedAfter == nil {
		t.Fatalf("filter = %#v", received)
	}
}

func TestCancelOperationParsesColonSuffix(t *testing.T) {
	dispatcher := &fakeDispatcher{}
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeTaskService{cancel: func(_ context.Context, _ identity.Principal, taskID, runID, key string) (tasks.CancelResult, error) {
			if taskID != "tsk_1" || runID != "run_1" || key != "cancel-1" {
				t.Fatalf("task=%q run=%q key=%q", taskID, runID, key)
			}
			return tasks.CancelResult{Run: tasks.Run{ID: runID, Status: "canceling", LeaseEpoch: 7}, Deliver: true}, nil
		}},
		nil,
		dispatcher,
		nil,
	)
	request := httptest.NewRequest(http.MethodPost, "/tasks/tsk_1/runs/run_1:cancel", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	request.Header.Set("Idempotency-Key", "cancel-1")
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
		fakeTaskService{retry: func(_ context.Context, _ identity.Principal, taskID string, useLatest bool, key string, revision int64) (tasks.Task, error) {
			if taskID != "tsk_1" || !useLatest || key != "retry-1" || revision != 3 {
				t.Fatalf("task=%q useLatest=%t key=%q revision=%d", taskID, useLatest, key, revision)
			}
			return tasks.Task{}, tasks.ErrInvalidState
		}},
		nil,
		&fakeDispatcher{},
		nil,
	)
	request := httptest.NewRequest(http.MethodPost, "/tasks/tsk_1:retry", strings.NewReader(`{"revision_selection":"current_production_revision"}`))
	request.Header.Set("Authorization", "Bearer access-token")
	request.Header.Set("Idempotency-Key", "retry-1")
	request.Header.Set("If-Match", `"3"`)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestAppendMessageUsesHeaderIdempotencyKeyAndDispatchesNewRun(t *testing.T) {
	dispatcher := &fakeDispatcher{}
	var receivedKey, receivedTask, receivedMessage string
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeTaskService{append: func(_ context.Context, _ identity.Principal, taskID, key string, revision int64, request tasks.AppendMessageRequest) (tasks.Task, bool, error) {
			if revision != 5 {
				t.Fatalf("revision=%d", revision)
			}
			receivedTask, receivedKey, receivedMessage = taskID, key, request.Message
			return tasks.Task{ID: taskID, Status: "queued", CurrentRun: tasks.Run{ID: "run_2", Status: "queued"}}, false, nil
		}},
		nil,
		dispatcher,
		nil,
	)
	request := httptest.NewRequest(http.MethodPost, "/tasks/tsk_1/messages", strings.NewReader(`{"message":"Use a different target"}`))
	request.Header.Set("Authorization", "Bearer access-token")
	request.Header.Set("Idempotency-Key", "follow-up-1")
	request.Header.Set("If-Match", `"5"`)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if receivedTask != "tsk_1" || receivedKey != "follow-up-1" || receivedMessage != "Use a different target" {
		t.Fatalf("received task=%q key=%q message=%q", receivedTask, receivedKey, receivedMessage)
	}
	if dispatcher.dispatches != 1 {
		t.Fatalf("dispatches = %d", dispatcher.dispatches)
	}
}

func TestListRunsAndApprovalDetailAreRequesterScoped(t *testing.T) {
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeTaskService{runs: func(_ context.Context, _ identity.Principal, taskID string, _ int) ([]tasks.RunAttempt, error) {
			if taskID != "tsk_1" {
				t.Fatalf("task ID = %q", taskID)
			}
			return []tasks.RunAttempt{{ID: "run_1", Attempt: 1, Status: "failed"}}, nil
		}},
		fakeApprovalService{get: func(_ context.Context, _ identity.Principal, approvalID string) (approvals.Request, error) {
			if approvalID != "apr_1" {
				t.Fatalf("approval ID = %q", approvalID)
			}
			return approvals.Request{ID: approvalID, TaskID: "tsk_1", Status: "rejected", Decision: &approvals.Decision{Decision: "reject"}}, nil
		}},
		&fakeDispatcher{},
		nil,
	)
	for _, path := range []string{"/tasks/tsk_1/runs", "/approvals/apr_1"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer access-token")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body = %s", path, response.Code, response.Body.String())
		}
	}
}

func TestListArtifactsPassesOnlyRequesterFilters(t *testing.T) {
	var taskID, classification string
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeTaskService{artifacts: func(_ context.Context, _ identity.Principal, task, class string, _ *tasks.ArtifactCursor, _ int) (tasks.ArtifactPage, error) {
			taskID, classification = task, class
			return tasks.ArtifactPage{Items: []tasks.Artifact{{ID: "art_1", TaskID: task, Classification: class}}}, nil
		}},
		nil,
		&fakeDispatcher{},
		nil,
	)
	request := httptest.NewRequest(http.MethodGet, "/artifacts?task_id=tsk_1&classification=internal", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if taskID != "tsk_1" || classification != "internal" {
		t.Fatalf("filters task=%q classification=%q", taskID, classification)
	}
}

func TestArtifactDownloadIssuesOnlyACommandScopedGrant(t *testing.T) {
	var requestedID string
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeTaskService{downloadArtifact: func(_ context.Context, actor identity.Principal, artifactID string) (tasks.ArtifactDownloadGrant, error) {
			if actor.ID != "principal-1" {
				t.Fatalf("actor=%q", actor.ID)
			}
			requestedID = artifactID
			return tasks.ArtifactDownloadGrant{ArtifactID: artifactID, DownloadURL: "https://downloads.example.test/art_1", ExpiresAt: time.Date(2026, 8, 17, 1, 0, 0, 0, time.UTC)}, nil
		}},
		nil,
		&fakeDispatcher{},
		nil,
	)
	request := httptest.NewRequest(http.MethodPost, "/artifacts/art_1:download", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if requestedID != "art_1" || !strings.Contains(response.Body.String(), `"artifact_id":"art_1"`) || !strings.Contains(response.Body.String(), `"download_url"`) {
		t.Fatalf("requested=%q body=%s", requestedID, response.Body.String())
	}
}

func TestArtifactDownloadRejectsUnavailableArtifact(t *testing.T) {
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeTaskService{downloadArtifact: func(context.Context, identity.Principal, string) (tasks.ArtifactDownloadGrant, error) {
			return tasks.ArtifactDownloadGrant{}, tasks.ErrInvalidState
		}},
		nil,
		&fakeDispatcher{},
		nil,
	)
	request := httptest.NewRequest(http.MethodPost, "/artifacts/art_1:download", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "artifact_unavailable") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestApprovalReadsResolveExpiredRequestsBeforeListing(t *testing.T) {
	dispatcher := &fakeDispatcher{}
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeTaskService{},
		fakeApprovalService{expire: func(_ context.Context, _ identity.Principal) ([]approvals.Resolution, error) {
			return []approvals.Resolution{{ApprovalID: "apr_1", RunID: "run_1", Decision: "reject", Reason: "approval expired"}}, nil
		}},
		dispatcher,
		nil,
	)
	request := httptest.NewRequest(http.MethodGet, "/approvals", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if dispatcher.resolvedRun != "run_1" || dispatcher.resolvedDecision != "reject" {
		t.Fatalf("resolution = %#v", dispatcher)
	}
}

func TestDecideApprovalRequiresExactActionDigest(t *testing.T) {
	dispatcher := &fakeDispatcher{}
	var received approvals.DecisionInput
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeTaskService{},
		fakeApprovalService{
			decide: func(_ context.Context, _ identity.Principal, input approvals.DecisionInput) (approvals.Resolution, error) {
				received = input
				return approvals.Resolution{ApprovalID: input.ID, RunID: "run-1", Decision: "approve"}, nil
			},
			get: func(_ context.Context, _ identity.Principal, approvalID string) (approvals.Request, error) {
				return approvals.Request{ID: approvalID, Status: "satisfied", ActionDigest: "sha256:one"}, nil
			},
		},
		dispatcher,
		nil,
	)
	request := httptest.NewRequest(http.MethodPost, "/approvals/apr_1:decide", strings.NewReader(`{"decision":"approve","action_digest":"sha256:one","approval_revision":1}`))
	request.Header.Set("Authorization", "Bearer access-token")
	request.Header.Set("Idempotency-Key", "decision-1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if received.ID != "apr_1" || received.ActionDigest != "sha256:one" || received.Idempotency != "decision-1" {
		t.Fatalf("input = %#v", received)
	}
	if !strings.Contains(response.Body.String(), `"status":"satisfied"`) {
		t.Fatalf("response = %s", response.Body.String())
	}
}

func TestDecideApprovalReturnsServerWinningApprovalOnConflict(t *testing.T) {
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeTaskService{},
		fakeApprovalService{
			decide: func(context.Context, identity.Principal, approvals.DecisionInput) (approvals.Resolution, error) {
				return approvals.Resolution{}, approvals.ErrChanged
			},
			get: func(_ context.Context, _ identity.Principal, approvalID string) (approvals.Request, error) {
				return approvals.Request{ID: approvalID, Status: "rejected", ActionDigest: "sha256:one"}, nil
			},
		},
		&fakeDispatcher{},
		nil,
	)
	request := httptest.NewRequest(http.MethodPost, "/approvals/apr_1:decide", strings.NewReader(`{"decision":"approve","action_digest":"sha256:one","approval_revision":1}`))
	request.Header.Set("Authorization", "Bearer access-token")
	request.Header.Set("Idempotency-Key", "decision-1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"approval_changed"`) || !strings.Contains(response.Body.String(), `"current_resource"`) || !strings.Contains(response.Body.String(), `"status":"rejected"`) {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestCopilotRoutesRequireBearerAuthentication(t *testing.T) {
	handler := New(fakeAuthenticator{err: identity.ErrUnauthorized}, fakeTaskService{}, nil, &fakeDispatcher{}, nil)
	request := httptest.NewRequest(http.MethodGet, "/agents", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", response.Code)
	}
}
