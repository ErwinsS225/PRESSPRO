import { render } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import type { ReactElement, ReactNode } from 'react';

/**
 * jsdom ne calcule aucune mise en page : `ResponsiveContainer` mesurerait un cadre de
 * 0 px et ne dessinerait rien. Ces vérifications fixent donc le cadre à 420 × 230 px
 * (le tracé reste celui du navigateur, seule la mesure est simulée) et figent les
 * animations pour que le DOM soit stable et comparable.
 */
vi.mock('recharts', async (importOriginal) => {
  const recharts = await importOriginal<typeof import('recharts')>();
  const React = await import('react');

  const figeAnimation = (Composant: React.ElementType) =>
    function SansAnimation({ children, ...props }: { children?: ReactNode }) {
      return React.createElement(Composant, { ...props, isAnimationActive: false }, children);
    };

  return {
    ...recharts,
    ResponsiveContainer: ({ children }: { children?: ReactNode }) =>
      React.Children.map(children, (enfant) =>
        React.isValidElement(enfant)
          ? React.cloneElement(enfant as ReactElement<{ width: number; height: number }>, {
              width: 420,
              height: 230,
            })
          : enfant
      ),
    Area: figeAnimation(recharts.Area),
    Pie: figeAnimation(recharts.Pie),
    Bar: figeAnimation(recharts.Bar),
    RadialBar: figeAnimation(recharts.RadialBar),
    Funnel: figeAnimation(recharts.Funnel),
  };
});

import { CollectionRadialChart } from '@/components/reui/collection-radial-chart';
import { DepositsBarChart } from '@/components/reui/deposits-bar-chart';
import { OrdersPipelineChart } from '@/components/reui/orders-pipeline-chart';
import { OrdersStatusChart } from '@/components/reui/orders-status-chart';
import { RevenueDonutChart } from '@/components/reui/revenue-donut-chart';
import { RevenueTrendChart } from '@/components/reui/revenue-trend-chart';

const mois = [
  { mois: '2026-08', label: 'août 26', encaisse: 0, facture: 0, depots: 0 },
  { mois: '2026-09', label: 'sept. 26', encaisse: 6000, facture: 6000, depots: 1 },
  { mois: '2026-10', label: 'oct. 26', encaisse: 8000, facture: 11500, depots: 2 },
];

const journees = [
  { jour: '2026-10-10', label: '10/10', depots: 2, pieces: 5, encaisse: 4000 },
  { jour: '2026-10-11', label: '11/10', depots: 3, pieces: 9, encaisse: 7000 },
];

const reglements = {
  tranches: [
    { etat: 'solde' as const, label: 'Dépôts soldés', value: 6000 },
    { etat: 'acompte' as const, label: 'Acomptes versés', value: 5000 },
    { etat: 'reste' as const, label: 'Reste à percevoir', value: 3500 },
  ],
  encaisse: 11000,
  facture: 14500,
  reste: 3500,
  taux: 75.9,
};

const etapes = [
  { etape: 'received' as const, label: 'Dépôts réceptionnés', value: 5 },
  { etape: 'washing' as const, label: 'Passés au lavage', value: 4 },
  { etape: 'ironing' as const, label: 'Repassés et finis', value: 3 },
  { etape: 'ready' as const, label: 'Prêts au comptoir', value: 3 },
  { etape: 'delivered' as const, label: 'Livrés au client', value: 2 },
];

/** Étendue horizontale d’un tracé SVG, en unités du cadre. */
function largeurTrace(d: string): number {
  const points = [...d.matchAll(/(-?\d+(?:\.\d+)?)[ ,](-?\d+(?:\.\d+)?)/g)].map(([, x]) => Number(x));
  return points.length > 1 ? Math.max(...points) - Math.min(...points) : 0;
}

