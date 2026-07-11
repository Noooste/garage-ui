import { useEffect, useState } from 'react';
import { useNavigate, useParams, Link } from 'react-router-dom';
import { objectsApi } from '@/lib/api';
import { useBuckets } from '@/hooks/useApi';
import { useBucketCan } from '@/hooks/usePermissions';
import type { ObjectMetadata } from '@/types';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { IconTile } from '@/components/ui/icon-tile';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import { ArrowLeft, ChevronRight, Copy, Download, File, Loader2, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { downloadObject, formatBytes } from '@/lib/file-utils';
import { formatDate } from '@/lib/utils';

function CardSection({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="overflow-hidden rounded-xl border border-[var(--border)] bg-[var(--card)]">
      <div className="border-b border-[var(--border)] px-5 py-3.5">
        <h3 className="text-[14px] font-semibold tracking-[-0.01em]">{title}</h3>
      </div>
      {children}
    </section>
  );
}

function DetailRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="grid grid-cols-1 gap-1 px-5 py-3.5 sm:grid-cols-[200px_1fr] sm:gap-4">
      <dt className="text-[12.5px] font-medium text-[var(--muted-foreground)]">{label}</dt>
      <dd className="text-[13.5px] text-[var(--foreground)] break-words">{children}</dd>
    </div>
  );
}

