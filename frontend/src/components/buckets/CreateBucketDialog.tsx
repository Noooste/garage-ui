import { useState, type ReactNode } from 'react';
import { AlertTriangle, Check, Database, ShieldCheck, X } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Checkbox } from '@/components/ui/checkbox';
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { IconTile } from '@/components/ui/icon-tile';
import { CredentialField } from '@/components/ui/credential-field';
import type { CreateBucketResult, KeyPermissions, NewKeyRequest } from '@/lib/create-bucket-with-key';
import { toast } from 'sonner';

interface CreateBucketDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreateBucket: (name: string, key?: NewKeyRequest) => Promise<CreateBucketResult>;
  canCreateKey: boolean;
}

const DEFAULT_PERMISSIONS: KeyPermissions = { read: true, write: true, owner: false };

const PERMISSION_ROWS = [
  { field: 'read', label: 'Read', desc: 'GetObject, HeadObject, ListObjects' },
  { field: 'write', label: 'Write', desc: 'PutObject, DeleteObject' },
  { field: 'owner', label: 'Owner', desc: 'DeleteBucket, PutBucketPolicy' },
] as const;

function StatusRow({ ok, children }: { ok: boolean; children: ReactNode }) {
  return (
    <li className="flex items-start gap-2 text-[13.5px]">
      {ok ? (
        <Check className="mt-0.5 h-4 w-4 flex-shrink-0 text-[var(--primary)]" />
      ) : (
        <X className="mt-0.5 h-4 w-4 flex-shrink-0 text-destructive" />
      )}
      <span className="flex-1">{children}</span>
    </li>
  );
}

