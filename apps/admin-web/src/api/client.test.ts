import { afterEach, describe, expect, it, vi } from 'vitest';
import { AdminApi, AdminApiError } from './client';

const originalFetch = globalThis.fetch;

afterEach(() => { globalThis.fetch = originalFetch; });

describe('AdminApi', () => {
  it('uses the Admin bearer token and revision precondition when publishing', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ id: 'agtv_1' }), { status: 201 }));
    globalThis.fetch = fetchMock;
    const api = new AdminApi(() => 'admin-token');
    await api.publish('agt_1', 4);
    expect(fetchMock).toHaveBeenCalledWith('/api/admin/v1/agents/agt_1:publish', expect.objectContaining({
      method: 'POST', headers: expect.objectContaining({ Authorization: 'Bearer admin-token', 'If-Match': '4' }),
    }));
  });

  it('fails locally when the authenticated session is absent', async () => {
    const api = new AdminApi(() => null);
    await expect(api.listWorkspaces()).rejects.toBeInstanceOf(AdminApiError);
  });
});
