import { render, screen, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AgentPicker } from './AgentPicker';

const mocked = vi.hoisted(() => ({ api: { listAgents: vi.fn() } }));
vi.mock('../../api/ApiProvider', () => ({ useCopilotApi: () => mocked.api }));

const agents = [
  { id: 'agt_1', display_name: 'Lifecycle Demo', description: 'A deterministic fixture.', category: 'Development' },
];

function renderPicker() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={queryClient}><AgentPicker selectedId="" onSelect={vi.fn()} /></QueryClientProvider>);
}

describe('AgentPicker', () => {
  beforeEach(() => vi.clearAllMocks());

  it('renders loading state while catalog requests are pending', () => {
    mocked.api.listAgents.mockReturnValue(new Promise(() => undefined));
    renderPicker();
    expect(screen.getByText('Loading agents')).toBeInTheDocument();
  });

  it('renders an empty state when no published agents match', async () => {
    mocked.api.listAgents.mockResolvedValue({ items: [] });
    renderPicker();
    expect(await screen.findByText('No matching agents')).toBeInTheDocument();
  });

  it('renders an error state and retries the catalog request', async () => {
    mocked.api.listAgents.mockRejectedValue(new Error('Catalog unavailable'));
    renderPicker();
    expect(await screen.findByText('Catalog unavailable')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Try again' })).toBeInTheDocument();
  });

  it('filters by category and renders matching agents', async () => {
    mocked.api.listAgents.mockImplementation((_search = '', category = '') => {
      if (category) return Promise.resolve({ items: agents.filter((agent) => agent.category === category) });
      return Promise.resolve({ items: agents });
    });
    renderPicker();
    await waitFor(() => expect(screen.getByRole('option', { name: 'Development' })).toBeInTheDocument());
    expect(screen.getByText('Lifecycle Demo')).toBeInTheDocument();
  });
});
