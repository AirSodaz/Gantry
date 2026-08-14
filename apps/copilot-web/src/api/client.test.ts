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
});
