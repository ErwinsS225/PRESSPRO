import * as React from 'react';
import * as RechartsPrimitive from 'recharts';

import { cn } from '@/lib/utils';

/** Configuration d'un graphique : libellé et couleur par série. */
export type ChartConfig = {
  [k in string]: {
    label?: React.ReactNode;
    icon?: React.ComponentType;
    color?: string;
  };
};

type ChartContextProps = { config: ChartConfig };

const ChartContext = React.createContext<ChartContextProps | null>(null);

function useChart() {
  const context = React.useContext(ChartContext);
  if (!context) throw new Error('useChart doit être utilisé dans un <ChartContainer />');
  return context;
}

/** Conteneur : expose les couleurs en variables CSS (`--color-<clé>`) et fixe la hauteur. */
function ChartContainer({
  id,
  className,
  children,
  config,
  ...props
}: React.ComponentProps<'div'> & {
  config: ChartConfig;
  children: React.ComponentProps<typeof RechartsPrimitive.ResponsiveContainer>['children'];
}) {
  const uniqueId = React.useId();
  const chartId = `chart-${id || uniqueId.replace(/:/g, '')}`;

  return (
    <ChartContext.Provider value={{ config }}>
      <div
        data-chart={chartId}
        className={cn(
          '[&_.recharts-cartesian-axis-tick_text]:fill-muted-foreground [&_.recharts-cartesian-grid_line]:stroke-border/50 [&_.recharts-curve.recharts-tooltip-cursor]:stroke-border [&_.recharts-layer]:outline-hidden [&_.recharts-sector]:outline-hidden [&_.recharts-surface]:outline-hidden flex aspect-video justify-center text-xs',
          className
        )}
        {...props}
      >
        <style
          dangerouslySetInnerHTML={{
            __html: Object.entries(config)
              .filter(([, value]) => value.color)
              .map(([key, value]) => `[data-chart="${chartId}"] { --color-${key}: ${value.color}; }`)
              .join('\n'),
          }}
        />
        <RechartsPrimitive.ResponsiveContainer>{children}</RechartsPrimitive.ResponsiveContainer>
      </div>
    </ChartContext.Provider>
  );
}

const ChartTooltip = RechartsPrimitive.Tooltip;

type TooltipPayload = {
  name?: string;
  dataKey?: string | number;
  value?: number | string;
  color?: string;
  payload?: Record<string, unknown>;
}[];

/** Infobulle : pastille de couleur + libellé de série + valeur en chiffres tabulaires. */
function ChartTooltipContent({
  active,
  payload,
  className,
  label,
  hideLabel = false,
  formatter,
}: {
  active?: boolean;
  payload?: TooltipPayload;
  className?: string;
  label?: React.ReactNode;
  hideLabel?: boolean;
  formatter?: (value: number | string, name: string) => React.ReactNode;
}) {
  const { config } = useChart();
  if (!active || !payload?.length) return null;

  return (
    <div
      className={cn(
        'bg-popover text-popover-foreground grid min-w-32 gap-1.5 rounded-lg border border-border px-2.5 py-2 text-xs shadow-lg',
        className
      )}
    >
      {!hideLabel && label ? <div className="font-medium">{label}</div> : null}
      {payload.map((item, index) => {
        const cle = String(item.dataKey ?? item.name ?? index);
        const serie = config[cle] ?? config[String(item.name ?? '')];
        return (
          <div key={cle} className="flex w-full items-center justify-between gap-2">
            {formatter ? (
              formatter(item.value ?? '', String(item.name ?? cle))
            ) : (
              <>
                <div className="flex items-center gap-1.5">
                  <div
                    className="h-2.5 w-2.5 shrink-0 rounded-xs"
                    style={{ background: item.color ?? serie?.color }}
                  />
                  <span className="text-muted-foreground">{serie?.label ?? item.name ?? cle}</span>
                </div>
                <span className="text-foreground font-semibold tabular-nums">
                  {typeof item.value === 'number' ? item.value.toLocaleString('fr-FR') : item.value}
                </span>
              </>
            )}
          </div>
        );
      })}
    </div>
  );
}

const ChartLegend = RechartsPrimitive.Legend;

/** Légende : reprend les libellés et couleurs de la configuration. */
function ChartLegendContent({
  className,
  payload,
  nameKey,
  hideIcon = false,
}: {
  className?: string;
  payload?: {
    value?: string;
    dataKey?: string | number;
    color?: string;
    payload?: Record<string, unknown>;
  }[];
  nameKey?: string;
  hideIcon?: boolean;
}) {
  const { config } = useChart();
  if (!payload?.length) return null;

  return (
    <div className={cn('flex flex-wrap items-center justify-center gap-3', className)}>
      {payload.map((item, index) => {
        const cle = String(nameKey ? (item.payload?.[nameKey] ?? item.value) : (item.dataKey ?? item.value));
        const serie = config[cle];
        return (
          <div key={`${cle}-${index}`} className="flex items-center gap-1.5">
            {!hideIcon && (
              <div
                className="h-2 w-2 shrink-0 rounded-xs"
                style={{ background: serie?.color ?? item.color }}
              />
            )}
            <span className="text-muted-foreground text-xs">{serie?.label ?? cle}</span>
          </div>
        );
      })}
    </div>
  );
}

export { ChartContainer, ChartTooltip, ChartTooltipContent, ChartLegend, ChartLegendContent, useChart };
