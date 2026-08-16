import { useDeferredValue, useEffect, useMemo } from 'react';
import { useSearchParams } from 'react-router-dom';
import { Check, Search, Tag } from 'lucide-react';
import { Select, type SelectOption, TextInput } from '@gantry/design-system';
import { useQuery } from '@tanstack/react-query';
import { useCopilotApi } from '../../api/ApiProvider';
import type { Agent } from '../../api/types';
import { EmptyState, ErrorState, LoadingState } from '../../components/AsyncState';

export function AgentPicker({
  selectedId,
  onSelect,
}: {
  selectedId: string;
  onSelect: (agent: Agent) => void;
}) {
  const api = useCopilotApi();
  const [params, setParams] = useSearchParams();
  const search = params.get('search') ?? '';
  const category = params.get('category') ?? '';
  const deferredSearch = useDeferredValue(search);
  const setFilter = (key: 'search' | 'category', value: string) => {
    const next = new URLSearchParams(params);
    if (value) next.set(key, value); else next.delete(key);
    setParams(next, { replace: true });
  };

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
  }, [query.data?.items, selectedId]);

  const categoryOptions = useMemo<SelectOption[]>(() => {
    const rawCategories = Array.from(
      new Set(
        (categoriesQuery.data?.items ?? [])
          .map((agent) => agent.category)
          .filter((cat): cat is string => Boolean(cat))
      )
    ).sort();

    return [
      { value: '', label: 'All categories' },
      ...rawCategories.map((cat) => ({
        value: cat,
        label: cat,
        icon: <Tag size={13} />,
      })),
    ];
  }, [categoriesQuery.data?.items]);

  return (
    <section className="picker-section" aria-labelledby="agent-picker-title">
      <div className="section-heading-row">
        <div>
          <span className="eyebrow">Capability</span>
          <h2 id="agent-picker-title">Choose an agent</h2>
        </div>
        <span className="result-count">{query.data?.items.length ?? 0} available</span>
      </div>

      <div className="picker-filters">
        <TextInput
          label="Search agents"
          value={search}
          onChange={(event) => setFilter('search', event.target.value)}
          placeholder="Search by name or description"
          icon={<Search size={16} />}
        />

        <div className="picker-select-wrap">
          <Select
            label="Category"
            options={categoryOptions}
            value={category}
            onChange={(value) => setFilter('category', value)}
            placeholder="All categories"
          />
        </div>
      </div>

      {query.isLoading ? <LoadingState label="Loading agents" /> : null}
      {query.isError ? (
        <ErrorState
          message={
            query.error instanceof Error ? query.error.message : 'The agent catalog could not be loaded.'
          }
          onRetry={() => void query.refetch()}
        />
      ) : null}
      {!query.isLoading && !query.isError && query.data?.items.length === 0 ? (
        <EmptyState title="No matching agents" detail="Try another search term or clear category filter." />
      ) : null}

      <div className="agent-list">
        {query.data?.items.map((agent) => {
          const isSelected = selectedId === agent.id;
          return (
            <button
              type="button"
              key={agent.id}
              className={`agent-option ${isSelected ? 'agent-option-selected' : ''}`}
              onClick={() => onSelect(agent)}
            >
              <span className="agent-symbol" aria-hidden="true">
                {agent.display_name.slice(0, 1).toUpperCase()}
              </span>
              <span className="agent-option-copy">
                <strong>{agent.display_name}</strong>
                <span>{agent.description}</span>
                {agent.owner_name ? <small>Owner: {agent.owner_name}</small> : null}
              </span>
              {isSelected ? (
                <Check size={18} className="agent-check-icon" aria-label="Selected" />
              ) : null}
            </button>
          );
        })}
      </div>
    </section>
  );
}
