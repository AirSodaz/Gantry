import { ArrowLeft, FileText, ShieldCheck } from "lucide-react";
import { Link, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import {
  CodeBlock,
  formatDate,
  shortHash,
  StatusMark,
} from "@gantry/design-system";
import { useAdminApi } from "../../api/ApiProvider";
import { ErrorState, LoadingState } from "../../components/AsyncState";

export function AgentVersionPage() {
  const { agentId = "", revisionHash = "" } = useParams();
  const api = useAdminApi();

  const revision = useQuery({
    queryKey: ["admin-agent-revision", agentId, revisionHash],
    queryFn: () => api.getRevision(agentId, revisionHash),
    enabled: agentId !== "" && revisionHash !== "",
  });

  const review = useQuery({
    queryKey: ["admin-revision-review", agentId, revisionHash],
    queryFn: () => api.getRevisionReview(agentId, revisionHash),
    enabled: agentId !== "" && revisionHash !== "",
  });

  if (revision.isLoading)
    return <LoadingState label="Loading immutable Revision" />;
  if (revision.error || !revision.data) {
    return (
      <div className="admin-page">
        <ErrorState message="This immutable Revision is unavailable in your administrative scope." />
      </div>
    );
  }

  const item = revision.data;

  return (
    <section className="admin-page">
      <Link className="admin-back-link" to={`/agents/${agentId}/versions`}>
        <ArrowLeft size={15} /> Versions
      </Link>
      <header className="admin-page-heading">
        <div>
          <div className="admin-detail-title">
            <FileText size={22} />
            <h1>{shortHash(item.revision_hash)}</h1>
            <StatusMark status={review.data?.status ?? item.review_status} />
          </div>
          <p>{item.message}</p>
        </div>
      </header>

      <div className="admin-detail-grid admin-version-detail-grid">
        <section className="admin-detail-block">
          <div className="admin-section-heading">
            <div>
              <span>Identity</span>
              <h2>Revision metadata</h2>
            </div>
            <ShieldCheck size={18} />
          </div>
          <Detail label="Full revision hash" value={item.revision_hash} />
          <Detail label="Source Draft" value={item.source_draft_name} />
          <Detail label="Spec digest" value={item.spec_digest} />
          <Detail label="Created at" value={formatDate(item.created_at)} />
        </section>

        <section className="admin-detail-block">
          <div className="admin-section-heading">
            <div>
              <span>Immutable snapshot</span>
              <h2>Agent specification</h2>
            </div>
          </div>
          <CodeBlock
            code={JSON.stringify(item.spec, null, 2)}
            language="json"
            maxHeight={400}
          />
        </section>
      </div>
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
