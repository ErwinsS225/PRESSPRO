import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  fetchPlatformStats,
  fetchTenants,
  formatXOF,
  registerTenant,
  setTenantStatus,
  subscriptionLabel,
} from './adminApi';

const STATS = {
  total_registered_pressings: 9,
  total_users: 7,
  active_subscriptions: 3,
  trialing_subscriptions: 6,
  monthly_recurring_mrr_xof: 90000,
  subscriptions_by_status: { ACTIVE: 3, TRIALING: 6 },
  subscriptions_by_plan: { FREE: 7, PRO: 2 },
  data_source: 'postgresql',
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

describe('fetchPlatformStats', () => {
  it('transmet le jeton et retourne les indicateurs réels', async () => {
    const fetchMock = vi.fn(async (_url: string, _init?: RequestInit) => jsonResponse(STATS));
    vi.stubGlobal('fetch', fetchMock);

    const stats = await fetchPlatformStats('jeton-superadmin');

    expect(stats).toEqual(STATS);
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/admin/stats', {
      headers: expect.objectContaining({ Authorization: 'Bearer jeton-superadmin' }),
    });
  });

  it('retourne null quand l\'API refuse l\'accès (403)', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse({ error: 'accès refusé' }, 403)));
    await expect(fetchPlatformStats('jeton')).resolves.toBeNull();
  });

  it('retourne null quand le réseau est indisponible', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => {
        throw new Error('réseau indisponible');
      })
    );
    await expect(fetchPlatformStats('jeton')).resolves.toBeNull();
  });
});

describe('fetchTenants', () => {
  it('retourne l\'annuaire des établissements', async () => {
    const tenants = [
      {
        tenant_id: 't-1',
        name: 'Pressing Ivoire Cocody',
        slug: 'pressing-ivoire-cocody',
        city: 'Abidjan',
        phone: '+225 07 00 11 22 33',
        owner_email: 'gerant@pressing-ivoire.ci',
        plan_code: 'FREE',
        plan_name: 'Gratuit permanent',
        price_monthly_xof: 0,
        status: 'TRIALING',
        created_at: '2026-10-11T00:00:00Z',
        active: true,
      },
    ];
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse({ tenants, total: 1 })));

    await expect(fetchTenants('jeton')).resolves.toEqual(tenants);
  });

  it('retourne une liste vide en cas d\'erreur serveur (jamais de plantage de la vue)', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse({ error: 'interne' }, 500)));
    await expect(fetchTenants('jeton')).resolves.toEqual([]);
  });
});

describe('setTenantStatus', () => {
  it('suspend un établissement (PATCH réel)', async () => {
    const fetchMock = vi.fn(async (_url: string, _init?: RequestInit) =>
      jsonResponse({ tenant_id: 't-1', status: 'SUSPENDED' })
    );
    vi.stubGlobal('fetch', fetchMock);

    const result = await setTenantStatus('t-1', 'SUSPENDED', 'jeton', 'impayé');

    expect(result.ok).toBe(true);
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/admin/tenants/t-1/status',
      expect.objectContaining({ method: 'PATCH' })
    );
    const body = JSON.parse((fetchMock.mock.calls[0][1] as RequestInit).body as string);
    expect(body).toEqual({ status: 'SUSPENDED', reason: 'impayé' });
  });

  it('remonte le message d\'erreur métier renvoyé par l\'API', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse({ error: 'aucun abonnement pour cet établissement' }, 404))
    );
    await expect(setTenantStatus('inconnu', 'SUSPENDED', 'jeton')).resolves.toEqual({
      ok: false,
      error: 'aucun abonnement pour cet établissement',
    });
  });
});

describe('registerTenant', () => {
  it('inscrit un nouvel établissement avec ses informations d\'exploitation', async () => {
    const fetchMock = vi.fn(async (_url: string, _init?: RequestInit) =>
      jsonResponse({ tenant: { id: 'nouveau' } }, 201)
    );
    vi.stubGlobal('fetch', fetchMock);

    const result = await registerTenant({
      tenant_name: 'Pressing de la Lagune',
      city: 'Abidjan (Treichville)',
      phone: '+225 07 12 34 56 78',
      owner_full_name: 'Awa Traoré',
      owner_email: 'awa@lagune.ci',
      password: 'motdepasse123',
    });

    expect(result.ok).toBe(true);
    const body = JSON.parse((fetchMock.mock.calls[0][1] as RequestInit).body as string);
    expect(body.tenant_name).toBe('Pressing de la Lagune');
    expect(body.owner_email).toBe('awa@lagune.ci');
  });

  it('signale un email déjà utilisé (409)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse({ error: 'cet email est déjà utilisé' }, 409))
    );
    await expect(
      registerTenant({
        tenant_name: 'Doublon',
        city: 'Abidjan',
        phone: '',
        owner_full_name: 'X',
        owner_email: 'doublon@test.ci',
        password: 'motdepasse123',
      })
    ).resolves.toEqual({ ok: false, error: 'cet email est déjà utilisé' });
  });
});

describe('libellés', () => {
  it('traduit les statuts d\'abonnement', () => {
    expect(subscriptionLabel('ACTIVE')).toBe('Actif');
    expect(subscriptionLabel('TRIALING')).toBe("Période d'essai");
    expect(subscriptionLabel('SUSPENDED')).toBe('Suspendu');
    expect(subscriptionLabel('PAST_DUE')).toBe('Impayé');
  });

  it('formate les montants en francs CFA', () => {
    expect(formatXOF(45000)).toContain('45');
    expect(formatXOF(45000)).toMatch(/FCFA$/);
    expect(formatXOF(0)).toBe('0 FCFA');
  });
});
