import { describe, expect, it } from 'vitest';
import { sortItems, type SortValue } from './sort-utils';

interface Row {
  name: string;
  size?: number;
  created?: Date;
}

const accessors: Record<'name' | 'size' | 'created', (row: Row) => SortValue> = {
  name: (row) => row.name,
  size: (row) => row.size,
  created: (row) => row.created,
};

const names = (rows: Row[]) => rows.map((row) => row.name);

describe('sortItems', () => {
  it('sorts strings naturally and case-insensitively', () => {
    const rows = [{ name: 'bucket-10' }, { name: 'Bucket-2' }, { name: 'alpha' }, { name: 'bucket-1' }];
    expect(names(sortItems(rows, { column: 'name', direction: 'asc' }, accessors))).toEqual([
      'alpha',
      'bucket-1',
      'Bucket-2',
      'bucket-10',
    ]);
    expect(names(sortItems(rows, { column: 'name', direction: 'desc' }, accessors))).toEqual([
      'bucket-10',
      'Bucket-2',
      'bucket-1',
      'alpha',
    ]);
  });

  it('sorts strings by code point in binary order, matching S3 key order', () => {
    // S3 lists keys by UTF-8 bytes: "logs-10" before "logs-2", uppercase first.
    const rows = [{ name: 'logs-2' }, { name: 'b' }, { name: 'logs-10' }, { name: 'Z' }, { name: 'logs-1' }];
    expect(names(sortItems(rows, { column: 'name', direction: 'asc' }, accessors, { stringOrder: 'binary' }))).toEqual([
      'Z',
      'b',
      'logs-1',
      'logs-10',
      'logs-2',
    ]);
    expect(names(sortItems(rows, { column: 'name', direction: 'desc' }, accessors, { stringOrder: 'binary' }))).toEqual([
      'logs-2',
      'logs-10',
      'logs-1',
      'b',
      'Z',
    ]);
  });

  it('orders characters beyond the BMP after high BMP characters in binary order', () => {
    // UTF-16 code units would put the emoji (surrogate 0xD83D) first; UTF-8
    // bytes, and so S3, put U+FF61 first.
    const rows = [{ name: 'a\u{1F600}y' }, { name: 'a\uFF61' }, { name: 'a' }, { name: 'a\u{1F600}x' }];
    expect(names(sortItems(rows, { column: 'name', direction: 'asc' }, accessors, { stringOrder: 'binary' }))).toEqual([
      'a',
      'a\uFF61',
      'a\u{1F600}x',
      'a\u{1F600}y',
    ]);
  });

  it('sorts numbers in both directions', () => {
    const rows = [{ name: 'a', size: 5 }, { name: 'b', size: 50 }, { name: 'c', size: 0 }];
    expect(names(sortItems(rows, { column: 'size', direction: 'asc' }, accessors))).toEqual(['c', 'a', 'b']);
    expect(names(sortItems(rows, { column: 'size', direction: 'desc' }, accessors))).toEqual(['b', 'a', 'c']);
  });

  it('sorts dates chronologically', () => {
    const rows = [
      { name: 'mid', created: new Date('2025-06-01') },
      { name: 'old', created: new Date('2024-01-01') },
      { name: 'new', created: new Date('2026-01-01') },
    ];
    expect(names(sortItems(rows, { column: 'created', direction: 'desc' }, accessors))).toEqual([
      'new',
      'mid',
      'old',
    ]);
  });

  it('keeps missing values last in either direction', () => {
    const rows = [
      { name: 'none' },
      { name: 'small', size: 1 },
      { name: 'nan', size: Number.NaN },
      { name: 'big', size: 9 },
      { name: 'bad-date', created: new Date('not a date') },
    ];
    expect(names(sortItems(rows, { column: 'size', direction: 'asc' }, accessors))).toEqual([
      'small',
      'big',
      'none',
      'nan',
      'bad-date',
    ]);
    expect(names(sortItems(rows, { column: 'size', direction: 'desc' }, accessors))).toEqual([
      'big',
      'small',
      'none',
      'nan',
      'bad-date',
    ]);
  });

  it('is stable and does not mutate the input', () => {
    const rows = [{ name: 'x', size: 1 }, { name: 'y', size: 1 }, { name: 'z', size: 0 }];
    const snapshot = [...rows];
    expect(names(sortItems(rows, { column: 'size', direction: 'desc' }, accessors))).toEqual(['x', 'y', 'z']);
    expect(rows).toEqual(snapshot);
  });
});
