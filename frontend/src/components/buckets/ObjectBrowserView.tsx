import {useState} from 'react';
import {useDropzone} from 'react-dropzone';
import {Button} from '@/components/ui/button';
import {Input} from '@/components/ui/input';
import {Header} from '@/components/layout/header';
import {ObjectsTable} from './ObjectsTable';
import {CreateDirectoryDialog} from './CreateDirectoryDialog';
import {DeleteObjectDialog} from './DeleteObjectDialog';
import {ArrowLeft, ChevronRight, FolderPlus, Home, RotateCwIcon, Search, Trash, Upload} from 'lucide-react';
import {getBreadcrumbs} from '@/lib/file-utils';
import type {S3Object} from '@/types';

interface ObjectBrowserViewProps {
  bucketName: string;
  objects: S3Object[];
  currentPath: string;
  searchQuery: string;
  isLoading?: boolean;
  isTruncated?: boolean;
  nextContinuationToken?: string;
  itemsPerPage: number;
  onSearchChange: (query: string) => void;
  onNavigateToFolder: (path: string) => void;
  onBackToBuckets: () => void;
  onUploadFiles: (files: File[]) => Promise<boolean>;
  onDeleteObject: (key: string) => Promise<boolean>;
  onDeleteMultipleObjects: (keys: string[]) => Promise<boolean>;
  onCreateDirectory: (name: string) => Promise<boolean>;
  onRefresh: () => Promise<void>;
  onPageChange: (token?: string) => void;
  onItemsPerPageChange: (count: number) => void;
  isRefreshing: boolean;
  isNavigating: boolean;
  initialPageToken?: string;
  initialItemsPerPage?: number;
}

