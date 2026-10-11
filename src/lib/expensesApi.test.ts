import { afterEach, describe, expect, it, vi } from 'vitest';
import { categoryLabel, createExpense, deleteExpense, fetchExpenses, monthBounds } from './expensesApi';

const PAYLOAD = {
  expenses: [
    {
      id: 'exp-1',
      tenant_id: 't-1',
      category: 'loyer',
      label: 'Loyer octobre',
      amount_xof: 150000,
      spent_at: '2026-10-01',
      created_at: '2026-10-01T08:00:00Z',
      updated_at: '2026-10-01T08:00:00Z',
    },
  ],
  count: 1,
  total_xof: 150000,
  by_category: [{ category: 'loyer', total_xof: 150000, count: 1 }],
  period: { from: '2026-10-01', to: '2026-10-31' },
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

describe('monthBounds', () => {
  it('retourne le premier et le dernier jour du mois (février bissextile inclus)', () => {
    expect(monthBounds('2026-10')).toEqual({ from: '2026-10-01', to: '2026-10-31' });
    expect(monthBounds('2024-02')).toEqual({ from: '2024-02-01', to: '2024-02-29' });
  });
});

describe('fetchExpenses', () => {
  it('transmet la période en paramètres de requête', async () => {
    const fetchMock = vi.fn(async (_url: string, _init?: RequestInit) => jsonResponse(PAYLOAD));
    vi.stubGlobal('fetch', fetchMock);

    const data = await fetchExpenses({ from: '2026-10-01', to: '2026-10-31' }, 'jeton');

    expect(data?.total_xof).toBe(150000);
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/expenses?from=2026-10-01&to=2026-10-31',
      expect.objectContaining({ headers: expect.objectContaining({ Authorization: 'Bearer jeton' }) })
    );
  });

  it('retourne null quand l’accès est refusé (403)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse({ error: 'accès refusé' }, 403))
    );
    await expect(fetchExpenses({}, 'jeton')).resolves.toBeNull();
  });
});

describe('createExpense', () => {
  const entree = { category: 'loyer', label: 'Loyer octobre', amount_xof: 150000 };

  it('enregistre la dépense avec le site d’imputation', async () => {
    const fetchMock = vi.fn(async (_url: string, _init?: RequestInit) =>
      jsonResponse({ id: 'exp-2', ...entree, spent_at: '2026-10-02' }, 201)
    );
    vi.stubGlobal('fetch', fetchMock);

    const resultat = await createExpense(entree, 'jeton', 'b-2');

    expect(resultat.ok).toBe(true);
    const [, init] = fetchMock.mock.calls[0];
    expect(init?.method).toBe('POST');
    expect((init?.headers as Record<string, string>)['X-Branch-ID']).toBe('b-2');
  });

  it('signale le module réservé aux formules payantes (402)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        jsonResponse(
          { error: 'fonctionnalité non incluse dans votre formule', feature: 'expenses.advanced' },
          402
        )
      )
    );

    const resultat = await createExpense(entree, 'jeton');

    expect(resultat.ok).toBe(false);
    expect(resultat.planLocked).toBe(true);
  });
});

describe('deleteExpense', () => {
  it('supprime une dépense', async () => {
    const fetchMock = vi.fn(async (_url: string, _init?: RequestInit) => jsonResponse({ status: 'deleted' }));
    vi.stubGlobal('fetch', fetchMock);

    const resultat = await deleteExpense('exp-1', 'jeton');

    expect(resultat.ok).toBe(true);
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/expenses/exp-1',
      expect.objectContaining({ method: 'DELETE' })
    );
  });
});

describe('categoryLabel', () => {
  it('traduit les postes de charge', () => {
    expect(categoryLabel('matieres_premieres')).toBe('Matières premières');
    expect(categoryLabel('inconnue')).toBe('inconnue');
  });
});
