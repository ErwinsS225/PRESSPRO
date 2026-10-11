import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { Badge } from '@/components/reui/badge';
import { Frame, FramePanel } from '@/components/reui/frame';
import { QuickActionCard } from '@/components/reui/quick-action-card';
import { StatCard } from '@/components/reui/stat-card';
import { SubscriptionsTable } from '@/components/reui/subscriptions-table';
import { DepositsCarousel } from '@/components/reui/deposits-carousel';
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
