import { render, screen } from '@testing-library/react';
import type { ReactNode } from 'react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ArtifactDetailPage } from './ArtifactDetailPage';
import { ArtifactsPage } from './ArtifactsPage';

const mocked = vi.hoisted(() => ({
  api: {
    listArtifacts: vi.fn(),
    getArtifact: vi.fn(),
  },
}));
vi.mock('../../api/ApiProvider', () => ({ useCopilotApi: () => mocked.api }));

const artifact = {
  id: 'art_1', task_id: 'tsk_1', run_id: 'run_1', filename: 'report.pdf',
  media_type: 'application/pdf', size_bytes: 2048, digest: 'sha256:artifact',
  classification: 'internal', state: 'available', scan_status: 'passed',
};

function renderRoute(path: string, element: ReactNode) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(<QueryClientProvider client={client}><MemoryRouter initialEntries={[path]}><Routes><Route path="/artifacts" element={element} /><Route path="/artifacts/:artifactId" element={element} /></Routes></MemoryRouter></QueryClientProvider>);
}

describe('Artifact pages', () => {
  beforeEach(() => vi.clearAllMocks());

  it('lists only the API-projected artifacts', async () => {
    mocked.api.listArtifacts.mockResolvedValue({ items: [artifact] });
    renderRoute('/artifacts?task_id=tsk_1&classification=internal', <ArtifactsPage />);

    expect(await screen.findByText('report.pdf')).toBeInTheDocument();
    expect(mocked.api.listArtifacts).toHaveBeenCalledWith('tsk_1', 'internal');
    expect(screen.getByRole('link', { name: /report.pdf/i })).toHaveAttribute('href', '/artifacts/art_1');
  });

  it('keeps download unavailable until the server issues a URL', async () => {
    mocked.api.getArtifact.mockResolvedValue({ ...artifact, state: 'declared', scan_status: 'pending' });
    renderRoute('/artifacts/art_1', <ArtifactDetailPage />);

    expect(await screen.findByText('report.pdf')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Download unavailable' })).toBeDisabled();
  });
});
