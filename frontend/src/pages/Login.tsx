import { useEffect } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useAuthStore } from '@/store/auth-store';
import { BasicLoginForm } from '@/components/auth/BasicLoginForm';
import { OIDCLoginView } from '@/components/auth/OIDCLoginView';
import { TokenLoginForm } from '@/components/auth/TokenLoginForm';
import { LoadingSpinner } from '@/components/auth/LoadingSpinner';

export function Login() {
  const { config, isLoading, initialize, isAuthenticated } = useAuthStore();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();

  const loginSuccess = searchParams.get('login');
  const returnUrl = searchParams.get('returnUrl') || '/';

  useEffect(() => {
    if (loginSuccess === 'success') {
      initialize().then(() => {
        navigate(decodeURIComponent(returnUrl));
      });
    }
  }, [loginSuccess, initialize, navigate, returnUrl]);

  useEffect(() => {
    if (isAuthenticated && !loginSuccess) {
      navigate(decodeURIComponent(returnUrl));
    }
  }, [isAuthenticated, navigate, returnUrl, loginSuccess]);

  if (isLoading || loginSuccess === 'success') {
    return <LoadingSpinner />;
  }

  // No auth enabled, redirect to dashboard immediately
  if (config && !config.admin.enabled && !config.oidc.enabled && !config.token.enabled) {
    navigate('/');
    return null;
  }

  const showAdmin = config?.admin.enabled || false;
  const showOIDC = config?.oidc.enabled || false;
  const showToken = config?.token.enabled || false;

  // Token-only auth (zero-config fallback)
  if (showToken && !showAdmin && !showOIDC) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background p-4">
        <div className="w-full max-w-md">
          <TokenLoginForm />
        </div>
      </div>
    );
  }

  // Both admin and OIDC enabled
  if (showAdmin && showOIDC) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background p-4">
        <div className="w-full max-w-md">
          <BasicLoginForm showOIDC={true} config={config} />
        </div>
      </div>
    );
  }

  // Only OIDC
  if (showOIDC) {
    return <OIDCLoginView />;
  }

  // Only admin
  if (showAdmin) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-background p-4">
        <div className="w-full max-w-md">
          <BasicLoginForm />
        </div>
      </div>
    );
  }

  return <LoadingSpinner />;
}
