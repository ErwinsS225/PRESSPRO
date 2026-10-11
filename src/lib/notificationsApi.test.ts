import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  channelLabel,
  fetchNotifications,
  phoneDigits,
  sendClientNotification,
  smsLink,
  statusLabel,
  templateLabel,
  whatsappLink,
} from './notificationsApi';

function jsonResponse(body: unknown, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  } as Response;
}

const NOTIFICATION = {
  id: 'ntf-1',
  tenant_id: 'tenant-1',
  order_id: 'cmd-1',
  channel: 'whatsapp',
  template: 'commande_prete',
  recipient: '+225 05 44 33 22 11',
  body: 'Bonjour Awa, votre linge est prêt…',
  status: 'simule',
  provider: 'journal',
  attempts: 1,
  created_at: '2026-10-11T10:00:00Z',
};

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('sendClientNotification', () => {
  it('transmet la demande d’envoi et retourne la notification journalisée', async () => {
    const fetchMock = vi.fn(async (_url: string, init?: RequestInit) => {
      expect(init?.method).toBe('POST');
      expect(JSON.parse(String(init?.body))).toEqual({
        order_id: 'cmd-1',
        channel: 'whatsapp',
        template: 'commande_prete',
      });
      return jsonResponse({ notification: NOTIFICATION, status: 'simule' }, 201);
    });
    vi.stubGlobal('fetch', fetchMock);

    const resultat = await sendClientNotification(
      { order_id: 'cmd-1', channel: 'whatsapp', template: 'commande_prete' },
      'jeton'
    );

    expect(resultat.ok).toBe(true);
    expect(resultat.notification?.status).toBe('simule');
    expect(fetchMock.mock.calls[0][1]?.headers).toEqual(
      expect.objectContaining({ Authorization: 'Bearer jeton' })
    );
  });

  it('signale l’envoi réservé aux formules payantes (402)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        jsonResponse({ error: 'formule supérieure requise', feature: 'notifications.advanced' }, 402)
      )
    );

    const resultat = await sendClientNotification({ order_id: 'cmd-1', channel: 'sms' }, 'jeton');

    expect(resultat.ok).toBe(false);
    expect(resultat.planLocked).toBe(true);
    expect(resultat.error).toContain('formule supérieure');
  });

  it('remonte une commande inconnue (404) et un canal invalide (400)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse({ error: 'commande introuvable' }, 404))
    );
    expect((await sendClientNotification({ order_id: 'absent', channel: 'sms' })).error).toBe(
      'commande introuvable'
    );

    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse({ error: 'canal inconnu (whatsapp ou sms)' }, 400))
    );
    expect((await sendClientNotification({ order_id: 'cmd-1', channel: 'whatsapp' })).ok).toBe(false);
  });

  it('retourne une erreur explicite quand le réseau est indisponible', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => {
        throw new Error('Failed to fetch');
      })
    );
    expect((await sendClientNotification({ order_id: 'cmd-1', channel: 'sms' })).error).toBe(
      'Failed to fetch'
    );
  });
});

describe('fetchNotifications', () => {
  it('filtre le journal par commande', async () => {
    const fetchMock = vi.fn(async (url: string) => {
      expect(String(url)).toContain('/api/v1/notifications?order_id=cmd-1&limit=20');
      return jsonResponse({ notifications: [NOTIFICATION], count: 1 });
    });
    vi.stubGlobal('fetch', fetchMock);

    const resultat = await fetchNotifications({ orderId: 'cmd-1', limit: 20 }, 'jeton');

    expect(resultat.ok).toBe(true);
    expect(resultat.notifications).toHaveLength(1);
    expect(resultat.notifications?.[0].body).toContain('Bonjour Awa');
  });

  it('retourne un journal vide sans filtre et gère l’erreur serveur', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse({ notifications: [], count: 0 }))
    );
    expect((await fetchNotifications()).notifications).toEqual([]);

    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse({ error: 'journal indisponible' }, 500))
    );
    expect((await fetchNotifications()).error).toBe('journal indisponible');
  });
});

describe('liens d’envoi manuel', () => {
  it('construit un lien WhatsApp international sans caractères parasites', () => {
    const lien = whatsappLink('+225 05 44 33 22 11', 'Bonjour Awa, votre linge est prêt.');
    expect(phoneDigits('+225 05 44 33 22 11')).toBe('2250544332211');
    expect(lien).toBe(
      'https://wa.me/2250544332211?text=' + encodeURIComponent('Bonjour Awa, votre linge est prêt.')
    );
  });

  it('construit un lien SMS natif et tolère un numéro absent', () => {
    expect(smsLink('05 44 33 22 11', 'Rappel')).toBe('sms:+0544332211?&body=Rappel');
    expect(whatsappLink('', 'Bonjour')).toBe('https://wa.me/?text=Bonjour');
    expect(smsLink('', 'Bonjour')).toBe('sms:?&body=Bonjour');
  });
});

describe('libellés', () => {
  it('traduit canaux, modèles et statuts', () => {
    expect(channelLabel('whatsapp')).toBe('WhatsApp');
    expect(channelLabel('sms')).toBe('SMS');
    expect(channelLabel('email')).toBe('email');
    expect(templateLabel('commande_prete')).toBe('Commande prête');
    expect(templateLabel('perso')).toBe('perso');
    expect(templateLabel(undefined)).toBe('Message libre');
    expect(statusLabel('simule')).toContain('Journalisé');
    expect(statusLabel('envoye')).toBe('Envoyé');
    expect(statusLabel('echec')).toBe('Échec');
  });
});
