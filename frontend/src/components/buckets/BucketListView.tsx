import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Badge } from '@/components/ui/badge';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { FolderIcon, Globe, Loader2, MoreVertical, Search, Settings, Trash2 } from 'lucide-react';
import { formatBytes } from '@/lib/file-utils';
import { formatDate } from '@/lib/utils';
import { useBucketCan } from '@/hooks/usePermissions';
import type { Bucket } from '@/types';

interface BucketListViewProps {
  buckets: Bucket[];
  searchQuery: string;
  isLoading?: boolean;
  onSearchChange: (query: string) => void;
  onViewBucket: (bucketName: string) => void;
  onOpenSettings: (bucket: Bucket) => void;
  onDeleteBucket: (bucket: Bucket) => void;
  onWebsiteSettings: (bucket: Bucket) => void;
}

export function BucketListView({
  buckets,
  searchQuery,
  isLoading = false,
  onSearchChange,
  onViewBucket,
  onOpenSettings,
  onDeleteBucket,
  onWebsiteSettings,
}: BucketListViewProps) {
  const canBucket = useBucketCan();
  const filteredBuckets = buckets.filter((bucket) =>
    bucket.name.toLowerCase().includes(searchQuery.toLowerCase())
  );

  return (
    <div className="space-y-4 sm:space-y-6">
      {/* Toolbar */}
      <div className="relative w-full max-w-xs">
        <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
        <Input
          placeholder="Search buckets..."
          value={searchQuery}
          onChange={(e) => onSearchChange(e.target.value)}
          className="pl-8"
        />
      </div>

      {/* Buckets Table */}
      <div className="border rounded-lg overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead className="hidden sm:table-cell">Region</TableHead>
              <TableHead className="hidden md:table-cell">Objects</TableHead>
              <TableHead>Size</TableHead>
              <TableHead className="hidden lg:table-cell">Created</TableHead>
              <TableHead className="w-[50px]"></TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={6} className="text-center py-12">
                  <div className="flex items-center justify-center gap-2 text-muted-foreground">
                    <Loader2 className="h-5 w-5 animate-spin" />
                    <span>Loading buckets...</span>
                  </div>
                </TableCell>
              </TableRow>
            ) : filteredBuckets.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6} className="text-center py-12 text-muted-foreground">
                  {searchQuery ? 'No buckets found matching your search' : 'No buckets yet'}
                </TableCell>
              </TableRow>
            ) : (
              filteredBuckets.map((bucket) => (
                <TableRow
                  key={bucket.name}
                  className="cursor-pointer hover:bg-muted/50"
                  onClick={() => onViewBucket(bucket.name)}
                >
                  <TableCell className="font-medium max-w-[200px]">
                    <span className="truncate">{bucket.name}</span>
                    {bucket.websiteAccess && (
                      <Badge variant="neutral" className="text-xs ml-2">
                        <Globe className="h-3 w-3 mr-1" />
                        Website
                      </Badge>
                    )}
                  </TableCell>
                  <TableCell className="hidden sm:table-cell">
                    <Badge variant="neutral">{bucket.region || 'default'}</Badge>
                  </TableCell>
                  <TableCell className="hidden md:table-cell">{bucket.objectCount?.toLocaleString() || 0}</TableCell>
                  <TableCell>{bucket.size ? formatBytes(bucket.size) : '0 B'}</TableCell>
                  <TableCell className="hidden lg:table-cell">{formatDate(bucket.creationDate)}</TableCell>
                  <TableCell>
                    <DropdownMenu>
                      <DropdownMenuTrigger onClick={(e) => e.stopPropagation()}>
                        <Button variant="ghost" size="icon" className="-m-3 top-1 relative">
                          <MoreVertical className="h-4 w-4" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem onClick={(e) => {
                          e.stopPropagation();
                          onViewBucket(bucket.name);
                        }}>
                          <FolderIcon className="h-4 w-4" />
                          View Objects
                        </DropdownMenuItem>
                        {canBucket(bucket, 'bucket.update') && (
                          <>
                            <DropdownMenuItem onClick={(e) => {
                              e.stopPropagation();
                              onOpenSettings(bucket);
                            }}>
                              <Settings className="h-4 w-4" />
                              Settings
                            </DropdownMenuItem>
                            <DropdownMenuItem onClick={(e) => {
                              e.stopPropagation();
                              onWebsiteSettings(bucket);
                            }}>
                              <Globe className="h-4 w-4" />
                              Website Settings
                            </DropdownMenuItem>
                          </>
                        )}
                        {canBucket(bucket, 'bucket.delete') && (
                          <>
                            <DropdownMenuSeparator />
                            <DropdownMenuItem
                              className="text-destructive"
                              onClick={(e) => {
                                e.stopPropagation();
                                onDeleteBucket(bucket);
                              }}
                            >
                              <Trash2 className="h-4 w-4" />
                              Delete
                            </DropdownMenuItem>
                          </>
                        )}
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
  );
}