export function ObjectBrowserView({
  bucketName,
  objects,
  currentPath,
  searchQuery,
  isLoading = false,
  isTruncated = false,
  nextContinuationToken,
  itemsPerPage,
  onSearchChange,
  onNavigateToFolder,
  onBackToBuckets,
  onUploadFiles,
  onDeleteObject,
  onDeleteMultipleObjects,
  onCreateDirectory,
  onRefresh,
  onPageChange,
  onItemsPerPageChange,
  isRefreshing,
  isNavigating,
  initialPageToken,
  initialItemsPerPage,
}: ObjectBrowserViewProps) {
  const [showUploadZone, setShowUploadZone] = useState(false);
  const [deleteObjectDialogOpen, setDeleteObjectDialogOpen] = useState(false);
  const [selectedObject, setSelectedObject] = useState<S3Object | null>(null);
  const [createDirDialogOpen, setCreateDirDialogOpen] = useState(false);
  const [selectedFileKeys, setSelectedFileKeys] = useState<Set<string>>(new Set());

  const { getRootProps, getInputProps, isDragActive } = useDropzone({
    onDrop: async (acceptedFiles, _fileRejections, event) => {
      // Get files with their full paths from DataTransferItems API
      const filesWithPaths: File[] = [];

      // Type cast event to DragEvent to access dataTransfer
      const dragEvent = event as DragEvent;

      if (dragEvent.dataTransfer?.items) {
        // Use DataTransferItemList API to preserve folder structure
        const items = Array.from(dragEvent.dataTransfer.items);
        await Promise.all(items.map(async (item: DataTransferItem) => {
          if (item.kind === 'file') {
            const entry = item.webkitGetAsEntry?.();
            if (entry) {
              await traverseFileTree(entry, '', filesWithPaths);
            }
          }
        }));
      } else {
        // Fallback to standard files
        filesWithPaths.push(...acceptedFiles);
      }

      await onUploadFiles(filesWithPaths.length > 0 ? filesWithPaths : acceptedFiles);
      setShowUploadZone(false);
    },
    noClick: true,
  });

  // Helper function to traverse file/directory tree
  const traverseFileTree = async (item: any, path: string, files: File[]): Promise<void> => {
    return new Promise((resolve) => {
      if (item.isFile) {
        item.file((file: File) => {
          // Add the relative path to the file object
          const fullPath = path + file.name;
          Object.defineProperty(file, 'webkitRelativePath', {
            value: fullPath,
            writable: false
          });
          files.push(file);
          resolve();
        });
      } else if (item.isDirectory) {
        const dirReader = item.createReader();
        dirReader.readEntries(async (entries: any[]) => {
          for (const entry of entries) {
            await traverseFileTree(entry, path + item.name + '/', files);
          }
          resolve();
        });
      } else {
        resolve();
      }
    });
  };

  const handleToggleFileSelection = (key: string) => {
    const newSelected = new Set(selectedFileKeys);
    if (newSelected.has(key)) {
      newSelected.delete(key);
    } else {
      newSelected.add(key);
    }
    setSelectedFileKeys(newSelected);
  };

  const handleSelectAllFiles = () => {
    const fileKeys = objects
      .filter(obj => !obj.isFolder)
      .map(obj => obj.key);

    if (selectedFileKeys.size === fileKeys.length && fileKeys.length > 0) {
      setSelectedFileKeys(new Set());
    } else {
      setSelectedFileKeys(new Set(fileKeys));
    }
  };

  const handleBulkDeleteFiles = async () => {
    if (selectedFileKeys.size === 0) return;

    await onDeleteMultipleObjects(Array.from(selectedFileKeys));
    setSelectedFileKeys(new Set());
  };

  const handleDeleteObject = async (key: string): Promise<boolean> => {
    const success = await onDeleteObject(key);
    if (success) {
      setDeleteObjectDialogOpen(false);
      setSelectedObject(null);
    }
    return success;
  };

  const uploadFiles = async (files: File[]) => {
    await onUploadFiles(files);
    setShowUploadZone(false);
  };

  return (
    <div>
      <Header title={`Objects in ${bucketName}`} />
      <div className="p-4 sm:p-6 space-y-4 sm:space-y-6">
        {/* Back Button */}
        <Button variant="outline" onClick={onBackToBuckets} className="text-sm sm:text-base">
          <ArrowLeft className="h-4 w-4" />
          <span className="hidden sm:inline">Back to Buckets</span>
          <span className="sm:hidden">Back</span>
        </Button>

        {/* Breadcrumb Navigation */}
        <div className="flex items-center gap-2 text-xs sm:text-sm overflow-x-auto">
          <Home className="h-4 w-4 text-muted-foreground" />
          {getBreadcrumbs(currentPath).map((crumb, index) => (
            <div key={index} className="flex items-center gap-2">
              {index > 0 && <ChevronRight className="h-4 w-4 text-muted-foreground" />}
              <button
                onClick={() => onNavigateToFolder(crumb.path)}
                className={
                  index === getBreadcrumbs(currentPath).length - 1
                    ? 'font-medium'
                    : 'text-muted-foreground hover:text-foreground'
                }
              >
                {crumb.label}
              </button>
            </div>
          ))}
        </div>

        {/* Toolbar */}
        <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
          <div className="relative flex-1 max-w-full sm:max-w-xs">
            <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
            <Input
              placeholder="Search objects..."
              value={searchQuery}
              onChange={(e) => onSearchChange(e.target.value)}
              className="pl-8"
            />
          </div>
          <div className="flex items-center gap-2 flex-wrap">
            {selectedFileKeys.size > 0 && (
              <Button
                onClick={handleBulkDeleteFiles}
                title={`Delete ${selectedFileKeys.size} selected file(s)`}
                className="bg-transparent border border-red-500 text-red-500 hover:bg-red-500/5"
              >
                <Trash className="h-4 w-4" />
                Delete {selectedFileKeys.size} file{selectedFileKeys.size !== 1 ? 's' : ''}
              </Button>
            )}
            <Button variant="secondary" onClick={() => setShowUploadZone(!showUploadZone)} className="flex-1 sm:flex-initial">
              <Upload className="h-4 w-4" />
              <span className="hidden sm:inline">Upload</span>
            </Button>
            <Button onClick={() => setCreateDirDialogOpen(true)} className="flex-1 sm:flex-initial">
              <FolderPlus className="h-4 w-4" />
              <span className="hidden sm:inline">Add Directory</span>
            </Button>
            <Button variant="outline" size="icon" onClick={onRefresh} title="Refresh" disabled={isRefreshing}>
              <RotateCwIcon className={`h-4 w-4 transition-transform duration-500 ${isRefreshing ? 'animate-spin' : ''}`} />
            </Button>
          </div>
        </div>

        {/* Upload Zone */}
        {showUploadZone && (
          <div className="border rounded-lg p-6 bg-muted/30 space-y-4">
            <div className="flex gap-6">
              <div className="flex-shrink-0 flex items-center justify-center">
                <div className="w-20 h-20 bg-primary/10 rounded-lg flex items-center justify-center">
                  <svg
                    className="w-12 h-12 text-primary"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                    <polyline points="17 8 12 3 7 8" />
                    <line x1="12" y1="3" x2="12" y2="15" />
                  </svg>
                </div>
              </div>

              <div className="flex-1 space-y-3">
                <div
                  {...getRootProps()}
                  className={`border-2 border-dashed rounded-lg p-6 text-center cursor-pointer transition-colors ${
                    isDragActive
                      ? 'border-primary bg-primary/5'
                      : 'border-muted-foreground/25 hover:border-muted-foreground/50'
                  }`}
                >
                  <input {...getInputProps()} />
                  <p className="text-sm">
                    Drag and drop files/folders or{' '}
                    <label
                      htmlFor="file-input"
                      className="font-medium text-primary hover:underline cursor-pointer"
                    >
                      select files
                    </label>
                    {' / '}
                    <label
                      htmlFor="folder-input"
                      className="font-medium text-primary hover:underline cursor-pointer"
                    >
                      select folder
                    </label>
                  </p>
                  <input
                    id="file-input"
                    type="file"
                    multiple
                    onChange={(e) => {
                      if (e.target.files) {
                        const files = Array.from(e.target.files);
                        uploadFiles(files);
                        e.target.value = '';
                      }
                    }}
                    style={{ display: 'none' }}
                  />
                  <input
                    id="folder-input"
                    type="file"
                    {...({ webkitdirectory: '', directory: '', mozdirectory: '' } as any)}
                    onChange={(e) => {
                      if (e.target.files) {
                        const files = Array.from(e.target.files);
                        uploadFiles(files);
                        e.target.value = '';
                      }
                    }}
                    style={{ display: 'none' }}
                  />
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Objects Table with Drag & Drop */}
        <div
          {...getRootProps()}
          className={`relative border rounded-lg transition-all duration-200 overflow-visible ${
            isDragActive
              ? 'border-primary bg-primary/5 border-2 shadow-lg'
              : 'border-border'
          }`}
        >
          <input {...getInputProps()} />

          {/* Drag & Drop Overlay */}
          {isDragActive && (
            <div className="absolute inset-0 z-50 bg-primary/10 backdrop-blur-sm rounded-lg flex items-center justify-center pointer-events-none">
              <div className="bg-background/95 border-2 border-primary border-dashed rounded-lg p-8 shadow-xl">
                <div className="flex flex-col items-center gap-4">
                  <div className="relative">
                    <Upload className="h-16 w-16 text-primary animate-bounce" />
                    <div className="absolute inset-0 h-16 w-16 text-primary opacity-30 animate-ping">
                      <Upload className="h-16 w-16" />
                    </div>
                  </div>
                  <div className="text-center space-y-2">
                    <p className="text-lg font-semibold text-primary">Drop files here to upload</p>
                    <p className="text-sm text-muted-foreground">Files will be uploaded to {currentPath || 'root'}</p>
                  </div>
                </div>
              </div>
            </div>
          )}

          <ObjectsTable
            objects={objects}
            currentPath={currentPath}
            searchQuery={searchQuery}
            selectedFileKeys={selectedFileKeys}
            isDragActive={isDragActive}
            isLoading={isLoading && !isRefreshing && !isNavigating}
            isTruncated={isTruncated}
            nextContinuationToken={nextContinuationToken}
            itemsPerPage={itemsPerPage}
            onNavigateToFolder={onNavigateToFolder}
            onDeleteObject={(obj) => {
              setSelectedObject(obj);
              setDeleteObjectDialogOpen(true);
            }}
            onToggleFileSelection={handleToggleFileSelection}
            onSelectAllFiles={handleSelectAllFiles}
            onPageChange={onPageChange}
            onItemsPerPageChange={onItemsPerPageChange}
            initialPageToken={initialPageToken}
            initialItemsPerPage={initialItemsPerPage}
          />
        </div>
      </div>

      {/* Create Directory Dialog */}
      <CreateDirectoryDialog
        open={createDirDialogOpen}
        onOpenChange={setCreateDirDialogOpen}
        currentPath={currentPath}
        onCreateDirectory={onCreateDirectory}
      />

      {/* Delete Object Dialog */}
      <DeleteObjectDialog
        open={deleteObjectDialogOpen}
        onOpenChange={setDeleteObjectDialogOpen}
        object={selectedObject}
        onDeleteObject={handleDeleteObject}
      />
    </div>
  );
}
