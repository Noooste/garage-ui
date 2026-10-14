export type SortDirection = 'asc' | 'desc';

export interface SortState<K extends string> {
  column: K;
  direction: SortDirection;
}

export type SortValue = string | number | Date | null | undefined;

/**
 * How strings compare:
 * - 'natural': case-insensitive, with digit runs compared by value, so
 *   "bucket-2" sorts before "bucket-10". For lists loaded in full.
 * - 'binary': by Unicode code point, which matches the UTF-8 byte order S3
 *   lists keys in. Use it where rows arrive in server pages, so a page sorted
 *   by name still lines up with the pages before and after it.
 */
export type StringOrder = 'natural' | 'binary';

export interface SortOptions {
  stringOrder?: StringOrder;
}

const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: 'base' });

function compareCodePoints(a: string, b: string): number {
  let i = 0;
  while (i < a.length && i < b.length) {
    const ca = a.codePointAt(i)!;
    const cb = b.codePointAt(i)!;
    if (ca !== cb) return ca - cb;
    i += ca > 0xffff ? 2 : 1;
  }
  return a.length - b.length;
}

/**
 * Compares two present cell values in ascending order. Dates and numbers
 * compare numerically; strings follow `stringOrder`.
 */
function compareAscending(a: SortValue, b: SortValue, stringOrder: StringOrder): number {
  if (typeof a === 'string' && typeof b === 'string') {
    return stringOrder === 'binary' ? compareCodePoints(a, b) : collator.compare(a, b);
  }
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
 * value through `accessors`. Missing values (null, undefined, NaN, invalid
 * dates) always sort last, regardless of direction, so empty cells never
 * crowd the top of the table. The sort is stable, so rows with equal values
 * keep their incoming order.
 */
export function sortItems<T, K extends string>(
  items: readonly T[],
  sort: SortState<K>,
  accessors: Record<K, (item: T) => SortValue>,
  { stringOrder = 'natural' }: SortOptions = {},
): T[] {
  const accessor = accessors[sort.column];
  const factor = sort.direction === 'asc' ? 1 : -1;
  // Read each value once up front rather than on every comparison; accessors
  // may parse dates or strip path prefixes.
  return items
    .map((item) => {
      const value = accessor(item);
      return { item, value, missing: isMissing(value) };
    })
    .sort((x, y) => {
      if (x.missing || y.missing) return x.missing === y.missing ? 0 : x.missing ? 1 : -1;
      return compareAscending(x.value, y.value, stringOrder) * factor;
    })
    .map(({ item }) => item);
}
