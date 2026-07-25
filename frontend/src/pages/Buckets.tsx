import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { Plus } from 'lucide-react';
import { useBuckets, useCreateBucket, useDeleteBucket } from '@/hooks/useApi';
import { usePermissions } from '@/hooks/usePermissions';
import { accessApi, bucketsApi } from '@/lib/api';
import { queryKeys } from '@/lib/query-client';
import { createBucketWithKey, type NewKeyRequest } from '@/lib/create-bucket-with-key';
import { BucketListView } from '@/components/buckets/BucketListView';
import { CreateBucketDialog } from '@/components/buckets/CreateBucketDialog';
import { DangerousConfirmDialog } from '@/components/ui/dangerous-confirm-dialog';
import { PageHeader } from '@/components/ui/page-header';
import { Button } from '@/components/ui/button';
import type { Bucket } from '@/types';

export function Buckets() {
  const navigate = useNavigate();
  const [searchQuery, setSearchQuery] = useState('');
  const [createOpen, setCreateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Bucket | null>(null);
  const [deleting, setDeleting] = useState(false);

  const queryClient = useQueryClient();
  const { hasAnyPerm } = usePermissions();
  const { data: buckets = [], isLoading } = useBuckets();
  const createMutation = useCreateBucket();
  const deleteMutation = useDeleteBucket();

  // Both grant permissions are required by POST /buckets/:name/permissions, and
  // the bucket does not exist yet, so this is a best-effort check across the
  // subject's bindings. A 403 at call time surfaces in the dialog's outcome panel.
  const canCreateKey =
    hasAnyPerm('key.create') &&
    hasAnyPerm('permission.allow_bucket_key') &&
    hasAnyPerm('permission.deny_bucket_key');

  const createBucket = async (name: string, key?: NewKeyRequest) => {
    const result = await createBucketWithKey(
      {
        createBucket: (n) => createMutation.mutateAsync({ name: n }),
        // Called directly, not through useCreateAccessKey and
        // useGrantBucketPermission: their success toasts would stack on top of
        // the dialog's own outcome panel.
        createKey: (n) => accessApi.createKey(n),
        grant: (bucket, accessKeyId, permissions) =>
          bucketsApi.grantPermission(bucket, accessKeyId, permissions),
      },
      name,
      key,
    );

    if (result.key) {
      queryClient.invalidateQueries({ queryKey: queryKeys.accessKeys.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.buckets.detail(name) });
    }

    return result;
  };

  const confirmDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await deleteMutation.mutateAsync(deleteTarget.name);
      setDeleteTarget(null);
    } catch {
      // error toast handled by axios interceptor
    } finally {
      setDeleting(false);
    }
  };

  return (
    <div>
      <PageHeader
        title="Buckets"
        subtitle={`${buckets.length} bucket${buckets.length === 1 ? '' : 's'}`}
        actions={
          hasAnyPerm('bucket.create') && (
            <Button onClick={() => setCreateOpen(true)}>
              <Plus /> Create bucket
            </Button>
          )
        }
      />
      <div className="p-4 sm:p-6">
        <BucketListView
          buckets={buckets}
          searchQuery={searchQuery}
          isLoading={isLoading}
          onSearchChange={setSearchQuery}
          onViewBucket={(name) => navigate(`/buckets/${name}/objects`)}
          onOpenSettings={(b) => navigate(`/buckets/${b.name}/settings`)}
          onWebsiteSettings={(b) => navigate(`/buckets/${b.name}/website`)}
          onDeleteBucket={(b) => setDeleteTarget(b)}
        />
      </div>

      <CreateBucketDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreateBucket={createBucket}
        canCreateKey={canCreateKey}
      />

      <DangerousConfirmDialog
        open={!!deleteTarget}
        onOpenChange={(o) => !o && setDeleteTarget(null)}
        title={deleteTarget ? `Delete bucket "${deleteTarget.name}"?` : ''}
        description="All objects in this bucket will be permanently removed."
        confirmationText={deleteTarget?.name ?? ''}
        confirmLabel="Delete bucket"
        loading={deleting}
        onConfirm={confirmDelete}
      />
    </div>
  );
}
