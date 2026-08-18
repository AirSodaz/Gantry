import { useDeferredValue, useEffect, useMemo } from "react";
import { useSearchParams } from "react-router-dom";
import { Check, Search, Star, Tag } from "lucide-react";
import {
  Button,
  IconButton,
  Select,
  type SelectOption,
  TextInput,
} from "@gantry/design-system";
import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { useCopilotApi } from "../../api/ApiProvider";
import type { Agent } from "../../api/types";
import {
  EmptyState,
  ErrorState,
  LoadingState,
} from "../../components/AsyncState";

export function AgentPicker({
  selectedId,
  onSelect,
}: {
  selectedId: string;
  onSelect: (agent: Agent) => void;
}) {
  const api = useCopilotApi();
  const [params, setParams] = useSearchParams();
  const search = params.get("search") ?? "";
  const category = params.get("category") ?? "";
  const collectionParam = params.get("collection");
  const collection =
    collectionParam === "favorites" || collectionParam === "recent"
      ? collectionParam
      : "all";
  const deferredSearch = useDeferredValue(search);
  const queryClient = useQueryClient();
  const setFilter = (
    key: "search" | "category" | "collection",
    value: string,
  ) => {
    const next = new URLSearchParams(params);
    if (value && !(key === "collection" && value === "all")) next.set(key, value);
    else next.delete(key);
    setParams(next, { replace: true });
  };

  const query = useInfiniteQuery({
    queryKey: ["agents", deferredSearch, category, collection],
    initialPageParam: "",
    queryFn: ({ pageParam }) =>
      api.listAgents(deferredSearch, category, pageParam, collection),
    getNextPageParam: (page) =>
      page.page_info?.has_more ? page.page_info.next_cursor : undefined,
  });
  const items = query.data?.pages.flatMap((page) => page.items) ?? [];

  const categoriesQuery = useQuery({
    queryKey: ["agent-categories"],
    queryFn: () => api.listAgents(),
    staleTime: 60_000,
  });

  const favoriteMutation = useMutation({
    mutationFn: ({ id, isFavorite }: { id: string; isFavorite: boolean }) =>
      api.setAgentFavorite(id, isFavorite),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["agents"] });
    },
  });

  useEffect(() => {
    const initial = items.find((agent) => agent.id === selectedId);
    if (initial) onSelect(initial);
  }, [items, selectedId]);

  const categoryOptions = useMemo<SelectOption[]>(() => {
    const rawCategories = Array.from(
      new Set(
        (categoriesQuery.data?.items ?? [])
          .map((agent) => agent.category)
          .filter((cat): cat is string => Boolean(cat)),
      ),
    ).sort();

    return [
      { value: "", label: "All categories" },
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
        <span className="result-count">{items.length} available</span>
      </div>

      <div className="picker-filters">
        <TextInput
          label="Search agents"
          value={search}
          onChange={(event) => setFilter("search", event.target.value)}
          placeholder="Search by name or description"
          icon={<Search size={16} />}
        />

        <div className="picker-select-wrap">
          <Select
            label="Category"
            options={categoryOptions}
            value={category}
            onChange={(value) => setFilter("category", value)}
            placeholder="All categories"
          />
        </div>
      </div>

      <div className="agent-collection-tabs" role="tablist" aria-label="Agent collection">
        {([
          ["all", "All"],
          ["favorites", "Favorites"],
          ["recent", "Recent"],
        ] as const).map(([value, label]) => (
          <button
            key={value}
            type="button"
            role="tab"
            aria-selected={collection === value}
            className={collection === value ? "agent-collection-tab-active" : ""}
            onClick={() => setFilter("collection", value)}
          >
            {label}
          </button>
        ))}
      </div>

      {query.isLoading ? <LoadingState label="Loading agents" /> : null}
      {query.isError ? (
        <ErrorState
          message={
            query.error instanceof Error
              ? query.error.message
              : "The agent catalog could not be loaded."
          }
          onRetry={() => void query.refetch()}
        />
      ) : null}
      {!query.isLoading && !query.isError && items.length === 0 ? (
        <EmptyState
          title="No matching agents"
          detail="Try another search term or clear category filter."
        />
      ) : null}
      {favoriteMutation.isError ? (
        <p className="inline-error" role="alert">
          The favorite preference could not be updated.
        </p>
      ) : null}

      <div className="agent-list">
        {items.map((agent) => {
          const isSelected = selectedId === agent.id;
          return (
            <div
              key={agent.id}
              className={`agent-option ${isSelected ? "agent-option-selected" : ""}`}
            >
              <button
                type="button"
                className="agent-option-main"
                onClick={() => onSelect(agent)}
              >
                <span className="agent-symbol" aria-hidden="true">
                  {agent.display_name.slice(0, 1).toUpperCase()}
                </span>
                <span className="agent-option-copy">
                  <strong>{agent.display_name}</strong>
                  <span>{agent.description}</span>
                  {agent.owner?.display_name ? (
                    <small>Owner: {agent.owner.display_name}</small>
                  ) : null}
                </span>
                {isSelected ? (
                  <Check
                    size={18}
                    className="agent-check-icon"
                    aria-label="Selected"
                  />
                ) : null}
              </button>
              <IconButton
                label={agent.is_favorite ? "Remove from favorites" : "Add to favorites"}
                size="sm"
                variant={agent.is_favorite ? "active" : "quiet"}
                disabled={favoriteMutation.isPending}
                onClick={() =>
                  favoriteMutation.mutate({
                    id: agent.id,
                    isFavorite: !agent.is_favorite,
                  })
                }
              >
                <Star size={16} fill={agent.is_favorite ? "currentColor" : "none"} />
              </IconButton>
            </div>
          );
        })}
      </div>
      {query.hasNextPage ? (
        <div className="list-more">
          <Button
            variant="secondary"
            onClick={() => void query.fetchNextPage()}
            disabled={query.isFetchingNextPage}
          >
            {query.isFetchingNextPage ? "Loading..." : "Load more"}
          </Button>
        </div>
      ) : null}
    </section>
  );
}