describe('dessin des graphiques (cadre 420 × 230 px)', () => {
  it('trace les deux aires mensuelles, leur dégradé et les mois sur l’axe', () => {
    const { container } = render(<RevenueTrendChart title="Évolution" data={mois} />);

    expect(container.querySelectorAll('.recharts-area')).toHaveLength(2);
    expect(container.querySelectorAll('.recharts-area-area')).toHaveLength(2);
    expect(container.querySelectorAll('.recharts-area-curve')).toHaveLength(2);
    expect(container.querySelectorAll('linearGradient')).toHaveLength(2);

    const graduations = [
      ...container.querySelectorAll('.recharts-xAxis-tick-labels .recharts-cartesian-axis-tick-value'),
    ].map((noeud) => noeud.textContent);
    expect(graduations).toEqual(['août 26', 'sept. 26', 'oct. 26']);

    // L'axe des ordonnées abrège les montants (« 3 k », « 12 k ») au format français.
    const montants = [...container.querySelectorAll('.recharts-yAxis-tick-labels text')].map(
      (noeud) => noeud.textContent
    );
    expect(montants).toEqual(['0', '3 k', '6 k', '9 k', '12 k']);
  });

  it('trace une barre par série et par jour d’activité', () => {
    const { container } = render(<DepositsBarChart title="Charge" data={journees} />);

    // Deux séries (dépôts, pièces) sur deux journées : quatre barres dessinées.
    expect(container.querySelectorAll('.recharts-bar-rectangle')).toHaveLength(4);
    expect(container.querySelectorAll('.recharts-bar-rectangles')).toHaveLength(2);

    const hauteurs = [...container.querySelectorAll('.recharts-bar-rectangle path')].map((noeud) =>
      Number(noeud.getAttribute('height'))
    );
    // Les barres « pièces » (5 puis 9) montent plus haut, la seconde journée domine.
    expect(hauteurs[1]).toBeGreaterThan(0);
    expect(hauteurs[3]).toBeGreaterThan(hauteurs[1]);
  });

  it('trace l’anneau des encaissements par prestation avec son total au centre', () => {
    const { container } = render(
      <RevenueDonutChart
        title="Encaissements"
        description="Chiffre d’affaires par prestation"
        data={[
          { source: 'chemise', label: 'Chemise', value: 9000 },
          { source: 'costume', label: 'Costume', value: 6000 },
        ]}
      />
    );

    expect(container.querySelectorAll('.recharts-pie-sector')).toHaveLength(2);
    // `toLocaleString('fr-FR')` insère une espace fine insécable : on la normalise.
    const centre = (container.querySelector('text')?.textContent ?? '').replace(/[\u202f\u00a0]/g, ' ');
    expect(centre).toContain('15 000');
    expect(centre).toContain('Total encaissé');
  });

  it('trace la répartition par statut, avec parts hachurées', () => {
    const { container } = render(
      <OrdersStatusChart
        title="Atelier"
        description="Répartition"
        data={[
          { statut: 'received', label: 'Réceptionné', value: 2 },
          { statut: 'ironing', label: 'Repassage', value: 3, hachure: true },
          { statut: 'ready', label: 'Prêt comptoir', value: 1, hachure: true },
        ]}
      />
    );

    const secteurs = [...container.querySelectorAll('.recharts-pie-sector path')];
    expect(secteurs).toHaveLength(3);
    // Les statuts à surveiller sont remplis par un motif hachuré défini en <defs>.
    expect(container.querySelectorAll('pattern[class*="hachure"], pattern[id*="hachure"]').length).toBe(2);
    expect(secteurs[1].getAttribute('fill')).toContain('hachure');
  });

  it('trace un anneau par tranche de règlement', () => {
    const { container } = render(<CollectionRadialChart title="Recouvrement" recouvrement={reglements} />);

    const secteurs = container.querySelectorAll('.recharts-radial-bar-sector');
    expect(secteurs).toHaveLength(3);
    expect(container.querySelectorAll('.recharts-radial-bar-background-sector')).toHaveLength(3);

    // Les couleurs suivent l’ordre des tranches : soldés (vert), acomptes (ambre), reste (rose).
    expect([...secteurs].map((secteur) => secteur.getAttribute('fill'))).toEqual([
      'var(--chart-1)',
      'var(--chart-3)',
      'var(--chart-5)',
    ]);
  });

  it('trace un trapèze par étape du parcours, du plus large au plus étroit', () => {
    const { container } = render(<OrdersPipelineChart title="Parcours" data={etapes} />);

    const trapezes = [...container.querySelectorAll('.recharts-funnel-trapezoid')];
    expect(trapezes).toHaveLength(5);

    const largeurs = trapezes.map((trapeze) =>
      largeurTrace(trapeze.querySelector('path')?.getAttribute('d') ?? '')
    );
    expect(largeurs.every((largeur) => largeur > 0)).toBe(true);
    expect(largeurs[0]).toBeGreaterThan(largeurs.at(-1) ?? 0);

    const etiquettes = [...container.querySelectorAll('.recharts-label-list text')].map(
      (noeud) => noeud.textContent
    );
    expect(etiquettes).toEqual(etapes.map((etape) => etape.label));
  });
});
