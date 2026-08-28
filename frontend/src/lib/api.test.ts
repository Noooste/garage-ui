import axios, { type AxiosInstance } from 'axios';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { BASE_PATH_META_NAME } from './base-path';

/**
 * Issue #107: every URL the API client builds — the axios base URLs, the 401
 * redirect, the OIDC login hand-off — has to carry the deployment's base path,
 * otherwise a subpath deployment talks to the reverse proxy's root.
 */

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

type LoadedApi = typeof import('./api');

interface LoadResult {
  api: LoadedApi;
  /** axios instances in creation order: [apiClient, authApiClient]. */
  instances: AxiosInstance[];
}

/**
 * api.ts resolves the base path once at module load, exactly like the browser
 * does, so each case sets up the document and then imports it fresh.
 */
async function loadApi(basePath: string | null): Promise<LoadResult> {
  document.head.querySelectorAll(`meta[name="${BASE_PATH_META_NAME}"]`).forEach((el) => el.remove());
  if (basePath !== null) {
    const meta = document.createElement('meta');
    meta.setAttribute('name', BASE_PATH_META_NAME);
    meta.setAttribute('content', basePath);
    document.head.appendChild(meta);
  }

  const instances: AxiosInstance[] = [];
  const realCreate = axios.create.bind(axios);
  vi.spyOn(axios, 'create').mockImplementation((config) => {
    const instance = realCreate(config);
    instances.push(instance);
    return instance;
  });

  vi.resetModules();
  const api = await import('./api');
  return { api, instances };
}

function stubLocation(pathname: string): { href: string; pathname: string } {
  const location = { pathname, href: '' };
  Object.defineProperty(window, 'location', {
    value: location,
    writable: true,
    configurable: true,
  });
  return location;
}

const originalLocation = window.location;

beforeEach(() => {
  localStorage.clear();
});

afterEach(() => {
  vi.restoreAllMocks();
  document.head.querySelectorAll(`meta[name="${BASE_PATH_META_NAME}"]`).forEach((el) => el.remove());
  Object.defineProperty(window, 'location', {
    value: originalLocation,
    writable: true,
    configurable: true,
  });
});

describe('axios base URLs', () => {
  it('serve from the root by default', async () => {
    const { instances } = await loadApi('');
    expect(instances.map((i) => i.defaults.baseURL)).toEqual(['/api', '/auth']);
  });

  it('carry the base path under a subpath deployment', async () => {
    const { instances } = await loadApi('/garage-ui');
    expect(instances.map((i) => i.defaults.baseURL)).toEqual(['/garage-ui/api', '/garage-ui/auth']);
  });

  it('handle a nested prefix', async () => {
    const { instances } = await loadApi('/admin/garage-ui');
    expect(instances.map((i) => i.defaults.baseURL)).toEqual([
      '/admin/garage-ui/api',
      '/admin/garage-ui/auth',
    ]);
  });

  it('normalize an unnormalized deployment value', async () => {
    const { instances } = await loadApi('garage-ui/');
    expect(instances.map((i) => i.defaults.baseURL)).toEqual(['/garage-ui/api', '/garage-ui/auth']);
  });
});

describe('OIDC login redirect', () => {
  it('points at the prefixed backend endpoint', async () => {
    const location = stubLocation('/garage-ui/login');
    const { api } = await loadApi('/garage-ui');

    api.authApi.loginOIDC();

    expect(location.href).toBe('/garage-ui/auth/oidc/login');
  });

  it('is unprefixed for a root deployment', async () => {
    const location = stubLocation('/login');
    const { api } = await loadApi('');

    api.authApi.loginOIDC();

    expect(location.href).toBe('/auth/oidc/login');
  });
});

describe('401 handling', () => {
  /** Makes the api instance reject every request with the given status. */
  function failWith(instance: AxiosInstance, status: number) {
    instance.defaults.adapter = () =>
      Promise.reject({
        response: { status, data: null, statusText: 'x' },
        request: {},
        message: 'failed',
      });
  }

  it('redirects to the prefixed login route', async () => {
    const location = stubLocation('/garage-ui/buckets');
    const { api, instances } = await loadApi('/garage-ui');
    failWith(instances[0], 401);
    localStorage.setItem('auth-token', 'stale');

    await expect(api.healthApi.getVersion()).rejects.toBeDefined();

    expect(location.href).toBe('/garage-ui/login');
    expect(localStorage.getItem('auth-token')).toBeNull();
  });

  it('does not redirect when already on the prefixed login route', async () => {
    const location = stubLocation('/garage-ui/login');
    const { api, instances } = await loadApi('/garage-ui');
    failWith(instances[0], 401);

    await expect(api.healthApi.getVersion()).rejects.toBeDefined();

    // Redirecting here would reload the login page in a loop.
    expect(location.href).toBe('');
  });

  it('still redirects when the unprefixed login path is the current one', async () => {
    // /login is a different page than /garage-ui/login under a subpath
    // deployment, so the loop guard must not match it.
    const location = stubLocation('/login');
    const { api, instances } = await loadApi('/garage-ui');
    failWith(instances[0], 401);

    await expect(api.healthApi.getVersion()).rejects.toBeDefined();

    expect(location.href).toBe('/garage-ui/login');
  });

  it('redirects to /login for a root deployment', async () => {
    const location = stubLocation('/buckets');
    const { api, instances } = await loadApi('');
    failWith(instances[0], 401);

    await expect(api.healthApi.getVersion()).rejects.toBeDefined();

    expect(location.href).toBe('/login');
  });
});
