// Package adminapi owns the HTTP transport for the administrative agent lifecycle.
package adminapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/AirSodaz/gantry/internal/adminoverview"
	"github.com/AirSodaz/gantry/internal/agentlifecycle"
	"github.com/AirSodaz/gantry/internal/authorization"
	"github.com/AirSodaz/gantry/internal/configassets"
	"github.com/AirSodaz/gantry/internal/identity"
)

type authenticator interface {
	Authenticate(context.Context, string) (identity.Principal, error)
}

type authorizer interface {
	RequireAdmin(context.Context, identity.Principal) error
}

type lifecycleService interface {
	ListWorkspaces(context.Context, identity.Principal) ([]authorization.Workspace, error)
	ListAgents(context.Context, identity.Principal, string) ([]agentlifecycle.Agent, error)
	Create(context.Context, identity.Principal, agentlifecycle.CreateRequest) (agentlifecycle.Agent, error)
	Get(context.Context, identity.Principal, string) (agentlifecycle.Agent, error)
	GetOverview(context.Context, identity.Principal, string) (agentlifecycle.AgentOverview, error)
	GetDraft(context.Context, identity.Principal, string) (agentlifecycle.Draft, error)
	UpdateDraft(context.Context, identity.Principal, string, int, json.RawMessage) (agentlifecycle.Draft, error)
	ListVersions(context.Context, identity.Principal, string) ([]agentlifecycle.Version, error)
	GetVersion(context.Context, identity.Principal, string, string) (agentlifecycle.Version, error)
	GetReview(context.Context, identity.Principal, string) (agentlifecycle.Review, error)
	SubmitReview(context.Context, identity.Principal, string, int, string) (agentlifecycle.Review, error)
	DecideReview(context.Context, identity.Principal, string, string, string) (agentlifecycle.Review, error)
	Publish(context.Context, identity.Principal, string, int) (agentlifecycle.Version, bool, error)
	Retire(context.Context, identity.Principal, string) error
	Rollback(context.Context, identity.Principal, string, string) error
}

type targetLifecycleService interface {
	GetTargetOverview(context.Context, identity.Principal, string) (agentlifecycle.AgentTargetOverview, error)
	ListNamedDrafts(context.Context, identity.Principal, string) ([]agentlifecycle.NamedDraft, error)
	GetNamedDraft(context.Context, identity.Principal, string, string) (agentlifecycle.NamedDraft, error)
	CreateNamedDraft(context.Context, identity.Principal, string, agentlifecycle.CreateDraftRequest) (agentlifecycle.NamedDraft, error)
	UpdateNamedDraft(context.Context, identity.Principal, string, string, int, json.RawMessage) (agentlifecycle.NamedDraft, error)
	ArchiveNamedDraft(context.Context, identity.Principal, string, string) error
	CommitNamedDraft(context.Context, identity.Principal, string, string, agentlifecycle.CommitDraftRequest) (agentlifecycle.Revision, error)
	ListRevisions(context.Context, identity.Principal, string) ([]agentlifecycle.Revision, error)
	GetRevision(context.Context, identity.Principal, string, string) (agentlifecycle.Revision, error)
	GetRevisionReview(context.Context, identity.Principal, string, string) (agentlifecycle.RevisionReview, error)
	SubmitRevisionReview(context.Context, identity.Principal, string, string, string) (agentlifecycle.RevisionReview, error)
	DecideRevisionReview(context.Context, identity.Principal, string, string, string, string) (agentlifecycle.RevisionReview, error)
	ListDeployments(context.Context, identity.Principal, string) ([]agentlifecycle.Deployment, error)
	CreateTestDeployment(context.Context, identity.Principal, string, agentlifecycle.CreateDeploymentRequest) (agentlifecycle.Deployment, error)
	PublishRevision(context.Context, identity.Principal, string, agentlifecycle.PublishRevisionRequest) (agentlifecycle.Deployment, error)
	StopTestDeployment(context.Context, identity.Principal, string, string) error
}

