import {useEffect, useMemo, useState} from 'react';
import {Cell, Pie, PieChart} from 'recharts';
import type {BucketUsage} from '@/types';
import {formatBytes} from '@/lib/file-utils';
import {formatShare, toUsageSlices, type UsageSlice} from '@/lib/bucket-usage';
import {categoricalPalette, otherColor} from '@/lib/chart-colors';
import {cn} from '@/lib/utils';

interface BucketUsageChartProps {
  data: BucketUsage[];
}

const SIZE = 200;
const bucketLabel = (n: number) => `${n} ${n === 1 ? 'bucket' : 'buckets'}`;

export function BucketUsageChart({ data }: BucketUsageChartProps) {
  const [isDark, setIsDark] = useState(false);
  const [active, setActive] = useState<number | null>(null);

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

  const palette = isDark ? categoricalPalette.dark : categoricalPalette.light;
  const slices = useMemo(() => toUsageSlices(data, palette.length), [data, palette.length]);

  if (slices.length === 0) {
    return <div className="py-8 text-center text-[13.5px] text-[var(--muted-foreground)]">All buckets are empty</div>;
  }

  const colorOf = (slice: UsageSlice, index: number) =>
    slice.isOther ? (isDark ? otherColor.dark : otherColor.light) : palette[index];
  const total = slices.reduce((sum, s) => sum + s.size, 0);
  const focus = active === null ? null : slices[active];
  const dimmed = (index: number) => active !== null && active !== index;

  return (
    <div className="@container" onMouseLeave={() => setActive(null)}>
      <div className="flex flex-col items-center gap-6 @sm:flex-row">
        {/* The legend carries the same data as text, so the drawing is hidden from screen readers */}
        <div className="relative shrink-0" style={{ width: SIZE, height: SIZE }} aria-hidden="true">
          <PieChart width={SIZE} height={SIZE} accessibilityLayer={false}>
            <Pie
              data={slices}
              dataKey="size"
              nameKey="name"
              innerRadius={68}
              outerRadius={96}
              paddingAngle={slices.length > 1 ? 1.5 : 0}
              cornerRadius={3}
              stroke="none"
              startAngle={90}
              endAngle={-270}
              rootTabIndex={-1}
              onMouseEnter={(_, index) => setActive(index)}
              isAnimationActive={false}
            >
              {slices.map((slice, index) => (
                <Cell
                  key={slice.isOther ? ' other' : slice.name}
                  fill={colorOf(slice, index)}
                  fillOpacity={dimmed(index) ? 0.25 : 1}
                  style={{ transition: 'fill-opacity 150ms ease-out', outline: 'none' }}
                />
              ))}
            </Pie>
          </PieChart>
          <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
            <span className="text-[17px] font-semibold tabular-nums">{formatBytes(focus ? focus.size : total)}</span>
            <span className="mt-0.5 max-w-[112px] truncate text-[12px] text-[var(--muted-foreground)]">
              {focus
                ? focus.isOther ? `Other, ${bucketLabel(focus.bucketCount)}` : focus.name
                : 'total stored'}
            </span>
          </div>
        </div>

        <ul className="w-full min-w-0 flex-1 space-y-0.5" aria-label="Storage by bucket">
          {slices.map((slice, index) => (
            <li
              key={slice.isOther ? ' other' : slice.name}
              onMouseEnter={() => setActive(index)}
              className={cn(
                'flex items-center gap-2.5 rounded-md px-2 py-1 text-[13px] transition-opacity duration-150',
                dimmed(index) && 'opacity-45',
              )}
            >
              <span className="size-2.5 shrink-0 rounded-[3px]" style={{ background: colorOf(slice, index) }} />
              <span className="min-w-0 flex-1 truncate">
                {slice.isOther ? (
                  <>
                    Other <span className="text-[var(--muted-foreground)]">({bucketLabel(slice.bucketCount)})</span>
                  </>
                ) : (
                  slice.name
                )}
              </span>
              <span className="tabular-nums text-[var(--muted-foreground)]">{formatShare(slice.share)}</span>
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}
