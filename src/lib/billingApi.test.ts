import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  fetchEntitlements,
  fetchPlans,
  formatDate,
  formatXOF,
  libelleDroit,
  libelleStatut,
  valeurDroit,
} from './billingApi';

function reponse(body: unknown, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  } as Response;
}

const DROITS_VALIDES = {
  tenant_id: 'ten-1',
  plan_code: 'BUSINESS',
  subscribed_plan_code: 'FREE',
  plan_name: 'Business',
  status: 'TRIALING',
  on_trial: true,
  trial_ends_at: '2026-10-19T10:00:00Z',
  days_left_trial: 7,
  entitlements: [
    { key: 'orders.monthly.max', enabled: true, limit: 50, usage: 3, remaining: 47, source: 'plan' },
  ],
  price_monthly_xof: 75000,
  orders_period: { start: '2026-10-01T00:00:00Z', end: '2026-10-31T00:00:00Z' },
};

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('fetchEntitlements', () => {
  it('renvoie les droits tels que le serveur les donne, avec le jeton', async () => {
    const fetchMock = vi.fn(async () => reponse(DROITS_VALIDES));
    vi.stubGlobal('fetch', fetchMock);

    const res = await fetchEntitlements('jeton-1');

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/me/entitlements',
      expect.objectContaining({
        headers: expect.objectContaining({ Authorization: 'Bearer jeton-1' }),
      })
    );
    expect(res).toEqual({ ok: true, data: DROITS_VALIDES });
  });

  it('signale une session expirée (401) sans données de repli', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => reponse({ error: 'unauthorized' }, 401))
    );
    const res = await fetchEntitlements('expire');
    expect(res.ok).toBe(false);
    if (!res.ok) expect(res.error).toMatchObject({ kind: 'auth', status: 401 });
  });

  it('signale un accès refusé (403)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => reponse({}, 403))
    );
    const res = await fetchEntitlements('x');
    expect(res.ok).toBe(false);
    if (!res.ok) expect(res.error.kind).toBe('forbidden');
  });

  it('signale une erreur serveur (500)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => reponse({}, 500))
    );
    const res = await fetchEntitlements('x');
    expect(res.ok).toBe(false);
    if (!res.ok) expect(res.error).toMatchObject({ kind: 'server', status: 500 });
  });

  it('signale un réseau indisponible', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => {
        throw new TypeError('Failed to fetch');
      })
    );
    const res = await fetchEntitlements('x');
    expect(res.ok).toBe(false);
    if (!res.ok) expect(res.error.kind).toBe('network');
  });

  it('rejette une réponse sans la forme attendue, au lieu de l’afficher', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => reponse({ plan: 'PRO' }))
    );
    const res = await fetchEntitlements('x');
    expect(res.ok).toBe(false);
    if (!res.ok) expect(res.error.kind).toBe('invalid');
  });
});

describe('fetchPlans', () => {
  it('lit la grille depuis l’API', async () => {
    const plans = [
      { code: 'FREE', name: 'Gratuit', price_monthly_xof: 0, entitlements: [] },
      { code: 'PRO', name: 'Pro', price_monthly_xof: 45000, entitlements: [] },
    ];
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => reponse({ plans }))
    );
    const res = await fetchPlans();
    expect(res).toEqual({ ok: true, data: plans });
  });

  it('rejette une grille qui n’est pas une liste', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => reponse({ plans: 'PRO' }))
    );
    const res = await fetchPlans();
    expect(res.ok).toBe(false);
  });
});

describe('formatage', () => {
  it('formate les montants en francs CFA et refuse un montant inconnu', () => {
    expect(formatXOF(45000)).toMatch(/^45\s000 FCFA$/);
    expect(formatXOF(null)).toBeNull();
    expect(formatXOF(undefined)).toBeNull();
  });

  it('formate les dates en français et refuse une date invalide', () => {
    expect(formatDate('2026-10-19T10:00:00Z')).toMatch(/19 octobre 2026/);
    expect(formatDate('pas-une-date')).toBeNull();
    expect(formatDate(null)).toBeNull();
  });

  it('libellés de droits et de statuts, avec repli sur la valeur brute', () => {
    expect(libelleDroit('orders.monthly.max')).toBe('Commandes (fenêtre de 30 jours)');
    expect(libelleDroit('inconnu.droit')).toBe('inconnu.droit');
    expect(libelleStatut('TRIALING')).toBe('Essai gratuit');
    expect(libelleStatut('INCONNU')).toBe('INCONNU');
  });

  it('valeur d’un droit : non inclus, illimité, nombre ou inclus', () => {
    expect(valeurDroit({ key: 'reports.advanced', enabled: false, source: 'plan' })).toBe('non inclus');
    expect(valeurDroit({ key: 'orders.monthly.max', enabled: true, limit: 50, source: 'plan' })).toBe('50');
    expect(valeurDroit({ key: 'users.max', enabled: true, source: 'plan' })).toBe('illimité');
    expect(valeurDroit({ key: 'reports.advanced', enabled: true, source: 'plan' })).toBe('inclus');
  });
});
