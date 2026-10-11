import { cva, type VariantProps } from 'class-variance-authority';
import * as React from 'react';

import { cn } from '@/lib/utils';

/**
 * Badge ReUI — variantes pleines, atténuées (« -light ») et contour (« -outline »),
 * utilisées pour les statuts d'atelier, les formules et les états d'abonnement.
 */
const badgeVariants = cva(
  'inline-flex w-fit shrink-0 items-center justify-center gap-1 whitespace-nowrap rounded-md border font-medium transition-colors [&_svg]:pointer-events-none [&_svg]:shrink-0',
  {
    variants: {
      variant: {
        default: 'border-transparent bg-primary text-primary-foreground',
        secondary: 'border-transparent bg-secondary text-secondary-foreground',
        outline: 'border-border text-foreground',
        info: 'border-transparent bg-sky-600 text-white dark:bg-sky-500',
        success: 'border-transparent bg-emerald-600 text-white dark:bg-emerald-500',
        warning: 'border-transparent bg-amber-500 text-neutral-950',
        destructive: 'border-transparent bg-destructive text-destructive-foreground',
        'info-light':
          'border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-500/30 dark:bg-sky-500/10 dark:text-sky-300',
        'success-light':
          'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-500/30 dark:bg-emerald-500/10 dark:text-emerald-300',
        'warning-light':
          'border-amber-200 bg-amber-50 text-amber-700 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-300',
        'destructive-light':
          'border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-500/30 dark:bg-rose-500/10 dark:text-rose-300',
        'info-outline': 'border-sky-300 text-sky-700 dark:border-sky-500/40 dark:text-sky-300',
        'success-outline':
          'border-emerald-300 text-emerald-700 dark:border-emerald-500/40 dark:text-emerald-300',
        'warning-outline': 'border-amber-300 text-amber-700 dark:border-amber-500/40 dark:text-amber-300',
        'destructive-outline': 'border-rose-300 text-rose-700 dark:border-rose-500/40 dark:text-rose-300',
      },
      size: {
        sm: 'h-5 px-1.5 text-[11px] [&_svg:not([class*="size-"])]:size-3',
        default: 'h-6 px-2 text-xs [&_svg:not([class*="size-"])]:size-3.5',
        lg: 'h-7 px-2.5 text-xs [&_svg:not([class*="size-"])]:size-4',
      },
    },
    defaultVariants: { variant: 'default', size: 'default' },
  }
);

export interface BadgeProps extends React.ComponentProps<'span'>, VariantProps<typeof badgeVariants> {}

function Badge({ className, variant, size, ...props }: BadgeProps) {
  return <span data-slot="badge" className={cn(badgeVariants({ variant, size }), className)} {...props} />;
}

export { Badge, badgeVariants };
