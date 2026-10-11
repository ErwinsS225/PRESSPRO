import { useId } from 'react';
import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from 'recharts';

import { LigneInfoBulle, Mesure, ResumeGraphique } from '@/components/reui/chart-helpers';
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card';
import {
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from '@/components/ui/chart';
import { montantCourt, type PointMois } from '@/lib/analytique';

export interface RevenueTrendChartProps {
  title: string;
  description?: string;
  /** Mois consécutifs, du plus ancien au plus récent (voir `serieMensuelle`). */
  data: PointMois[];
}

const config = {
  encaisse: { label: 'Encaissé', color: 'var(--chart-1)' },
  facture: { label: 'Facturé', color: 'var(--chart-2)' },
} satisfies ChartConfig;

const fcfa = (valeur: number) => `${valeur.toLocaleString('fr-FR')} FCFA`;

/**
 * Évolution mensuelle des encaissements (modèle c-chart « area » à dégradés) :
 * deux aires superposées — facturation et règlements réellement encaissés — pour
 * visualiser d'un coup d'œil l'écart entre le travail livré et l'argent perçu.
 */
export function RevenueTrendChart({ title, description, data }: RevenueTrendChartProps) {
  const prefixe = useId().replace(/:/g, '');
  const totalEncaisse = data.reduce((somme, point) => somme + point.encaisse, 0);
  const totalFacture = data.reduce((somme, point) => somme + point.facture, 0);
  const meilleur = data.reduce<PointMois | null>(
    (sommet, point) => (sommet === null || point.encaisse > sommet.encaisse ? point : sommet),
    null
  );
  const moyenne = data.length > 0 ? Math.round(totalEncaisse / data.length) : 0;

  return (
    <Card className="w-full">
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        <CardDescription>
          {description ?? 'Facturation et encaissements mois par mois, en FCFA'}
        </CardDescription>
      </CardHeader>
      <CardContent className="flex-1 pb-0">
        {data.length === 0 ? (
          <p className="text-muted-foreground flex h-[230px] items-center justify-center text-center text-xs">
            Aucune date de dépôt exploitable : la courbe s’affichera avec le premier ticket.
          </p>
        ) : (
          <ChartContainer config={config} className="h-[230px] w-full">
            <AreaChart accessibilityLayer data={data} margin={{ left: 4, right: 12, top: 6 }}>
              <defs>
                <linearGradient id={`${prefixe}-facture`} x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="var(--color-facture)" stopOpacity={0.55} />
                  <stop offset="95%" stopColor="var(--color-facture)" stopOpacity={0.05} />
                </linearGradient>
                <linearGradient id={`${prefixe}-encaisse`} x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="var(--color-encaisse)" stopOpacity={0.9} />
                  <stop offset="95%" stopColor="var(--color-encaisse)" stopOpacity={0.08} />
                </linearGradient>
              </defs>
              <CartesianGrid vertical={false} />
              <XAxis dataKey="label" tickLine={false} axisLine={false} tickMargin={8} />
              <YAxis
                tickLine={false}
                axisLine={false}
                width={44}
                tickFormatter={(valeur: number) => montantCourt(valeur)}
              />
              <ChartTooltip
                content={
                  <ChartTooltipContent
                    formatter={(valeur, nom) => (
                      <LigneInfoBulle
                        label={config[nom as keyof typeof config]?.label ?? nom}
                        valeur={fcfa(Number(valeur))}
                        couleur={`var(--color-${nom})`}
                      />
                    )}
                  />
                }
              />
              <ChartLegend content={<ChartLegendContent className="pt-4" />} />
              <Area
                dataKey="facture"
                type="monotone"
                fill={`url(#${prefixe}-facture)`}
                stroke="var(--color-facture)"
                strokeWidth={2}
              />
              <Area
                dataKey="encaisse"
                type="monotone"
                fill={`url(#${prefixe}-encaisse)`}
                stroke="var(--color-encaisse)"
                strokeWidth={2}
              />
            </AreaChart>
          </ChartContainer>
        )}
      </CardContent>
      <CardFooter className="flex-col items-stretch pb-0">
        <ResumeGraphique>
          <Mesure label="Encaissé sur la période" valeur={fcfa(totalEncaisse)} />
          <Mesure label="Facturé" valeur={fcfa(totalFacture)} />
          <Mesure label="Moyenne mensuelle" valeur={fcfa(moyenne)} />
          {meilleur && meilleur.encaisse > 0 ? (
            <Mesure label="Meilleur mois" valeur={`${meilleur.label} · ${fcfa(meilleur.encaisse)}`} />
          ) : null}
        </ResumeGraphique>
      </CardFooter>
    </Card>
  );
}
