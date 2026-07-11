import { useState, useEffect, useCallback, useRef } from 'react';
import { objectsApi } from '@/lib/api';
import type { S3Object, UploadTask } from '@/types';
import { toast } from 'sonner';

// How long to wait after the last keystroke before actually searching. Keeps
// typing from firing a request (and a client-side re-filter) on every key.
const SEARCH_DEBOUNCE_MS = 750;

export function useBucketObjects(bucketName: string | null, currentPath: string = '', searchQuery: string = '', deepSearch: boolean = false) {
  const [objects, setObjects] = useState<S3Object[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [isNavigating, setIsNavigating] = useState(false);
  const [error, setError] = useState<Error | null>(null);
  const [isTruncated, setIsTruncated] = useState(false);
  const [nextContinuationToken, setNextContinuationToken] = useState<string | undefined>(undefined);
  const [itemsPerPage, setItemsPerPage] = useState(25);
  const [currentContinuationToken, setCurrentContinuationToken] = useState<string | undefined>(undefined);
  const [debouncedSearch, setDebouncedSearch] = useState('');
  const previousPathRef = useRef<string>(currentPath);
  const [uploadTasks, setUploadTasks] = useState<UploadTask[]>([]);
  const clearTasksTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  // Monotonic sequence guarding against stale responses: when a newer fetch or
  // search starts, older in-flight responses are discarded instead of clobbering
  // the current view (e.g. a slow search resolving after the query was cleared).
  const fetchSeqRef = useRef(0);

  // Prefix search (the default) narrows the Garage listing to keys starting with
  // the query, within the current folder — server-side, paginated, and O(matches)
  // like the AWS S3 / R2 consoles. Deep search instead uses a recursive scan
  // (see searchObjects) and does not touch listPrefix.
  const listPrefix = debouncedSearch && !deepSearch ? currentPath + debouncedSearch : currentPath;

  const fetchObjects = useCallback(async (continuationToken?: string, isRefresh = false, isNav = false) => {
    if (!bucketName) return;

    const seq = ++fetchSeqRef.current;
    try {
      if (isRefresh) {
        setIsRefreshing(true);
      } else if (isNav) {
        setIsNavigating(true);
      } else {
        setIsLoading(true);
      }
      setError(null);
      const response = await objectsApi.list(bucketName, listPrefix, itemsPerPage, continuationToken);
      if (seq !== fetchSeqRef.current) return;
      setObjects(response.objects);
      setIsTruncated(response.isTruncated);
      setNextContinuationToken(response.nextContinuationToken);
      setCurrentContinuationToken(continuationToken);
    } catch (err) {
      if (seq !== fetchSeqRef.current) return;
      setError(err as Error);
      console.error('Failed to fetch objects:', err);
    } finally {
      if (seq === fetchSeqRef.current) {
        setIsLoading(false);
        setIsRefreshing(false);
        setIsNavigating(false);
      }
    }
  }, [bucketName, listPrefix, itemsPerPage]);

  const searchObjects = useCallback(async (query: string) => {
    if (!bucketName) return;

    const seq = ++fetchSeqRef.current;
    try {
      setIsLoading(true);
      setError(null);
      const response = await objectsApi.search(bucketName, query, currentPath || undefined);
      if (seq !== fetchSeqRef.current) return;
      setObjects(response.objects);
      setIsTruncated(response.isTruncated);
      // Search results are not token-paginated.
      setNextContinuationToken(undefined);
      setCurrentContinuationToken(undefined);
    } catch (err) {
      if (seq !== fetchSeqRef.current) return;
      setError(err as Error);
      console.error('Failed to search objects:', err);
    } finally {
      if (seq === fetchSeqRef.current) setIsLoading(false);
    }
  }, [bucketName, currentPath]);

  // Debounce the search query so we don't fire a recursive scan per keystroke.
  useEffect(() => {
    const t = setTimeout(() => setDebouncedSearch(searchQuery.trim()), SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(t);
  }, [searchQuery]);

  useEffect(() => {
    if (!bucketName) return;

    // Deep search: recursive substring scan across the current subtree.
    if (debouncedSearch && deepSearch) {
      searchObjects(debouncedSearch);
      return;
    }

    // Normal listing, or prefix-filtered listing (listPrefix carries the query).
    const isPathChange = previousPathRef.current !== currentPath && objects.length > 0;
    previousPathRef.current = currentPath;

    fetchObjects(undefined, false, isPathChange);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [bucketName, currentPath, itemsPerPage, debouncedSearch, deepSearch]);

  useEffect(() => {
    return () => {
      if (clearTasksTimerRef.current) clearTimeout(clearTasksTimerRef.current);
    };
  }, []);

  const uploadFiles = useCallback(async (files: File[]) => {
    if (!bucketName) return false;

    const hasRelativePaths = files.some((file) => !!file.webkitRelativePath);

    const folders = new Set<string>();
    files.forEach((file) => {
      if (file.webkitRelativePath) {
        const parts = file.webkitRelativePath.split('/');
        if (parts.length > 1) {
          folders.add(parts[0]);
        }
      }
    });

    const tasks: UploadTask[] = files.map((file, index) => {
      const relativePath = file.webkitRelativePath || file.name;
      const key = currentPath ? `${currentPath}${relativePath}` : relativePath;
      return {
        id: `${Date.now()}-${index}`,
        file,
        key,
        bucket: bucketName,
        progress: 0,
        status: 'pending' as const,
      };
    });

    setUploadTasks(tasks);

    const results = await Promise.all(tasks.map(async (task) => {
      try {
        setUploadTasks(prev => prev.map(t =>
          t.id === task.id ? { ...t, status: 'uploading' as const } : t
        ));

        await objectsApi.upload(bucketName, task.key, task.file, (progress) => {
          setUploadTasks(prev => prev.map(t => {
            if (t.id !== task.id || t.progress === progress) return t;
            return { ...t, progress };
          }));
        });

        setUploadTasks(prev => prev.map(t =>
          t.id === task.id ? { ...t, status: 'completed' as const, progress: 100 } : t
        ));
        return true;
      } catch (error) {
        const errorMessage = error instanceof Error ? error.message : 'Upload failed';
        setUploadTasks(prev => prev.map(t =>
          t.id === task.id ? { ...t, status: 'error' as const, error: errorMessage } : t
        ));
        console.error(`Failed to upload ${task.key}:`, error);
        return false;
      }
    }));

    const successCount = results.filter(Boolean).length;
    const errorCount = results.length - successCount;

    if (errorCount === 0) {
      if (hasRelativePaths && folders.size > 0) {
        const folderNames = Array.from(folders).join(', ');
        toast.success(`Successfully uploaded ${successCount} file${successCount > 1 ? 's' : ''} from ${folders.size} folder${folders.size > 1 ? 's' : ''} (${folderNames})`);
      } else {
        toast.success(`Successfully uploaded ${successCount} file${successCount > 1 ? 's' : ''}`);
      }
    } else if (successCount > 0) {
      toast.warning(`Uploaded ${successCount} file${successCount > 1 ? 's' : ''}, ${errorCount} failed`);
    } else {
      toast.error(`Failed to upload ${errorCount} file${errorCount > 1 ? 's' : ''}`);
    }

    if (clearTasksTimerRef.current) clearTimeout(clearTasksTimerRef.current);
    clearTasksTimerRef.current = setTimeout(() => {
      setUploadTasks([]);
      clearTasksTimerRef.current = null;
    }, 3000);

    await fetchObjects(currentContinuationToken, true);
    return successCount > 0;
  }, [bucketName, currentPath, currentContinuationToken, fetchObjects]);

  const deleteObject = useCallback(async (key: string) => {
    if (!bucketName) return false;

    try {
      setObjects(prev => prev.filter(obj => obj.key !== key));

      await objectsApi.delete(bucketName, key);
      toast.success(`Object "${key}" deleted successfully`);
      await fetchObjects(currentContinuationToken, true);
      return true;
    } catch (error) {
      console.error('Delete object error:', error);
      await fetchObjects(currentContinuationToken, true);
      return false;
    }
  }, [bucketName, currentContinuationToken, fetchObjects]);

  const deleteMultipleObjects = useCallback(async (keys: string[]) => {
    if (!bucketName || keys.length === 0) return false;

    try {
      setObjects(prev => prev.filter(obj => !keys.includes(obj.key)));

      await objectsApi.deleteMultiple(bucketName, keys, currentPath || undefined);
      toast.success(`Successfully deleted ${keys.length} file${keys.length > 1 ? 's' : ''}`);
      await fetchObjects(currentContinuationToken, true);
      return true;
    } catch (error) {
      console.error('Bulk delete error:', error);
      await fetchObjects(currentContinuationToken, true);
      return false;
    }
  }, [bucketName, currentPath, currentContinuationToken, fetchObjects]);

  const createDirectory = useCallback(async (dirName: string) => {
    if (!bucketName) return false;

    try {
      const dirKey = currentPath ? `${currentPath}${dirName}/` : `${dirName}/`;
      await objectsApi.createDirectory(bucketName, dirKey);
      toast.success(`Directory "${dirName}" created successfully`);
      await fetchObjects(currentContinuationToken, true);
      return true;
    } catch (error) {
      console.error('Create directory error:', error);
      return false;
    }
  }, [bucketName, currentPath, currentContinuationToken, fetchObjects]);

  return {
    objects,
    // The debounced query the current results reflect — use this (not the raw
    // input) to filter/label results so the view waits instead of twitching.
    debouncedSearch,
    isLoading,
    isRefreshing,
    isNavigating,
    error,
    isTruncated,
    nextContinuationToken,
    currentContinuationToken,
    itemsPerPage,
    setItemsPerPage,
    fetchObjects,
    uploadFiles,
    uploadTasks,
    deleteObject,
    deleteMultipleObjects,
    createDirectory,
  };
}
