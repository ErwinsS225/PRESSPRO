import { useCallback, useEffect, useState } from 'react';

import { Badge } from '@/components/reui/badge';
import { Carousel, CarouselContent, CarouselItem, type CarouselApi } from '@/components/ui/carousel';
import { cn } from '@/lib/utils';
import type { Order } from '@/types';

export interface DepositsCarouselProps {
  orders: Order[];
  maxSlides?: number;
  onSelect?: (order: Order) => void;
}

const STATUTS: Record<
  Order['status'],
  { label: string; variant: 'info-light' | 'warning-light' | 'success-light' | 'secondary' }
> = {
  received: { label: 'Réceptionné', variant: 'secondary' },
  washing: { label: 'En lavage', variant: 'info-light' },
  ironing: { label: 'Repassage', variant: 'warning-light' },
  ready: { label: 'Prêt comptoir', variant: 'success-light' },
  delivered: { label: 'Livré', variant: 'secondary' },
};

const FONDS = [
  'from-neutral-900 via-neutral-800 to-neutral-700',
  'from-sky-900 via-sky-800 to-neutral-800',
  'from-emerald-900 via-emerald-800 to-neutral-800',
  'from-violet-900 via-violet-800 to-neutral-800',
  'from-amber-800 via-amber-700 to-neutral-800',
];

/**
 * Carrousel des dépôts récents (c-carousel-10) : grande diapositive + bandeau de
 * vignettes cliquables — chaque diapositive résume un ticket du comptoir.
 */
export function DepositsCarousel({ orders, maxSlides = 8, onSelect }: DepositsCarouselProps) {
  const [mainApi, setMainApi] = useState<CarouselApi>();
  const [thumbApi, setThumbApi] = useState<CarouselApi>();
  const [selectedIndex, setSelectedIndex] = useState(0);

  const depots = orders.slice(0, maxSlides);

  const onThumbClick = useCallback(
    (index: number) => {
      if (!mainApi) return;
      mainApi.scrollTo(index);
    },
    [mainApi]
  );

  const onSelectSlide = useCallback(() => {
    if (!mainApi || !thumbApi) return;
    const index = mainApi.selectedScrollSnap();
    setSelectedIndex(index);
    thumbApi.scrollTo(index);
  }, [mainApi, thumbApi]);

  useEffect(() => {
    if (!mainApi) return;
    // Idem : la diapositive sélectionnée provient des événements Embla.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    onSelectSlide();
    mainApi.on('select', onSelectSlide);
    mainApi.on('reInit', onSelectSlide);
    return () => {
      mainApi.off('select', onSelectSlide);
      mainApi.off('reInit', onSelectSlide);
    };
  }, [mainApi, onSelectSlide]);

  if (depots.length === 0) {
    return (
      <div className="text-muted-foreground flex h-40 items-center justify-center rounded-xl border border-border text-xs">
        Aucun dépôt récent à afficher.
      </div>
    );
  }

  const courant = depots[selectedIndex] ?? depots[0];
  const statut = STATUTS[courant.status];

  return (
    <div className="flex w-full flex-col gap-4">
      <div className="group relative w-full overflow-hidden rounded-xl">
        {/* Diapositive principale : résumé du ticket */}
        <Carousel setApi={setMainApi} className="w-full">
          <CarouselContent className="ml-0">
            {depots.map((order, index) => {
              const etat = STATUTS[order.status];
              return (
                <CarouselItem key={order.id} className="pl-0">
                  <button
                    type="button"
                    onClick={() => onSelect?.(order)}
                    className={cn(
                      'relative flex aspect-video w-full flex-col justify-between bg-gradient-to-br p-5 text-left text-white',
                      FONDS[index % FONDS.length]
                    )}
                  >
                    <div className="flex items-start justify-between gap-3">
                      <div>
                        <p className="font-mono text-xs text-white/70">{order.orderNumber}</p>
                        <p className="mt-1 text-base font-semibold">{order.customerName}</p>
                        <p className="text-xs text-white/70">{order.customerPhone || '—'}</p>
                      </div>
                      <Badge variant={etat.variant} size="sm">
                        {etat.label}
                      </Badge>
                    </div>
                    <div className="flex items-end justify-between gap-3">
                      <div className="text-xs text-white/80">
                        <p>Reçu le {order.receivedAt || '—'}</p>
                        <p>
                          {order.items.length} article(s)
                          {order.items[0] ? ` · ${order.items[0].serviceName}` : ''}
                        </p>
                      </div>
                      <div className="text-right">
                        <p className="font-mono text-lg font-semibold tabular-nums">
                          {order.totalAmount.toLocaleString('fr-FR')} FCFA
                        </p>
                        <p className="text-xs text-white/70">
                          Payé {order.paidAmount.toLocaleString('fr-FR')} · reste{' '}
                          {Math.max(0, order.totalAmount - order.paidAmount).toLocaleString('fr-FR')}
                        </p>
                      </div>
                    </div>
                  </button>
                </CarouselItem>
              );
            })}
          </CarouselContent>
        </Carousel>

        {/* Bandeau de vignettes superposé */}
        <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/70 to-transparent p-3 pt-8">
          <div className="relative mx-auto w-full max-w-md">
            <Carousel
              setApi={setThumbApi}
              opts={{ containScroll: 'keepSnaps', dragFree: true }}
              className="w-full"
            >
              <CarouselContent className="-ml-2 flex-row">
                {depots.map((order, index) => (
                  <CarouselItem
                    key={order.id}
                    className="basis-1/4 cursor-pointer pl-2 sm:basis-1/6"
                    onClick={() => onThumbClick(index)}
                  >
                    <div
                      className={cn(
                        'relative flex aspect-square flex-col items-center justify-center rounded-md border-2 bg-neutral-900/80 text-[10px] text-white transition-all duration-300',
                        index === selectedIndex
                          ? 'border-white opacity-100 ring-2 ring-black/20'
                          : 'border-white/40 opacity-50 hover:opacity-80'
                      )}
                    >
                      <span className="font-mono font-semibold">{order.orderNumber.split('-').pop()}</span>
                      <span className="mt-0.5 text-white/70">{STATUTS[order.status].label.slice(0, 8)}</span>
                    </div>
                  </CarouselItem>
                ))}
              </CarouselContent>
            </Carousel>
          </div>
        </div>
      </div>

      <p className="text-muted-foreground text-xs">
        Dépôt affiché :{' '}
        <span className="text-foreground font-medium">
          {courant.orderNumber} — {courant.customerName}
        </span>{' '}
        <Badge variant={statut.variant} size="sm">
          {statut.label}
        </Badge>
      </p>
    </div>
  );
}
