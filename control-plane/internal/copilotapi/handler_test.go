package copilotapi

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/AirSodaz/gantry/internal/approvals"
	"github.com/AirSodaz/gantry/internal/identity"
	"github.com/AirSodaz/gantry/internal/runs"
	"github.com/AirSodaz/gantry/internal/sessions"
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

type fakeSessionService struct {
	setAgentFavorite   func(context.Context, identity.Principal, string, string, sessions.SetAgentFavoriteRequest) (sessions.Agent, error)
	submit             func(context.Context, identity.Principal, string, sessions.SubmitRequest) (sessions.Session, bool, error)
	list               func(context.Context, identity.Principal, sessions.ListFilter, *sessions.SessionCursor, int) (sessions.SessionPage, error)
	get                func(context.Context, identity.Principal, string) (sessions.Session, error)
	append             func(context.Context, identity.Principal, string, string, int64, sessions.AppendMessageRequest) (sessions.Session, bool, error)
	runs               func(context.Context, identity.Principal, string, *sessions.RunCursor, int) (sessions.RunPage, error)
	createAttachment   func(context.Context, identity.Principal, string, sessions.CreateAttachmentRequest) (sessions.Attachment, bool, error)
	getAttachment      func(context.Context, identity.Principal, string) (sessions.Attachment, error)
	uploadAttachment   func(context.Context, identity.Principal, string, string, io.Reader) error
	completeAttachment func(context.Context, identity.Principal, string, string) (sessions.Attachment, bool, error)
	cancel             func(context.Context, identity.Principal, string, string, string) (sessions.CancelResult, error)
	retry              func(context.Context, identity.Principal, string, bool, string, int64) (sessions.RetryResult, error)
}

