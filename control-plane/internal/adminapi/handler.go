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

	"github.com/AirSodaz/gantry/internal/adminaudit"
	"github.com/AirSodaz/gantry/internal/adminevaluation"
	"github.com/AirSodaz/gantry/internal/adminintegration"
	"github.com/AirSodaz/gantry/internal/adminoverview"
	"github.com/AirSodaz/gantry/internal/adminplatform"
	"github.com/AirSodaz/gantry/internal/adminpolicy"
	"github.com/AirSodaz/gantry/internal/adminruns"
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
	ListAgents(context.Context, identity.Principal, agentlifecycle.AgentListOptions) ([]agentlifecycle.Agent, error)
	Create(context.Context, identity.Principal, agentlifecycle.CreateRequest) (agentlifecycle.Agent, error)
	Get(context.Context, identity.Principal, string) (agentlifecycle.Agent, error)
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

type runService interface {
	List(context.Context, identity.Principal, adminruns.ListOptions) ([]adminruns.Run, error)
	Get(context.Context, identity.Principal, string) (adminruns.Detail, error)
}

type auditService interface {
	List(context.Context, identity.Principal, adminaudit.ListOptions) (adminaudit.ListResult, error)
	Get(context.Context, identity.Principal, string) (adminaudit.Detail, error)
	CreateExport(context.Context, identity.Principal, adminaudit.ListOptions) (adminaudit.Export, error)
	GetExport(context.Context, identity.Principal, string) (adminaudit.Export, error)
	DownloadExport(context.Context, identity.Principal, string) (adminaudit.Download, error)
}

type policyService interface {
	List(context.Context, identity.Principal, adminpolicy.ListOptions) (adminpolicy.ListResult, error)
	Create(context.Context, identity.Principal, adminpolicy.CreateRequest) (adminpolicy.Policy, adminpolicy.Draft, error)
	Get(context.Context, identity.Principal, string) (adminpolicy.Policy, error)
	GetDraft(context.Context, identity.Principal, string) (adminpolicy.Draft, error)
	UpdateDraft(context.Context, identity.Principal, string, string, adminpolicy.UpdateDraftRequest) (adminpolicy.Draft, error)
	Validate(context.Context, identity.Principal, string) (adminpolicy.Draft, error)
	ListVersions(context.Context, identity.Principal, string) ([]adminpolicy.Version, error)
	Publish(context.Context, identity.Principal, string, string, string, adminpolicy.PublishRequest) (adminpolicy.Version, error)
	ListBindings(context.Context, identity.Principal, string) ([]adminpolicy.Binding, error)
	Bind(context.Context, identity.Principal, string, string, adminpolicy.BindRequest) (adminpolicy.Binding, error)
	RevokeBinding(context.Context, identity.Principal, string, string, string) (adminpolicy.Binding, error)
	Simulate(context.Context, identity.Principal, string, adminpolicy.SimulationRequest) (adminpolicy.Simulation, error)
	Retire(context.Context, identity.Principal, string, string, string) (adminpolicy.Policy, error)
}

type evaluationService interface {
	ListSuites(context.Context, identity.Principal, adminevaluation.ListOptions) (adminevaluation.SuiteList, error)
	CreateSuite(context.Context, identity.Principal, adminevaluation.CreateSuiteRequest) (adminevaluation.Suite, error)
	GetSuite(context.Context, identity.Principal, string) (adminevaluation.Suite, error)
	PatchSuite(context.Context, identity.Principal, string, string, adminevaluation.PatchSuiteRequest) (adminevaluation.Suite, error)
	ListCases(context.Context, identity.Principal, string) ([]adminevaluation.Case, error)
	CreateCase(context.Context, identity.Principal, string, adminevaluation.CreateCaseRequest) (adminevaluation.Case, error)
	PatchCase(context.Context, identity.Principal, string, string, string, adminevaluation.PatchCaseRequest) (adminevaluation.Case, error)
	ValidateSuite(context.Context, identity.Principal, string) (adminevaluation.Validation, error)
	ListVersions(context.Context, identity.Principal, string) ([]adminevaluation.Version, error)
	PublishVersion(context.Context, identity.Principal, string, string, string, adminevaluation.PublishVersionRequest) (adminevaluation.Version, error)
	ListRuns(context.Context, identity.Principal, string) ([]adminevaluation.Run, error)
	CreateRun(context.Context, identity.Principal, string, string, adminevaluation.CreateRunRequest) (adminevaluation.Run, error)
	GetRun(context.Context, identity.Principal, string) (adminevaluation.Run, error)
	CancelRun(context.Context, identity.Principal, string) (adminevaluation.Run, error)
	ListRunRegressions(context.Context, identity.Principal, string) (adminevaluation.RegressionList, error)
	ListGates(context.Context, identity.Principal, string, string) (adminevaluation.GateList, error)
	OverrideGate(context.Context, identity.Principal, string, adminevaluation.OverrideGateRequest) (adminevaluation.Gate, error)
}