type assetService interface {
	ListSkills(context.Context, identity.Principal, configassets.ListOptions) ([]configassets.Skill, error)
	GetSkill(context.Context, identity.Principal, string) (configassets.Skill, error)
	ListSkillUsage(context.Context, identity.Principal, string) ([]configassets.AssetUsage, error)
	CreateSkill(context.Context, identity.Principal, configassets.CreateSkillRequest) (configassets.Skill, error)
	ListPlugins(context.Context, identity.Principal, configassets.ListOptions) ([]configassets.Plugin, error)
	GetPlugin(context.Context, identity.Principal, string) (configassets.PluginDetail, error)
	ListPluginUsage(context.Context, identity.Principal, string) ([]configassets.AssetUsage, error)
	CreatePlugin(context.Context, identity.Principal, configassets.CreatePluginRequest) (configassets.Plugin, error)
	EnablePlugin(context.Context, identity.Principal, string, string) error
	DisablePlugin(context.Context, identity.Principal, string, string) error
	ListTools(context.Context, identity.Principal, configassets.ListOptions) ([]configassets.Tool, error)
	GetTool(context.Context, identity.Principal, string) (configassets.Tool, error)
	ListToolUsage(context.Context, identity.Principal, string) ([]configassets.AssetUsage, error)
	CreateTool(context.Context, identity.Principal, configassets.CreateToolRequest) (configassets.Tool, error)
	ActivateSkill(context.Context, identity.Principal, string, string) error
	DeprecateSkill(context.Context, identity.Principal, string, string) error
	RetireSkill(context.Context, identity.Principal, string, string) error
	ActivatePlugin(context.Context, identity.Principal, string, string) error
	DeprecatePlugin(context.Context, identity.Principal, string, string) error
	RetirePlugin(context.Context, identity.Principal, string, string) error
	ActivateTool(context.Context, identity.Principal, string, string) error
	DeprecateTool(context.Context, identity.Principal, string, string) error
	RetireTool(context.Context, identity.Principal, string, string) error
}

type overviewService interface {
	Get(context.Context, identity.Principal, string) (adminoverview.Overview, error)
}

type Handler struct {
	auth      authenticator
	authorize authorizer
	service   lifecycleService
	target    targetLifecycleService
	assets    assetService
	overview  overviewService
	logger    *slog.Logger
}

func New(auth authenticator, authorize authorizer, service lifecycleService, logger *slog.Logger) http.Handler {
	return newHandler(auth, authorize, service, nil, nil, nil, logger)
}

func NewWithAssets(auth authenticator, authorize authorizer, service lifecycleService, assets assetService, logger *slog.Logger) http.Handler {
	return newHandler(auth, authorize, service, nil, assets, nil, logger)
}

func NewWithAssetsAndOverview(auth authenticator, authorize authorizer, service lifecycleService, assets assetService, overview overviewService, logger *slog.Logger) http.Handler {
	return newHandler(auth, authorize, service, nil, assets, overview, logger)
}

func NewWithTarget(auth authenticator, authorize authorizer, service lifecycleService, target targetLifecycleService, assets assetService, overview overviewService, logger *slog.Logger) http.Handler {
	return newHandler(auth, authorize, service, target, assets, overview, logger)
}

