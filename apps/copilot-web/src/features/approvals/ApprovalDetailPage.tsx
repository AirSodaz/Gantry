import { useRef, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { ArrowLeft, Check, FileCheck2, X } from 'lucide-react';
import { Button, StatusMark } from '@gantry/design-system';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCopilotApi } from '../../api/ApiProvider';
import { ErrorState, LoadingState } from '../../components/AsyncState';

export function ApprovalDetailPage() {
  const { approvalId = '' } = useParams();
  const api = useCopilotApi();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [reason, setReason] = useState('');
  const keys = useRef(new Map<string, string>());
  const detail = useQuery({ queryKey: ['approval', approvalId], queryFn: () => api.getApproval(approvalId), enabled: Boolean(approvalId) });
  const decide = useMutation({
    mutationFn: (decision: 'approve' | 'reject') => {
      const key = `${approvalId}:${decision}`;
      let idempotencyKey = keys.current.get(key);
      if (!idempotencyKey) {
        idempotencyKey = crypto.randomUUID();
        keys.current.set(key, idempotencyKey);
      }
      return api.decideApproval(approvalId, decision, detail.data?.action_digest ?? '', reason, idempotencyKey);
    },
    onSuccess: async () => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['approval', approvalId] }),
        queryClient.invalidateQueries({ queryKey: ['approvals'] }),
        queryClient.invalidateQueries({ queryKey: ['tasks'] }),
      ]);
    },
  });

  if (detail.isLoading) return <div className="page-wrap"><LoadingState label="Loading approval" /></div>;
  if (detail.isError || !detail.data) return <div className="page-wrap"><ErrorState message={detail.error instanceof Error ? detail.error.message : 'This approval could not be loaded.'} onRetry={() => void detail.refetch()} /></div>;
  const approval = detail.data;
  const preview = approval.action_preview ?? {};
  const canDecide = approval.status === 'pending' && Boolean(approval.action_digest);
  const decision = approval.latest_decision;

  return <div className="page-wrap narrow-page approval-detail-page">
    <Link to="/approvals" className="back-link"><ArrowLeft size={15} /> Approvals</Link>
    <div className="page-heading"><div><span className="eyebrow">Action approval</span><h1>{approval.tool_name ?? 'Tool action'} · {approval.operation ?? 'operation'}</h1><p>Review the exact action before it can continue.</p></div><StatusMark status={approval.status} /></div>
    <section className="approval-detail-card">
      <div className="approval-detail-row"><span>Target</span><strong>{approval.target ?? 'No external target declared'}</strong></div>
      <div className="approval-detail-row"><span>Effect</span><strong>{typeof preview.effect === 'string' ? preview.effect : approval.effect ?? 'write'}</strong></div>
      <div className="approval-detail-row"><span>Risk</span><StatusMark status={approval.risk_class ?? 'write'} /></div>
      <div className="approval-detail-row"><span>Expires</span><strong>{formatDate(approval.expires_at)}</strong></div>
      <div className="approval-digest"><span>Action digest</span><code>{approval.action_digest}</code></div>
      <details className="technical-details"><summary>Technical details</summary><dl><div><dt>Policy version</dt><dd>{approval.policy_version ?? 'Not recorded'}</dd></div><div><dt>Action identity</dt><dd>{approval.action_id}</dd></div><div><dt>Run identity</dt><dd>{approval.run_id}</dd></div></dl></details>
    </section>
    {decision ? <section className="decision-evidence"><FileCheck2 size={18} /><div><strong>{decision.decision === 'approve' ? 'Approved' : 'Rejected'}</strong><p>{decision.reason || 'No decision reason was provided.'}</p><span>{formatDate(decision.created_at)}</span></div></section> : null}
    {canDecide ? <section className="decision-form"><label htmlFor="approval-reason">Decision note <span>(optional)</span></label><textarea id="approval-reason" value={reason} onChange={(event) => setReason(event.target.value)} maxLength={2000} placeholder="Add context for the task history" /><div className="approval-actions"><Button variant="secondary" disabled={decide.isPending} onClick={() => decide.mutate('approve')}><Check size={16} /> Approve action</Button><Button variant="danger" disabled={decide.isPending} onClick={() => decide.mutate('reject')}><X size={16} /> Reject action</Button></div></section> : null}
    {decide.isError ? <p className="inline-error" role="alert">{decide.error instanceof Error ? decide.error.message : 'The approval decision could not be recorded.'}</p> : null}
    {approval.task_id ? <Button variant="quiet" onClick={() => navigate(`/tasks/${approval.task_id}`)}>Open task</Button> : null}
  </div>;
}

function formatDate(value?: string) {
  if (!value) return 'Not available';
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value));
}
