import { CSSProperties } from 'react';
import { Cell, Label, Pie, PieChart } from 'recharts';

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { ChartContainer, ChartTooltip, ChartTooltipContent, type ChartConfig } from '@/components/ui/chart';

export interface RevenueSlice {
  /** Clé technique (sert aussi d'identifiant de dégradé). */
  source: string;
  label: string;
  value: number;
}

export interface RevenueDonutChartProps {
  title: string;
  description: string;
  data: RevenueSlice[];
  /** Unité affichée au centre et dans l'infobulle. */
  unit?: string;
  centerLabel?: string;
}

const COULEURS = ['var(--chart-1)', 'var(--chart-2)', 'var(--chart-3)', 'var(--chart-4)', 'var(--chart-5)'];

/**
 * Anneau de répartition (c-chart-19) : dégradés, ombre portée et total au centre —
 * utilisé pour la répartition du chiffre d'affaires par prestation.
 */
export function RevenueDonutChart({
  title,
  description,
  data,
  unit = 'FCFA',
  centerLabel = 'Total encaissé',
}: RevenueDonutChartProps) {
  const total = data.reduce((somme, d) => somme + d.value, 0);
  const config = Object.fromEntries(
    data.map((d, index) => [d.source, { label: d.label, color: COULEURS[index % COULEURS.length] }])
  ) satisfies ChartConfig;

  const tranches = data.map((d, index) => ({
    ...d,
    fill: `url(#degrade-${d.source})`,
    couleur: COULEURS[index % COULEURS.length],
  }));

  return (
    <Card className="w-full">
      <CardHeader className="items-center pb-0">
        <CardTitle>{title}</CardTitle>
        <CardDescription>{description}</CardDescription>
      </CardHeader>
      <CardContent className="flex-1 pb-0">
        <ChartContainer config={config} className="mx-auto aspect-square max-h-[280px]">
          <PieChart accessibilityLayer>
            <defs>
              <filter id="anneau-3d" x="-20%" y="-20%" width="140%" height="140%">
                <feDropShadow dx="0" dy="8" stdDeviation="5" floodOpacity="0.2" />
              </filter>
              {data.map((d, index) => (
                <linearGradient key={d.source} id={`degrade-${d.source}`} x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor={COULEURS[index % COULEURS.length]} stopOpacity={1} />
                  <stop offset="100%" stopColor={COULEURS[index % COULEURS.length]} stopOpacity={0.8} />
                </linearGradient>
              ))}
            </defs>
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
                        {Number(value).toLocaleString('fr-FR')} {unit}
                      </span>
                    </div>
                  )}
                />
              }
            />
            <Pie
              data={tranches}
              dataKey="value"
              nameKey="source"
              innerRadius={65}
              outerRadius={95}
              cornerRadius={8}
              paddingAngle={4}
              stroke="var(--background)"
              strokeWidth={4}
              style={{ filter: 'url(#anneau-3d)' }}
            >
              {tranches.map((tranche) => (
                <Cell key={tranche.source} fill={tranche.fill} />
              ))}
              <Label
                content={({ viewBox }) => {
                  if (viewBox && 'cx' in viewBox && 'cy' in viewBox) {
                    return (
                      <text x={viewBox.cx} y={viewBox.cy} textAnchor="middle" dominantBaseline="middle">
                        <tspan
                          x={viewBox.cx}
                          y={viewBox.cy}
                          className="fill-foreground text-xl font-bold tabular-nums"
                        >
                          {total.toLocaleString('fr-FR')}
                        </tspan>
                        <tspan
                          x={viewBox.cx}
                          y={(viewBox.cy || 0) + 20}
                          className="fill-muted-foreground text-[11px]"
                        >
                          {unit} · {centerLabel}
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
