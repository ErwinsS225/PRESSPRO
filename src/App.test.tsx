import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import App from './App';

/**
 * Test de fumée : l'application doit se monter sans exception. Un écran blanc
 * côté navigateur provient le plus souvent d'une erreur d'exécution au montage
 * (import cassé, hook mal utilisé), que ce test intercepte.
 */
const COMMANDE = {
  id: 'cmd-1',
  tenant_id: 'a0000000-0000-0000-0000-000000000001',
  customer_id: 'cli-1',
  customer_name: 'Awa Koné',
  customer_phone: '+225 05 44 33 22 11',
  order_number: 'CMD-2026-101',
  status: 'ready',
  payment_status: 'partially_paid',
  total_amount: 8500,
  paid_amount: 5000,
  remaining_due: 3500,
  created_at: '2026-10-10T09:00:00Z',
  due_date: '2026-10-12',
  items: [],
};

beforeEach(() => {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string) => {
      const cible = String(url);
      const corps = cible.includes('/auth/login')
        ? {
            token: 'jeton-demo',
            user: {
              id: 'usr-gerant-001',
              tenant_id: 'a0000000-0000-0000-0000-000000000001',
              email: 'gerant@pressing-ivoire.ci',
              full_name: 'M. Bamba Seydou',
              role: 'owner',
              status: 'active',
            },
          }
        : cible.includes('/notifications')
          ? { notifications: [], count: 0 }
          : cible.includes('/orders')
            ? [COMMANDE] // l'API renvoie une liste plate (snake_case)
            : cible.includes('/customers')
              ? { customers: [], count: 0 }
              : cible.includes('/services')
                ? { services: [], count: 0 }
                : { status: 'ok' };
      return { ok: true, status: 200, json: async () => corps } as Response;
    })
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('App (fumée)', () => {
  it('monte l’application et affiche la navigation principale', async () => {
    render(<App />);

    await waitFor(() => expect(screen.getAllByText('PressingOS').length).toBeGreaterThan(0));
    // La navigation principale est présente (sidebar + barre mobile).
    expect(screen.getAllByRole('button', { name: /Commandes/i }).length).toBeGreaterThan(0);
    expect(screen.getAllByRole('button', { name: /Rapports/i }).length).toBeGreaterThan(0);
  });

  it('ouvre la modale de notification client depuis le comptoir', async () => {
    render(<App />);

    // Le tableau de bord s'affiche d'abord : on rejoint le comptoir.
    const onglet = await waitFor(() => screen.getAllByRole('button', { name: /Commandes/i })[0]);
    fireEvent.click(onglet);

    // Puis on ouvre la notification du premier client de la liste.
    const bouton = await screen.findByRole('button', { name: /Notifier Awa Koné/i });
    fireEvent.click(bouton);

    // La modale se monte sans erreur et propose les deux canaux.
    await waitFor(() => expect(screen.getByLabelText('Message')).toBeInTheDocument());
    expect(screen.getByRole('button', { name: /Envoyer/ })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /Ouvrir dans WhatsApp/ })).toBeInTheDocument();
  });
});
