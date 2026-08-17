import {
  ArrowLeft,
  GitBranch,
  History,
  ShieldCheck,
  TestTube2,
} from "lucide-react";
import { Link, useParams } from "react-router-dom";
import { formatDate, shortHash, StatusMark } from "@gantry/design-system";
import { useQuery } from "@tanstack/react-query";
import { useAdminApi } from "../../api/ApiProvider";
import { ErrorState, LoadingState } from "../../components/AsyncState";

export function AgentVersionsPage() {
  const { agentId = "" } = useParams();
  const api = useAdminApi();
  const lifecycle = useQuery({
    queryKey: ["admin-agent-lifecycle", agentId],
    queryFn: () => api.getAgentLifecycle(agentId),
    enabled: agentId !== "",
  });
  const revisions = useQuery({
    queryKey: ["admin-agent-revisions", agentId],
    queryFn: () => api.listRevisions(agentId),
    enabled: agentId !== "",
  });
  const deployments = useQuery({
    queryKey: ["admin-agent-deployments", agentId],
    queryFn: () => api.listDeployments(agentId),
    enabled: agentId !== "",
  });

  if (lifecycle.isLoading || revisions.isLoading || deployments.isLoading) {
    return <LoadingState label="Loading Agent Versions" />;
  }
  if (
    lifecycle.error ||
    revisions.error ||
    deployments.error ||
    !lifecycle.data
  ) {
    return (
      <div className="admin-page">
        <ErrorState message="This Agent history is unavailable in your administrative scope." />
      </div>
    );
  }

  const { agent, drafts, production_deployment: production } = lifecycle.data;
  const tests = deployments.data?.items ?? [];
  const items = revisions.data?.items ?? [];

  return (
    <section className="admin-page">
      <Link className="admin-back-link" to={`/agents/${agentId}`}>
        <ArrowLeft size={15} /> Agent overview
      </Link>
      <header className="admin-detail-heading">
        <div>
          <div className="admin-detail-title">
            <History size={22} />
            <h1>Versions</h1>
            <StatusMark status={agent.lifecycle_status} />
          </div>
          <p>
            {agent.display_name} · immutable configuration history and
            deployment pointers
          </p>
        </div>
        <Link
          className="ds-button ds-button-primary"
          to={`/agents/${agentId}/design`}
        >
          Open Main Draft
        </Link>
      </header>

      <div className="admin-overview-grid">
        <section className="admin-detail-block">
          <div className="admin-section-heading">
            <div>
              <span>Production</span>
              <h2>Default Deployment</h2>
            </div>
            <ShieldCheck size={18} />
          </div>
          {production ? (
            <>
              <div className="admin-overview-highlight">
                <strong>{shortHash(production.revision_hash)}</strong>
                <StatusMark status={production.status} />
              </div>
              <Detail label="Moved" value={formatDate(production.updated_at)} />
              <Link
                className="admin-inline-link"
                to={`/agents/${agentId}/revisions/${production.revision_hash}`}
              >
                Inspect Revision
              </Link>
            </>
          ) : (
            <p className="admin-muted">No Production Deployment.</p>
          )}
        </section>

        <section className="admin-detail-block">
          <div className="admin-section-heading">
            <div>
              <span>Test environments</span>
              <h2>Active Deployments</h2>
            </div>
            <TestTube2 size={18} />
          </div>
          {tests.filter((item) => item.status === "active").length === 0 ? (
            <p className="admin-muted">No active Test Deployments.</p>
          ) : (
            <ul className="admin-detail-list">
              {tests
                .filter((item) => item.status === "active")
                .map((item) => (
                  <li key={item.id}>
                    <Link
                      className="admin-overview-row-link"
                      to={`/agents/${agentId}/revisions/${item.revision_hash}`}
                    >
                      <strong>{item.name}</strong>
                      <span>{shortHash(item.revision_hash)}</span>
                    </Link>
                  </li>
                ))}
            </ul>
          )}
        </section>
      </div>

      <section className="admin-detail-block">
        <div className="admin-section-heading">
          <div>
            <span>Draft pointers</span>
            <h2>Latest Revisions by Draft</h2>
          </div>
          <GitBranch size={18} />
        </div>
        {drafts.length === 0 ? (
          <p className="admin-muted">No active Drafts.</p>
        ) : (
          <ul className="admin-detail-list">
            {drafts.map((draft) => (
              <li key={draft.id}>
                <div className="admin-overview-row-link">
                  <strong>{draft.name}</strong>
                  <span>
                    {draft.latest_revision_hash ? (
                      <Link
                        className="admin-inline-link"
                        to={`/agents/${agentId}/revisions/${draft.latest_revision_hash}`}
                      >
                        {shortHash(draft.latest_revision_hash)}
                      </Link>
                    ) : (
                      "No committed Revision"
                    )}
                  </span>
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section className="admin-detail-block">
        <div className="admin-section-heading">
          <div>
            <span>Immutable history</span>
            <h2>Revision Table</h2>
          </div>
          <History size={18} />
        </div>
        {items.length === 0 ? (
          <p className="admin-muted">No committed Revisions.</p>
        ) : (
          <div className="admin-table-wrap">
            <table className="admin-table">
              <thead>
                <tr>
                  <th>Revision</th>
                  <th>Message</th>
                  <th>Draft</th>
                  <th>Review</th>
                  <th>Deployment</th>
                  <th>Runs</th>
                  <th>Created</th>
                </tr>
              </thead>
              <tbody>
                {items.map((item) => (
                  <tr key={item.revision_hash}>
                    <td>
                      <Link
                        className="admin-inline-link"
                        to={`/agents/${agentId}/revisions/${item.revision_hash}`}
                      >
                        {shortHash(item.revision_hash)}
                      </Link>
                    </td>
                    <td>
                      <strong>{item.message}</strong>
                      <br />
                      <code>{shortHash(item.spec_digest)}</code>
                    </td>
                    <td>{item.source_draft_name}</td>
                    <td>
                      <StatusMark status={item.review_status} />
                    </td>
                    <td>
                      {item.production_deployed ? (
                        <span className="admin-deployment-label">
                          Production
                        </span>
                      ) : null}
                      {item.test_deployed ? (
                        <span className="admin-deployment-count">Test</span>
                      ) : null}
                      {!item.production_deployed && !item.test_deployed ? (
                        <span className="admin-muted">None</span>
                      ) : null}
                    </td>
                    <td>{item.run_count}</td>
                    <td>{formatDate(item.created_at)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </section>
  );
}

function Detail({ label, value }: { label: string; value: string }) {
  return (
    <div className="admin-detail-field">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}
