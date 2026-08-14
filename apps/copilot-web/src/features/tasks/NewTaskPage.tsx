import { FormEvent, useMemo, useRef, useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import {
  ArrowUp,
  Bot,
  FileCode2,
  HelpCircle,
  Info,
  Lightbulb,
  ShieldAlert,
  Sparkles,
  Zap,
} from 'lucide-react';
import { useMutation } from '@tanstack/react-query';
import { useCopilotApi } from '../../api/ApiProvider';
import { AgentPicker } from '../catalog/AgentPicker';
import type { Agent, SubmitTaskInput } from '../../api/types';
import { getSubmissionKey } from './submission';

const STARTER_PROMPTS = [
  {
    icon: Sparkles,
    title: 'Summarize feedback',
    prompt: 'Summarize the latest customer feedback into three actionable themes.',
  },
  {
    icon: Zap,
    title: 'Optimize performance',
    prompt: 'Audit the execution pipeline and suggest optimizations to reduce latency.',
  },
  {
    icon: FileCode2,
    title: 'Generate test matrix',
    prompt: 'Generate an end-to-end test matrix covering edge cases and failover recovery.',
  },
  {
    icon: ShieldAlert,
    title: 'Security review',
    prompt: 'Review API boundaries and access controls for potential privilege escalation paths.',
  },
];

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
    mutationFn: ({
      taskInput,
      idempotencyKey,
    }: {
      taskInput: SubmitTaskInput;
      idempotencyKey: string;
    }) => api.submitTask(taskInput, idempotencyKey),
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

  const handleKeyDown = (event: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === 'Enter' && (event.metaKey || event.ctrlKey)) {
      event.preventDefault();
      if (input && !mutation.isPending) {
        const nextKey = getSubmissionKey(input, pendingSubmission.current);
        pendingSubmission.current = nextKey;
        mutation.mutate({ taskInput: input, idempotencyKey: nextKey.key });
      }
    }
  };

  return (
    <div className="page-wrap workbench-page">
      <div className="page-heading">
        <div>
          <span className="eyebrow">Workspace</span>
          <h1>What would you like to accomplish?</h1>
          <p>
            Choose an approved agent, describe the outcome, and Gantry will keep the run state
            visible.
          </p>
        </div>
      </div>

      <div className="workbench-grid">
        <div className="composer-column">
          <AgentPicker
            selectedId={selectedAgent?.id ?? queryAgentId ?? ''}
            onSelect={setSelectedAgent}
          />

          {/* ChatGPT Prompt Capsule Form */}
          <form className="chatgpt-prompt-capsule" onSubmit={submit}>
            <div className="chatgpt-prompt-inner">
              <label className="sr-only" htmlFor="task-message">
                Task message
              </label>

              <textarea
                id="task-message"
                value={message}
                onChange={(event) => setMessage(event.target.value)}
                onKeyDown={handleKeyDown}
                placeholder={
                  selectedAgent
                    ? `Message ${selectedAgent.display_name}… (Ctrl+Enter to send)`
                    : 'Select an agent above, then describe your task here…'
                }
                rows={3}
                className="chatgpt-prompt-textarea"
              />

              {/* Bottom Action Bar inside Capsule */}
              <div className="chatgpt-prompt-toolbar">
                <div className="chatgpt-prompt-meta">
                  {selectedAgent ? (
                    <div className="chatgpt-agent-chip">
                      <Bot size={13} strokeWidth={2.2} />
                      <span>{selectedAgent.display_name}</span>
                    </div>
                  ) : (
                    <div className="chatgpt-agent-chip chatgpt-agent-chip-empty">
                      <HelpCircle size={13} />
                      <span>No agent selected</span>
                    </div>
                  )}

                  {import.meta.env.DEV ? (
                    <label className="dev-toggle">
                      <input
                        type="checkbox"
                        checked={holdOpen}
                        onChange={(event) => setHoldOpen(event.target.checked)}
                      />
                      <span>Test cancel</span>
                    </label>
                  ) : null}
                </div>

                <div className="chatgpt-prompt-submit-area">
                  <button
                    type="submit"
                    disabled={!input || mutation.isPending}
                    aria-label={
                      mutation.isPending
                        ? 'Submitting task'
                        : mutation.isError
                        ? 'Retry submission'
                        : 'Start task'
                    }
                    title={
                      !selectedAgent
                        ? 'Please select an agent first'
                        : !message.trim()
                        ? 'Please type a task message'
                        : 'Start task (Ctrl+Enter)'
                    }
                    className={`chatgpt-send-button ${
                      input && !mutation.isPending ? 'chatgpt-send-button-active' : ''
                    }`}
                  >
                    {mutation.isPending ? (
                      <span className="ds-spin chatgpt-spinner" />
                    ) : (
                      <ArrowUp size={18} strokeWidth={2.5} />
                    )}
                  </button>
                </div>
              </div>
            </div>

            {mutation.isError ? (
              <p className="inline-error" role="alert" style={{ marginTop: '12px' }}>
                {mutation.error instanceof Error
                  ? mutation.error.message
                  : 'The task could not be submitted.'}{' '}
                Submit again to retry with the same idempotency key.
              </p>
            ) : null}

            <div className="composer-bottom-meta">
              <span className="composer-hint">
                <Info size={13} /> Your task is private to your account.
              </span>
            </div>
          </form>

          {/* ChatGPT Starter Prompt Tiles */}
          <div className="starter-prompts-section">
            <div className="starter-prompts-header">
              <Lightbulb size={15} />
              <span>Prompt suggestions</span>
            </div>
            <div className="starter-prompts-grid">
              {STARTER_PROMPTS.map((item, idx) => {
                const Icon = item.icon;
                return (
                  <button
                    type="button"
                    key={`starter-${idx}`}
                    className="starter-prompt-card"
                    onClick={() => setMessage(item.prompt)}
                  >
                    <div className="starter-prompt-icon">
                      <Icon size={16} />
                    </div>
                    <div className="starter-prompt-content">
                      <strong>{item.title}</strong>
                      <p>{item.prompt}</p>
                    </div>
                  </button>
                );
              })}
            </div>
          </div>
        </div>

        {/* Guidance Side Panel */}
        <aside className="context-panel" aria-label="Task guidance">
          <div className="context-panel-top">
            <span className="context-kicker">How it works</span>
            <h2>A focused workspace for approved capabilities.</h2>
          </div>
          <ol className="steps-list">
            <li>
              <span>01</span>
              <div>
                <strong>Choose an agent</strong>
                <p>Each capability is published for your workspace.</p>
              </div>
            </li>
            <li>
              <span>02</span>
              <div>
                <strong>State the outcome</strong>
                <p>Keep the request focused on the result you need.</p>
              </div>
            </li>
            <li>
              <span>03</span>
              <div>
                <strong>Follow the run</strong>
                <p>Return anytime to see the current durable status.</p>
              </div>
            </li>
          </ol>
        </aside>
      </div>
    </div>
  );
}
