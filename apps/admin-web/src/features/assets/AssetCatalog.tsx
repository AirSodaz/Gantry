import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Archive,
  Cable,
  CircleSlash,
  Database,
  Package,
  Plus,
  RotateCcw,
} from "lucide-react";
import { Link } from "react-router-dom";
import {
  Button,
  Select,
  type SelectOption,
  StatusMark,
} from "@gantry/design-system";
import { useAdminApi } from "../../api/ApiProvider";
import type { Skill, Plugin, Tool } from "../../api/types";
import { ErrorState, LoadingState } from "../../components/AsyncState";

export type AssetKind = "skills" | "plugins" | "tools";
export type AssetItem = Skill | Plugin | Tool;
export type AssetAction = "activate" | "deprecate" | "retire";

const headings: Record<
  AssetKind,
  { title: string; description: string; icon: typeof Package }
> = {
  skills: {
    title: "Skills",
    description: "Imported workspace artifacts pinned by Agent revisions.",
    icon: Package,
  },
  plugins: {
    title: "Plugins",
    description:
      "Organization package versions available for workspace enablement.",
    icon: Database,
  },
  tools: {
    title: "Tools",
    description:
      "Registered descriptor versions available to governed bindings.",
    icon: Cable,
  },
};

export function AssetCatalog({ kind }: { kind: AssetKind }) {
  const api = useAdminApi();
  const queryClient = useQueryClient();
  const [workspaceID, setWorkspaceID] = useState("");
  const [search, setSearch] = useState("");
  const [status, setStatus] = useState("");
  const [isAdding, setIsAdding] = useState(false);
  const [form, setForm] = useState<Record<string, string>>({});
  const meta = headings[kind];

  const workspaces = useQuery({
    queryKey: ["admin-workspaces"],
    queryFn: () => api.listWorkspaces(),
    enabled: kind !== "tools",
  });
  const query = useQuery<{ items: AssetItem[] }>({
    queryKey: ["admin-assets", kind, workspaceID, search, status],
    queryFn: async () => {
      const options = { workspaceId: workspaceID, search, status };
      if (kind === "skills")
        return api.listSkills(options) as Promise<{ items: AssetItem[] }>;
      if (kind === "plugins")
        return api.listPlugins(options) as Promise<{ items: AssetItem[] }>;
      return api.listTools(options) as Promise<{ items: AssetItem[] }>;
    },
  });

  const mutation = useMutation<AssetItem, Error, void>({
    mutationFn: async () => {
      const parseObject = (value: string | undefined, label: string) => {
        if (!value?.trim()) return {};
        const parsed = JSON.parse(value);
        if (!parsed || typeof parsed !== "object" || Array.isArray(parsed))
          throw new Error(`${label} must be an object.`);
        return parsed as Record<string, unknown>;
      };
      if (kind === "skills") {
        return api.registerSkill({
          workspace_id: form.workspace_id ?? workspaceID,
          slug: form.slug ?? "",
          display_name: form.display_name ?? "",
          description: form.description ?? "",
          source_type: (form.source_type ?? "locator") as Skill["source_type"],
          source_ref: form.source_ref ?? "",
          declared_version: form.declared_version ?? "",
          content_digest: form.content_digest ?? "",
          metadata_json: parseObject(form.metadata_json, "Artifact metadata"),
        });
      }
      if (kind === "plugins") {
        return api.registerPlugin({
          slug: form.slug ?? "",
          display_name: form.display_name ?? "",
          description: form.description ?? "",
          version: form.version ?? "",
          content_digest: form.content_digest ?? "",
          manifest_json: parseObject(form.manifest_json, "Plugin manifest"),
        });
      }
      let schema: Record<string, unknown> = {};
      if (form.schema_json?.trim()) {
        const parsed = JSON.parse(form.schema_json);
        if (!parsed || typeof parsed !== "object" || Array.isArray(parsed))
          throw new Error("Schema JSON must be an object.");
        schema = parsed as Record<string, unknown>;
      }
      return api.registerTool({
        server_name: form.server_name ?? "",
        server_type: (form.server_type ?? "builtin") as Tool["server_type"],
        endpoint_ref: form.endpoint_ref ?? "",
        fully_qualified_name: form.fully_qualified_name ?? "",
        version: form.version ?? "",
        effect: (form.effect ?? "read") as Tool["effect"],
        idempotency: (form.idempotency ?? "read_only") as Tool["idempotency"],
        content_digest: form.content_digest ?? "",
        schema_json: schema,
      });
    },
    onSuccess: () => {
      setForm({});
      setIsAdding(false);
      void queryClient.invalidateQueries({ queryKey: ["admin-assets", kind] });
    },
  });

  const statusMutation = useMutation<
    void,
    Error,
    { id: string; action: AssetAction }
  >({
    mutationFn: ({ id, action }) => {
      if (kind === "skills") {
        if (action === "activate") return api.activateSkill(id);
        if (action === "deprecate") return api.deprecateSkill(id);
        return api.retireSkill(id);
      }
      if (kind === "plugins") {
        if (action === "activate") return api.activatePlugin(id);
        if (action === "deprecate") return api.deprecatePlugin(id);
        return api.retirePlugin(id);
      }
      if (action === "activate") return api.activateTool(id);
      if (action === "deprecate") return api.deprecateTool(id);
      return api.retireTool(id);
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["admin-assets", kind] });
    },
  });

  const workspaceOptions = useMemo<SelectOption[]>(
    () =>
      (workspaces.data?.items ?? []).map((workspace) => ({
        value: workspace.id,
        label: workspace.display_name,
      })),
    [workspaces.data?.items],
  );

  const statusOptions = useMemo<SelectOption[]>(() => {
    const values =
      kind === "skills"
        ? [
            ["available", "Available"],
            ["deprecated", "Deprecated"],
            ["retired", "Retired"],
          ]
        : kind === "plugins"
          ? [
              ["active", "Active"],
              ["deprecated", "Deprecated"],
              ["retired", "Retired"],
            ]
          : [
              ["active", "Active"],
              ["proposed", "Proposed"],
              ["deprecated", "Deprecated"],
              ["retired", "Retired"],
            ];
    return values.map(([value, label]) => ({ value, label }));
  }, [kind]);

  const items = query.data?.items ?? [];
  const Icon = meta.icon;

  if (query.isLoading || (kind !== "tools" && workspaces.isLoading))
    return <LoadingState label={`Loading ${meta.title.toLowerCase()}`} />;
  if (query.error || (kind !== "tools" && workspaces.error))
    return (
      <div className="admin-page">
        <ErrorState
          message={`The ${meta.title.toLowerCase()} catalog could not be loaded.`}
        />
      </div>
    );

  return (
    <section className="admin-page">
      <header className="admin-page-heading">
        <div>
          <h1>{meta.title}</h1>
          <p>{meta.description}</p>
        </div>
        <Button onClick={() => setIsAdding((value) => !value)}>
          <Plus size={16} />{" "}
          {isAdding
            ? "Close"
            : `Register ${kind === "skills" ? "skill" : kind === "plugins" ? "plugin" : "tool"}`}
        </Button>
      </header>

      <div className="admin-filter-bar admin-asset-filter-bar">
        <label className="admin-filter-input">
          <span className="admin-field-label">Search</span>
          <input
            className="ds-input"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder={`Search ${meta.title.toLowerCase()}`}
          />
        </label>
        {kind !== "tools" ? (
          <Select
            label="Workspace"
            options={workspaceOptions}
            value={workspaceID}
            onChange={setWorkspaceID}
            placeholder="All manageable workspaces"
          />
        ) : null}
        <Select
          label="Status"
          options={statusOptions}
          value={status}
          onChange={setStatus}
          placeholder="All statuses"
        />
      </div>

      {isAdding ? (
        <AssetForm
          kind={kind}
          form={form}
          setForm={setForm}
          workspaceOptions={workspaceOptions}
          onSubmit={() => mutation.mutate()}
          busy={mutation.isPending}
          error={mutation.error?.message}
        />
      ) : null}

      {items.length === 0 ? (
        <div className="admin-empty">
          <Icon size={22} />
          <strong>No {kind} registered</strong>
          <span>
            Register an immutable catalog entry to use it in an Agent draft.
          </span>
        </div>
      ) : (
        <div className="admin-agent-list">
          {items.map((item) => {
            const name =
              "display_name" in item
                ? item.display_name
                : item.fully_qualified_name;
            const version =
              "version" in item
                ? item.version
                : "declared_version" in item
                  ? item.declared_version || "Undeclared"
                  : "";
            return (
              <div className="admin-agent-row" key={item.id}>
                <span className="admin-agent-icon">
                  <Icon size={17} />
                </span>
                <Link className="admin-agent-copy" to={`/${kind}/${item.id}`}>
                  <strong>{name}</strong>
                  <span>
                    {version} · {item.content_digest.slice(0, 18)}...
                  </span>
                </Link>
                <StatusMark status={item.status} />
                <AssetActions
                  status={item.status}
                  busy={statusMutation.isPending}
                  onAction={(action) =>
                    statusMutation.mutate({ id: item.id, action })
                  }
                />
              </div>
            );
          })}
        </div>
      )}
      {statusMutation.error ? (
        <p className="admin-error" role="alert">
          {statusMutation.error.message}
        </p>
      ) : null}
    </section>
  );
}

