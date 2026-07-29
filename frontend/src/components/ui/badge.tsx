import * as React from 'react';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '@/lib/utils';

const badgeVariants = cva(
  'inline-flex items-center rounded-md border px-2 py-0.5 text-[12px] font-medium tracking-tight',
  {
    variants: {
      variant: {
        neutral:
          'bg-[var(--card)] border-[var(--border)] text-[var(--muted-foreground)]',
        success:
          'bg-[var(--success-soft)] border-transparent text-[color:#2ca02c] dark:text-[color:#73bf69]',
        warning:
          'bg-[var(--accent-primary-soft)] border-[var(--accent-primary-border)] text-[var(--primary)]',
        danger:
          'bg-[var(--danger-soft)] border-[var(--danger-border)] text-[var(--destructive)]',
        primary:
          'bg-[var(--primary)] border-transparent text-[var(--primary-foreground)]',
      },
    },
    defaultVariants: { variant: 'neutral' },
  }
);

export interface BadgeProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof badgeVariants> {}

export function Badge({ className, variant, ...props }: BadgeProps) {
  return <div className={cn(badgeVariants({ variant }), className)} {...props} />;
}
