import { describe, expect, it } from 'vitest';
import type { BucketUsage } from '@/types';
import { buildUsageSlices, OTHER_SLICE_NAME } from './usage-slices';
import { getChartColors, getOtherColor, getSeriesColors } from './chart-colors';

const usage = (bucketName: string, size: number): BucketUsage => ({ bucketName, size, objectCount: 1, percentage: 0 });

const options = { maxSlices: 9, minShare: 0.02 };

describe('buildUsageSlices', () => {
  it('returns no slices when nothing is stored', () => {
    expect(buildUsageSlices([], options)).toEqual([]);
    expect(buildUsageSlices([usage('empty', 0)], options)).toEqual([]);
  });

  it('drops empty buckets and orders slices by size', () => {
    const slices = buildUsageSlices([usage('small', 30), usage('empty', 0), usage('big', 70)], options);
    expect(slices).toEqual([
      { name: 'big', size: 70, share: 0.7, bucketCount: 1, isOther: false },
      { name: 'small', size: 30, share: 0.3, bucketCount: 1, isOther: false },
    ]);
  });

  it('merges buckets below the minimum share into "Other"', () => {
    const slices = buildUsageSlices(
      [usage('a', 900), usage('tiny-1', 10), usage('b', 70), usage('tiny-2', 15), usage('tiny-3', 5)],
      options,
    );
    expect(slices.map((s) => s.name)).toEqual(['a', 'b', OTHER_SLICE_NAME]);
    expect(slices[2]).toEqual({ name: OTHER_SLICE_NAME, size: 30, share: 0.03, bucketCount: 3, isOther: true });
  });

  it('keeps a single small leftover bucket as its own slice', () => {
    const slices = buildUsageSlices([usage('a', 990), usage('tiny', 10)], options);
    expect(slices.map((s) => s.name)).toEqual(['a', 'tiny']);
    expect(slices.some((s) => s.isOther)).toBe(false);
  });

  it('caps named slices at maxSlices and puts the rest into "Other"', () => {
    const buckets = Array.from({ length: 12 }, (_, i) => usage(`b${i}`, 100 - i));
    const slices = buildUsageSlices(buckets, { maxSlices: 4, minShare: 0 });
    expect(slices.map((s) => s.name)).toEqual(['b0', 'b1', 'b2', 'b3', OTHER_SLICE_NAME]);
    expect(slices[4].bucketCount).toBe(8);
  });

  it('uses "Other" for a single leftover when no named slot is free', () => {
    const slices = buildUsageSlices([usage('a', 50), usage('b', 30), usage('c', 20)], { maxSlices: 2, minShare: 0 });
    expect(slices.map((s) => s.name)).toEqual(['a', 'b', OTHER_SLICE_NAME]);
    expect(slices[2].bucketCount).toBe(1);
  });
});

describe('chart series colors', () => {
  it.each([false, true])('reserve the "Other" color and never repeat (dark: %s)', (isDark) => {
    const series = getSeriesColors(isDark);
    expect(series).not.toContain(getOtherColor(isDark));
    expect(new Set(series).size).toBe(series.length);
    expect(series.length).toBe(getChartColors(isDark).length - 1);
  });
});
