import { useState, useCallback, useMemo } from 'react';
import { useQuery } from '@tanstack/react-query';

interface RegistryOptions<T, F> {
  key: string;
  fetcher: (params: { page: number; limit: number; q?: string; filters?: F }) => Promise<{
    items: T[];
    total: number;
    page: number;
    limit: number;
    has_next: boolean;
  }>;
  initialFilters?: F;
  defaultLimit?: number;
  getItemId?: (item: T) => string | undefined;
}

/**
 * Enterprise-grade hook for managing Registry (List) state and data.
 */
export function useRegistry<T, F = Record<string, unknown>>({
  key,
  fetcher,
  initialFilters,
  defaultLimit = 20,
  getItemId,
}: RegistryOptions<T, F>) {
  const [page, setPage] = useState(1);
  const [limit, setLimit] = useState(defaultLimit);
  const [query, setQuery] = useState('');
  const [filters, setFilters] = useState<F | undefined>(initialFilters);
  const [selectedIDs, setSelectedIDs] = useState<string[]>([]);

  // TanStack Query for automatic caching and life-cycle management
  const { data, isLoading, isError, error, refetch, isFetching } = useQuery({
    queryKey: [key, page, limit, query, filters],
    queryFn: () => fetcher({ page, limit, q: query, filters: filters as F }),
  });

  const items = useMemo(() => data?.items || [], [data]);
  const total = data?.total || 0;
  const hasNext = data?.has_next || false;

  const handlePageChange = useCallback((p: number) => {
    setPage(p);
    setSelectedIDs([]);
  }, []);

  const handleLimitChange = useCallback((l: number) => {
    setLimit(l);
    setPage(1);
    setSelectedIDs([]);
  }, []);

  const handleSearch = useCallback((q: string) => {
    setQuery(q);
    setPage(1);
    setSelectedIDs([]);
  }, []);

  const updateFilters = useCallback((nextFilters: Partial<F>) => {
    setFilters((current) => ({ ...current, ...nextFilters } as F));
    setPage(1);
    setSelectedIDs([]);
  }, []);

  const toggleSelection = useCallback((id: string) => {
    setSelectedIDs((current) => 
      current.includes(id) ? current.filter((i) => i !== id) : [...current, id]
    );
  }, []);

  const selectAll = useCallback(() => {
    const allIDs = items.map((i: unknown) => {
      if (getItemId) {
        return getItemId(i as T);
      }
      const item = i as Record<string, string>;
      return item.id || item.user_id || item.group_id;
    }).filter((id): id is string => !!id);
    
    setSelectedIDs((current) => 
      current.length === items.length ? [] : allIDs
    );
  }, [getItemId, items]);

  return {
    // Data
    items,
    total,
    page,
    limit,
    hasNext,
    
    // States
    loading: isLoading || isFetching,
    error: isError ? (error as Error).message : null,
    query,
    filters: (filters || {}) as F,
    selectedIDs,
    
    // Setters
    setPage: handlePageChange,
    setLimit: handleLimitChange,
    setQuery: handleSearch,
    setFilters: updateFilters,
    setSelectedIDs,
    toggleSelection,
    selectAll,
    refresh: refetch,
  };
}
