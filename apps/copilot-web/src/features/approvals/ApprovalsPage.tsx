import { Link } from 'react-router-dom';
import { ArrowUpRight, FileCheck2 } from 'lucide-react';
import { Button, StatusMark } from '@gantry/design-system';
import { useInfiniteQuery } from '@tanstack/react-query';
import { useCopilotApi } from '../../api/ApiProvider';
import { ErrorState, LoadingState } from '../../components/AsyncState';

export function ApprovalsPage() {
  const api = useCopilotApi();
  const query = useInfiniteQuery({ queryKey: ['approvals'], initialPageParam: '', queryFn: ({ pageParam }) => api.listApprovals(pageParam), getNextPageParam: (page) => page.page_info?.has_more ? page.page_info.next_cursor : undefined, refetchInterval: 5000 });

  if (query.isLoading) return <div className="page-wrap"><LoadingState label="Loading approvals" /></div>;
  if (query.isError) return <div className="page-wrap"><ErrorState message="Approvals could not be loaded." onRetry={() => void query.refetch()} /></div>;
  const items = query.data?.pages.flatMap((page) => page.items) ?? [];
  return (
    <div className="page-wrap narrow-page">
      <div className="page-heading"><div><span className="eyebrow">Action approvals</span><h1>Review actions before they run.</h1><p>{items.length === 1 ? '1 action is waiting for your decision.' : `${items.length} actions are waiting for your decision.`}</p></div></div>
      {items.length === 0 ? <div className="state-empty"><FileCheck2 size={24} /><strong>No pending actions</strong><p>New approval requests will appear here when a task reaches a governed write step.</p></div> : <div className="approval-list">{items.map((item) => {
        const target = item.target ?? 'No external target declared';
        const expiresAt = item.expires_at ?? new Date().toISOString();
		const agentName = item.agent_display_name ?? 'Agent';
        const digest = item.action_digest ?? '';
        const id = item.id ?? '';
        const toolName = item.tool_name ?? 'Tool action';
        const operation = item.operation ?? 'operation';
        const riskClass = item.risk_class ?? 'write';
		return <div className="approval-row" key={id}><Link className="approval-row-main" to={`/approvals/${encodeURIComponent(id)}`}><div className="approval-copy"><div className="approval-title"><strong>{toolName} · {operation}</strong><StatusMark status={riskClass} /></div><p>{agentName} · {target}</p><code>{digest}</code><span>Requested {formatAge(item.created_at)} · Expires {new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(expiresAt))}</span></div><ArrowUpRight size={17} aria-hidden="true" /></Link>{item.task_id ? <Link className="approval-task-link" to={`/tasks/${encodeURIComponent(item.task_id)}`}>Open task</Link> : null}</div>;
      })}</div>}
		{query.hasNextPage ? <div className="list-more"><Button variant="secondary" onClick={() => void query.fetchNextPage()} disabled={query.isFetchingNextPage}>{query.isFetchingNextPage ? 'Loading...' : 'Load more'}</Button></div> : null}
    </div>
  );
}

function formatAge(value?: string) {
	if (!value) return 'recently';
	const minutes = Math.max(0, Math.floor((Date.now() - new Date(value).getTime()) / 60_000));
	if (minutes < 1) return 'just now';
	if (minutes < 60) return `${minutes}m ago`;
	const hours = Math.floor(minutes / 60);
	if (hours < 24) return `${hours}h ago`;
	return `${Math.floor(hours / 24)}d ago`;
}
