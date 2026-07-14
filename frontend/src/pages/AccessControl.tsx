import {useEffect, useMemo, useState} from 'react';
import {cn} from '@/lib/utils';
import {PageHeader} from '@/components/ui/page-header';
import {Button} from '@/components/ui/button';
import {Input} from '@/components/ui/input';
import {Badge} from '@/components/ui/badge';
import {Table, TableBody, TableCell, TableHead, TableHeader, TableRow,} from '@/components/ui/table';
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
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {Tabs, TabsContent} from '@/components/ui/tabs';
import {Card, CardContent, CardHeader, CardTitle} from '@/components/ui/card';
import {Checkbox} from '@/components/ui/checkbox';
import {Select, SelectOption} from '@/components/ui/select';
import {useQueryClient} from '@tanstack/react-query';
import {accessApi, bucketsApi} from '@/lib/api';
import {queryKeys} from '@/lib/query-client';
import {formatDate} from '@/lib/utils';
import type {AccessKey, Bucket, BucketPermission} from '@/types';
import {AlertTriangle, Calendar, Check, Copy, Database, Edit, Eye, EyeOff, Key, KeyRound, Loader2, MoreVertical, Plus, Search, ShieldCheck, ShieldX, Trash2,} from 'lucide-react';
import {toast} from 'sonner';

