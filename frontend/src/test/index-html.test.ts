import { describe, expect, it } from 'vitest';
import html from '../../index.html?raw';
import viteConfig from '../../vite.config.ts?raw';
import css from '../index.css?raw';
import { normalizeVitePrefix } from '../../vite-base-path';

/**
 * Issue #107: the deployment's subpath is injected into index.html by the
 * backend at request time (internal/routes.InjectBasePath), and the asset URLs
 * have to be relative so `<base href="/prefix/">` can anchor them.
 *
 * These are contract tests between the frontend build and that injection: if
 * the tags are renamed or the base config goes absolute, the backend silently
 * stops rewriting and a subpath deployment 404s on every asset.
 */

describe('index.html', () => {
  it('ships a <base> tag for the backend to rewrite', () => {
    // The backend's regex matches <base ... href="...">.
    expect(html).toMatch(/<base[^>]*\shref\s*=\s*"\/"/);
  });

  it('ships the base-path meta tag the app reads at runtime', () => {
    expect(html).toMatch(/<meta[^>]*\sname\s*=\s*"garage-ui-base-path"[^>]*\scontent\s*=\s*""/);
  });

  it('has exactly one of each, so injection cannot duplicate them', () => {
    expect(html.match(/<base\s/g)).toHaveLength(1);
    expect(html.match(/garage-ui-base-path/g)).toHaveLength(1);
  });

  it('keeps the doctype first, so injection never triggers quirks mode', () => {
    expect(html.trimStart().startsWith('<!doctype html>')).toBe(true);
  });

  it('references static files relatively so <base href> can anchor them', () => {
    // Absolute /foo.png would bypass <base href> and 404 under a subpath.
    // /src/main.tsx is the dev-server entry and is replaced at build time.
    const absoluteRefs = [...html.matchAll(/(?:src|href)="(\/[^/"][^"]*)"/g)]
      .map((match) => match[1])
      .filter((url) => url !== '/' && url !== '/src/main.tsx');
    expect(absoluteRefs).toEqual([]);
  });
});

describe('vite config', () => {
  it('emits relative asset URLs by default', () => {
    expect(viteConfig).toMatch(/base:\s*envBasePath\s*\|\|\s*'\.\/'/);
  });

  it('bakes the tags in when a fixed prefix is requested', () => {
    // VITE_BASE_PATH is the "serve dist/ from nginx" escape hatch; without the
    // rewritten tags the app would resolve its base path as the root.
    expect(viteConfig).toMatch(/garage-ui-base-path-tags/);
    expect(viteConfig).toMatch(/envBasePath \? \[react\(\), basePathTags\(envBasePath\)\]/);
  });

  it('feeds the normalized prefix to both the base and the tags', () => {
    expect(viteConfig).toMatch(/normalizeVitePrefix\(process\.env\.VITE_BASE_PATH\)/);
    expect(viteConfig).toMatch(/base:\s*envBasePath\s*\|\|\s*'\.\/'/);
  });
});

/**
 * Review #5: VITE_BASE_PATH was only stripped of trailing slashes. A value
 * without a leading slash produced <base href="garage-ui/">, which resolves
 * against the current route — so ./garage.png 404s at /buckets/foo.
 */
describe('normalizeVitePrefix', () => {
  it.each([
    ['garage-ui', '/garage-ui'],
    ['garage-ui/', '/garage-ui'],
    ['/garage-ui', '/garage-ui'],
    ['/garage-ui/', '/garage-ui'],
    ['//garage-ui//', '/garage-ui'],
    ['admin/garage-ui', '/admin/garage-ui'],
    ['  garage-ui  ', '/garage-ui'],
    ['/', ''],
    ['', ''],
    [undefined, ''],
  ])('normalizes %o to %o', (raw, expected) => {
    expect(normalizeVitePrefix(raw)).toBe(expected);
  });

  it('produces a base href that survives a deep route', () => {
    const href = `${normalizeVitePrefix('garage-ui')}/`;
    const resolved = new URL('./garage.png', new URL(href, 'https://host/buckets/foo')).href;
    expect(resolved).toBe('https://host/garage-ui/garage.png');
  });
});

/**
 * The normalizer is documented as a mirror of config.NormalizeBasePath. Where
 * the backend refuses to boot, the build has to fail too: a dist/ baked with a
 * prefix the server rejects is worse than a build error, because it only shows
 * up as a CrashLoopBackOff. The value is also interpolated into HTML
 * attributes, so the charset is what keeps it from breaking out of one.
 */
describe('normalizeVitePrefix rejections', () => {
  it.each([
    ['/api', 'reserved first segment'],
    ['/Health', 'reserved first segment, case-folded'],
    ['/assets', 'reserved first segment'],
    ['/a/../b', '".." segment'],
    ['/a<b>', 'character outside the segment charset'],
    ['/a"b', 'character outside the segment charset'],
    ['/a b', 'character outside the segment charset'],
    ['/a?b', 'query separator'],
    ['/a#b', 'fragment separator'],
    ['/a$1b', '"$" would be a replacement reference'],
  ])('rejects %o (%s)', (raw) => {
    expect(() => normalizeVitePrefix(raw)).toThrow(/VITE_BASE_PATH/);
  });
});

describe('index.css', () => {
  it('references fonts relatively so they follow the deployment prefix', () => {
    // url(...) resolves against the stylesheet, not the document, so a leading
    // slash is origin-absolute and <base href> cannot rebase it: every face
    // 404s under a subpath and the UI silently falls back to system fonts.
    const absolute = [...css.matchAll(/url\(\s*['"]?(\/[^)'"]*)/g)].map((match) => match[1]);
    expect(absolute).toEqual([]);
  });
});
