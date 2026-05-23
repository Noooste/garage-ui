import * as React from 'react';
import { cn } from '@/lib/utils';

export interface SwitchProps extends Omit<React.InputHTMLAttributes<HTMLInputElement>, 'onChange'> {
  checked?: boolean;
  onCheckedChange?: (checked: boolean) => void;
}

const Switch = React.forwardRef<HTMLInputElement, SwitchProps>(
  ({ className, checked, onCheckedChange, ...props }, ref) => {
    return (
      <label className="relative inline-flex cursor-pointer items-center">
        <input
          type="checkbox"
          className="sr-only peer"
          ref={ref}
          checked={checked}
          onChange={(e) => onCheckedChange?.(e.target.checked)}
          {...props}
        />
        <div
          className={cn(
            'relative h-6 w-11 rounded-full border-2 transition-colors',
            checked ? 'border-[#ff9329] bg-[#ff9329]' : 'border-[#6b7280] bg-[#6b7280]',
            'peer-focus-visible:ring-2 peer-focus-visible:ring-ring peer-focus-visible:ring-offset-2 peer-focus-visible:ring-offset-background',
            'peer-disabled:cursor-not-allowed peer-disabled:opacity-50',
            className
          )}
        >
          <span
            className={cn(
              'absolute top-0 left-0 h-5 w-5 rounded-full bg-white shadow transition-transform',
              checked ? 'translate-x-full' : 'translate-x-0'
            )}
          />
        </div>
      </label>
    );
  }
);
Switch.displayName = 'Switch';

export { Switch };
