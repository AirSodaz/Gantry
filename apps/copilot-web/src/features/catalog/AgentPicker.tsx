import { useDeferredValue, useEffect, useState } from 'react';
import { Check, Search } from 'lucide-react';
import { TextInput } from '@gantry/design-system';
import { useQuery } from '@tanstack/react-query';
import { useCopilotApi } from '../../api/ApiProvider';
import type { Agent } from '../../api/types';
import { EmptyState, ErrorState, LoadingState } from '../../components/AsyncState';

export function AgentPicker({ selectedId, onSelect }: { selectedId: string; onSelect: (agent: Agent) => void }) {
  const api = useCopilotApi();
  const [search, setSearch] = useState('');
  const [category, setCategory] = useState('');
  const deferredSearch = useDeferredValue(search);
  const query = useQuery({
    queryKey: ['agents', deferredSearch, category],
    queryFn: () => api.listAgents(deferredSearch, category),
  });
  const categoriesQuery = useQuery({
    queryKey: ['agent-categories'],
    queryFn: () => api.listAgents(),
    staleTime: 60_000,
  });

  useEffect(() => {
    const initial = query.data?.items.find((agent) => agent.id === selectedId);
    if (initial) onSelect(initial);
  }, [query.data?.items, selectedId]); // onSelect is an event boundary owned by the page.

  return (
    <section className="picker-section" aria-labelledby="agent-picker-title">
      <div className="section-heading-row">
        <div><span className="eyebrow">Capability</span><h2 id="agent-picker-title">Choose an agent</h2></div>
        <span className="result-count">{query.data?.items.length ?? 0} available</span>
      </div>
      <TextInput label="Search agents" value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Search by name or description" />
      <div className="search-icon" aria-hidden="true"><Search size={16} /></div>
      <label className="category-field">
        <span>Category</span>
        <select value={category} onChange={(event) => setCategory(event.target.value)}>
          <option value="">All categories</option>
          {[...new Set((categoriesQuery.data?.items ?? []).map((agent) => agent.category).filter(Boolean))].sort().map((value) => <option key={value} value={value}>{value}</option>)}
        </select>
      </label>
      {query.isLoading ? <LoadingState label="Loading agents" /> : null}
      {query.isError ? <ErrorState message={query.error instanceof Error ? query.error.message : 'The agent catalog could not be loaded.'} onRetry={() => void query.refetch()} /> : null}
      {!query.isLoading && !query.isError && query.data?.items.length === 0 ? <EmptyState title="No matching agents" detail="Try another search term." /> : null}
      <div className="agent-list">
        {query.data?.items.map((agent) => (
          <button type="button" key={agent.id} className={`agent-option ${selectedId === agent.id ? 'agent-option-selected' : ''}`} onClick={() => onSelect(agent)}>
            <span className="agent-symbol" aria-hidden="true">{agent.display_name.slice(0, 1)}</span>
            <span className="agent-option-copy"><strong>{agent.display_name}</strong><span>{agent.description}</span></span>
            {selectedId === agent.id ? <Check size={18} aria-label="Selected" /> : null}
          </button>
        ))}
      </div>
    </section>
  );
}
