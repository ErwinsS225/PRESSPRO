import type { ReactNode } from 'react';

import { Badge } from '@/components/reui/badge';

/** Couleurs des étapes d'atelier, partagées par la répartition et l'entonnoir. */
export const COULEURS_STATUTS: Record<string, string> = {
  received: 'var(--chart-4)',
  washing: 'var(--chart-2)',
  ironing: 'var(--chart-3)',
  ready: 'var(--chart-1)',
  delivered: 'var(--chart-5)',
};

/** Ligne d'infobulle : pastille de couleur, libellé de série et valeur alignée. */
export function LigneInfoBulle({
  label,
  valeur,
  couleur,
}: {
  label: ReactNode;
  valeur: ReactNode;
  couleur?: string;
}) {
  return (
    <div className="flex w-full items-center justify-between gap-2">
      <div className="flex items-center gap-1.5">
        <div className="h-2.5 w-2.5 shrink-0 rounded-xs" style={{ background: couleur }} />
        <span className="text-muted-foreground">{label}</span>
      </div>
      <span className="text-foreground font-semibold tabular-nums">{valeur}</span>
    </div>
  );
}

/** Bandeau récapitulatif sous un graphique : mesures séparées par des puces. */
export function ResumeGraphique({ children }: { children: ReactNode }) {
  return (
    <div className="border-border/60 text-muted-foreground mt-4 flex flex-wrap items-center gap-x-4 gap-y-1 border-t pt-3 text-xs">
      {children}
    </div>
  );
}

/** Mesure « libellé : valeur » du bandeau récapitulatif. */
export function Mesure({ label, valeur }: { label: string; valeur: ReactNode }) {
  return (
    <span className="flex items-center gap-1.5">
      <span>{label}</span>
      <span className="text-foreground font-mono font-medium tabular-nums">{valeur}</span>
    </span>
  );
}

/** Étiquette d'état (succès, vigilance, alerte) réutilisant les badges ReUI. */
export function EtatMesure({
  variant,
  children,
}: {
  variant: 'success' | 'warning' | 'warning-outline' | 'destructive-outline';
  children: ReactNode;
}) {
  return (
    <Badge variant={variant} size="sm">
      {children}
    </Badge>
  );
}