export function CreateBucketDialog({
  open,
  onOpenChange,
  onCreateBucket,
  canCreateKey,
}: CreateBucketDialogProps) {
  const [bucketName, setBucketName] = useState('');
  const [withKey, setWithKey] = useState(false);
  const [keyName, setKeyName] = useState('');
  const [permissions, setPermissions] = useState<KeyPermissions>(DEFAULT_PERMISSIONS);
  const [creating, setCreating] = useState(false);
  const [result, setResult] = useState<CreateBucketResult | null>(null);

  // Closing resets the form here rather than in an effect on `open`: Cancel,
  // Done, the close button, the backdrop and Escape all route through Dialog's
  // onOpenChange, so this is the single place a close can happen.
  const handleOpenChange = (next: boolean) => {
    if (!next) {
      setBucketName('');
      setWithKey(false);
      setKeyName('');
      setPermissions(DEFAULT_PERMISSIONS);
      setCreating(false);
      setResult(null);
    }
    onOpenChange(next);
  };

  const derivedKeyName = `${bucketName || 'my-bucket'}-key`;

  const handleCreate = async () => {
    if (!bucketName) {
      toast.error('Please enter a bucket name');
      return;
    }

    setCreating(true);
    try {
      const outcome = await onCreateBucket(
        bucketName,
        withKey && canCreateKey ? { name: keyName || derivedKeyName, permissions } : undefined,
      );

      // Which panel to show follows from the outcome, not from the form state.
      if (outcome.bucket === 'failed') return; // the axios interceptor already toasted
      if (outcome.key || outcome.keyError) {
        setResult(outcome);
        return;
      }
      handleOpenChange(false);
    } finally {
      setCreating(false);
    }
  };

  if (result) {
    const degraded = !!result.keyError || !!result.grantError;
    return (
      <Dialog open={open} onOpenChange={handleOpenChange} size="form">
        <DialogContent>
          <DialogHeader>
            <IconTile
              icon={degraded ? <AlertTriangle /> : <ShieldCheck />}
              tone="primary"
              size="md"
            />
            <div className="min-w-0 flex-1">
              <DialogTitle>{result.key ? 'Bucket and key created' : 'Bucket created'}</DialogTitle>
              <DialogDescription>
                {result.key
                  ? 'Copy your secret access key now, this is the only time it will be shown.'
                  : 'The bucket is ready, but the access key was not created.'}
              </DialogDescription>
            </div>
          </DialogHeader>
          <DialogBody className="space-y-5">
            <ul className="space-y-2">
              <StatusRow ok>
                Bucket <span className="font-mono">{bucketName}</span> created
              </StatusRow>
              {result.key ? (
                <StatusRow ok>
                  Access key <span className="font-mono">{result.key.name}</span> created
                </StatusRow>
              ) : (
                <StatusRow ok={false}>
                  Access key could not be created. You can create one from Access control.
                </StatusRow>
              )}
              {result.key && (
                <StatusRow ok={!result.grantError}>
                  {result.grantError
                    ? 'Permissions were not applied on the bucket. Grant them from Access control.'
                    : 'Permissions granted on the bucket'}
                </StatusRow>
              )}
            </ul>

            {result.key && (
              <>
                <CredentialField label="Access Key ID" value={result.key.accessKeyId} breakAll />
                <CredentialField label="Secret Access Key" value={result.key.secretKey} breakAll />
                <div className="flex gap-3 rounded-lg border border-[var(--accent-primary-border)] bg-[var(--accent-primary-soft)] px-3.5 py-3">
                  <AlertTriangle className="mt-0.5 h-4 w-4 flex-shrink-0 text-[var(--primary)]" />
                  <div className="space-y-0.5">
                    <p className="text-[13.5px] font-medium text-[var(--foreground)]">
                      Save this key now
                    </p>
                    <p className="text-[12.5px] leading-[1.5] text-[var(--muted-foreground)]">
                      The secret access key cannot be retrieved again. If lost, you'll need to create a new key.
                    </p>
                  </div>
                </div>
              </>
            )}
          </DialogBody>
          <DialogFooter>
            <Button onClick={() => handleOpenChange(false)}>Done</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    );
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange} size="form">
      <DialogContent>
        <DialogHeader>
          <IconTile icon={<Database />} tone="primary" size="md" />
          <div className="flex-1">
            <DialogTitle>Create New Bucket</DialogTitle>
            <DialogDescription>
              Create a new storage bucket for your objects
            </DialogDescription>
          </div>
        </DialogHeader>
        <DialogBody className="space-y-5">
          <div className="space-y-2">
            <label htmlFor="new-bucket-name" className="text-sm font-medium">
              Bucket Name
            </label>
            <Input
              id="new-bucket-name"
              autoFocus
              placeholder="my-bucket-name"
              value={bucketName}
              onChange={(e) => setBucketName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  handleCreate();
                }
              }}
            />
            <p className="text-xs text-muted-foreground">
              Must be unique and follow DNS naming conventions
            </p>
          </div>

          {canCreateKey && (
            <div className="space-y-3 rounded-lg border border-[var(--border)] p-4">
              <label className="flex cursor-pointer items-start gap-3">
                <Checkbox
                  checked={withKey}
                  className="mt-0.5"
                  onCheckedChange={(checked) => setWithKey(checked as boolean)}
                />
                <div className="flex-1">
                  <div className="text-[13.5px] font-medium">Also create an access key</div>
                  <p className="mt-0.5 text-[12.5px] text-[var(--muted-foreground)]">
                    Optional, an S3 key scoped to this bucket. You can also do this later from Access control.
                  </p>
                </div>
              </label>

              {withKey && (
                <div className="space-y-4 border-t border-[var(--border)] pt-4">
                  <div className="space-y-1.5">
                    <label htmlFor="new-bucket-key-name" className="text-[13px] font-medium">
                      Key name
                    </label>
                    <Input
                      id="new-bucket-key-name"
                      placeholder={derivedKeyName}
                      value={keyName}
                      onChange={(e) => setKeyName(e.target.value)}
                    />
                    <p className="text-[12.5px] text-[var(--muted-foreground)]">
                      Leave empty to use {derivedKeyName}.
                    </p>
                  </div>

                  <div className="space-y-1.5">
                    <label className="text-[13px] font-medium">Permissions</label>
                    <div className="divide-y divide-[var(--border)] rounded-md border border-[var(--border)]">
                      {PERMISSION_ROWS.map((p) => (
                        <label
                          key={p.field}
                          htmlFor={`new-bucket-key-${p.field}`}
                          className="flex cursor-pointer items-start gap-3 px-3.5 py-3 transition-colors hover:bg-[var(--accent)]"
                        >
                          <Checkbox
                            id={`new-bucket-key-${p.field}`}
                            checked={permissions[p.field]}
                            className="mt-0.5"
                            onCheckedChange={(checked) =>
                              setPermissions((prev) => ({ ...prev, [p.field]: checked as boolean }))
                            }
                          />
                          <div className="flex-1">
                            <div className="text-[13.5px] font-medium">{p.label}</div>
                            <p className="mt-0.5 font-mono text-[12px] text-[var(--muted-foreground)]">
                              {p.desc}
                            </p>
                          </div>
                        </label>
                      ))}
                    </div>
                  </div>
                </div>
              )}
            </div>
          )}
        </DialogBody>
        <DialogFooter>
          <Button variant="secondary" onClick={() => handleOpenChange(false)}>
            Cancel
          </Button>
          <Button variant="primary" onClick={handleCreate} disabled={!bucketName || creating}>
            {creating ? 'Creating…' : 'Create'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
