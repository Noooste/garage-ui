export interface AuthConfig {
  admin: {
    enabled: boolean;
  };
  oidc: {
    enabled: boolean;
    provider?: string;
  };
  token: {
    enabled: boolean;
  };
}

export interface AuthUser {
  username: string;
  email?: string;
  name?: string;
}

export interface AuthState {
  user: AuthUser | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  error: string | null;
}
