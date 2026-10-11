import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ClientNotificationModal } from './ClientNotificationModal';
import { Order, User } from '../types';

const gerant: User = {
  id: 'usr-gerant-001',
  email: 'gerant@pressing-ivoire.ci',
  full_name: 'Gérant Pressing Ivoire',
  role: 'owner',
  status: 'active',
};

const commande: Order = {
  id: 'cmd-1',
  orderNumber: 'CMD-2026-101',
  customerName: 'Awa Koné',
  customerPhone: '+225 05 44 33 22 11',
  items: [
    {
      id: 'it-1',
      serviceName: 'Chemise',
      quantity: 3,
      unitPrice: 1000,
      subtotal: 3000,
    },
  ],
  status: 'ready',
  paymentStatus: 'partially_paid',
  totalAmount: 8500,
  paidAmount: 5000,
  receivedAt: '2026-10-10T09:00:00Z',
  dueDate: 'Demain 17:00',
};

function jsonResponse(body: unknown, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  } as Response;
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('ClientNotificationModal', () => {
  it('affiche le modèle appliqué, le solde restant et le lien WhatsApp de repli', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse({ notifications: [], count: 0 }))
    );

    render(
      <ClientNotificationModal order={commande} currentUser={gerant} token="jeton" onClose={() => {}} />
    );

    const message = (await screen.findByLabelText('Message')) as HTMLTextAreaElement;
    // Le formatage français utilise une espace fine insécable (U+202F).
    const valeur = message.value.replace(/\u202f|\u00a0/g, ' ');
    expect(valeur).toContain('Bonjour Awa');
    expect(valeur).toContain('CMD-2026-101');
    expect(valeur).toContain('Reste à payer : 3 500 FCFA');
    expect(screen.getByText('Aucun message pour cette commande.')).toBeInTheDocument();

    const lien = screen.getByRole('link', { name: /Ouvrir dans WhatsApp/ });
    expect(lien).toHaveAttribute('href', expect.stringContaining('https://wa.me/2250544332211'));
  });

  it('change de modèle et de canal (SMS → lien natif)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse({ notifications: [] }))
    );

    render(
      <ClientNotificationModal order={commande} currentUser={gerant} token="jeton" onClose={() => {}} />
    );
    await screen.findByText('Aucun message pour cette commande.');

    fireEvent.click(screen.getByRole('button', { name: 'Commande livrée' }));
    expect((screen.getByLabelText('Message') as HTMLTextAreaElement).value).toContain('a bien été remise');

    fireEvent.click(screen.getByRole('button', { name: /SMS/ }));
    expect(screen.getByRole('link', { name: /Ouvrir le SMS/ })).toHaveAttribute(
      'href',
      expect.stringContaining('sms:+2250544332211')
    );
  });

  it('journalise le message et rafraîchit l’historique après envoi', async () => {
    const notif = {
      id: 'ntf-1',
      tenant_id: 'tenant-1',
      order_id: 'cmd-1',
      channel: 'whatsapp',
      template: 'commande_prete',
      recipient: '+225 05 44 33 22 11',
      body: 'Bonjour Awa, votre linge est prêt au comptoir.',
      status: 'simule',
      provider: 'journal',
      attempts: 1,
      created_at: '2026-10-11T10:00:00Z',
    };
    let journal = [] as (typeof notif)[];
    const fetchMock = vi.fn(async (url: string, init?: RequestInit) => {
      const cible = String(url);
      if (cible.includes('/api/v1/notifications') && init?.method === 'POST') {
        journal = [notif];
        return jsonResponse({ notification: notif, status: 'simule', message: 'message journalisé' }, 201);
      }
      return jsonResponse({ notifications: journal, count: journal.length });
    });
    vi.stubGlobal('fetch', fetchMock);

    const toasts: string[] = [];
    render(
      <ClientNotificationModal
        order={commande}
        currentUser={gerant}
        token="jeton"
        onClose={() => {}}
        showToast={(m) => toasts.push(m)}
      />
    );
    await screen.findByText('Aucun message pour cette commande.');

    fireEvent.click(screen.getByRole('button', { name: 'Envoyer' }));

    await waitFor(() => expect(toasts.some((m) => m.includes('Message journalisé'))).toBe(true));
    await waitFor(() =>
      expect(
        screen.getAllByText(/Bonjour Awa, votre linge est prêt au comptoir/).length
      ).toBeGreaterThanOrEqual(1)
    );
    expect(screen.getByText(/Journalisé \(sans envoi\)/)).toBeInTheDocument();
  });

  it('affiche l’offre commerciale quand l’envoi serveur est verrouillé (402)', async () => {
    const fetchMock = vi.fn(async (url: string, init?: RequestInit) => {
      if (init?.method === 'POST') {
        return jsonResponse({ error: 'formule supérieure requise', feature: 'notifications.advanced' }, 402);
      }
      return jsonResponse({ notifications: [] });
    });
    vi.stubGlobal('fetch', fetchMock);

    render(
      <ClientNotificationModal order={commande} currentUser={gerant} token="jeton" onClose={() => {}} />
    );
    await screen.findByText('Aucun message pour cette commande.');

    fireEvent.click(screen.getByRole('button', { name: 'Envoyer' }));

    await waitFor(() =>
      expect(screen.getByText(/Envoi automatique réservé aux formules payantes/)).toBeInTheDocument()
    );
    expect(screen.getByRole('link', { name: /Ouvrir dans WhatsApp/ })).toBeInTheDocument();
  });

  it('rend le message éditable et l’envoie tel quel', async () => {
    let corpsEnvoye: Record<string, unknown> = {};
    const fetchMock = vi.fn(async (url: string, init?: RequestInit) => {
      if (init?.method === 'POST') {
        corpsEnvoye = JSON.parse(String(init.body));
        return jsonResponse(
          {
            notification: {
              id: 'ntf-2',
              tenant_id: 'tenant-1',
              order_id: 'cmd-1',
              channel: 'whatsapp',
              recipient: '+225 05 44 33 22 11',
              body: String(corpsEnvoye.message),
              status: 'simule',
              attempts: 1,
              created_at: '2026-10-11T10:05:00Z',
            },
            status: 'simule',
          },
          201
        );
      }
      return jsonResponse({ notifications: [] });
    });
    vi.stubGlobal('fetch', fetchMock);

    render(
      <ClientNotificationModal order={commande} currentUser={gerant} token="jeton" onClose={() => {}} />
    );
    await screen.findByText('Aucun message pour cette commande.');

    const zone = screen.getByLabelText('Message') as HTMLTextAreaElement;
    fireEvent.change(zone, { target: { value: 'Bonjour Awa, votre linge vous attend !' } });
    expect(screen.getByText(/message personnalisé/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: 'Envoyer' }));

    await waitFor(() => expect(corpsEnvoye.message).toBe('Bonjour Awa, votre linge vous attend !'));
    expect(corpsEnvoye.template).toBeUndefined();
  });
});
