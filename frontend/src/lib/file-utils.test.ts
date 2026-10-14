import { describe, expect, it } from 'vitest';
import { formatCount } from './file-utils';

describe('formatCount', () => {
  it('keeps small counts exact and compacts large ones', () => {
    expect(formatCount(4)).toBe((4).toLocaleString());
    expect(formatCount(9_999)).toBe((9_999).toLocaleString());
    expect(formatCount(3_000_050)).toMatch(/^3\s?M$/);
    expect(formatCount(66_546_546_546_546)).toMatch(/^66\.5\s?T$/);
  });
});
