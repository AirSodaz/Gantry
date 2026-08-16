import { Link, useParams } from 'react-router-dom';
import { ArrowLeft, Download, FileArchive } from 'lucide-react';
import { Button, StatusMark } from '@gantry/design-system';
import { useQuery } from '@tanstack/react-query';
import { useCopilotApi } from '../../api/ApiProvider';
import { ErrorState, LoadingState } from '../../components/AsyncState';

export function ArtifactDetailPage() {
  const { artifactId = '' } = useParams();
  const api = useCopilotApi();
  const query = useQuery({ queryKey: ['artifact', artifactId], queryFn: () => api.getArtifact(artifactId), enabled: Boolean(artifactId) });
  if (query.isLoading) return <div className="page-wrap"><LoadingState label="Loading artifact" /></div>;
  if (query.isError || !query.data) return <div className="page-wrap"><ErrorState message={query.error instanceof Error ? query.error.message : 'This artifact could not be loaded.'} onRetry={() => void query.refetch()} /></div>;
  const artifact = query.data;
  const download = () => { if (artifact.download_url) window.open(artifact.download_url, '_blank', 'noopener,noreferrer'); };
  return <div className="page-wrap narrow-page artifact-detail-page">
    <Link to="/artifacts" className="back-link"><ArrowLeft size={15} /> Artifacts</Link>
    <div className="page-heading"><div><span className="eyebrow">Artifact detail</span><h1>{artifact.filename}</h1><p>{artifact.media_type}</p></div><FileArchive size={24} aria-hidden="true" /></div>
    <section className="artifact-detail-grid">
      <div><span>Availability</span><StatusMark status={artifact.state} /></div>
      <div><span>Scan</span><StatusMark status={artifact.scan_status} /></div>
      <div><span>Classification</span><StatusMark status={artifact.classification ?? 'internal'} /></div>
      <div><span>Size</span><strong>{formatBytes(artifact.size_bytes)}</strong></div>
      <div><span>Task</span><Link to={`/tasks/${artifact.task_id}`}>{artifact.task_id}</Link></div>
      <div><span>Digest</span><code>{artifact.digest}</code></div>
    </section>
    <Button disabled={!artifact.download_url} onClick={download}><Download size={16} /> {artifact.download_url ? 'Download' : 'Download unavailable'}</Button>
  </div>;
}

function formatBytes(value: number) {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${Math.round(value / 1024)} KB`;
  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
}
