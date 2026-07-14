import * as React from 'react';
import {createPortal} from 'react-dom';
import {cn} from '@/lib/utils';

interface DropdownMenuContextValue {
  open: boolean;
  setOpen: (open: boolean) => void;
  triggerRef: React.RefObject<HTMLButtonElement | null>;
}

const DropdownMenuContext = React.createContext<DropdownMenuContextValue | undefined>(undefined);

function useDropdownMenu() {
  const context = React.useContext(DropdownMenuContext);
  if (!context) {
    throw new Error('useDropdownMenu must be used within a DropdownMenu');
  }
  return context;
}

interface DropdownMenuProps {
  children: React.ReactNode;
}

const DropdownMenu: React.FC<DropdownMenuProps> = ({ children }) => {
  const [open, setOpen] = React.useState(false);
  const triggerRef = React.useRef<HTMLButtonElement>(null);
  return (
    <DropdownMenuContext.Provider value={{ open, setOpen, triggerRef }}>
      <div className="relative inline-block text-left">{children}</div>
    </DropdownMenuContext.Provider>
  );
};

const DropdownMenuTrigger = React.forwardRef<
  HTMLButtonElement,
  React.ButtonHTMLAttributes<HTMLButtonElement>
>(({ onClick, ...props }, ref) => {
  const { open, setOpen, triggerRef } = useDropdownMenu();

  // Merge the forwarded ref with the context triggerRef
  React.useImperativeHandle(ref, () => triggerRef.current as HTMLButtonElement);

  return (
    <button
      ref={triggerRef}
      onClick={(e) => {
        setOpen(!open);
        onClick?.(e);
      }}
      {...props}
    />
  );
});
DropdownMenuTrigger.displayName = 'DropdownMenuTrigger';

interface DropdownMenuContentProps extends React.HTMLAttributes<HTMLDivElement> {
  align?: 'start' | 'end' | 'center';
}

const DropdownMenuContent = React.forwardRef<HTMLDivElement, DropdownMenuContentProps>(
  ({ className, children, align = 'start', ...props }) => {
    const { open, setOpen, triggerRef } = useDropdownMenu();
    const contentRef = React.useRef<HTMLDivElement>(null);
    const [position, setPosition] = React.useState({ top: 0, left: 0 });

    // Calculate position based on trigger element
    React.useEffect(() => {
      const updatePosition = () => {
        if (open && triggerRef.current) {
          const rect = triggerRef.current.getBoundingClientRect();
          const scrollY = window.scrollY || document.documentElement.scrollTop;
          const scrollX = window.scrollX || document.documentElement.scrollLeft;

          let left = rect.left + scrollX;
          const top = rect.bottom + scrollY + 8; // 8px gap (mt-2)

          // Adjust horizontal alignment
          if (align === 'end') {
            left = rect.right + scrollX - 224; // 224px = w-56
          } else if (align === 'center') {
            left = rect.left + scrollX + (rect.width / 2) - 112; // 112px = half of w-56
          }

          setPosition({ top, left });
        }
      };

      updatePosition();

      if (open) {
        window.addEventListener('scroll', updatePosition, true);
        window.addEventListener('resize', updatePosition);
      }

      return () => {
        window.removeEventListener('scroll', updatePosition, true);
        window.removeEventListener('resize', updatePosition);
      };
    }, [open, align, triggerRef]);

    React.useEffect(() => {
      const handleClickOutside = (event: MouseEvent) => {
        const isClickOnTrigger = triggerRef.current?.contains(event.target as Node);
        const isClickOnContent = contentRef.current?.contains(event.target as Node);

        if (!isClickOnContent && !isClickOnTrigger) {
          setOpen(false);
        }
      };

      if (open) {
        document.addEventListener('mousedown', handleClickOutside);
      }

      return () => {
        document.removeEventListener('mousedown', handleClickOutside);
      };
    }, [open, setOpen, triggerRef]);

    if (!open) return null;

    const content = (
      <div
        ref={contentRef}
        style={{
          backgroundColor: 'var(--popover)',
          position: 'fixed',
          top: `${position.top}px`,
          left: `${position.left}px`,
        }}
        className={cn(
          'z-50 w-56 origin-top-right rounded-md text-popover-foreground shadow-lg ring-1 ring-border border border-border focus:outline-none',
          className
        )}
        {...props}
      >
        <div className="py-1">{children}</div>
      </div>
    );

    return createPortal(content, document.body);
  }
);
DropdownMenuContent.displayName = 'DropdownMenuContent';

const DropdownMenuItem = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, onClick, ...props }, ref) => {
    const { setOpen } = useDropdownMenu();
    return (
      <div
        ref={ref}
        className={cn(
          'relative flex cursor-pointer select-none items-center gap-2 rounded-sm px-2 py-1.5 text-sm outline-none transition-colors hover:bg-accent hover:text-accent-foreground focus:bg-accent focus:text-accent-foreground data-[disabled]:pointer-events-none [&_svg]:h-4 [&_svg]:w-4 [&_svg]:shrink-0',
          className
        )}
        onClick={(e) => {
          onClick?.(e);
          setOpen(false);
        }}
        {...props}
      />
    );
  }
);
DropdownMenuItem.displayName = 'DropdownMenuItem';

const DropdownMenuSeparator = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div ref={ref} className={cn('-mx-1 my-1 h-px bg-muted', className)} {...props} />
  )
);
DropdownMenuSeparator.displayName = 'DropdownMenuSeparator';

export {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
};
