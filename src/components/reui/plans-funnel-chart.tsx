import { CSSProperties } from 'react';
import { Label, Pie, PieChart, Sector, type PieSectorDataItem } from 'recharts';

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import {
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from '@/components/ui/chart';

export interface PlanSlice {
  /** Code technique de la formule (free, pro, business…). */
  plan: string;
  label: string;
  value: number;
}

export interface PlansFunnelChartProps {
  title: string;
  description?: string;
  data: PlanSlice[];
  /** Formule considérée comme gratuite : exclue du calcul des clients payants. */
  freePlan?: string;
  centerLabel?: string;
}

const COULEURS = ['var(--chart-5)', 'var(--chart-3)', 'var(--chart-2)', 'var(--chart-1)'];

/**
 * Entonnoir de conversion (c-chart-22) : part payante mise en avant au centre, secteur
 * agrandi au survol — utilisée pour la répartition des pressings par formule.
 */
export function PlansFunnelChart({
  title,
  description,
  data,
  freePlan = 'free',
  centerLabel = 'Pressings payants',
}: PlansFunnelChartProps) {
  const total = data.reduce((somme, d) => somme + d.value, 0);
  const gratuits = data.find((d) => d.plan.toLowerCase() === freePlan)?.value ?? 0;
  const payants = total - gratuits;
  const taux = total > 0 ? ((payants / total) * 100).toFixed(1) : '0.0';

  const config = Object.fromEntries(
    data.map((d, index) => [d.plan, { label: d.label, color: COULEURS[index % COULEURS.length] }])
  ) satisfies ChartConfig;

  const tranches = data.map((d, index) => ({
    ...d,
    fill: COULEURS[index % COULEURS.length],
  }));

  return (
    <Card className="w-full">
      <CardHeader className="items-center pb-0">
        <CardTitle>{title}</CardTitle>
        <CardDescription>
          {description ?? `${taux} % des pressings sont sur une formule payante`}
        </CardDescription>
      </CardHeader>
      <CardContent className="flex-1 pb-0">
        <ChartContainer config={config} className="mx-auto aspect-square max-h-[300px]">
          <PieChart accessibilityLayer>
            <ChartTooltip
              content={
                <ChartTooltipContent
                  className="min-w-40 gap-2.5"
                  formatter={(value, name) => (
                    <div className="flex w-full items-center justify-between gap-2">
                      <div className="flex items-center gap-1.5">
                        <div
                          className="h-2.5 w-2.5 shrink-0 rounded-xs bg-(--color-bg)"
                          style={{ '--color-bg': `var(--color-${name})` } as CSSProperties}
                        />
                        <span className="text-muted-foreground">{config[name]?.label ?? name}</span>
                      </div>
                      <span className="text-foreground font-semibold tabular-nums">
                        {Number(value).toLocaleString('fr-FR')}
                      </span>
                    </div>
                  )}
                />
              }
            />
            <ChartLegend content={<ChartLegendContent nameKey="plan" className="-translate-y-2" />} />
            <Pie
              data={tranches}
              dataKey="value"
              nameKey="plan"
              innerRadius={55}
              cornerRadius={5}
              paddingAngle={3}
              stroke="var(--background)"
              strokeWidth={3}
              activeShape={({ outerRadius = 0, ...props }: PieSectorDataItem) => (
                <Sector {...props} outerRadius={outerRadius + 10} />
              )}
            >
              <Label
                content={({ viewBox }) => {
                  if (viewBox && 'cx' in viewBox && 'cy' in viewBox) {
                    return (
                      <text x={viewBox.cx} y={viewBox.cy} textAnchor="middle" dominantBaseline="middle">
                        <tspan
                          x={viewBox.cx}
                          y={viewBox.cy}
                          className="fill-foreground text-2xl font-bold tabular-nums"
                        >
                          {payants.toLocaleString('fr-FR')}
                        </tspan>
                        <tspan
                          x={viewBox.cx}
                          y={(viewBox.cy || 0) + 20}
                          className="fill-muted-foreground text-[11px]"
                        >
                          {centerLabel}
                        </tspan>
                      </text>
                    );
                  }
                  return null;
                }}
              />
            </Pie>
          </PieChart>
        </ChartContainer>
      </CardContent>
    </Card>
  );
}
