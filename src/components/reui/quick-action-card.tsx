import { ChevronRightIcon } from 'lucide-react';
import * as React from 'react';

import { Card, CardContent } from '@/components/ui/card';
import { cn } from '@/lib/utils';

export interface QuickActionCardProps {
  icon: React.ReactNode;
  title: string;
  description: string;
  linkLabel: string;
  onSelect: () => void;
  className?: string;
}

/**
 * Carte de raccourci (c-card-17) : pictogramme, intitulé cliquable, description et
 * lien d'action — utilisée pour les gestes fréquents du comptoir.
 */
export function QuickActionCard({
  icon,
  title,
  description,
  linkLabel,
  onSelect,
  className,
}: QuickActionCardProps) {
  return (
    <Card className={cn('w-full', className)}>
      <CardContent className="flex flex-col gap-3">
        <div className="bg-primary rounded-md [&_svg]:text-primary-foreground flex size-11 items-center justify-center [&_svg]:size-5">
          {icon}
        </div>
        <button
          type="button"
          onClick={onSelect}
          className="text-foreground hover:text-primary block text-left text-sm leading-tight font-medium"
        >
          {title}
        </button>
        <p className="text-muted-foreground text-xs leading-relaxed">{description}</p>
        <button
          type="button"
          onClick={onSelect}
          className="text-primary inline-flex items-center gap-1 text-xs font-medium hover:underline"
        >
          {linkLabel}
          <ChevronRightIcon aria-hidden="true" className="size-2.5 shrink-0" />
        </button>
      </CardContent>
    </Card>
  );
}
