import { describe, expect, it, afterEach } from 'vitest';
import {
  BASE_PATH_META_NAME,
  normalizeBasePath,
  resolveBasePath,
  routerBasename,
  stripBasePath,
  withBasePath,
} from './base-path';

/**
 * Issue #107: the SPA has to work when it is mounted under a subpath by a
 * path-routing reverse proxy. Everything here pins the contract the rest of
 * the frontend builds URLs from.
 */

function makeDocument(html: string): Document {
  return new DOMParser().parseFromString(html, 'text/html');
}

describe('normalizeBasePath', () => {
  it.each([
    ['', ''],
    ['/', ''],
    ['///', ''],
    ['   ', ''],
    ['garage-ui', '/garage-ui'],
    ['/garage-ui', '/garage-ui'],
    ['/garage-ui/', '/garage-ui'],
    ['/garage-ui//', '/garage-ui'],
    ['  /garage-ui/  ', '/garage-ui'],
    ['/admin/garage-ui/', '/admin/garage-ui'],
    ['//admin///garage-ui//', '/admin/garage-ui'],
    ['/garage_ui.v2-final~1', '/garage_ui.v2-final~1'],
  ])('normalizes %o to %o', (raw, expected) => {
    expect(normalizeBasePath(raw)).toBe(expected);
  });

  it('treats null and undefined as the root', () => {
    expect(normalizeBasePath(null)).toBe('');
    expect(normalizeBasePath(undefined)).toBe('');
  });

  it('keeps only the path of an absolute <base href>', () => {
    expect(normalizeBasePath('https://host.ts.net/garage-ui/')).toBe('/garage-ui');
    expect(normalizeBasePath('http://localhost:3000/')).toBe('');
    expect(normalizeBasePath('https://host.ts.net')).toBe('');
  });

  it('drops relative and traversal segments', () => {
    expect(normalizeBasePath('./')).toBe('');
    expect(normalizeBasePath('./garage-ui/')).toBe('/garage-ui');
    expect(normalizeBasePath('/garage-ui/../admin')).toBe('/admin');
  });

  it('is idempotent', () => {
    for (const raw of ['', '/', 'garage-ui/', '//a//b//', 'https://host/x/']) {
      const once = normalizeBasePath(raw);
      expect(normalizeBasePath(once)).toBe(once);
    }
  });
});

describe('resolveBasePath', () => {
  it('prefers the meta tag the backend injects', () => {
    const doc = makeDocument(
      `<html><head><base href="/wrong/"><meta name="${BASE_PATH_META_NAME}" content="/garage-ui"></head><body></body></html>`,
    );
    expect(resolveBasePath(doc)).toBe('/garage-ui');
  });

  it('reads an empty meta tag as the root, without falling through', () => {
    const doc = makeDocument(
      `<html><head><base href="/should-not-win/"><meta name="${BASE_PATH_META_NAME}" content=""></head><body></body></html>`,
    );
    expect(resolveBasePath(doc)).toBe('');
  });

  it('falls back to <base href> when no meta tag is present', () => {
    const doc = makeDocument('<html><head><base href="/garage-ui/"></head><body></body></html>');
    expect(resolveBasePath(doc)).toBe('/garage-ui');
  });

  it('normalizes the meta value, so an unnormalized deployment value still works', () => {
    const doc = makeDocument(
      `<html><head><meta name="${BASE_PATH_META_NAME}" content="garage-ui/"></head><body></body></html>`,
    );
    expect(resolveBasePath(doc)).toBe('/garage-ui');
  });

  it('handles a nested prefix', () => {
    const doc = makeDocument(
      `<html><head><meta name="${BASE_PATH_META_NAME}" content="/admin/garage-ui"></head><body></body></html>`,
    );
    expect(resolveBasePath(doc)).toBe('/admin/garage-ui');
  });

  it('falls back to the build-time base when the document carries neither tag', () => {
    const doc = makeDocument('<html><head></head><body></body></html>');
    // The dev server and the default build both mean "root".
    expect(resolveBasePath(doc)).toBe('');
  });

  it('means the root when there is no document at all', () => {
    expect(resolveBasePath(undefined)).toBe('');
  });
});

describe('withBasePath', () => {
  it.each([
    ['/login', '', '/login'],
    ['/login', '/garage-ui', '/garage-ui/login'],
    ['login', '/garage-ui', '/garage-ui/login'],
    ['/api', '/garage-ui', '/garage-ui/api'],
    ['/auth/oidc/login', '/admin/garage-ui', '/admin/garage-ui/auth/oidc/login'],
  ])('prefixes %o under %o', (path, basePath, expected) => {
    expect(withBasePath(path, basePath)).toBe(expected);
  });

  it('maps the app root without producing a trailing slash', () => {
    expect(withBasePath('/', '/garage-ui')).toBe('/garage-ui');
    expect(withBasePath('', '/garage-ui')).toBe('/garage-ui');
    expect(withBasePath('/', '')).toBe('/');
  });

  it('never yields a double slash', () => {
    for (const basePath of ['', '/garage-ui', '/admin/garage-ui']) {
      for (const path of ['/', '/login', 'login', '/api/v1/health']) {
        expect(withBasePath(path, basePath)).not.toMatch(/\/\//);
      }
    }
  });
});

describe('stripBasePath', () => {
  it.each([
    ['/garage-ui/login', '/garage-ui', '/login'],
    ['/garage-ui', '/garage-ui', '/'],
    ['/garage-ui/', '/garage-ui', '/'],
    ['/login', '', '/login'],
    ['/garage-ui/buckets/x/objects/a/b.txt', '/garage-ui', '/buckets/x/objects/a/b.txt'],
  ])('strips %o under %o', (path, basePath, expected) => {
    expect(stripBasePath(path, basePath)).toBe(expected);
  });

  it('leaves paths outside the prefix alone', () => {
    expect(stripBasePath('/other/page', '/garage-ui')).toBe('/other/page');
    // A prefix must match on a segment boundary, not as a string prefix.
    expect(stripBasePath('/garage-ui-other/page', '/garage-ui')).toBe('/garage-ui-other/page');
  });
});

describe('routerBasename', () => {
  it('is "/" at the root, because React Router rejects an empty basename', () => {
    expect(routerBasename('')).toBe('/');
  });

  it('is the prefix under a subpath', () => {
    expect(routerBasename('/garage-ui')).toBe('/garage-ui');
    expect(routerBasename('/admin/garage-ui')).toBe('/admin/garage-ui');
  });
});

afterEach(() => {
  document.head.querySelectorAll(`meta[name="${BASE_PATH_META_NAME}"], base`).forEach((el) => el.remove());
});
