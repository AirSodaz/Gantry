import { useEffect, useMemo, useRef, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { ArrowLeft, Ban, CheckCircle2, RotateCcw, Send, Timer, XCircle } from 'lucide-react';
import { Button, StatusMark } from '@gantry/design-system';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCopilotApi } from '../../api/ApiProvider';
import type { Artifact } from '../../api/types';
import { ErrorState, LoadingState } from '../../components/AsyncState';

const activeStatuses = new Set(['queued', 'running', 'canceling', 'provisioning', 'awaiting_approval', 'suspended']);

export function TaskPage() {
  const { taskId = '' } = useParams();
  const navigate = useNavigate();
  const api = useCopilotApi();
  const queryClient = useQueryClient();
  const [output, setOutput] = useState('');
  const [followUp, setFollowUp] = useState('');
  const [streamState, setStreamState] = useState<'connecting' | 'connected' | 'reconnecting' | 'closed'>('connecting');
  const cursorRef = useRef('');
  const seenSequences = useRef(new Set<number>());
  const followUpKeyRef = useRef<string | null>(null);
  const cancelKeyRef = useRef<string | null>(null);
  const retryKeyRef = useRef<string | null>(null);
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
      if (!cancelKeyRef.current) cancelKeyRef.current = crypto.randomUUID();
      return api.cancelRun(taskId, runId, cancelKeyRef.current);
    },
    onSuccess: () => {
      cancelKeyRef.current = null;
      void queryClient.invalidateQueries({ queryKey: ['task', taskId] });
    },
  });
  const retryMutation = useMutation({
    mutationFn: () => {
      if (!retryKeyRef.current) retryKeyRef.current = crypto.randomUUID();
      return api.retryTask(taskId, retryKeyRef.current);
    },
    onSuccess: () => {
      retryKeyRef.current = null;
      void queryClient.invalidateQueries({ queryKey: ['task', taskId] });
    },
  });
  const followUpMutation = useMutation({
    mutationFn: () => {
      if (!followUpKeyRef.current) followUpKeyRef.current = crypto.randomUUID();
      return api.appendTaskMessage(taskId, { message: followUp }, followUpKeyRef.current);
    },
    onSuccess: (nextTask) => {
      setFollowUp('');
      followUpKeyRef.current = null;
      void queryClient.setQueryData(['task', taskId], nextTask);
      void queryClient.invalidateQueries({ queryKey: ['task-runs', taskId] });
    },
  });
  const task = taskQuery.data;
  const runsQuery = useQuery({
    queryKey: ['task-runs', taskId],
    queryFn: () => api.listTaskRuns(taskId),
    enabled: Boolean(taskId),
  });
  const approvalsQuery = useQuery({
    queryKey: ['task-approvals', taskId],
    queryFn: () => api.listApprovals(),
    enabled: task?.status === 'awaiting_approval',
    refetchInterval: task?.status === 'awaiting_approval' ? 5000 : false,
  });
  const canCancel = Boolean(task && activeStatuses.has(task.status) && task.current_run?.id && task.status !== 'canceling');
  const canRetry = Boolean(task && (task.status === 'failed' || task.status === 'canceled'));
  const canContinue = task?.status === 'awaiting_requester_input';
  const pendingApproval = approvalsQuery.data?.items.find((approval) => approval.run_id === task?.current_run?.id);
  const timeline = useMemo(() => buildTimeline(task?.status), [task?.status]);

  useEffect(() => {
    if (!taskId) return;
    let cancelled = false;
    let socket: WebSocket | null = null;
    let retryTimer: number | undefined;
    let attempt = 0;
    let terminal = false;

    const connect = async () => {
      if (cancelled) return;
      setStreamState(attempt === 0 ? 'connecting' : 'reconnecting');
      try {
        const ticket = await api.createEventsTicket(taskId);
        if (cancelled) return;
        if (typeof WebSocket === 'undefined') {
          setStreamState('closed');
          return;
        }
        const base = import.meta.env.VITE_COPILOT_API_BASE ?? '/api/copilot/v1';
        const wsOrigin = `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}`;
        const params = new URLSearchParams({ ticket: ticket.ticket });
        if (cursorRef.current) params.set('after', cursorRef.current);
        socket = new WebSocket(`${wsOrigin}${base}/tasks/${encodeURIComponent(taskId)}/events?${params.toString()}`);
        socket.onopen = () => {
          attempt = 0;
          setStreamState('connected');
        };
        socket.onmessage = (message) => {
          try {
            const frame = JSON.parse(message.data as string) as EventFrame;
            if (frame.type === 'cursor_expired') {
              cursorRef.current = '';
              seenSequences.current.clear();
              setOutput('');
              void queryClient.invalidateQueries({ queryKey: ['task', taskId] });
              socket?.close();
              return;
            }
            if (frame.type === 'event' && frame.event) {
              cursorRef.current = frame.cursor ?? cursorRef.current;
              if (seenSequences.current.has(frame.event.sequence)) return;
              seenSequences.current.add(frame.event.sequence);
              const payload = frame.event.payload as Record<string, unknown>;
              if (frame.event.type === 'model.delta' && typeof payload.text === 'string') {
                setOutput((current) => current + payload.text);
              }
              void queryClient.invalidateQueries({ queryKey: ['task', taskId] });
              if (isTerminalEvent(frame.event.type)) {
                terminal = true;
                socket?.close();
              }
            }
          } catch {
            // Ignore malformed frames; the next state poll remains authoritative.
          }
        };
        socket.onclose = () => {
          if (cancelled || terminal) {
            setStreamState('closed');
            return;
          }
          attempt += 1;
          retryTimer = window.setTimeout(connect, Math.min(10_000, 500 * 2 ** Math.min(attempt, 5)));
        };
        socket.onerror = () => socket?.close();
      } catch {
        attempt += 1;
        retryTimer = window.setTimeout(connect, Math.min(10_000, 500 * 2 ** Math.min(attempt, 5)));
      }
    };
    void connect();
    return () => {
      cancelled = true;
      if (retryTimer !== undefined) window.clearTimeout(retryTimer);
      socket?.close();
      setStreamState('closed');
    };
  }, [api, queryClient, taskId]);

  if (taskQuery.isLoading) return <div className="page-wrap"><LoadingState label="Loading task" /></div>;
  if (taskQuery.isError || !task) return <div className="page-wrap"><ErrorState message={taskQuery.error instanceof Error ? taskQuery.error.message : 'This task could not be loaded.'} onRetry={() => void taskQuery.refetch()} /></div>;

  return (
    <div className="page-wrap task-page">
      <Link to="/tasks" className="back-link"><ArrowLeft size={15} /> My tasks</Link>
      <div className="task-workbench">
        <section className="conversation-panel">
          <div className="conversation-heading"><div><span className="eyebrow">Task detail</span><h1>{task.agent_display_name ?? task.agent_id}</h1><p>Task {task.id}</p></div><StatusMark status={task.status} /></div>
          <div className="conversation-messages" aria-label="Task conversation">
            {task.messages?.map((message) => <div className={`request-message ${message.role === 'agent' ? 'agent-message' : ''}`} key={message.id}><span className="message-label">{message.role === 'agent' ? 'Agent' : 'Your request'}</span><p>{message.content}</p></div>)}
            {!task.messages?.length ? <div className="request-message"><span className="message-label">Your request</span><p>This task was submitted to {task.agent_display_name ?? task.agent_id}.</p></div> : null}
          </div>
          <div className="run-output" aria-live="polite">
            <div className="output-heading"><span>Live output</span><span className={`stream-state ${streamState}`}>{streamState}</span></div>
            <pre>{output || 'Waiting for runner output…'}</pre>
          </div>
          <div className="timeline" aria-label="Task timeline">
            {timeline.map(({ label, status, Icon }) => <div className={`timeline-item ${status}`} key={label}><span className="timeline-icon"><Icon size={16} /></span><span>{label}</span></div>)}
          </div>
          {task.current_run?.status_reason ? <div className="reason-box"><strong>Run note</strong><p>{task.current_run.status_reason}</p></div> : null}
          {canContinue ? <form className="follow-up-composer" onSubmit={(event) => { event.preventDefault(); if (followUp.trim()) followUpMutation.mutate(); }}><label htmlFor="task-follow-up">Continue this task</label><textarea id="task-follow-up" value={followUp} onChange={(event) => setFollowUp(event.target.value)} placeholder="Describe what should change before the next attempt" maxLength={8000} disabled={followUpMutation.isPending} /><Button type="submit" disabled={!followUp.trim() || followUpMutation.isPending}><Send size={16} /> {followUpMutation.isPending ? 'Starting…' : 'Continue'}</Button></form> : null}
          {cancelMutation.isError || retryMutation.isError || followUpMutation.isError ? <p className="inline-error" role="alert">{(cancelMutation.error ?? retryMutation.error ?? followUpMutation.error) instanceof Error ? (cancelMutation.error ?? retryMutation.error ?? followUpMutation.error)?.message : 'The command could not be completed.'}</p> : null}
          <div className="task-actions">
            <Button variant="danger" disabled={!canCancel || cancelMutation.isPending} onClick={() => cancelMutation.mutate()}><Ban size={16} /> {cancelMutation.isPending ? 'Canceling…' : 'Cancel run'}</Button>
            {canRetry ? <Button variant="secondary" disabled={retryMutation.isPending} onClick={() => retryMutation.mutate()}><RotateCcw size={16} /> {retryMutation.isPending ? 'Retrying…' : 'Retry task'}</Button> : null}
            {pendingApproval ? <Button variant="secondary" onClick={() => navigate(`/approvals/${pendingApproval.id}`)}>Open approval</Button> : null}
            <Button variant="quiet" onClick={() => navigate('/tasks')}>Back to history</Button>
          </div>
          {task.artifacts?.length ? <div className="artifact-list"><h2>Artifacts</h2>{task.artifacts.map((artifact) => <ArtifactRow key={artifact.id} artifact={artifact} onDownload={async () => {
            const result = await api.getArtifact(artifact.id);
            if (result.download_url) window.open(result.download_url, '_blank', 'noopener,noreferrer');
          }} />)}</div> : null}
          {runsQuery.data?.items.length ? <section className="run-attempts"><h2>Run attempts</h2>{runsQuery.data.items.map((run) => <div className="run-attempt-row" key={run.id}><span>Attempt {run.attempt_number}</span><StatusMark status={run.status} /><span>{run.status_reason || formatDate(run.completed_at ?? run.started_at ?? run.created_at)}</span></div>)}</section> : null}
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

type EventFrame = {
  type: string;
  cursor?: string;
  event?: { sequence: number; type: string; payload: unknown };
};

function ArtifactRow({ artifact, onDownload }: { artifact: Artifact; onDownload: () => Promise<void> }) {
  const [downloadError, setDownloadError] = useState(false);
  const available = artifact.state === 'available' && artifact.scan_status === 'passed';
  const download = async () => {
    try {
      setDownloadError(false);
      await onDownload();
    } catch {
      setDownloadError(true);
    }
  };
  return <div className="artifact-row"><div><strong>{artifact.filename ?? artifact.id}</strong><span>{artifact.media_type ?? 'Artifact'} · {formatBytes(artifact.size_bytes ?? 0)} · {available ? 'Ready' : 'Processing'}</span>{downloadError ? <span className="artifact-error" role="alert">Download failed. Try again.</span> : null}</div><Button variant="secondary" disabled={!available} onClick={() => void download()}>{downloadError ? 'Retry download' : 'Download'}</Button></div>;
}

function isTerminalEvent(type: string) {
  return type === 'run.completed' || type === 'run.failed' || type === 'run.canceled';
}

function formatBytes(value: number) {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${Math.round(value / 1024)} KB`;
  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
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
