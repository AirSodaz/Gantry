import { useMemo } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { ArrowUpRight, CheckCircle2, Clock, Filter, ListTodo, XCircle } from 'lucide-react';
import { Button, Select, type SelectOption, StatusMark } from '@gantry/design-system';
import { useInfiniteQuery, useQuery } from '@tanstack/react-query';
import { useCopilotApi } from '../../api/ApiProvider';
import { EmptyState, ErrorState, LoadingState } from '../../components/AsyncState';

const STATUS_OPTIONS: SelectOption[] = [
  { value: '', label: 'All statuses', icon: <Filter size={13} /> },
  { value: 'queued', label: 'Queued', icon: <Clock size={13} /> },
  { value: 'running', label: 'Running', icon: <Clock size={13} /> },
  { value: 'completed', label: 'Completed', icon: <CheckCircle2 size={13} /> },
  { value: 'failed', label: 'Failed', icon: <XCircle size={13} /> },
  { value: 'canceled', label: 'Canceled', icon: <XCircle size={13} /> },
  { value: 'awaiting_approval', label: 'Awaiting approval', icon: <Clock size={13} /> },
  { value: 'awaiting_requester_input', label: 'Needs input', icon: <Clock size={13} /> },
];

const ACTION_OPTIONS: SelectOption[] = [
  { value: '', label: 'All requester actions' },
  { value: 'approval', label: 'Approval needed' },
  { value: 'input', label: 'Input needed' },
];

const TIME_OPTIONS: SelectOption[] = [
  { value: '', label: 'Any time' },
  { value: '7d', label: 'Last 7 days' },
  { value: '30d', label: 'Last 30 days' },
];

export function MyTasksPage() {
  const api = useCopilotApi();
  const [params, setParams] = useSearchParams();
  const status = params.get('status') ?? '';
  const agentId = params.get('agent_id') ?? '';
  const requesterAction = params.get('requester_action') ?? '';
  const timeRange = params.get('time_range') ?? '';
  const createdAfter = useMemo(() => rangeStart(timeRange), [timeRange]);
  const agentsQuery = useQuery({ queryKey: ['agents', 'task-history'], queryFn: () => api.listAgents() });
  const agentOptions = useMemo<SelectOption[]>(() => [
    { value: '', label: 'All agents' },
    ...(agentsQuery.data?.items ?? []).map((agent) => ({ value: agent.id, label: agent.display_name })),
  ], [agentsQuery.data]);
  const query = useInfiniteQuery({
    queryKey: ['tasks', status, agentId, requesterAction, timeRange],
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) => api.listTasks({ status, agentId, requesterAction, createdAfter, cursor: pageParam }),
		getNextPageParam: (page) => page.page_info?.has_more ? page.page_info.next_cursor : undefined,
  });
	const items = query.data?.pages.flatMap((page) => page.items) ?? [];
  const setFilter = (key: 'status' | 'agent_id' | 'requester_action' | 'time_range', value: string) => {
    const next = new URLSearchParams(params);
    if (value) next.set(key, value); else next.delete(key);
    setParams(next, { replace: true });
  };

  return (
    <div className="page-wrap narrow-page">
      <div className="page-heading page-heading-inline">
        <div>
          <span className="eyebrow">History</span>
          <h1>My tasks</h1>
          <p>Every request submitted from your account.</p>
        </div>

        <div className="history-filter-wrap">
          <Select
            label="Status"
            options={STATUS_OPTIONS}
            value={status}
            onChange={(value) => setFilter('status', value)}
            placeholder="All statuses"
          />
          <Select label="Agent" options={agentOptions} value={agentId} onChange={(value) => setFilter('agent_id', value)} placeholder="All agents" />
          <Select label="Requester action" options={ACTION_OPTIONS} value={requesterAction} onChange={(value) => setFilter('requester_action', value)} placeholder="All requester actions" />
          <Select label="Time" options={TIME_OPTIONS} value={timeRange} onChange={(value) => setFilter('time_range', value)} placeholder="Any time" />
        </div>
      </div>

      {query.isLoading ? <LoadingState label="Loading tasks" /> : null}
      {query.isError ? (
        <ErrorState
          message={
            query.error instanceof Error ? query.error.message : 'Your tasks could not be loaded.'
          }
          onRetry={() => void query.refetch()}
        />
      ) : null}
      {!query.isLoading && !query.isError && items.length === 0 ? (
        <EmptyState
          title="No tasks yet"
          detail={status || agentId || requesterAction || timeRange ? 'No tasks match the selected filters.' : 'Start with a new task when you are ready.'}
        />
      ) : null}

      <div className="task-list" aria-label="My tasks">
        {items.map((task) => (
          <Link to={`/tasks/${task.id}`} className="task-row" key={task.id}>
            <span className="task-row-icon" aria-hidden="true">
              <ListTodo size={17} />
            </span>
            <span className="task-row-copy">
              <strong>{task.agent_display_name ?? task.agent_id}</strong>
              <span>{formatDate(task.created_at)}</span>
            </span>
            <StatusMark status={task.status} />
            <ArrowUpRight size={16} className="task-row-arrow" aria-hidden="true" />
          </Link>
        ))}
      </div>
		{query.hasNextPage ? <div className="list-more"><Button variant="secondary" onClick={() => void query.fetchNextPage()} disabled={query.isFetchingNextPage}>{query.isFetchingNextPage ? 'Loading...' : 'Load more'}</Button></div> : null}
    </div>
  );
}

function rangeStart(range: string) {
  if (range !== '7d' && range !== '30d') return undefined;
  const date = new Date();
  date.setDate(date.getDate() - (range === '7d' ? 7 : 30));
  return date.toISOString();
}

function formatDate(value?: string) {
  if (!value) return 'Recently submitted';
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(
    new Date(value)
  );
}
