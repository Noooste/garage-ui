import type { BucketUsage } from '@/types';

/** Buckets below this share of total storage are merged into "Other". */
export const MIN_SLICE_SHARE = 0.02;

export type UsageSlice = {
  name: string;
  size: number;
  /** Fraction of total storage, 0 to 1. */
  share: number;
  isOther: boolean;
  /** Number of buckets in the slice: 1, or more for "Other". */
  bucketCount: number;
};

/**
 * Turns per-bucket usage into chart slices, largest first. Empty buckets are
 * dropped, and buckets too small to read (or past maxSlices) become one "Other" slice.
 */
export function toUsageSlices(data: BucketUsage[], maxSlices: number): UsageSlice[] {
  const sorted = data.filter((b) => b.size > 0).sort((a, b) => b.size - a.size);
  const total = sorted.reduce((sum, b) => sum + b.size, 0);

  // sorted is descending, so the buckets above the threshold form a prefix
  let namedCount = Math.min(sorted.filter((b) => b.size / total >= MIN_SLICE_SHARE).length, maxSlices);
  // An "Other" holding a single bucket would hide its name for nothing
  if (sorted.length - namedCount === 1 && namedCount < maxSlices) namedCount++;

  const slices: UsageSlice[] = sorted.slice(0, namedCount).map((b) => ({
    name: b.bucketName,
    size: b.size,
    share: b.size / total,
    isOther: false,
    bucketCount: 1,
  }));

  const rest = sorted.slice(namedCount);
  if (rest.length > 0) {
    const size = rest.reduce((sum, b) => sum + b.size, 0);
    slices.push({ name: 'Other', size, share: size / total, isOther: true, bucketCount: rest.length });
  }
  return slices;
}

export function formatShare(share: number): string {
  return share > 0 && share < 0.001 ? '<0.1%' : `${(share * 100).toFixed(1)}%`;
}
