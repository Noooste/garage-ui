import { describe, expect, it } from 'vitest';
import { act, renderHook } from '@testing-library/react';
import { useSort } from './useSort';

describe('useSort', () => {
  it('starts with the initial sort', () => {
    const { result } = renderHook(() => useSort({ column: 'name', direction: 'asc' }));
    expect(result.current.sort).toEqual({ column: 'name', direction: 'asc' });
  });

  it('flips the direction when the active column is toggled', () => {
    const { result } = renderHook(() => useSort({ column: 'name', direction: 'asc' }));
    act(() => result.current.toggleSort('name'));
    expect(result.current.sort).toEqual({ column: 'name', direction: 'desc' });
    act(() => result.current.toggleSort('name'));
    expect(result.current.sort).toEqual({ column: 'name', direction: 'asc' });
  });

  it("switches to a new column in that column's first direction", () => {
    const { result } = renderHook(() =>
      useSort<'name' | 'size'>({ column: 'name', direction: 'desc' }, { size: 'desc' }),
    );
    act(() => result.current.toggleSort('size'));
    expect(result.current.sort).toEqual({ column: 'size', direction: 'desc' });
    act(() => result.current.toggleSort('name'));
    expect(result.current.sort).toEqual({ column: 'name', direction: 'asc' });
  });
});
