import { afterEach, describe, expect, it, vi } from "vitest";
import { AdminApi, AdminApiError } from "./client";

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
});

describe("AdminApi", () => {
  it("fails locally when the authenticated session is absent", async () => {
    const api = new AdminApi(() => null);
    await expect(api.listWorkspaces()).rejects.toBeInstanceOf(AdminApiError);
  });

  it("uses the configuration catalog routes and workspace enablement command", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ id: "skill_1" }), { status: 201 }),
      )
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    globalThis.fetch = fetchMock;
    const api = new AdminApi(() => "admin-token");
    await api.registerSkill({
      workspace_id: "ws_1",
      slug: "search",
      display_name: "Search",
      description: "",
      source_type: "locator",
      source_ref: "registry://search",
      declared_version: "",
      content_digest: "sha256:1",
    });
    await api.enablePlugin("plugin_1", "ws_1");
    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/admin/v1/skills",
      expect.objectContaining({
        method: "POST",
        body: expect.stringContaining("registry://search"),
      }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/admin/v1/plugins/plugin_1/enable",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ workspace_id: "ws_1" }),
      }),
    );
  });

  it("loads asset detail and posts an auditable lifecycle command", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ id: "skill_1", status: "deprecated" }), {
          status: 200,
        }),
      )
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    globalThis.fetch = fetchMock;
    const api = new AdminApi(() => "admin-token");
    await api.getSkill("skill_1");
    await api.activateSkill("skill_1", "validated replacement package");
    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/admin/v1/skills/skill_1",
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer admin-token",
        }),
      }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/admin/v1/skills/skill_1:activate",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ reason: "validated replacement package" }),
      }),
    );
  });

  it("encodes catalog search and status filters", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(
        new Response(JSON.stringify({ items: [] }), { status: 200 }),
      );
    globalThis.fetch = fetchMock;
    const api = new AdminApi(() => "admin-token");
    await api.listTools({ search: "data export", status: "deprecated" });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/admin/v1/tools?search=data+export&status=deprecated",
      expect.anything(),
    );
  });

  it("encodes the Admin Run workbench filters", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(
        new Response(JSON.stringify({ items: [] }), { status: 200 }),
      );
    globalThis.fetch = fetchMock;
    const api = new AdminApi(() => "admin-token");
    await api.listRuns({
      workspaceId: "ws_1",
      agentId: "agt_1",
      revisionHash: "sha256:abc",
      status: "failed",
      limit: 20,
    });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/admin/v1/runs?workspace_id=ws_1&agent_id=agt_1&revision_hash=sha256%3Aabc&status=failed&limit=20",
      expect.anything(),
    );
  });

  it("preserves Policy Draft ETags and idempotency headers", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            policy_id: "pol_1",
            etag: "2",
            document: {},
            schema_version: "gantry.policy/v1",
            validation: { state: "valid", findings: [] },
          }),
          { status: 200 },
        ),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ id: "pver_1" }), { status: 201 }),
      );
    globalThis.fetch = fetchMock;
    const api = new AdminApi(() => "admin-token");
    await api.updatePolicyDraft("pol_1", "2", {
      document: { kind: "approval", rules: [] },
    });
    await api.publishPolicyVersion("pol_1", "2", "Publish", "publish-1");
    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/admin/v1/policies/pol_1/draft",
      expect.objectContaining({
        headers: expect.objectContaining({ "If-Match": '"2"' }),
      }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/admin/v1/policies/pol_1/versions",
      expect.objectContaining({
        headers: expect.objectContaining({
          "If-Match": '"2"',
          "Idempotency-Key": "publish-1",
        }),
      }),
    );
  });

  it("uses the Integration management routes", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ items: [] }), { status: 200 }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ id: "int_1" }), { status: 201 }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ items: [] }), { status: 200 }),
      );
    globalThis.fetch = fetchMock;
    const api = new AdminApi(() => "admin-token");
    await api.listIntegrations({
      state: "active",
      search: "human resources",
      environment: "production",
    });
    await api.createIntegration({ slug: "hr", display_name: "HR" });
    await api.listIntegrationClients("int_1");
    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/admin/v1/integrations?state=active&search=human+resources&environment=production",
      expect.anything(),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/admin/v1/integrations",
      expect.objectContaining({ method: "POST" }),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      3,
      "/api/admin/v1/integrations/int_1/clients",
      expect.anything(),
    );
  });

  it("uses the Platform provider and runner pool routes", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ items: [] }), { status: 200 }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ items: [] }), { status: 200 }),
      );
    globalThis.fetch = fetchMock;
    const api = new AdminApi(() => "admin-token");
    await api.listPlatformProviders();
    await api.listRunnerPools();
    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/admin/v1/platform/model-providers",
      expect.anything(),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/admin/v1/platform/runner-pools",
      expect.anything(),
    );
  });

  it("uses the scope-aware Platform settings routes with ETag and idempotency", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ items: [] }), { status: 200 }),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            scope: {},
            values: {},
            etag: "3",
            validation_state: "valid",
          }),
          { status: 200 },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            state: "valid",
            findings: [],
            semantic_diff: [],
            required_capabilities: [],
          }),
          { status: 200 },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            scope: {},
            values: {},
            etag: "4",
            validation_state: "valid",
          }),
          { status: 200 },
        ),
      );
    globalThis.fetch = fetchMock;
    const api = new AdminApi(() => "admin-token");
    await api.listLimitPolicies("ws_1");
    await api.getPlatformSettings("workspace", "ws_1");
    await api.validatePlatformSettings({
      workspace_id: "ws_1",
      values: { retention: { audit_days: 30 } },
    });
    await api.applyPlatformSettings(
      "3",
      { workspace_id: "ws_1", values: { retention: { audit_days: 30 } } },
      "settings-1",
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/admin/v1/platform/limit-policies?workspace_id=ws_1",
      expect.anything(),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/admin/v1/platform/settings?scope=workspace&workspace_id=ws_1",
      expect.anything(),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      4,
      "/api/admin/v1/platform/settings:apply",
      expect.objectContaining({
        headers: expect.objectContaining({
          "If-Match": '"3"',
          "Idempotency-Key": "settings-1",
        }),
      }),
    );
  });

  it("uses the Evaluation Gate and Regression routes", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ items: [] }), { status: 200 }),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({ comparison_state: "pending", items: [] }),
          { status: 200 },
        ),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ id: "egate_1", state: "overridden" }), {
          status: 200,
        }),
      );
    globalThis.fetch = fetchMock;
    const api = new AdminApi(() => "admin-token");

    await api.listEvaluationGates({
      workspaceId: "ws_1",
      agentRevisionHash: "sha256:revision",
    });
    await api.listEvaluationRunRegressions("erun_1");
    await api.overrideEvaluationGate("egate_1", {
      reason: "Incident mitigation",
      expires_at: "2026-08-17T00:00:00Z",
    });

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/admin/v1/evaluation-gates?workspace_id=ws_1&agent_revision_hash=sha256%3Arevision",
      expect.anything(),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/admin/v1/evaluation-runs/erun_1/regressions",
      expect.anything(),
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      3,
      "/api/admin/v1/evaluation-gates/egate_1:override",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({
          reason: "Incident mitigation",
          expires_at: "2026-08-17T00:00:00Z",
        }),
      }),
    );
  });

  it("encodes Evaluation Suite search and state filters", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(
        new Response(
          JSON.stringify({ items: [], page_info: { next_cursor: null } }),
          { status: 200 },
        ),
      );
    globalThis.fetch = fetchMock;
    const api = new AdminApi(() => "admin-token");
    await api.listEvaluationSuites({
      workspaceId: "wsp_development",
      state: "published",
      search: "smoke suite",
    });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/admin/v1/evaluation-suites?workspace_id=wsp_development&state=published&search=smoke+suite",
      expect.anything(),
    );
  });
});
