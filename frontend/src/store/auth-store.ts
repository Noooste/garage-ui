import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { AuthConfig, AuthUser, AuthState } from '@/types/auth';
import { authApi } from '@/lib/api';
import { withBasePath } from '@/lib/base-path';

// A reverse proxy or IdP whose own session has expired answers the XHR with its
// login page under a 200 instead of letting it through. Storing that body as the
// config makes every `config.admin.enabled` read throw, which takes the whole
// React tree down with it.
const isAuthConfig = (value: unknown): value is AuthConfig =>
  typeof value === 'object' &&
  value !== null &&
  (['admin', 'oidc', 'token'] as const).every(
    (method) =>
      typeof (value as Record<string, { enabled?: unknown } | undefined>)[method]?.enabled ===
      'boolean'
  );

interface AuthStore extends AuthState {
  config: AuthConfig | null;

  // Actions
  setUser: (user: AuthUser | null) => void;
  setConfig: (config: AuthConfig) => void;
  setLoading: (loading: boolean) => void;
  setError: (error: string | null) => void;
  setAuthenticated: (authenticated: boolean) => void;

  // Async actions
  initialize: () => Promise<void>;
  loginAdmin: (username: string, password: string) => Promise<void>;
  loginToken: (token: string) => Promise<void>;
  loginOIDC: () => void;
  logout: () => Promise<void>;
}

export const useAuthStore = create<AuthStore>()(
  persist(
    (set, get) => ({
      user: null,
      config: null,
      isAuthenticated: false,
      isLoading: true,
      error: null,

      setUser: (user) => set({ user, isAuthenticated: !!user }),
      setConfig: (config) => set({ config }),
      setLoading: (isLoading) => set({ isLoading }),
      setError: (error) => set({ error }),
      setAuthenticated: (isAuthenticated) => set({ isAuthenticated }),

      initialize: async () => {
        try {
          set({ isLoading: true, error: null });

          // Fetch auth configuration
          const configResponse = await authApi.getConfig();
          if (!isAuthConfig(configResponse.data)) {
            throw new Error('Unexpected /auth/config response');
          }
          const config = configResponse.data;
          set({ config });

          // If no auth is enabled, mark as authenticated immediately
          if (!config.admin.enabled && !config.oidc.enabled && !config.token.enabled) {
            set({
              isAuthenticated: true,
              isLoading: false,
              user: { username: 'guest' }
            });
            return;
          }

          // Try to get current user (check if already authenticated)
          try {
            const userResponse = await authApi.me();
            const user = userResponse.data.user;
            set({
              user,
              isAuthenticated: true,
              isLoading: false
            });
          } catch {
            // Not authenticated - this is okay
            set({
              user: null,
              isAuthenticated: false,
              isLoading: false
            });
          }
        } catch (error) {
          console.error('Failed to initialize auth:', error);
          set({
            error: 'Failed to initialize authentication',
            isLoading: false,
            isAuthenticated: false
          });
        }
      },

      loginAdmin: async (username, password) => {
        try {
          set({ isLoading: true, error: null });

          const response = await authApi.loginAdmin(username, password);
          const { token, user } = response.data;

          // Store token in localStorage
          localStorage.setItem('auth-token', token);

          // Update state
          set({
            user,
            isAuthenticated: true,
            isLoading: false,
            error: null
          });
        } catch (error) {
          const errorMessage =
            (error as { response?: { data?: { error?: { message?: string } } } })
              .response?.data?.error?.message ||
            (error instanceof Error ? error.message : 'Login failed');
          set({
            error: errorMessage,
            isLoading: false,
            isAuthenticated: false,
            user: null
          });
          throw error; // Re-throw for form handling
        }
      },

      loginToken: async (token) => {
        try {
          set({ isLoading: true, error: null });

          const response = await authApi.loginToken(token);
          const { token: sessionToken, user } = response.data;

          localStorage.setItem('auth-token', sessionToken);

          set({
            user,
            isAuthenticated: true,
            isLoading: false,
            error: null,
          });
        } catch (error) {
          const errorMessage =
            (error as { response?: { data?: { error?: { message?: string } } } })
              .response?.data?.error?.message ||
            (error instanceof Error ? error.message : 'Login failed');
          set({
            error: errorMessage,
            isLoading: false,
            isAuthenticated: false,
            user: null,
          });
          throw error;
        }
      },

      loginOIDC: () => {
        // Redirect to OIDC login endpoint (prefixed when deployed on a subpath)
        window.location.href = withBasePath('/auth/oidc/login');
      },

      logout: async () => {
        const { config } = get();
        let logoutUrl: string | undefined;

        try {
          // Call logout endpoint for OIDC mode
          if (config?.oidc.enabled) {
            const response = await authApi.logoutOIDC();
            logoutUrl = response.data.logout_url;
          } else if (config?.admin.enabled) {
            await authApi.logoutAdmin();
          }
        } catch (error) {
          console.error('Logout API call failed:', error);
        }

        // Clear local storage
        localStorage.removeItem('auth-token');

        // Clear state
        set({
          user: null,
          isAuthenticated: false,
          error: null
        });

        // Redirect to login page (prefixed when deployed on a subpath)
        window.location.href = withBasePath(logoutUrl ?? '/login');
      },
    }),
    {
      name: 'auth-storage',
      partialize: (state) => ({
        user: state.user,
        // Don't persist config, isLoading, or error
      }),
    }
  )
);
