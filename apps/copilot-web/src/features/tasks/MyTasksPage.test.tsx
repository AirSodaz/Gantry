import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MyTasksPage } from './MyTasksPage';

const mocked = vi.hoisted(() => ({
  api: { listTasks: vi.fn(), listAgents: vi.fn() },
}));
vi.mock('../../api/ApiProvider', () => ({ useCopilotApi: () => mocked.api }));

function renderPage(path = '/tasks') {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[path]}><Routes><Route path="/tasks" element={<MyTasksPage />} /></Routes></MemoryRouter></QueryClientProvider>);
}

describe('MyTasksPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocked.api.listAgents.mockResolvedValue({ items: [{ id: 'agt_1', display_name: 'Support agent' }] });
    mocked.api.listTasks.mockResolvedValue({ items: [] });
  });

  it('hydrates API filters from the shareable task-history URL', async () => {
    renderPage('/tasks?status=completed&agent_id=agt_1&requester_action=input&time_range=7d');

    await waitFor(() => expect(mocked.api.listTasks).toHaveBeenCalledWith(expect.objectContaining({ status: 'completed', agentId: 'agt_1', requesterAction: 'input', createdAfter: expect.any(String) })));
    expect(await screen.findByText('No tasks yet')).toBeInTheDocument();
  });
});
