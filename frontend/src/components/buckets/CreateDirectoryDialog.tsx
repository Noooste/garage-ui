import { useState } from 'react';
import { FolderPlus } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
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
import { toast } from 'sonner';

interface CreateDirectoryDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  currentPath: string;
  onCreateDirectory: (name: string) => Promise<boolean>;
}

export function CreateDirectoryDialog({ open, onOpenChange, currentPath, onCreateDirectory }: CreateDirectoryDialogProps) {
  const [dirName, setDirName] = useState('');

  // Clear the field when the dialog closes (adjust-during-render, not an effect).
  const [wasOpen, setWasOpen] = useState(open);
  if (open !== wasOpen) {
    setWasOpen(open);
    if (!open) setDirName('');
  }

  const handleCreate = async () => {
    if (!dirName) {
      toast.error('Please enter a directory name');
      return;
    }

    const success = await onCreateDirectory(dirName);
    if (success) {
      setDirName('');
      onOpenChange(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <IconTile icon={<FolderPlus />} tone="primary" size="md" />
          <div className="flex-1">
            <DialogTitle>Create Directory</DialogTitle>
            <DialogDescription>
              Create a new directory in {currentPath || 'the root'}
            </DialogDescription>
          </div>
        </DialogHeader>
        <DialogBody className="space-y-4">
          <div className="space-y-2">
            <label className="text-sm font-medium">Directory Name</label>
            <Input
              autoFocus
              placeholder="my-directory"
              value={dirName}
              onChange={(e) => setDirName(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  handleCreate();
                }
              }}
            />
          </div>
        </DialogBody>
        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={handleCreate} disabled={!dirName}>
            Create
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
