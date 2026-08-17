import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ArrowLeft, Unplug } from "lucide-react";
import { Link, useParams } from "react-router-dom";
import { Button, CodeBlock, StatusMark } from "@gantry/design-system";
import { useAdminApi } from "../../api/ApiProvider";
import type { AssetUsage, PluginDetail, Skill, Tool } from "../../api/types";
import { ErrorState, LoadingState } from "../../components/AsyncState";
import type { AssetKind } from "./AssetCatalog";

export function AssetDetailPage({ kind }: { kind: AssetKind }) {
  const api = useAdminApi();
  const queryClient = useQueryClient();
  const { assetId = "" } = useParams<{ assetId: string }>();

  const detail = useQuery<Skill | PluginDetail | Tool>({
    queryKey: ["admin-asset", kind, assetId],
    queryFn: () =>
      kind === "skills"
        ? api.getSkill(assetId)
        : kind === "plugins"
          ? api.getPlugin(assetId)
          : api.getTool(assetId),
    enabled: assetId !== "",
  });

  const usage = useQuery<{ items: AssetUsage[] }>({
    queryKey: ["admin-asset-usage", kind, assetId],
    queryFn: () =>
      kind === "skills"
        ? api.listSkillUsage(assetId)
        : kind === "plugins"
          ? api.listPluginUsage(assetId)
          : api.listToolUsage(assetId),
    enabled: assetId !== "" && detail.isSuccess,
  });

  const disablePlugin = useMutation({
    mutationFn: (workspaceID: string) =>
      api.disablePlugin(assetId, workspaceID),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ["admin-asset", kind, assetId],
      });
    },
  });

  const workspaces = useQuery({
    queryKey: ["admin-workspaces"],
    queryFn: () => api.listWorkspaces(),
    enabled: kind === "plugins",
  });

  const [enableWorkspaceID, setEnableWorkspaceID] = useState("");
  const enablePlugin = useMutation({
    mutationFn: () => api.enablePlugin(assetId, enableWorkspaceID),
    onSuccess: () => {
      setEnableWorkspaceID("");
      void queryClient.invalidateQueries({
        queryKey: ["admin-asset", kind, assetId],
      });
    },
  });

  const title =
    kind === "skills"
      ? "Skill artifact"
      : kind === "plugins"
        ? "Plugin version"
        : "Tool descriptor";
  if (detail.isLoading)
    return <LoadingState label={`Loading ${title.toLowerCase()}`} />;
  if (detail.error || !detail.data) {
    return (
      <div className="admin-page">
        <ErrorState
          message={`The ${title.toLowerCase()} could not be loaded.`}
        />
      </div>
    );
  }

  const item = detail.data;
  const name =
    "display_name" in item ? item.display_name : item.fully_qualified_name;
  const version =
    "declared_version" in item
      ? item.declared_version || "Undeclared"
      : "version" in item
        ? item.version
        : "";

  return (
    <section className="admin-page">
      <Link className="admin-back-link" to={`/${kind}`}>
        <ArrowLeft size={15} /> Back to {kind}
      </Link>
      <header className="admin-page-heading">
        <div>
          <h1>{name}</h1>
          <p>
            {title} · {version}
          </p>
        </div>
        <StatusMark status={item.status} />
      </header>

      <div className="admin-detail-grid">
        <section className="admin-detail-block">
          <h2>Identity</h2>
          <DetailField label="Catalog ID" value={item.id} />
          <DetailField label="Content digest" value={item.content_digest} />
          {"source_ref" in item ? (
            <>
              <DetailField label="Source type" value={item.source_type} />
              <DetailField label="Source reference" value={item.source_ref} />
            </>
          ) : null}
          {"server_name" in item ? (
            <>
              <DetailField
                label="Server"
                value={`${item.server_name} · ${item.server_type}`}
              />
              <DetailField
                label="Endpoint reference"
                value={item.endpoint_ref || "Not configured"}
              />
              <DetailField label="Effect" value={item.effect} />
              <DetailField label="Idempotency" value={item.idempotency} />
            </>
          ) : null}
        </section>

        {"workspaces" in item ? (
          <section className="admin-detail-block">
            <h2>Workspace enablement</h2>
            {item.workspaces.length === 0 ? (
              <p className="admin-muted">No workspace enablements.</p>
            ) : (
              <ul className="admin-detail-list">
                {item.workspaces.map((workspace) => (
                  <li key={workspace.id}>
                    <div className="admin-overview-highlight">
                      <strong>{workspace.display_name}</strong>
                      <Button
                        size="sm"
                        variant="quiet"
                        disabled={
                          disablePlugin.isPending || enablePlugin.isPending
                        }
                        title={`Disable in ${workspace.display_name}`}
                        onClick={() => disablePlugin.mutate(workspace.id)}
                      >
                        <Unplug size={14} /> Disable
                      </Button>
                    </div>
                    <span>{workspace.id}</span>
                  </li>
                ))}
              </ul>
            )}
            <div className="admin-form-row">
              <select
                className="ds-input"
                value={enableWorkspaceID}
                onChange={(event) => setEnableWorkspaceID(event.target.value)}
                disabled={enablePlugin.isPending}
              >
                <option value="">Choose a workspace to enable</option>
                {workspaces.data?.items
                  ?.filter((w) => !item.workspaces.some((ew) => ew.id === w.id))
                  .map((w) => (
                    <option key={w.id} value={w.id}>
                      {w.display_name}
                    </option>
                  ))}
              </select>
              <Button
                size="sm"
                variant="secondary"
                disabled={!enableWorkspaceID || enablePlugin.isPending}
                onClick={() => enablePlugin.mutate()}
              >
                {enablePlugin.isPending ? "Enabling…" : "Enable in workspace"}
              </Button>
            </div>
          </section>
        ) : null}

        {"schema_json" in item &&
        item.schema_json &&
        Object.keys(item.schema_json).length > 0 ? (
          <section className="admin-detail-block">
            <h2>Input and output schema</h2>
            <CodeBlock
              code={JSON.stringify(item.schema_json, null, 2)}
              language="json"
              maxHeight={300}
            />
          </section>
        ) : null}

        {"metadata_json" in item &&
        item.metadata_json &&
        Object.keys(item.metadata_json).length > 0 ? (
          <section className="admin-detail-block">
            <h2>Normalized artifact metadata</h2>
            <CodeBlock
              code={JSON.stringify(item.metadata_json, null, 2)}
              language="json"
              maxHeight={300}
            />
          </section>
        ) : null}

        {"manifest_json" in item &&
        item.manifest_json &&
        Object.keys(item.manifest_json).length > 0 ? (
          <section className="admin-detail-block">
            <h2>Contained assets and manifest</h2>
            <CodeBlock
              code={JSON.stringify(item.manifest_json, null, 2)}
              language="json"
              maxHeight={300}
            />
          </section>
        ) : null}

        <section className="admin-detail-block">
          <h2>Agent usage</h2>
          {usage.isLoading ? (
            <LoadingState label="Loading usage" />
          ) : usage.error ? (
            <ErrorState message="Usage could not be loaded." />
          ) : usage.data?.items.length ? (
            <ul className="admin-detail-list">
              {usage.data.items.map((entry) => (
                <li
                  key={`${entry.reference_kind}-${entry.agent_id}-${entry.reference_id}`}
                >
                  <strong>{entry.agent_name}</strong>
                  <span>
                    {entry.reference_kind} {entry.reference_id}
                    {entry.reference_hash
                      ? ` · ${entry.reference_hash}`
                      : ""} · {entry.workspace_id}
                  </span>
                </li>
              ))}
            </ul>
          ) : (
            <p className="admin-muted">
              No Agent draft or revision references this asset.
            </p>
          )}
        </section>
      </div>
    </section>
  );
}

function DetailField({ label, value }: { label: string; value: string }) {
  return (
    <div className="admin-detail-field">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}
