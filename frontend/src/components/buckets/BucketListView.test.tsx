import { describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, within } from '@testing-library/react';
import type { Bucket } from '@/types';

vi.mock('@/hooks/usePermissions', () => ({
  useBucketCan: () => () => true,
}));

const { BucketListView } = await import('./BucketListView');

const buckets: Bucket[] = [
  { name: 'media', creationDate: '2025-03-01T00:00:00Z', size: 500, objectCount: 3, websiteAccess: false },
  { name: 'Archive', creationDate: '2024-01-01T00:00:00Z', size: 9000, objectCount: 1, websiteAccess: false },
  { name: 'logs', creationDate: '2026-01-01T00:00:00Z', size: 20, objectCount: 40, websiteAccess: false },
];

function renderList(searchQuery = '') {
  return render(
    <BucketListView
      buckets={buckets}
      searchQuery={searchQuery}
      onSearchChange={() => {}}
      onViewBucket={() => {}}
      onOpenSettings={() => {}}
      onDeleteBucket={() => {}}
      onWebsiteSettings={() => {}}
    />,
  );
}

function rowNames() {
  return screen
    .getAllByRole('row')
    .slice(1)
    .map((row) => within(row).getAllByRole('cell')[0].textContent);
}

function header(name: string) {
  return screen.getByRole('columnheader', { name: new RegExp(name) });
}

describe('BucketListView sorting', () => {
  it('sorts by bucket name A to Z by default', () => {
    renderList();
    expect(rowNames()).toEqual(['Archive', 'logs', 'media']);
    expect(header('Name')).toHaveAttribute('aria-sort', 'ascending');
  });

  it('sorts by size, largest first, and flips on a second click', () => {
    renderList();
    fireEvent.click(within(header('Size')).getByRole('button'));
    expect(rowNames()).toEqual(['Archive', 'media', 'logs']);
    expect(header('Size')).toHaveAttribute('aria-sort', 'descending');
    expect(header('Name')).toHaveAttribute('aria-sort', 'none');

    fireEvent.click(within(header('Size')).getByRole('button'));
    expect(rowNames()).toEqual(['logs', 'media', 'Archive']);
  });

  it('sorts by creation date, newest first', () => {
    renderList();
    fireEvent.click(within(header('Created')).getByRole('button'));
    expect(rowNames()).toEqual(['logs', 'media', 'Archive']);
  });

  it('keeps the sort order while filtering', () => {
    renderList('a');
    fireEvent.click(within(header('Name')).getByRole('button'));
    expect(rowNames()).toEqual(['media', 'Archive']);
  });
});
