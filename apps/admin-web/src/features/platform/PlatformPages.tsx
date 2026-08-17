import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useParams } from "react-router-dom";
import {
  ArrowLeft,
  Cpu,
  Lock,
  Plus,
  Radio,
  Server,
  Shield,
  ShieldAlert,
  ShieldCheck,
  Zap,
} from "lucide-react";
import {
  Badge,
  Button,
  EmptyState,
  Modal,
  Select,
  type SelectOption,
  shortHash,
  StatusMark,
  Tabs,
} from "@gantry/design-system";
import { useAdminApi } from "../../api/ApiProvider";
import type {
  DataClassification,
  ModelProvider,
  RunnerPool,
} from "../../api/types";
import { ErrorState, LoadingState } from "../../components/AsyncState";
import "./PlatformPages.css";
const POOL_TIER_OPTIONS: SelectOption[] = [
  {
    value: "development",
    label: "Development (Container Process)",
    icon: <Zap size={13} />,
  },
  {
    value: "gvisor",
    label: "gVisor (User-space Kernel Sandbox)",
    icon: <ShieldCheck size={13} />,
  },
  {
    value: "microvm",
    label: "MicroVM (Hardware-level Virtualization)",
    icon: <Server size={13} />,
  },
];

const HANDLING_OPTIONS: SelectOption[] = [
  { value: "internal", label: "Internal Only", icon: <Lock size={13} /> },
  { value: "confidential", label: "Confidential", icon: <Lock size={13} /> },
  {
    value: "restricted",
    label: "Restricted / Compliance",
    icon: <Lock size={13} />,
  },
  { value: "public", label: "Public", icon: <Radio size={13} /> },
];

const RETENTION_OPTIONS: SelectOption[] = [
  { value: "standard", label: "Standard (Indefinite until retired)" },
  { value: "30_days", label: "30-Day Automated Purge" },
  { value: "90_days", label: "90-Day Automated Purge" },
  { value: "audit_hold", label: "Audit Legal Hold" },
];

