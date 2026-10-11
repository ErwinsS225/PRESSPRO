import { afterEach, describe, expect, it, vi } from 'vitest';
import { fetchReportSummary, orderStatusLabel, periodBounds } from './reportsApi';

const RAPPORT = {
  period: { from: '2026-10-01', to: '2026-10-31' },
  data_source: 'postgresql',
  revenue_xof: 7000,
  orders_count: 2,
  orders_by_status: { delivered: 1, ready: 1 },
  average_ticket_xof: 4500,
  expenses_xof: 175000,
  margin_xof: -168000,
  expenses_by_category: [{ category: 'loyer', total_xof: 150000, count: 1 }],
  top_services: [{ label: 'Chemise', count: 3, revenue_xof: 7000 }],
  consolidated: false,
  plan_hint: 'La consolidation multi-sites est incluse dans la formule BUSINESS.',
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

describe('fetchReportSummary', () => {
  it('charge le rapport de la période demandée', async () => {
    const fetchMock = vi.fn(async (_url: string, _init?: RequestInit) => jsonResponse(RAPPORT));
    vi.stubGlobal('fetch', fetchMock);

    const resultat = await fetchReportSummary({ from: '2026-10-01', to: '2026-10-31' }, 'jeton');

    expect(resultat.ok).toBe(true);
    expect(resultat.report?.revenue_xof).toBe(7000);
    expect(resultat.report?.top_services[0].label).toBe('Chemise');
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/reports/summary?from=2026-10-01&to=2026-10-31',
      expect.objectContaining({ headers: expect.objectContaining({ Authorization: 'Bearer jeton' }) })
    );
  });

  it('signale les rapports réservés aux formules payantes (402)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        jsonResponse(
          { error: 'fonctionnalité non incluse dans votre formule', feature: 'reports.advanced' },
          402
        )
      )
    );

    const resultat = await fetchReportSummary({ from: '2026-10-01', to: '2026-10-31' }, 'jeton');

    expect(resultat.ok).toBe(false);
    expect(resultat.planLocked).toBe(true);
  });

  it('remonte une période invalide (400)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        jsonResponse(
          { error: 'période invalide (format AAAA-MM-JJ attendu, début antérieur ou égal à la fin)' },
          400
        )
      )
    );
    const resultat = await fetchReportSummary({ from: '2026-10-31', to: '2026-10-01' }, 'jeton');
    expect(resultat.ok).toBe(false);
    expect(resultat.error).toContain('période invalide');
  });

  it('retourne une erreur explicite quand le réseau est indisponible', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => {
        throw new Error('réseau indisponible');
      })
    );
    const resultat = await fetchReportSummary({ from: '2026-10-01', to: '2026-10-31' }, 'jeton');
    expect(resultat.ok).toBe(false);
    expect(resultat.error).toBe('réseau indisponible');
  });
});

describe('periodBounds', () => {
  it('borne « ce mois » au premier jour du mois courant', () => {
    const bornes = periodBounds('mois');
    const maintenant = new Date().toISOString().slice(0, 10);
    expect(bornes.to).toBe(maintenant);
    expect(bornes.from.slice(0, 7)).toBe(maintenant.slice(0, 7));
    expect(bornes.from.endsWith('-01')).toBe(true);
  });

  it('borne « mois dernier » du premier au dernier jour du mois précédent', () => {
    const bornes = periodBounds('mois_dernier');
    const [annee, mois, jour] = bornes.from.split('-').map((v) => parseInt(v, 10));
    expect(jour).toBe(1);

    const dernierJour = new Date(Date.UTC(annee, mois, 0)).getUTCDate();
    expect(bornes.to).toBe(`${bornes.from.slice(0, 7)}-${String(dernierJour).padStart(2, '0')}`);
  });

  it('borne « 30 derniers jours »', () => {
    const bornes = periodBounds('trente_jours');
    const debut = new Date(bornes.from);
    const fin = new Date(bornes.to);
    const jours = Math.round((fin.getTime() - debut.getTime()) / (24 * 3600 * 1000));
    expect(jours).toBe(29);
  });
});

describe('orderStatusLabel', () => {
  it('traduit les statuts de commande', () => {
    expect(orderStatusLabel('delivered')).toBe('Livrée');
    expect(orderStatusLabel('ready')).toBe('Prête');
    expect(orderStatusLabel('inconnu')).toBe('inconnu');
  });
});
