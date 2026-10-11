import { SettingsIcon } from 'lucide-react';
import * as React from 'react';

import { Badge, type BadgeProps } from '@/components/reui/badge';
import { Button } from '@/components/ui/button';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';

export interface SubscriptionRow {
  /** Établissement (colonne « service » du modèle : la ressource souscrite). */
  service: string;
  /** Complément affiché sous le nom (ville, propriétaire, échéance…). */
  subtitle?: React.ReactNode;
  plan: string;
  planVariant?: BadgeProps['variant'];
  billing: string;
  status: string;
  statusVariant?: BadgeProps['variant'];
  /** Précision affichée sous l'état (fin d'essai, motif de suspension…). */
  statusDetail?: React.ReactNode;
}

export interface SubscriptionsTableProps<TRow extends SubscriptionRow = SubscriptionRow> {
  rows: TRow[];
  onManage?: (row: TRow) => void;
  /** Libellé fixe ou dérivé de la ligne (« Suspendre » / « Réactiver »). */
  manageLabel?: string | ((row: TRow) => string);
  /** Valeur de `service` dont l'action est en cours (spinner/désactivation). */
  busyKey?: string | null;
}

/**
 * Tableau des abonnements (c-table-10) : ressource, formule, facturation, état et
 * action de gestion — utilisé pour les pressings souscrits (espace éditeur) et les
 * formules de l'établissement.
 */
export function SubscriptionsTable<TRow extends SubscriptionRow = SubscriptionRow>({
  rows,
  onManage,
  manageLabel = 'Gérer',
  busyKey = null,
}: SubscriptionsTableProps<TRow>) {
  const libelle = (row: TRow) => (typeof manageLabel === 'function' ? manageLabel(row) : manageLabel);

  return (
    <div className="mx-auto flex w-full flex-col">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Établissement</TableHead>
            <TableHead>Formule</TableHead>
            <TableHead>Facturation</TableHead>
            <TableHead>État</TableHead>
            <TableHead className="text-right">Actions</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row) => (
            <TableRow key={`${row.service}-${row.plan}`}>
              <TableCell>
                <div className="flex flex-col">
                  <span className="text-sm font-medium">{row.service}</span>
                  {row.subtitle && <span className="text-muted-foreground text-xs">{row.subtitle}</span>}
                </div>
              </TableCell>
              <TableCell>
                <Badge variant={row.planVariant ?? 'default'} size="sm">
                  {row.plan}
                </Badge>
              </TableCell>
              <TableCell className="text-sm tabular-nums">{row.billing}</TableCell>
              <TableCell>
                <div className="flex flex-col items-start gap-1">
                  <Badge variant={row.statusVariant ?? 'secondary'} size="sm">
                    {row.status}
                  </Badge>
                  {row.statusDetail && (
                    <span className="text-muted-foreground text-[11px]">{row.statusDetail}</span>
                  )}
                </div>
              </TableCell>
              <TableCell className="text-right">
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-7"
                  onClick={() => onManage?.(row)}
                  disabled={busyKey === row.service}
                  aria-label={`${libelle(row)} — ${row.service}`}
                >
                  <SettingsIcon className="size-3.5" aria-hidden="true" />
                  {libelle(row)}
                </Button>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}
