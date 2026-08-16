import { useSearchParams, Link } from 'react-router-dom';
import { FileArchive, Filter, ArrowUpRight } from 'lucide-react';
import { Select, StatusMark, type SelectOption } from '@gantry/design-system';
import { useQuery } from '@tanstack/react-query';
import { useCopilotApi } from '../../api/ApiProvider';
import { EmptyState, ErrorState, LoadingState } from '../../components/AsyncState';

const CLASSIFICATION_OPTIONS: SelectOption[] = [
  { value: '', label: 'All classifications', icon: <Filter size={13} /> },
  { value: 'public', label: 'Public' },
  { value: 'internal', label: 'Internal' },
  { value: 'confidential', label: 'Confidential' },
];

export function ArtifactsPage() {
  const api = useCopilotApi();
  const [params, setParams] = useSearchParams();
  const taskId = params.get('task_id') ?? '';
  const classification = params.get('classification') ?? '';
  const query = useQuery({
    queryKey: ['artifacts', taskId, classification],
    queryFn: () => api.listArtifacts(taskId, classification),
  });
  const setFilter = (name: 'task_id' | 'classification', value: string) => {
    const next = new URLSearchParams(params);
    if (value) next.set(name, value); else next.delete(name);
    setParams(next, { replace: true });
  };

  return <div className="page-wrap artifacts-page">
    <div className="page-heading page-heading-inline"><div><span className="eyebrow">Files</span><h1>Artifacts</h1><p>Files produced by tasks you started.</p></div></div>
    <div className="artifact-filters">
      <label>Task <input value={taskId} onChange={(event) => setFilter('task_id', event.target.value.trim())} placeholder="Filter by task ID" /></label>
      <Select label="Classification" options={CLASSIFICATION_OPTIONS} value={classification} onChange={(value) => setFilter('classification', value)} placeholder="All classifications" />
    </div>
    {query.isLoading ? <LoadingState label="Loading artifacts" /> : null}
    {query.isError ? <ErrorState message={query.error instanceof Error ? query.error.message : 'Artifacts could not be loaded.'} onRetry={() => void query.refetch()} /> : null}
    {!query.isLoading && !query.isError && query.data?.items.length === 0 ? <EmptyState title="No artifacts found" detail="Files produced by your tasks will appear here." /> : null}
    <div className="artifact-browser-list" aria-label="Artifacts">
      {query.data?.items.map((artifact) => <Link className="artifact-browser-row" key={artifact.id} to={`/artifacts/${encodeURIComponent(artifact.id)}`}>
        <span className="artifact-browser-icon" aria-hidden="true"><FileArchive size={18} /></span>
        <span className="artifact-browser-copy"><strong>{artifact.filename}</strong><span>{artifact.media_type} · {formatBytes(artifact.size_bytes)}</span><small>{artifact.task_id}</small></span>
        <span className="artifact-browser-states"><StatusMark status={artifact.classification ?? 'internal'} /><StatusMark status={artifact.state} /></span>
        <ArrowUpRight size={17} aria-hidden="true" />
      </Link>)}
    </div>
  </div>;
}

function formatBytes(value: number) {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${Math.round(value / 1024)} KB`;
  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
}
