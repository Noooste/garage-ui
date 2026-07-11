import { Outlet, useLocation, useParams } from 'react-router-dom';
import { Sidebar } from './sidebar';
import { TopBar } from './top-bar';
import { useState, useMemo } from 'react';
import { Menu } from 'lucide-react';
import { Button } from '@/components/ui/button';
import type { BreadcrumbItem } from '@/components/ui/breadcrumb';
import { usePermissions } from '@/hooks/usePermissions';
import { NoAccess } from '@/pages/NoAccess';

function useCrumbs(): BreadcrumbItem[] {
  const location = useLocation();
  const params = useParams();
  return useMemo(() => {
    const path = location.pathname;
    if (path === '/') return [{ label: 'Dashboard' }];
    if (path === '/cluster') return [{ label: 'Cluster' }];
    if (path === '/access') return [{ label: 'Access Control' }];
    if (path === '/buckets') return [{ label: 'Buckets' }];
    if (path.startsWith('/buckets/')) {
      const bucketName = (params as { bucketName?: string }).bucketName ?? path.split('/')[2];
      const crumbs: BreadcrumbItem[] = [
        { label: 'Buckets', to: '/buckets' },
        { label: bucketName, to: `/buckets/${bucketName}/objects` },
      ];
      const segs = path.split('/').slice(3); // after /buckets/:name
      if (segs[0] && segs[0] !== 'objects') {
        const tabLabel = segs[0][0].toUpperCase() + segs[0].slice(1);
        crumbs.push({ label: tabLabel });
      }
      return crumbs;
    }
    return [];
  }, [location.pathname, params]);
}

export function Layout() {
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const crumbs = useCrumbs();
  const { noAccess } = usePermissions();

  return (
    <div className="flex h-screen overflow-hidden bg-[var(--background)]">
      <Button
        variant="ghost"
        size="icon"
        className="fixed left-3 top-3 z-50 md:hidden"
        onClick={() => setSidebarOpen(!sidebarOpen)}
        aria-label="Toggle navigation"
      >
        <Menu className="h-5 w-5" />
      </Button>

      {sidebarOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/50 md:hidden"
          onClick={() => setSidebarOpen(false)}
        />
      )}

      <Sidebar isOpen={sidebarOpen} onClose={() => setSidebarOpen(false)} />
      <div className="flex min-w-0 flex-1 flex-col">
        <TopBar crumbs={crumbs} />
        <main className="flex-1 overflow-y-auto scrollbar-thin">
          {noAccess ? <NoAccess /> : <Outlet />}
        </main>
      </div>
    </div>
  );
}
