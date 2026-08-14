import { useRef, useState } from 'react';
import { Check, FileCheck2, X } from 'lucide-react';
import { Button, StatusMark } from '@gantry/design-system';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCopilotApi } from '../../api/ApiProvider';
import { ErrorState, LoadingState } from '../../components/AsyncState';

export function ApprovalsPage() {
  const api = useCopilotApi();
  const queryClient = useQueryClient();
  const [busy, setBusy] = useState<string | null>(null);
  const decisionKeys = useRef(new Map<string, string>());
  const query = useQuery({ queryKey: ['approvals'], queryFn: () => api.listApprovals(), refetchInterval: 5000 });
  const decide = useMutation({
    mutationFn: ({ id, digest, decision }: { id: string; digest: string; decision: 'approve' | 'reject' }) => {
      const key = `${id}:${decision}`;
      let idempotencyKey = decisionKeys.current.get(key);
      if (!idempotencyKey) {
        idempotencyKey = crypto.randomUUID();
        decisionKeys.current.set(key, idempotencyKey);
      }
      return api.decideApproval(id, decision, digest, '', idempotencyKey);
    },
    onMutate: ({ id }) => setBusy(id),
    onSettled: () => { setBusy(null); void queryClient.invalidateQueries({ queryKey: ['approvals'] }); void queryClient.invalidateQueries({ queryKey: ['tasks'] }); },
  });

  if (query.isLoading) return <div className="page-wrap"><LoadingState label="Loading approvals" /></div>;
  if (query.isError) return <div className="page-wrap"><ErrorState message="Approvals could not be loaded." onRetry={() => void query.refetch()} /></div>;
  const items = query.data?.items ?? [];
  return (
    <div className="page-wrap narrow-page">
      <div className="page-heading"><div><span className="eyebrow">Action approvals</span><h1>Review actions before they run.</h1><p>These requests are bound to one run, tool operation, and action digest.</p></div></div>
      {items.length === 0 ? <div className="state-empty"><FileCheck2 size={24} /><strong>No pending actions</strong><p>New approval requests will appear here when a task reaches a governed write step.</p></div> : <div className="approval-list">{items.map((item) => {
        const target = item.target ?? 'No external target declared';
        const expiresAt = item.expires_at ?? new Date().toISOString();
        const digest = item.action_digest ?? '';
        const id = item.id ?? '';
        const toolName = item.tool_name ?? 'Tool action';
        const operation = item.operation ?? 'operation';
        const riskClass = item.risk_class ?? 'write';
        return <article className="approval-row" key={id}><div className="approval-copy"><div className="approval-title"><strong>{toolName} · {operation}</strong><StatusMark status={riskClass} /></div><p>{target}</p><code>{digest}</code><span>Expires {new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(expiresAt))}</span></div><div className="approval-actions"><Button variant="secondary" disabled={busy === id || !digest} onClick={() => decide.mutate({ id, digest, decision: 'approve' })}><Check size={15} /> Approve</Button><Button variant="danger" disabled={busy === id || !digest} onClick={() => decide.mutate({ id, digest, decision: 'reject' })}><X size={15} /> Reject</Button></div></article>;
      })}</div>}
      {decide.isError ? <p className="inline-error" role="alert">{decide.error instanceof Error ? decide.error.message : 'The approval decision could not be recorded.'}</p> : null}
    </div>
  );
}
