import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ReportsView } from './ReportsView';
import { User } from '../types';

const gerant: User = {
  id: 'usr-gerant-001',
  email: 'gerant@pressing-ivoire.ci',
  full_name: 'Gérant Pressing Ivoire',
  role: 'owner',
  status: 'active',
};

const BASE_RAPPORT = {
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

describe('ReportsView', () => {
  it('affiche les indicateurs réels calculés par le serveur', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (url: string) => {
        expect(String(url)).toContain('/api/v1/reports/summary?from=');
        return jsonResponse(BASE_RAPPORT);
      })
    );

    render(<ReportsView currentUser={gerant} token="jeton" />);

    // 7 000 FCFA apparaît en encaissé et dans le CA des prestations.
    await waitFor(() => expect(screen.getAllByText('7 000 FCFA').length).toBeGreaterThanOrEqual(1));
    expect(screen.getByText('175 000 FCFA')).toBeInTheDocument();
    expect(screen.getByText('-168 000 FCFA')).toBeInTheDocument();
    expect(screen.getByText(/Panier moyen 4 500 FCFA/)).toBeInTheDocument();
    expect(screen.getByText('Livrée')).toBeInTheDocument();
    expect(screen.getByText(/Chemise/)).toBeInTheDocument();
  });

  it('remplace la consolidation par une invitation BUSINESS quand la formule ne l’inclut pas', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse(BASE_RAPPORT))
    );

    render(<ReportsView currentUser={gerant} token="jeton" />);

    await waitFor(() => expect(screen.getByText(/Consolidation par établissement/)).toBeInTheDocument());
    expect(screen.getByText(/incluse dans la formule BUSINESS/)).toBeInTheDocument();
    expect(screen.queryByRole('table')).not.toBeInTheDocument();
  });

  it('affiche la consolidation par établissement quand elle est incluse', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        jsonResponse({
          ...BASE_RAPPORT,
          revenue_xof: 42000,
          margin_xof: 12000,
          consolidated: true,
          plan_hint: undefined,
          by_branch: [
            {
              branch_id: 'b-1',
              name: 'Établissement principal',
              orders_count: 8,
              revenue_xof: 30000,
              expenses_xof: 20000,
              margin_xof: 10000,
            },
            {
              branch_id: 'b-2',
              name: 'Site Yopougon',
              orders_count: 3,
              revenue_xof: 12000,
              expenses_xof: 10000,
              margin_xof: 2000,
            },
          ],
        })
      )
    );

    render(<ReportsView currentUser={gerant} token="jeton" />);

    await waitFor(() => expect(screen.getByRole('table')).toBeInTheDocument());
    expect(screen.getByText('Site Yopougon')).toBeInTheDocument();
    // 12 000 FCFA : encaissé du site et marge du site dans le tableau consolidé.
    expect(screen.getAllByText('12 000 FCFA').length).toBeGreaterThanOrEqual(2);
  });

  it('explique que les rapports sont réservés aux formules payantes (402)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        jsonResponse(
          { error: 'fonctionnalité non incluse dans votre formule', feature: 'reports.advanced' },
          402
        )
      )
    );

    render(<ReportsView currentUser={gerant} token="jeton" />);

    await waitFor(() =>
      expect(screen.getByText(/Rapports avancés réservés aux formules payantes/)).toBeInTheDocument()
    );
  });

  it('recharge le rapport quand la période change', async () => {
    const fetchMock = vi.fn(async (_url: string) => jsonResponse(BASE_RAPPORT));
    vi.stubGlobal('fetch', fetchMock);

    render(<ReportsView currentUser={gerant} token="jeton" />);
    await waitFor(() => expect(screen.getAllByText('7 000 FCFA').length).toBeGreaterThanOrEqual(1));

    fireEvent.click(screen.getByRole('button', { name: 'Mois dernier' }));

    await waitFor(() => {
      const urls = fetchMock.mock.calls.map(([url]) => String(url));
      expect(urls.some((u) => u.includes('from=') && u !== urls[0])).toBe(true);
    });
  });
});