type integrationService interface {
	List(context.Context, identity.Principal, string) ([]adminintegration.Integration, error)
	Get(context.Context, identity.Principal, string) (adminintegration.Integration, error)
	Create(context.Context, identity.Principal, adminintegration.CreateIntegrationRequest) (adminintegration.Integration, error)
	Patch(context.Context, identity.Principal, string, adminintegration.PatchIntegrationRequest) (adminintegration.Integration, error)
	ListClients(context.Context, identity.Principal, string) ([]adminintegration.Client, error)
	CreateClient(context.Context, identity.Principal, string, adminintegration.CreateClientRequest) (adminintegration.Client, error)
	RotateClient(context.Context, identity.Principal, string, string) (adminintegration.Client, error)
	DisableClient(context.Context, identity.Principal, string) error
	ListPublications(context.Context, identity.Principal, string) ([]adminintegration.Publication, error)
	CreatePublication(context.Context, identity.Principal, string, adminintegration.CreatePublicationRequest) (adminintegration.Publication, error)
	RevokePublication(context.Context, identity.Principal, string) error
	ListWebhooks(context.Context, identity.Principal, string) ([]adminintegration.Webhook, error)
	CreateWebhook(context.Context, identity.Principal, string, adminintegration.CreateWebhookRequest) (adminintegration.Webhook, error)
	Redeliver(context.Context, identity.Principal, string, string) (adminintegration.Delivery, error)
}

type platformService interface {
	ListProviders(context.Context, identity.Principal) ([]adminplatform.ModelProvider, error)
	CreateProvider(context.Context, identity.Principal, adminplatform.CreateProviderRequest) (adminplatform.ModelProvider, error)
	ListRoutes(context.Context, identity.Principal, string) ([]adminplatform.ProviderRoute, error)
	PutRoute(context.Context, identity.Principal, string, string, string, adminplatform.PutRouteRequest) (adminplatform.ProviderRoute, error)
	QuarantineProvider(context.Context, identity.Principal, string) (adminplatform.ModelProvider, error)
	ListRunnerPools(context.Context, identity.Principal) ([]adminplatform.RunnerPool, error)
	CreateRunnerPool(context.Context, identity.Principal, adminplatform.CreateRunnerPoolRequest) (adminplatform.RunnerPool, error)
	ListRunners(context.Context, identity.Principal, string) ([]adminplatform.Runner, error)
	SetPoolState(context.Context, identity.Principal, string, string) (adminplatform.RunnerPool, error)
	ListCredentials(context.Context, identity.Principal) ([]adminplatform.CredentialReference, error)
	RotateCredential(context.Context, identity.Principal, string) (adminplatform.CredentialReference, error)
	RevokeCredential(context.Context, identity.Principal, string) (adminplatform.CredentialReference, error)
	ListClassifications(context.Context, identity.Principal) ([]adminplatform.DataClassification, error)
	CreateClassification(context.Context, identity.Principal, adminplatform.CreateDataClassificationRequest) (adminplatform.DataClassification, error)
	ListLimitPolicies(context.Context, identity.Principal, string) ([]adminplatform.LimitPolicy, error)
	UpsertLimitPolicy(context.Context, identity.Principal, string, string, adminplatform.UpsertLimitPolicyRequest) (adminplatform.LimitPolicy, error)
	ListEnvironmentProfiles(context.Context, identity.Principal, string) ([]adminplatform.EnvironmentProfile, error)
	UpsertEnvironmentProfile(context.Context, identity.Principal, string, string, adminplatform.UpsertEnvironmentProfileRequest) (adminplatform.EnvironmentProfile, error)
	GetSettings(context.Context, identity.Principal, string) (adminplatform.PlatformSettingsProjection, error)
	ValidateSettings(context.Context, identity.Principal, adminplatform.SettingsApplyRequest) (adminplatform.SettingsValidation, error)
	ApplySettings(context.Context, identity.Principal, string, adminplatform.SettingsApplyRequest) (adminplatform.PlatformSettingsProjection, error)
}

