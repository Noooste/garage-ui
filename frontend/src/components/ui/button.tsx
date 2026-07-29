import * as React from 'react';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '@/lib/utils';

const buttonVariants = cva(
  [
    'inline-flex items-center justify-center gap-2 whitespace-nowrap',
    'rounded-md text-[14px] font-medium tracking-tight',
    'transition-colors ring-offset-background',
    'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)] focus-visible:ring-offset-2',
    'disabled:pointer-events-none disabled:opacity-50',
    '[&_svg]:h-3.5 [&_svg]:w-3.5 [&_svg]:shrink-0',
  ].join(' '),
  {
    variants: {
      variant: {
        primary:
          'bg-[var(--primary)] text-[var(--primary-foreground)] font-semibold hover:brightness-[1.04] cursor-pointer',
        secondary:
          'bg-transparent border border-[var(--border)] text-[var(--foreground)] hover:bg-[var(--accent)] cursor-pointer',
        ghost:
          'bg-transparent text-[var(--muted-foreground)] hover:bg-[var(--accent)] hover:text-[var(--foreground)] cursor-pointer',
        destructive:
          'bg-[var(--destructive)] text-[var(--destructive-foreground)] font-semibold hover:brightness-[1.05] cursor-pointer',
        link:
          'text-[var(--primary)] underline-offset-4 hover:underline cursor-pointer',
      },
      size: {
        sm: 'h-8 px-3',
        default: 'h-[38px] px-4',
        lg: 'h-11 px-5',
        'icon-sm': 'h-8 w-8 p-0',
        icon: 'h-[38px] w-[38px] p-0',
        'icon-lg': 'h-11 w-11 p-0',
      },
    },
    defaultVariants: {
      variant: 'primary',
      size: 'default',
    },
  }
);

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, ...props }, ref) => (
    <button className={cn(buttonVariants({ variant, size, className }))} ref={ref} {...props} />
  )
);
Button.displayName = 'Button';

export { Button };
