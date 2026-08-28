/**
 * Subpath support (issue #107).
 *
 * The build is deployment-agnostic: assets are emitted with relative URLs and
 * the backend injects the deployment's prefix into index.html at request time
 * (`<base href="/prefix/">` plus `<meta name="garage-ui-base-path">`). One
 * image can therefore be served from `/`, `/garage-ui`, or `/admin/garage-ui`
 * without a rebuild.
 *
 * Everything the app builds by hand — the router basename, the axios base
 * URLs, the full-page redirects — has to go through here.
 */

export const BASE_PATH_META_NAME = 'garage-ui-base-path';

/**
 * Canonical form: '' (served from the root) or '/prefix' with no trailing
 * slash. Mirrors config.NormalizeBasePath on the backend.
 */
export function normalizeBasePath(raw: string | null | undefined): string {
  if (!raw) return '';

  const trimmed = raw.trim();
  if (trimmed === '' || trimmed === '/') return '';

  // A `<base href>` is an absolute or relative URL, not a bare path; keep only
  // its path component so 'https://host.ts.net/garage-ui/' works too.
  let path = trimmed;
  const schemeMatch = /^[a-z][a-z0-9+.-]*:\/\/[^/]*(\/.*)?$/i.exec(path);
  if (schemeMatch) {
    path = schemeMatch[1] ?? '';
  }

  const segments: string[] = [];
  for (const segment of path.split('/')) {
    if (segment === '' || segment === '.') continue;
    if (segment === '..') {
      segments.pop();
      continue;
    }
    segments.push(segment);
  }
  if (segments.length === 0) return '';

  return '/' + segments.join('/');
}

function readMetaBasePath(doc: Document | undefined): string | null {
  const meta = doc?.querySelector(`meta[name="${BASE_PATH_META_NAME}"]`);
  if (!meta) return null;
  // An empty content attribute is a deliberate "served from the root", not a
  // missing value — return it so the <base href> fallback is skipped.
  return meta.getAttribute('content') ?? null;
}

function readBaseHref(doc: Document | undefined): string | null {
  const base = doc?.querySelector('base');
  return base?.getAttribute('href') ?? null;
}

function readBuildBase(): string | null {
  try {
    return import.meta.env?.BASE_URL ?? null;
  } catch {
    return null;
  }
}

/**
 * Resolves the base path this app instance is mounted under.
 *
 * Precedence: the backend-injected meta tag, then `<base href>`, then Vite's
 * build-time base (dev server / a build pinned to a fixed base). Anything
 * unresolvable means the root.
 */
export function resolveBasePath(doc: Document | undefined = typeof document === 'undefined' ? undefined : document): string {
  const meta = readMetaBasePath(doc);
  if (meta !== null) return normalizeBasePath(meta);

  const baseHref = readBaseHref(doc);
  if (baseHref !== null) return normalizeBasePath(baseHref);

  return normalizeBasePath(readBuildBase());
}

/**
 * The base path resolved once at module load. index.html is rendered by the
 * server before the bundle runs, so this never changes during a session.
 */
export const BASE_PATH: string = resolveBasePath();

/** Prefixes an absolute in-app path, e.g. '/login' -> '/garage-ui/login'. */
export function withBasePath(path: string, basePath: string = BASE_PATH): string {
  if (path === '' || path === '/') return basePath === '' ? '/' : basePath;
  const absolute = path.startsWith('/') ? path : `/${path}`;
  return `${basePath}${absolute}`;
}

/**
 * Strips the base path from a browser path, so route comparisons work the same
 * under a subpath as at the root. Paths outside the prefix are returned as-is.
 */
export function stripBasePath(path: string, basePath: string = BASE_PATH): string {
  if (basePath === '') return path;
  if (path === basePath) return '/';
  if (path.startsWith(`${basePath}/`)) return path.slice(basePath.length);
  return path;
}

/**
 * React Router's basename. '/' is what the router expects for a root
 * deployment; it rejects an empty string.
 */
export function routerBasename(basePath: string = BASE_PATH): string {
  return basePath === '' ? '/' : basePath;
}
