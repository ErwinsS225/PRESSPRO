import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from 'recharts';

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
import { montantCourt, type PointJour } from '@/lib/analytique';

export interface DepositsBarChartProps {
  title: string;
  description?: string;
  /** Derniers jours d'activité, du plus ancien au plus récent (voir `serieJournaliere`). */
  data: PointJour[];
}

const config = {
  depots: { label: 'Dépôts', color: 'var(--chart-2)' },
  pieces: { label: 'Pièces de linge', color: 'var(--chart-4)' },
} satisfies ChartConfig;

/**
 * Charge quotidienne du comptoir (modèle c-chart « bar ») : nombre de dépôts et
 * volume de pièces reçues chaque jour d'activité, pour dimensionner l'équipe de
 * l'atelier et repérer les journées de pointe.
 */
export function DepositsBarChart({ title, description, data }: DepositsBarChartProps) {
  const totalDepots = data.reduce((somme, point) => somme + point.depots, 0);
  const totalPieces = data.reduce((somme, point) => somme + point.pieces, 0);
  const pointe = data.reduce<PointJour | null>(
    (sommet, point) => (sommet === null || point.pieces > sommet.pieces ? point : sommet),
    null
  );

  return (
    <Card className="w-full">
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        <CardDescription>
          {description ??
            (data.length > 0
              ? `Dépôts et pièces reçues sur les ${data.length} derniers jours d’activité`
              : 'Aucune activité enregistrée pour le moment')}
        </CardDescription>
      </CardHeader>
      <CardContent className="flex-1 pb-0">
        {data.length === 0 ? (
          <p className="text-muted-foreground flex h-[220px] items-center justify-center text-center text-xs">
            Aucun dépôt enregistré : la charge du comptoir s’affichera dès la première journée.
          </p>
        ) : (
          <ChartContainer config={config} className="h-[220px] w-full">
            <BarChart accessibilityLayer data={data} margin={{ left: 4, right: 12, top: 6 }}>
              <CartesianGrid vertical={false} />
              <XAxis dataKey="label" tickLine={false} axisLine={false} tickMargin={8} />
              <YAxis
                tickLine={false}
                axisLine={false}
                width={32}
                tickFormatter={(valeur: number) => montantCourt(valeur)}
              />
              <ChartTooltip
                cursor={false}
                content={
                  <ChartTooltipContent
                    formatter={(valeur, nom) => (
                      <LigneInfoBulle
                        label={config[nom as keyof typeof config]?.label ?? nom}
                        valeur={Number(valeur).toLocaleString('fr-FR')}
                        couleur={`var(--color-${nom})`}
                      />
                    )}
                  />
                }
              />
              <ChartLegend content={<ChartLegendContent className="pt-4" />} />
              <Bar dataKey="depots" fill="var(--color-depots)" radius={4} />
              <Bar dataKey="pieces" fill="var(--color-pieces)" radius={4} />
            </BarChart>
          </ChartContainer>
        )}
      </CardContent>
      <CardFooter className="flex-col items-stretch pb-0">
        <ResumeGraphique>
          <Mesure label="Dépôts" valeur={totalDepots} />
          <Mesure label="Pièces reçues" valeur={totalPieces} />
          <Mesure
            label="Moyenne"
            valeur={`${(data.length > 0 ? totalPieces / data.length : 0).toFixed(1)} pièces / jour`}
          />
          {pointe && pointe.pieces > 0 ? (
            <Mesure label="Journée de pointe" valeur={`${pointe.label} · ${pointe.pieces} pièces`} />
          ) : null}
        </ResumeGraphique>
      </CardFooter>
    </Card>
  );
}