export function PlatformPage() {
  const api = useAdminApi();
  const qc = useQueryClient();
  const [activeTab, setActiveTab] = useState<
    "providers" | "pools" | "classifications"
  >("providers");

  // Modals
  const [showProviderModal, setShowProviderModal] = useState(false);
  const [showPoolModal, setShowPoolModal] = useState(false);
  const [showClassificationModal, setShowClassificationModal] = useState(false);

  // Provider Form State
  const [providerName, setProviderName] = useState("");
  const [credentialRef, setCredentialRef] = useState("");
  const [dataClasses, setDataClasses] = useState("internal");

  // Runner Pool Form State
  const [poolTier, setPoolTier] = useState<
    "development" | "gvisor" | "microvm"
  >("development");
  const [maxConcurrency, setMaxConcurrency] = useState(4);

  // Classification Form State
  const [classificationLabel, setClassificationLabel] = useState("");
  const [handling, setHandling] = useState<
    "public" | "internal" | "confidential" | "restricted"
  >("internal");
  const [retentionClass, setRetentionClass] = useState("standard");
  const providers = useQuery({
    queryKey: ["admin-platform-providers"],
    queryFn: () => api.listPlatformProviders(),
  });
  const pools = useQuery({
    queryKey: ["admin-platform-pools"],
    queryFn: () => api.listRunnerPools(),
  });
  const credentials = useQuery({
    queryKey: ["admin-platform-credentials"],
    queryFn: () => api.listPlatformCredentials(),
  });
  const classifications = useQuery({
    queryKey: ["admin-platform-classifications"],
    queryFn: () => api.listDataClassifications(),
  });

  const createProvider = useMutation({
    mutationFn: () =>
      api.createPlatformProvider({
        name: providerName,
        data_classes: dataClasses
          .split(",")
          .map((s) => s.trim())
          .filter(Boolean),
        credential_reference_id: credentialRef,
      }),
    onSuccess: () => {
      setProviderName("");
      setCredentialRef("");
      setShowProviderModal(false);
      void qc.invalidateQueries({ queryKey: ["admin-platform-providers"] });
    },
  });

  const createPool = useMutation({
    mutationFn: () =>
      api.createRunnerPool({
        isolation_tier: poolTier,
        compatible_protocols: ["gantry.runner/v1"],
        capacity: { max_concurrency: maxConcurrency },
      }),
    onSuccess: () => {
      setShowPoolModal(false);
      void qc.invalidateQueries({ queryKey: ["admin-platform-pools"] });
    },
  });

  const quarantine = useMutation({
    mutationFn: (id: string) => api.quarantineProvider(id),
    onSuccess: () =>
      void qc.invalidateQueries({ queryKey: ["admin-platform-providers"] }),
  });

  const drain = useMutation({
    mutationFn: (id: string) => api.drainRunnerPool(id),
    onSuccess: () =>
      void qc.invalidateQueries({ queryKey: ["admin-platform-pools"] }),
  });

  const createClassification = useMutation({
    mutationFn: () =>
      api.createDataClassification({
        label: classificationLabel,
        handling,
        retention_class: retentionClass,
        allowed_provider_ids: [],
        allowed_tool_classes: [],
      }),
    onSuccess: () => {
      setClassificationLabel("");
      setShowClassificationModal(false);
      void qc.invalidateQueries({
        queryKey: ["admin-platform-classifications"],
      });
    },
  });

  const providerList = providers.data?.items ?? [];
  const poolList = pools.data?.items ?? [];
  const classificationList = classifications.data?.items ?? [];

  const tabs = useMemo(
    () => [
      {
        id: "providers",
        label: "Model Providers",
        icon: <Cpu size={15} />,
        badge: providerList.length,
      },
      {
        id: "pools",
        label: "Runner Pools",
        icon: <Server size={15} />,
        badge: poolList.length,
      },
      {
        id: "classifications",
        label: "Data Classifications",
        icon: <Shield size={15} />,
        badge: classificationList.length,
      },
    ],
    [providerList.length, poolList.length, classificationList.length],
  );

  if (
    providers.isLoading ||
    pools.isLoading ||
    credentials.isLoading ||
    classifications.isLoading
  ) {
    return <LoadingState label="Loading platform infrastructure" />;
  }

  if (
    providers.error ||
    pools.error ||
    credentials.error ||
    classifications.error
  ) {
    return (
      <div className="admin-page">
        <ErrorState
          message="Platform resources could not be loaded."
          onRetry={() => {
            void providers.refetch();
            void pools.refetch();
            void credentials.refetch();
            void classifications.refetch();
          }}
        />
      </div>
    );
  }

  return (
    <div className="admin-page platform-page">
      {/* Header */}
      <header className="platform-header">
        <div className="platform-header-title">
          <h1>Platform Infrastructure</h1>
          <p>
            Manage upstream AI model providers, sandboxed execution runners, and
            data governance policies.
          </p>
        </div>
        <div className="platform-header-actions">
          {activeTab === "providers" && (
            <Button onClick={() => setShowProviderModal(true)}>
              <Plus size={15} /> Add Model Provider
            </Button>
          )}
          {activeTab === "pools" && (
            <Button onClick={() => setShowPoolModal(true)}>
              <Plus size={15} /> Add Runner Pool
            </Button>
          )}
          {activeTab === "classifications" && (
            <Button onClick={() => setShowClassificationModal(true)}>
              <Plus size={15} /> New Classification
            </Button>
          )}
        </div>
      </header>

      {/* Metrics Banner */}
      <div className="platform-metrics-grid">
        <div className="platform-metric-card">
          <div className="platform-metric-icon">
            <Cpu size={20} />
          </div>
          <div className="platform-metric-content">
            <span>Model Providers</span>
            <strong>{providerList.length}</strong>
            <small>
              {providerList.filter((p) => p.state === "active").length} active
              routes
            </small>
          </div>
        </div>

        <div className="platform-metric-card">
          <div className="platform-metric-icon">
            <Server size={20} />
          </div>
          <div className="platform-metric-content">
            <span>Runner Pools</span>
            <strong>{poolList.length}</strong>
            <small>
              {poolList.reduce(
                (acc, p) =>
                  acc +
                  (typeof p.capacity?.max_concurrency === "number"
                    ? p.capacity.max_concurrency
                    : 1),
                0,
              )}{" "}
              max concurrency
            </small>
          </div>
        </div>

        <div className="platform-metric-card">
          <div className="platform-metric-icon">
            <ShieldCheck size={20} />
          </div>
          <div className="platform-metric-content">
            <span>Data Classifications</span>
            <strong>{classificationList.length}</strong>
            <small>Governed data handling tiers</small>
          </div>
        </div>
      </div>

      {/* Tabs Navigation */}
      <Tabs
        tabs={tabs}
        activeId={activeTab}
        onChange={(id) => setActiveTab(id as typeof activeTab)}
      />

      {/* Tab: Model Providers */}
      {activeTab === "providers" && (
        <section aria-label="Model Providers">
          {providerList.length === 0 ? (
            <EmptyState
              icon={<Cpu size={24} />}
              title="No model providers configured"
              description="Register an upstream model provider (OpenAI, Anthropic, or custom endpoint) to route agent tasks."
              action={
                <Button size="sm" onClick={() => setShowProviderModal(true)}>
                  <Plus size={14} /> Add Model Provider
                </Button>
              }
            />
          ) : (
            <div className="platform-resource-grid">
              {providerList.map((provider) => (
                <ProviderCard
                  key={provider.id}
                  provider={provider}
                  onQuarantine={() => quarantine.mutate(provider.id)}
                  isQuarantining={quarantine.isPending}
                />
              ))}
            </div>
          )}
        </section>
      )}

      {/* Tab: Runner Pools */}
      {activeTab === "pools" && (
        <section aria-label="Runner Pools">
          {poolList.length === 0 ? (
            <EmptyState
              icon={<Server size={24} />}
              title="No runner pools configured"
              description="Create an isolated runner pool (gVisor, MicroVM, or local dev) to execute sandbox agent environments."
              action={
                <Button size="sm" onClick={() => setShowPoolModal(true)}>
                  <Plus size={14} /> Add Runner Pool
                </Button>
              }
            />
          ) : (
            <div className="platform-resource-grid">
              {poolList.map((pool) => (
                <RunnerPoolCard
                  key={pool.id}
                  pool={pool}
                  onDrain={() => drain.mutate(pool.id)}
                  isDraining={drain.isPending}
                />
              ))}
            </div>
          )}
        </section>
      )}

      {/* Tab: Data Classifications */}
      {activeTab === "classifications" && (
        <section aria-label="Data Classifications">
          {classificationList.length === 0 ? (
            <EmptyState
              icon={<Shield size={24} />}
              title="No data classifications defined"
              description="Define organization-wide data handling levels (e.g. Public, Internal, Confidential) and retention policies."
              action={
                <Button
                  size="sm"
                  onClick={() => setShowClassificationModal(true)}
                >
                  <Plus size={14} /> New Classification
                </Button>
              }
            />
          ) : (
            <div className="platform-resource-grid">
              {classificationList.map((dc) => (
                <DataClassificationCard key={dc.id} classification={dc} />
              ))}
            </div>
          )}
        </section>
      )}

      {/* Modal: Add Model Provider */}
      <Modal
        isOpen={showProviderModal}
        onClose={() => setShowProviderModal(false)}
        title="Add Model Provider"
        description="Register an upstream LLM API provider with secure credential references and allowed data handling tiers."
        maxWidth={500}
        footer={
          <>
            <Button
              variant="quiet"
              size="sm"
              onClick={() => setShowProviderModal(false)}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              isLoading={createProvider.isPending}
              disabled={
                !providerName.trim() ||
                !credentialRef.trim() ||
                createProvider.isPending
              }
              onClick={() => createProvider.mutate()}
            >
              Add Provider
            </Button>
          </>
        }
      >
        <form
          className="admin-form-body"
          onSubmit={(e) => {
            e.preventDefault();
            createProvider.mutate();
          }}
        >
          <label className="admin-field">
            <span className="admin-field-label">Provider Name</span>
            <input
              className="ds-input"
              value={providerName}
              onChange={(e) => setProviderName(e.target.value)}
              placeholder="e.g. openai-production, anthropic-eu, local-ollama"
              required
            />
          </label>

          <Select
            label="Credential Reference"
            options={(credentials.data?.items ?? [])
              .map((cred) => ({
                value: cred.id,
                label: `${cred.id} (${cred.target_service})`,
                icon: <Lock size={13} />,
              }))
              .concat(
                (credentials.data?.items ?? []).length === 0
                  ? [
                      {
                        value: "default-credential",
                        label: "default-credential (Development Broker)",
                        icon: <Lock size={13} />,
                      },
                    ]
                  : [],
              )}
            value={credentialRef}
            onChange={setCredentialRef}
            placeholder="Choose credential reference"
          />

          <label className="admin-field">
            <span className="admin-field-label">Allowed Data Classes</span>
            <input
              className="ds-input"
              value={dataClasses}
              onChange={(e) => setDataClasses(e.target.value)}
              placeholder="internal, confidential, public (comma-separated)"
            />
          </label>

          {createProvider.error ? (
            <p className="admin-error" role="alert">
              {createProvider.error instanceof Error
                ? createProvider.error.message
                : "Failed to register provider."}
            </p>
          ) : null}
        </form>
      </Modal>

      {/* Modal: Add Runner Pool */}
      <Modal
        isOpen={showPoolModal}
        onClose={() => setShowPoolModal(false)}
        title="Add Runner Pool"
        description="Provision a sandboxed execution pool for agent tasks."
        maxWidth={500}
        footer={
          <>
            <Button
              variant="quiet"
              size="sm"
              onClick={() => setShowPoolModal(false)}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              isLoading={createPool.isPending}
              disabled={createPool.isPending}
              onClick={() => createPool.mutate()}
            >
              Create Pool
            </Button>
          </>
        }
      >
        <form
          className="admin-form-body"
          onSubmit={(e) => {
            e.preventDefault();
            createPool.mutate();
          }}
        >
          <Select
            label="Isolation Tier"
            options={POOL_TIER_OPTIONS}
            value={poolTier}
            onChange={(val) => setPoolTier(val as typeof poolTier)}
          />

          <label className="admin-field">
            <span className="admin-field-label">Max Concurrency</span>
            <input
              className="ds-input"
              type="number"
              min={1}
              max={64}
              value={maxConcurrency}
              onChange={(e) => setMaxConcurrency(Number(e.target.value) || 1)}
              required
            />
          </label>

          {createPool.error ? (
            <p className="admin-error" role="alert">
              {createPool.error instanceof Error
                ? createPool.error.message
                : "Failed to create runner pool."}
            </p>
          ) : null}
        </form>
      </Modal>

      {/* Modal: Add Data Classification */}
      <Modal
        isOpen={showClassificationModal}
        onClose={() => setShowClassificationModal(false)}
        title="Create Data Classification"
        description="Establish data protection boundaries and retention classes."
        maxWidth={500}
        footer={
          <>
            <Button
              variant="quiet"
              size="sm"
              onClick={() => setShowClassificationModal(false)}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              isLoading={createClassification.isPending}
              disabled={
                !classificationLabel.trim() || createClassification.isPending
              }
              onClick={() => createClassification.mutate()}
            >
              Save Classification
            </Button>
          </>
        }
      >
        <form
          className="admin-form-body"
          onSubmit={(e) => {
            e.preventDefault();
            createClassification.mutate();
          }}
        >
          <label className="admin-field">
            <span className="admin-field-label">Classification Label</span>
            <input
              className="ds-input"
              value={classificationLabel}
              onChange={(e) => setClassificationLabel(e.target.value)}
              placeholder="e.g. Restricted, PII, Confidential"
              required
            />
          </label>

          <Select
            label="Handling Tier"
            options={HANDLING_OPTIONS}
            value={handling}
            onChange={(val) => setHandling(val as typeof handling)}
          />

          <Select
            label="Retention Class"
            options={RETENTION_OPTIONS}
            value={retentionClass}
            onChange={setRetentionClass}
          />

          {createClassification.error ? (
            <p className="admin-error" role="alert">
              {createClassification.error instanceof Error
                ? createClassification.error.message
                : "Failed to create data classification."}
            </p>
          ) : null}
        </form>
      </Modal>
    </div>
  );
}