type fakeApprovalService struct {
	list   func(context.Context, identity.Principal, string, *approvals.Cursor, int) (approvals.Page, error)
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

func (s fakeApprovalService) List(ctx context.Context, actor identity.Principal, state string, after *approvals.Cursor, limit int) (approvals.Page, error) {
	if s.list != nil {
		return s.list(ctx, actor, state, after, limit)
	}
	return approvals.Page{}, nil
}
func (s fakeApprovalService) Decide(ctx context.Context, actor identity.Principal, input approvals.DecisionInput) (approvals.Resolution, error) {
	if s.decide != nil {
		return s.decide(ctx, actor, input)
	}
	return approvals.Resolution{}, approvals.ErrNotFound
}

func (s fakeSessionService) ListAgents(context.Context, identity.Principal, string, string, string, *sessions.AgentCursor, int) (sessions.AgentPage, error) {
	return sessions.AgentPage{}, nil
}

func (s fakeSessionService) SetAgentFavorite(ctx context.Context, actor identity.Principal, agentID, key string, request sessions.SetAgentFavoriteRequest) (sessions.Agent, error) {
	if s.setAgentFavorite != nil {
		return s.setAgentFavorite(ctx, actor, agentID, key, request)
	}
	return sessions.Agent{}, sessions.ErrNotFound
}

func (s fakeSessionService) Submit(ctx context.Context, actor identity.Principal, key string, request sessions.SubmitRequest) (sessions.Session, bool, error) {
	if s.submit == nil {
		return sessions.Session{}, false, errors.New("unexpected submit")
	}
	return s.submit(ctx, actor, key, request)
}

func (s fakeSessionService) List(ctx context.Context, actor identity.Principal, filter sessions.ListFilter, after *sessions.SessionCursor, limit int) (sessions.SessionPage, error) {
	if s.list != nil {
		return s.list(ctx, actor, filter, after, limit)
	}
	return sessions.SessionPage{}, nil
}

func (s fakeSessionService) Get(ctx context.Context, actor identity.Principal, sessionID string) (sessions.Session, error) {
	if s.get == nil {
		return sessions.Session{}, sessions.ErrNotFound
	}
	return s.get(ctx, actor, sessionID)
}

func (s fakeSessionService) AppendMessage(ctx context.Context, actor identity.Principal, sessionID, key string, revision int64, request sessions.AppendMessageRequest) (sessions.Session, bool, error) {
	if s.append != nil {
		return s.append(ctx, actor, sessionID, key, revision, request)
	}
	return sessions.Session{}, false, sessions.ErrNotFound
}

func (s fakeSessionService) ListRuns(ctx context.Context, actor identity.Principal, sessionID string, after *sessions.RunCursor, limit int) (sessions.RunPage, error) {
	if s.runs != nil {
		return s.runs(ctx, actor, sessionID, after, limit)
	}
	return sessions.RunPage{}, sessions.ErrNotFound
}

type fakeArtifactService struct {
	get      func(context.Context, identity.Principal, string) (runs.Artifact, error)
	list     func(context.Context, identity.Principal, string, string, string, *runs.ArtifactCursor, int) (runs.ArtifactPage, error)
	download func(context.Context, identity.Principal, string) (runs.ArtifactDownloadGrant, error)
}

func (s fakeArtifactService) GetArtifact(ctx context.Context, actor identity.Principal, artifactID string) (runs.Artifact, error) {
	if s.get != nil {
		return s.get(ctx, actor, artifactID)
	}
	return runs.Artifact{}, runs.ErrNotFound
}

func (s fakeArtifactService) ListMyArtifacts(ctx context.Context, actor identity.Principal, sessionID, classification, state string, after *runs.ArtifactCursor, limit int) (runs.ArtifactPage, error) {
	if s.list != nil {
		return s.list(ctx, actor, sessionID, classification, state, after, limit)
	}
	return runs.ArtifactPage{}, nil
}

func (s fakeArtifactService) DownloadArtifact(ctx context.Context, actor identity.Principal, artifactID string) (runs.ArtifactDownloadGrant, error) {
	if s.download != nil {
		return s.download(ctx, actor, artifactID)
	}
	return runs.ArtifactDownloadGrant{}, runs.ErrNotFound
}

func (s fakeSessionService) CreateAttachment(ctx context.Context, actor identity.Principal, key string, request sessions.CreateAttachmentRequest) (sessions.Attachment, bool, error) {
	if s.createAttachment != nil {
		return s.createAttachment(ctx, actor, key, request)
	}
	return sessions.Attachment{}, false, sessions.ErrNotFound
}

func (s fakeSessionService) GetAttachment(ctx context.Context, actor identity.Principal, attachmentID string) (sessions.Attachment, error) {
	if s.getAttachment != nil {
		return s.getAttachment(ctx, actor, attachmentID)
	}
	return sessions.Attachment{}, sessions.ErrNotFound
}

func (s fakeSessionService) UploadAttachment(ctx context.Context, actor identity.Principal, attachmentID, token string, body io.Reader) error {
	if s.uploadAttachment != nil {
		return s.uploadAttachment(ctx, actor, attachmentID, token, body)
	}
	return sessions.ErrNotFound
}

func (s fakeSessionService) CompleteAttachment(ctx context.Context, actor identity.Principal, attachmentID, key string) (sessions.Attachment, bool, error) {
	if s.completeAttachment != nil {
		return s.completeAttachment(ctx, actor, attachmentID, key)
	}
	return sessions.Attachment{}, false, sessions.ErrNotFound
}

func (s fakeSessionService) Cancel(ctx context.Context, actor identity.Principal, sessionID, runID, key string) (sessions.CancelResult, error) {
	if s.cancel != nil {
		return s.cancel(ctx, actor, sessionID, runID, key)
	}
	return sessions.CancelResult{}, sessions.ErrNotFound
}

func (s fakeSessionService) Retry(ctx context.Context, actor identity.Principal, sessionID string, useLatest bool, key string, revision int64) (sessions.RetryResult, error) {
	if s.retry != nil {
		return s.retry(ctx, actor, sessionID, useLatest, key, revision)
	}
	return sessions.RetryResult{}, sessions.ErrNotFound
}

func (s fakeSessionService) RetryRun(ctx context.Context, actor identity.Principal, sessionID, _ string, useLatest bool, key string, revision int64) (sessions.RetryResult, error) {
	return s.Retry(ctx, actor, sessionID, useLatest, key, revision)
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

func TestSubmitSessionUsesHeaderIdempotencyKey(t *testing.T) {
	dispatcher := &fakeDispatcher{}
	var receivedKey string
	var receivedRequest sessions.SubmitRequest
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeSessionService{submit: func(_ context.Context, actor identity.Principal, key string, request sessions.SubmitRequest) (sessions.Session, bool, error) {
			if actor.ID != "principal-1" {
				t.Fatalf("actor ID = %q", actor.ID)
			}
			receivedKey = key
			receivedRequest = request
			return sessions.Session{ID: "ses_1", State: "active", ExecutingRun: &sessions.Run{ID: "run_1", State: "queued"}}, false, nil
		}},
		nil,
		dispatcher,
		nil,
	)

	request := httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader(`{"agent_id":"agt_lifecycle_demo","message":"hello"}`))
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

func TestSubmitSessionRejectsBodyIdempotencyKey(t *testing.T) {
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeSessionService{},
		nil,
		&fakeDispatcher{},
		nil,
	)
	request := httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader(`{"agent_id":"agt_lifecycle_demo","message":"hello","idempotency_key":"not-supported"}`))
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestSubmitSessionReturnsOKForIdempotentRetry(t *testing.T) {
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeSessionService{submit: func(context.Context, identity.Principal, string, sessions.SubmitRequest) (sessions.Session, bool, error) {
			return sessions.Session{ID: "ses_1"}, true, nil
		}},
		nil,
		&fakeDispatcher{},
		nil,
	)
	request := httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader(`{"agent_id":"agt_lifecycle_demo","message":"hello"}`))
	request.Header.Set("Authorization", "Bearer access-token")
	request.Header.Set("Idempotency-Key", "retry-safe-key")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestSetAgentFavoriteUsesRequesterScopedCommand(t *testing.T) {
	var received sessions.SetAgentFavoriteRequest
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1", OrganizationID: "org-1"}},
		fakeSessionService{setAgentFavorite: func(_ context.Context, actor identity.Principal, agentID, key string, request sessions.SetAgentFavoriteRequest) (sessions.Agent, error) {
			if actor.ID != "principal-1" || agentID != "agt_1" || key != "favorite-1" {
				t.Fatalf("actor=%q agent=%q key=%q", actor.ID, agentID, key)
			}
			received = request
			return sessions.Agent{ID: agentID, DisplayName: "Finance", IsFavorite: request.IsFavorite}, nil
		}},
		nil,
		&fakeDispatcher{},
		nil,
	)
	request := httptest.NewRequest(http.MethodPut, "/agents/agt_1/favorite", strings.NewReader(`{"is_favorite":true}`))
	request.Header.Set("Authorization", "Bearer access-token")
	request.Header.Set("Idempotency-Key", "favorite-1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !received.IsFavorite || !strings.Contains(response.Body.String(), `"is_favorite":true`) {
		t.Fatalf("status=%d request=%#v body=%s", response.Code, received, response.Body.String())
	}
}

