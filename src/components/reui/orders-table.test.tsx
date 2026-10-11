import { fireEvent, render, screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { OrdersTable } from '@/components/reui/orders-table';
import type { Order } from '../../types';

function commande(partiel: Partial<Order> & Pick<Order, 'orderNumber' | 'customerName'>): Order {
  return {
    id: partiel.orderNumber,
    customerPhone: '+225 07 00 00 00 00',
    items: [{ id: 'it-1', serviceName: 'Chemise', quantity: 2, unitPrice: 1000, subtotal: 2000 }],
    status: 'received',
    paymentStatus: 'unpaid',
    totalAmount: 2000,
    paidAmount: 0,
    receivedAt: '11/10/2026 09:30',
    dueDate: '12/10/2026 17:00',
    ...partiel,
  };
}

const DEPOTS: Order[] = [
  commande({
    orderNumber: 'CMD-2026-101',
    customerName: 'Awa Koné',
    items: [{ id: 'it-1', serviceName: 'Chemise', quantity: 3, unitPrice: 1000, subtotal: 3000 }],
    status: 'ready',
    paymentStatus: 'partially_paid',
    totalAmount: 8500,
    paidAmount: 5000,
    dueDate: '12/10/2026 17:00',
  }),
  commande({
    orderNumber: 'CMD-2026-102',
    customerName: 'Bamba Traoré',
    items: [{ id: 'it-2', serviceName: 'Costume', quantity: 1, unitPrice: 6000, subtotal: 6000 }],
    status: 'delivered',
    paymentStatus: 'paid',
    totalAmount: 6000,
    paidAmount: 6000,
    dueDate: '05/09/2026 17:00',
  }),
  commande({
    orderNumber: 'CMD-2026-103',
    customerName: 'Élodie N’Guessan',
    items: [{ id: 'it-3', serviceName: 'Drap 2 places', quantity: 4, unitPrice: 2500, subtotal: 10000 }],
    status: 'washing',
    paymentStatus: 'unpaid',
    totalAmount: 10000,
    paidAmount: 0,
    dueDate: '14/10/2026 12:00',
  }),
];

/** Lignes du corps du tableau (hors en-tête et pied). */
function lignes(): HTMLElement[] {
  const corps = document.querySelector('[data-slot="table-body"]');
  return within(corps as HTMLElement)
    .getAllByRole('row')
    .filter((ligne) => ligne.getAttribute('data-slot') !== undefined || true);
}

describe('OrdersTable — lecture du comptoir', () => {
  it('affiche les colonnes demandées et les badges d’étape et de règlement', () => {
    render(<OrdersTable orders={DEPOTS} columns={['ticket', 'client', 'statut', 'reglement', 'montant']} />);

    expect(screen.getByText('N° ticket')).toBeInTheDocument();
    expect(screen.getByText('Statut atelier')).toBeInTheDocument();
    expect(screen.queryByText('Actions')).not.toBeInTheDocument();

    expect(screen.getByText('4. Prêt au comptoir')).toBeInTheDocument();
    expect(screen.getByText('Acompte versé')).toBeInTheDocument();
    expect(screen.getByText('Reste 3 500 F')).toBeInTheDocument();
    expect(screen.getByText('À encaisser')).toBeInTheDocument();
  });

  it('résume la sélection dans le pied de tableau', () => {
    render(
      <OrdersTable
        orders={DEPOTS}
        columns={['ticket', 'articles', 'statut', 'reglement', 'montant', 'actions']}
      />
    );

    const pied = document.querySelector('[data-slot="table-footer"]') as HTMLElement;
    expect(within(pied).getByText('3 dépôts')).toBeInTheDocument();
    expect(within(pied).getByText('8 pièces')).toBeInTheDocument();
    expect(within(pied).getByText('2 en atelier')).toBeInTheDocument();
    expect(within(pied).getByText('1 soldé')).toBeInTheDocument();
    expect(within(pied).getByText('24 500 F')).toBeInTheDocument();
    expect(within(pied).getByText('11 000 F encaissés')).toBeInTheDocument();
  });

  it('trie les lignes par colonne, dans les deux sens', () => {
    render(<OrdersTable orders={DEPOTS} columns={['ticket', 'client', 'montant']} />);

    // `toLocaleString('fr-FR')` sépare les milliers par une espace fine insécable.
    const montants = () =>
      lignes().map((ligne) =>
        within(ligne)
          .getAllByRole('cell')[2]
          .textContent?.replace(/[\u202f\u00a0]/g, ' ')
          .trim()
      );

    expect(montants()).toEqual(['8 500 F', '6 000 F', '10 000 F']);

    fireEvent.click(screen.getByRole('button', { name: 'Trier par montant' }));
    expect(montants()).toEqual(['6 000 F', '8 500 F', '10 000 F']);
    expect(screen.getByRole('columnheader', { name: /montant/i })).toHaveAttribute('aria-sort', 'ascending');

    fireEvent.click(screen.getByRole('button', { name: 'Trier par montant' }));
    expect(montants()).toEqual(['10 000 F', '8 500 F', '6 000 F']);
    expect(screen.getByRole('columnheader', { name: /montant/i })).toHaveAttribute('aria-sort', 'descending');
  });

  it('filtre par étape d’atelier avec les compteurs du bandeau', () => {
    render(<OrdersTable orders={DEPOTS} columns={['ticket', 'client']} toolbar />);

    const ongletLavage = screen.getByRole('button', { name: /Lavage/ });
    expect(ongletLavage).toHaveTextContent('1');

    fireEvent.click(ongletLavage);
    expect(lignes()).toHaveLength(1);
    expect(screen.getByText('CMD-2026-103')).toBeInTheDocument();
    expect(screen.queryByText('CMD-2026-101')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Tous/ })).toHaveTextContent('3');
  });

  it('recherche par ticket, client, téléphone ou article, sans tenir compte des accents', () => {
    render(<OrdersTable orders={DEPOTS} columns={['ticket', 'client']} toolbar />);
    const champ = screen.getByRole('searchbox', { name: 'Rechercher un dépôt' });

    fireEvent.change(champ, { target: { value: 'elodie' } });
    expect(screen.getByText('Élodie N’Guessan')).toBeInTheDocument();
    expect(lignes()).toHaveLength(1);

    fireEvent.change(champ, { target: { value: 'costume' } });
    expect(screen.getByText('CMD-2026-102')).toBeInTheDocument();
    expect(lignes()).toHaveLength(1);

    fireEvent.change(champ, { target: { value: 'introuvable' } });
    expect(lignes()).toHaveLength(1); // la ligne d'état vide occupe le tableau
    expect(screen.getByText('Aucun dépôt ne correspond aux critères.')).toBeInTheDocument();
  });

  it('affiche les dépôts par paliers', () => {
    render(<OrdersTable orders={DEPOTS} columns={['ticket', 'client']} pageSize={2} />);

    expect(lignes()).toHaveLength(2);
    expect(screen.getByText('2 sur 3 dépôts affichés')).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /Afficher plus/ }));
    expect(lignes()).toHaveLength(3);
    expect(screen.queryByRole('button', { name: /Afficher plus/ })).not.toBeInTheDocument();
  });
});