type Handler struct {
	auth         authenticator
	authorize    authorizer
	service      lifecycleService
	target       targetLifecycleService
	assets       assetService
	overview     overviewService
	runs         runService
	audit        auditService
	policies     policyService
	evaluations  evaluationService
	integrations integrationService
	platform     platformService
	logger       *slog.Logger
}

func New(auth authenticator, authorize authorizer, service lifecycleService, logger *slog.Logger) http.Handler {
	return newHandler(auth, authorize, service, nil, nil, nil, nil, logger)
}

func NewWithAssets(auth authenticator, authorize authorizer, service lifecycleService, assets assetService, logger *slog.Logger) http.Handler {
	return newHandler(auth, authorize, service, nil, assets, nil, nil, logger)
}

func NewWithAssetsAndOverview(auth authenticator, authorize authorizer, service lifecycleService, assets assetService, overview overviewService, logger *slog.Logger) http.Handler {
	return newHandler(auth, authorize, service, nil, assets, overview, nil, logger)
}

func NewWithTarget(auth authenticator, authorize authorizer, service lifecycleService, target targetLifecycleService, assets assetService, overview overviewService, runs runService, logger *slog.Logger) http.Handler {
	return newHandler(auth, authorize, service, target, assets, overview, runs, logger)
}

func NewWithTargetAndAudit(auth authenticator, authorize authorizer, service lifecycleService, target targetLifecycleService, assets assetService, overview overviewService, runs runService, audit auditService, logger *slog.Logger) http.Handler {
	return newHandlerWithPolicy(auth, authorize, service, target, assets, overview, runs, audit, nil, logger)
}

func NewWithTargetAuditAndPolicy(auth authenticator, authorize authorizer, service lifecycleService, target targetLifecycleService, assets assetService, overview overviewService, runs runService, audit auditService, policies policyService, logger *slog.Logger) http.Handler {
	return newHandlerWithPolicy(auth, authorize, service, target, assets, overview, runs, audit, policies, logger)
}

func NewWithTargetAuditPolicyEvaluation(auth authenticator, authorize authorizer, service lifecycleService, target targetLifecycleService, assets assetService, overview overviewService, runs runService, audit auditService, policies policyService, evaluations evaluationService, logger *slog.Logger) http.Handler {
	return newHandlerWithEvaluation(auth, authorize, service, target, assets, overview, runs, audit, policies, evaluations, nil, nil, logger)
}

func NewWithTargetAuditPolicyEvaluationIntegrations(auth authenticator, authorize authorizer, service lifecycleService, target targetLifecycleService, assets assetService, overview overviewService, runs runService, audit auditService, policies policyService, evaluations evaluationService, integrations integrationService, logger *slog.Logger) http.Handler {
	return newHandlerWithEvaluation(auth, authorize, service, target, assets, overview, runs, audit, policies, evaluations, integrations, nil, logger)
}

func NewWithTargetAuditPolicyEvaluationPlatform(auth authenticator, authorize authorizer, service lifecycleService, target targetLifecycleService, assets assetService, overview overviewService, runs runService, audit auditService, policies policyService, evaluations evaluationService, integrations integrationService, platform platformService, logger *slog.Logger) http.Handler {
	return newHandlerWithEvaluation(auth, authorize, service, target, assets, overview, runs, audit, policies, evaluations, integrations, platform, logger)
}