function ProviderCard({
  provider,
  onQuarantine,
  isQuarantining,
}: {
  provider: ModelProvider;
  onQuarantine: () => void;
  isQuarantining: boolean;
}) {
  return (
    <div className="platform-resource-card">
      <div className="platform-card-header">
        <div className="platform-card-title-wrap">
          <div className="platform-card-icon">
            <Cpu size={16} />
          </div>
          <div>
            <strong>{provider.name}</strong>
            <div style={{ fontSize: "11px", color: "var(--ds-text-dim)" }}>
              ID: {provider.id}
            </div>
          </div>
        </div>
        <StatusMark status={provider.state} />
      </div>

      <div className="platform-card-body">
        <div className="platform-card-detail">
          <span>Credential reference</span>
          <code>
            {provider.credential_reference_id
              ? shortHash(provider.credential_reference_id)
              : "Unset"}
          </code>
        </div>
        <div className="platform-card-detail">
          <span>Data classes</span>
          <div className="platform-card-tags">
            {provider.data_classes.map((cls) => (
              <Badge key={cls} size="sm" variant="neutral">
                {cls}
              </Badge>
            ))}
          </div>
        </div>
      </div>

      <div className="platform-card-footer">
        <Link
          className="ds-button ds-button-quiet ds-button-sm"
          to={`/platform/providers/${provider.id}`}
        >
          <Radio size={13} /> Routes
        </Link>
        {provider.state !== "quarantined" ? (
          <Button
            size="sm"
            variant="danger"
            disabled={isQuarantining}
            onClick={onQuarantine}
          >
            <ShieldAlert size={13} /> Quarantine
          </Button>
        ) : (
          <Badge size="sm" variant="danger">
            Quarantined
          </Badge>
        )}
      </div>
    </div>
  );
}