func newHandler(auth authenticator, authorize authorizer, service lifecycleService, target targetLifecycleService, assets assetService, overview overviewService, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	h := Handler{auth: auth, authorize: authorize, service: service, target: target, assets: assets, overview: overview, logger: logger}
	mux := http.NewServeMux()
	mux.Handle("GET /overview", h.withActor(h.getOverview))
	mux.Handle("GET /workspaces", h.withActor(h.listWorkspaces))
	mux.Handle("GET /agents", h.withActor(h.listAgents))
	mux.Handle("POST /agents", h.withActor(h.createAgent))
	mux.Handle("GET /agents/{agentID}", h.withActor(h.getAgent))
	mux.Handle("GET /agents/{agentID}/overview", h.withActor(h.getAgentOverview))
	mux.Handle("GET /agents/{agentID}/draft", h.withActor(h.getDraft))
	mux.Handle("PUT /agents/{agentID}/draft", h.withActor(h.updateDraft))
	mux.Handle("GET /agents/{agentID}/versions", h.withActor(h.listVersions))
	mux.Handle("GET /agents/{agentID}/versions/{versionID}", h.withActor(h.getVersion))
	mux.Handle("GET /agents/{agentID}/review", h.withActor(h.getReview))
	if target != nil {
		h.registerTargetRoutes(mux)
	}
	if assets != nil {
		mux.Handle("GET /skills", h.withActor(h.listSkills))
		mux.Handle("GET /skills/{skillID}", h.withActor(h.getSkill))
		mux.Handle("GET /skills/{skillID}/usage", h.withActor(h.listSkillUsage))
		mux.Handle("POST /skills", h.withActor(h.createSkill))
		mux.Handle("GET /plugins", h.withActor(h.listPlugins))
		mux.Handle("GET /plugins/{pluginID}", h.withActor(h.getPlugin))
		mux.Handle("GET /plugins/{pluginID}/usage", h.withActor(h.listPluginUsage))
		mux.Handle("POST /plugins", h.withActor(h.createPlugin))
		mux.Handle("POST /plugins/{pluginID}/enable", h.withActor(h.enablePlugin))
		mux.Handle("POST /plugins/{pluginID}/disable", h.withActor(h.disablePlugin))
		mux.Handle("GET /tools", h.withActor(h.listTools))
		mux.Handle("GET /tools/{toolID}", h.withActor(h.getTool))
		mux.Handle("GET /tools/{toolID}/usage", h.withActor(h.listToolUsage))
		mux.Handle("POST /tools", h.withActor(h.createTool))
		mux.Handle("POST /skills/{operation...}", h.withActor(h.skillCommand))
		mux.Handle("POST /plugins/{operation...}", h.withActor(h.pluginCommand))
		mux.Handle("POST /tools/{operation...}", h.withActor(h.toolCommand))
	}
	mux.Handle("POST /agents/{operation...}", h.withActor(h.command))
	return mux
}

func (h Handler) listSkills(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.assets.ListSkills(r.Context(), actor, configassets.ListOptions{WorkspaceID: r.URL.Query().Get("workspace_id"), Search: r.URL.Query().Get("search"), Status: r.URL.Query().Get("status")})
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "page_info": map[string]bool{"has_more": false}})
}

func (h Handler) getSkill(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	item, err := h.assets.GetSkill(r.Context(), actor, r.PathValue("skillID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h Handler) listSkillUsage(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.assets.ListSkillUsage(r.Context(), actor, r.PathValue("skillID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "page_info": map[string]bool{"has_more": false}})
}

func (h Handler) createSkill(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var request configassets.CreateSkillRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, err := h.assets.CreateSkill(r.Context(), actor, request)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h Handler) listPlugins(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.assets.ListPlugins(r.Context(), actor, configassets.ListOptions{Search: r.URL.Query().Get("search"), Status: r.URL.Query().Get("status")})
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "page_info": map[string]bool{"has_more": false}})
}

func (h Handler) getPlugin(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	item, err := h.assets.GetPlugin(r.Context(), actor, r.PathValue("pluginID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h Handler) listPluginUsage(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.assets.ListPluginUsage(r.Context(), actor, r.PathValue("pluginID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "page_info": map[string]bool{"has_more": false}})
}

func (h Handler) createPlugin(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var request configassets.CreatePluginRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, err := h.assets.CreatePlugin(r.Context(), actor, request)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h Handler) enablePlugin(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var request configassets.EnablePluginRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	if err := h.assets.EnablePlugin(r.Context(), actor, r.PathValue("pluginID"), request.WorkspaceID); err != nil {
		h.writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handler) disablePlugin(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var request configassets.EnablePluginRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	if err := h.assets.DisablePlugin(r.Context(), actor, r.PathValue("pluginID"), request.WorkspaceID); err != nil {
		h.writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handler) listTools(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.assets.ListTools(r.Context(), actor, configassets.ListOptions{Search: r.URL.Query().Get("search"), Status: r.URL.Query().Get("status")})
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "page_info": map[string]bool{"has_more": false}})
}

func (h Handler) getTool(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	item, err := h.assets.GetTool(r.Context(), actor, r.PathValue("toolID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h Handler) listToolUsage(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.assets.ListToolUsage(r.Context(), actor, r.PathValue("toolID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "page_info": map[string]bool{"has_more": false}})
}

