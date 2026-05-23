import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { bucketsApi, objectsApi, accessApi, garageApi, analyticsApi } from '@/lib/api';
import { queryKeys } from '@/lib/query-client';
import { toast } from 'sonner';


export function useBuckets() {
  return useQuery({
    queryKey: queryKeys.buckets.list(),
    queryFn: () => bucketsApi.list(),
  });
}

export function useBucket(name: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.buckets.detail(name),
    queryFn: () => bucketsApi.get(name),
    enabled: enabled && !!name,
  });
}

export function useCreateBucket() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ name, region }: { name: string; region?: string }) =>
      bucketsApi.create(name, region),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.buckets.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.dashboard.all });
      toast.success('Bucket created successfully');
    },
  });
}

export function useDeleteBucket() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (name: string) => bucketsApi.delete(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.buckets.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.dashboard.all });
      toast.success('Bucket deleted successfully');
    },
  });
}

export function useGrantBucketPermission() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ bucketName, accessKeyId, permissions }: {
      bucketName: string;
      accessKeyId: string;
      permissions: { read: boolean; write: boolean; owner: boolean };
    }) => bucketsApi.grantPermission(bucketName, accessKeyId, permissions),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.buckets.detail(variables.bucketName) });
      queryClient.invalidateQueries({ queryKey: queryKeys.accessKeys.all });
      toast.success('Permissions granted successfully');
    },
  });
}

export function useUpdateBucketQuotas() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      bucketName,
      maxSize,
      maxObjects,
    }: {
      bucketName: string;
      maxSize: number | null;
      maxObjects: number | null;
    }) => bucketsApi.updateBucketQuotas(bucketName, { maxSize, maxObjects }),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.buckets.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.buckets.detail(variables.bucketName) });
      toast.success('Quotas updated successfully');
    },
  });
}


export function useObjects(bucket: string, prefix?: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.objects.list(bucket, prefix),
    queryFn: () => objectsApi.list(bucket, prefix),
    enabled: enabled && !!bucket,
  });
}

export function useUploadObject() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ bucket, key, file }: { bucket: string; key: string; file: File }) =>
      objectsApi.upload(bucket, key, file),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.objects.list(variables.bucket) });
      queryClient.invalidateQueries({ queryKey: queryKeys.buckets.detail(variables.bucket) });
      queryClient.invalidateQueries({ queryKey: queryKeys.dashboard.all });
      toast.success('File uploaded successfully');
    },
  });
}

export function useUploadMultipleObjects() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ bucket, files }: { bucket: string; files: File[] }) =>
      objectsApi.uploadMultiple(bucket, files),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.objects.list(variables.bucket) });
      queryClient.invalidateQueries({ queryKey: queryKeys.buckets.detail(variables.bucket) });
      queryClient.invalidateQueries({ queryKey: queryKeys.dashboard.all });
      toast.success('Files uploaded successfully');
    },
  });
}

export function useDeleteObject() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ bucket, key }: { bucket: string; key: string }) =>
      objectsApi.delete(bucket, key),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.objects.list(variables.bucket) });
      queryClient.invalidateQueries({ queryKey: queryKeys.buckets.detail(variables.bucket) });
      queryClient.invalidateQueries({ queryKey: queryKeys.dashboard.all });
      toast.success('File deleted successfully');
    },
  });
}

export function useDeleteMultipleObjects() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ bucket, keys, prefix }: { bucket: string; keys: string[]; prefix?: string }) =>
      objectsApi.deleteMultiple(bucket, keys, prefix),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.objects.list(variables.bucket) });
      queryClient.invalidateQueries({ queryKey: queryKeys.buckets.detail(variables.bucket) });
      queryClient.invalidateQueries({ queryKey: queryKeys.dashboard.all });
      toast.success(`${variables.keys.length} files deleted successfully`);
    },
  });
}


export function useAccessKeys() {
  return useQuery({
    queryKey: queryKeys.accessKeys.list(),
    queryFn: () => accessApi.listKeys(),
  });
}

export function useAccessKey(keyId: string, enabled = true) {
  return useQuery({
    queryKey: queryKeys.accessKeys.detail(keyId),
    queryFn: () => accessApi.getKey(keyId),
    enabled: enabled && !!keyId,
  });
}

export function useCreateAccessKey() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ name, permissions }: { name: string; permissions?: any[] }) =>
      accessApi.createKey(name, permissions),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.accessKeys.all });
      toast.success('Access key created successfully');
    },
  });
}

export function useDeleteAccessKey() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (keyId: string) => accessApi.deleteKey(keyId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.accessKeys.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.buckets.all });
      toast.success('Access key deleted successfully');
    },
  });
}

export function useUpdateAccessKey() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ keyId, updates }: { keyId: string; updates: any }) =>
      accessApi.updateKey(keyId, updates),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.accessKeys.detail(variables.keyId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.accessKeys.list() });
      toast.success('Access key updated successfully');
    },
  });
}


export function useClusterHealth() {
  return useQuery({
    queryKey: queryKeys.cluster.health(),
    queryFn: () => garageApi.getClusterHealth(),
    staleTime: 30 * 1000, // Refresh health every 30 seconds
  });
}

export function useClusterStatus() {
  return useQuery({
    queryKey: queryKeys.cluster.status(),
    queryFn: () => garageApi.getClusterStatus(),
    staleTime: 60 * 1000, // Refresh status every minute
  });
}

export function useClusterStatistics() {
  return useQuery({
    queryKey: queryKeys.cluster.statistics(),
    queryFn: () => garageApi.getClusterStatistics(),
    staleTime: 60 * 1000, // Refresh statistics every minute
  });
}


export function useDashboardMetrics() {
  return useQuery({
    queryKey: queryKeys.dashboard.metrics(),
    queryFn: () => analyticsApi.getMetrics(),
    staleTime: 2 * 60 * 1000, // Refresh dashboard every 2 minutes
  });
}

// Combined hook for dashboard data
export function useDashboardData() {
  const metrics = useDashboardMetrics();
  const buckets = useBuckets();
  const health = useClusterHealth();

  return {
    metrics,
    buckets,
    health,
    isLoading: metrics.isLoading || buckets.isLoading || health.isLoading,
    isError: metrics.isError || buckets.isError || health.isError,
  };
}
