import { FormEvent, useMemo, useRef, useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import {
  ArrowUp,
  Bot,
  CircleCheck,
  FileCode2,
  HelpCircle,
  Info,
  Lightbulb,
  LoaderCircle,
  Paperclip,
  ShieldAlert,
  Sparkles,
  X,
  Zap,
} from 'lucide-react';
import { useMutation } from '@tanstack/react-query';
import { useCopilotApi } from '../../api/ApiProvider';
import { AgentPicker } from '../catalog/AgentPicker';
import type { Agent, Attachment, SubmitTaskInput } from '../../api/types';
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

type PendingAttachment = {
  localId: string;
  filename: string;
  sizeBytes: number;
  progress: number;
  state: 'hashing' | 'uploading' | 'validating' | 'available' | 'error';
  attachment?: Attachment;
  error?: string;
};

export function NewTaskPage() {
  const api = useCopilotApi();
  const navigate = useNavigate();
  const location = useLocation();
  const [selectedAgent, setSelectedAgent] = useState<Agent | null>(null);
  const [message, setMessage] = useState('');
  const [holdOpen, setHoldOpen] = useState(false);
  const [attachments, setAttachments] = useState<PendingAttachment[]>([]);
  const pendingSubmission = useRef<{ key: string; signature: string } | null>(null);
  const queryAgentId = new URLSearchParams(location.search).get('agent');
  const hasPendingAttachment = attachments.some((item) => !['available', 'error'].includes(item.state));
  const attachmentIDs = attachments.flatMap((item) => item.state === 'available' && item.attachment ? [item.attachment.id] : []);

  const input = useMemo<SubmitTaskInput | null>(() => {
    if (!selectedAgent || !message.trim() || hasPendingAttachment) return null;
    return {
      agent_id: selectedAgent.id,
      message: message.trim(),
      ...(attachmentIDs.length ? { attachment_ids: attachmentIDs } : {}),
      ...(import.meta.env.DEV && holdOpen ? { structured_input: { mode: 'await_cancel' } } : {}),
    };
  }, [attachmentIDs, hasPendingAttachment, holdOpen, message, selectedAgent]);

  const updateAttachment = (localId: string, patch: Partial<PendingAttachment>) => {
    setAttachments((current) => current.map((item) => item.localId === localId ? { ...item, ...patch } : item));
  };

  const selectAttachments = async (files: FileList | null) => {
    if (!files) return;
    await Promise.all(Array.from(files).map(async (file) => {
      const localId = crypto.randomUUID();
      setAttachments((current) => [...current, { localId, filename: file.name, sizeBytes: file.size, progress: 0, state: 'hashing' }]);
      try {
        if (file.size > 64 * 1024 * 1024) throw new Error('Files must be 64 MB or smaller.');
        const digest = await digestFile(file);
        const attachment = await api.createAttachment({
          filename: file.name,
          media_type: file.type || 'application/octet-stream',
          size_bytes: file.size,
          digest,
          classification: 'internal',
        });
        updateAttachment(localId, { state: 'uploading', attachment });
        await api.uploadAttachment(attachment, file, (progress) => updateAttachment(localId, { progress }));
        updateAttachment(localId, { state: 'validating', progress: 100 });
        const completed = await api.completeAttachment(attachment.id);
        if (completed.state !== 'available' || completed.scan_status !== 'passed') {
          throw new Error('The attachment is still being scanned.');
        }
        updateAttachment(localId, { state: 'available', attachment: completed });
      } catch (error) {
        updateAttachment(localId, { state: 'error', error: error instanceof Error ? error.message : 'Attachment upload failed.' });
      }
    }));
  };

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

              {attachments.length ? (
                <ul className="attachment-list" aria-label="Selected attachments">
                  {attachments.map((attachment) => (
                    <li key={attachment.localId} className={`attachment-item attachment-${attachment.state}`}>
                      {attachment.state === 'available' ? <CircleCheck size={15} aria-hidden="true" /> : attachment.state === 'error' ? <ShieldAlert size={15} aria-hidden="true" /> : <LoaderCircle size={15} aria-hidden="true" className="ds-spin" />}
                      <span className="attachment-name">{attachment.filename}</span>
                      <span className="attachment-status">
                        {attachment.state === 'hashing' ? 'Preparing' : attachment.state === 'uploading' ? `${attachment.progress}%` : attachment.state === 'validating' ? 'Validating' : attachment.state === 'available' ? 'Ready' : attachment.error}
                      </span>
                      <button type="button" className="ds-icon-button ds-icon-button-sm attachment-remove" onClick={() => setAttachments((current) => current.filter((item) => item.localId !== attachment.localId))} aria-label={`Remove ${attachment.filename}`} title={`Remove ${attachment.filename}`}><X size={14} /></button>
                    </li>
                  ))}
                </ul>
              ) : null}

              {/* Bottom Action Bar inside Capsule */}
              <div className="chatgpt-prompt-toolbar">
                <div className="chatgpt-prompt-meta">
                  <label className="attachment-add" title="Add attachment">
                    <Paperclip size={15} aria-hidden="true" />
                    <span className="sr-only">Add attachment</span>
                    <input type="file" multiple onChange={(event) => { void selectAttachments(event.target.files); event.currentTarget.value = ''; }} />
                  </label>
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

async function digestFile(file: File) {
  if (!crypto.subtle) throw new Error('This browser cannot securely prepare attachments.');
  const data = await file.arrayBuffer();
  const digest = await crypto.subtle.digest('SHA-256', data);
  return `sha256:${Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, '0')).join('')}`;
}
