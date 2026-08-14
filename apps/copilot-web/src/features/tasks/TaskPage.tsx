import { useMemo } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { ArrowLeft, Ban, CheckCircle2, RotateCcw, Timer, XCircle } from 'lucide-react';
import { Button, StatusMark } from '@gantry/design-system';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCopilotApi } from '../../api/ApiProvider';
import { ErrorState, LoadingState } from '../../components/AsyncState';

const activeStatuses = new Set(['queued', 'running', 'canceling', 'provisioning', 'awaiting_approval', 'suspended']);

export function TaskPage() {
  const { taskId = '' } = useParams();
  const navigate = useNavigate();
  const api = useCopilotApi();
  const queryClient = useQueryClient();
  const taskQuery = useQuery({
    queryKey: ['task', taskId],
    queryFn: () => api.getTask(taskId),
    enabled: Boolean(taskId),
    refetchInterval: (query) => activeStatuses.has(query.state.data?.status ?? '') ? 1000 : false,
  });
  const cancelMutation = useMutation({
    mutationFn: () => {
      const runId = taskQuery.data?.current_run?.id;
      if (!runId) throw new Error('This task has no active run.');
      return api.cancelRun(taskId, runId);
    },
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['task', taskId] }),
  });
  const retryMutation = useMutation({
    mutationFn: () => api.retryTask(taskId),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: ['task', taskId] }),
  });
  const task = taskQuery.data;
  const canCancel = Boolean(task && activeStatuses.has(task.status) && task.current_run?.id && task.status !== 'canceling');
  const canRetry = Boolean(task && (task.status === 'failed' || task.status === 'canceled'));
  const timeline = useMemo(() => buildTimeline(task?.status), [task?.status]);

  if (taskQuery.isLoading) return <div className="page-wrap"><LoadingState label="Loading task" /></div>;
  if (taskQuery.isError || !task) return <div className="page-wrap"><ErrorState message={taskQuery.error instanceof Error ? taskQuery.error.message : 'This task could not be loaded.'} onRetry={() => void taskQuery.refetch()} /></div>;

  return (
    <div className="page-wrap task-page">
      <Link to="/tasks" className="back-link"><ArrowLeft size={15} /> My tasks</Link>
      <div className="task-workbench">
        <section className="conversation-panel">
          <div className="conversation-heading"><div><span className="eyebrow">Task detail</span><h1>{task.agent_display_name ?? task.agent_id}</h1><p>Task {task.id}</p></div><StatusMark status={task.status} /></div>
          <div className="request-message"><span className="message-label">Your request</span><p>This task was submitted to {task.agent_display_name ?? task.agent_id}.</p></div>
          <div className="timeline" aria-label="Task timeline">
            {timeline.map(({ label, status, Icon }) => <div className={`timeline-item ${status}`} key={label}><span className="timeline-icon"><Icon size={16} /></span><span>{label}</span></div>)}
          </div>
          {task.current_run?.status_reason ? <div className="reason-box"><strong>Run note</strong><p>{task.current_run.status_reason}</p></div> : null}
          {cancelMutation.isError || retryMutation.isError ? <p className="inline-error" role="alert">{(cancelMutation.error ?? retryMutation.error) instanceof Error ? (cancelMutation.error ?? retryMutation.error)?.message : 'The command could not be completed.'}</p> : null}
          <div className="task-actions">
            <Button variant="danger" disabled={!canCancel || cancelMutation.isPending} onClick={() => cancelMutation.mutate()}><Ban size={16} /> {cancelMutation.isPending ? 'Canceling…' : 'Cancel run'}</Button>
            {canRetry ? <Button variant="secondary" disabled={retryMutation.isPending} onClick={() => retryMutation.mutate()}><RotateCcw size={16} /> {retryMutation.isPending ? 'Retrying…' : 'Retry task'}</Button> : null}
            <Button variant="quiet" onClick={() => navigate('/tasks')}>Back to history</Button>
          </div>
        </section>
        <aside className="run-inspector">
          <span className="context-kicker">Run inspector</span>
          <div className="inspector-row"><span>Run</span><strong>{task.current_run?.id ?? 'Not assigned'}</strong></div>
          <div className="inspector-row"><span>State</span><StatusMark status={task.current_run?.status ?? task.status} /></div>
          <div className="inspector-row"><span>Created</span><strong>{formatDate(task.created_at)}</strong></div>
          <div className="inspector-note"><Timer size={16} /><span>State refreshes automatically while the run is active.</span></div>
        </aside>
      </div>
    </div>
  );
}

function buildTimeline(status?: string) {
  const completed = status === 'completed';
  const failed = status === 'failed';
  const canceled = status === 'canceled';
  return [
    { label: 'Task accepted', status: 'done', Icon: CheckCircle2 },
    { label: 'Run assigned', status: activeStatuses.has(status ?? '') || completed || failed || canceled ? 'done' : 'pending', Icon: CheckCircle2 },
    { label: completed ? 'Run completed' : failed ? 'Run failed' : canceled ? 'Run canceled' : 'Run in progress', status: completed || failed || canceled ? 'done' : 'current', Icon: completed ? CheckCircle2 : failed ? XCircle : Timer },
  ];
}

function formatDate(value?: string) {
  if (!value) return 'Recently';
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value));
}