func (h Handler) createTool(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var request configassets.CreateToolRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, err := h.assets.CreateTool(r.Context(), actor, request)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h Handler) skillCommand(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	assetID, operation, ok := strings.Cut(r.PathValue("operation"), ":")
	if !ok || assetID == "" {
		http.NotFound(w, r)
		return
	}
	var request configassets.AssetStatusRequest
	if err := decodeOptionalJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	var err error
	switch operation {
	case "activate":
		err = h.assets.ActivateSkill(r.Context(), actor, assetID, request.Reason)
	case "deprecate":
		err = h.assets.DeprecateSkill(r.Context(), actor, assetID, request.Reason)
	case "retire":
		err = h.assets.RetireSkill(r.Context(), actor, assetID, request.Reason)
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handler) pluginCommand(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	assetID, operation, ok := strings.Cut(r.PathValue("operation"), ":")
	if !ok || assetID == "" {
		http.NotFound(w, r)
		return
	}
	var request configassets.AssetStatusRequest
	if err := decodeOptionalJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	var err error
	switch operation {
	case "activate":
		err = h.assets.ActivatePlugin(r.Context(), actor, assetID, request.Reason)
	case "deprecate":
		err = h.assets.DeprecatePlugin(r.Context(), actor, assetID, request.Reason)
	case "retire":
		err = h.assets.RetirePlugin(r.Context(), actor, assetID, request.Reason)
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handler) toolCommand(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	assetID, operation, ok := strings.Cut(r.PathValue("operation"), ":")
	if !ok || assetID == "" {
		http.NotFound(w, r)
		return
	}
	var request configassets.AssetStatusRequest
	if err := decodeOptionalJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	var err error
	switch operation {
	case "activate":
		err = h.assets.ActivateTool(r.Context(), actor, assetID, request.Reason)
	case "deprecate":
		err = h.assets.DeprecateTool(r.Context(), actor, assetID, request.Reason)
	case "retire":
		err = h.assets.RetireTool(r.Context(), actor, assetID, request.Reason)
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type actorHandler func(http.ResponseWriter, *http.Request, identity.Principal)

func (h Handler) withActor(next actorHandler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor, err := h.auth.Authenticate(r.Context(), r.Header.Get("Authorization"))
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", "A valid Admin access token is required.")
			return
		}
		if err := h.authorize.RequireAdmin(r.Context(), actor); err != nil {
			if errors.Is(err, authorization.ErrForbidden) {
				writeError(w, http.StatusForbidden, "forbidden", "Administrative access is required.")
				return
			}
			h.writeInternal(w, err)
			return
		}
		next(w, r, actor)
	})
}

func (h Handler) listWorkspaces(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.service.ListWorkspaces(r.Context(), actor)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "page_info": map[string]bool{"has_more": false}})
}

