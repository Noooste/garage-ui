import { render, screen, waitFor } from '@testing-library/react';
import { Outlet } from 'react-router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { BASE_PATH_META_NAME } from '@/lib/base-path';

/**
 * Issue #107: client-side routing has to work when the SPA is served from a
 * subpath. React Router only does that if it is given a basename — without it
 * every route lookup sees "/garage-ui/buckets" and matches nothing.
 *
 * The layout, the pages and the auth store are stubbed: what is under test is
 * the routing wiring, not what the pages render.
 */

const authState = {
  initialize: vi.fn(),
  isLoading: false,
  isAuthenticated: true,
  user: { username: 'u' },
  config: { admin: { enabled: false }, oidc: { enabled: false }, token: { enabled: false } },
};

vi.mock('@/store/auth-store', () => ({
  useAuthStore: (selector?: (state: typeof authState) => unknown) =>
    selector ? selector(authState) : authState,
}));

vi.mock('sonner', () => ({
  Toaster: () => null,
  toast: { error: vi.fn(), success: vi.fn() },
}));

vi.mock('@/components/layout/layout', () => ({
  Layout: () => (
    <div data-testid="layout">
      <Outlet />
    </div>
  ),
}));

vi.mock('@/components/layout/bucket-detail-shell', () => ({
  BucketDetailShell: () => (
    <div data-testid="bucket-shell">
      <Outlet />
    </div>
  ),
}));

const page = (name: string) => () => <div data-testid={`page-${name}`}>{name}</div>;

vi.mock('@/pages/Dashboard', () => ({ Dashboard: page('dashboard') }));
vi.mock('@/pages/Buckets', () => ({ Buckets: page('buckets') }));
vi.mock('@/pages/BucketObjects', () => ({ BucketObjects: page('objects') }));
vi.mock('@/pages/BucketPermissions', () => ({ BucketPermissions: page('permissions') }));
vi.mock('@/pages/BucketWebsite', () => ({ BucketWebsite: page('website') }));
vi.mock('@/pages/BucketSettings', () => ({ BucketSettings: page('settings') }));
vi.mock('@/pages/Cluster', () => ({ Cluster: page('cluster') }));
vi.mock('@/pages/AccessControl', () => ({ AccessControl: page('access') }));
vi.mock('@/pages/Login', () => ({ Login: page('login') }));
vi.mock('@/components/buckets/ObjectDetailsView', () => ({ ObjectDetailsView: page('object-details') }));

// jsdom has no matchMedia; the theme provider asks for the system theme.
if (!window.matchMedia) {
  window.matchMedia = ((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia;
}

function setBasePath(basePath: string) {
  document.head.querySelectorAll(`meta[name="${BASE_PATH_META_NAME}"]`).forEach((el) => el.remove());
  const meta = document.createElement('meta');
  meta.setAttribute('name', BASE_PATH_META_NAME);
  meta.setAttribute('content', basePath);
  document.head.appendChild(meta);
}

/** Mirrors a browser landing on `url` before the bundle boots. */
async function renderAppAt(basePath: string, url: string) {
  setBasePath(basePath);
  window.history.pushState({}, '', url);
  vi.resetModules();
  const { default: App } = await import('./App');
  return render(<App />);
}

beforeEach(() => {
  authState.isAuthenticated = true;
  authState.isLoading = false;
});

afterEach(() => {
  document.head.querySelectorAll(`meta[name="${BASE_PATH_META_NAME}"]`).forEach((el) => el.remove());
  window.history.pushState({}, '', '/');
  vi.clearAllMocks();
});

describe('routing under a base path', () => {
  it.each([
    ['/garage-ui/', 'page-dashboard'],
    ['/garage-ui/buckets', 'page-buckets'],
    ['/garage-ui/cluster', 'page-cluster'],
    ['/garage-ui/access', 'page-access'],
    ['/garage-ui/login', 'page-login'],
    ['/garage-ui/buckets/my-bucket/objects', 'page-objects'],
    ['/garage-ui/buckets/my-bucket/permissions', 'page-permissions'],
  ])('resolves %s', async (url, testId) => {
    await renderAppAt('/garage-ui', url);
    expect(await screen.findByTestId(testId)).toBeInTheDocument();
  });

  it('resolves a nested prefix', async () => {
    await renderAppAt('/admin/garage-ui', '/admin/garage-ui/buckets');
    expect(await screen.findByTestId('page-buckets')).toBeInTheDocument();
  });

  it('keeps working at the root', async () => {
    await renderAppAt('', '/buckets');
    expect(await screen.findByTestId('page-buckets')).toBeInTheDocument();
  });

  // The regression this locks in: without a basename the router sees the full
  // "/garage-ui/buckets" path, matches nothing, and the user gets a blank page.
  it('does not fall through to an empty match', async () => {
    await renderAppAt('/garage-ui', '/garage-ui/buckets');
    expect(screen.queryByTestId('page-dashboard')).not.toBeInTheDocument();
    expect(await screen.findByTestId('page-buckets')).toBeInTheDocument();
  });
});

describe('router-driven navigation', () => {
  it('keeps redirects inside the base path', async () => {
    authState.isAuthenticated = false;
    authState.config = { admin: { enabled: true }, oidc: { enabled: false }, token: { enabled: false } };

    await renderAppAt('/garage-ui', '/garage-ui/buckets');

    // ProtectedRoute sends unauthenticated users to /login; the router has to
    // put the prefix back on when it writes the URL.
    await waitFor(() => {
      expect(window.location.pathname).toBe('/garage-ui/login');
    });
    expect(window.location.search).toContain('returnUrl=%2Fbuckets');

    authState.config = { admin: { enabled: false }, oidc: { enabled: false }, token: { enabled: false } };
  });

  it('resolves the index redirect of a bucket route under the prefix', async () => {
    await renderAppAt('/garage-ui', '/garage-ui/buckets/my-bucket');

    // <Navigate to="objects" replace /> is relative, so it must land on the
    // prefixed objects route.
    expect(await screen.findByTestId('page-objects')).toBeInTheDocument();
    await waitFor(() => {
      expect(window.location.pathname).toBe('/garage-ui/buckets/my-bucket/objects');
    });
  });
});
