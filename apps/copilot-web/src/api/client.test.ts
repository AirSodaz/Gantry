import { describe, expect, it, vi } from 'vitest';
import { CopilotApi, CopilotApiError } from './client';

describe('CopilotApi', () => {
  it('sends bearer and idempotency credentials in headers only', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ id: 'tsk_1' }), { status: 201 }));
    vi.stubGlobal('fetch', fetchMock);
    const api = new CopilotApi(() => 'token-1');

    await api.submitTask({ agent_id: 'agt_1', message: 'hello' }, 'key-1');

    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(init.headers).toEqual(expect.objectContaining({ Authorization: 'Bearer token-1', 'Idempotency-Key': 'key-1' }));
    expect(init.body).toBe(JSON.stringify({ agent_id: 'agt_1', message: 'hello' }));
    expect(init.body).not.toContain('idempotency_key');
  });

  it('maps structured API errors without hiding the status', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { message: 'Resource was not found.' } }), { status: 404 })));
    const api = new CopilotApi(() => 'token-1');

    await expect(api.getTask('tsk_missing')).rejects.toEqual(expect.objectContaining({ status: 404, message: 'Resource was not found.' } satisfies Partial<CopilotApiError>));
  });

  it('preserves a server-winning resource from a command conflict', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ error: { code: 'approval_changed', message: 'Approval changed.', current_resource: { id: 'apr_1', status: 'rejected' } } }), { status: 409 })));
    const api = new CopilotApi(() => 'token-1');

    await expect(api.decideApproval('apr_1', 'approve', 'sha256:action-1', 1)).rejects.toEqual(expect.objectContaining({
      status: 409,
      code: 'approval_changed',
      currentResource: { id: 'apr_1', status: 'rejected' },
    } satisfies Partial<CopilotApiError>));
  });

  it('retains the task conversation ETag for conditional commands', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({ id: 'tsk_1', agent_id: 'agt_1', status: 'failed', conversation_revision: 3 }), { status: 200, headers: { ETag: '"3"' } })));
    const api = new CopilotApi(() => 'token-1');

    await expect(api.getTask('tsk_1')).resolves.toEqual(expect.objectContaining({ conversation_etag: '"3"' }));
  });

  it('sends the selected retry revision under the documented request field', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ id: 'tsk_1' }), { status: 201, headers: { ETag: '"4"' } }));
    vi.stubGlobal('fetch', fetchMock);
    const api = new CopilotApi(() => 'token-1');

    await api.retryTask('tsk_1', 'retry-1', '"3"', 'current_production_revision');

    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(init.headers).toEqual(expect.objectContaining({ 'Idempotency-Key': 'retry-1', 'If-Match': '"3"' }));
    expect(init.body).toBe(JSON.stringify({ revision_selection: 'current_production_revision' }));
  });

  it('includes the rendered cursor when refreshing an event ticket', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ ticket: 'evt_1', task_id: 'tsk_1', websocket_url: 'wss://copilot.example.test/events', expires_at: '2026-08-17T01:00:00Z' }), { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
    const api = new CopilotApi(() => 'token-1');

    await api.createEventsTicket('tsk_1', 'cur_1');

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toContain('/tasks/tsk_1/events:ticket');
    expect(init.body).toBe(JSON.stringify({ last_cursor: 'cur_1' }));
  });

  it('requests download references through the audited artifact command', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ artifact_id: 'art_1', download_url: 'https://downloads.example.test/art_1', expires_at: '2026-08-17T01:00:00Z' }), { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
    const api = new CopilotApi(() => 'token-1');

    await api.requestArtifactDownload('art_1');

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toContain('/artifacts/art_1:download');
    expect(init.method).toBe('POST');
    expect(init.headers).toEqual(expect.objectContaining({ Authorization: 'Bearer token-1' }));
  });

  it('sends action digest and decision idempotency in the approval body', async () => {
    vi.stubGlobal('crypto', { randomUUID: () => 'decision-key-1' });
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ status: 'satisfied' }), { status: 200 }));
    vi.stubGlobal('fetch', fetchMock);
    const api = new CopilotApi(() => 'token-1');

    await api.decideApproval('apr_1', 'approve', 'sha256:action-1', 1);

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toContain('/approvals/apr_1:decide');
    expect(init.body).toBe(JSON.stringify({ decision: 'approve', action_digest: 'sha256:action-1', approval_revision: 1, reason: '' }));
    expect(init.headers).toEqual(expect.objectContaining({ Authorization: 'Bearer token-1', 'Idempotency-Key': 'decision-key-1' }));
  });
});
