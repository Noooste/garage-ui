/**
 * Build-time base path handling for VITE_BASE_PATH (issue #107).
 *
 * Lives outside src/ because vite.config.ts is type-checked as its own project
 * (tsconfig.node.json) and must not pull in browser-only types. Kept in one
 * place so the value that ends up in `base`, in `<base href>` and in the meta
 * tag can never drift apart.
 */

/**
 * Path roots the Go server serves itself, which a prefix would shadow. Mirrors
 * config.reservedFirstSegments; the backend folds case, so this compares
 * lowercased.
 */
const RESERVED_FIRST_SEGMENTS = new Set([
  'api',
  'assets',
  'auth',
  'docs',
  'health',
  'login',
  'metrics',
]);

/**
 * Mirrors config.segmentPattern. Deliberately narrower than RFC 3986: the
 * value is interpolated into HTML attributes, so everything that could break
 * out of one is rejected rather than escaped.
 */
const SEGMENT_PATTERN = /^[A-Za-z0-9._~%+-]+$/;

/**
 * Canonical form: '' (root) or '/prefix' with no trailing slash. A leading
 * slash is not optional — a relative `<base href="garage-ui/">` would resolve
 * against the current route, so `./garage.png` on /buckets/foo would end up at
 * /buckets/garage-ui/garage.png.
 *
 * Throws on anything config.NormalizeBasePath would reject, so a bad
 * VITE_BASE_PATH fails the build instead of producing a dist/ that the backend
 * refuses to boot with. Empty segments are collapsed, not rejected, so a
 * templated '//garage//ui/' still works.
 */
export function normalizeVitePrefix(raw: string | undefined | null): string {
  if (!raw) return '';

  const trimmed = raw.trim();
  if (trimmed === '') return '';

  const reject = (reason: string): never => {
    throw new Error(`VITE_BASE_PATH ${JSON.stringify(raw)}: ${reason}`);
  };

  if (/[?#\\]/.test(trimmed)) {
    reject('must not contain "?", "#" or "\\"');
  }

  const segments = trimmed.split('/').filter((segment) => segment !== '' && segment !== '.');
  if (segments.length === 0) return '';

  for (const segment of segments) {
    if (segment === '..') {
      reject('must not contain ".." segments');
    }
    if (!SEGMENT_PATTERN.test(segment)) {
      reject(`segment ${JSON.stringify(segment)} must match ${SEGMENT_PATTERN.source}`);
    }
  }

  if (RESERVED_FIRST_SEGMENTS.has(segments[0].toLowerCase())) {
    reject(
      `${JSON.stringify(segments[0])} is a route the server already serves ` +
        `(reserved first segments: ${[...RESERVED_FIRST_SEGMENTS].sort().join(', ')})`,
    );
  }

  return `/${segments.join('/')}`;
}
