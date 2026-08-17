import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Activity,
  AlertCircle,
  Building,
  CheckCircle2,
  Code2,
  Cpu,
  Layers,
  Save,
  Shield,
  ShieldCheck,
  Sliders,
  Sparkles,
} from "lucide-react";
import {
  Badge,
  Button,
  EmptyState,
  formatBytes,
  Select,
  type SelectOption,
  Tabs,
} from "@gantry/design-system";
import { useAdminApi } from "../../api/ApiProvider";
import { ErrorState, LoadingState } from "../../components/AsyncState";
import "./PlatformPages.css";

type Scope = "organization" | "workspace";

const SCOPE_OPTIONS: SelectOption[] = [
  {
    value: "organization",
    label: "Organization (Global Defaults)",
    icon: <Building size={14} />,
  },
  {
    value: "workspace",
    label: "Workspace Override",
    icon: <Layers size={14} />,
  },
];

export function PlatformSettingsPage() {
  const api = useAdminApi();
  const qc = useQueryClient();
  const [scope, setScope] = useState<Scope>("organization");
  const [workspaceID, setWorkspaceID] = useState("");
  const [activeTab, setActiveTab] = useState<"policies" | "studio">("policies");
  const [draft, setDraft] = useState("{}");
  const [jsonError, setJsonError] = useState("");

  const workspaces = useQuery({
    queryKey: ["admin-workspaces"],
    queryFn: () => api.listWorkspaces(),
  });

  const activeWorkspace =
    scope === "workspace" ? workspaceID.trim() : undefined;
  const settings = useQuery({
    queryKey: ["admin-platform-settings", scope, activeWorkspace],
    queryFn: () => api.getPlatformSettings(scope, activeWorkspace),
    enabled: scope === "organization" || Boolean(activeWorkspace),
  });

  const values = useMemo(
    () => settings.data?.values ?? {},
    [settings.data?.values],
  );

  useEffect(() => {
    if (settings.data?.values) {
      setDraft(JSON.stringify(settings.data.values, null, 2));
    }
  }, [settings.data?.values]);

  const validate = useMutation({
    mutationFn: () => {
      let parsed: Record<string, unknown>;
      try {
        parsed = JSON.parse(draft) as Record<string, unknown>;
      } catch (err) {
        throw new Error(
          err instanceof Error
            ? `Invalid JSON: ${err.message}`
            : "Invalid JSON format",
        );
      }
      return api.validatePlatformSettings({
        workspace_id: activeWorkspace,
        values: parsed,
      });
    },
  });

  const apply = useMutation({
    mutationFn: () => {
      let parsed: Record<string, unknown>;
      try {
        parsed = JSON.parse(draft) as Record<string, unknown>;
      } catch (err) {
        throw new Error(
          err instanceof Error
            ? `Invalid JSON: ${err.message}`
            : "Invalid JSON format",
        );
      }
      return api.applyPlatformSettings(
        settings.data?.etag ?? "",
        { workspace_id: activeWorkspace, values: parsed },
        crypto.randomUUID(),
      );
    },
    onSuccess: () => {
      setJsonError("");
      void qc.invalidateQueries({ queryKey: ["admin-platform-settings"] });
    },
  });

  const workspaceOptions = useMemo<SelectOption[]>(() => {
    return [
      { value: "", label: "Select workspace override" },
      ...(workspaces.data?.items ?? []).map((w) => ({
        value: w.id,
        label: w.display_name,
      })),
    ];
  }, [workspaces.data?.items]);

  if (settings.isLoading || workspaces.isLoading)
    return <LoadingState label="Loading platform settings" />;
  if (settings.error || workspaces.error) {
    return (
      <div className="admin-page">
        <ErrorState
          message="Platform settings could not be loaded."
          onRetry={() => {
            void settings.refetch();
            void workspaces.refetch();
          }}
        />
      </div>
    );
  }

  const limitPolicies = Array.isArray(values.limit_policies)
    ? (values.limit_policies as Array<Record<string, unknown>>)
    : [];
  const environments = Array.isArray(values.environment_profiles)
    ? (values.environment_profiles as Array<Record<string, unknown>>)
    : [];
  const classifications = Array.isArray(values.data_classifications)
    ? (values.data_classifications as Array<Record<string, unknown>>)
    : [];

  const isValid = settings.data?.validation_state === "valid";

  const settingTabs = [
    {
      id: "policies",
      label: "Governed Profiles & Limits",
      icon: <Sliders size={15} />,
    },
    { id: "studio", label: "Settings JSON Studio", icon: <Code2 size={15} /> },
  ];

  return (
    <div className="admin-page platform-page">
      {/* Header */}
      <header className="platform-header">
        <div className="platform-header-title">
          <h1>Platform Settings & Policy Posture</h1>
          <p>
            Configure scope-aware runtime guardrails, publication postures,
            environment limits, and JSON configuration.
          </p>
        </div>
      </header>

      {/* Scope Selector Bar */}
      <div className="settings-scope-bar">
        <div className="settings-scope-selector">
          <div style={{ width: "280px" }}>
            <Select
              label="Target Governance Scope"
              options={SCOPE_OPTIONS}
              value={scope}
              onChange={(val) => {
                const nextScope = val as Scope;
                setScope(nextScope);
                if (nextScope === "organization") setWorkspaceID("");
              }}
            />
          </div>

          {scope === "workspace" ? (
            <div style={{ width: "260px" }}>
              <Select
                label="Workspace"
                options={workspaceOptions}
                value={workspaceID}
                onChange={setWorkspaceID}
                placeholder="Choose workspace"
              />
            </div>
          ) : null}
        </div>

        <div className="settings-scope-badges">
          <Badge
            size="md"
            variant={scope === "organization" ? "neutral" : "info"}
          >
            {scope === "organization" ? (
              <Building size={13} />
            ) : (
              <Layers size={13} />
            )}
            {scope === "organization"
              ? "Global Organization Scope"
              : `Workspace: ${workspaceID || "None"}`}
          </Badge>
          <Badge size="md" variant={isValid ? "success" : "warning"}>
            {isValid ? <CheckCircle2 size={13} /> : <AlertCircle size={13} />}
            {settings.data?.validation_state
              ? `Validation: ${settings.data.validation_state}`
              : "Unvalidated"}
          </Badge>
          <Badge size="md" variant="default">
            ETag: {settings.data?.etag ?? "1"}
          </Badge>
        </div>
      </div>

      {/* Tabs */}
      <Tabs
        tabs={settingTabs}
        activeId={activeTab}
        onChange={(id) => setActiveTab(id as typeof activeTab)}
      />

      {/* Tab: Governed Profiles & Limits */}
      {activeTab === "policies" && (
        <div className="platform-page">
          {/* Limit Policies */}
          <section className="settings-studio-card">
            <div className="settings-studio-header">
              <div
                style={{ display: "flex", alignItems: "center", gap: "8px" }}
              >
                <Cpu size={18} style={{ color: "var(--ds-brand-cyan)" }} />
                <h3 style={{ margin: 0, fontSize: "15px", fontWeight: 600 }}>
                  Execution Limits & Budgets
                </h3>
              </div>
              <Badge size="sm" variant="default">
                {limitPolicies.length} limit rules
              </Badge>
            </div>

            <div className="settings-studio-body">
              {limitPolicies.length === 0 ? (
                <EmptyState
                  icon={<Cpu size={22} />}
                  title="No custom limit policies"
                  description="Standard organization defaults apply: 12 turns, 128KB output, and 300s timeout."
                />
              ) : (
                <div className="admin-table-wrap">
                  <table className="admin-table">
                    <thead>
                      <tr>
                        <th>Scope Target</th>
                        <th>Max Concurrency</th>
                        <th>Max Duration</th>
                        <th>Max Output Bytes</th>
                        <th>ETag</th>
                      </tr>
                    </thead>
                    <tbody>
                      {limitPolicies.map((item) => (
                        <tr key={String(item.id)}>
                          <td>
                            <strong>
                              {item.workspace_id
                                ? `Workspace ${String(item.workspace_id)}`
                                : "Organization default"}
                            </strong>
                          </td>
                          <td>{String(item.concurrency)} concurrent runs</td>
                          <td>{String(item.duration_seconds)}s</td>
                          <td>{formatBytes(Number(item.output_bytes))}</td>
                          <td>
                            <code>{String(item.etag)}</code>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          </section>

          {/* Environment Profiles */}
          <section className="settings-studio-card">
            <div className="settings-studio-header">
              <div
                style={{ display: "flex", alignItems: "center", gap: "8px" }}
              >
                <ShieldCheck
                  size={18}
                  style={{ color: "var(--ds-brand-cyan)" }}
                />
                <h3 style={{ margin: 0, fontSize: "15px", fontWeight: 600 }}>
                  Environment Deployment Profiles
                </h3>
              </div>
              <Badge size="sm" variant="default">
                {environments.length} environments
              </Badge>
            </div>

            <div className="settings-studio-body">
              {environments.length === 0 ? (
                <EmptyState
                  icon={<Shield size={22} />}
                  title="Default environment configuration"
                  description="Three tiers active: Development (relaxed), Staging (pre-release validation), and Production (strict review gate)."
                />
              ) : (
                <div className="admin-table-wrap">
                  <table className="admin-table">
                    <thead>
                      <tr>
                        <th>Environment</th>
                        <th>Publication Posture</th>
                        <th>State</th>
                        <th>Scope Target</th>
                        <th>ETag</th>
                      </tr>
                    </thead>
                    <tbody>
                      {environments.map((item) => (
                        <tr key={String(item.id)}>
                          <td>
                            <strong>{String(item.name)}</strong>
                          </td>
                          <td>
                            <Badge
                              size="sm"
                              variant={
                                item.publication_posture === "strict_approval"
                                  ? "warning"
                                  : "info"
                              }
                            >
                              {String(item.publication_posture)}
                            </Badge>
                          </td>
                          <td>{String(item.state)}</td>
                          <td>
                            {item.workspace_id
                              ? String(item.workspace_id)
                              : "Organization bound"}
                          </td>
                          <td>
                            <code>{String(item.etag)}</code>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          </section>

          {/* Data Classifications */}
          <section className="settings-studio-card">
            <div className="settings-studio-header">
              <div
                style={{ display: "flex", alignItems: "center", gap: "8px" }}
              >
                <Activity size={18} style={{ color: "var(--ds-brand-cyan)" }} />
                <h3 style={{ margin: 0, fontSize: "15px", fontWeight: 600 }}>
                  Data Governance Classifications
                </h3>
              </div>
              <Badge size="sm" variant="default">
                {classifications.length} classifications
              </Badge>
            </div>

            <div className="settings-studio-body">
              {classifications.length === 0 ? (
                <p className="admin-muted">
                  Standard data governance handling classes configured.
                </p>
              ) : (
                <div className="platform-resource-grid">
                  {classifications.map((item) => (
                    <div
                      className="platform-resource-card"
                      key={String(item.id)}
                    >
                      <div className="platform-card-header">
                        <strong>{String(item.label)}</strong>
                        <Badge size="sm" variant="neutral">
                          {String(item.handling)}
                        </Badge>
                      </div>
                      <div className="platform-card-body">
                        <div className="platform-card-detail">
                          <span>Retention Policy</span>
                          <code>{String(item.retention_class)}</code>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </section>
        </div>
      )}

      {/* Tab: Settings JSON Studio */}
      {activeTab === "studio" && (
        <section className="settings-studio-card">
          <div className="settings-studio-header">
            <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
              <Code2 size={18} style={{ color: "var(--ds-brand-cyan)" }} />
              <div>
                <h3 style={{ margin: 0, fontSize: "15px", fontWeight: 600 }}>
                  Settings Document JSON Studio
                </h3>
                <span style={{ fontSize: "12px", color: "var(--ds-text-dim)" }}>
                  Authoritative declarative configuration payload for{" "}
                  {scope === "organization" ? "Organization" : workspaceID}
                </span>
              </div>
            </div>
          </div>

          <div className="settings-studio-body">
            <label className="admin-field" style={{ margin: 0 }}>
              <textarea
                className="ds-input admin-code-input"
                rows={16}
                value={draft}
                onChange={(event) => {
                  setDraft(event.target.value);
                  setJsonError("");
                }}
                style={{
                  fontFamily: "var(--ds-font-mono)",
                  fontSize: "13px",
                  lineHeight: 1.6,
                }}
              />
            </label>

            {validate.data ? (
              <div
                style={{
                  padding: "12px 16px",
                  borderRadius: "var(--ds-radius)",
                  background:
                    validate.data.state === "valid"
                      ? "rgba(16, 163, 127, 0.1)"
                      : "rgba(239, 68, 68, 0.1)",
                  border: `1px solid ${validate.data.state === "valid" ? "rgba(16, 163, 127, 0.3)" : "rgba(239, 68, 68, 0.3)"}`,
                  fontSize: "13px",
                }}
              >
                <strong>
                  Validation Status: {validate.data.state.toUpperCase()}
                </strong>
                {validate.data.findings?.length ? (
                  <ul style={{ margin: "8px 0 0", paddingLeft: "20px" }}>
                    {validate.data.findings.map((f, i) => (
                      <li key={i}>{JSON.stringify(f)}</li>
                    ))}
                  </ul>
                ) : (
                  <p
                    style={{ margin: "4px 0 0", color: "var(--ds-text-muted)" }}
                  >
                    No syntax or semantic validation findings.
                  </p>
                )}
              </div>
            ) : null}

            {jsonError || validate.error || apply.error ? (
              <p className="admin-error" role="alert">
                {jsonError ||
                  (validate.error ?? apply.error)?.message ||
                  "An error occurred."}
              </p>
            ) : null}
          </div>

          <footer className="settings-studio-footer">
            <Button
              variant="secondary"
              disabled={validate.isPending}
              onClick={() => {
                setJsonError("");
                validate.mutate();
              }}
            >
              <Sparkles size={15} />{" "}
              {validate.isPending ? "Validating…" : "Validate Syntax"}
            </Button>
            <Button
              disabled={apply.isPending || !settings.data?.etag}
              onClick={() => {
                setJsonError("");
                apply.mutate();
              }}
            >
              <Save size={15} />{" "}
              {apply.isPending ? "Applying…" : "Apply Settings Payload"}
            </Button>
          </footer>
        </section>
      )}
    </div>
  );
}
