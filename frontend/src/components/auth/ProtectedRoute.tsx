import { Navigate, useLocation } from 'react-router';
import { useAuthStore } from '@/store/auth-store';
import { LoadingSpinner } from './LoadingSpinner';

interface ProtectedRouteProps {
  children: React.ReactNode;
}

export function ProtectedRoute({ children }: ProtectedRouteProps) {
  const { isAuthenticated, isLoading, config } = useAuthStore();
  const location = useLocation();

  if (isLoading) {
    return <LoadingSpinner />;
  }

  // If no auth is enabled, always allow access
  if (config && !config.admin.enabled && !config.oidc.enabled) {
    return <>{children}</>;
  }

  // If not authenticated, redirect to login with return URL
  if (!isAuthenticated) {
    const returnUrl = encodeURIComponent(location.pathname + location.search);
    return <Navigate to={`/login?returnUrl=${returnUrl}`} replace />;
  }

  return <>{children}</>;
}
