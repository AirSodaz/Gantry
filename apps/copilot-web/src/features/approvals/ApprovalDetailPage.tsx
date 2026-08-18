import { useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { ArrowLeft, Check, FileCheck2, X } from "lucide-react";
import { Button, CodeBlock, StatusMark } from "@gantry/design-system";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useCopilotApi } from "../../api/ApiProvider";
import { CopilotApiError } from "../../api/client";
import type { Approval } from "../../api/types";
import { ErrorState, LoadingState } from "../../components/AsyncState";
export function ApprovalDetailPage() {
  const { approvalId = "" } = useParams();
  const api = useCopilotApi();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [reason, setReason] = useState("");
  const keys = useRef(new Map<string, string>());
  const detail = useQuery({
    queryKey: ["approval", approvalId],
    queryFn: () => api.getApproval(approvalId),
    enabled: Boolean(approvalId),
  });
  const decide = useMutation({
    mutationFn: (decision: "approve" | "reject") => {
      const key = `${approvalId}:${decision}`;
      let idempotencyKey = keys.current.get(key);
      if (!idempotencyKey) {
        idempotencyKey = crypto.randomUUID();
        keys.current.set(key, idempotencyKey);
      }
      return api.decideApproval(
        approvalId,
        decision,
        detail.data?.action_digest ?? "",
        detail.data?.approval_revision ?? 0,
        reason,
        idempotencyKey,
      );
    },
    onSuccess: async (approval) => {
      queryClient.setQueryData(["approval", approvalId], approval);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["approval", approvalId] }),
        queryClient.invalidateQueries({ queryKey: ["approvals"] }),
        queryClient.invalidateQueries({ queryKey: ["sessions"] }),
      ]);
    },
    onError: async (error) => {
      const currentResource =
        error instanceof CopilotApiError ? error.currentResource : undefined;
      if (!isApproval(currentResource, approvalId)) return;
      queryClient.setQueryData(["approval", approvalId], currentResource);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["approvals"] }),
        queryClient.invalidateQueries({ queryKey: ["sessions"] }),
      ]);
    },
  });

  if (detail.isLoading)
    return (
      <div className="page-wrap">
        <LoadingState label="Loading approval" />
      </div>
    );
  if (detail.isError || !detail.data)
    return (
      <div className="page-wrap">
        <ErrorState
          message={
            detail.error instanceof Error
              ? detail.error.message
              : "This approval could not be loaded."
          }
          onRetry={() => void detail.refetch()}
        />
      </div>
    );
  const approval = detail.data;
  const preview = approval.preview;
  const canDecide =
    approval.state === "pending" &&
    Boolean(approval.action_digest) &&
    (approval.approval_revision ?? 0) > 0;
  const decision = approval.decision;

  return (
    <div className="page-wrap narrow-page approval-detail-page">
      <Link to="/approvals" className="back-link">
        <ArrowLeft size={15} /> Approvals
      </Link>
      <div className="page-heading">
        <div>
          <span className="eyebrow">Action approval</span>
          <h1>
            {preview.tool_display_name ?? "Tool action"} ·{" "}
            {preview.operation_display_name ?? "operation"}
          </h1>
          <p>Review the exact action before it can continue.</p>
        </div>
        <StatusMark status={approval.state} />
      </div>
      <section className="approval-detail-card">
        <div className="approval-detail-row">
          <span>Target</span>
          <strong>{preview.target ?? "No external target declared"}</strong>
        </div>
        <div className="approval-detail-row">
          <span>Effect</span>
          <strong>
            {typeof preview.effect === "string"
              ? preview.effect
              : "write"}
          </strong>
        </div>
        <div className="approval-detail-row">
          <span>Risk</span>
          <StatusMark status={preview.risk_class} />
        </div>
        <div className="approval-detail-row">
          <span>Expires</span>
          <strong>{formatDate(approval.expires_at)}</strong>
        </div>
        <div className="approval-digest">
          <span>Action digest</span>
          <code>{approval.action_digest}</code>
        </div>
        {preview && Object.keys(preview).length > 0 ? (
          <div style={{ marginTop: "12px" }}>
            <span
              style={{
                fontSize: "12px",
                fontWeight: 600,
                color: "var(--ds-text-muted)",
                display: "block",
                marginBottom: "6px",
              }}
            >
              Action parameters preview
            </span>
            <CodeBlock
              code={JSON.stringify(preview, null, 2)}
              language="json"
              maxHeight={240}
            />
          </div>
        ) : null}
        <details className="technical-details">
          <summary>Technical details</summary>
          <dl>
            <div>
              <dt>Policy reason</dt>
              <dd>{preview.policy_reason ?? "Not recorded"}</dd>
            </div>
            <div>
              <dt>Action identity</dt>
              <dd>{approval.action_id}</dd>
            </div>
            <div>
              <dt>Run identity</dt>
              <dd>{approval.run_id}</dd>
            </div>
          </dl>
        </details>
      </section>
      {decision ? (
        <section className="decision-evidence">
          <FileCheck2 size={18} />
          <div>
            <strong>
              {decision.decision === "approve" ? "Approved" : "Rejected"}
            </strong>
            <p>{decision.reason || "No decision reason was provided."}</p>
            <span>{formatDate(decision.decided_at)}</span>
          </div>
        </section>
      ) : null}
      {canDecide ? (
        <section className="decision-form">
          <label htmlFor="approval-reason">
            Decision note <span>(optional)</span>
          </label>
          <textarea
            id="approval-reason"
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            maxLength={2000}
            placeholder="Add context for the session history"
          />
          <div className="approval-actions">
            <Button
              variant="secondary"
              disabled={decide.isPending}
              onClick={() => decide.mutate("approve")}
            >
              <Check size={16} /> Approve action
            </Button>
            <Button
              variant="danger"
              disabled={decide.isPending}
              onClick={() => decide.mutate("reject")}
            >
              <X size={16} /> Reject action
            </Button>
          </div>
        </section>
      ) : null}
      {decide.isError ? (
        <p className="inline-error" role="alert">
          {decide.error instanceof Error
            ? decide.error.message
            : "The approval decision could not be recorded."}
        </p>
      ) : null}
      {approval.session_id ? (
        <Button
          variant="quiet"
          onClick={() => navigate(`/sessions/${approval.session_id}`)}
        >
          Open session
        </Button>
      ) : null}
    </div>
  );
}

function formatDate(value?: string) {
  if (!value) return "Not available";
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

function isApproval(value: unknown, approvalId: string): value is Approval {
  return (
    typeof value === "object" &&
    value !== null &&
    "id" in value &&
    (value as { id?: unknown }).id === approvalId
  );
}