export function ObjectDetailsView() {
  const navigate = useNavigate();
  const { bucketName, '*': encodedObjectKey } = useParams();
  const objectKey = encodedObjectKey ? decodeURIComponent(encodedObjectKey) : undefined;

  const { data: buckets = [] } = useBuckets();
  const bucket = buckets.find((b) => b.name === bucketName);
  const canBucket = useBucketCan();
  const canDelete = canBucket(bucket, 'object.delete');

  const [metadata, setMetadata] = useState<ObjectMetadata | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);

  useEffect(() => {
    if (!bucketName || !objectKey) {
      setError('Bucket name and object key are required');
      setIsLoading(false);
      return;
    }
    const fetchMetadata = async () => {
      try {
        setIsLoading(true);
        setError(null);
        const data = await objectsApi.getMetadata(bucketName, objectKey);
        setMetadata(data);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to load object metadata');
      } finally {
        setIsLoading(false);
      }
    };
    fetchMetadata();
  }, [bucketName, objectKey]);

  const parentPath = objectKey?.split('/').slice(0, -1).join('/') ?? '';
  const fileName = objectKey?.split('/').pop() || objectKey || '';
  const backHref = `/buckets/${bucketName}/objects${parentPath ? `?prefix=${encodeURIComponent(parentPath + '/')}` : ''}`;
  const pathSegments = parentPath ? parentPath.split('/').filter(Boolean) : [];

  const copy = (text: string, label = 'Copied') => {
    navigator.clipboard.writeText(text);
    toast.success(label);
  };

  const handleDownload = () => {
    if (!bucketName || !objectKey) return;
    downloadObject(bucketName, objectKey);
  };

  const handleDelete = async () => {
    if (!bucketName || !objectKey) return;
    try {
      setDeleting(true);
      await objectsApi.delete(bucketName, objectKey);
      toast.success('Object deleted');
      navigate(backHref);
    } catch {
      // error toast handled by axios interceptor
    } finally {
      setDeleting(false);
      setDeleteOpen(false);
    }
  };

  if (isLoading) {
    return (
      <div className="flex h-64 items-center justify-center gap-2 text-[var(--muted-foreground)]">
        <Loader2 className="h-4 w-4 animate-spin" /> Loading object details…
      </div>
    );
  }

  if (error || !metadata) {
    return (
      <div className="px-7 py-6">
        <Button variant="secondary" onClick={() => navigate(backHref)} className="mb-4">
          <ArrowLeft className="h-4 w-4" /> Back
        </Button>
        <div className="rounded-xl border border-[var(--danger-border)] bg-[var(--danger-soft)] px-5 py-4 text-[13.5px] text-[var(--destructive)]">
          {error || 'Object not found'}
        </div>
      </div>
    );
  }

  return (
    <div className="px-7 py-6 space-y-6">
      {/* Back + breadcrumb */}
      <div className="flex items-center gap-2 text-[13px] text-[var(--muted-foreground)]">
        <Link
          to={backHref}
          className="inline-flex items-center gap-1.5 rounded-md px-2 py-1 hover:bg-[var(--accent)] hover:text-[var(--foreground)]"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          Objects
        </Link>
        {pathSegments.map((seg, i) => (
          <span key={i} className="inline-flex items-center gap-1">
            <ChevronRight className="h-3.5 w-3.5 opacity-50" />
            <span className="font-mono">{seg}</span>
          </span>
        ))}
        <ChevronRight className="h-3.5 w-3.5 opacity-50" />
        <span className="truncate font-mono text-[var(--foreground)]">{fileName}</span>
      </div>

      {/* Hero */}
      <section className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex min-w-0 items-start gap-3">
          <IconTile icon={<File />} tone="primary" size="lg" />
          <div className="min-w-0">
            <h1 className="truncate text-[22px] font-semibold tracking-[-0.02em]">{fileName}</h1>
            <button
              type="button"
              onClick={() => copy(metadata.key, 'Object key copied')}
              title="Copy key"
              className="group mt-1 inline-flex max-w-full items-center gap-1.5 truncate font-mono text-[13px] text-[var(--muted-foreground)] hover:text-[var(--foreground)]"
            >
              <span className="truncate">{metadata.key}</span>
              <Copy className="h-3 w-3 flex-shrink-0 opacity-60 group-hover:opacity-100" />
            </button>
            <div className="mt-2 flex flex-wrap gap-1.5">
              <Badge>{formatBytes(metadata.size)}</Badge>
              <Badge>{metadata.contentType || 'application/octet-stream'}</Badge>
              {metadata.storageClass && <Badge>{metadata.storageClass}</Badge>}
            </div>
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <Button variant="secondary" onClick={handleDownload}>
            <Download className="h-4 w-4" /> Download
          </Button>
          {canDelete && (
            <Button variant="destructive" onClick={() => setDeleteOpen(true)}>
              <Trash2 className="h-4 w-4" /> Delete
            </Button>
          )}
        </div>
      </section>

      {/* Details */}
      <CardSection title="Details">
        <dl className="divide-y divide-[var(--border)]">
          <DetailRow label="Size">{formatBytes(metadata.size)}</DetailRow>
          <DetailRow label="Content type">{metadata.contentType || 'application/octet-stream'}</DetailRow>
          <DetailRow label="Storage class">{metadata.storageClass || 'Standard'}</DetailRow>
          <DetailRow label="Last modified">{formatDate(metadata.lastModified)}</DetailRow>
          <DetailRow label="ETag">
            <button
              type="button"
              onClick={() => copy(metadata.etag, 'ETag copied')}
              className="inline-flex max-w-full items-center gap-1.5 truncate rounded-md bg-[var(--surface-sunken)] px-2 py-0.5 font-mono text-[12.5px] hover:bg-[var(--accent)]"
            >
              <span className="truncate">{metadata.etag}</span>
              <Copy className="h-3 w-3 flex-shrink-0 opacity-60" />
            </button>
          </DetailRow>
          {metadata.versionId && (
            <DetailRow label="Version ID">
              <span className="font-mono text-[12.5px]">{metadata.versionId}</span>
            </DetailRow>
          )}
        </dl>
      </CardSection>

      {/* Custom metadata */}
      {metadata.metadata && Object.keys(metadata.metadata).length > 0 && (
        <CardSection title="Custom metadata">
          <dl className="divide-y divide-[var(--border)]">
            {Object.entries(metadata.metadata).map(([key, value]) => (
              <DetailRow key={key} label={key}>
                <span className="font-mono text-[12.5px]">{value}</span>
              </DetailRow>
            ))}
          </dl>
        </CardSection>
      )}

      {/* Preview */}
      <CardSection title="Preview">
        <div className="px-5 py-10 text-center text-[13px] text-[var(--muted-foreground)]">
          No preview available for this object.
        </div>
      </CardSection>

      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={`Delete "${fileName}"?`}
        description="Applications referencing this object will no longer be able to read it."
        confirmLabel="Delete object"
        loading={deleting}
        onConfirm={handleDelete}
      />
    </div>
  );
}
