import { afterEach, describe, expect, it, vi } from 'vitest';
import { AdminApi, AdminApiError } from './client';

const originalFetch = globalThis.fetch;

afterEach(() => { globalThis.fetch = originalFetch; });

describe('AdminApi', () => {
  it('fails locally when the authenticated session is absent', async () => {
    const api = new AdminApi(() => null);
    await expect(api.listWorkspaces()).rejects.toBeInstanceOf(AdminApiError);
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

  it('encodes catalog search and status filters', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ items: [] }), { status: 200 }));
    globalThis.fetch = fetchMock;
    const api = new AdminApi(() => 'admin-token');
    await api.listTools({ search: 'data export', status: 'deprecated' });
    expect(fetchMock).toHaveBeenCalledWith('/api/admin/v1/tools?search=data+export&status=deprecated', expect.anything());
  });

  it('encodes the Admin Run workbench filters', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ items: [] }), { status: 200 }));
    globalThis.fetch = fetchMock;
    const api = new AdminApi(() => 'admin-token');
    await api.listRuns({ workspaceId: 'ws_1', agentId: 'agt_1', revisionHash: 'sha256:abc', status: 'failed', limit: 20 });
    expect(fetchMock).toHaveBeenCalledWith('/api/admin/v1/runs?workspace_id=ws_1&agent_id=agt_1&revision_hash=sha256%3Aabc&status=failed&limit=20', expect.anything());
  });
});
