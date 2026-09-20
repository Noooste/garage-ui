import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router';

const getConfig = vi.fn();
const me = vi.fn();

vi.mock('@/lib/api', () => ({
  authApi: {
    getConfig: () => getConfig(),
    me: () => me(),
  },
}));

const { useAuthStore } = await import('@/store/auth-store');
const { ProtectedRoute } = await import('./ProtectedRoute');

beforeEach(() => {
  vi.clearAllMocks();
  useAuthStore.setState({
    config: null,
    user: null,
    isAuthenticated: false,
    isLoading: true,
    error: null,
  });
});

// ProtectedRoute redirects by rendering <Navigate>, and its target carries the
// current path as returnUrl. It needs a real /login route to land on, or each
// redirect lands back on itself with a longer returnUrl and never settles.
function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route
          path="/"
          element={
            <ProtectedRoute>
              <div>dashboard</div>
            </ProtectedRoute>
          }
        />
        <Route path="/login" element={<div>login page</div>} />
      </Routes>
    </MemoryRouter>
  );
}

// An auth proxy or IdP whose own session has expired answers the XHR with its
// 200 HTML login page instead of JSON. Storing that body used to make every
// later `config.admin.enabled` read throw, which unmounted the React tree and
// left a white page with no status code behind it.
describe('boot with a hijacked /auth/config response', () => {
  const hijacked = { data: '<!doctype html><html><body>Sign in</body></html>' };

  it('keeps a non-object auth config out of the store', async () => {
    getConfig.mockResolvedValue(hijacked);

    await useAuthStore.getState().initialize();

    expect(useAuthStore.getState().config).toBeNull();
    expect(useAuthStore.getState().isLoading).toBe(false);
    expect(useAuthStore.getState().isAuthenticated).toBe(false);
  });

  it('sends the user to the login page instead of crashing the tree', async () => {
    getConfig.mockResolvedValue(hijacked);
    await useAuthStore.getState().initialize();

    renderAt('/');

    expect(screen.getByText('login page')).toBeInTheDocument();
    expect(screen.queryByText('dashboard')).toBeNull();
  });

  it('still accepts a well-formed config', async () => {
    getConfig.mockResolvedValue({
      data: { admin: { enabled: true }, oidc: { enabled: false }, token: { enabled: false } },
    });
    me.mockRejectedValue(new Error('401'));

    await useAuthStore.getState().initialize();

    expect(useAuthStore.getState().config).toEqual({
      admin: { enabled: true },
      oidc: { enabled: false },
      token: { enabled: false },
    });
  });
});
