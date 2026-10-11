import {
  ArrowDownIcon,
  ArrowUpIcon,
  MoreHorizontalIcon,
  PinIcon,
  SettingsIcon,
  Share2Icon,
  TrashIcon,
  TriangleAlertIcon,
} from 'lucide-react';
import * as React from 'react';

import { Badge } from '@/components/reui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Separator } from '@/components/ui/separator';

export interface StatCardProps {
  title: string;
  value: string;
  /** Variation en pourcentage (signe ignoré : `positive` décide de la couleur). */
  delta?: number;
  positive?: boolean;
  /** Comparaison affichée sous le séparateur (« Vs mois dernier : … »). */
  comparison?: string;
  actions?: {
    onSettings?: () => void;
    onAlert?: () => void;
    onPin?: () => void;
    onShare?: () => void;
    onRemove?: () => void;
  };
}

/**
 * Carte d'indicateur (c-card-15) : valeur, variation colorée, comparaison et menu
 * d'actions contextuelles — utilisée pour les KPI du tableau de bord.
 */
export function StatCard({ title, value, delta, positive = true, comparison, actions }: StatCardProps) {
  return (
    <Card className="w-full">
      <CardContent className="flex flex-col gap-4">
        <div className="flex items-center justify-between gap-3">
          <h3 className="text-muted-foreground text-sm font-medium">{title}</h3>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" className="-me-1.5 size-8" aria-label="Plus d'options">
                <MoreHorizontalIcon aria-hidden="true" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-52">
              <DropdownMenuItem onSelect={actions?.onSettings}>
                <SettingsIcon aria-hidden="true" />
                Paramètres
              </DropdownMenuItem>
              <DropdownMenuItem onSelect={actions?.onAlert}>
                <TriangleAlertIcon aria-hidden="true" />
                Ajouter une alerte
              </DropdownMenuItem>
              <DropdownMenuItem onSelect={actions?.onPin}>
                <PinIcon aria-hidden="true" />
                Épingler au tableau de bord
              </DropdownMenuItem>
              <DropdownMenuItem onSelect={actions?.onShare}>
                <Share2Icon aria-hidden="true" />
                Partager
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem variant="destructive" onSelect={actions?.onRemove}>
                <TrashIcon aria-hidden="true" />
                Retirer
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>

        <div className="space-y-2.5">
          <div className="flex items-center gap-2.5">
            <span className="text-foreground text-2xl font-medium tracking-tight tabular-nums">{value}</span>
            {typeof delta === 'number' && (
              <Badge variant={positive ? 'success-light' : 'destructive-light'}>
                {positive ? <ArrowUpIcon aria-hidden="true" /> : <ArrowDownIcon aria-hidden="true" />}
                {Math.abs(delta).toLocaleString('fr-FR')} %
              </Badge>
            )}
          </div>
          {comparison && (
            <>
              <Separator />
              <div className="text-muted-foreground text-xs">
                {comparison.split(':')[0]}&nbsp;:{' '}
                <span className="text-foreground font-medium tabular-nums">
                  {comparison.split(':').slice(1).join(':').trim()}
                </span>
              </div>
            </>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
