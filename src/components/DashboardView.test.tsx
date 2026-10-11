import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { DashboardView } from './DashboardView';
import type { Order } from '../types';

const COMMANDES: Order[] = [
  {
    id: 'cmd-1',
    orderNumber: 'CMD-2026-101',
    customerName: 'Awa Koné',
    customerPhone: '+225 05 44 33 22 11',
    items: [
      { id: 'it-1', serviceName: 'Chemise', quantity: 3, unitPrice: 1000, subtotal: 3000 },
      { id: 'it-2', serviceName: 'Drap 2 places', quantity: 1, unitPrice: 2500, subtotal: 2500 },
    ],
    status: 'ready',
    paymentStatus: 'partially_paid',
    totalAmount: 8500,
    paidAmount: 5000,
    receivedAt: '10/10/2026 09:00',
    dueDate: '12/10/2026 17:00',
  },
  {
    id: 'cmd-2',
    orderNumber: 'CMD-2026-102',
    customerName: 'Bamba Traoré',
    customerPhone: '+225 07 09 88 77 66',
    items: [{ id: 'it-3', serviceName: 'Costume', quantity: 1, unitPrice: 6000, subtotal: 6000 }],
    status: 'delivered',
    paymentStatus: 'paid',
    totalAmount: 6000,
    paidAmount: 6000,
    receivedAt: '02/09/2026 08:00',
    dueDate: '05/09/2026 17:00',
  },
];

describe('DashboardView — sections ReUI', () => {
  it('branche les composants ReUI sur les données réelles du pressing', () => {
    const onOpenTicket = vi.fn();
    render(
      <DashboardView
        orders={COMMANDES}
        health={null}
        onNavigate={() => {}}
        onNewOrder={() => {}}
        onOpenTicket={onOpenTicket}
      />
    );

    // c-card-15 : indicateur avec variation calculée entre les deux mois des dépôts.
    expect(screen.getByText("Chiffre d'affaires encaissé")).toBeInTheDocument();
    expect(screen.getAllByText('11 000 FCFA').length).toBeGreaterThan(0);
    // La comparaison est découpée en deux nœuds (libellé / valeur) : on vise la valeur.
    expect(screen.getAllByText('6 000 FCFA').length).toBeGreaterThan(0);

    // c-chart-19 et c-chart-21 : répartitions par prestation et par étape d'atelier.
    expect(screen.getByText('Encaissements par prestation')).toBeInTheDocument();
    expect(screen.getByText(/Suivi de l.atelier/)).toBeInTheDocument();

    // c-carousel-10 : dépôts récents navigables.
    expect(screen.getByText('Derniers dépôts au comptoir')).toBeInTheDocument();
    expect(screen.getAllByText(/CMD-2026-101/).length).toBeGreaterThan(0);

    // Graphiques analytiques : série mensuelle, recouvrement, charge et entonnoir.
    expect(screen.getByText('Encaissements des 6 derniers mois')).toBeInTheDocument();
    expect(screen.getByText('Recouvrement des règlements')).toBeInTheDocument();
    expect(screen.getByText('75,9 %')).toBeInTheDocument(); // 11 000 encaissés sur 14 500 facturés
    expect(screen.getAllByText('11 000 FCFA').length).toBeGreaterThan(0); // KPI + anneau de recouvrement
    expect(screen.getByText('Charge du comptoir')).toBeInTheDocument();
    expect(screen.getByText('Parcours du linge en atelier')).toBeInTheDocument();
    expect(screen.getByText(/Conversion réception → livraison : 50 %/)).toBeInTheDocument();

    // Tableau des dépôts : composant partagé avec le comptoir (colonnes triables).
    expect(screen.getByRole('button', { name: 'Trier par montant' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: /statut atelier/i })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /Imprimer le ticket CMD-2026-101/ }));
    expect(onOpenTicket).toHaveBeenCalledWith(expect.objectContaining({ orderNumber: 'CMD-2026-101' }));

    // c-card-17 : raccourcis vers le comptoir, les rapports et la saisie.
    expect(screen.getByRole('button', { name: 'Nouveau dépôt de linge' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Rapports & exports' })).toBeInTheDocument();
  });

  it('route les raccourcis vers la vue demandée', () => {
    const onNavigate = vi.fn();
    render(<DashboardView orders={COMMANDES} health={null} onNavigate={onNavigate} onNewOrder={() => {}} />);

    screen.getByRole('button', { name: /Consulter les rapports/ }).click();
    expect(onNavigate).toHaveBeenCalledWith('reports');
  });
});
