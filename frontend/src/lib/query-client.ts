import {QueryClient} from '@tanstack/react-query';

// Create a query client with default options
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5 * 60 * 1000, // Data is fresh for 5 minutes
      gcTime: 10 * 60 * 1000, // Cache data for 10 minutes (formerly cacheTime)
      retry: 1, // Retry failed requests once
      refetchOnWindowFocus: false, // Don't refetch when window regains focus
      refetchOnMount: false, // Don't refetch on component mount if data exists
      placeholderData: (previousData: unknown) => previousData, // Keep previous data while fetching new data
    },
  },
});

// Query keys for consistent cache management
export const queryKeys = {
  buckets: {
    all: ['buckets'] as const,
    list: () => [...queryKeys.buckets.all, 'list'] as const,
    detail: (name: string) => [...queryKeys.buckets.all, 'detail', name] as const,
  },
  objects: {
    all: ['objects'] as const,
    list: (bucket: string, prefix?: string) => [...queryKeys.objects.all, 'list', bucket, prefix] as const,
  },
  accessKeys: {
    all: ['accessKeys'] as const,
    list: () => [...queryKeys.accessKeys.all, 'list'] as const,
    detail: (keyId: string) => [...queryKeys.accessKeys.all, 'detail', keyId] as const,
  },
  cluster: {
    all: ['cluster'] as const,
    health: () => [...queryKeys.cluster.all, 'health'] as const,
    status: () => [...queryKeys.cluster.all, 'status'] as const,
    statistics: () => [...queryKeys.cluster.all, 'statistics'] as const,
  },
  dashboard: {
    all: ['dashboard'] as const,
    metrics: () => [...queryKeys.dashboard.all, 'metrics'] as const,
  },
};