func TestGetSessionReturnsConversationETag(t *testing.T) {
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeSessionService{get: func(context.Context, identity.Principal, string) (sessions.Session, error) {
			return sessions.Session{ID: "ses_1", ConversationRevision: 7}, nil
		}},
		nil,
		&fakeDispatcher{},
		nil,
	)
	request := httptest.NewRequest(http.MethodGet, "/sessions/ses_1", nil)
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
		fakeSessionService{},
		nil,
		&fakeDispatcher{},
		nil,
	)
	request := httptest.NewRequest(http.MethodPost, "/sessions/ses_1/messages", strings.NewReader(`{"message":"Use a different target"}`))
	request.Header.Set("Authorization", "Bearer access-token")
	request.Header.Set("Idempotency-Key", "follow-up-1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusPreconditionRequired || !strings.Contains(response.Body.String(), "conversation_etag_required") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestAppendMessageReturnsCurrentSessionOnConversationConflict(t *testing.T) {
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeSessionService{
			append: func(context.Context, identity.Principal, string, string, int64, sessions.AppendMessageRequest) (sessions.Session, bool, error) {
				return sessions.Session{}, false, sessions.ErrConversationChanged
			},
			get: func(context.Context, identity.Principal, string) (sessions.Session, error) {
				return sessions.Session{ID: "ses_1", State: "active", ConversationRevision: 8}, nil
			},
		},
		nil,
		&fakeDispatcher{},
		nil,
	)
	request := httptest.NewRequest(http.MethodPost, "/sessions/ses_1/messages", strings.NewReader(`{"message":"Use a different target"}`))
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
	var received sessions.CreateAttachmentRequest
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1", OrganizationID: "org-1"}},
		fakeSessionService{createAttachment: func(_ context.Context, actor identity.Principal, _ string, request sessions.CreateAttachmentRequest) (sessions.Attachment, bool, error) {
			if actor.ID != "principal-1" {
				t.Fatalf("actor=%q", actor.ID)
			}
			received = request
			return sessions.Attachment{ID: "att_1", Filename: request.Filename, State: "declared", ScanStatus: "pending", UploadURL: "/api/copilot/v1/attachments/att_1/content", UploadToken: "short-lived"}, false, nil
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
	if !strings.Contains(response.Body.String(), `"upload_path":"/attachments/att_1/content"`) {
		t.Fatalf("upload path=%s", response.Body.String())
	}
}

func TestCompleteAttachmentParsesColonSuffix(t *testing.T) {
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeSessionService{completeAttachment: func(_ context.Context, _ identity.Principal, attachmentID, _ string) (sessions.Attachment, bool, error) {
			if attachmentID != "att_1" {
				t.Fatalf("attachment=%q", attachmentID)
			}
			return sessions.Attachment{ID: attachmentID, State: "available", ScanStatus: "passed"}, false, nil
		}},
		nil,
		&fakeDispatcher{},
		nil,
	)
	request := httptest.NewRequest(http.MethodPost, "/attachments/att_1:complete", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	request.Header.Set("Idempotency-Key", "complete-1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestCompleteAttachmentReturnsIdempotencyConflict(t *testing.T) {
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeSessionService{completeAttachment: func(context.Context, identity.Principal, string, string) (sessions.Attachment, bool, error) {
			return sessions.Attachment{}, false, sessions.ErrIdempotencyConflict
		}},
		nil,
		&fakeDispatcher{},
		nil,
	)
	request := httptest.NewRequest(http.MethodPost, "/attachments/att_2:complete", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	request.Header.Set("Idempotency-Key", "complete-1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"code":"idempotency_conflict"`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestSessionLookupDoesNotExposeOtherUsersResources(t *testing.T) {
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-2"}},
		fakeSessionService{get: func(context.Context, identity.Principal, string) (sessions.Session, error) {
			return sessions.Session{}, sessions.ErrNotFound
		}},
		nil,
		&fakeDispatcher{},
		nil,
	)
	request := httptest.NewRequest(http.MethodGet, "/sessions/ses_other_user", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestListSessionsParsesMemberFilters(t *testing.T) {
	var received sessions.ListFilter
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeSessionService{list: func(_ context.Context, _ identity.Principal, filter sessions.ListFilter, _ *sessions.SessionCursor, _ int) (sessions.SessionPage, error) {
			received = filter
			return sessions.SessionPage{}, nil
		}},
		nil,
		&fakeDispatcher{},
		nil,
	)
	request := httptest.NewRequest(http.MethodGet, "/sessions?state=active&mode=shared&agent_id=agt_1&my_action=approval&updated_after=2026-08-16T00:00:00Z", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", response.Code, response.Body.String())
	}
	if received.State != "active" || received.Mode != "shared" || received.AgentID != "agt_1" || received.MyAction != "approval" || received.UpdatedAfter == nil {
		t.Fatalf("filter = %#v", received)
	}
}

func TestCancelOperationParsesColonSuffix(t *testing.T) {
	dispatcher := &fakeDispatcher{}
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeSessionService{cancel: func(_ context.Context, _ identity.Principal, sessionID, runID, key string) (sessions.CancelResult, error) {
			if sessionID != "ses_1" || runID != "run_1" || key != "cancel-1" {
				t.Fatalf("session=%q run=%q key=%q", sessionID, runID, key)
			}
			return sessions.CancelResult{Run: sessions.Run{ID: runID, Status: "canceling", LeaseEpoch: 7}, Deliver: true}, nil
		}},
		nil,
		dispatcher,
		nil,
	)
	request := httptest.NewRequest(http.MethodPost, "/sessions/ses_1/runs/run_1:cancel", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	request.Header.Set("Idempotency-Key", "cancel-1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if dispatcher.canceledRun != "run_1" || dispatcher.canceledEpoch != 7 {
		t.Fatalf("cancel delivery = %#v", dispatcher)
	}
}

func TestRetryOperationMapsInvalidStateToConflict(t *testing.T) {
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeSessionService{retry: func(_ context.Context, _ identity.Principal, sessionID string, useLatest bool, key string, revision int64) (sessions.RetryResult, error) {
			if sessionID != "ses_1" || !useLatest || key != "retry-1" || revision != 3 {
				t.Fatalf("session=%q useLatest=%t key=%q revision=%d", sessionID, useLatest, key, revision)
			}
			return sessions.RetryResult{}, sessions.ErrInvalidState
		}},
		nil,
		&fakeDispatcher{},
		nil,
	)
	request := httptest.NewRequest(http.MethodPost, "/sessions/ses_1/runs/run_1:retry", strings.NewReader(`{"revision_selection":"current_production_revision"}`))
	request.Header.Set("Authorization", "Bearer access-token")
	request.Header.Set("Idempotency-Key", "retry-1")
	request.Header.Set("If-Match", `"3"`)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestRetryOperationReturnsRecordedRunAndDoesNotRedispatchReplay(t *testing.T) {
	dispatcher := &fakeDispatcher{}
	duplicate := false
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeSessionService{retry: func(_ context.Context, _ identity.Principal, sessionID string, _ bool, _ string, _ int64) (sessions.RetryResult, error) {
			return sessions.RetryResult{Run: sessions.Run{ID: "run_retried", SessionSequence: 9, State: "queued"}, Duplicate: duplicate}, nil
		}},
		nil,
		dispatcher,
		nil,
	)
	call := func() *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/sessions/ses_1/runs/run_failed:retry", strings.NewReader(`{"revision_selection":"original_revision"}`))
		request.Header.Set("Authorization", "Bearer access-token")
		request.Header.Set("Idempotency-Key", "retry-1")
		request.Header.Set("If-Match", `"3"`)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	response := call()
	if response.Code != http.StatusCreated || !strings.Contains(response.Body.String(), `"id":"run_retried"`) || dispatcher.dispatches != 1 {
		t.Fatalf("new retry status=%d dispatches=%d body=%s", response.Code, dispatcher.dispatches, response.Body.String())
	}
	duplicate = true
	response = call()
	if response.Code != http.StatusOK || dispatcher.dispatches != 1 || !strings.Contains(response.Body.String(), `"session_sequence":9`) {
		t.Fatalf("replay status=%d dispatches=%d body=%s", response.Code, dispatcher.dispatches, response.Body.String())
	}
}

func TestAppendMessageUsesHeaderIdempotencyKeyAndDispatchesNewRun(t *testing.T) {
	dispatcher := &fakeDispatcher{}
	var receivedKey, receivedSession, receivedMessage string
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeSessionService{append: func(_ context.Context, _ identity.Principal, sessionID, key string, revision int64, request sessions.AppendMessageRequest) (sessions.Session, bool, error) {
			if revision != 5 {
				t.Fatalf("revision=%d", revision)
			}
			receivedSession, receivedKey, receivedMessage = sessionID, key, request.Message
			return sessions.Session{ID: sessionID, State: "active", ExecutingRun: &sessions.Run{ID: "run_2", State: "queued"}}, false, nil
		}},
		nil,
		dispatcher,
		nil,
	)
	request := httptest.NewRequest(http.MethodPost, "/sessions/ses_1/messages", strings.NewReader(`{"message":"Use a different target"}`))
	request.Header.Set("Authorization", "Bearer access-token")
	request.Header.Set("Idempotency-Key", "follow-up-1")
	request.Header.Set("If-Match", `"5"`)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if receivedSession != "ses_1" || receivedKey != "follow-up-1" || receivedMessage != "Use a different target" {
		t.Fatalf("received session=%q key=%q message=%q", receivedSession, receivedKey, receivedMessage)
	}
	if dispatcher.dispatches != 1 {
		t.Fatalf("dispatches = %d", dispatcher.dispatches)
	}
}

func TestListRunsAndApprovalDetailAreRequesterScoped(t *testing.T) {
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeSessionService{runs: func(_ context.Context, _ identity.Principal, sessionID string, _ *sessions.RunCursor, _ int) (sessions.RunPage, error) {
			if sessionID != "ses_1" {
				t.Fatalf("session ID = %q", sessionID)
			}
			return sessions.RunPage{Items: []sessions.Run{{ID: "run_1", SessionSequence: 1, State: "failed"}}}, nil
		}},
		fakeApprovalService{get: func(_ context.Context, _ identity.Principal, approvalID string) (approvals.Request, error) {
			if approvalID != "apr_1" {
				t.Fatalf("approval ID = %q", approvalID)
			}
			return approvals.Request{ID: approvalID, SessionID: "ses_1", Status: "rejected", Decision: &approvals.Decision{Decision: "reject"}}, nil
		}},
		&fakeDispatcher{},
		nil,
	)
	for _, path := range []string{"/sessions/ses_1/runs", "/approvals/apr_1"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer access-token")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d, body = %s", path, response.Code, response.Body.String())
		}
	}
}