function CredentialField({
  label,
  value,
  mono = true,
  breakAll = false,
  maskable = false,
  loading = false,
}: {
  label: string;
  value: string;
  mono?: boolean;
  breakAll?: boolean;
  maskable?: boolean;
  loading?: boolean;
}) {
  const [copied, setCopied] = useState(false);
  const [revealed, setRevealed] = useState(!maskable);
  const copy = () => {
    if (!value) return;
    navigator.clipboard.writeText(value);
    setCopied(true);
    toast.success(`${label} copied`);
    setTimeout(() => setCopied(false), 1600);
  };
  const display = loading ? '' : revealed || !maskable ? value : '•'.repeat(Math.min(40, value.length || 40));
  return (
    <div className="space-y-1.5">
      <label className="text-[12px] font-medium uppercase tracking-[0.06em] text-[var(--muted-foreground)]">
        {label}
      </label>
      <div className="flex items-stretch gap-2">
        <button
          type="button"
          onClick={copy}
          disabled={loading || !value}
          title="Click to copy"
          className={cn(
            'flex-1 min-w-0 rounded-md border border-[var(--border)] bg-[var(--surface-sunken)]',
            'px-3 py-2 text-left text-[13.5px] transition-colors hover:bg-[var(--accent)]',
            'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]',
            'disabled:cursor-not-allowed disabled:opacity-70 disabled:hover:bg-[var(--surface-sunken)]',
            mono && 'font-mono',
            breakAll ? 'break-all' : 'truncate',
          )}
        >
          {loading ? (
            <span className="inline-flex items-center gap-2 text-[var(--muted-foreground)]">
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
              Loading…
            </span>
          ) : (
            display
          )}
        </button>
        {maskable && (
          <Button
            variant="secondary"
            size="icon"
            onClick={() => setRevealed((r) => !r)}
            aria-label={revealed ? 'Hide' : 'Reveal'}
            disabled={loading || !value}
          >
            {revealed ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
          </Button>
        )}
        <Button variant="secondary" size="icon" onClick={copy} aria-label={`Copy ${label}`} disabled={loading || !value}>
          {copied ? <Check className="h-4 w-4 text-[var(--primary)]" /> : <Copy className="h-4 w-4" />}
        </Button>
      </div>
    </div>
  );
}

export function AccessControl() {
  const queryClient = useQueryClient();
  const [keys, setKeys] = useState<AccessKey[]>([]);
  const [searchQuery, setSearchQuery] = useState('');
  const [isLoading, setIsLoading] = useState(true);
  const [createDialogOpen, setCreateDialogOpen] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [selectedKey, setSelectedKey] = useState<AccessKey | null>(null);
  const [newKeyName, setNewKeyName] = useState('');

  // Create key with permissions state
  const [createAvailableBuckets, setCreateAvailableBuckets] = useState<Bucket[]>([]);
  const [createSelectedBucket, setCreateSelectedBucket] = useState<string>('');
  const [createPermissionRead, setCreatePermissionRead] = useState(false);
  const [createPermissionWrite, setCreatePermissionWrite] = useState(false);
  const [createPermissionOwner, setCreatePermissionOwner] = useState(false);
  const [createGrantPermissions, setCreateGrantPermissions] = useState(false);
  const [newlyCreatedKey, setNewlyCreatedKey] = useState<AccessKey | null>(null);

  // Edit permissions state
  const [editPermissionsDialogOpen, setEditPermissionsDialogOpen] = useState(false);
  const [editingKey, setEditingKey] = useState<AccessKey | null>(null);
  const [availableBuckets, setAvailableBuckets] = useState<Bucket[]>([]);
  const [selectedBucket, setSelectedBucket] = useState<string>('');
  const [permissionRead, setPermissionRead] = useState(false);
  const [permissionWrite, setPermissionWrite] = useState(false);
  const [permissionOwner, setPermissionOwner] = useState(false);
  const [savingPermissions, setSavingPermissions] = useState(false);

  // Key settings state (activation/expiration)
  const [settingsDialogOpen, setSettingsDialogOpen] = useState(false);
  const [settingsKey, setSettingsKey] = useState<AccessKey | null>(null);
  const [keyStatus, setKeyStatus] = useState<'active' | 'inactive'>('active');
  const [expirationDate, setExpirationDate] = useState<string>('');
  const [neverExpires, setNeverExpires] = useState(true);

  // Secret key dialog state
  const [secretKeyDialogOpen, setSecretKeyDialogOpen] = useState(false);
  const [revealedSecretKey, setRevealedSecretKey] = useState<string>('');
  const [isLoadingSecretKey, setIsLoadingSecretKey] = useState(false);

  // Key details dialog state
  const [keyDetailsDialogOpen, setKeyDetailsDialogOpen] = useState(false);
  const [viewingKey, setViewingKey] = useState<AccessKey | null>(null);
  const [detailsSecretKey, setDetailsSecretKey] = useState<string>('');
  const [isLoadingDetailsSecretKey, setIsLoadingDetailsSecretKey] = useState(false);

  useEffect(() => {
    const fetchKeys = async () => {
      try {
        setIsLoading(true);
        const data = await accessApi.listKeys();
        setKeys(data);
      } catch (error) {
        console.error('Failed to fetch keys:', error);
      } finally {
        setIsLoading(false);
      }
    };

    fetchKeys();
  }, []);

  const filteredKeys = useMemo(
    () =>
      keys.filter(
        (key) =>
          key.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
          key.accessKeyId.toLowerCase().includes(searchQuery.toLowerCase())
      ),
    [keys, searchQuery]
  );

  const handleCreateKey = async () => {
    if (!newKeyName) {
      toast.error('Please enter a key name');
      return;
    }

    try {
      const newKey = await accessApi.createKey(newKeyName);

      // If user wants to grant permissions inline
      if (createGrantPermissions && createSelectedBucket) {
        if (createPermissionRead || createPermissionWrite || createPermissionOwner) {
          try {
            await bucketsApi.grantPermission(createSelectedBucket, newKey.accessKeyId, {
              read: createPermissionRead,
              write: createPermissionWrite,
              owner: createPermissionOwner,
            });
          } catch (error) {
            console.error('Failed to grant permissions:', error);
            // Continue even if permission grant fails - key is already created
          }
        }
      }

      // Store the newly created key to show the secret key
      setNewlyCreatedKey(newKey);

      // Refresh keys list
      const data = await accessApi.listKeys();
      setKeys(data);
      queryClient.invalidateQueries({ queryKey: queryKeys.accessKeys.all });
      if (createGrantPermissions && createSelectedBucket) {
        queryClient.invalidateQueries({ queryKey: queryKeys.buckets.detail(createSelectedBucket) });
      }
      toast.success(`API Key "${newKeyName}" created successfully`);
    } catch (error) {
      // Error toast is handled by API interceptor
      console.error('Create key error:', error);
    }
  };

  const handleCloseCreateDialog = () => {
    setCreateDialogOpen(false);
    setNewKeyName('');
    setCreateSelectedBucket('');
    setCreatePermissionRead(false);
    setCreatePermissionWrite(false);
    setCreatePermissionOwner(false);
    setCreateGrantPermissions(false);
    setNewlyCreatedKey(null);
  };

  const handleOpenCreateDialog = async () => {
    setCreateDialogOpen(true);
    setNewKeyName('');
    setCreateSelectedBucket('');
    setCreatePermissionRead(false);
    setCreatePermissionWrite(false);
    setCreatePermissionOwner(false);
    setCreateGrantPermissions(false);

    // Load available buckets
    try {
      const buckets = await bucketsApi.list();
      setCreateAvailableBuckets(buckets);
    } catch (error) {
      console.error('Failed to load buckets:', error);
    }
  };

  const handleDeleteKey = async () => {
    if (!selectedKey) return;

    try {
      await accessApi.deleteKey(selectedKey.accessKeyId);
      const keyName = selectedKey.name;
      setDeleteDialogOpen(false);
      setSelectedKey(null);

      // Refresh keys list
      const data = await accessApi.listKeys();
      setKeys(data);
      queryClient.invalidateQueries({ queryKey: queryKeys.accessKeys.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.buckets.all });
      toast.success(`API Key "${keyName}" deleted successfully`);
    } catch (error) {
      // Error toast is handled by API interceptor
      console.error('Delete key error:', error);
    }
  };

  const handleOpenSettings = (key: AccessKey) => {
    setSettingsKey(key);
    setKeyStatus(key.status);
    setSettingsDialogOpen(true);

    // Set expiration date if it exists
    if (key.expiration) {
      const expDate = new Date(key.expiration);
      // Format as YYYY-MM-DDTHH:mm for datetime-local input
      const formattedDate = expDate.toISOString().slice(0, 16);
      setExpirationDate(formattedDate);
      setNeverExpires(false);
    } else {
      setExpirationDate('');
      setNeverExpires(true);
    }
  };

  const handleSaveKeySettings = async () => {
    if (!settingsKey) return;

    try {
      const updates: { status?: string; expiration?: string } = {};

      updates.status = keyStatus;

      if (!neverExpires && expirationDate) {
        updates.expiration = new Date(expirationDate).toISOString();
      } else if (neverExpires) {
        // Clear expiration by setting status to active
        updates.status = 'active';
      }

      await accessApi.updateKey(settingsKey.accessKeyId, updates);

      // Refresh keys list
      const data = await accessApi.listKeys();
      setKeys(data);
      queryClient.invalidateQueries({ queryKey: queryKeys.accessKeys.all });

      setSettingsDialogOpen(false);
      toast.success(`Key settings updated successfully`);
    } catch (error) {
      // Error toast is handled by API interceptor
      console.error('Update key settings error:', error);
    }
  };

  const handleRevealSecretKey = async (key: AccessKey) => {
    setSelectedKey(key);
    setIsLoadingSecretKey(true);
    setSecretKeyDialogOpen(true);
    setRevealedSecretKey('');

    try {
      const secretKey = await accessApi.getSecretKey(key.accessKeyId);
      setRevealedSecretKey(secretKey);
    } catch (error) {
      console.error('Failed to fetch secret key:', error);
      setSecretKeyDialogOpen(false);
    } finally {
      setIsLoadingSecretKey(false);
    }
  };

  const handleOpenEditPermissions = async (key: AccessKey) => {
    setEditingKey(key);
    setEditPermissionsDialogOpen(true);
    setSelectedBucket('');
    setPermissionRead(false);
    setPermissionWrite(false);
    setPermissionOwner(false);

    // Load available buckets
    try {
      const buckets = await bucketsApi.list();
      setAvailableBuckets(buckets);
    } catch (error) {
      console.error('Failed to load buckets:', error);
    }
  };

  const handleBucketChange = (bucketName: string) => {
    setSelectedBucket(bucketName);

    if (!bucketName || !editingKey) {
      // Reset permissions if no bucket selected
      setPermissionRead(false);
      setPermissionWrite(false);
      setPermissionOwner(false);
      return;
    }

    // Find if this key already has permissions on the selected bucket
    const bucketPermission = editingKey.permissions.find(
      perm => perm.bucketName === bucketName || perm.bucketId === bucketName
    );

    if (bucketPermission) {
      // Set the checkboxes to reflect current permissions
      setPermissionRead(bucketPermission.read);
      setPermissionWrite(bucketPermission.write);
      setPermissionOwner(bucketPermission.owner);
    } else {
      // No permissions set yet, reset checkboxes
      setPermissionRead(false);
      setPermissionWrite(false);
      setPermissionOwner(false);
    }
  };

  const handleGrantBucketPermission = async () => {
    if (!editingKey || !selectedBucket) {
      toast.error('Please select a bucket');
      return;
    }

    if (!permissionRead && !permissionWrite && !permissionOwner) {
      toast.error('Please select at least one permission');
      return;
    }

    setSavingPermissions(true);
    try {
      // Call backend API to grant bucket permissions
      await bucketsApi.grantPermission(selectedBucket, editingKey.accessKeyId, {
        read: permissionRead,
        write: permissionWrite,
        owner: permissionOwner,
      });

      toast.success(`Permissions granted on bucket "${selectedBucket}" successfully`);
      setEditPermissionsDialogOpen(false);
      setSelectedBucket('');
      setPermissionRead(false);
      setPermissionWrite(false);
      setPermissionOwner(false);

      // Refresh keys list to update permissions
      const data = await accessApi.listKeys();
      setKeys(data);
      queryClient.invalidateQueries({ queryKey: queryKeys.accessKeys.all });
      queryClient.invalidateQueries({ queryKey: queryKeys.buckets.detail(selectedBucket) });
    } catch (error) {
      // Error toast is handled by API interceptor
      console.error('Grant permission error:', error);
    } finally {
      setSavingPermissions(false);
    }
  };

  // Helper function to format permission flags as a readable string
  const formatPermissions = (perm: BucketPermission): string => {
    const perms = [];
    if (perm.read) perms.push('Read');
    if (perm.write) perms.push('Write');
    if (perm.owner) perms.push('Owner');
    return perms.join(', ') || 'None';
  };

  const handleRowClick = async (key: AccessKey) => {
    setViewingKey(key);
    setKeyDetailsDialogOpen(true);
    setDetailsSecretKey('');
    setIsLoadingDetailsSecretKey(true);

    // Fetch the secret key immediately
    try {
      const secretKey = await accessApi.getSecretKey(key.accessKeyId);
      setDetailsSecretKey(secretKey);
    } catch (error) {
      console.error('Failed to fetch secret key:', error);
    } finally {
      setIsLoadingDetailsSecretKey(false);
    }
  };

  return (
    <div>
      <PageHeader title="Access control" subtitle="Access keys and per-bucket permissions" />
      <div className="p-4 sm:p-6 space-y-4 sm:space-y-6">
        <Tabs defaultValue="keys">
          <TabsContent value="keys" className="space-y-4 sm:space-y-6 mt-4 sm:mt-6">
            {/* Stats */}
            <div className="grid gap-3 sm:gap-4 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3">
              <Card>
                <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                  <CardTitle className="text-sm font-medium">Total Keys</CardTitle>
                  <Key className="h-4 w-4 text-muted-foreground" />
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">{keys.length}</div>
                </CardContent>
              </Card>

              <Card>
                <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                  <CardTitle className="text-sm font-medium">Active Keys</CardTitle>
                  <ShieldCheck className="h-4 w-4 text-green-600" />
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">
                    {keys.filter((k) => k.status === 'active').length}
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                  <CardTitle className="text-sm font-medium">Inactive Keys</CardTitle>
                  <ShieldX className="h-4 w-4 text-red-600" />
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">
                    {keys.filter((k) => k.status === 'inactive').length}
                  </div>
                </CardContent>
              </Card>
            </div>

            {/* Toolbar */}
            <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
              <div className="relative flex-1 max-w-full sm:max-w-xs">
                <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
                <Input
                  placeholder="Search keys..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  className="pl-8"
                />
              </div>
              <Button onClick={handleOpenCreateDialog} className="w-full sm:w-auto">
                <Plus className="h-4 w-4" />
                Create Key
              </Button>
            </div>

            {/* Keys Table */}
            <div className="border rounded-lg overflow-visible">
              <div className="overflow-x-auto">
                <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead className="hidden sm:table-cell">Access Key ID</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead className="hidden md:table-cell">Created</TableHead>
                    <TableHead className="hidden md:table-cell">Permissions</TableHead>
                    <TableHead className="w-[50px]"></TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {isLoading ? (
                    <TableRow>
                      <TableCell colSpan={6} className="text-center py-12">
                        <div className="flex items-center justify-center gap-2 text-muted-foreground">
                          <Loader2 className="h-5 w-5 animate-spin" />
                          <span>Loading API keys...</span>
                        </div>
                      </TableCell>
                    </TableRow>
                  ) : filteredKeys.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={6} className="text-center py-12 text-muted-foreground">
                        {searchQuery ? 'No keys found matching your search' : 'No API keys yet'}
                      </TableCell>
                    </TableRow>
                  ) : (
                    filteredKeys.map((key) => (
                      <TableRow
                        key={key.accessKeyId}
                        onClick={() => handleRowClick(key)}
                        className="cursor-pointer hover:bg-muted/50"
                      >
                        <TableCell className="font-medium truncate max-w-[150px]">{key.name}</TableCell>
                        <TableCell className="hidden sm:table-cell">
                          <div className="flex items-center gap-2">
                            <code
                              className="text-xs bg-muted px-2 py-1 rounded truncate max-w-[150px] block cursor-pointer hover:bg-muted/80 transition-colors"
                              onClick={(e) => {
                                e.stopPropagation();
                                navigator.clipboard.writeText(key.accessKeyId);
                                toast.success('Access Key ID copied to clipboard');
                              }}
                            >
                              {key.accessKeyId}
                            </code>
                            <Button
                              variant="ghost"
                              size="icon"
                              className="h-6 w-6 flex-shrink-0"
                              onClick={(e) => {
                                e.stopPropagation();
                                navigator.clipboard.writeText(key.accessKeyId);
                                toast.success('Access Key ID copied to clipboard');
                              }}
                            >
                              <Copy className="h-3 w-3" />
                            </Button>
                          </div>
                        </TableCell>
                        <TableCell>
                          <Badge variant={key.status === 'active' ? 'primary' : 'neutral'}>
                            {key.status}
                          </Badge>
                        </TableCell>
                        <TableCell className="hidden md:table-cell">{formatDate(key.createdAt)}</TableCell>
                        <TableCell className="hidden md:table-cell">
                          <div className="flex flex-wrap gap-1">
                            {key.permissions.slice(0, 2).map((perm, idx) => (
                              <Badge key={idx} variant="neutral" className="text-xs">
                                {perm.bucketName}: {formatPermissions(perm)}
                              </Badge>
                            ))}
                            {key.permissions.length > 2 && (
                              <Badge variant="neutral" className="text-xs">
                                +{key.permissions.length - 2} more
                              </Badge>
                            )}
                            {key.permissions.length === 0 && (
                              <span className="text-xs text-muted-foreground">No permissions</span>
                            )}
                          </div>
                        </TableCell>
                        <TableCell onClick={(e) => e.stopPropagation()}>
                          <DropdownMenu>
                            <DropdownMenuTrigger>
                              <Button variant="ghost" size="icon">
                                <MoreVertical className="h-4 w-4" />
                              </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end">
                              <DropdownMenuItem onClick={() => handleRevealSecretKey(key)}>
                                <Key className="h-4 w-4" />
                                View Secret Key
                              </DropdownMenuItem>
                              <DropdownMenuSeparator />
                              <DropdownMenuItem onClick={() => handleOpenEditPermissions(key)}>
                                <Edit className="h-4 w-4" />
                                Edit Permissions
                              </DropdownMenuItem>
                              <DropdownMenuItem onClick={() => handleOpenSettings(key)}>
                                {key.status === 'active' ? (
                                  <>
                                    <ShieldX className="h-4 w-4" />
                                    Manage Status
                                  </>
                                ) : (
                                  <>
                                    <ShieldCheck className="h-4 w-4" />
                                    Manage Status
                                  </>
                                )}
                              </DropdownMenuItem>
                              <DropdownMenuSeparator />
                              <DropdownMenuItem
                                className="text-destructive"
                                onClick={() => {
                                  setSelectedKey(key);
                                  setDeleteDialogOpen(true);
                                }}
                              >
                                <Trash2 className="h-4 w-4" />
                                Delete
                              </DropdownMenuItem>
                            </DropdownMenuContent>
                          </DropdownMenu>
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
              </div>
            </div>
          </TabsContent>
        </Tabs>
      </div>

      {/* Create Key Dialog */}
      <Dialog open={createDialogOpen} onOpenChange={handleCloseCreateDialog} size="form">
        <DialogContent>
          {newlyCreatedKey ? (
            <>
              <DialogHeader>
                <IconTile icon={<ShieldCheck />} tone="primary" size="md" />
                <div className="min-w-0 flex-1">
                  <DialogTitle>API key created</DialogTitle>
                  <DialogDescription>
                    Copy your secret access key now, this is the only time it will be shown.
                  </DialogDescription>
                </div>
              </DialogHeader>
              <DialogBody className="space-y-5">
                <div className="space-y-1.5">
                  <label className="text-[12px] font-medium uppercase tracking-[0.06em] text-[var(--muted-foreground)]">
                    Key name
                  </label>
                  <div className="text-[14px] font-medium">{newlyCreatedKey.name}</div>
                </div>
                <CredentialField label="Access Key ID" value={newlyCreatedKey.accessKeyId} />
                <CredentialField
                  label="Secret Access Key"
                  value={newlyCreatedKey.secretKey || ''}
                  breakAll
                />
                <div className="flex gap-3 rounded-lg border border-[var(--accent-primary-border)] bg-[var(--accent-primary-soft)] px-3.5 py-3">
                  <AlertTriangle className="h-4 w-4 flex-shrink-0 text-[var(--primary)] mt-0.5" />
                  <div className="space-y-0.5">
                    <p className="text-[13.5px] font-medium text-[var(--foreground)]">
                      Save this key now
                    </p>
                    <p className="text-[12.5px] leading-[1.5] text-[var(--muted-foreground)]">
                      The secret access key cannot be retrieved again. If lost, you'll need to create a new key.
                    </p>
                  </div>
                </div>
              </DialogBody>
              <DialogFooter>
                <Button onClick={handleCloseCreateDialog}>Done</Button>
              </DialogFooter>
            </>
          ) : (
            <>
              <DialogHeader>
                <IconTile icon={<KeyRound />} tone="primary" size="md" />
                <div className="min-w-0 flex-1">
                  <DialogTitle>Create API key</DialogTitle>
                  <DialogDescription>
                    Generate a new access key pair. You can optionally grant bucket permissions in the same step.
                  </DialogDescription>
                </div>
              </DialogHeader>
              <DialogBody className="space-y-6">
                <div className="space-y-1.5">
                  <label htmlFor="new-key-name" className="text-[13px] font-medium">
                    Key name
                  </label>
                  <Input
                    id="new-key-name"
                    placeholder="e.g. backups-prod"
                    value={newKeyName}
                    onChange={(e) => setNewKeyName(e.target.value)}
                    autoFocus
                  />
                  <p className="text-[12.5px] text-[var(--muted-foreground)]">
                    A friendly name to identify this key in the console.
                  </p>
                </div>

                <div className="space-y-3 rounded-lg border border-[var(--border)] p-4">
                  <label className="flex cursor-pointer items-start gap-3">
                    <Checkbox
                      id="grant-permissions-on-create"
                      checked={createGrantPermissions}
                      className="mt-0.5"
                      onCheckedChange={(checked) => {
                        setCreateGrantPermissions(checked as boolean);
                        if (!checked) {
                          setCreateSelectedBucket('');
                          setCreatePermissionRead(false);
                          setCreatePermissionWrite(false);
                          setCreatePermissionOwner(false);
                        }
                      }}
                    />
                    <div className="flex-1">
                      <div className="text-[13.5px] font-medium">Grant bucket permissions now</div>
                      <p className="mt-0.5 text-[12.5px] text-[var(--muted-foreground)]">
                        Optional, you can also do this later from the key's edit menu.
                      </p>
                    </div>
                  </label>

                  {createGrantPermissions && (
                    <div className="space-y-4 border-t border-[var(--border)] pt-4">
                      <div className="space-y-1.5">
                        <label className="text-[13px] font-medium">Bucket</label>
                        <Select
                          value={createSelectedBucket}
                          onChange={(value) => setCreateSelectedBucket(value)}
                        >
                          <SelectOption value="">Select a bucket…</SelectOption>
                          {createAvailableBuckets.map((bucket) => (
                            <SelectOption key={bucket.name} value={bucket.name}>
                              {bucket.name}
                            </SelectOption>
                          ))}
                        </Select>
                      </div>

                      {createSelectedBucket && (
                        <div className="space-y-1.5">
                          <label className="text-[13px] font-medium">Permissions</label>
                          <div className="divide-y divide-[var(--border)] rounded-md border border-[var(--border)]">
                            {[
                              {
                                id: 'create-permission-read',
                                label: 'Read',
                                desc: 'GetObject, HeadObject, ListObjects',
                                checked: createPermissionRead,
                                setChecked: setCreatePermissionRead,
                              },
                              {
                                id: 'create-permission-write',
                                label: 'Write',
                                desc: 'PutObject, DeleteObject',
                                checked: createPermissionWrite,
                                setChecked: setCreatePermissionWrite,
                              },
                              {
                                id: 'create-permission-owner',
                                label: 'Owner',
                                desc: 'DeleteBucket, PutBucketPolicy',
                                checked: createPermissionOwner,
                                setChecked: setCreatePermissionOwner,
                              },
                            ].map((p) => (
                              <label
                                key={p.id}
                                htmlFor={p.id}
                                className="flex cursor-pointer items-start gap-3 px-3.5 py-3 transition-colors hover:bg-[var(--accent)]"
                              >
                                <Checkbox
                                  id={p.id}
                                  checked={p.checked}
                                  className="mt-0.5"
                                  onCheckedChange={(checked) => p.setChecked(checked as boolean)}
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
                      )}
                    </div>
                  )}
                </div>
              </DialogBody>
              <DialogFooter>
                <Button variant="secondary" onClick={handleCloseCreateDialog}>
                  Cancel
                </Button>
                <Button onClick={handleCreateKey} disabled={!newKeyName}>
                  Create key
                </Button>
              </DialogFooter>
            </>
          )}
        </DialogContent>
      </Dialog>

      {/* Delete Key Dialog */}
      <ConfirmDialog
        open={deleteDialogOpen}
        onOpenChange={setDeleteDialogOpen}
        title={`Delete "${selectedKey?.name ?? ''}"?`}
        description="Applications using this key will lose access immediately."
        confirmLabel="Delete key"
        onConfirm={handleDeleteKey}
      />

      {/* Secret Key Dialog */}
      <Dialog open={secretKeyDialogOpen} onOpenChange={setSecretKeyDialogOpen} size="form">
        <DialogContent>
          <DialogHeader>
            <IconTile icon={<KeyRound />} tone="primary" size="md" />
            <div className="min-w-0 flex-1">
              <DialogTitle className="truncate">{selectedKey?.name || 'Access key'}</DialogTitle>
              <DialogDescription>
                Reveal and copy this key's credentials. The secret is fetched on demand.
              </DialogDescription>
            </div>
          </DialogHeader>
          <DialogBody className="space-y-4">
            <CredentialField
              label="Access Key ID"
              value={selectedKey?.accessKeyId || ''}
              breakAll
            />
            <CredentialField
              label="Secret Access Key"
              value={revealedSecretKey}
              breakAll
              maskable
              loading={isLoadingSecretKey}
            />
          </DialogBody>
          <DialogFooter>
            <Button onClick={() => setSecretKeyDialogOpen(false)}>Close</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Key Settings Dialog */}
      <Dialog open={settingsDialogOpen} onOpenChange={setSettingsDialogOpen} size="form">
        <DialogContent>
          <DialogHeader>
            <IconTile icon={<ShieldCheck />} tone="primary" size="md" />
            <div className="min-w-0 flex-1">
              <DialogTitle className="truncate">Key settings · {settingsKey?.name}</DialogTitle>
              <DialogDescription>Manage activation and expiration for this access key.</DialogDescription>
            </div>
          </DialogHeader>
          <DialogBody className="space-y-6">
            <div className="space-y-2">
              <label className="text-[12px] font-medium uppercase tracking-[0.06em] text-[var(--muted-foreground)]">
                Status
              </label>
              <div className="grid grid-cols-2 gap-2">
                {(['active', 'inactive'] as const).map((s) => {
                  const selected = keyStatus === s;
                  return (
                    <button
                      key={s}
                      type="button"
                      onClick={() => setKeyStatus(s)}
                      className={cn(
                        'flex items-center justify-center gap-2 rounded-md border px-3 py-2.5 text-[13.5px] font-medium transition-colors',
                        selected
                          ? 'border-[var(--primary)] bg-[var(--accent-primary-soft)] text-[var(--foreground)]'
                          : 'border-[var(--border)] text-[var(--muted-foreground)] hover:bg-[var(--accent)] hover:text-[var(--foreground)]',
                      )}
                    >
                      {s === 'active' ? <ShieldCheck className="h-4 w-4" /> : <ShieldX className="h-4 w-4" />}
                      {s[0].toUpperCase() + s.slice(1)}
                    </button>
                  );
                })}
              </div>
              <p className="text-[12.5px] text-[var(--muted-foreground)]">
                Inactive keys cannot be used for authentication.
              </p>
            </div>

            <div className="space-y-3 rounded-lg border border-[var(--border)] p-4">
              <label className="flex cursor-pointer items-start gap-3">
                <Checkbox
                  id="never-expires"
                  checked={neverExpires}
                  className="mt-0.5"
                  onCheckedChange={(checked) => setNeverExpires(checked as boolean)}
                />
                <div className="flex-1">
                  <div className="text-[13.5px] font-medium">Never expires</div>
                  <p className="mt-0.5 text-[12.5px] text-[var(--muted-foreground)]">
                    Turn off to set an automatic expiration date.
                  </p>
                </div>
              </label>

              {!neverExpires && (
                <div className="space-y-1.5 border-t border-[var(--border)] pt-4">
                  <label className="text-[13px] font-medium">Expiration date &amp; time</label>
                  <Input
                    type="datetime-local"
                    value={expirationDate}
                    onChange={(e) => setExpirationDate(e.target.value)}
                    className="w-full"
                  />
                  <p className="text-[12.5px] text-[var(--muted-foreground)]">
                    The key will become inactive after this moment.
                  </p>
                </div>
              )}
            </div>
          </DialogBody>
          <DialogFooter>
            <Button variant="secondary" onClick={() => setSettingsDialogOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleSaveKeySettings}>Save settings</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Key Details Dialog */}
      <Dialog open={keyDetailsDialogOpen} onOpenChange={setKeyDetailsDialogOpen} size="form">
        <DialogContent>
          <DialogHeader>
            <IconTile icon={<KeyRound />} tone="primary" size="md" />
            <div className="min-w-0 flex-1">
              <DialogTitle className="truncate">{viewingKey?.name || 'API key'}</DialogTitle>
              <DialogDescription>View credentials and bucket permissions for this key.</DialogDescription>
            </div>
          </DialogHeader>
          <DialogBody className="space-y-5">
            {/* Meta strip */}
            <div className="flex flex-wrap items-center gap-x-5 gap-y-2 rounded-lg border border-[var(--border)] bg-[var(--surface-sunken)] px-4 py-3">
              <div className="flex items-center gap-2">
                <span className="text-[11px] font-medium uppercase tracking-[0.06em] text-[var(--muted-foreground)]">
                  Status
                </span>
                <Badge variant={viewingKey?.status === 'active' ? 'success' : 'neutral'}>
                  {viewingKey?.status}
                </Badge>
              </div>
              <div className="flex items-center gap-2 text-[13px] text-[var(--muted-foreground)]">
                <Calendar className="h-3.5 w-3.5" />
                <span className="text-[11px] font-medium uppercase tracking-[0.06em]">Created</span>
                <span className="text-[var(--foreground)]">
                  {viewingKey && formatDate(viewingKey.createdAt)}
                </span>
              </div>
              {viewingKey?.expiration && (
                <div className="flex items-center gap-2 text-[13px] text-[var(--muted-foreground)]">
                  <Calendar className="h-3.5 w-3.5" />
                  <span className="text-[11px] font-medium uppercase tracking-[0.06em]">Expires</span>
                  <span className="text-[var(--foreground)]">{formatDate(viewingKey.expiration)}</span>
                </div>
              )}
            </div>

            {/* Credentials */}
            <div className="space-y-4">
              <CredentialField
                label="Access Key ID"
                value={viewingKey?.accessKeyId || ''}
                breakAll
              />
              <CredentialField
                label="Secret Access Key"
                value={detailsSecretKey}
                breakAll
                maskable
                loading={isLoadingDetailsSecretKey}
              />
            </div>

            {/* Bucket Permissions */}
            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <label className="text-[12px] font-medium uppercase tracking-[0.06em] text-[var(--muted-foreground)]">
                  Bucket permissions
                </label>
                {viewingKey && viewingKey.permissions.length > 0 && (
                  <span className="text-[12px] text-[var(--muted-foreground)]">
                    {viewingKey.permissions.length} bucket
                    {viewingKey.permissions.length === 1 ? '' : 's'}
                  </span>
                )}
              </div>
              {viewingKey && viewingKey.permissions.length > 0 ? (
                <div className="divide-y divide-[var(--border)] overflow-hidden rounded-lg border border-[var(--border)]">
                  {viewingKey.permissions.map((perm, idx) => (
                    <div
                      key={idx}
                      className="flex items-center justify-between gap-3 px-3.5 py-2.5"
                    >
                      <div className="flex min-w-0 items-center gap-2.5">
                        <Database className="h-3.5 w-3.5 flex-shrink-0 text-[var(--muted-foreground)]" />
                        <span className="truncate font-mono text-[13px]">{perm.bucketName}</span>
                      </div>
                      <div className="flex flex-shrink-0 gap-1">
                        {perm.read && <Badge variant="neutral">Read</Badge>}
                        {perm.write && <Badge variant="neutral">Write</Badge>}
                        {perm.owner && <Badge variant="warning">Owner</Badge>}
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="rounded-lg border border-dashed border-[var(--border)] px-4 py-6 text-center">
                  <p className="text-[13px] text-[var(--muted-foreground)]">
                    No bucket permissions yet.
                  </p>
                </div>
              )}
            </div>
          </DialogBody>
          <DialogFooter>
            <Button
              variant="secondary"
              onClick={() => {
                setKeyDetailsDialogOpen(false);
                if (viewingKey) {
                  handleOpenEditPermissions(viewingKey);
                }
              }}
            >
              <Edit className="h-4 w-4" />
              Edit permissions
            </Button>
            <Button onClick={() => setKeyDetailsDialogOpen(false)}>Close</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Edit Permissions Dialog */}
      <Dialog open={editPermissionsDialogOpen} onOpenChange={setEditPermissionsDialogOpen} size="form">
        <DialogContent>
          <DialogHeader>
            <IconTile icon={<Edit />} tone="primary" size="md" />
            <div className="min-w-0 flex-1">
              <DialogTitle className="truncate">Bucket permissions · {editingKey?.name}</DialogTitle>
              <DialogDescription>
                Select a bucket, then toggle the scopes this key should have on it.
              </DialogDescription>
            </div>
          </DialogHeader>
          <DialogBody className="space-y-5">
            <div className="space-y-1.5">
              <label className="text-[13px] font-medium">Bucket</label>
              <Select value={selectedBucket} onChange={(value) => handleBucketChange(value)}>
                <SelectOption value="">Select a bucket…</SelectOption>
                {availableBuckets.map((bucket) => (
                  <SelectOption key={bucket.name} value={bucket.name}>
                    {bucket.name}
                  </SelectOption>
                ))}
              </Select>
            </div>

            {selectedBucket && (
              <>
                <div className="space-y-1.5">
                  <label className="text-[13px] font-medium">Permissions</label>
                  <div className="divide-y divide-[var(--border)] rounded-md border border-[var(--border)]">
                    {[
                      {
                        id: 'edit-permission-read',
                        label: 'Read',
                        desc: 'GetObject, HeadObject, ListObjects',
                        checked: permissionRead,
                        setChecked: setPermissionRead,
                      },
                      {
                        id: 'edit-permission-write',
                        label: 'Write',
                        desc: 'PutObject, DeleteObject',
                        checked: permissionWrite,
                        setChecked: setPermissionWrite,
                      },
                      {
                        id: 'edit-permission-owner',
                        label: 'Owner',
                        desc: 'DeleteBucket, PutBucketPolicy',
                        checked: permissionOwner,
                        setChecked: setPermissionOwner,
                      },
                    ].map((p) => (
                      <label
                        key={p.id}
                        htmlFor={p.id}
                        className="flex cursor-pointer items-start gap-3 px-3.5 py-3 transition-colors hover:bg-[var(--accent)]"
                      >
                        <Checkbox
                          id={p.id}
                          checked={p.checked}
                          className="mt-0.5"
                          onCheckedChange={(checked) => p.setChecked(checked as boolean)}
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

                {editingKey && (() => {
                  const current = editingKey.permissions.find(
                    (perm) => perm.bucketName === selectedBucket || perm.bucketId === selectedBucket,
                  );
                  const hasAny = current && (current.read || current.write || current.owner);
                  return (
                    <div className="rounded-lg border border-[var(--border)] bg-[var(--surface-sunken)] px-3.5 py-3">
                      <div className="text-[11px] font-medium uppercase tracking-[0.06em] text-[var(--muted-foreground)]">
                        Currently granted
                      </div>
                      {hasAny ? (
                        <div className="mt-1.5 flex flex-wrap gap-1.5">
                          {current!.read && <Badge variant="neutral">Read</Badge>}
                          {current!.write && <Badge variant="neutral">Write</Badge>}
                          {current!.owner && <Badge variant="warning">Owner</Badge>}
                        </div>
                      ) : (
                        <p className="mt-1 text-[12.5px] text-[var(--muted-foreground)]">
                          No permissions on this bucket yet.
                        </p>
                      )}
                    </div>
                  );
                })()}
              </>
            )}

            {editingKey && editingKey.permissions.length > 0 && (
              <div className="space-y-1.5">
                <label className="text-[12px] font-medium uppercase tracking-[0.06em] text-[var(--muted-foreground)]">
                  All bucket permissions for this key
                </label>
                <div className="max-h-48 divide-y divide-[var(--border)] overflow-y-auto rounded-md border border-[var(--border)]">
                  {editingKey.permissions.map((perm, idx) => (
                    <div key={idx} className="flex items-center justify-between gap-3 px-3.5 py-2.5">
                      <span className="truncate font-mono text-[13px]">{perm.bucketName}</span>
                      <div className="flex flex-shrink-0 gap-1">
                        {perm.read && <Badge variant="neutral">R</Badge>}
                        {perm.write && <Badge variant="neutral">W</Badge>}
                        {perm.owner && <Badge variant="warning">O</Badge>}
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </DialogBody>
          <DialogFooter>
            <Button variant="secondary" onClick={() => setEditPermissionsDialogOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleGrantBucketPermission} disabled={!selectedBucket || savingPermissions}>
              {savingPermissions && <Loader2 className="h-4 w-4 animate-spin" />}
              {savingPermissions ? 'Saving...' : 'Save permissions'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
