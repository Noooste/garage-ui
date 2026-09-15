/**
 * Build-time base path handling for VITE_BASE_PATH (issue #107).
 *
 * Lives outside src/ because vite.config.ts is type-checked as its own project
 * (tsconfig.node.json) and must not pull in browser-only types. Kept in one
 * place so the value that ends up in `base`, in `<base href>` and in the meta
 * tag can never drift apart.
 */

/**
 * Canonical form: '' (root) or '/prefix' with no trailing slash. A leading
 * slash is not optional — a relative `<base href="garage-ui/">` would resolve
 * against the current route, so `./garage.png` on /buckets/foo would end up at
 * /buckets/garage-ui/garage.png.
 */
export function normalizeVitePrefix(raw: string | undefined | null): string {
  if (!raw) return '';

  const segments = raw
    .trim()
    .split('/')
    .filter((segment) => segment !== '' && segment !== '.' && segment !== '..');

  return segments.length === 0 ? '' : `/${segments.join('/')}`;
}
