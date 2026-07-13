import {useEffect, useMemo, useState} from 'react';
import {useNavigate} from 'react-router-dom';
import {Badge} from '@/components/ui/badge';
import {Button} from '@/components/ui/button';
import {Checkbox} from '@/components/ui/checkbox';
import {Table, TableBody, TableCell, TableHead, TableHeader, TableRow} from '@/components/ui/table';
import {Tooltip, TooltipContent, TooltipProvider, TooltipTrigger} from '@/components/ui/tooltip';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {ChevronLeft, ChevronRight, Download, Eye, FileIcon, FolderIcon, Loader2, MoreVertical, Trash2} from 'lucide-react';
import {Select, SelectOption} from '@/components/ui/select';
import {formatBytes, formatRelativeTime} from '@/lib/file-utils';
import type {S3Object} from '@/types';

interface ObjectsTableProps {
  bucketName: string;
  objects: S3Object[];
  currentPath: string;
  searchQuery: string;
  selectedFileKeys: Set<string>;
  selectedFolderKeys: Set<string>;
  isDragActive: boolean;
  isLoading?: boolean;
  isTruncated?: boolean;
  nextContinuationToken?: string;
  itemsPerPage: number;
  onNavigateToFolder: (key: string) => void;
  onDeleteObject: (object: S3Object) => void;
  onDeleteFolder: (object: S3Object) => void;
  onToggleFileSelection: (key: string) => void;
  onToggleFolderSelection: (key: string) => void;
  // Receives the keys of the currently *visible* (filtered) rows so selection
  // stays aligned with what the search is actually showing.
  onSelectAll: (fileKeys: string[], folderKeys: string[]) => void;
  onPageChange: (token?: string) => void;
  onItemsPerPageChange: (count: number) => void;
  initialPageToken?: string;
  initialItemsPerPage?: number;
}

type SortColumn = 'name' | 'size' | 'modified';
type SortDirection = 'asc' | 'desc';

