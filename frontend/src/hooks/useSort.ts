import { useState } from 'react';
import type { SortDirection, SortState } from '@/lib/sort-utils';

/**
 * Column sort state for a table. Clicking the active column flips its
 * direction; clicking another column activates it in that column's preferred
 * first direction (e.g. size or date columns usually start descending).
 */
export function useSort<K extends string>(
  initial: SortState<K>,
  firstDirection: Partial<Record<K, SortDirection>> = {},
) {
  const [sort, setSort] = useState<SortState<K>>(initial);

  const toggleSort = (column: K) => {
    setSort((current) =>
      current.column === column
        ? { column, direction: current.direction === 'asc' ? 'desc' : 'asc' }
        : { column, direction: firstDirection[column] ?? 'asc' },
    );
  };

  return { sort, toggleSort };
}
