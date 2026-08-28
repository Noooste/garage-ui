import { describe, expect, it } from 'vitest';
import html from '../../index.html?raw';
import viteConfig from '../../vite.config.ts?raw';

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
});