function RunnerPoolCard({
  pool,
  onDrain,
  isDraining,
}: {
  pool: RunnerPool;
  onDrain: () => void;
  isDraining: boolean;
}) {
  const maxConc =
    typeof pool.capacity?.max_concurrency === "number"
      ? pool.capacity.max_concurrency
      : 1;
  const isDevelopment = pool.isolation_tier === "development";
  const isGvisor = pool.isolation_tier === "gvisor";

  return (
    <div className="platform-resource-card">
      <div className="platform-card-header">
        <div className="platform-card-title-wrap">
          <div className="platform-card-icon">
            <Server size={16} />
          </div>
          <div>
            <strong>{pool.id}</strong>
            <div style={{ fontSize: "11px", color: "var(--ds-text-dim)" }}>
              Protocol: gantry.runner/v1
            </div>
          </div>
        </div>
        <StatusMark status={pool.state} />
      </div>

      <div className="platform-card-body">
        <div className="platform-card-detail">
          <span>Isolation Tier</span>
          <Badge
            size="sm"
            variant={isDevelopment ? "warning" : isGvisor ? "success" : "info"}
          >
            <Zap size={11} /> {pool.isolation_tier.toUpperCase()}
          </Badge>
        </div>
        <div className="platform-card-detail">
          <span>Concurrency Capacity</span>
          <strong>{maxConc} concurrent tasks</strong>
        </div>
      </div>

      <div className="platform-card-footer">
        {pool.state !== "draining" && pool.state !== "disabled" ? (
          <Button
            size="sm"
            variant="quiet"
            disabled={isDraining}
            onClick={onDrain}
          >
            Drain Pool
          </Button>
        ) : (
          <Badge size="sm" variant="warning">
            {pool.state}
          </Badge>
        )}
      </div>
    </div>
  );
}

