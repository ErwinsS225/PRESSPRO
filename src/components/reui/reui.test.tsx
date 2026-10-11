import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { Badge } from '@/components/reui/badge';
import { Frame, FramePanel } from '@/components/reui/frame';
import { QuickActionCard } from '@/components/reui/quick-action-card';
import { StatCard } from '@/components/reui/stat-card';
import { SubscriptionsTable } from '@/components/reui/subscriptions-table';
import { CollectionRadialChart } from '@/components/reui/collection-radial-chart';
import { DepositsBarChart } from '@/components/reui/deposits-bar-chart';
import { DepositsCarousel } from '@/components/reui/deposits-carousel';
import { OrdersPipelineChart } from '@/components/reui/orders-pipeline-chart';
import { RevenueTrendChart } from '@/components/reui/revenue-trend-chart';
import type { Order } from '@/types';

const COMMANDE: Order = {
  id: 'cmd-1',
  orderNumber: 'CMD-2026-101',
  customerName: 'Awa Koné',
  customerPhone: '+225 05 44 33 22 11',
  items: [{ id: 'it-1', serviceName: 'Chemise', quantity: 3, unitPrice: 1000, subtotal: 3000 }],
  status: 'ready',
  paymentStatus: 'partially_paid',
  totalAmount: 8500,
  paidAmount: 5000,
  receivedAt: '10/10/2026 09:00',
  dueDate: '12/10/2026 17:00',
};

describe('Badge ReUI (c-badge-4 / c-badge-9)', () => {
  it('expose les variantes pleines, atténuées et contour', () => {
    render(
      <div>
        <Badge variant="success">Actif</Badge>
        <Badge variant="success-light" size="sm">
          Formule PRO
        </Badge>
        <Badge variant="destructive-outline" size="sm">
          Suspendu
        </Badge>
        <Badge variant="info" size="lg">
          BUSINESS
        </Badge>
      </div>
    );

    expect(screen.getByText('Actif').className).toContain('bg-emerald-600');
    expect(screen.getByText('Formule PRO').className).toContain('text-emerald-700');
    expect(screen.getByText('Formule PRO').className).toContain('h-5');
    expect(screen.getByText('Suspendu').className).toContain('border-rose-300');
    expect(screen.getByText('BUSINESS').className).toContain('h-7');
  });
});

describe('Frame ReUI', () => {
  it('empile des panneaux séparés par un liseré', () => {
    render(
      <Frame>
        <FramePanel>Premier panneau</FramePanel>
        <FramePanel>Second panneau</FramePanel>
      </Frame>
    );

    const panneaux = screen.getAllByText(/panneau/);
    expect(panneaux).toHaveLength(2);
    expect(panneaux[0].closest('[data-slot="frame"]')).not.toBeNull();
    expect(panneaux[0].className).toContain('not-last:border-b');
  });
});

describe('StatCard ReUI (c-card-15)', () => {
  it('affiche la valeur, la variation et la comparaison', () => {
    render(
      <StatCard
        title="Chiffre d'affaires encaissé"
        value="53 000 FCFA"
        delta={12.5}
        positive
        comparison="Mois précédent : 43 500 FCFA"
      />
    );

    expect(screen.getByText("Chiffre d'affaires encaissé")).toBeInTheDocument();
    expect(screen.getByText('53 000 FCFA')).toBeInTheDocument();
    expect(screen.getByText('12,5 %')).toBeInTheDocument();
    expect(screen.getByText('43 500 FCFA')).toBeInTheDocument();
  });

  it('signale une baisse avec la variante destructive et masque la variation absente', () => {
    const { rerender } = render(<StatCard title="Marge" value="-182 000 FCFA" delta={-8} positive={false} />);

    const badge = screen.getByText('8 %');
    expect(badge.className).toContain('text-rose-700');

    rerender(<StatCard title="Marge" value="-182 000 FCFA" />);
    expect(screen.queryByText('8 %')).not.toBeInTheDocument();
  });
});