func newHandler(auth authenticator, authorize authorizer, service lifecycleService, target targetLifecycleService, assets assetService, overview overviewService, runs runService, logger *slog.Logger) http.Handler {
	return newHandlerWithAudit(auth, authorize, service, target, assets, overview, runs, nil, logger)
}

func newHandlerWithAudit(auth authenticator, authorize authorizer, service lifecycleService, target targetLifecycleService, assets assetService, overview overviewService, runs runService, audit auditService, logger *slog.Logger) http.Handler {
	return newHandlerWithPolicy(auth, authorize, service, target, assets, overview, runs, audit, nil, logger)
}

func newHandlerWithPolicy(auth authenticator, authorize authorizer, service lifecycleService, target targetLifecycleService, assets assetService, overview overviewService, runs runService, audit auditService, policies policyService, logger *slog.Logger) http.Handler {
	return newHandlerWithEvaluation(auth, authorize, service, target, assets, overview, runs, audit, policies, nil, nil, nil, logger)
}

func newHandlerWithEvaluation(auth authenticator, authorize authorizer, service lifecycleService, target targetLifecycleService, assets assetService, overview overviewService, runs runService, audit auditService, policies policyService, evaluations evaluationService, integrations integrationService, platform platformService, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	h := Handler{auth: auth, authorize: authorize, service: service, target: target, assets: assets, overview: overview, runs: runs, audit: audit, policies: policies, evaluations: evaluations, integrations: integrations, platform: platform, logger: logger}
	mux := http.NewServeMux()
	mux.Handle("GET /overview", h.withActor(h.getOverview))
	mux.Handle("GET /workspaces", h.withActor(h.listWorkspaces))
	mux.Handle("GET /agents", h.withActor(h.listAgents))
	mux.Handle("POST /agents", h.withActor(h.createAgent))
	mux.Handle("GET /agents/{agentID}", h.withActor(h.getAgent))
	if runs != nil {
		mux.Handle("GET /runs", h.withActor(h.listRuns))
		mux.Handle("GET /runs/{runID}", h.withActor(h.getRun))
	}
	if audit != nil {
		mux.Handle("GET /audit-events", h.withActor(h.listAuditEvents))
		mux.Handle("GET /audit-events/{eventID}", h.withActor(h.getAuditEvent))
		mux.Handle("POST /audit-events:export", h.withActor(h.createAuditExport))
		mux.Handle("GET /audit-exports/{exportID}", h.withActor(h.getAuditExport))
		mux.Handle("GET /audit-exports/{exportID}/download", h.withActor(h.downloadAuditExport))
	}
	if policies != nil {
		mux.Handle("GET /policies", h.withActor(h.listPolicies))
		mux.Handle("POST /policies", h.withActor(h.createPolicy))
		mux.Handle("GET /policies/{policyID}", h.withActor(h.getPolicy))
		mux.Handle("GET /policies/{policyID}/draft", h.withActor(h.getPolicyDraft))
		mux.Handle("PATCH /policies/{policyID}/draft", h.withActor(h.updatePolicyDraft))
		mux.Handle("POST /policies/{policyID}", h.withActor(h.policyCommand))
		mux.Handle("GET /policies/{policyID}/versions", h.withActor(h.listPolicyVersions))
		mux.Handle("POST /policies/{policyID}/versions", h.withActor(h.publishPolicyVersion))
		mux.Handle("GET /policies/{policyID}/bindings", h.withActor(h.listPolicyBindings))
		mux.Handle("POST /policies/{policyID}/bindings", h.withActor(h.bindPolicy))
		mux.Handle("POST /policy-bindings/{bindingID}", h.withActor(h.policyBindingCommand))
	}
	if evaluations != nil {
		mux.Handle("GET /evaluation-suites", h.withActor(h.listEvaluationSuites))
		mux.Handle("POST /evaluation-suites", h.withActor(h.createEvaluationSuite))
		mux.Handle("GET /evaluation-suites/{suiteID}", h.withActor(h.getEvaluationSuite))
		mux.Handle("PATCH /evaluation-suites/{suiteID}", h.withActor(h.patchEvaluationSuite))
		mux.Handle("GET /evaluation-suites/{suiteID}/cases", h.withActor(h.listEvaluationCases))
		mux.Handle("POST /evaluation-suites/{suiteID}/cases", h.withActor(h.createEvaluationCase))
		mux.Handle("PATCH /evaluation-suites/{suiteID}/cases/{caseID}", h.withActor(h.patchEvaluationCase))
		mux.Handle("POST /evaluation-suites/{suiteID}", h.withActor(h.evaluationSuiteCommand))
		mux.Handle("GET /evaluation-suites/{suiteID}/versions", h.withActor(h.listEvaluationVersions))
		mux.Handle("POST /evaluation-suites/{suiteID}/versions", h.withActor(h.publishEvaluationVersion))
		mux.Handle("GET /evaluation-suites/{suiteID}/runs", h.withActor(h.listEvaluationRuns))
		mux.Handle("POST /evaluation-suites/{suiteID}/runs", h.withActor(h.createEvaluationRun))
		mux.Handle("GET /evaluation-runs/{runID}", h.withActor(h.getEvaluationRun))
		mux.Handle("POST /evaluation-runs/{runID}", h.withActor(h.evaluationRunCommand))
		mux.Handle("GET /evaluation-runs/{runID}/regressions", h.withActor(h.listEvaluationRunRegressions))
		mux.Handle("GET /evaluation-gates", h.withActor(h.listEvaluationGates))
		mux.Handle("POST /evaluation-gates/{gateID}", h.withActor(h.evaluationGateCommand))
	}
	if integrations != nil {
		mux.Handle("GET /integrations", h.withActor(h.listIntegrations))
		mux.Handle("POST /integrations", h.withActor(h.createIntegration))
		mux.Handle("GET /integrations/{integrationID}", h.withActor(h.getIntegration))
		mux.Handle("PATCH /integrations/{integrationID}", h.withActor(h.patchIntegration))
		mux.Handle("GET /integrations/{integrationID}/clients", h.withActor(h.listIntegrationClients))
		mux.Handle("POST /integrations/{integrationID}/clients", h.withActor(h.createIntegrationClient))
		mux.Handle("POST /integration-clients/{clientID}", h.withActor(h.integrationClientCommand))
		mux.Handle("GET /integrations/{integrationID}/publications", h.withActor(h.listIntegrationPublications))
		mux.Handle("POST /integrations/{integrationID}/publications", h.withActor(h.createIntegrationPublication))
		mux.Handle("POST /integration-publications/{publicationID}", h.withActor(h.integrationPublicationCommand))
		mux.Handle("GET /integrations/{integrationID}/webhooks", h.withActor(h.listIntegrationWebhooks))
		mux.Handle("POST /integrations/{integrationID}/webhooks", h.withActor(h.createIntegrationWebhook))
		mux.Handle("POST /webhook-endpoints/{endpointID}", h.withActor(h.webhookCommand))
	}
	if platform != nil {
		mux.Handle("GET /platform/model-providers", h.withActor(h.listPlatformProviders))
		mux.Handle("POST /platform/model-providers", h.withActor(h.createPlatformProvider))
		mux.Handle("GET /platform/model-providers/{providerID}/routes", h.withActor(h.listProviderRoutes))
		mux.Handle("PUT /platform/model-providers/{providerID}/routes/{routeID}", h.withActor(h.putProviderRoute))
		mux.Handle("POST /platform/model-providers/{providerID}", h.withActor(h.platformProviderCommand))
		mux.Handle("GET /platform/runner-pools", h.withActor(h.listRunnerPools))
		mux.Handle("POST /platform/runner-pools", h.withActor(h.createRunnerPool))
		mux.Handle("GET /platform/runner-pools/{poolID}/runners", h.withActor(h.listRunners))
		mux.Handle("POST /platform/runner-pools/{poolID}", h.withActor(h.runnerPoolCommand))
		mux.Handle("GET /platform/credentials", h.withActor(h.listPlatformCredentials))
		mux.Handle("POST /platform/credentials/{credentialID}", h.withActor(h.platformCredentialCommand))
		mux.Handle("GET /platform/data-classifications", h.withActor(h.listDataClassifications))
		mux.Handle("POST /platform/data-classifications", h.withActor(h.createDataClassification))
		mux.Handle("GET /platform/limit-policies", h.withActor(h.listLimitPolicies))
		mux.Handle("PUT /platform/limit-policies/{policyID}", h.withActor(h.upsertLimitPolicy))
		mux.Handle("GET /platform/environment-profiles", h.withActor(h.listEnvironmentProfiles))
		mux.Handle("PUT /platform/environment-profiles/{profileID}", h.withActor(h.upsertEnvironmentProfile))
		mux.Handle("GET /platform/settings", h.withActor(h.getPlatformSettings))
		mux.Handle("POST /platform/settings:validate", h.withActor(h.validatePlatformSettings))
		mux.Handle("POST /platform/settings:apply", h.withActor(h.applyPlatformSettings))
	}
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
	return mux
}

