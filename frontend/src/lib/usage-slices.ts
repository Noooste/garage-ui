import type { BucketUsage } from '@/types';

// A type alias rather than an interface: recharts takes chart data as
// index-signature records, which interfaces are not assignable to.
export type UsageSlice = {
  name: string;
  size: number;
  /** Fraction of the total stored bytes, 0..1. */
  share: number;
  /** Number of buckets in this slice: 1, or more for the "Other" slice. */
  bucketCount: number;
  isOther: boolean;
};

export interface UsageSliceOptions {
  /** Most named slices to show; the rest go into "Other". */
  maxSlices: number;
  /** Buckets below this share of the total go into "Other". */
  minShare: number;
}

export const OTHER_SLICE_NAME = 'Other';

/**
 * Turns per-bucket usage into pie slices that stay readable with many
 * buckets: empty buckets are dropped, the largest buckets get their own
 * slice, and buckets too small to see (or beyond `maxSlices`) are merged into
 * one "Other" slice. A single leftover bucket keeps its own slice when there
 * is room, since an "Other" of one bucket would only hide its name.
 */
export function buildUsageSlices(
  usage: readonly BucketUsage[],
  { maxSlices, minShare }: UsageSliceOptions,
): UsageSlice[] {
  const stored = usage.filter((bucket) => bucket.size > 0);
  const total = stored.reduce((sum, bucket) => sum + bucket.size, 0);
  if (total === 0) return [];

  const toSlice = (bucket: BucketUsage): UsageSlice => ({
    name: bucket.bucketName,
    size: bucket.size,
    share: bucket.size / total,
    bucketCount: 1,
    isOther: false,
  });

  const bySize = [...stored].sort((a, b) => b.size - a.size);
  const named = bySize.filter((bucket) => bucket.size / total >= minShare).slice(0, maxSlices);
  const rest = bySize.slice(named.length);

  if (rest.length === 1 && named.length < maxSlices) return [...named, rest[0]].map(toSlice);

  const slices = named.map(toSlice);
  if (rest.length > 0) {
    const size = rest.reduce((sum, bucket) => sum + bucket.size, 0);
    slices.push({ name: OTHER_SLICE_NAME, size, share: size / total, bucketCount: rest.length, isOther: true });
  }
  return slices;
}
