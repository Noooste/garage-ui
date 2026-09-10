import { useEffect } from 'react';
import {BrowserRouter, Navigate, Route, Routes} from 'react-router';
import {QueryClientProvider} from '@tanstack/react-query';
import {ThemeProvider} from '@/components/theme-provider';
import {useTheme} from '@/components/theme-context';
import {Layout} from '@/components/layout/layout';
import {BucketDetailShell} from '@/components/layout/bucket-detail-shell';
import {Dashboard} from '@/pages/Dashboard';
import {Buckets} from '@/pages/Buckets';
import {BucketObjects} from '@/pages/BucketObjects';
import {ObjectDetailsView} from '@/components/buckets/ObjectDetailsView';
import {BucketPermissions} from '@/pages/BucketPermissions';
import {BucketWebsite} from '@/pages/BucketWebsite';
import {BucketSettings} from '@/pages/BucketSettings';
import {Cluster} from '@/pages/Cluster';
import {AccessControl} from '@/pages/AccessControl';
import {Login} from '@/pages/Login';
import {Toaster} from 'sonner';
import {queryClient} from '@/lib/query-client';
import {useAuthStore} from '@/store/auth-store';
import {ProtectedRoute} from '@/components/auth/ProtectedRoute';
import {routerBasename} from '@/lib/base-path';
import {LoadingSpinner} from '@/components/auth/LoadingSpinner';

function ThemedToaster() {
  const { theme } = useTheme();
  return (
    <Toaster
      richColors
      position="bottom-right"
      theme={theme}
      toastOptions={{
        classNames: {
          toast:
            'rounded-lg border border-[var(--border)] bg-[var(--card)] text-[var(--foreground)] font-sans shadow-lg',
          title: 'text-[14px] font-medium',
          description: 'text-[13px] text-[var(--muted-foreground)]',
        },
      }}
    />
  );
}

function App() {
  const { initialize, isLoading } = useAuthStore();

  useEffect(() => {
    initialize();
  }, [initialize]);

  if (isLoading) {
    return <LoadingSpinner />;
  }

  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider defaultTheme="system" storageKey="Noooste/garage-ui-theme">
        {/* basename keeps client-side routing working when the app is served
            from a subpath (issue #107); it is "/" for a root deployment. */}
        <BrowserRouter basename={routerBasename()}>
          <Routes>
            <Route path="/login" element={<Login />} />

            <Route
              path="/"
              element={
                <ProtectedRoute>
                  <Layout />
                </ProtectedRoute>
              }
            >
              <Route index element={<Dashboard />} />
              <Route path="buckets" element={<Buckets />} />
              <Route path="buckets/:bucketName" element={<BucketDetailShell />}>
                <Route index element={<Navigate to="objects" replace />} />
                <Route path="objects" element={<BucketObjects />} />
                <Route path="objects/*" element={<ObjectDetailsView />} />
                <Route path="permissions" element={<BucketPermissions />} />
                <Route path="website" element={<BucketWebsite />} />
                <Route path="settings" element={<BucketSettings />} />
              </Route>
              <Route path="cluster" element={<Cluster />} />
              <Route path="access" element={<AccessControl />} />
            </Route>
          </Routes>
        </BrowserRouter>
        <ThemedToaster />
      </ThemeProvider>
    </QueryClientProvider>
  );
}

export default App;
