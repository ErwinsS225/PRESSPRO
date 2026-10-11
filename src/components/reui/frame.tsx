import * as React from 'react';

import { cn } from '@/lib/utils';

/** Frame — conteneur à panneaux empilés (séparés par un liseré). */
function Frame({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot="frame"
      className={cn(
        'bg-card text-card-foreground flex flex-col overflow-hidden rounded-xl border border-border',
        className
      )}
      {...props}
    />
  );
}

/** FramePanel — panneau d'un Frame ; se décale au survol pour marquer la profondeur. */
function FramePanel({ className, ...props }: React.ComponentProps<'div'>) {
  return (
    <div
      data-slot="frame-panel"
      className={cn(
        'bg-card not-last:border-b not-last:border-border p-4 transition-colors hover:bg-muted/50',
        className
      )}
      {...props}
    />
  );
}

export { Frame, FramePanel };
