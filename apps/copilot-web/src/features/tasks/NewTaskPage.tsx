import { FormEvent, useMemo, useRef, useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { ArrowUp, Info, Sparkles } from 'lucide-react';
import { Button } from '@gantry/design-system';
import { useMutation } from '@tanstack/react-query';
import { useCopilotApi } from '../../api/ApiProvider';
import { AgentPicker } from '../catalog/AgentPicker';
import type { Agent, SubmitTaskInput } from '../../api/types';
import { getSubmissionKey } from './submission';

export function NewTaskPage() {
  const api = useCopilotApi();
  const navigate = useNavigate();
  const location = useLocation();
  const [selectedAgent, setSelectedAgent] = useState<Agent | null>(null);
  const [message, setMessage] = useState('');
  const [holdOpen, setHoldOpen] = useState(false);
  const pendingSubmission = useRef<{ key: string; signature: string } | null>(null);
  const queryAgentId = new URLSearchParams(location.search).get('agent');
  const input = useMemo<SubmitTaskInput | null>(() => {
    if (!selectedAgent || !message.trim()) return null;
    return {
      agent_id: selectedAgent.id,
      message: message.trim(),
      ...(import.meta.env.DEV && holdOpen ? { structured_input: { mode: 'await_cancel' } } : {}),
    };
  }, [holdOpen, message, selectedAgent]);
  const mutation = useMutation({
    mutationFn: ({ taskInput, idempotencyKey }: { taskInput: SubmitTaskInput; idempotencyKey: string }) => api.submitTask(taskInput, idempotencyKey),
    onSuccess: (task) => {
      pendingSubmission.current = null;
      navigate(`/tasks/${task.id}`);
    },
  });

  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (!input) return;
    const nextKey = getSubmissionKey(input, pendingSubmission.current);
    pendingSubmission.current = nextKey;
    mutation.mutate({ taskInput: input, idempotencyKey: nextKey.key });
  };

  return (
    <div className="page-wrap workbench-page">
      <div className="page-heading">
        <div><span className="eyebrow">Workspace</span><h1>What would you like to accomplish?</h1><p>Choose an approved agent, describe the outcome, and Gantry will keep the run state visible.</p></div>
      </div>
      <div className="workbench-grid">
        <div className="composer-column">
          <AgentPicker selectedId={selectedAgent?.id ?? queryAgentId ?? ''} onSelect={setSelectedAgent} />
          <form className="composer" onSubmit={submit}>
            <div className="composer-heading"><div className="composer-icon"><Sparkles size={18} /></div><div><h2>Describe your task</h2><p>Be clear about the result you need.</p></div></div>
            <label className="composer-input-label" htmlFor="task-message">Task message</label>
            <textarea id="task-message" value={message} onChange={(event) => setMessage(event.target.value)} placeholder="For example: summarize the latest customer feedback into three themes." rows={5} />
            {import.meta.env.DEV ? <label className="dev-toggle"><input type="checkbox" checked={holdOpen} onChange={(event) => setHoldOpen(event.target.checked)} /> Keep run open for cancellation testing</label> : null}
            {mutation.isError ? <p className="inline-error" role="alert">{mutation.error instanceof Error ? mutation.error.message : 'The task could not be submitted.'} Submit again to retry with the same idempotency key.</p> : null}
            <div className="composer-footer"><span className="composer-hint"><Info size={14} /> Your task is private to your account.</span><Button type="submit" disabled={!input || mutation.isPending}>{mutation.isPending ? 'Submitting…' : mutation.isError ? 'Retry submission' : 'Start task'} <ArrowUp size={16} /></Button></div>
          </form>
        </div>
        <aside className="context-panel" aria-label="Task guidance">
          <div className="context-panel-top"><span className="context-kicker">How it works</span><h2>A focused workspace for approved capabilities.</h2></div>
          <ol className="steps-list"><li><span>01</span><div><strong>Choose an agent</strong><p>Each capability is published for your workspace.</p></div></li><li><span>02</span><div><strong>State the outcome</strong><p>Keep the request focused on the result you need.</p></div></li><li><span>03</span><div><strong>Follow the run</strong><p>Return anytime to see the current durable status.</p></div></li></ol>
        </aside>
      </div>
    </div>
  );
}