function DataClassificationCard({
  classification,
}: {
  classification: DataClassification;
}) {
  return (
    <div className="platform-resource-card">
      <div className="platform-card-header">
        <div className="platform-card-title-wrap">
          <div className="platform-card-icon">
            <ShieldCheck size={16} />
          </div>
          <div>
            <strong>{classification.label}</strong>
            <div style={{ fontSize: "11px", color: "var(--ds-text-dim)" }}>
              ID: {classification.id}
            </div>
          </div>
        </div>
        <Badge size="sm" variant="default">
          <Lock size={11} /> {classification.handling}
        </Badge>
      </div>

      <div className="platform-card-body">
        <div className="platform-card-detail">
          <span>Retention Class</span>
          <code>{classification.retention_class}</code>
        </div>
        <div className="platform-card-detail">
          <span>Allowed Providers</span>
          <span>
            {classification.allowed_provider_ids.length
              ? classification.allowed_provider_ids.join(", ")
              : "All authorized"}
          </span>
        </div>
      </div>
    </div>
  );
}

export function ProviderRoutesPage() {
  const api = useAdminApi();
  const { providerId = "" } = useParams();
  const routes = useQuery({
    queryKey: ["admin-provider-routes", providerId],
    queryFn: () => api.listProviderRoutes(providerId),
    enabled: Boolean(providerId),
  });

  if (routes.isLoading) return <LoadingState label="Loading provider routes" />;
  if (routes.error)
    return <ErrorState message="Provider routes could not be loaded." />;

  return (
    <div className="admin-page platform-page">
      <Link className="admin-back-link" to="/platform">
        <ArrowLeft size={14} /> Back to platform
      </Link>
      <div className="admin-page-header">
        <div>
          <h1>Provider Routes</h1>
          <p>
            Allowed models, routing policy, and live route states for provider{" "}
            {providerId}.
          </p>
        </div>
      </div>

      {routes.data?.items.length ? (
        <div className="admin-table-wrap">
          <table className="admin-table">
            <thead>
              <tr>
                <th>Route ID</th>
                <th>Allowed Models</th>
                <th>State</th>
                <th>ETag</th>
              </tr>
            </thead>
            <tbody>
              {routes.data.items.map((route) => (
                <tr key={route.id}>
                  <td>
                    <strong>{route.id}</strong>
                  </td>
                  <td>
                    <div className="platform-card-tags">
                      {route.allowed_models.map((m) => (
                        <Badge key={m} size="sm" variant="neutral">
                          {m}
                        </Badge>
                      ))}
                    </div>
                  </td>
                  <td>
                    <StatusMark status={route.state} />
                  </td>
                  <td>
                    <code>{route.etag}</code>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : (
        <EmptyState
          icon={<Radio size={24} />}
          title="No provider routes registered"
          description="No specific model routing restrictions configured for this provider. Standard model wildcards apply."
        />
      )}
    </div>
  );
}