func (h Handler) getOverview(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	if h.overview == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "The Admin overview is not configured.")
		return
	}
	overview, err := h.overview.Get(r.Context(), actor, r.URL.Query().Get("workspace_id"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (h Handler) listAgents(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.service.ListAgents(r.Context(), actor, r.URL.Query().Get("workspace_id"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "page_info": map[string]bool{"has_more": false}})
}

func (h Handler) createAgent(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var request agentlifecycle.CreateRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	agent, err := h.service.Create(r.Context(), actor, request)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, agent)
}

func (h Handler) getAgent(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	agent, err := h.service.Get(r.Context(), actor, r.PathValue("agentID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, agent)
}

func (h Handler) getAgentOverview(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	overview, err := h.service.GetOverview(r.Context(), actor, r.PathValue("agentID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (h Handler) getDraft(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	draft, err := h.service.GetDraft(r.Context(), actor, r.PathValue("agentID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, draft)
}

func (h Handler) updateDraft(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	revision, err := revisionHeader(r)
	if err != nil {
		writeError(w, http.StatusPreconditionRequired, "revision_required", "If-Match must contain the current draft revision.")
		return
	}
	var request struct {
		Spec json.RawMessage `json:"spec"`
	}
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	draft, err := h.service.UpdateDraft(r.Context(), actor, r.PathValue("agentID"), revision, request.Spec)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, draft)
}

func (h Handler) listVersions(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	items, err := h.service.ListVersions(r.Context(), actor, r.PathValue("agentID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "page_info": map[string]bool{"has_more": false}})
}

func (h Handler) getVersion(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	version, err := h.service.GetVersion(r.Context(), actor, r.PathValue("agentID"), r.PathValue("versionID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, version)
}

func (h Handler) getReview(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	review, err := h.service.GetReview(r.Context(), actor, r.PathValue("agentID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, review)
}

func (h Handler) command(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	agentID, operation, ok := strings.Cut(r.PathValue("operation"), ":")
	if !ok || agentID == "" {
		http.NotFound(w, r)
		return
	}
	switch operation {
	case "review":
		revision, err := revisionHeader(r)
		if err != nil {
			writeError(w, http.StatusPreconditionRequired, "revision_required", "If-Match must contain the current draft revision.")
			return
		}
		var request struct {
			ReleaseNotes string `json:"release_notes"`
		}
		if err := decodeJSON(w, r, &request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
			return
		}
		review, err := h.service.SubmitReview(r.Context(), actor, agentID, revision, request.ReleaseNotes)
		if err != nil {
			h.writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, review)
	case "review-decision":
		var request agentlifecycle.ReviewDecisionRequest
		if err := decodeJSON(w, r, &request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
			return
		}
		review, err := h.service.DecideReview(r.Context(), actor, agentID, request.Decision, request.Reason)
		if err != nil {
			h.writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, review)
	case "publish":
		revision, err := revisionHeader(r)
		if err != nil {
			writeError(w, http.StatusPreconditionRequired, "revision_required", "If-Match must contain the current draft revision.")
			return
		}
		version, duplicate, err := h.service.Publish(r.Context(), actor, agentID, revision)
		if err != nil {
			h.writeServiceError(w, err)
			return
		}
		status := http.StatusCreated
		if duplicate {
			status = http.StatusOK
		}
		writeJSON(w, status, version)
	case "retire":
		if err := h.service.Retire(r.Context(), actor, agentID); err != nil {
			h.writeServiceError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case "rollback":
		var request agentlifecycle.RollbackRequest
		if err := decodeJSON(w, r, &request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
			return
		}
		if err := h.service.Rollback(r.Context(), actor, agentID, request.VersionID); err != nil {
			h.writeServiceError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.NotFound(w, r)
	}
}

func revisionHeader(r *http.Request) (int, error) {
	value := strings.Trim(strings.TrimSpace(r.Header.Get("If-Match")), `"`)
	revision, err := strconv.Atoi(value)
	if err != nil || revision < 1 {
		return 0, errors.New("invalid revision")
	}
	return revision, nil
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("multiple JSON values")
	}
	return nil
}

func decodeOptionalJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	if r.Body == nil || r.Body == http.NoBody || r.ContentLength == 0 {
		return nil
	}
	return decodeJSON(w, r, destination)
}

func (h Handler) writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, agentlifecycle.ErrNotFound), errors.Is(err, configassets.ErrNotFound), errors.Is(err, authorization.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Resource was not found.")
	case errors.Is(err, agentlifecycle.ErrInvalidInput):
		writeError(w, http.StatusUnprocessableEntity, "invalid_input", "The agent request is not valid.")
	case errors.Is(err, configassets.ErrInvalidInput):
		writeError(w, http.StatusUnprocessableEntity, "invalid_input", "The configuration asset request is not valid.")
	case errors.Is(err, agentlifecycle.ErrRevisionConflict):
		writeError(w, http.StatusPreconditionFailed, "revision_conflict", "The draft was changed by another administrator.")
	case errors.Is(err, agentlifecycle.ErrInvalidState):
		writeError(w, http.StatusConflict, "invalid_state", "The agent is not in a state that permits this operation.")
	case errors.Is(err, agentlifecycle.ErrReviewRequired):
		writeError(w, http.StatusConflict, "review_required", "An approved review for the current draft is required before publication.")
	case errors.Is(err, authorization.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "Administrative access is required.")
	default:
		h.writeInternal(w, err)
	}
}

func (h Handler) writeInternal(w http.ResponseWriter, err error) {
	h.logger.Error("admin API request failed", "error", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