describe('QuickActionCard ReUI (c-card-17)', () => {
  it('déclenche le raccourci depuis le titre comme depuis le lien', () => {
    const onSelect = vi.fn();
    render(
      <QuickActionCard
        icon={<span aria-hidden="true" />}
        title="Nouveau dépôt de linge"
        description="Enregistrez un client et ses articles."
        linkLabel="Enregistrer un dépôt"
        onSelect={onSelect}
      />
    );

    fireEvent.click(screen.getByRole('button', { name: 'Nouveau dépôt de linge' }));
    fireEvent.click(screen.getByRole('button', { name: /Enregistrer un dépôt/ }));
    expect(onSelect).toHaveBeenCalledTimes(2);
  });
});

describe('SubscriptionsTable ReUI (c-table-10)', () => {
  it('rend les formules, la facturation et l’action de gestion', () => {
    const onManage = vi.fn();
    render(
      <SubscriptionsTable
        rows={[
          {
            service: 'Pressing Ivoire Cocody',
            subtitle: <span>Abidjan (Cocody)</span>,
            plan: 'BUSINESS',
            planVariant: 'info',
            billing: '75 000 FCFA / mois',
            status: 'Actif',
            statusVariant: 'success-light',
          },
          {
            service: 'Pressing de la Lagune',
            subtitle: <span>Grand-Bassam</span>,
            plan: 'FREE',
            planVariant: 'secondary',
            billing: '0 FCFA / mois',
            status: 'Suspendu',
            statusVariant: 'destructive-light',
            statusDetail: 'Impayé depuis le 01/10',
          },
        ]}
        manageLabel={(row) => (row.status === 'Actif' ? 'Suspendre' : 'Réactiver')}
        onManage={onManage}
      />
    );

    expect(screen.getByText('Pressing Ivoire Cocody')).toBeInTheDocument();
    expect(screen.getByText('75 000 FCFA / mois')).toBeInTheDocument();
    expect(screen.getByText('Abidjan (Cocody)')).toBeInTheDocument();
    expect(screen.getByText('Impayé depuis le 01/10')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Réactiver — Pressing de la Lagune' }));
    expect(onManage).toHaveBeenCalledWith(expect.objectContaining({ service: 'Pressing de la Lagune' }));
  });
});

describe('DepositsCarousel ReUI (c-carousel-10)', () => {
  it('affiche le ticket courant et permet de changer de dépôt', () => {
    const onSelect = vi.fn();
    const seconde: Order = {
      ...COMMANDE,
      id: 'cmd-2',
      orderNumber: 'CMD-2026-102',
      customerName: 'Bamba Traoré',
    };
    render(<DepositsCarousel orders={[COMMANDE, seconde]} onSelect={onSelect} />);

    expect(screen.getAllByText('CMD-2026-101').length).toBeGreaterThan(0);
    expect(screen.getByText(/Dépôt affiché/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /Awa Koné/ }));
    expect(onSelect).toHaveBeenCalledWith(COMMANDE);
  });

  it('annonce l’absence de dépôt plutôt que d’afficher un cadre vide', () => {
    render(<DepositsCarousel orders={[]} />);
    expect(screen.getByText(/Aucun dépôt récent/)).toBeInTheDocument();
  });
});

describe('RevenueTrendChart — évolution mensuelle', () => {
  it('affiche les séries, les mesures de période et le meilleur mois', () => {
    render(
      <RevenueTrendChart
        title="Encaissements des 6 derniers mois"
        data={[
          { mois: '2026-09', label: 'sept. 26', encaisse: 6000, facture: 6000, depots: 1 },
          { mois: '2026-10', label: 'oct. 26', encaisse: 8000, facture: 11500, depots: 2 },
        ]}
      />
    );

    expect(screen.getByText('Encaissements des 6 derniers mois')).toBeInTheDocument();
    expect(screen.getByText('Encaissé sur la période')).toBeInTheDocument();
    expect(screen.getByText('14 000 FCFA')).toBeInTheDocument();
    expect(screen.getByText('Moyenne mensuelle')).toBeInTheDocument();
    expect(screen.getByText('7 000 FCFA')).toBeInTheDocument();
    expect(screen.getByText('oct. 26 · 8 000 FCFA')).toBeInTheDocument();
  });

  it('annonce l’absence de date exploitable plutôt qu’une courbe vide', () => {
    render(<RevenueTrendChart title="Encaissements des 6 derniers mois" data={[]} />);

    expect(screen.getByText(/Aucune date de dépôt exploitable/)).toBeInTheDocument();
  });
});

