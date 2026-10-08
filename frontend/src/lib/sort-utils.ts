export type SortDirection = 'asc' | 'desc';

export interface SortState<K extends string> {
  column: K;
  direction: SortDirection;
}

export type SortValue = string | number | Date | null | undefined;

const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: 'base' });

/**
 * Compares two cell values in ascending order. Strings use a natural,
 * case-insensitive collation (so "bucket-2" sorts before "bucket-10"); dates
 * and numbers compare numerically. Missing values always sort last,
 * regardless of direction, so empty cells never crowd the top of the table.
 */
function compareAscending(a: SortValue, b: SortValue): number {
  if (typeof a === 'string' && typeof b === 'string') return collator.compare(a, b);
  const an = a instanceof Date ? a.getTime() : (a as number);
  const bn = b instanceof Date ? b.getTime() : (b as number);
  return an - bn;
}

function isMissing(value: SortValue): boolean {
  return value === null || value === undefined || (typeof value === 'number' && Number.isNaN(value))
    || (value instanceof Date && Number.isNaN(value.getTime()));
}

/**
 * Returns a sorted copy of `items` according to `sort`, reading each column's
 * value through `accessors`. The sort is stable, so rows with equal values
 * keep their incoming order.
 */
export function sortItems<T, K extends string>(
  items: readonly T[],
  sort: SortState<K>,
  accessors: Record<K, (item: T) => SortValue>,
): T[] {
  const accessor = accessors[sort.column];
  const factor = sort.direction === 'asc' ? 1 : -1;
  return [...items].sort((x, y) => {
    const a = accessor(x);
    const b = accessor(y);
    const aMissing = isMissing(a);
    const bMissing = isMissing(b);
    if (aMissing || bMissing) return aMissing === bMissing ? 0 : aMissing ? 1 : -1;
    return compareAscending(a, b) * factor;
  });
}