function AssetActions({
  status,
  busy,
  onAction,
}: {
  status: string;
  busy: boolean;
  onAction: (action: AssetAction) => void;
}) {
  const actions: AssetAction[] =
    status === "retired"
      ? []
      : status === "proposed"
        ? ["activate"]
        : status === "deprecated"
          ? ["activate", "retire"]
          : ["deprecate", "retire"];
  if (actions.length === 0) return null;
  const labels: Record<AssetAction, string> = {
    activate: "Activate",
    deprecate: "Deprecate",
    retire: "Retire",
  };
  const icons: Record<AssetAction, typeof RotateCcw> = {
    activate: RotateCcw,
    deprecate: CircleSlash,
    retire: Archive,
  };
  return (
    <span className="admin-asset-actions">
      {actions.map((action) => {
        const ActionIcon = icons[action];
        return (
          <Button
            key={action}
            type="button"
            size="sm"
            variant={action === "retire" ? "danger" : "quiet"}
            disabled={busy}
            onClick={() => onAction(action)}
            title={labels[action]}
          >
            <ActionIcon size={14} />
            {labels[action]}
          </Button>
        );
      })}
    </span>
  );
}

function AssetForm({
  kind,
  form,
  setForm,
  workspaceOptions,
  onSubmit,
  busy,
  error,
}: {
  kind: AssetKind;
  form: Record<string, string>;
  setForm: (value: Record<string, string>) => void;
  workspaceOptions: SelectOption[];
  onSubmit: () => void;
  busy: boolean;
  error?: string;
}) {
  const update = (key: string, value: string) =>
    setForm({ ...form, [key]: value });
  const fields =
    kind === "skills"
      ? [
          ["slug", "Slug"],
          ["display_name", "Display name"],
          ["description", "Description"],
          ["source_ref", "Source reference"],
          ["declared_version", "Declared version"],
          ["content_digest", "Content digest"],
        ]
      : kind === "plugins"
        ? [
            ["slug", "Slug"],
            ["display_name", "Display name"],
            ["description", "Description"],
            ["version", "Version"],
            ["content_digest", "Content digest"],
          ]
        : [
            ["server_name", "Server name"],
            ["fully_qualified_name", "Fully qualified tool name"],
            ["version", "Version"],
            ["endpoint_ref", "Endpoint reference"],
            ["content_digest", "Content digest"],
          ];

  return (
    <form
      className="admin-asset-form"
      onSubmit={(event) => {
        event.preventDefault();
        onSubmit();
      }}
    >
      <div className="admin-form-grid">
        {kind === "skills" ? (
          <label className="admin-field">
            <span className="admin-field-label">Workspace</span>
            <select
              className="ds-input"
              value={form.workspace_id ?? ""}
              onChange={(event) => update("workspace_id", event.target.value)}
              required
            >
              <option value="">Choose a workspace</option>
              {workspaceOptions.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </label>
        ) : null}
        {fields.map(([key, label]) => (
          <label className="admin-field" key={key}>
            <span className="admin-field-label">{label}</span>
            <input
              className="ds-input"
              value={form[key] ?? ""}
              onChange={(event) => update(key, event.target.value)}
              required={key !== "description"}
            />
          </label>
        ))}
      </div>
      {error ? (
        <p className="admin-error" role="alert">
          {error}
        </p>
      ) : null}
      <div className="admin-form-actions">
        <Button type="submit" disabled={busy}>
          {busy ? "Registering…" : "Register"}
        </Button>
      </div>
    </form>
  );
}
