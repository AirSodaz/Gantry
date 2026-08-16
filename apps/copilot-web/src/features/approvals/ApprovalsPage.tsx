import { Link } from 'react-router-dom';
import { ArrowUpRight, FileCheck2 } from 'lucide-react';
import { StatusMark } from '@gantry/design-system';
import { useQuery } from '@tanstack/react-query';
import { useCopilotApi } from '../../api/ApiProvider';
import { ErrorState, LoadingState } from '../../components/AsyncState';

export function ApprovalsPage() {
  const api = useCopilotApi();
  const query = useQuery({ queryKey: ['approvals'], queryFn: () => api.listApprovals(), refetchInterval: 5000 });

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
        return <Link className="approval-row" key={id} to={`/approvals/${encodeURIComponent(id)}`}><div className="approval-copy"><div className="approval-title"><strong>{toolName} · {operation}</strong><StatusMark status={riskClass} /></div><p>{target}</p><code>{digest}</code><span>Expires {new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(expiresAt))}</span></div><ArrowUpRight size={17} aria-hidden="true" /></Link>;
      })}</div>}
    </div>
  );
}
