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
	"github.com/AirSodaz/gantry/internal/configassets"
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

type fakeAssetService struct {
	createSkill   func(context.Context, identity.Principal, configassets.CreateSkillRequest) (configassets.Skill, error)
	enablePlugin  func(context.Context, identity.Principal, string, string) error
	getSkill      func(string) (configassets.Skill, error)
	skillUsage    func(string) ([]configassets.AssetUsage, error)
	listSkillOpts func(configassets.ListOptions)
	status        func(string, string, string) error
}

func (s fakeAssetService) ListSkills(_ context.Context, _ identity.Principal, options configassets.ListOptions) ([]configassets.Skill, error) {
	if s.listSkillOpts != nil {
		s.listSkillOpts(options)
	}
	return nil, nil
}
func (s fakeAssetService) GetSkill(_ context.Context, _ identity.Principal, id string) (configassets.Skill, error) {
	if s.getSkill == nil {
		return configassets.Skill{}, configassets.ErrNotFound
	}
	return s.getSkill(id)
}
func (s fakeAssetService) ListSkillUsage(_ context.Context, _ identity.Principal, id string) ([]configassets.AssetUsage, error) {
	if s.skillUsage == nil {
		return nil, configassets.ErrNotFound
	}
	return s.skillUsage(id)
}
func (s fakeAssetService) CreateSkill(ctx context.Context, actor identity.Principal, request configassets.CreateSkillRequest) (configassets.Skill, error) {
	if s.createSkill == nil {
		return configassets.Skill{}, configassets.ErrNotFound
	}
	return s.createSkill(ctx, actor, request)
}
func (fakeAssetService) ListPlugins(context.Context, identity.Principal, configassets.ListOptions) ([]configassets.Plugin, error) {
	return nil, nil
}
func (fakeAssetService) GetPlugin(context.Context, identity.Principal, string) (configassets.PluginDetail, error) {
	return configassets.PluginDetail{}, configassets.ErrNotFound
}
func (fakeAssetService) ListPluginUsage(context.Context, identity.Principal, string) ([]configassets.AssetUsage, error) {
	return nil, configassets.ErrNotFound
}
func (fakeAssetService) CreatePlugin(context.Context, identity.Principal, configassets.CreatePluginRequest) (configassets.Plugin, error) {
	return configassets.Plugin{}, configassets.ErrNotFound
}
func (s fakeAssetService) EnablePlugin(ctx context.Context, actor identity.Principal, pluginID, workspaceID string) error {
	if s.enablePlugin == nil {
		return configassets.ErrNotFound
	}
	return s.enablePlugin(ctx, actor, pluginID, workspaceID)
}
func (fakeAssetService) ListTools(context.Context, identity.Principal, configassets.ListOptions) ([]configassets.Tool, error) {
	return nil, nil
}
func (fakeAssetService) GetTool(context.Context, identity.Principal, string) (configassets.Tool, error) {
	return configassets.Tool{}, configassets.ErrNotFound
}
func (fakeAssetService) ListToolUsage(context.Context, identity.Principal, string) ([]configassets.AssetUsage, error) {
	return nil, configassets.ErrNotFound
}
func (fakeAssetService) CreateTool(context.Context, identity.Principal, configassets.CreateToolRequest) (configassets.Tool, error) {
	return configassets.Tool{}, configassets.ErrNotFound
}
func (s fakeAssetService) ActivateSkill(_ context.Context, _ identity.Principal, id, reason string) error {
	return s.callStatus("skill", id, "activate", reason)
}
func (s fakeAssetService) DeprecateSkill(_ context.Context, _ identity.Principal, id, reason string) error {
	return s.callStatus("skill", id, "deprecate", reason)
}
func (s fakeAssetService) RetireSkill(_ context.Context, _ identity.Principal, id, reason string) error {
	return s.callStatus("skill", id, "retire", reason)
}
func (s fakeAssetService) ActivatePlugin(_ context.Context, _ identity.Principal, id, reason string) error {
	return s.callStatus("plugin", id, "activate", reason)
}
func (s fakeAssetService) DeprecatePlugin(_ context.Context, _ identity.Principal, id, reason string) error {
	return s.callStatus("plugin", id, "deprecate", reason)
}
func (s fakeAssetService) RetirePlugin(_ context.Context, _ identity.Principal, id, reason string) error {
	return s.callStatus("plugin", id, "retire", reason)
}
func (s fakeAssetService) ActivateTool(_ context.Context, _ identity.Principal, id, reason string) error {
	return s.callStatus("tool", id, "activate", reason)
}
func (s fakeAssetService) DeprecateTool(_ context.Context, _ identity.Principal, id, reason string) error {
	return s.callStatus("tool", id, "deprecate", reason)
}
func (s fakeAssetService) RetireTool(_ context.Context, _ identity.Principal, id, reason string) error {
	return s.callStatus("tool", id, "retire", reason)
}
func (s fakeAssetService) callStatus(kind, id, operation, reason string) error {
	if s.status == nil {
		return configassets.ErrNotFound
	}
	return s.status(kind, id, operation+":"+reason)
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

func TestSkillRegistrationAndPluginEnablementRoutesForwardScopedRequests(t *testing.T) {
	created, enabled := false, false
	assets := fakeAssetService{
		createSkill: func(_ context.Context, _ identity.Principal, request configassets.CreateSkillRequest) (configassets.Skill, error) {
			created = request.WorkspaceID == "ws_1" && request.SourceType == "locator" && request.ContentDigest == "sha256:1"
			return configassets.Skill{ID: "skill_1"}, nil
		},
		enablePlugin: func(_ context.Context, _ identity.Principal, pluginID, workspaceID string) error {
			enabled = pluginID == "plugin_1" && workspaceID == "ws_1"
			return nil
		},
	}
	handler := NewWithAssets(fakeAuthenticator{actor: identity.Principal{ID: "prn_1"}}, fakeAuthorizer{}, fakeLifecycleService{}, assets, nil)
	create := httptest.NewRequest(http.MethodPost, "/skills", strings.NewReader(`{"workspace_id":"ws_1","slug":"search","display_name":"Search","source_type":"locator","source_ref":"registry://search","content_digest":"sha256:1"}`))
	createResponse := httptest.NewRecorder()
	handler.ServeHTTP(createResponse, create)
	enable := httptest.NewRequest(http.MethodPost, "/plugins/plugin_1/enable", strings.NewReader(`{"workspace_id":"ws_1"}`))
	enableResponse := httptest.NewRecorder()
	handler.ServeHTTP(enableResponse, enable)
	if createResponse.Code != http.StatusCreated || enableResponse.Code != http.StatusNoContent || !created || !enabled {
		t.Fatalf("create=%d enable=%d created=%t enabled=%t", createResponse.Code, enableResponse.Code, created, enabled)
	}
}

func TestAssetLifecycleRoutesForwardOperationAndReason(t *testing.T) {
	called := ""
	assets := fakeAssetService{status: func(kind, id, operationAndReason string) error {
		called = kind + ":" + id + ":" + operationAndReason
		return nil
	}}
	handler := NewWithAssets(fakeAuthenticator{actor: identity.Principal{ID: "prn_1"}}, fakeAuthorizer{}, fakeLifecycleService{}, assets, nil)
	request := httptest.NewRequest(http.MethodPost, "/skills/skill_1:deprecate", strings.NewReader(`{"reason":"upstream replacement"}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || called != "skill:skill_1:deprecate:upstream replacement" {
		t.Fatalf("status=%d called=%q", response.Code, called)
	}
}

func TestSkillUsageRouteReturnsScopedReferences(t *testing.T) {
	assets := fakeAssetService{skillUsage: func(id string) ([]configassets.AssetUsage, error) {
		if id != "skill_1" {
			t.Fatalf("asset id=%q", id)
		}
		return []configassets.AssetUsage{{AgentID: "agt_1", AgentName: "Search", ReferenceKind: "draft", ReferenceIndex: 2}}, nil
	}}
	handler := NewWithAssets(fakeAuthenticator{actor: identity.Principal{ID: "prn_1"}}, fakeAuthorizer{}, fakeLifecycleService{}, assets, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/skills/skill_1/usage", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"agent_id":"agt_1"`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestSkillListForwardsCatalogFilters(t *testing.T) {
	var options configassets.ListOptions
	assets := fakeAssetService{listSkillOpts: func(value configassets.ListOptions) { options = value }}
	handler := NewWithAssets(fakeAuthenticator{actor: identity.Principal{ID: "prn_1"}}, fakeAuthorizer{}, fakeLifecycleService{}, assets, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/skills?workspace_id=ws_1&search=search&status=deprecated", nil))
	if response.Code != http.StatusOK || options != (configassets.ListOptions{WorkspaceID: "ws_1", Search: "search", Status: "deprecated"}) {
		t.Fatalf("status=%d options=%+v", response.Code, options)
	}
}
