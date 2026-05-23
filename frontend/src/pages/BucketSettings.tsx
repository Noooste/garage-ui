import { useMemo, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { AlertTriangle, Gauge, Info } from 'lucide-react';
import { useForm, Controller } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { useBuckets, useDeleteBucket, useUpdateBucketQuotas } from '@/hooks/useApi';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { EmptyState } from '@/components/ui/empty-state';
import { DangerousConfirmDialog } from '@/components/ui/dangerous-confirm-dialog';
import { Switch } from '@/components/ui/switch';
import { Input } from '@/components/ui/input';
import { Select, SelectOption } from '@/components/ui/select';
import { formatBytes } from '@/lib/file-utils';
import { formatDate as formatDateUtil } from '@/lib/utils';
import {
  bytesToQuotaValue,
  quotaValueToBytes,
  QUOTA_UNIT_BYTES,
  type QuotaUnit,
} from '@/lib/quota-utils';

const formatBytesOrDash = (n?: number) => (n == null ? '—' : formatBytes(n));
const formatDateOrDash = (iso?: string) => (iso ? formatDateUtil(iso) : '—');

const quotaFormSchema = z
  .object({
    maxSizeEnabled: z.boolean(),
    maxSizeValue: z.string(),
    maxSizeUnit: z.enum(['MB', 'GB', 'TB']),
    maxObjectsEnabled: z.boolean(),
    maxObjectsValue: z.string(),
  })
  .superRefine((data, ctx) => {
    if (data.maxSizeEnabled) {
      const n = Number(data.maxSizeValue);
      if (!Number.isFinite(n) || !Number.isInteger(n) || n <= 0) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['maxSizeValue'],
          message: 'Enter a positive whole number',
        });
      }
    }
    if (data.maxObjectsEnabled) {
      const n = Number(data.maxObjectsValue);
      if (!Number.isFinite(n) || !Number.isInteger(n) || n <= 0) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ['maxObjectsValue'],
          message: 'Enter a positive whole number',
        });
      }
    }
  });

type QuotaFormValues = z.infer<typeof quotaFormSchema>;

function deriveDefaults(quotas: { maxSize?: number; maxObjects?: number } | null | undefined): QuotaFormValues {
  const size = quotas?.maxSize;
  const objects = quotas?.maxObjects;
  if (size != null) {
    const { value, unit } = bytesToQuotaValue(size);
    return {
      maxSizeEnabled: true,
      maxSizeValue: String(value),
      maxSizeUnit: unit,
      maxObjectsEnabled: objects != null,
      maxObjectsValue: objects != null ? String(objects) : '',
    };
  }
  return {
    maxSizeEnabled: false,
    maxSizeValue: '',
    maxSizeUnit: 'GB',
    maxObjectsEnabled: objects != null,
    maxObjectsValue: objects != null ? String(objects) : '',
  };
}

