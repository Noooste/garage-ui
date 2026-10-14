import { describe, expect, it } from 'vitest';
import { formatShare, toUsageSlices } from './bucket-usage';

const bucket = (bucketName: string, size: number) => ({ bucketName, size, objectCount: 0, percentage: 0 });

describe('toUsageSlices', () => {
  it('sorts by size and drops empty buckets', () => {
    const slices = toUsageSlices([bucket('a', 10), bucket('empty', 0), bucket('b', 30)], 8);
    expect(slices.map((s) => s.name)).toEqual(['b', 'a']);
    expect(slices[0].share).toBe(0.75);
  });

  it('merges buckets under 2% into Other', () => {
    const slices = toUsageSlices([bucket('big', 970), bucket('tiny1', 10), bucket('tiny2', 10), bucket('tiny3', 10)], 8);
    expect(slices).toEqual([
      { name: 'big', size: 970, share: 0.97, isOther: false, bucketCount: 1 },
      { name: 'Other', size: 30, share: 0.03, isOther: true, bucketCount: 3 },
    ]);
  });

  it('keeps a lone small bucket named instead of an Other of one', () => {
    const slices = toUsageSlices([bucket('big', 990), bucket('tiny', 10)], 8);
    expect(slices.map((s) => s.name)).toEqual(['big', 'tiny']);
  });

  it('caps named slices so colors never repeat', () => {
    const data = Array.from({ length: 12 }, (_, i) => bucket(`b${i}`, 100));
    const slices = toUsageSlices(data, 8);
    expect(slices).toHaveLength(9);
    expect(slices[8]).toMatchObject({ isOther: true, bucketCount: 4, size: 400 });
  });

  it('returns nothing when every bucket is empty', () => {
    expect(toUsageSlices([bucket('a', 0)], 8)).toEqual([]);
  });
});

describe('formatShare', () => {
  it('formats percentages and floors tiny shares', () => {
    expect(formatShare(0.4567)).toBe('45.7%');
    expect(formatShare(0.0002)).toBe('<0.1%');
  });
});
