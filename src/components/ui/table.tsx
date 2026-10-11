import * as React from 'react';

import { cn } from '@/lib/utils';

/**
 * Tableau (primitive shadcn/ui) habillé aux couleurs de l'application : en-tête
 * discret, lignes séparées par un filet et survol foncé. `Table` inclut son
 * conteneur défilant, si bien qu'une section n'a plus à gérer l'`overflow-x`.
 */
function Table({
  className,
  containerClassName,
  ...props
}: React.ComponentProps<'table'> & { containerClassName?: string }) {
  return (
    <div data-slot="table-container" className={cn('relative w-full overflow-x-auto', containerClassName)}>
      <table
        data-slot="table"
        className={cn('w-full caption-bottom border-collapse text-left text-xs', className)}
        {...props}
      />
    </div>
  );
}

function TableHeader({ className, ...props }: React.ComponentProps<'thead'>) {
  return (
    <thead
      data-slot="table-header"
      className={cn(
        'text-[11px] font-medium text-neutral-500 dark:text-neutral-400',
        'border-b border-neutral-100 bg-neutral-50/75 dark:border-neutral-800 dark:bg-neutral-900/60',
        className
      )}
      {...props}
    />
  );
}

function TableBody({ className, ...props }: React.ComponentProps<'tbody'>) {
  return (
    <tbody
      data-slot="table-body"
      className={cn(
        'font-normal divide-y divide-neutral-100 dark:divide-neutral-800',
        '[&_tr:last-child]:border-0',
        className
      )}
      {...props}
    />
  );
}

function TableFooter({ className, ...props }: React.ComponentProps<'tfoot'>) {
  return (
    <tfoot
      data-slot="table-footer"
      className={cn(
        'border-t border-neutral-200 bg-neutral-50/75 font-medium dark:border-neutral-800 dark:bg-neutral-900/60',
        className
      )}
      {...props}
    />
  );
}

function TableRow({ className, ...props }: React.ComponentProps<'tr'>) {
  return (
    <tr
      data-slot="table-row"
      className={cn('transition-colors hover:bg-neutral-900/[0.06] dark:hover:bg-white/[0.06]', className)}
      {...props}
    />
  );
}

function TableHead({ className, ...props }: React.ComponentProps<'th'>) {
  return (
    <th
      data-slot="table-head"
      className={cn('h-10 whitespace-nowrap px-4 py-2.5 text-left align-middle', className)}
      {...props}
    />
  );
}

function TableCell({ className, ...props }: React.ComponentProps<'td'>) {
  return <td data-slot="table-cell" className={cn('px-4 py-3 align-middle', className)} {...props} />;
}

function TableCaption({ className, ...props }: React.ComponentProps<'caption'>) {
  return (
    <caption
      data-slot="table-caption"
      className={cn('mt-4 text-xs text-neutral-500 dark:text-neutral-400', className)}
      {...props}
    />
  );
}

/**
 * Ligne d'état vide : occupe toute la largeur et neutralise le survol, pour que
 * l'absence de résultat soit annoncée dans le tableau plutôt que hors de lui.
 */
function TableEmpty({
  colSpan,
  className,
  children,
}: {
  colSpan: number;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <TableRow className="hover:bg-transparent dark:hover:bg-transparent">
      <TableCell
        colSpan={colSpan}
        className={cn('px-5 py-10 text-center text-neutral-500 dark:text-neutral-400', className)}
      >
        {children}
      </TableCell>
    </TableRow>
  );
}

export {
  Table,
  TableHeader,
  TableBody,
  TableFooter,
  TableHead,
  TableRow,
  TableCell,
  TableCaption,
  TableEmpty,
};
