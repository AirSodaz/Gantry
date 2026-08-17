import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowLeft,
  Check,
  FileKey2,
  Plus,
  RotateCcw,
  Shield,
  SlidersHorizontal,
} from "lucide-react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  Button,
  EmptyState,
  formatDate,
  Select,
  type SelectOption,
} from "@gantry/design-system";
import { useAdminApi } from "../../api/ApiProvider";
import type { CreatePolicyInput, Policy, PolicyDraft } from "../../api/types";
import { ErrorState, LoadingState } from "../../components/AsyncState";

const types: SelectOption[] = [
  { value: "", label: "All policy types" },
  ...[
    "approval",
    "model",
    "tool",
    "command",
    "network",
    "credential",
    "data",
    "budget",
    "retention",
    "evaluation",
    "publication",
  ].map((value) => ({ value, label: value })),
];

export function PoliciesPage() {
  const api = useAdminApi();
  const navigate = useNavigate();
  const [type, setType] = useState("");
  const [showCreate, setShowCreate] = useState(false);
  const [createError, setCreateError] = useState("");

  const policies = useQuery({
    queryKey: ["admin-policies", type],
    queryFn: () => api.listPolicies({ type }),
  });
  const create = useMutation<
    { policy: Policy; draft: PolicyDraft },
    Error,
    CreatePolicyInput
  >({
    mutationFn: (input) => api.createPolicy(input),
    onSuccess: (item) => {
      void policies.refetch();
      navigate(`/policies/${item.policy.id}`);
    },
  });

  const [name, setName] = useState("");
  const [createType, setCreateType] = useState("approval");
  const [document, setDocument] = useState(
    '{"kind":"approval","rules":[],"default_effect":"deny"}',
  );

  if (policies.isLoading) return <LoadingState label="Loading policies" />;
  if (policies.error) {
    return (
      <div className="admin-page">
        <ErrorState message="The policy catalog could not be loaded." />
      </div>
    );
  }

  const handleCreateSubmit = (event: React.FormEvent) => {
    event.preventDefault();
    setCreateError("");
    let parsed: Record<string, unknown>;
    try {
      parsed = JSON.parse(document) as Record<string, unknown>;
    } catch (err) {
      setCreateError(
        err instanceof Error
          ? `Invalid JSON: ${err.message}`
          : "Invalid JSON format",
      );
      return;
    }
    create.mutate({
      type: createType as Policy["type"],
      name,
      document: parsed,
    });
  };

  return (
    <section className="admin-page">
      <header className="admin-page-heading">
        <div>
          <h1>Policies</h1>
          <p>
            Typed governance documents, immutable Versions, and exact Bindings.
          </p>
        </div>
        <Button size="sm" onClick={() => setShowCreate((value) => !value)}>
          <Plus size={15} /> {showCreate ? "Close" : "New policy"}
        </Button>
      </header>

      {showCreate ? (
        <form className="admin-form-panel" onSubmit={handleCreateSubmit}>
          <div className="admin-form-grid">
            <label className="admin-filter-input">
              <span className="admin-field-label">Name</span>
              <input
                className="ds-input"
                value={name}
                onChange={(event) => setName(event.target.value)}
                required
              />
            </label>
            <Select
              label="Type"
              options={types.slice(1)}
              value={createType}
              onChange={setCreateType}
            />
            <label className="admin-filter-input admin-form-wide">
              <span className="admin-field-label">Typed document (JSON)</span>
              <textarea
                className="ds-input admin-code-input"
                value={document}
                onChange={(event) => {
                  setDocument(event.target.value);
                  setCreateError("");
                }}
                rows={8}
                required
              />
            </label>
          </div>
          {createError || create.error ? (
            <p className="admin-error" role="alert">
              {createError ||
                (create.error instanceof Error
                  ? create.error.message
                  : "Failed to create policy.")}
            </p>
          ) : null}
          <div className="admin-form-actions">
            <Button
              type="submit"
              isLoading={create.isPending}
              disabled={!name.trim() || create.isPending}
            >
              Create policy draft
            </Button>
          </div>
        </form>
      ) : null}

      <div className="admin-filter-bar">
        <Select
          label="Type"
          options={types}
          value={type}
          onChange={setType}
          placeholder="All policy types"
        />
      </div>

      {(policies.data?.items ?? []).length === 0 ? (
        <EmptyState
          icon={<Shield size={22} />}
          title="No policies in this scope"
          description="Create a typed Draft to begin governance."
          action={
            <Button size="sm" onClick={() => setShowCreate(true)}>
              <Plus size={14} /> Create policy
            </Button>
          }
        />
      ) : (
        <div className="admin-table-wrap">
          <table className="admin-table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Type</th>
                <th>Scope</th>
                <th>State</th>
                <th>Bindings</th>
                <th>Draft ETag</th>
              </tr>
            </thead>
            <tbody>
              {policies.data?.items.map((policy) => (
                <tr key={policy.id}>
                  <td>
                    <Link
                      className="admin-inline-link"
                      to={`/policies/${policy.id}`}
                    >
                      {policy.name}
                    </Link>
                    <br />
                    <span className="admin-muted">{policy.id}</span>
                  </td>
                  <td>{policy.type}</td>
                  <td>{policy.workspace_id ?? "Organization"}</td>
                  <td>{policy.state}</td>
                  <td>{policy.active_binding_count}</td>
                  <td>{policy.draft_etag}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}

export function PolicyDetailPage() {
  const { policyId = "" } = useParams();
  const api = useAdminApi();
  const queryClient = useQueryClient();
  const policy = useQuery({
    queryKey: ["admin-policy", policyId],
    queryFn: () => api.getPolicy(policyId),
    enabled: policyId !== "",
  });
  const draft = useQuery({
    queryKey: ["admin-policy-draft", policyId],
    queryFn: () => api.getPolicyDraft(policyId),
    enabled: policyId !== "",
  });
  const versions = useQuery({
    queryKey: ["admin-policy-versions", policyId],
    queryFn: () => api.listPolicyVersions(policyId),
    enabled: policyId !== "",
  });
  const bindings = useQuery({
    queryKey: ["admin-policy-bindings", policyId],
    queryFn: () => api.listPolicyBindings(policyId),
    enabled: policyId !== "",
  });

  const [draftText, setDraftText] = useState("");
  const [message, setMessage] = useState("");
  const [reason] = useState("");
  const [jsonError, setJsonError] = useState("");

  const currentDraft: PolicyDraft | undefined = draft.data;
  const draftDocument = useMemo(
    () => currentDraft?.document ?? {},
    [currentDraft?.document],
  );
  const text = draftText || JSON.stringify(draftDocument, null, 2);

  const update = useMutation({
    mutationFn: () => {
      let parsed: Record<string, unknown>;
      try {
        parsed = JSON.parse(text) as Record<string, unknown>;
      } catch (err) {
        throw new Error(
          err instanceof Error
            ? `Invalid JSON: ${err.message}`
            : "Invalid JSON format",
        );
      }
      return api.updatePolicyDraft(policyId, draft.data?.etag ?? "", {
        document: parsed,
      });
    },
    onSuccess: () => {
      setJsonError("");
      void queryClient.invalidateQueries({
        queryKey: ["admin-policy-draft", policyId],
      });
      void queryClient.invalidateQueries({
        queryKey: ["admin-policy", policyId],
      });
      setDraftText("");
    },
  });

  const validate = useMutation({
    mutationFn: () => api.validatePolicy(policyId),
    onSuccess: (next) => {
      queryClient.setQueryData(["admin-policy-draft", policyId], next);
    },
  });

  const publish = useMutation({
    mutationFn: () =>
      api.publishPolicyVersion(
        policyId,
        draft.data?.etag ?? "",
        message,
        `policy-publish-${policyId}-${draft.data?.etag ?? ""}`,
      ),
    onSuccess: () => {
      setMessage("");
      void queryClient.invalidateQueries({
        queryKey: ["admin-policy-versions", policyId],
      });
      void queryClient.invalidateQueries({
        queryKey: ["admin-policy", policyId],
      });
    },
  });

  const simulate = useMutation({
    mutationFn: () => api.simulatePolicy(policyId, {}),
  });

  const retire = useMutation({
    mutationFn: () =>
      api.retirePolicy(policyId, reason, `policy-retire-${policyId}`),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ["admin-policy", policyId],
      });
    },
  });

  if (
    policy.isLoading ||
    draft.isLoading ||
    versions.isLoading ||
    bindings.isLoading
  ) {
    return <LoadingState label="Loading policy" />;
  }
  if (policy.error || draft.error || !policy.data || !draft.data) {
    return (
      <div className="admin-page">
        <ErrorState message="This policy is unavailable in your administrative scope." />
      </div>
    );
  }

  const item = policy.data;

  return (
    <section className="admin-page">
      <Link className="admin-back-link" to="/policies">
        <ArrowLeft size={15} /> Policies
      </Link>
      <header className="admin-detail-heading">
        <div>
          <div className="admin-detail-title">
            <Shield size={22} />
            <h1>{item.name}</h1>
          </div>
          <p>
            {item.type} · {item.state} · {item.id}
          </p>
        </div>
        <div className="admin-detail-actions">
          <Button
            size="sm"
            variant="secondary"
            onClick={() => validate.mutate()}
            isLoading={validate.isPending}
          >
            <RotateCcw size={15} /> Validate
          </Button>
          <Button
            size="sm"
            variant="secondary"
            onClick={() => simulate.mutate()}
            isLoading={simulate.isPending}
          >
            <SlidersHorizontal size={15} /> Simulate
          </Button>
          {item.state !== "retired" ? (
            <Button
              size="sm"
              variant="danger"
              onClick={() => retire.mutate()}
              isLoading={retire.isPending}
            >
              Retire policy
            </Button>
          ) : null}
        </div>
      </header>

      <div className="admin-detail-grid">
        <section className="admin-detail-block">
          <div className="admin-section-heading">
            <div>
              <span>Mutable working copy</span>
              <h2>Draft</h2>
            </div>
            <FileKey2 size={18} />
          </div>
          <p className="admin-muted">
            ETag {draft.data.etag} · validation {draft.data.validation.state}
          </p>
          <textarea
            className="ds-input admin-code-input"
            value={text}
            onChange={(event) => {
              setDraftText(event.target.value);
              setJsonError("");
            }}
            rows={15}
          />
          {jsonError || update.error ? (
            <p className="admin-error" role="alert">
              {jsonError ||
                (update.error instanceof Error
                  ? update.error.message
                  : "Save draft failed.")}
            </p>
          ) : null}
          <div className="admin-form-actions">
            <Button
              size="sm"
              onClick={() => update.mutate()}
              isLoading={update.isPending}
            >
              <Check size={15} /> Save Draft
            </Button>
            <input
              className="ds-input"
              value={message}
              onChange={(event) => setMessage(event.target.value)}
              placeholder="Version message (for publishing)"
            />
            <Button
              size="sm"
              variant="secondary"
              onClick={() => publish.mutate()}
              isLoading={publish.isPending}
              disabled={!message.trim() || publish.isPending}
            >
              Publish version
            </Button>
          </div>
        </section>

        <section className="admin-detail-block">
          <h2>Immutable versions</h2>
          {(versions.data?.items ?? []).length === 0 ? (
            <p className="admin-muted">No published versions.</p>
          ) : (
            <ul className="admin-detail-list">
              {versions.data?.items.map((ver) => (
                <li key={ver.id}>
                  <div>
                    <strong>Version {ver.id}</strong>
                    <span>{ver.message}</span>
                  </div>
                  <span>{formatDate(ver.created_at)}</span>
                </li>
              ))}
            </ul>
          )}

          <h2 style={{ marginTop: "24px" }}>Active bindings</h2>
          {(bindings.data?.items ?? []).length === 0 ? (
            <p className="admin-muted">No active bindings.</p>
          ) : (
            <ul className="admin-detail-list">
              {bindings.data?.items.map((b) => (
                <li key={b.id}>
                  <strong>
                    {b.target.scope === "workspace"
                      ? `Workspace ${b.target.workspace_id}`
                      : "Organization"}{" "}
                    ({b.environment})
                  </strong>
                  <span>
                    ID: {b.id} · State: {b.state}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </section>
      </div>
    </section>
  );
}
