import { useQuery } from '@tanstack/react-query';
import { capabilitiesApi } from '@/lib/api';
import { queryKeys } from '@/lib/query-client';

export function useCapabilities() {
  return useQuery({
    queryKey: queryKeys.capabilities.get(),
    queryFn: () => capabilitiesApi.get(),
    staleTime: Infinity,
    gcTime: Infinity,
  });
}
