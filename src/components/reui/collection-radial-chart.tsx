import { PolarAngleAxis, RadialBar, RadialBarChart } from 'recharts';

import { EtatMesure, LigneInfoBulle, Mesure, ResumeGraphique } from '@/components/reui/chart-helpers';
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from '@/components/ui/card';
import { ChartContainer, ChartTooltip, ChartTooltipContent, type ChartConfig } from '@/components/ui/chart';
import type { Recouvrement } from '@/lib/analytique';

export interface CollectionRadialChartProps {
  title: string;
  description?: string;
  /** Résultat de `recouvrement(orders)` : tranches, encaissé, facturé, reste, taux. */
  recouvrement: Recouvrement;
}

const COULEURS: Record<string, string> = {
  solde: 'var(--chart-1)',
  acompte: 'var(--chart-3)',
  reste: 'var(--chart-5)',
};

const fcfa = (valeur: number) => `${valeur.toLocaleString('fr-FR')} FCFA`;

/**
 * Taux de recouvrement (modèle c-chart « radial ») : anneaux concentriques pour
 * les dépôts soldés, les acomptes versés et le reste à percevoir, avec le
 * pourcentage encaissé au centre — le comptoir sait immédiatement combien de
 * trésorerie dort encore dans les tickets non réglés.
 */
export function CollectionRadialChart({ title, description, recouvrement }: CollectionRadialChartProps) {
  const { tranches, encaisse, facture, reste, taux } = recouvrement;
  const total = tranches.reduce((somme, tranche) => somme + tranche.value, 0);

  const config = Object.fromEntries(
    tranches.map((tranche) => [tranche.etat, { label: tranche.label, color: COULEURS[tranche.etat] }])
  ) satisfies ChartConfig;

  const donnees = tranches.map((tranche) => ({
    etat: tranche.etat,
    label: tranche.label,
    value: tranche.value,
    fill: COULEURS[tranche.etat],
  }));

  return (
    <Card className="w-full">
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        <CardDescription>
          {description ?? `${taux.toLocaleString('fr-FR')} % du chiffre facturé est déjà encaissé`}
        </CardDescription>
      </CardHeader>
      <CardContent className="flex-1 pb-0">
        {facture === 0 ? (
          <p className="text-muted-foreground flex h-[230px] items-center justify-center text-center text-xs">
            Aucun montant facturé : le taux de recouvrement s’affichera avec le premier dépôt.
          </p>
        ) : (
          <div className="relative">
            <ChartContainer config={config} className="mx-auto aspect-square max-h-[230px] w-full">
              <RadialBarChart data={donnees} innerRadius="34%" outerRadius="100%" barSize={22}>
                <PolarAngleAxis
                  type="number"
                  domain={[0, Math.max(total, 1)]}
                  tick={false}
                  axisLine={false}
                />
                <ChartTooltip
                  content={
                    <ChartTooltipContent
                      hideLabel
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
                <RadialBar dataKey="value" background cornerRadius={6} />
              </RadialBarChart>
            </ChartContainer>
            <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
              <span className="text-foreground text-2xl font-bold tabular-nums">
                {taux.toLocaleString('fr-FR')} %
              </span>
              <span className="text-muted-foreground text-[11px]">recouvré</span>
            </div>
          </div>
        )}

        <ul className="mt-3 flex flex-wrap items-center justify-center gap-x-4 gap-y-1">
          {donnees.map((tranche) => (
            <li key={tranche.etat} className="flex items-center gap-1.5 text-xs">
              <span
                aria-hidden="true"
                className="h-2 w-2 shrink-0 rounded-xs"
                style={{ background: tranche.fill }}
              />
              <span className="text-muted-foreground">{tranche.label}</span>
              <span className="text-foreground font-mono font-medium tabular-nums">
                {tranche.value.toLocaleString('fr-FR')}
              </span>
            </li>
          ))}
        </ul>
      </CardContent>
      <CardFooter className="flex-col items-stretch pb-0">
        <ResumeGraphique>
          <Mesure label="Encaissé" valeur={fcfa(encaisse)} />
          <Mesure label="Facturé" valeur={fcfa(facture)} />
          <Mesure label="Reste à percevoir" valeur={fcfa(reste)} />
          {reste > 0 ? (
            <EtatMesure variant="warning">Relances au retrait</EtatMesure>
          ) : (
            <EtatMesure variant="success">Comptes soldés</EtatMesure>
          )}
        </ResumeGraphique>
      </CardFooter>
    </Card>
  );
}
