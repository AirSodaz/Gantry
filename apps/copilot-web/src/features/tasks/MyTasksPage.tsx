import { useState } from 'react';
import { Link } from 'react-router-dom';
import { ArrowUpRight, ListTodo } from 'lucide-react';
import { StatusMark } from '@gantry/design-system';
import { useQuery } from '@tanstack/react-query';
import { useCopilotApi } from '../../api/ApiProvider';
import { EmptyState, ErrorState, LoadingState } from '../../components/AsyncState';

const filters = ['', 'queued', 'running', 'completed', 'failed', 'canceled'];

export function MyTasksPage() {
  const api = useCopilotApi();
  const [filter, setFilter] = useState('');
  const query = useQuery({ queryKey: ['tasks', filter], queryFn: () => api.listTasks(filter) });
  return (
    <div className="page-wrap narrow-page">
      <div className="page-heading page-heading-inline"><div><span className="eyebrow">History</span><h1>My tasks</h1><p>Every request submitted from your account.</p></div><label className="filter-field"><span>Filter</span><select value={filter} onChange={(event) => setFilter(event.target.value)}>{filters.map((value) => <option key={value} value={value}>{value || 'All statuses'}</option>)}</select></label></div>
      {query.isLoading ? <LoadingState label="Loading tasks" /> : null}
      {query.isError ? <ErrorState message={query.error instanceof Error ? query.error.message : 'Your tasks could not be loaded.'} onRetry={() => void query.refetch()} /> : null}
      {!query.isLoading && !query.isError && query.data?.items.length === 0 ? <EmptyState title="No tasks yet" detail="Start with a new task when you are ready." /> : null}
      <div className="task-list" aria-label="My tasks">
        {query.data?.items.map((task) => (
          <Link to={`/tasks/${task.id}`} className="task-row" key={task.id}>
            <span className="task-row-icon" aria-hidden="true"><ListTodo size={17} /></span>
            <span className="task-row-copy"><strong>{task.agent_display_name ?? task.agent_id}</strong><span>{formatDate(task.created_at)}</span></span>
            <StatusMark status={task.status} />
            <ArrowUpRight size={16} aria-hidden="true" />
          </Link>
        ))}
      </div>
    </div>
  );
}

function formatDate(value?: string) {
  if (!value) return 'Recently submitted';
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value));
}