func TestListRunsReturnsSessionBoundNextCursor(t *testing.T) {
	var after *sessions.RunCursor
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeSessionService{runs: func(_ context.Context, _ identity.Principal, sessionID string, cursor *sessions.RunCursor, _ int) (sessions.RunPage, error) {
			if sessionID != "ses_1" {
				t.Fatalf("session ID = %q", sessionID)
			}
			after = cursor
			if cursor != nil {
				return sessions.RunPage{Items: []sessions.Run{{ID: "run_1", SessionSequence: 1, State: "failed"}}}, nil
			}
			return sessions.RunPage{Items: []sessions.Run{{ID: "run_2", SessionSequence: 2, State: "completed"}}, HasMore: true}, nil
		}},
		nil,
		&fakeDispatcher{},
		nil,
	)
	request := httptest.NewRequest(http.MethodGet, "/sessions/ses_1/runs?limit=1", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	var first struct {
		PageInfo struct {
			HasMore    bool   `json:"has_more"`
			NextCursor string `json:"next_cursor"`
		} `json:"page_info"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	if !first.PageInfo.HasMore || first.PageInfo.NextCursor == "" {
		t.Fatalf("page info = %#v", first.PageInfo)
	}

	request = httptest.NewRequest(http.MethodGet, "/sessions/ses_1/runs?cursor="+first.PageInfo.NextCursor, nil)
	request.Header.Set("Authorization", "Bearer access-token")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("next page status = %d, body = %s", response.Code, response.Body.String())
	}
	if after == nil || after.SessionSequence != 2 || after.ID != "run_2" {
		t.Fatalf("after = %#v", after)
	}
}

func TestListApprovalsPassesStateFilter(t *testing.T) {
	var receivedState string
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeSessionService{},
		fakeApprovalService{list: func(_ context.Context, _ identity.Principal, state string, _ *approvals.Cursor, _ int) (approvals.Page, error) {
			receivedState = state
			return approvals.Page{}, nil
		}},
		&fakeDispatcher{},
		nil,
	)
	request := httptest.NewRequest(http.MethodGet, "/approvals?state=rejected", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if receivedState != "rejected" {
		t.Fatalf("state = %q", receivedState)
	}
}

func TestListArtifactsPassesOnlyRequesterFilters(t *testing.T) {
	var sessionID, classification, state string
	artifacts := fakeArtifactService{list: func(_ context.Context, _ identity.Principal, session, class, artifactState string, _ *runs.ArtifactCursor, _ int) (runs.ArtifactPage, error) {
		sessionID, classification, state = session, class, artifactState
		return runs.ArtifactPage{Items: []runs.Artifact{{ID: "art_1", SessionID: session, Classification: class}}}, nil
	}}
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeSessionService{},
		nil,
		&fakeDispatcher{},
		nil,
		artifacts,
	)
	request := httptest.NewRequest(http.MethodGet, "/artifacts?session_id=ses_1&classification=internal&state=available", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if sessionID != "ses_1" || classification != "internal" || state != "available" {
		t.Fatalf("filters session=%q classification=%q state=%q", sessionID, classification, state)
	}
}

func TestArtifactDownloadIssuesOnlyACommandScopedGrant(t *testing.T) {
	var requestedID string
	artifacts := fakeArtifactService{download: func(_ context.Context, actor identity.Principal, artifactID string) (runs.ArtifactDownloadGrant, error) {
		if actor.ID != "principal-1" {
			t.Fatalf("actor=%q", actor.ID)
		}
		requestedID = artifactID
		return runs.ArtifactDownloadGrant{ArtifactID: artifactID, DownloadURL: "https://downloads.example.test/art_1", ExpiresAt: time.Date(2026, 8, 17, 1, 0, 0, 0, time.UTC)}, nil
	}}
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeSessionService{},
		nil,
		&fakeDispatcher{},
		nil,
		artifacts,
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
	artifacts := fakeArtifactService{download: func(context.Context, identity.Principal, string) (runs.ArtifactDownloadGrant, error) {
		return runs.ArtifactDownloadGrant{}, runs.ErrInvalidState
	}}
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeSessionService{},
		nil,
		&fakeDispatcher{},
		nil,
		artifacts,
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
		fakeSessionService{},
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
		fakeSessionService{},
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
	if !strings.Contains(response.Body.String(), `"state":"approved"`) {
		t.Fatalf("response = %s", response.Body.String())
	}
}

func TestDecideApprovalReturnsServerWinningApprovalOnConflict(t *testing.T) {
	handler := New(
		fakeAuthenticator{principal: identity.Principal{ID: "principal-1"}},
		fakeSessionService{},
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
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"approval_changed"`) || !strings.Contains(response.Body.String(), `"current_resource"`) || !strings.Contains(response.Body.String(), `"state":"rejected"`) {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestCopilotRoutesRequireBearerAuthentication(t *testing.T) {
	handler := New(fakeAuthenticator{err: identity.ErrUnauthorized}, fakeSessionService{}, nil, &fakeDispatcher{}, nil)
	request := httptest.NewRequest(http.MethodGet, "/agents", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", response.Code)
	}
}
