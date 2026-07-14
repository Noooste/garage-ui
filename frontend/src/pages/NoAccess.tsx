import { ShieldOff } from 'lucide-react';
import { EmptyState } from '@/components/ui/empty-state';

/** Shown to authenticated users whose identity matches no team. */
export function NoAccess() {
  return (
    <div className="flex min-h-[60vh] items-center justify-center px-6 py-6">
      <EmptyState
        icon={<ShieldOff />}
        tone="neutral"
        title="You don't have access"
        description="Your account is signed in but isn't assigned to any team on this Garage UI. Contact your administrator to be added to a team."
      />
    </div>
  );
}
