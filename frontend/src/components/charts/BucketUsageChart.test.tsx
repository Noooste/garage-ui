import { beforeAll, describe, expect, it } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import type { BucketUsage } from '@/types';
import { getOtherColor, getSeriesColors } from '@/lib/chart-colors';
import { BucketUsageChart } from './BucketUsageChart';

// recharts' ResponsiveContainer needs ResizeObserver, which jsdom lacks.
beforeAll(() => {
  globalThis.ResizeObserver ??= class {
    observe() {}
    unobserve() {}
    disconnect() {}
  };
});

const usage = (bucketName: string, size: number): BucketUsage => ({ bucketName, size, objectCount: 1, percentage: 0 });

function legendItems() {
  return within(screen.getByRole('list')).getAllByRole('listitem');
}

describe('BucketUsageChart legend', () => {
  it('lists slices largest first with their share, without empty buckets', () => {
    render(<BucketUsageChart data={[usage('small', 250), usage('empty', 0), usage('big', 750)]} />);
    expect(legendItems().map((item) => item.textContent)).toEqual(['big75.0%', 'small25.0%']);
  });

  it('merges tiny buckets into a gray "Other" entry', () => {
    render(
      <BucketUsageChart data={[usage('a', 9_800), usage('tiny-1', 100), usage('tiny-2', 100)]} />,
    );
    const items = legendItems();
    expect(items.map((item) => item.textContent)).toEqual(['a98.0%', 'Other2.0%']);
    const swatch = (item: HTMLElement) => (item.firstElementChild as HTMLElement).style.backgroundColor;
    const toRgb = (hex: string) => {
      const n = parseInt(hex.slice(1), 16);
      return `rgb(${n >> 16}, ${(n >> 8) & 255}, ${n & 255})`;
    };
    expect(swatch(items[0])).toBe(toRgb(getSeriesColors(false)[0]));
    expect(swatch(items[1])).toBe(toRgb(getOtherColor(false)));
  });

  it('shows shares below 0.1% as "<0.1%"', () => {
    render(<BucketUsageChart data={[usage('huge', 1_000_000), usage('speck', 1)]} />);
    expect(legendItems().map((item) => item.textContent)).toEqual(['huge100.0%', 'speck<0.1%']);
  });

  it('keeps every entry when bucket names repeat', () => {
    render(<BucketUsageChart data={[usage('backup', 600), usage('backup', 400)]} />);
    expect(legendItems()).toHaveLength(2);
  });

  it('shows an empty state when nothing is stored', () => {
    render(<BucketUsageChart data={[usage('empty', 0)]} />);
    expect(screen.getByText('No data available')).toBeInTheDocument();
    expect(screen.queryByRole('list')).not.toBeInTheDocument();
  });
});
