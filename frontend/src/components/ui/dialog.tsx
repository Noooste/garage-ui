import * as React from 'react';
import { createPortal } from 'react-dom';
import { X } from 'lucide-react';
import { cn } from '@/lib/utils';

type DialogSize = 'standard' | 'form' | 'destructive';

interface DialogContextValue {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  size: DialogSize;
}

const DialogContext = React.createContext<DialogContextValue | undefined>(undefined);

function useDialog() {
  const ctx = React.useContext(DialogContext);
  if (!ctx) throw new Error('useDialog must be used within a Dialog');
  return ctx;
}

interface DialogProps {
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  size?: DialogSize;
  children: React.ReactNode;
}

const Dialog: React.FC<DialogProps> = ({ open = false, onOpenChange, size = 'standard', children }) => (
  <DialogContext.Provider value={{ open, onOpenChange: onOpenChange || (() => {}), size }}>
    {children}
  </DialogContext.Provider>
);

const DialogTrigger = React.forwardRef<HTMLButtonElement, React.ButtonHTMLAttributes<HTMLButtonElement>>(
  ({ onClick, ...props }, ref) => {
    const { onOpenChange } = useDialog();
    return (
      <button
        ref={ref}
        onClick={(e) => { onOpenChange(true); onClick?.(e); }}
        {...props}
      />
    );
  }
);
DialogTrigger.displayName = 'DialogTrigger';

const widthClass: Record<DialogSize, string> = {
  standard: 'max-w-[480px]',
  form: 'max-w-[600px]',
  destructive: 'max-w-[440px]',
};

const DialogOverlay: React.FC = () => {
  const { onOpenChange } = useDialog();
  return (
    <div
      className="fixed inset-0 z-50 bg-black/55 backdrop-blur-[8px]"
      onClick={() => onOpenChange(false)}
    />
  );
};

const DialogContent = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, children, ...props }, ref) => {
    const { open, onOpenChange, size } = useDialog();
    const containerRef = React.useRef<HTMLDivElement>(null);

    const onOpenChangeRef = React.useRef(onOpenChange);
    React.useEffect(() => {
      onOpenChangeRef.current = onOpenChange;
    });

    React.useEffect(() => {
      if (!open) return;
      const previouslyFocused = document.activeElement as HTMLElement | null;

      const keyHandler = (e: KeyboardEvent) => {
        if (e.key === 'Escape') {
          e.stopPropagation();
          onOpenChangeRef.current(false);
          return;
        }
        if (e.key !== 'Tab' || !containerRef.current) return;
        const focusables = containerRef.current.querySelectorAll<HTMLElement>(
          'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])'
        );
        if (focusables.length === 0) return;
        const first = focusables[0];
        const last = focusables[focusables.length - 1];
        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault();
          last.focus();
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault();
          first.focus();
        }
      };
      document.addEventListener('keydown', keyHandler);

      setTimeout(() => {
        const first = containerRef.current?.querySelector<HTMLElement>(
          'input:not([disabled]), button:not([disabled]), [tabindex]:not([tabindex="-1"])'
        );
        first?.focus();
      }, 0);

      return () => {
        document.removeEventListener('keydown', keyHandler);
        previouslyFocused?.focus?.();
      };
    }, [open]);

    if (!open) return null;

    return createPortal(
      <>
        <DialogOverlay />
        <div
          ref={containerRef}
          role="dialog"
          aria-modal="true"
          className={cn(
            'fixed left-1/2 top-1/2 z-50 w-[calc(100%-2rem)] -translate-x-1/2 -translate-y-1/2',
            widthClass[size],
          )}
        >
          <div
            ref={ref}
            className={cn(
              'relative overflow-hidden rounded-xl border border-[var(--border)]',
              'bg-[var(--card)] text-[var(--card-foreground)]',
              'shadow-[0_20px_40px_rgba(0,0,0,0.3)]',
              className,
            )}
            {...props}
          >
            {children}
            <button
              type="button"
              onClick={() => onOpenChange(false)}
              aria-label="Close"
              className="absolute right-3 top-3 inline-flex h-7 w-7 items-center justify-center rounded-md text-[var(--muted-foreground)] hover:bg-[var(--accent)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)]"
            >
              <X className="h-4 w-4" />
            </button>
          </div>
        </div>
      </>,
      document.body,
    );
  }
);
DialogContent.displayName = 'DialogContent';

const DialogHeader: React.FC<React.HTMLAttributes<HTMLDivElement>> = ({ className, ...props }) => (
  <div
    className={cn(
      'flex items-start gap-3 border-b border-[var(--border)] px-6 py-5',
      className,
    )}
    {...props}
  />
);
DialogHeader.displayName = 'DialogHeader';

const DialogBody: React.FC<React.HTMLAttributes<HTMLDivElement>> = ({ className, ...props }) => (
  <div className={cn('px-6 py-5', className)} {...props} />
);
DialogBody.displayName = 'DialogBody';

const DialogFooter: React.FC<React.HTMLAttributes<HTMLDivElement>> = ({ className, ...props }) => (
  <div
    className={cn(
      'flex justify-end gap-2 border-t border-[var(--border)] bg-[var(--surface-sunken)] px-6 py-3.5',
      className,
    )}
    {...props}
  />
);
DialogFooter.displayName = 'DialogFooter';

const DialogTitleText = React.forwardRef<HTMLHeadingElement, React.HTMLAttributes<HTMLHeadingElement>>(
  ({ className, ...props }, ref) => (
    <h2
      ref={ref}
      className={cn('text-[20px] font-semibold tracking-[-0.015em] leading-tight', className)}
      {...props}
    />
  )
);
DialogTitleText.displayName = 'DialogTitle';

const DialogDescription = React.forwardRef<HTMLParagraphElement, React.HTMLAttributes<HTMLParagraphElement>>(
  ({ className, ...props }, ref) => (
    <p
      ref={ref}
      className={cn('mt-1 text-[13.5px] leading-[1.45] text-[var(--muted-foreground)]', className)}
      {...props}
    />
  )
);
DialogDescription.displayName = 'DialogDescription';

export {
  Dialog,
  DialogTrigger,
  DialogContent,
  DialogHeader,
  DialogBody,
  DialogFooter,
  DialogTitleText as DialogTitle,
  DialogDescription,
};
