import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ArrowLeft,
  Archive,
  CheckCircle2,
  FileClock,
  GitBranch,
  Plus,
  Send,
  ShieldCheck,
  TestTube2,
  TriangleAlert,
} from "lucide-react";
import {
  Button,
  Select,
  type SelectOption,
  shortHash,
  StatusMark,
} from "@gantry/design-system";
import { Link, useParams, useSearchParams } from "react-router-dom";
import { useEffect, useState } from "react";
import { useAdminApi } from "../../api/ApiProvider";
import type { AgentSpec, NamedAgentDraft } from "../../api/types";
import { ErrorState, LoadingState } from "../../components/AsyncState";

const defaultSpec: AgentSpec = {
  kind: "gantry.agent/v1",
  model: { provider: "scripted", model: "deterministic" },
  workspace_root: ".",
  limits: { max_turns: 12, max_output_bytes: 131072 },
  checkpoint: { enabled: false },
  command_policy: { allow_shell: false },
};

const modelProviderOptions: SelectOption[] = [
  { value: "scripted", label: "Scripted" },
  { value: "openai", label: "OpenAI" },
  { value: "openai-compatible", label: "OpenAI-compatible" },
  { value: "anthropic", label: "Anthropic" },
];

export function AgentDetailPage() {
  const { agentId = "" } = useParams();
  const [searchParams, setSearchParams] = useSearchParams();
  const api = useAdminApi();
  const queryClient = useQueryClient();

  const lifecycle = useQuery({
    queryKey: ["admin-agent-lifecycle", agentId],
    queryFn: () => api.getAgentLifecycle(agentId),
    enabled: agentId !== "",
  });

  const queryDraftId = searchParams.get("draft");
  const draftID = queryDraftId ?? lifecycle.data?.main_draft.id ?? "";
  const isCustomDraft = Boolean(
    queryDraftId && queryDraftId !== lifecycle.data?.main_draft.id,
  );
  const draft = useQuery({
    queryKey: ["admin-named-draft", agentId, draftID],
    queryFn: () => api.getNamedDraft(agentId, draftID),
    enabled: Boolean(agentId && draftID && isCustomDraft),
  });

  const skills = useQuery({
    queryKey: ["admin-skills", lifecycle.data?.agent.workspace_id],
    queryFn: () =>
      api.listSkills({
        workspaceId: lifecycle.data!.agent.workspace_id,
        status: "available",
      }),
    enabled: Boolean(lifecycle.data?.agent.workspace_id),
  });

  const plugins = useQuery({
    queryKey: ["admin-plugins"],
    queryFn: () => api.listPlugins({ status: "active" }),
  });

  const tools = useQuery({
    queryKey: ["admin-tools"],
    queryFn: () => api.listTools({ status: "active" }),
  });

  const [spec, setSpec] = useState<AgentSpec>(defaultSpec);
  const [message, setMessage] = useState("");
  const [releaseNotes, setReleaseNotes] = useState("");
  const [testName, setTestName] = useState("");
  const [testPurpose, setTestPurpose] = useState("");
  const [newDraftName, setNewDraftName] = useState("");
  const [committedHash, setCommittedHash] = useState("");

  const selectedDraft: NamedAgentDraft | undefined = isCustomDraft
    ? (draft.data ?? lifecycle.data?.drafts.find((d) => d.id === queryDraftId))
    : lifecycle.data?.main_draft;
  const revisionHash =
    committedHash || selectedDraft?.latest_revision_hash || "";

  const review = useQuery({
    queryKey: ["admin-revision-review", agentId, revisionHash],
    queryFn: () => api.getRevisionReview(agentId, revisionHash),
    enabled: Boolean(revisionHash),
  });

  useEffect(() => {
    if (selectedDraft?.spec)
      setSpec(selectedDraft.spec as unknown as AgentSpec);
    setCommittedHash("");
    setReleaseNotes("");
  }, [selectedDraft?.id, selectedDraft?.working_copy_etag]);

  const refresh = () =>
    Promise.all([
      queryClient.invalidateQueries({
        queryKey: ["admin-agent-lifecycle", agentId],
      }),
      queryClient.invalidateQueries({
        queryKey: ["admin-named-draft", agentId],
      }),
      queryClient.invalidateQueries({
        queryKey: ["admin-agent-revisions", agentId],
      }),
      queryClient.invalidateQueries({
        queryKey: ["admin-revision-review", agentId],
      }),
      queryClient.invalidateQueries({ queryKey: ["admin-agents"] }),
    ]);

  const save = useMutation({
    mutationFn: () =>
      api.updateNamedDraft(
        agentId,
        selectedDraft!.id,
        selectedDraft!.working_copy_etag,
        spec,
      ),
    onSuccess: refresh,
  });

  const commit = useMutation({
    mutationFn: () => api.commitDraft(agentId, selectedDraft!.id, message),
    onSuccess: (revision) => {
      setCommittedHash(revision.revision_hash);
      setMessage("");
      void refresh();
    },
  });

  const submitReview = useMutation({
    mutationFn: () =>
      api.submitRevisionReview(agentId, revisionHash, releaseNotes),
    onSuccess: refresh,
  });

  const decideReview = useMutation({
    mutationFn: (decision: "approve" | "reject") =>
      api.decideRevisionReview(agentId, revisionHash, decision, releaseNotes),
    onSuccess: refresh,
  });

  const publish = useMutation({
    mutationFn: () =>
      api.publishRevision(
        agentId,
        revisionHash,
        lifecycle.data?.production_deployment?.revision_hash ?? "",
      ),
    onSuccess: refresh,
  });

  const createTest = useMutation({
    mutationFn: () =>
      api.createTestDeployment(agentId, {
        name: testName,
        revision_hash: revisionHash,
        purpose: testPurpose,
      }),
    onSuccess: () => {
      setTestName("");
      setTestPurpose("");
      void refresh();
    },
  });

  const createDraft = useMutation({
    mutationFn: () =>
      api.createDraft(agentId, {
        name: newDraftName,
        from_revision_hash: revisionHash || undefined,
      }),
    onSuccess: (newDraft) => {
      setNewDraftName("");
      setSearchParams({ draft: newDraft.id });
      void refresh();
    },
  });

  const archive = useMutation({
    mutationFn: () => api.archiveDraft(agentId, selectedDraft!.id),
    onSuccess: () => {
      setSearchParams({});
      void refresh();
    },
  });

  if (lifecycle.isLoading || draft.isLoading)
    return <LoadingState label="Loading agent design" />;
  if (lifecycle.error || draft.error || !lifecycle.data || !selectedDraft) {
    return (
      <div className="admin-page">
        <ErrorState message="This agent design is unavailable in your administrative scope." />
      </div>
    );
  }

  const isDirty = JSON.stringify(spec) !== JSON.stringify(selectedDraft.spec);
  const busy =
    save.isPending ||
    commit.isPending ||
    submitReview.isPending ||
    decideReview.isPending ||
    publish.isPending ||
    createTest.isPending ||
    createDraft.isPending ||
    archive.isPending;
  const mutationError =
    save.error ??
    commit.error ??
    submitReview.error ??
    decideReview.error ??
    publish.error ??
    createTest.error ??
    createDraft.error ??
    archive.error;
  const reviewApproved = review.data?.status === "approved";

  const selectBindings = (
    field: "skills" | "plugins",
    valueKey: "artifact_id" | "plugin_version_id",
    selected: string[],
  ) =>
    setSpec((current) => ({
      ...current,
      [field]: selected.map((id) => ({ [valueKey]: id })),
    }));

  const selectToolBindings = (selected: string[]) =>
    setSpec((current) => ({
      ...current,
      tool_bindings: selected.map(
        (id) =>
          current.tool_bindings?.find(
            (binding) => binding.descriptor_id === id,
          ) ?? { descriptor_id: id },
      ),
    }));

  const updateToolOperations = (descriptorID: string, value: string) =>
    setSpec((current) => ({
      ...current,
      tool_bindings: (current.tool_bindings ?? []).map((binding) =>
        binding.descriptor_id === descriptorID
          ? {
              ...binding,
              operations: value
                .split(",")
                .map((op) => op.trim())
                .filter(Boolean),
            }
          : binding,
      ),
    }));

  return (
    <section className="admin-page admin-detail-page">
      <Link className="admin-back-link" to={`/agents/${agentId}`}>
        <ArrowLeft size={16} /> <span>Agent overview</span>
      </Link>

      <header className="admin-detail-heading">
        <div>
          <div className="admin-detail-title">
            <h1>{lifecycle.data.agent.display_name}</h1>
            <StatusMark status={lifecycle.data.agent.lifecycle_status} />
          </div>
          <p>{lifecycle.data.agent.description}</p>
        </div>
        <div className="admin-command-row">
          <Button
            variant="secondary"
            onClick={() => save.mutate()}
            disabled={!isDirty || busy}
          >
            {save.isPending ? "Saving…" : "Save Draft"}
          </Button>
          <Button
            onClick={() => commit.mutate()}
            disabled={
              isDirty ||
              selectedDraft.validation_status !== "valid" ||
              !message.trim() ||
              busy
            }
          >
            <GitBranch size={16} />{" "}
            {commit.isPending ? "Committing…" : "Commit Revision"}
          </Button>
        </div>
      </header>

      {mutationError ? <ErrorState message={mutationError.message} /> : null}

      <div className="admin-detail-grid">
        <section className="admin-editor">
          <div className="admin-section-heading">
            <div>
              <span>
                {selectedDraft.name} · ETag {selectedDraft.working_copy_etag}
              </span>
              <h2>Execution configuration</h2>
            </div>
            <FileClock size={19} />
          </div>

          <div
            className={`admin-validation ${
              selectedDraft.validation_status === "valid"
                ? "admin-validation-valid"
                : "admin-validation-invalid"
            }`}
          >
            {selectedDraft.validation_status === "valid" ? (
              <CheckCircle2 size={17} />
            ) : (
              <TriangleAlert size={17} />
            )}
            <div>
              <strong>
                {selectedDraft.validation_status === "valid"
                  ? "Draft is valid"
                  : "Draft needs attention"}
              </strong>
              {selectedDraft.validation_findings.map((finding) => (
                <span key={`${finding.path}-${finding.message}`}>
                  {finding.path || "Specification"}: {finding.message}
                </span>
              ))}
            </div>
          </div>

          <label className="admin-field">
            <span className="admin-field-label">System prompt</span>
            <textarea
              className="ds-input admin-textarea"
              rows={7}
              value={spec.system_prompt ?? ""}
              onChange={(event) =>
                setSpec((current) => ({
                  ...current,
                  system_prompt: event.target.value,
                }))
              }
              disabled={busy}
              placeholder="Provide behavioral guidance and instructions for this agent."
            />
          </label>

          <label className="admin-field">
            <span className="admin-field-label">User input template</span>
            <textarea
              className="ds-input admin-textarea"
              rows={3}
              value={spec.user_input ?? ""}
              onChange={(event) =>
                setSpec((current) => ({
                  ...current,
                  user_input: event.target.value,
                }))
              }
              disabled={busy}
              placeholder="Optional user input template (e.g. {{user_message}})"
            />
          </label>

          <div className="admin-form-grid admin-model-fields">
            <Select
              label="Model provider"
              options={modelProviderOptions}
              value={spec.model.provider}
              onChange={(value) =>
                setSpec((current) => ({
                  ...current,
                  model: {
                    ...current.model,
                    provider: value as AgentSpec["model"]["provider"],
                  },
                }))
              }
              disabled={busy}
            />
            <label className="admin-field">
              <span className="admin-field-label">Model</span>
              <input
                className="ds-input"
                value={spec.model.model}
                onChange={(event) =>
                  setSpec((current) => ({
                    ...current,
                    model: { ...current.model, model: event.target.value },
                  }))
                }
                disabled={busy}
                placeholder="e.g. gpt-4o, claude-3-5-sonnet, deterministic"
              />
            </label>
          </div>

          <div className="admin-asset-bindings">
            <div className="admin-section-heading">
              <div>
                <span>Immutable references</span>
                <h2>Configuration assets</h2>
              </div>
            </div>

            <label className="admin-field">
              <span className="admin-field-label">Skills</span>
              <select
                className="ds-input admin-multiselect"
                multiple
                value={(spec.skills ?? []).map(
                  (binding) => binding.artifact_id,
                )}
                onChange={(event) =>
                  selectBindings(
                    "skills",
                    "artifact_id",
                    Array.from(
                      event.currentTarget.selectedOptions,
                      (option) => option.value,
                    ),
                  )
                }
                disabled={busy || skills.isLoading}
              >
                {(skills.data?.items ?? []).map((skill) => (
                  <option key={skill.id} value={skill.id}>
                    {skill.display_name}{" "}
                    {skill.declared_version || "undeclared"}
                  </option>
                ))}
              </select>
            </label>

            <label className="admin-field">
              <span className="admin-field-label">Plugin versions</span>
              <select
                className="ds-input admin-multiselect"
                multiple
                value={(spec.plugins ?? []).map(
                  (binding) => binding.plugin_version_id,
                )}
                onChange={(event) =>
                  selectBindings(
                    "plugins",
                    "plugin_version_id",
                    Array.from(
                      event.currentTarget.selectedOptions,
                      (option) => option.value,
                    ),
                  )
                }
                disabled={busy || plugins.isLoading}
              >
                {(plugins.data?.items ?? []).map((plugin) => (
                  <option key={plugin.id} value={plugin.id}>
                    {plugin.display_name} {plugin.version}
                  </option>
                ))}
              </select>
            </label>

            <label className="admin-field">
              <span className="admin-field-label">Tool descriptors</span>
              <select
                className="ds-input admin-multiselect"
                multiple
                value={(spec.tool_bindings ?? []).map(
                  (binding) => binding.descriptor_id,
                )}
                onChange={(event) =>
                  selectToolBindings(
                    Array.from(
                      event.currentTarget.selectedOptions,
                      (option) => option.value,
                    ),
                  )
                }
                disabled={busy || tools.isLoading}
              >
                {(tools.data?.items ?? []).map((tool) => (
                  <option key={tool.id} value={tool.id}>
                    {tool.fully_qualified_name} {tool.version}
                  </option>
                ))}
              </select>
            </label>

            {(spec.tool_bindings ?? []).length > 0 ? (
              <div className="admin-binding-constraints">
                <span className="admin-field-label">Allowed operations</span>
                {(spec.tool_bindings ?? []).map((binding) => (
                  <label
                    className="admin-binding-constraint"
                    key={binding.descriptor_id}
                  >
                    <span>
                      {tools.data?.items.find(
                        (tool) => tool.id === binding.descriptor_id,
                      )?.fully_qualified_name ?? binding.descriptor_id}
                    </span>
                    <input
                      className="ds-input"
                      value={(binding.operations ?? []).join(", ")}
                      onChange={(event) =>
                        updateToolOperations(
                          binding.descriptor_id,
                          event.target.value,
                        )
                      }
                      disabled={busy}
                      placeholder="e.g. read, write, execute (comma-separated)"
                    />
                  </label>
                ))}
              </div>
            ) : null}
          </div>
        </section>

        <aside className="admin-version-panel">
          <div className="admin-section-heading">
            <div>
              <span>Drafts</span>
              <h2>Named working copies</h2>
            </div>
          </div>
          <ol className="admin-version-list">
            {lifecycle.data.drafts.map((item) => (
              <li key={item.id}>
                <Link
                  className="admin-overview-row-link"
                  to={`/agents/${agentId}/design${item.id === lifecycle.data.main_draft.id ? "" : `?draft=${encodeURIComponent(item.id)}`}`}
                >
                  <strong>{item.name}</strong>
                  <StatusMark status={item.status} />
                </Link>
                <span>
                  ETag {item.working_copy_etag} ·{" "}
                  {item.latest_revision_hash
                    ? shortHash(item.latest_revision_hash)
                    : "uncommitted"}
                </span>
                {item.name !== "Main" && item.id === selectedDraft.id ? (
                  <Button
                    variant="quiet"
                    size="sm"
                    onClick={() => archive.mutate()}
                    disabled={busy}
                  >
                    <Archive size={14} /> Archive
                  </Button>
                ) : null}
              </li>
            ))}
          </ol>
          <div className="admin-form-row">
            <input
              className="ds-input"
              value={newDraftName}
              onChange={(event) => setNewDraftName(event.target.value)}
              placeholder="New draft name"
              disabled={busy}
            />
            <Button
              size="sm"
              variant="secondary"
              onClick={() => createDraft.mutate()}
              disabled={!newDraftName.trim() || busy}
            >
              <Plus size={14} /> Create
            </Button>
          </div>
        </aside>
      </div>

      <label className="admin-field" style={{ marginTop: "20px" }}>
        <span className="admin-field-label">Commit message</span>
        <input
          className="ds-input"
          value={message}
          onChange={(event) => setMessage(event.target.value)}
          placeholder="Describe the intended behavior change"
          disabled={busy}
        />
      </label>

      <section className="admin-review-panel">
        <div className="admin-section-heading">
          <div>
            <span>Revision-bound governance</span>
            <h2>
              {revisionHash
                ? `Review ${shortHash(revisionHash)}`
                : "Commit a Revision to review"}
            </h2>
          </div>
          <ShieldCheck size={19} />
        </div>

        {revisionHash ? (
          <>
            <div className="admin-review-status">
              <StatusMark status={review.data?.status ?? "not_submitted"} />
              <strong>
                {review.data?.status === "not_submitted"
                  ? "Not submitted for review"
                  : `Review ${review.data?.status}`}
              </strong>
              <span>
                {review.data?.risk_summary.total ?? 0} changes ·{" "}
                {review.data?.risk_summary.high ?? 0} high risk
              </span>
            </div>
            <label className="admin-field">
              <span className="admin-field-label">
                Release notes or decision reason
              </span>
              <textarea
                value={releaseNotes}
                onChange={(event) => setReleaseNotes(event.target.value)}
                disabled={busy}
                className="ds-input admin-textarea"
                rows={3}
                placeholder="Explain the changes or provide approval/rejection justification"
              />
            </label>
            <div className="admin-review-actions">
              {!review.data ||
              review.data.status === "not_submitted" ||
              review.data.status === "rejected" ? (
                <Button
                  onClick={() => submitReview.mutate()}
                  disabled={busy || !releaseNotes.trim()}
                >
                  Submit for review
                </Button>
              ) : null}
              {review.data?.status === "pending" ? (
                <>
                  <Button
                    variant="primary"
                    onClick={() => decideReview.mutate("approve")}
                    disabled={busy}
                  >
                    Approve Revision
                  </Button>
                  <Button
                    variant="danger"
                    onClick={() => decideReview.mutate("reject")}
                    disabled={busy}
                  >
                    Reject Revision
                  </Button>
                </>
              ) : null}
              {reviewApproved ? (
                <Button onClick={() => publish.mutate()} disabled={busy}>
                  <Send size={16} />{" "}
                  {publish.isPending
                    ? "Publishing…"
                    : "Move Production pointer"}
                </Button>
              ) : null}
            </div>
            {review.data?.diff.map((entry) => (
              <div className="admin-diff-row" key={entry.path}>
                <code>{entry.path}</code>
                <span>{entry.change}</span>
                <small>{entry.risk} risk</small>
              </div>
            ))}
          </>
        ) : (
          <p className="admin-muted">
            Commit the valid Draft with a message before starting review.
          </p>
        )}
      </section>

      <section className="admin-detail-block">
        <div className="admin-section-heading">
          <div>
            <span>Test environment</span>
            <h2>Create a Test Deployment</h2>
          </div>
          <TestTube2 size={18} />
        </div>
        {revisionHash ? (
          <div className="admin-form-grid">
            <input
              className="ds-input"
              value={testName}
              onChange={(event) => setTestName(event.target.value)}
              placeholder="Deployment name (e.g. QA, Staging, Canary)"
              disabled={busy}
            />
            <input
              className="ds-input"
              value={testPurpose}
              onChange={(event) => setTestPurpose(event.target.value)}
              placeholder="Purpose (optional)"
              disabled={busy}
            />
            <Button
              onClick={() => createTest.mutate()}
              disabled={!testName.trim() || busy}
            >
              {createTest.isPending ? "Creating…" : "Create Test Deployment"}
            </Button>
          </div>
        ) : (
          <p className="admin-muted">A committed Revision is required.</p>
        )}
      </section>
    </section>
  );
}