describe('OrdersTable — actions de comptoir', () => {
  it('imprime, notifie et fait avancer un dépôt depuis sa ligne', () => {
    const onOpenTicket = vi.fn();
    const onNotify = vi.fn();
    const onAdvance = vi.fn();

    render(
      <OrdersTable
        orders={[DEPOTS[0]]}
        columns={['ticket', 'client', 'statut', 'actions']}
        onOpenTicket={onOpenTicket}
        onNotify={onNotify}
        onAdvance={onAdvance}
      />
    );

    fireEvent.click(screen.getByRole('button', { name: /Notifier Awa Koné/ }));
    expect(onNotify).toHaveBeenCalledWith(expect.objectContaining({ orderNumber: 'CMD-2026-101' }));

    fireEvent.click(screen.getByRole('button', { name: /Imprimer le ticket CMD-2026-101/ }));
    expect(onOpenTicket).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole('button', { name: /Confirmer la livraison — CMD-2026-101/ }));
    expect(onAdvance).toHaveBeenCalledWith('CMD-2026-101');
  });

  it('signale un dépôt clôturé au lieu de proposer une action', () => {
    render(
      <OrdersTable orders={[DEPOTS[1]]} columns={['ticket', 'statut', 'actions']} onAdvance={() => {}} />
    );

    expect(screen.getByText('Clôturé')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Confirmer la livraison/ })).not.toBeInTheDocument();
  });

  it('n’affiche aucune colonne d’action en lecture seule', () => {
    render(<OrdersTable orders={DEPOTS} columns={['ticket', 'client']} />);

    expect(screen.queryByRole('button', { name: /Notifier/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Imprimer le ticket/ })).not.toBeInTheDocument();
  });
});
