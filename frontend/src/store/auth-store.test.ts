import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { BASE_PATH_META_NAME } from '@/lib/base-path';

/**
 * Issue #107: the store performs full-page redirects, which bypass the router
 * and therefore have to carry the base path themselves.
 */

const logoutOIDC = vi.fn().mockResolvedValue(undefined);
const logoutAdmin = vi.fn().mockResolvedValue(undefined);

vi.mock('@/lib/api', () => ({
  authApi: {
    logoutOIDC: (...args: unknown[]) => logoutOIDC(...args),
    logoutAdmin: (...args: unknown[]) => logoutAdmin(...args),
    getConfig: vi.fn(),
    me: vi.fn(),
    loginAdmin: vi.fn(),
    loginToken: vi.fn(),
    loginOIDC: vi.fn(),
  },
}));

function setBasePath(basePath: string) {
  document.head.querySelectorAll(`meta[name="${BASE_PATH_META_NAME}"]`).forEach((el) => el.remove());
  const meta = document.createElement('meta');
  meta.setAttribute('name', BASE_PATH_META_NAME);
  meta.setAttribute('content', basePath);
  document.head.appendChild(meta);
}

function stubLocation(): { href: string; pathname: string } {
  const location = { pathname: '/', href: '' };
  Object.defineProperty(window, 'location', { value: location, writable: true, configurable: true });
  return location;
}

async function loadStore(basePath: string) {
  setBasePath(basePath);
  vi.resetModules();
  const { useAuthStore } = await import('./auth-store');
  return useAuthStore;
}

const originalLocation = window.location;

beforeEach(() => {
  localStorage.clear();
});

afterEach(() => {
  vi.clearAllMocks();
  document.head.querySelectorAll(`meta[name="${BASE_PATH_META_NAME}"]`).forEach((el) => el.remove());
  Object.defineProperty(window, 'location', {
    value: originalLocation,
    writable: true,
    configurable: true,
  });
});

describe('loginOIDC', () => {
  it('redirects to the prefixed OIDC endpoint', async () => {
    const location = stubLocation();
    const useAuthStore = await loadStore('/garage-ui');

    useAuthStore.getState().loginOIDC();

    expect(location.href).toBe('/garage-ui/auth/oidc/login');
  });

  it('redirects to the unprefixed endpoint at the root', async () => {
    const location = stubLocation();
    const useAuthStore = await loadStore('');

    useAuthStore.getState().loginOIDC();

    expect(location.href).toBe('/auth/oidc/login');
  });
});

describe('logout', () => {
  it('returns to the prefixed login page and clears the token', async () => {
    const location = stubLocation();
    const useAuthStore = await loadStore('/garage-ui');
    localStorage.setItem('auth-token', 'token');

    await useAuthStore.getState().logout();

    expect(location.href).toBe('/garage-ui/login');
    expect(localStorage.getItem('auth-token')).toBeNull();
    expect(useAuthStore.getState().isAuthenticated).toBe(false);
  });

  it('returns to /login at the root', async () => {
    const location = stubLocation();
    const useAuthStore = await loadStore('');

    await useAuthStore.getState().logout();

    expect(location.href).toBe('/login');
  });

  it('still redirects when the logout call fails', async () => {
    const location = stubLocation();
    const useAuthStore = await loadStore('/garage-ui');
    useAuthStore.setState({ config: { admin: { enabled: false }, oidc: { enabled: true }, token: { enabled: false } } });
    logoutOIDC.mockRejectedValueOnce(new Error('network'));

    await useAuthStore.getState().logout();

    expect(location.href).toBe('/garage-ui/login');
  });

  // The IdP's end_session_endpoint is an absolute URL that already carries
  // post_logout_redirect_uri. Prefixing it turns it into an in-app path, the
  // SPA shell answers, and the SSO session is never ended - so the next login
  // silently skips credentials.
  it('sends the IdP logout URL through untouched', async () => {
    const location = stubLocation();
    const useAuthStore = await loadStore('/garage-ui');
    useAuthStore.setState({ config: { admin: { enabled: false }, oidc: { enabled: true }, token: { enabled: false } } });
    const idpLogout =
      'https://keycloak.example/realms/r/protocol/openid-connect/logout?id_token_hint=abc';
    logoutOIDC.mockResolvedValueOnce({ data: { logout_url: idpLogout } });

    await useAuthStore.getState().logout();

    expect(location.href).toBe(idpLogout);
  });

  it('falls back to the prefixed login page when the provider has no end_session_endpoint', async () => {
    const location = stubLocation();
    const useAuthStore = await loadStore('/garage-ui');
    useAuthStore.setState({ config: { admin: { enabled: false }, oidc: { enabled: true }, token: { enabled: false } } });
    logoutOIDC.mockResolvedValueOnce({ data: {} });

    await useAuthStore.getState().logout();

    expect(location.href).toBe('/garage-ui/login');
  });
});
