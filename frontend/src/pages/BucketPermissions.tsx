import { useState } from 'react';
import { useParams } from 'react-router';
import { KeyRound, ShieldCheck } from 'lucide-react';
import { useAccessKeys, useGrantBucketPermission } from '@/hooks/useApi';
import type { AccessKey } from '@/types';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Select, SelectOption } from '@/components/ui/select';
import { EmptyState } from '@/components/ui/empty-state';
import { Badge } from '@/components/ui/badge';
import { toast } from 'sonner';

// Stable fallback so the render-adjust below doesn't see a fresh array
// identity on every render while the keys are still loading.
const NO_KEYS: AccessKey[] = [];

export function BucketPermissions() {
  const { bucketName = '' } = useParams<{ bucketName: string }>();
  const { data: availableKeys = NO_KEYS } = useAccessKeys();
  const grant = useGrantBucketPermission();

  const [selectedKey, setSelectedKey] = useState('');
  const [read, setRead] = useState(false);
  const [write, setWrite] = useState(false);
  const [owner, setOwner] = useState(false);

  // Prefill the checkboxes from the selected key's existing grant (adjust
  // during render, not an effect).
  const [prevSync, setPrevSync] = useState({ selectedKey, availableKeys, bucketName });
  if (
    prevSync.selectedKey !== selectedKey ||
    prevSync.availableKeys !== availableKeys ||
    prevSync.bucketName !== bucketName
  ) {
    setPrevSync({ selectedKey, availableKeys, bucketName });
    if (!selectedKey) {
      setRead(false); setWrite(false); setOwner(false);
    } else {
      const key = availableKeys.find((k) => k.accessKeyId === selectedKey);
      const existing = key?.permissions.find(
        (p) => p.bucketName === bucketName || p.bucketId === bucketName,
      );
      setRead(existing?.read ?? false);
      setWrite(existing?.write ?? false);
      setOwner(existing?.owner ?? false);
    }
  }

  const canSubmit = !!selectedKey && (read || write || owner) && !grant.isPending;

  const onGrant = async () => {
    if (!selectedKey) { toast.error('Please select an access key'); return; }
    if (!read && !write && !owner) { toast.error('Please select at least one permission'); return; }
    try {
      await grant.mutateAsync({
        bucketName,
        accessKeyId: selectedKey,
        permissions: { read, write, owner },
      });
      setSelectedKey(''); setRead(false); setWrite(false); setOwner(false);
    } catch {
      // error toast handled by axios interceptor
    }
  };

  const granted = availableKeys
    .map((k) => ({
      key: k,
      perm: k.permissions.find(
        (p) => p.bucketName === bucketName || p.bucketId === bucketName,
      ),
    }))
    .filter((x) => !!x.perm);

  return (
    <div className="space-y-6 px-7 py-6">
      <section className="rounded-xl border border-[var(--border)] bg-[var(--card)]">
        <header className="flex items-center gap-2 border-b border-[var(--border)] px-5 py-3">
          <ShieldCheck className="h-4 w-4 text-[var(--primary)]" />
          <h2 className="text-[15px] font-semibold">Grant access</h2>
        </header>
        <div className="space-y-5 px-5 py-5">
          <div className="space-y-1.5">
            <label className="text-[13.5px] font-medium">Access key</label>
            <Select value={selectedKey} onChange={(v) => setSelectedKey(v)}>
              <SelectOption value="">-- Select an access key --</SelectOption>
              {availableKeys.map((k) => (
                <SelectOption key={k.accessKeyId} value={k.accessKeyId}>
                  {k.name} ({k.accessKeyId})
                </SelectOption>
              ))}
            </Select>
            <p className="text-[12.5px] text-[var(--muted-foreground)]">
              Choose which access key should have permissions on this bucket. Current permissions pre-fill below.
            </p>
          </div>

          <div className="space-y-1.5">
            <div className="text-[13.5px] font-medium">Permissions</div>
            <div className="space-y-3 rounded-lg border border-[var(--border)] p-4">
              <PermRow
                id="perm-read"
                checked={read}
                onChange={setRead}
                title="Read"
                description="Allows reading objects from the bucket (GetObject, HeadObject, ListObjects)"
              />
              <PermRow
                id="perm-write"
                checked={write}
                onChange={setWrite}
                title="Write"
                description="Allows writing and deleting objects in the bucket (PutObject, DeleteObject)"
              />
              <PermRow
                id="perm-owner"
                checked={owner}
                onChange={setOwner}
                title="Owner"
                description="Allows managing bucket settings and policies (DeleteBucket, PutBucketPolicy)"
              />
            </div>
          </div>

          <div className="pt-1">
            <Button onClick={onGrant} disabled={!canSubmit}>
              {grant.isPending ? 'Granting…' : 'Grant access'}
            </Button>
          </div>
        </div>
      </section>

      <section className="rounded-xl border border-[var(--border)] bg-[var(--card)]">
        <header className="flex items-center gap-2 border-b border-[var(--border)] px-5 py-3">
          <KeyRound className="h-4 w-4 text-[var(--primary)]" />
          <h2 className="text-[15px] font-semibold">Granted</h2>
        </header>
        {granted.length === 0 ? (
          <div className="p-5">
            <EmptyState
              icon={<KeyRound />}
              tone="neutral"
              title="No access granted"
              description="Grant at least one access key to make this bucket usable."
            />
          </div>
        ) : (
          <ul className="divide-y divide-[var(--border)]">
            {granted.map(({ key, perm }) => (
              <li key={key.accessKeyId} className="flex items-center gap-4 px-5 py-3">
                <div className="min-w-0 flex-1">
                  <div className="truncate text-[14px] font-medium">{key.name}</div>
                  <div className="truncate font-mono text-[12.5px] text-[var(--muted-foreground)]">
                    {key.accessKeyId}
                  </div>
                </div>
                <div className="flex items-center gap-1">
                  {perm!.read && <Badge variant="success">Read</Badge>}
                  {perm!.write && <Badge variant="warning">Write</Badge>}
                  {perm!.owner && <Badge variant="primary">Owner</Badge>}
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}

function PermRow({
  id,
  checked,
  onChange,
  title,
  description,
}: {
  id: string;
  checked: boolean;
  onChange: (v: boolean) => void;
  title: string;
  description: string;
}) {
  return (
    <div className="flex items-start gap-3">
      <Checkbox id={id} checked={checked} onCheckedChange={(c) => onChange(c as boolean)} />
      <div className="flex-1">
        <label htmlFor={id} className="text-[14px] font-medium leading-none cursor-pointer">
          {title}
        </label>
        <p className="mt-1 text-[12.5px] text-[var(--muted-foreground)]">{description}</p>
      </div>
    </div>
  );
}