export function BucketSettings() {
  const { bucketName = '' } = useParams<{ bucketName: string }>();
  const navigate = useNavigate();
  const { data: buckets = [], isLoading } = useBuckets();
  const bucket = buckets.find((b) => b.name === bucketName);
  const deleteMutation = useDeleteBucket();
  const updateQuotasMutation = useUpdateBucketQuotas();

  const [deleteOpen, setDeleteOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);

  const defaults = useMemo(() => deriveDefaults(bucket?.quotas), [bucket?.quotas]);

  const {
    control,
    register,
    handleSubmit,
    watch,
    reset,
    formState: { errors, isDirty, isSubmitting },
  } = useForm<QuotaFormValues>({
    resolver: zodResolver(quotaFormSchema),
    values: defaults,
  });

  const watched = watch();

  const currentSize = bucket?.size ?? 0;
  const currentObjects = bucket?.objectCount ?? 0;

  const newMaxSizeBytes =
    watched.maxSizeEnabled && watched.maxSizeValue !== '' && !Number.isNaN(Number(watched.maxSizeValue))
      ? quotaValueToBytes(Number(watched.maxSizeValue), watched.maxSizeUnit)
      : null;
  const newMaxObjects =
    watched.maxObjectsEnabled && watched.maxObjectsValue !== '' && !Number.isNaN(Number(watched.maxObjectsValue))
      ? Number(watched.maxObjectsValue)
      : null;

  const sizeBelowCurrent =
    newMaxSizeBytes !== null && bucket?.size != null && newMaxSizeBytes < currentSize;
  const objectsBelowCurrent =
    newMaxObjects !== null && bucket?.objectCount != null && newMaxObjects < currentObjects;

  if (isLoading) {
    return <div className="px-7 py-6 text-[13.5px] text-[var(--muted-foreground)]">Loading…</div>;
  }
  if (!bucket) {
    return (
      <div className="px-7 py-6">
        <EmptyState
          icon={<AlertTriangle />}
          tone="neutral"
          title="Bucket not found"
          description="The bucket you're looking for doesn't exist or you don't have access."
        />
      </div>
    );
  }

  const confirmDelete = async () => {
    setDeleting(true);
    try {
      await deleteMutation.mutateAsync(bucket.name);
      navigate('/buckets');
    } catch {
      setDeleting(false);
    }
  };

  const onSubmit = handleSubmit(async (values) => {
    const maxSize = values.maxSizeEnabled
      ? quotaValueToBytes(Number(values.maxSizeValue), values.maxSizeUnit)
      : null;
    const maxObjects = values.maxObjectsEnabled ? Number(values.maxObjectsValue) : null;
    await updateQuotasMutation.mutateAsync({ bucketName: bucket.name, maxSize, maxObjects });
  });

  return (
    <div className="space-y-6 px-7 py-6">
      {/* Info */}
      <section className="rounded-xl border border-[var(--border)] bg-[var(--card)]">
        <header className="flex items-center gap-2 border-b border-[var(--border)] px-5 py-3">
          <Info className="h-4 w-4 text-[var(--primary)]" />
          <h2 className="text-[15px] font-semibold">Bucket info</h2>
        </header>
        <dl className="grid grid-cols-1 gap-x-6 gap-y-4 px-5 py-5 sm:grid-cols-2">
          <Field label="Name" value={<span className="font-mono text-[13.5px]">{bucket.name}</span>} />
          <Field label="Region" value={bucket.region ?? '—'} />
          <Field label="Created" value={formatDateOrDash(bucket.creationDate)} />
          <Field label="Objects" value={bucket.objectCount != null ? bucket.objectCount.toLocaleString() : '—'} />
          <Field label="Size" value={formatBytesOrDash(bucket.size)} />
          <Field
            label="Website"
            value={
              <Badge variant={bucket.websiteAccess ? 'success' : 'neutral'}>
                {bucket.websiteAccess ? 'Enabled' : 'Disabled'}
              </Badge>
            }
          />
        </dl>
      </section>

      {/* Quotas */}
      <section className="rounded-xl border border-[var(--border)] bg-[var(--card)]">
        <header className="flex items-center gap-2 border-b border-[var(--border)] px-5 py-3">
          <Gauge className="h-4 w-4 text-[var(--primary)]" />
          <h2 className="text-[15px] font-semibold">Quotas</h2>
        </header>

        <form onSubmit={onSubmit} className="space-y-6 px-5 py-5">
          {/* Max size row */}
          <div className="space-y-2">
            <div className="flex flex-wrap items-center gap-3">
              <Controller
                control={control}
                name="maxSizeEnabled"
                render={({ field }) => (
                  <label className="flex items-center gap-2 text-[14px]">
                    <Switch checked={field.value} onCheckedChange={field.onChange} />
                    <span>Limit total size</span>
                  </label>
                )}
              />
              <Input
                type="number"
                min={1}
                step={1}
                className="w-32"
                disabled={!watched.maxSizeEnabled}
                {...register('maxSizeValue')}
              />
              <Controller
                control={control}
                name="maxSizeUnit"
                render={({ field }) => (
                  <Select
                    value={field.value}
                    onChange={(v) => field.onChange(v as QuotaUnit)}
                    disabled={!watched.maxSizeEnabled}
                    className="w-24"
                  >
                    {(Object.keys(QUOTA_UNIT_BYTES) as QuotaUnit[]).map((u) => (
                      <SelectOption key={u} value={u}>
                        {u}
                      </SelectOption>
                    ))}
                  </Select>
                )}
              />
            </div>
            <p className="text-[13px] text-[var(--muted-foreground)]">
              Current: {formatBytesOrDash(bucket.size)}
            </p>
            {errors.maxSizeValue && (
              <p className="text-[13px] text-[var(--destructive)]">{errors.maxSizeValue.message}</p>
            )}
            {sizeBelowCurrent && (
              <p className="text-[13px] text-amber-600 dark:text-amber-400">
                Current size ({formatBytes(currentSize)}) exceeds this limit. New writes will be rejected.
              </p>
            )}
          </div>

          {/* Max objects row */}
          <div className="space-y-2">
            <div className="flex flex-wrap items-center gap-3">
              <Controller
                control={control}
                name="maxObjectsEnabled"
                render={({ field }) => (
                  <label className="flex items-center gap-2 text-[14px]">
                    <Switch checked={field.value} onCheckedChange={field.onChange} />
                    <span>Limit object count</span>
                  </label>
                )}
              />
              <Input
                type="number"
                min={1}
                step={1}
                className="w-40"
                disabled={!watched.maxObjectsEnabled}
                {...register('maxObjectsValue')}
              />
            </div>
            <p className="text-[13px] text-[var(--muted-foreground)]">
              Current: {bucket.objectCount != null ? bucket.objectCount.toLocaleString() : '—'}
            </p>
            {errors.maxObjectsValue && (
              <p className="text-[13px] text-[var(--destructive)]">{errors.maxObjectsValue.message}</p>
            )}
            {objectsBelowCurrent && (
              <p className="text-[13px] text-amber-600 dark:text-amber-400">
                Current object count ({currentObjects.toLocaleString()}) exceeds this limit. New writes will be rejected.
              </p>
            )}
          </div>

          <div className="flex items-center gap-3 border-t border-[var(--border)] pt-4">
            <Button type="submit" disabled={!isDirty || isSubmitting}>
              Save changes
            </Button>
            <Button
              type="button"
              variant="ghost"
              onClick={() => reset(defaults)}
              disabled={!isDirty || isSubmitting}
            >
              Reset
            </Button>
          </div>
        </form>
      </section>

      {/* Danger zone */}
      <section className="rounded-xl border border-[var(--danger-border)] bg-[var(--card)]">
        <header className="border-b border-[var(--danger-border)] px-5 py-3">
          <h2 className="text-[15px] font-semibold text-[var(--destructive)]">Danger zone</h2>
          <p className="mt-0.5 text-[13.5px] text-[var(--muted-foreground)]">
            Destructive actions for this bucket.
          </p>
        </header>
        <div className="flex items-center justify-between gap-4 px-5 py-4">
          <div className="min-w-0">
            <div className="text-[14px] font-medium">Delete bucket</div>
            <div className="text-[13.5px] text-[var(--muted-foreground)]">
              All objects in this bucket will be permanently removed.
            </div>
          </div>
          <Button variant="destructive" onClick={() => setDeleteOpen(true)}>
            Delete bucket
          </Button>
        </div>
      </section>

      <DangerousConfirmDialog
        open={deleteOpen}
        onOpenChange={(o) => {
          if (!o && !deleting) setDeleteOpen(false);
        }}
        title={`Delete bucket "${bucket.name}"?`}
        description="This action cannot be undone."
        confirmationText={bucket.name}
        confirmLabel="Delete bucket"
        loading={deleting}
        onConfirm={confirmDelete}
      />
    </div>
  );
}

function Field({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div>
      <dt className="text-[11px] font-medium uppercase tracking-[0.08em] text-[var(--muted-foreground)]">{label}</dt>
      <dd className="mt-1 text-[14px]">{value}</dd>
    </div>
  );
}
