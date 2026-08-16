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

  it('uses the configuration catalog routes and workspace enablement command', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ id: 'skill_1' }), { status: 201 }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    globalThis.fetch = fetchMock;
    const api = new AdminApi(() => 'admin-token');
    await api.registerSkill({ workspace_id: 'ws_1', slug: 'search', display_name: 'Search', description: '', source_type: 'locator', source_ref: 'registry://search', declared_version: '', content_digest: 'sha256:1' });
    await api.enablePlugin('plugin_1', 'ws_1');
    expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/admin/v1/skills', expect.objectContaining({ method: 'POST', body: expect.stringContaining('registry://search') }));
    expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/admin/v1/plugins/plugin_1/enable', expect.objectContaining({ method: 'POST', body: JSON.stringify({ workspace_id: 'ws_1' }) }));
  });

  it('loads asset detail and posts an auditable lifecycle command', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(JSON.stringify({ id: 'skill_1', status: 'deprecated' }), { status: 200 }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));
    globalThis.fetch = fetchMock;
    const api = new AdminApi(() => 'admin-token');
    await api.getSkill('skill_1');
    await api.activateSkill('skill_1', 'validated replacement package');
    expect(fetchMock).toHaveBeenNthCalledWith(1, '/api/admin/v1/skills/skill_1', expect.objectContaining({ headers: expect.objectContaining({ Authorization: 'Bearer admin-token' }) }));
    expect(fetchMock).toHaveBeenNthCalledWith(2, '/api/admin/v1/skills/skill_1:activate', expect.objectContaining({ method: 'POST', body: JSON.stringify({ reason: 'validated replacement package' }) }));
  });
});
