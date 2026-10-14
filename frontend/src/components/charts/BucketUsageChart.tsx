import {useEffect, useMemo, useState} from 'react';
import {Cell, Pie, PieChart, ResponsiveContainer, Tooltip} from 'recharts';
import type {BucketUsage} from '@/types';
import {formatBytes} from '@/lib/file-utils';
import {getOtherColor, getSeriesColors, getTextColor, getTooltipStyle} from '@/lib/chart-colors';
import {buildUsageSlices, type UsageSlice} from '@/lib/usage-slices';

interface BucketUsageChartProps {
  data: BucketUsage[];
}

// Buckets below 2% of the total would be slivers too thin to see or label.
const MIN_SLICE_SHARE = 0.02;

// One decimal, like the Breakdown list next to the chart. Stored data never
// reads as 0.0%, since empty buckets are left out of the chart altogether.
function formatShare(share: number): string {
  return share < 0.001 ? '<0.1%' : `${(share * 100).toFixed(1)}%`;
}

export function BucketUsageChart({ data }: BucketUsageChartProps) {
  const [isDark, setIsDark] = useState(false);

  useEffect(() => {
    // Check if dark mode is enabled
    const checkDarkMode = () => {
      setIsDark(document.documentElement.classList.contains('dark'));
    };

    checkDarkMode();

    // Listen for theme changes
    const observer = new MutationObserver(checkDarkMode);
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] });

    return () => observer.disconnect();
  }, []);

  const colors = getSeriesColors(isDark);
  const otherColor = getOtherColor(isDark);
  const textColor = getTextColor(isDark);
  const tooltipStyle = getTooltipStyle(isDark);

  // One color per named slice, so the cap on named slices is the palette size.
  const slices = useMemo(
    () => buildUsageSlices(data, { maxSlices: colors.length, minShare: MIN_SLICE_SHARE }),
    [data, colors.length],
  );
  const colorOf = (index: number) => (slices[index].isOther ? otherColor : colors[index % colors.length]);

  if (slices.length === 0) {
    return (
      <div className="py-8 text-center text-[13.5px] text-[var(--muted-foreground)]">No data available</div>
    );
  }

  return (
    <div className="space-y-4">
      <ResponsiveContainer width="100%" height={240}>
        <PieChart>
          <Pie
            data={slices}
            cx="50%"
            cy="50%"
            outerRadius={100}
            dataKey="size"
            nameKey="name"
          >
            {/* Keyed by position: bucket names come from local aliases,
                which are per key and can repeat across buckets. */}
            {slices.map((slice, index) => (
              <Cell key={`${index}-${slice.name}`} fill={colorOf(index)} />
            ))}
          </Pie>
          <Tooltip
            formatter={(value, name, item) => {
              const slice = item.payload as UsageSlice | undefined;
              const detail = slice?.isOther
                ? ` across ${slice.bucketCount} bucket${slice.bucketCount === 1 ? '' : 's'}`
                : '';
              return [`${formatBytes(value as number)}${detail}`, name];
            }}
            contentStyle={tooltipStyle as React.CSSProperties}
            itemStyle={{ color: textColor }}
          />
        </PieChart>
      </ResponsiveContainer>
      {/* An HTML legend rather than recharts' <Legend>: it wraps freely below
          the pie instead of eating into the fixed chart height, and it keeps
          the slice order (largest first). */}
      <ul className="flex flex-wrap justify-center gap-x-4 gap-y-1.5 text-[12.5px]">
        {slices.map((slice, index) => (
          <li key={`${index}-${slice.name}`} className="flex min-w-0 items-center gap-1.5">
            <span className="h-2.5 w-2.5 shrink-0 rounded-sm" style={{ backgroundColor: colorOf(index) }} />
            <span className="truncate" title={slice.name}>{slice.name}</span>
            <span className="text-[var(--muted-foreground)]">{formatShare(slice.share)}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}
