import { describe, expect, it } from 'vitest';
import { highlight } from './highlight';

describe('highlight', () => {
  it('highlights a known language', () => {
    const html = highlight('{"a": 1}', 'json');
    expect(html).toContain('hljs-');
  });

  it('escapes html in the source', () => {
    const html = highlight('<script>alert(1)</script>', 'xml');
    expect(html).not.toContain('<script>');
  });

  it('falls back to auto detection for null language', () => {
    const html = highlight('SELECT * FROM t;', null);
    expect(typeof html).toBe('string');
    expect(html.length).toBeGreaterThan(0);
  });

  it('falls back to auto detection for an unregistered language', () => {
    const html = highlight('plain words', 'klingon');
    expect(typeof html).toBe('string');
  });
});