describe('DepositsBarChart — charge quotidienne', () => {
  it('résume les dépôts, les pièces et la journée de pointe', () => {
    render(
      <DepositsBarChart
        title="Charge du comptoir"
        data={[
          { jour: '2026-10-10', label: '10/10', depots: 2, pieces: 5, encaisse: 4000 },
          { jour: '2026-10-11', label: '11/10', depots: 3, pieces: 9, encaisse: 7000 },
        ]}
      />
    );

    expect(screen.getByText('Charge du comptoir')).toBeInTheDocument();
    expect(screen.getByText('5')).toBeInTheDocument();
    expect(screen.getByText('14')).toBeInTheDocument();
    expect(screen.getByText('7.0 pièces / jour')).toBeInTheDocument();
    expect(screen.getByText('11/10 · 9 pièces')).toBeInTheDocument();
  });

  it('annonce une période sans activité', () => {
    render(<DepositsBarChart title="Charge du comptoir" data={[]} />);

    expect(screen.getByText('Aucune activité enregistrée pour le moment')).toBeInTheDocument();
    expect(screen.getByText(/Aucun dépôt enregistré/)).toBeInTheDocument();
  });
});

describe('CollectionRadialChart — taux de recouvrement', () => {
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

  it('met le pourcentage encaissé au centre et détaille les tranches', () => {
    render(<CollectionRadialChart title="Recouvrement des règlements" recouvrement={reglements} />);

    expect(screen.getByText('75,9 %')).toBeInTheDocument();
    expect(screen.getByText('recouvré')).toBeInTheDocument();
    expect(screen.getByText('Acomptes versés')).toBeInTheDocument();
    expect(screen.getByText('11 000 FCFA')).toBeInTheDocument();
    expect(screen.getByText('Relances au retrait')).toBeInTheDocument();
  });

  it('annonce l’absence de facturation plutôt qu’un anneau vide', () => {
    render(
      <CollectionRadialChart
        title="Recouvrement des règlements"
        recouvrement={{ tranches: [], encaisse: 0, facture: 0, reste: 0, taux: 0 }}
      />
    );

    expect(screen.getByText(/Aucun montant facturé/)).toBeInTheDocument();
    expect(screen.getByText('Comptes soldés')).toBeInTheDocument();
  });
});

describe('OrdersPipelineChart — entonnoir d’atelier', () => {
  const etapes = [
    { etape: 'received' as const, label: 'Dépôts réceptionnés', value: 5 },
    { etape: 'washing' as const, label: 'Passés au lavage', value: 4 },
    { etape: 'ironing' as const, label: 'Repassés et finis', value: 3 },
    { etape: 'ready' as const, label: 'Prêts au comptoir', value: 3 },
    { etape: 'delivered' as const, label: 'Livrés au client', value: 2 },
  ];

  it('affiche la conversion et le nombre de dépôts encore en atelier', () => {
    render(<OrdersPipelineChart title="Parcours du linge en atelier" data={etapes} />);

    expect(screen.getByText('Parcours du linge en atelier')).toBeInTheDocument();
    expect(screen.getByText(/Conversion réception → livraison : 40 %/)).toBeInTheDocument();
    expect(screen.getByText('40 %')).toBeInTheDocument();
    expect(screen.getByText('3 en cours d’atelier')).toBeInTheDocument();
  });

  it('signale un atelier vide sans dessiner d’entonnoir', () => {
    render(<OrdersPipelineChart title="Parcours du linge en atelier" data={[]} />);

    expect(screen.getByText(/Aucun dépôt enregistré/)).toBeInTheDocument();
    expect(screen.getByText('Atelier à jour')).toBeInTheDocument();
  });
});