export function ObjectsTable({
  bucketName,
  objects,
  currentPath,
  searchQuery,
  selectedFileKeys,
  selectedFolderKeys,
  isDragActive,
  isLoading = false,
  isTruncated = false,
  nextContinuationToken,
  itemsPerPage,
  onNavigateToFolder,
  onDeleteObject,
  onDeleteFolder,
  onToggleFileSelection,
  onToggleFolderSelection,
  onSelectAll,
  onPageChange,
  onItemsPerPageChange,
  initialPageToken,
  initialItemsPerPage,
}: ObjectsTableProps) {
  const navigate = useNavigate();
  const [sortColumn, setSortColumn] = useState<SortColumn>('name');
  const [sortDirection, setSortDirection] = useState<SortDirection>('asc');
  // Store tokens for each page: [undefined (page 1), token1 (page 2), token2 (page 3), ...]
  const [pageTokens, setPageTokens] = useState<(string | undefined)[]>([undefined]);
  const [currentPageIndex, setCurrentPageIndex] = useState(0);
  const [initialized, setInitialized] = useState(false);

  // Initialize from URL params on first load
  useEffect(() => {
    if (!initialized && initialItemsPerPage && initialItemsPerPage !== itemsPerPage) {
      onItemsPerPageChange(initialItemsPerPage);
      setInitialized(true);
    }
    if (!initialized && initialPageToken && initialPageToken !== nextContinuationToken) {
      // If we have an initial page token, trigger page change
      onPageChange(initialPageToken);
      setInitialized(true);
    }
    if (!initialized && !initialPageToken && !initialItemsPerPage) {
      setInitialized(true);
    }
  }, [initialized, initialPageToken, initialItemsPerPage, itemsPerPage, nextContinuationToken, onPageChange, onItemsPerPageChange]);

  const filteredObjects = useMemo(() => {
    const query = searchQuery.toLowerCase();
    const filtered = objects.filter((obj) => obj.key.toLowerCase().includes(query));
    return [...filtered].sort((a, b) => {
      const aIsFolder = a.isFolder ? 1 : 0;
      const bIsFolder = b.isFolder ? 1 : 0;
      if (aIsFolder !== bIsFolder) return bIsFolder - aIsFolder;

      let compareValue = 0;
      switch (sortColumn) {
        case 'name': {
          const aName = a.key.replace(currentPath, '').replace('/', '').toLowerCase();
          const bName = b.key.replace(currentPath, '').replace('/', '').toLowerCase();
          compareValue = aName.localeCompare(bName);
          break;
        }
        case 'size':
          compareValue = a.size - b.size;
          break;
        case 'modified': {
          const aDate = new Date(a.lastModified).getTime();
          const bDate = new Date(b.lastModified).getTime();
          compareValue = aDate - bDate;
          break;
        }
      }

      return sortDirection === 'asc' ? compareValue : -compareValue;
    });
  }, [objects, searchQuery, sortColumn, sortDirection, currentPath]);

  // Effect 2: Reset pagination ONLY on path navigation
  useEffect(() => {
    setPageTokens([undefined]);
    setCurrentPageIndex(0);
  }, [currentPath]);

  // Update page tokens when we get a new next token
  useEffect(() => {
    if (nextContinuationToken && isTruncated) {
      setPageTokens(prev => {
        const newTokens = [...prev];
        // Only add the token if we don't have it yet
        const nextIndex = currentPageIndex + 1;
        if (nextIndex >= newTokens.length) {
          newTokens[nextIndex] = nextContinuationToken;
        }
        return newTokens;
      });
    }
  }, [nextContinuationToken, isTruncated, currentPageIndex]);

  const hasPrevious = currentPageIndex > 0;
  const hasNext = isTruncated;

  const handleNextPage = () => {
    if (hasNext && nextContinuationToken) {
      const nextIndex = currentPageIndex + 1;
      setCurrentPageIndex(nextIndex);
      onPageChange(nextContinuationToken);
      window.scrollTo({ top: 0, behavior: 'smooth' });
    }
  };

  const handlePreviousPage = () => {
    if (hasPrevious) {
      const prevIndex = currentPageIndex - 1;
      setCurrentPageIndex(prevIndex);
      const previousToken = pageTokens[prevIndex];
      onPageChange(previousToken);
      window.scrollTo({ top: 0, behavior: 'smooth' });
    }
  };

  const handleItemsPerPageChange = (value: string) => {
    onItemsPerPageChange(Number(value));
    setPageTokens([undefined]); // Reset to first page
    setCurrentPageIndex(0);
  };

  const handleSort = (column: SortColumn) => {
    if (sortColumn === column) {
      setSortDirection(sortDirection === 'asc' ? 'desc' : 'asc');
    } else {
      setSortColumn(column);
      setSortDirection('asc');
    }
  };

  return (
    <>
      <div className="overflow-x-auto">
        <TooltipProvider>
        <Table>
          <TableHeader>
          <TableRow>
            <TableHead className="w-[50px]">
              <Checkbox
                checked={
                  filteredObjects.length > 0 &&
                  filteredObjects.every(obj =>
                    obj.isFolder ? selectedFolderKeys.has(obj.key) : selectedFileKeys.has(obj.key),
                  )
                }
                onCheckedChange={() =>
                  onSelectAll(
                    filteredObjects.filter(obj => !obj.isFolder).map(obj => obj.key),
                    filteredObjects.filter(obj => obj.isFolder).map(obj => obj.key),
                  )
                }
                aria-label="Select all objects"
              />
            </TableHead>
          <TableHead
            className="cursor-pointer hover:bg-muted/50"
            onClick={() => handleSort('name')}
          >
            Objects {sortColumn === 'name' && (sortDirection === 'asc' ? '↑' : '↓')}
          </TableHead>
          <TableHead className="hidden sm:table-cell">Type</TableHead>
          <TableHead className="hidden md:table-cell">Storage Class</TableHead>
          <TableHead
            className="cursor-pointer hover:bg-muted/50"
            onClick={() => handleSort('size')}
          >
            Size {sortColumn === 'size' && (sortDirection === 'asc' ? '↑' : '↓')}
          </TableHead>
          <TableHead
            className="cursor-pointer hover:bg-muted/50"
            onClick={() => handleSort('modified')}
          >
            Modified {sortColumn === 'modified' && (sortDirection === 'asc' ? '↑' : '↓')}
          </TableHead>
          <TableHead className="w-[50px]"></TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {isLoading ? (
          <TableRow>
            <TableCell colSpan={7} className="text-center py-12">
              <div className="flex items-center justify-center gap-2 text-muted-foreground">
                <Loader2 className="h-5 w-5 animate-spin" />
                <span>Loading objects...</span>
              </div>
            </TableCell>
          </TableRow>
        ) : filteredObjects.length === 0 ? (
          <TableRow>
            <TableCell colSpan={7} className="text-center py-12 text-muted-foreground">
              {searchQuery
                ? 'No objects found matching your search'
                : isDragActive
                ? 'Drop files or folders here'
                : 'No objects in this location'}
            </TableCell>
          </TableRow>
        ) : (
          filteredObjects.map((obj) => (
            <TableRow key={obj.key}>
              <TableCell className="w-[50px]">
                {obj.isFolder ? (
                  <Checkbox
                    checked={selectedFolderKeys.has(obj.key)}
                    onCheckedChange={() => onToggleFolderSelection(obj.key)}
                    aria-label={`Select folder ${obj.key} (deletes its contents recursively)`}
                  />
                ) : (
                  <Checkbox
                    checked={selectedFileKeys.has(obj.key)}
                    onCheckedChange={() => onToggleFileSelection(obj.key)}
                    aria-label={`Select file ${obj.key}`}
                  />
                )}
              </TableCell>
              <TableCell>
                <div className="flex items-center gap-2">
                  {obj.isFolder ? (
                    <FolderIcon className="h-4 w-4 text-muted-foreground" />
                  ) : (
                    <FileIcon className="h-4 w-4 text-muted-foreground" />
                  )}
                  {obj.isFolder ? (
                    <button
                      onClick={() => onNavigateToFolder(obj.key)}
                      className="font-medium cursor-pointer underline hover:text-primary"
                    >
                      {obj.key.replace(currentPath, '').replace('/', '')}
                    </button>
                  ) : (
                    <button
                      onClick={() => navigate(`/buckets/${bucketName}/objects/${encodeURIComponent(obj.key)}`)}
                      className="font-medium cursor-pointer hover:underline hover:text-primary"
                    >
                      {obj.key.replace(currentPath, '')}
                    </button>
                  )}
                </div>
              </TableCell>
              <TableCell className="hidden sm:table-cell">
                {obj.isFolder ? 'Directory' : (obj.contentType || 'application/octet-stream')}
              </TableCell>
              <TableCell className="hidden md:table-cell">
                {obj.storageClass && (
                  <Badge variant="neutral">{obj.storageClass}</Badge>
                )}
              </TableCell>
              <TableCell>{obj.isFolder ? null : formatBytes(obj.size)}</TableCell>
              <TableCell>
                {obj.lastModified ? (() => {
                  const d = new Date(obj.lastModified);
                  return (
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <div className="decoration-dashed decoration-1 underline underline-offset-6 cursor-pointer text-muted-foreground hover:text-foreground transition-colors">
                          {d.toLocaleDateString('en-GB', {
                            day: '2-digit',
                            month: 'short',
                            year: 'numeric',
                          })} {d.toLocaleTimeString('en-GB', {
                            hour: '2-digit',
                            minute: '2-digit',
                            second: '2-digit',
                            hour12: false,
                          })} CET
                        </div>
                      </TooltipTrigger>
                      <TooltipContent>
                        <div className="space-y-1 min-w-max">
                          <div className="flex gap-3 items-center">
                            <span className="text-sm text-gray-400 w-20 text-right">UTC</span>
                            <span className="text-sm text-white">
                              {d.toLocaleString('en-GB', {
                                day: '2-digit',
                                month: 'short',
                                year: 'numeric',
                                hour: '2-digit',
                                minute: '2-digit',
                                second: '2-digit',
                                hour12: false,
                                timeZone: 'UTC',
                              })} UTC
                            </span>
                          </div>
                          <div className="flex gap-3 items-center">
                            <span className="text-sm text-gray-400 w-20 text-right">Relative</span>
                            <span className="text-sm text-white">
                              {formatRelativeTime(d)}
                            </span>
                          </div>
                          <div className="flex gap-3 items-center">
                            <span className="text-sm text-gray-400 w-20 text-right">Timestamp</span>
                            <span className="text-sm text-white font-mono">
                              {d.toISOString()}
                            </span>
                          </div>
                        </div>
                      </TooltipContent>
                    </Tooltip>
                  );
                })() : null}
              </TableCell>
              <TableCell>
                {obj.isFolder ? (
                  <DropdownMenu>
                    <DropdownMenuTrigger>
                      <Button variant="ghost" size="icon" className="-m-6 top-1 relative">
                        <MoreVertical className="h-4 w-4" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem onClick={() => onNavigateToFolder(obj.key)}>
                        <FolderIcon className="h-4 w-4" />
                        Open
                      </DropdownMenuItem>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem
                        className="text-destructive"
                        onClick={() => onDeleteFolder(obj)}
                      >
                        <Trash2 className="h-4 w-4" />
                        Delete folder
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                ) : (
                  <DropdownMenu>
                    <DropdownMenuTrigger>
                      <Button variant="ghost" size="icon" className="-m-6 top-1 relative">
                        <MoreVertical className="h-4 w-4" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem onClick={() => navigate(`/buckets/${bucketName}/objects/${encodeURIComponent(obj.key)}`)}>
                        <Eye className="h-4 w-4" />
                        View Details
                      </DropdownMenuItem>
                      <DropdownMenuItem>
                        <Download className="h-4 w-4" />
                        Download
                      </DropdownMenuItem>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem
                        className="text-destructive"
                        onClick={() => onDeleteObject(obj)}
                      >
                        <Trash2 className="h-4 w-4" />
                        Delete
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                )}
              </TableCell>
            </TableRow>
          ))
        )}
      </TableBody>
    </Table>
        </TooltipProvider>
      </div>

    {/* Pagination Controls */}
    {(filteredObjects.length > 0 || hasPrevious) && (
      <div className="flex flex-col sm:flex-row items-center justify-between gap-4 px-4 py-4 border-t bg-background">
        {/* Items per page selector */}
        <div className="flex items-center gap-2 text-sm relative z-10">
          <span className="text-muted-foreground">Items per page:</span>
          <Select value={itemsPerPage.toString()} onChange={handleItemsPerPageChange}>
            <SelectOption value="10">10</SelectOption>
            <SelectOption value="25">25</SelectOption>
            <SelectOption value="50">50</SelectOption>
            <SelectOption value="100">100</SelectOption>
            <SelectOption value="200">200</SelectOption>
          </Select>
        </div>

        {/* Pagination info and controls */}
        <div className="flex items-center gap-4">
          <span className="text-sm text-muted-foreground">
            Page {currentPageIndex + 1} • Showing {filteredObjects.length} item{filteredObjects.length !== 1 ? 's' : ''}
          </span>

          <div className="flex items-center gap-2">
            <Button
              variant="primary"
              size="sm"
              onClick={handlePreviousPage}
              disabled={!hasPrevious}
              className="h-8"
            >
              <ChevronLeft className="h-4 w-4 mr-1" />
              Previous
            </Button>

            <Button
              variant="primary"
              size="sm"
              onClick={handleNextPage}
              disabled={!hasNext}
              className="h-8"
            >
              Next
              <ChevronRight className="h-4 w-4 ml-1" />
            </Button>
          </div>
        </div>
      </div>
    )}
    </>
  );
}