func (h Handler) listAuditEvents(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	limit, err := queryLimit(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "limit must be a positive integer no greater than 100.")
		return
	}
	result, err := h.audit.List(r.Context(), actor, adminaudit.ListOptions{
		WorkspaceID: r.URL.Query().Get("workspace_id"), ResourceType: r.URL.Query().Get("resource_type"), ResourceID: r.URL.Query().Get("resource_id"),
		ActorID: r.URL.Query().Get("actor_id"), EventType: r.URL.Query().Get("event_type"), Outcome: r.URL.Query().Get("outcome"), Risk: r.URL.Query().Get("risk"),
		CorrelationID: r.URL.Query().Get("correlation_id"), RunID: r.URL.Query().Get("run_id"), RevisionHash: r.URL.Query().Get("revision_hash"), PolicyVersionID: r.URL.Query().Get("policy_version_id"),
		Before: r.URL.Query().Get("before"), After: r.URL.Query().Get("after"), Limit: limit, Cursor: r.URL.Query().Get("cursor"),
	})
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h Handler) getAuditEvent(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	item, err := h.audit.Get(r.Context(), actor, r.PathValue("eventID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h Handler) createAuditExport(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	var options adminaudit.ListOptions
	if err := decodeOptionalJSON(w, r, &options); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "Request body must be valid JSON.")
		return
	}
	item, err := h.audit.CreateExport(r.Context(), actor, options)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, item)
}

