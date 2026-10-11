import { Funnel, FunnelChart, LabelList } from 'recharts';

import {
  COULEURS_STATUTS,
  EtatMesure,
  LigneInfoBulle,
  Mesure,
  ResumeGraphique,
} from '@/components/reui/chart-helpers';
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card';
import { ChartContainer, ChartTooltip, ChartTooltipContent, type ChartConfig } from '@/components/ui/chart';
import type { EtapePipeline } from '@/lib/analytique';

export interface OrdersPipelineChartProps {
  title: string;
  description?: string;
  /** Étapes cumulées de la réception à la livraison (voir `pipelineAtelier`). */
  data: EtapePipeline[];
}

/**
 * Entonnoir du parcours du linge (modèle c-chart « funnel ») : chaque marche
 * indique combien de dépôts ont atteint l'étape. La conversion finale
 * (livraisons / réceptions) signale les dépôts bloqués en atelier.
 */
export function OrdersPipelineChart({ title, description, data }: OrdersPipelineChartProps) {
  const recus = data[0]?.value ?? 0;
  const livres = data.at(-1)?.value ?? 0;
  const enAtelier = data.find((etape) => etape.etape === 'ready')?.value ?? 0;
  const conversion = recus > 0 ? Math.round((livres / recus) * 1000) / 10 : 0;

  const config = Object.fromEntries(
    data.map((etape) => [etape.etape, { label: etape.label, color: COULEURS_STATUTS[etape.etape] }])
  ) satisfies ChartConfig;

  const donnees = data.map((etape) => ({
    ...etape,
    fill: COULEURS_STATUTS[etape.etape] ?? 'var(--chart-5)',
  }));

  return (
    <Card className="w-full">
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        <CardDescription>
          {description ?? `Conversion réception → livraison : ${conversion.toLocaleString('fr-FR')} %`}
        </CardDescription>
      </CardHeader>
      <CardContent className="flex-1 pb-0">
        {recus === 0 ? (
          <p className="text-muted-foreground flex h-[230px] items-center justify-center text-center text-xs">
            Aucun dépôt enregistré : l’entonnoir se dessine dès la première réception au comptoir.
          </p>
        ) : (
          <ChartContainer config={config} className="h-[230px] w-full">
            <FunnelChart accessibilityLayer margin={{ left: 8, right: 96, top: 4 }}>
              <ChartTooltip
                content={
                  <ChartTooltipContent
                    hideLabel
                    formatter={(valeur, nom) => (
                      <LigneInfoBulle
                        label={config[nom as keyof typeof config]?.label ?? nom}
                        valeur={`${Number(valeur).toLocaleString('fr-FR')} dépôts`}
                        couleur={`var(--color-${nom})`}
                      />
                    )}
                  />
                }
              />
              <Funnel dataKey="value" data={donnees} nameKey="etape" isAnimationActive={false}>
                <LabelList
                  position="right"
                  dataKey="label"
                  stroke="none"
                  fontSize={11}
                  className="fill-muted-foreground"
                />
              </Funnel>
            </FunnelChart>
          </ChartContainer>
        )}
      </CardContent>
      <CardFooter className="flex-col items-stretch pb-0">
        <ResumeGraphique>
          <Mesure label="Dépôts reçus" valeur={recus} />
          <Mesure label="Arrivés au comptoir" valeur={enAtelier} />
          <Mesure label="Livrés au client" valeur={livres} />
          <Mesure label="Conversion" valeur={`${conversion.toLocaleString('fr-FR')} %`} />
          {recus > livres ? (
            <EtatMesure variant="warning-outline">{recus - livres} en cours d’atelier</EtatMesure>
          ) : (
            <EtatMesure variant="success">Atelier à jour</EtatMesure>
          )}
        </ResumeGraphique>
      </CardFooter>
    </Card>
  );
}
