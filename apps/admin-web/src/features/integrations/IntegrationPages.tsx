import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useParams, useSearchParams } from "react-router-dom";
import { ArrowLeft, Ban, Plus, XCircle } from "lucide-react";
import {
  Button,
  Select,
  type SelectOption,
  StatusMark,
  shortHash,
} from "@gantry/design-system";
import { useAdminApi } from "../../api/ApiProvider";
import { ErrorState, LoadingState } from "../../components/AsyncState";
import "./IntegrationPages.css";

const INTEGRATION_STATE_OPTIONS: SelectOption[] = [
  { value: "active", label: "Active" },
  { value: "disabled", label: "Disabled" },
  { value: "retired", label: "Retired" },
];
const INTEGRATION_ENVIRONMENT_OPTIONS: SelectOption[] = [
  { value: "development", label: "Development" },
  { value: "staging", label: "Staging" },
  { value: "production", label: "Production" },
];

export function IntegrationsPage() {
  const api = useAdminApi();
  const qc = useQueryClient();
  const [searchParams, setSearchParams] = useSearchParams();
  const [slug, setSlug] = useState("");
  const [name, setName] = useState("");

  const search = searchParams.get("search") ?? "";
  const state = searchParams.get("state") ?? "";
  const environment = searchParams.get("environment") ?? "";

  const list = useQuery({
    queryKey: ["admin-integrations", state, search, environment],
    queryFn: () =>
      api.listIntegrations({
        state: (state as "active" | "disabled" | "retired") || undefined,
        search: search || undefined,
        environment:
          (environment as "development" | "staging" | "production") ||
          undefined,
      }),
  });

  const updateFilter = (key: string, value: string) => {
    const next = new URLSearchParams(searchParams);
    if (value) next.set(key, value);
    else next.delete(key);
    setSearchParams(next);
  };

  const create = useMutation({
    mutationFn: () => api.createIntegration({ slug, display_name: name }),
    onSuccess: () => {
      setSlug("");
      setName("");
      void qc.invalidateQueries({ queryKey: ["admin-integrations"] });
    },
  });

  if (list.isLoading) return <LoadingState label="Loading integrations" />;
  if (list.error)
    return <ErrorState message="Integrations could not be loaded." />;

  return (
    <div className="admin-page integration-page">
      <div className="admin-page-header">
        <div>
          <h1>Integrations</h1>
          <p>
            Registered enterprise clients, exact Agent publications, and webhook
            endpoints.
          </p>
        </div>
      </div>

      <div className="integration-toolbar">
        <label className="admin-field integration-search">
          <span className="admin-field-label">Search</span>
          <input
            className="ds-input"
            value={search}
            onChange={(e) => updateFilter("search", e.target.value)}
            placeholder="Name or slug"
          />
        </label>
        <Select
          label="State"
          options={INTEGRATION_STATE_OPTIONS}
          value={state}
          onChange={(value) => updateFilter("state", value)}
          placeholder="All states"
        />
        <Select
          label="Environment"
          options={INTEGRATION_ENVIRONMENT_OPTIONS}
          value={environment}
          onChange={(value) => updateFilter("environment", value)}
          placeholder="All environments"
        />
      </div>

      <form
        className="integration-create-form"
        onSubmit={(e) => {
          e.preventDefault();
          create.mutate();
        }}
      >
        <label className="admin-field">
          <span className="admin-field-label">Display name</span>
          <input
            className="ds-input"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
          />
        </label>
        <label className="admin-field">
          <span className="admin-field-label">Slug</span>
          <input
            className="ds-input"
            value={slug}
            onChange={(e) => setSlug(e.target.value)}
            required
          />
        </label>
        <Button
          type="submit"
          isLoading={create.isPending}
          disabled={!name || !slug || create.isPending}
        >
          <Plus size={15} /> Create integration
        </Button>
      </form>
      {create.error ? (
        <p className="admin-error" role="alert">
          {create.error.message}
        </p>
      ) : null}

      <div className="admin-table-wrap">
        <table className="admin-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Slug</th>
              <th>State</th>
            </tr>
          </thead>
          <tbody>
            {(list.data?.items ?? []).map((item) => (
              <tr key={item.id}>
                <td>
                  <Link
                    className="admin-inline-link"
                    to={`/integrations/${item.id}`}
                  >
                    {item.display_name}
                  </Link>
                </td>
                <td>{item.slug}</td>
                <td>
                  <StatusMark status={item.state} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

export function IntegrationDetailPage() {
  const api = useAdminApi();
  const { integrationId = "" } = useParams();
  const [clientEnvironment, setClientEnvironment] = useState<
    "development" | "staging" | "production"
  >("development");
  const [clientAuthModes, setClientAuthModes] = useState("application");
  const [clientAudience, setClientAudience] = useState("");
  const [clientFingerprint, setClientFingerprint] = useState("");

  const [publication, setPublication] = useState({
    clientId: "",
    workspaceId: "",
    environment: "development" as "development" | "staging" | "production",
    revisionHash: "",
    inputDigest: "",
    outputDigest: "",
    authorityModes: "application",
  });

  const [webhook, setWebhook] = useState({
    environment: "development" as "development" | "staging" | "production",
    destination: "",
    signingKeyFingerprint: "",
    subscribedEvents: "run.completed",
  });

  const integration = useQuery({
    queryKey: ["admin-integration", integrationId],
    queryFn: () => api.getIntegration(integrationId),
    enabled: Boolean(integrationId),
  });
  const clients = useQuery({
    queryKey: ["admin-integration-clients", integrationId],
    queryFn: () => api.listIntegrationClients(integrationId),
    enabled: Boolean(integrationId),
  });
  const publications = useQuery({
    queryKey: ["admin-integration-publications", integrationId],
    queryFn: () => api.listIntegrationPublications(integrationId),
    enabled: Boolean(integrationId),
  });
  const webhooks = useQuery({
    queryKey: ["admin-integration-webhooks", integrationId],
    queryFn: () => api.listIntegrationWebhooks(integrationId),
    enabled: Boolean(integrationId),
  });

  const createClient = useMutation({
    mutationFn: () =>
      api.createIntegrationClient(integrationId, {
        environment: clientEnvironment,
        auth_modes: clientAuthModes
          .split(",")
          .map((item) => item.trim())
          .filter(Boolean),
        audience: clientAudience,
        credential_fingerprint: clientFingerprint,
      }),
    onSuccess: () => {
      setClientFingerprint("");
      setClientAudience("");
      void clients.refetch();
    },
  });

  const disableClient = useMutation({
    mutationFn: (id: string) => api.disableIntegrationClient(id),
    onSuccess: () => void clients.refetch(),
  });

  const createPublication = useMutation({
    mutationFn: () =>
      api.createIntegrationPublication(integrationId, {
        client_id: publication.clientId,
        workspace_id: publication.workspaceId,
        environment: publication.environment,
        revision_hash: publication.revisionHash,
        input_contract_digest: publication.inputDigest,
        output_contract_digest: publication.outputDigest,
        authority_modes: publication.authorityModes
          .split(",")
          .map((item) => item.trim())
          .filter(Boolean),
      }),
    onSuccess: () => void publications.refetch(),
  });

  const revokePublication = useMutation({
    mutationFn: (id: string) => api.revokeIntegrationPublication(id),
    onSuccess: () => void publications.refetch(),
  });

  const createWebhook = useMutation({
    mutationFn: () =>
      api.createIntegrationWebhook(integrationId, {
        environment: webhook.environment,
        destination: webhook.destination,
        signing_key_fingerprint: webhook.signingKeyFingerprint,
        subscribed_events: webhook.subscribedEvents
          .split(",")
          .map((item) => item.trim())
          .filter(Boolean),
      }),
    onSuccess: () => {
      setWebhook((item) => ({
        ...item,
        destination: "",
        signingKeyFingerprint: "",
      }));
      void webhooks.refetch();
    },
  });

  if (
    integration.isLoading ||
    clients.isLoading ||
    publications.isLoading ||
    webhooks.isLoading
  ) {
    return <LoadingState label="Loading integration" />;
  }
  if (integration.error)
    return <ErrorState message="The integration could not be loaded." />;
  const item = integration.data!;

  return (
    <div className="admin-page integration-page">
      <Link className="admin-inline-link" to="/integrations">
        <ArrowLeft size={14} /> Integrations
      </Link>
      <div className="admin-page-header">
        <div>
          <h1>{item.display_name}</h1>
          <p>
            {item.slug} · {item.state}
          </p>
        </div>
      </div>

      <section className="admin-detail-block">
        <h2>Clients</h2>
        <form
          className="integration-form"
          onSubmit={(event) => {
            event.preventDefault();
            createClient.mutate();
          }}
        >
          <Select
            label="Environment"
            options={INTEGRATION_ENVIRONMENT_OPTIONS}
            value={clientEnvironment}
            onChange={(value) =>
              setClientEnvironment(value as typeof clientEnvironment)
            }
          />
          <label className="admin-field">
            <span className="admin-field-label">Auth modes</span>
            <input
              className="ds-input"
              value={clientAuthModes}
              onChange={(event) => setClientAuthModes(event.target.value)}
              placeholder="application, delegated_user"
            />
          </label>
          <label className="admin-field">
            <span className="admin-field-label">Audience</span>
            <input
              className="ds-input"
              value={clientAudience}
              onChange={(event) => setClientAudience(event.target.value)}
              placeholder="Audience identifier"
            />
          </label>
          <label className="admin-field">
            <span className="admin-field-label">Credential fingerprint</span>
            <input
              className="ds-input"
              value={clientFingerprint}
              onChange={(event) => setClientFingerprint(event.target.value)}
              placeholder="Fingerprint"
              required
            />
          </label>
          <Button type="submit" isLoading={createClient.isPending}>
            Create client
          </Button>
        </form>
        {createClient.error ? (
          <p className="admin-error" role="alert">
            {createClient.error.message}
          </p>
        ) : null}

        <ul className="admin-detail-list" style={{ marginTop: "16px" }}>
          {(clients.data?.items ?? []).map((client) => (
            <li key={client.id}>
              <div>
                <strong>
                  {client.id} ({client.environment})
                </strong>
                <StatusMark status={client.status} />
                <span>
                  Fingerprint: {shortHash(client.credential_fingerprint)}
                </span>
              </div>
              <div className="admin-row-actions">
                <Button
                  size="sm"
                  variant="quiet"
                  onClick={() => disableClient.mutate(client.id)}
                  disabled={client.status === "disabled"}
                >
                  <Ban size={13} /> Disable
                </Button>
              </div>
            </li>
          ))}
        </ul>
      </section>

      <section className="admin-detail-block">
        <h2>Publications</h2>
        <form
          className="integration-form"
          onSubmit={(event) => {
            event.preventDefault();
            createPublication.mutate();
          }}
        >
          <label className="admin-field">
            <span className="admin-field-label">Client ID</span>
            <input
              className="ds-input"
              value={publication.clientId}
              onChange={(event) =>
                setPublication({ ...publication, clientId: event.target.value })
              }
              required
            />
          </label>
          <label className="admin-field">
            <span className="admin-field-label">Workspace ID</span>
            <input
              className="ds-input"
              value={publication.workspaceId}
              onChange={(event) =>
                setPublication({
                  ...publication,
                  workspaceId: event.target.value,
                })
              }
              required
            />
          </label>
          <Select
            label="Environment"
            options={INTEGRATION_ENVIRONMENT_OPTIONS}
            value={publication.environment}
            onChange={(value) =>
              setPublication({
                ...publication,
                environment: value as typeof publication.environment,
              })
            }
          />
          <label className="admin-field">
            <span className="admin-field-label">Revision hash</span>
            <input
              className="ds-input"
              value={publication.revisionHash}
              onChange={(event) =>
                setPublication({
                  ...publication,
                  revisionHash: event.target.value,
                })
              }
              placeholder="sha256:..."
              required
            />
          </label>
          <Button type="submit" isLoading={createPublication.isPending}>
            Publish
          </Button>
        </form>
        {createPublication.error ? (
          <p className="admin-error" role="alert">
            {createPublication.error.message}
          </p>
        ) : null}

        <ul className="admin-detail-list" style={{ marginTop: "16px" }}>
          {(publications.data?.items ?? []).map((pub) => (
            <li key={pub.id}>
              <div>
                <strong>
                  {pub.id} ({pub.environment})
                </strong>
                <span>
                  Revision: {shortHash(pub.revision_hash)} · Client:{" "}
                  {pub.client_id}
                </span>
              </div>
              <Button
                size="sm"
                variant="danger"
                onClick={() => revokePublication.mutate(pub.id)}
              >
                <XCircle size={13} /> Revoke
              </Button>
            </li>
          ))}
        </ul>
      </section>

      <section className="admin-detail-block">
        <h2>Webhooks</h2>
        <form
          className="integration-form"
          onSubmit={(event) => {
            event.preventDefault();
            createWebhook.mutate();
          }}
        >
          <Select
            label="Environment"
            options={INTEGRATION_ENVIRONMENT_OPTIONS}
            value={webhook.environment}
            onChange={(value) =>
              setWebhook({
                ...webhook,
                environment: value as typeof webhook.environment,
              })
            }
          />
          <label className="admin-field">
            <span className="admin-field-label">Destination</span>
            <input
              className="ds-input"
              type="url"
              value={webhook.destination}
              onChange={(event) =>
                setWebhook({ ...webhook, destination: event.target.value })
              }
              required
            />
          </label>
          <label className="admin-field">
            <span className="admin-field-label">Signing key fingerprint</span>
            <input
              className="ds-input"
              value={webhook.signingKeyFingerprint}
              onChange={(event) =>
                setWebhook({
                  ...webhook,
                  signingKeyFingerprint: event.target.value,
                })
              }
              required
            />
          </label>
          <Button type="submit" isLoading={createWebhook.isPending}>
            Register webhook
          </Button>
        </form>
        {createWebhook.error ? (
          <p className="admin-error" role="alert">
            {createWebhook.error.message}
          </p>
        ) : null}

        <ul className="admin-detail-list" style={{ marginTop: "16px" }}>
          {(webhooks.data?.items ?? []).map((wh) => (
            <li key={wh.id}>
              <div>
                <strong>
                  {wh.destination} ({wh.environment})
                </strong>
                <span>Events: {wh.subscribed_events.join(", ")}</span>
              </div>
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}