func (h Handler) getAuditExport(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	item, err := h.audit.GetExport(r.Context(), actor, r.PathValue("exportID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h Handler) downloadAuditExport(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	item, err := h.audit.DownloadExport(r.Context(), actor, r.PathValue("exportID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (h Handler) listRuns(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	limit, err := queryLimit(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "limit must be a positive integer no greater than 100.")
		return
	}
	items, err := h.runs.List(r.Context(), actor, adminruns.ListOptions{
		WorkspaceID:  r.URL.Query().Get("workspace_id"),
		AgentID:      r.URL.Query().Get("agent_id"),
		RevisionHash: r.URL.Query().Get("revision_hash"),
		Status:       r.URL.Query().Get("status"),
		Limit:        limit,
	})
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "page_info": map[string]bool{"has_more": false}})
}

func (h Handler) getRun(w http.ResponseWriter, r *http.Request, actor identity.Principal) {
	detail, err := h.runs.Get(r.Context(), actor, r.PathValue("runID"))
	if err != nil {
		h.writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func queryLimit(r *http.Request) (int, error) {
	value := strings.TrimSpace(r.URL.Query().Get("limit"))
	if value == "" {
		return 50, nil
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit < 1 || limit > 100 {
		return 0, errors.New("invalid limit")
	}
	return limit, nil
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
	items, err := h.service.ListAgents(r.Context(), actor, agentlifecycle.AgentListOptions{WorkspaceID: r.URL.Query().Get("workspace_id"), Search: r.URL.Query().Get("search"), Status: r.URL.Query().Get("status")})
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
	case errors.Is(err, agentlifecycle.ErrNotFound), errors.Is(err, configassets.ErrNotFound), errors.Is(err, adminruns.ErrNotFound), errors.Is(err, adminaudit.ErrNotFound), errors.Is(err, authorization.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Resource was not found.")
	case errors.Is(err, agentlifecycle.ErrInvalidInput):
		writeError(w, http.StatusUnprocessableEntity, "invalid_input", "The agent request is not valid.")
	case errors.Is(err, configassets.ErrInvalidInput):
		writeError(w, http.StatusUnprocessableEntity, "invalid_input", "The configuration asset request is not valid.")
	case errors.Is(err, adminruns.ErrInvalidInput):
		writeError(w, http.StatusUnprocessableEntity, "invalid_input", "The run query is not valid.")
	case errors.Is(err, adminaudit.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, "invalid_request", "The audit query is not valid.")
	case errors.Is(err, adminpolicy.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Resource was not found.")
	case errors.Is(err, adminpolicy.ErrInvalidInput):
		writeError(w, http.StatusUnprocessableEntity, "invalid_input", "The policy request is not valid.")
	case errors.Is(err, adminpolicy.ErrSchemaInvalid):
		writeError(w, http.StatusUnprocessableEntity, "schema_invalid", "The typed policy document is not valid.")
	case errors.Is(err, adminpolicy.ErrETagConflict):
		writeError(w, http.StatusConflict, "etag_conflict", "The policy draft changed; reload it before editing.")
	case errors.Is(err, adminpolicy.ErrIdempotencyConflict):
		writeError(w, http.StatusConflict, "idempotency_conflict", "The idempotency key was already used with a different request.")
	case errors.Is(err, adminpolicy.ErrInvalidState):
		writeError(w, http.StatusConflict, "invalid_state", "The policy is not in a state that permits this operation.")
	case errors.Is(err, adminevaluation.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Evaluation resource was not found.")
	case errors.Is(err, adminevaluation.ErrInvalidInput):
		writeError(w, http.StatusUnprocessableEntity, "invalid_input", "The evaluation request is not valid.")
	case errors.Is(err, adminevaluation.ErrFixtureInvalid):
		writeError(w, http.StatusConflict, "fixture_miss", "The evaluation suite has invalid or incomplete fixtures.")
	case errors.Is(err, adminevaluation.ErrETagConflict):
		writeError(w, http.StatusConflict, "etag_conflict", "The evaluation working copy changed; reload it before editing.")
	case errors.Is(err, adminevaluation.ErrIdempotencyConflict):
		writeError(w, http.StatusConflict, "idempotency_conflict", "The idempotency key was already used with a different request.")
	case errors.Is(err, adminevaluation.ErrInvalidState):
		writeError(w, http.StatusConflict, "invalid_state", "The evaluation resource is not in a state that permits this operation.")
	case errors.Is(err, adminintegration.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Resource was not found.")
	case errors.Is(err, adminintegration.ErrInvalidInput):
		writeError(w, http.StatusUnprocessableEntity, "invalid_input", "The integration request is not valid.")
	case errors.Is(err, adminintegration.ErrInvalidState):
		writeError(w, http.StatusConflict, "invalid_state", "The integration is not in a state that permits this operation.")
	case errors.Is(err, adminplatform.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "Resource was not found.")
	case errors.Is(err, adminplatform.ErrInvalidInput):
		writeError(w, http.StatusUnprocessableEntity, "invalid_input", "The platform request is not valid.")
	case errors.Is(err, adminplatform.ErrETagConflict):
		writeError(w, http.StatusConflict, "etag_conflict", "The platform resource changed; reload it before editing.")
	case errors.Is(err, adminplatform.ErrInvalidState):
		writeError(w, http.StatusConflict, "invalid_state", "The platform resource is not in a state that permits this operation.")
	case errors.Is(err, adminaudit.ErrExportNotReady):
		writeError(w, http.StatusConflict, "export_not_ready", "The audit export is not ready for download.")
	case errors.Is(err, adminaudit.ErrExportUnavailable):
		writeError(w, http.StatusServiceUnavailable, "export_unavailable", "Audit export storage is unavailable.")
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
