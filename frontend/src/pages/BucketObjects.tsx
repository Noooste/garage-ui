import { useEffect, useRef, useState } from 'react';
import { useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { ObjectBrowserView } from '@/components/buckets/ObjectBrowserView';
import { useBucketObjects } from '@/hooks/useBucketObjects';
import { useBuckets } from '@/hooks/useApi';
import { useBucketCan } from '@/hooks/usePermissions';

export function BucketObjects() {
  const { bucketName = '' } = useParams<{ bucketName: string }>();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();

  const { data: buckets = [] } = useBuckets();
  const bucket = buckets.find((b) => b.name === bucketName);
  const canBucket = useBucketCan();
  const canWrite = canBucket(bucket, 'object.write');
  const canDelete = canBucket(bucket, 'object.delete');

  const [currentPath, setCurrentPath] = useState(searchParams.get('prefix') ?? '');
  const [searchQuery, setSearchQuery] = useState('');
  const [deepSearch, setDeepSearch] = useState(false);
  const [initialPageToken, setInitialPageToken] = useState<string | undefined>(
    searchParams.get('page') ?? undefined,
  );
  const [initialItemsPerPage, setInitialItemsPerPage] = useState<number>(
    parseInt(searchParams.get('limit') ?? '25', 10),
  );

  useEffect(() => {
    const prefix = searchParams.get('prefix') ?? '';
    if (prefix !== currentPath) setCurrentPath(prefix);
    setInitialPageToken(searchParams.get('page') ?? undefined);
    setInitialItemsPerPage(parseInt(searchParams.get('limit') ?? '25', 10));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchParams]);

  const {
    objects,
    debouncedSearch,
    isLoading,
    isRefreshing,
    isNavigating,
    isTruncated,
    nextContinuationToken,
    itemsPerPage,
    setItemsPerPage,
    uploadFiles,
    uploadTasks,
    deleteObject,
    deleteMultipleObjects,
    createDirectory,
    fetchObjects,
  } = useBucketObjects(bucketName, currentPath, searchQuery, deepSearch);

  const handleNavigateToFolder = (path: string) => {
    setCurrentPath(path);
    // Navigating to a folder should show that folder's contents, not a stale
    // filter carried over from the folder we came from.
    setSearchQuery('');
    const next = new URLSearchParams();
    if (path) next.set('prefix', path);
    setSearchParams(next);
  };

  const handlePageChange = (token?: string) => {
    fetchObjects(token);
    const next = new URLSearchParams();
    if (currentPath) next.set('prefix', currentPath);
    if (token) next.set('page', token);
    if (itemsPerPage !== 25) next.set('limit', String(itemsPerPage));
    setSearchParams(next);
  };

  const handleItemsPerPageChange = (count: number) => {
    setItemsPerPage(count);
    const next = new URLSearchParams();
    if (currentPath) next.set('prefix', currentPath);
    if (count !== 25) next.set('limit', String(count));
    setSearchParams(next);
  };

  const handleRefresh = async () => {
    await fetchObjects(undefined, true);
  };
  const handleBackToBuckets = () => navigate('/buckets');

  // CustomEvent bridge for the Upload button in BucketDetailShell hero.
  const uploadInputRef = useRef<HTMLInputElement>(null);
  useEffect(() => {
    const handler = () => {
      if (canWrite) uploadInputRef.current?.click();
    };
    document.addEventListener('bucket:upload', handler);
    return () => document.removeEventListener('bucket:upload', handler);
  }, [canWrite]);

  return (
    <>
      <input
        ref={uploadInputRef}
        type="file"
        multiple
        hidden
        onChange={async (e) => {
          const files = Array.from(e.target.files ?? []);
          if (files.length > 0) await uploadFiles(files);
          e.target.value = '';
        }}
      />
      <ObjectBrowserView
        bucketName={bucketName}
        objects={objects}
        currentPath={currentPath}
        searchQuery={searchQuery}
        filterQuery={debouncedSearch}
        deepSearch={deepSearch}
        isLoading={isLoading}
        isTruncated={isTruncated}
        nextContinuationToken={nextContinuationToken}
        itemsPerPage={itemsPerPage}
        onSearchChange={setSearchQuery}
        onDeepSearchChange={setDeepSearch}
        onNavigateToFolder={handleNavigateToFolder}
        onBackToBuckets={handleBackToBuckets}
        onUploadFiles={canWrite ? uploadFiles : undefined}
        uploadTasks={uploadTasks}
        onDeleteObject={canDelete ? deleteObject : undefined}
        onDeleteMultipleObjects={canDelete ? deleteMultipleObjects : undefined}
        onCreateDirectory={canWrite ? createDirectory : undefined}
        onRefresh={handleRefresh}
        onPageChange={handlePageChange}
        onItemsPerPageChange={handleItemsPerPageChange}
        isRefreshing={isRefreshing}
        isNavigating={isNavigating}
        initialPageToken={initialPageToken}
        initialItemsPerPage={initialItemsPerPage}
      />
    </>
  );
}
