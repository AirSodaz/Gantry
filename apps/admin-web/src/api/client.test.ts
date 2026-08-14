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

  it('sends the review revision precondition and decision body', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ status: 'pending' }), { status: 201 }));
    globalThis.fetch = fetchMock;
    const api = new AdminApi(() => 'admin-token');
    await api.submitReview('agt_1', 7, 'Expand the approved demo mode.');
    expect(fetchMock).toHaveBeenCalledWith('/api/admin/v1/agents/agt_1:review', expect.objectContaining({ method: 'POST', headers: expect.objectContaining({ 'If-Match': '7' }), body: JSON.stringify({ release_notes: 'Expand the approved demo mode.' }) }));
  });

  it('posts an explicit immutable version rollback', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    globalThis.fetch = fetchMock;
    const api = new AdminApi(() => 'admin-token');
    await api.rollback('agt_1', 'agtv_1');
    expect(fetchMock).toHaveBeenCalledWith('/api/admin/v1/agents/agt_1:rollback', expect.objectContaining({ body: JSON.stringify({ version_id: 'agtv_1' }) }));
  });
});
