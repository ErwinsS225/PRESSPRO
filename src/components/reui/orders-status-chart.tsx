import { CSSProperties } from 'react';
import { Cell, Pie, PieChart } from 'recharts';

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import {
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from '@/components/ui/chart';

export interface StatusSlice {
  statut: string;
  label: string;
  value: number;
  /** Remplissage hachuré pour distinguer les statuts « à surveiller ». */
  hachure?: boolean;
}

export interface OrdersStatusChartProps {
  title: string;
  description: string;
  data: StatusSlice[];
}

const COULEURS: Record<string, string> = {
  received: 'var(--chart-4)',
  washing: 'var(--chart-2)',
  ironing: 'var(--chart-3)',
  ready: 'var(--chart-1)',
  delivered: 'var(--chart-5)',
};

/**
 * Répartition par statut (c-chart-21) : parts pleines ou hachurées selon l'urgence,
 * avec légende — utilisée pour suivre le linge dans l'atelier.
 */
export function OrdersStatusChart({ title, description, data }: OrdersStatusChartProps) {
  const config = Object.fromEntries(
    data.map((d) => [d.statut, { label: d.label, color: COULEURS[d.statut] ?? 'var(--chart-5)' }])
  ) satisfies ChartConfig;

  const tranches = data.map((d) => ({
    priority: d.statut,
    orders: d.value,
    fill: d.hachure ? `url(#hachure-${d.statut})` : (COULEURS[d.statut] ?? 'var(--chart-5)'),
  }));

  return (
    <Card className="w-full">
      <CardHeader className="items-center pb-0">
        <CardTitle>{title}</CardTitle>
        <CardDescription>{description}</CardDescription>
      </CardHeader>
      <CardContent className="flex-1 pb-0">
        <ChartContainer config={config} className="mx-auto aspect-square max-h-[300px]">
          <PieChart accessibilityLayer>
            <defs>
              <pattern id="hachure-ready" patternUnits="userSpaceOnUse" width="6" height="6">
                <rect width="6" height="6" fill="var(--chart-1)" opacity="0.3" />
                <path
                  d="M0,6 L6,0 M-2,2 L2,-2 M4,8 L8,4"
                  stroke="var(--chart-1)"
                  strokeWidth="1.5"
                  opacity="0.9"
                />
              </pattern>
              <pattern id="hachure-ironing" patternUnits="userSpaceOnUse" width="5" height="5">
                <rect width="5" height="5" fill="var(--chart-3)" opacity="0.2" />
                <circle cx="2.5" cy="2.5" r="1.2" fill="var(--chart-3)" opacity="0.7" />
              </pattern>
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
                        {Number(value).toLocaleString('fr-FR')}
                      </span>
                    </div>
                  )}
                />
              }
            />
            <ChartLegend content={<ChartLegendContent nameKey="priority" className="-translate-y-2" />} />
            <Pie
              data={tranches}
              dataKey="orders"
              nameKey="priority"
              innerRadius={40}
              cornerRadius={4}
              paddingAngle={3}
              stroke="var(--background)"
              strokeWidth={3}
            >
              {tranches.map((tranche) => (
                <Cell key={tranche.priority} fill={tranche.fill} />
              ))}
            </Pie>
          </PieChart>
        </ChartContainer>
      </CardContent>
    </Card>
  );
}
