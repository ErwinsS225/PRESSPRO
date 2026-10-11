import { afterEach, describe, expect, it, vi } from 'vitest';
import { branchStatusLabel, createBranch, fetchBranches, updateBranchStatus } from './branchesApi';

const SITES = [
  {
    id: 'b-1',
    tenant_id: 't-1',
    name: 'Établissement principal',
    is_primary: true,
    status: 'active',
    orders_count: 12,
    created_at: '2026-10-01T00:00:00Z',
    updated_at: '2026-10-01T00:00:00Z',
  },
  {
    id: 'b-2',
    tenant_id: 't-1',
    name: 'Site Yopougon',
    is_primary: false,
    status: 'suspended',
    orders_count: 3,
    created_at: '2026-10-05T00:00:00Z',
    updated_at: '2026-10-05T00:00:00Z',
  },
];

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

describe('fetchBranches', () => {
  it('retourne les établissements du pressing avec le jeton', async () => {
    const fetchMock = vi.fn(async (_url: string, _init?: RequestInit) => jsonResponse(SITES));
    vi.stubGlobal('fetch', fetchMock);

    const sites = await fetchBranches('jeton-gerant');

    expect(sites).toHaveLength(2);
    expect(sites?.[1].name).toBe('Site Yopougon');
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/branches', {
      headers: expect.objectContaining({ Authorization: 'Bearer jeton-gerant' }),
    });
  });

  it('retourne null quand l’API est injoignable', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => {
        throw new Error('réseau indisponible');
      })
    );
    await expect(fetchBranches()).resolves.toBeNull();
  });
});

describe('createBranch', () => {
  it('crée un site et retourne sa fiche', async () => {
    const fetchMock = vi.fn(async (_url: string, _init?: RequestInit) =>
      jsonResponse({ id: 'b-3', name: 'Site Treichville', is_primary: false, status: 'active' }, 201)
    );
    vi.stubGlobal('fetch', fetchMock);

    const resultat = await createBranch({ name: 'Site Treichville', city: 'Abidjan' }, 'jeton');

    expect(resultat.ok).toBe(true);
    expect(resultat.branch?.id).toBe('b-3');
    const [, init] = fetchMock.mock.calls[0];
    expect(init?.method).toBe('POST');
    expect(JSON.parse(String(init?.body)).name).toBe('Site Treichville');
  });

  it('signale le plafond multi-sites (402) pour proposer la formule BUSINESS', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        jsonResponse(
          {
            error: 'plafond atteint (1) — passez à une formule supérieure',
            feature: 'branches.multi_site',
            usage: 1,
          },
          402
        )
      )
    );

    const resultat = await createBranch({ name: 'Site 2' }, 'jeton');

    expect(resultat.ok).toBe(false);
    expect(resultat.quotaReached).toBe(true);
    expect(resultat.error).toContain('plafond atteint');
  });

  it('remonte le doublon de nom (409)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse({ error: 'un établissement de ce nom existe déjà' }, 409))
    );
    const resultat = await createBranch({ name: 'Site Yopougon' }, 'jeton');
    expect(resultat.ok).toBe(false);
    expect(resultat.error).toContain('existe déjà');
  });
});

describe('updateBranchStatus', () => {
  it('suspend un site via PATCH', async () => {
    const fetchMock = vi.fn(async (_url: string, _init?: RequestInit) =>
      jsonResponse({ id: 'b-2', status: 'suspended' })
    );
    vi.stubGlobal('fetch', fetchMock);

    const resultat = await updateBranchStatus('b-2', 'suspended', 'jeton');

    expect(resultat.ok).toBe(true);
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/branches/b-2', {
      method: 'PATCH',
      headers: expect.objectContaining({ Authorization: 'Bearer jeton' }),
      body: JSON.stringify({ status: 'suspended' }),
    });
  });

  it('refuse le cas du site principal (400)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        jsonResponse({ error: "l'établissement principal ne peut pas être suspendu ni archivé" }, 400)
      )
    );
    const resultat = await updateBranchStatus('b-1', 'suspended', 'jeton');
    expect(resultat.ok).toBe(false);
    expect(resultat.error).toContain('principal');
  });
});

describe('branchStatusLabel', () => {
  it('traduit les statuts', () => {
    expect(branchStatusLabel('active')).toBe('Actif');
    expect(branchStatusLabel('suspended')).toBe('Suspendu');
    expect(branchStatusLabel('archived')).toBe('Archivé');
  });
});
