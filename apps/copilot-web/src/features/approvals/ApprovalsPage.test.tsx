import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApprovalsPage } from './ApprovalsPage';

const mocked = vi.hoisted(() => ({ api: { listApprovals: vi.fn() } }));
vi.mock('../../api/ApiProvider', () => ({ useCopilotApi: () => mocked.api }));

function renderPage() {
	const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
	return render(<QueryClientProvider client={client}><MemoryRouter><ApprovalsPage /></MemoryRouter></QueryClientProvider>);
}

describe('ApprovalsPage', () => {
	beforeEach(() => vi.clearAllMocks());

	it('shows the pending count, agent, request age, and task link', async () => {
		mocked.api.listApprovals.mockResolvedValue({ items: [{
			id: 'apr_1', task_id: 'tsk_1', agent_display_name: 'Directory agent', tool_name: 'Directory', operation: 'update', target: 'employee/123', action_digest: 'sha256:exact', risk_class: 'write', created_at: new Date(Date.now() - 60_000).toISOString(), expires_at: '2026-08-20T08:00:00Z', status: 'pending',
		}] });
		renderPage();

		expect(await screen.findByText('1 action is waiting for your decision.')).toBeInTheDocument();
		expect(screen.getByText('Directory agent · employee/123')).toBeInTheDocument();
		expect(screen.getByRole('link', { name: 'Open task' })).toHaveAttribute('href', '/tasks/tsk_1');
		expect(screen.getByText(/Requested 1m ago/)).toBeInTheDocument();
	});
});
