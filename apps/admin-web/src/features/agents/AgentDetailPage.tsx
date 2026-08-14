import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  ArrowLeft,
  CheckCircle2,
  FileClock,
  RotateCcw,
  Send,
  ShieldCheck,
  TriangleAlert,
} from 'lucide-react';
import { Button, StatusMark } from '@gantry/design-system';
import { Link, useParams } from 'react-router-dom';
import { useEffect, useState } from 'react';
import { useAdminApi } from '../../api/ApiProvider';
import type { AgentSpec } from '../../api/types';
import { ErrorState, LoadingState } from '../../components/AsyncState';

const defaultSpec: AgentSpec = {
  kind: 'gantry.agent/v1',
  model: { provider: 'scripted', model: 'deterministic' },
  workspace_root: '.',
  limits: { max_turns: 12, max_output_bytes: 131072 },
  checkpoint: { enabled: false },
  command_policy: { allow_shell: false },
};

export function AgentDetailPage() {
  const { agentId = '' } = useParams();
  const api = useAdminApi();
  const queryClient = useQueryClient();

  const agent = useQuery({
    queryKey: ['admin-agent', agentId],
    queryFn: () => api.getAgent(agentId),
  });

  const draft = useQuery({
    queryKey: ['admin-draft', agentId],
    queryFn: () => api.getDraft(agentId),
  });

  const versions = useQuery({
    queryKey: ['admin-versions', agentId],
    queryFn: () => api.listVersions(agentId),
  });

  const review = useQuery({
    queryKey: ['admin-review', agentId],
    queryFn: () => api.getReview(agentId),
  });

  const [spec, setSpec] = useState<AgentSpec>(defaultSpec);
  const [releaseNotes, setReleaseNotes] = useState('');

  useEffect(() => {
    if (draft.data?.spec) setSpec(draft.data.spec as unknown as AgentSpec);
  }, [draft.data?.spec]);

  const refresh = () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: ['admin-agent', agentId] }),
      queryClient.invalidateQueries({ queryKey: ['admin-draft', agentId] }),
      queryClient.invalidateQueries({ queryKey: ['admin-versions', agentId] }),
      queryClient.invalidateQueries({ queryKey: ['admin-review', agentId] }),
      queryClient.invalidateQueries({ queryKey: ['admin-agents'] }),
    ]);

  const save = useMutation({
    mutationFn: () => api.updateDraft(agentId, draft.data!.revision, spec),
    onSuccess: refresh,
  });

  const publish = useMutation({
    mutationFn: () => api.publish(agentId, draft.data!.revision),
    onSuccess: refresh,
  });

  const retire = useMutation({
    mutationFn: () => api.retire(agentId),
    onSuccess: refresh,
  });

  const submitReview = useMutation({
    mutationFn: () => api.submitReview(agentId, draft.data!.revision, releaseNotes),
    onSuccess: refresh,
  });

  const decideReview = useMutation({
    mutationFn: (decision: 'approve' | 'reject') =>
      api.decideReview(agentId, decision, releaseNotes),
    onSuccess: refresh,
  });

  const rollback = useMutation({
    mutationFn: (versionId: string) => api.rollback(agentId, versionId),
    onSuccess: refresh,
  });

  if (agent.isLoading || draft.isLoading || versions.isLoading || review.isLoading) {
    return <LoadingState label="Loading agent" />;
  }

  if (
    agent.error ||
    draft.error ||
    versions.error ||
    review.error ||
    !agent.data ||
    !draft.data ||
    !review.data
  ) {
    return (
      <div className="admin-page">
        <ErrorState message="This agent is unavailable in your administrative scope." />
      </div>
    );
  }

  const isDirty = JSON.stringify(spec) !== JSON.stringify(draft.data.spec);
  const busy =
    save.isPending ||
    publish.isPending ||
    retire.isPending ||
    submitReview.isPending ||
    decideReview.isPending ||
    rollback.isPending;

  const reviewApproved =
    review.data.status === 'approved' && review.data.draft_revision === draft.data.revision;
  const reviewPending =
    review.data.status === 'pending' && review.data.draft_revision === draft.data.revision;

  const mutationError =
    save.error ??
    publish.error ??
    retire.error ??
    submitReview.error ??
    decideReview.error ??
    rollback.error;

  return (
    <section className="admin-page admin-detail-page">
      <Link className="admin-back-link" to="/">
        <ArrowLeft size={16} />
        <span>Agents</span>
      </Link>

      <header className="admin-detail-heading">
        <div>
          <div className="admin-detail-title">
            <h1>{agent.data.display_name}</h1>
            <StatusMark status={agent.data.lifecycle_status} />
          </div>
          <p>{agent.data.description}</p>
        </div>

        <div className="admin-command-row">
          <Button
            variant="secondary"
            onClick={() => save.mutate()}
            disabled={!isDirty || busy}
          >
            {save.isPending ? 'Saving…' : 'Save revision'}
          </Button>

          <Button
            onClick={() => publish.mutate()}
            disabled={
              draft.data.validation_status !== 'valid' ||
              isDirty ||
              !reviewApproved ||
              busy
            }
          >
            <Send size={16} />
            <span>{publish.isPending ? 'Publishing…' : 'Publish'}</span>
          </Button>

          {agent.data.lifecycle_status === 'published' ? (
            <Button
              variant="danger"
              onClick={() => retire.mutate()}
              disabled={busy}
            >
              {retire.isPending ? 'Retiring…' : 'Retire'}
            </Button>
          ) : null}
        </div>
      </header>

      {mutationError ? <ErrorState message={mutationError.message} /> : null}

      <div className="admin-detail-grid">
        <section className="admin-editor">
          <div className="admin-section-heading">
            <div>
              <span>Draft revision {draft.data.revision}</span>
              <h2>Execution configuration</h2>
            </div>
            <FileClock size={19} />
          </div>

          <div
            className={`admin-validation ${
              draft.data.validation_status === 'valid'
                ? 'admin-validation-valid'
                : 'admin-validation-invalid'
            }`}
          >
            {draft.data.validation_status === 'valid' ? (
              <CheckCircle2 size={17} />
            ) : (
              <TriangleAlert size={17} />
            )}
            <div>
              <strong>
                {draft.data.validation_status === 'valid'
                  ? 'Draft is valid'
                  : 'Draft needs attention'}
              </strong>
              {draft.data.validation_findings.map((finding) => (
                <span key={`${finding.path}-${finding.message}`}>
                  {finding.path || 'Specification'}: {finding.message}
                </span>
              ))}
            </div>
          </div>

          <dl className="admin-spec-summary">
            <div>
              <dt>Manifest kind</dt>
              <dd>{spec.kind}</dd>
            </div>
          </dl>
        </section>

        <aside className="admin-version-panel">
          <div className="admin-section-heading">
            <div>
              <span>Immutable</span>
              <h2>Version history</h2>
            </div>
          </div>

          {versions.data?.items.length === 0 ? (
            <p className="admin-muted">No version has been published.</p>
          ) : (
            <ol className="admin-version-list">
              {versions.data?.items.map((version) => (
                <li key={version.id}>
                  <div className="admin-version-title">
                    <strong>Version {version.version}</strong>
                    {agent.data.current_published_version_id === version.id ? (
                      <StatusMark status="published" />
                    ) : null}
                  </div>
                  <span>Draft revision {version.source_draft_revision}</span>
                  <code>{version.spec_digest.slice(0, 19)}...</code>
                  {agent.data.current_published_version_id !== version.id ? (
                    <Button
                      variant="secondary"
                      size="sm"
                      onClick={() => {
                        if (
                          window.confirm(
                            `Roll back to Version ${version.version}? This creates a new published version.`
                          )
                        ) {
                          rollback.mutate(version.id);
                        }
                      }}
                      disabled={busy}
                    >
                      <RotateCcw size={14} />
                      <span>Roll back</span>
                    </Button>
                  ) : null}
                </li>
              ))}
            </ol>
          )}
        </aside>
      </div>

      <section className="admin-review-panel">
        <div className="admin-section-heading">
          <div>
            <span>Governance</span>
            <h2>Review current draft</h2>
          </div>
          <ShieldCheck size={19} />
        </div>

        <div className="admin-review-status">
          <StatusMark status={review.data.status} />
          <strong>
            {review.data.status === 'not_submitted'
              ? 'Not submitted for review'
              : `Review ${review.data.status}`}
          </strong>
          <span>
            Draft revision {review.data.draft_revision} ·{' '}
            {review.data.risk_summary.total} changes ·{' '}
            {review.data.risk_summary.high} high risk
          </span>
        </div>

        <label className="admin-field">
          <span className="admin-field-label">Release notes or decision reason</span>
          <textarea
            value={releaseNotes}
            onChange={(event) => setReleaseNotes(event.target.value)}
            disabled={busy}
            placeholder="Summarize the behavior and risk of this revision."
            className="ds-input admin-textarea"
            rows={3}
          />
        </label>

        <div className="admin-review-actions">
          {review.data.status === 'not_submitted' ||
          review.data.status === 'rejected' ? (
            <Button
              onClick={() => submitReview.mutate()}
              disabled={busy || isDirty}
            >
              {submitReview.isPending ? 'Submitting…' : 'Submit for review'}
            </Button>
          ) : null}

          {reviewPending ? (
            <>
              <Button
                variant="primary"
                onClick={() => decideReview.mutate('approve')}
                disabled={busy}
              >
                {decideReview.isPending ? 'Recording…' : 'Approve draft'}
              </Button>
              <Button
                variant="danger"
                onClick={() => decideReview.mutate('reject')}
                disabled={busy}
              >
                Reject draft
              </Button>
            </>
          ) : null}
        </div>

        <div className="admin-diff-list">
          {review.data.diff.map((entry) => (
            <div className="admin-diff-row" key={entry.path}>
              <code>{entry.path}</code>
              <span>{entry.change}</span>
              <small>{entry.risk} risk</small>
            </div>
          ))}
        </div>

        {review.data.review_reason ? (
          <p className="admin-review-reason">
            <strong>Decision note:</strong> {review.data.review_reason}
          </p>
        ) : null}
      </section>
    </section>
  );
}
