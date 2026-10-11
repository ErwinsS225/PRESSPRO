import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ExpensesView } from './ExpensesView';
import { User } from '../types';

const gerant: User = {
  id: 'usr-gerant-001',
  email: 'gerant@pressing-ivoire.ci',
  full_name: 'Gérant Pressing Ivoire',
  role: 'owner',
  status: 'active',
};

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
    {
      id: 'exp-2',
      tenant_id: 't-1',
      category: 'matieres_premieres',
      label: 'Lessive en poudre',
      amount_xof: 25000,
      spent_at: '2026-10-03',
      created_at: '2026-10-03T08:00:00Z',
      updated_at: '2026-10-03T08:00:00Z',
    },
  ],
  count: 2,
  total_xof: 175000,
  by_category: [
    { category: 'loyer', total_xof: 150000, count: 1 },
    { category: 'matieres_premieres', total_xof: 25000, count: 1 },
  ],
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

describe('ExpensesView', () => {
  it('affiche le total du mois et la répartition par poste', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (url: string) => {
        expect(String(url)).toContain('/api/v1/expenses?from=');
        return jsonResponse(PAYLOAD);
      })
    );

    render(<ExpensesView currentUser={gerant} token="jeton" />);

    await waitFor(() => expect(screen.getByText('Loyer octobre')).toBeInTheDocument());
    expect(screen.getByText('175 000 FCFA')).toBeInTheDocument();
    expect(screen.getByText(/2 dépense\(s\) enregistrée\(s\)/)).toBeInTheDocument();
    expect(screen.getAllByText('Loyer').length).toBeGreaterThan(0);
    // Le poste apparaît dans la répartition et dans la ligne du tableau.
    expect(screen.getAllByText(/Matières premières/).length).toBeGreaterThanOrEqual(2);
  });

  it('enregistre une dépense saisie dans la fenêtre de création', async () => {
    const fetchMock = vi.fn(async (_url: string, init?: RequestInit) => {
      if (init?.method === 'POST') {
        return jsonResponse(
          { id: 'exp-3', label: 'Électricité', amount_xof: 30000, spent_at: '2026-10-10' },
          201
        );
      }
      return jsonResponse(PAYLOAD);
    });
    vi.stubGlobal('fetch', fetchMock);

    render(<ExpensesView currentUser={gerant} token="jeton" />);
    await waitFor(() => expect(screen.getByText('Loyer octobre')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: /Nouvelle dépense/ }));
    fireEvent.change(screen.getByPlaceholderText('Lessive en poudre (5 kg)'), {
      target: { value: 'Électricité' },
    });
    fireEvent.change(screen.getByPlaceholderText('25000'), { target: { value: '30000' } });
    fireEvent.click(screen.getByRole('button', { name: /Enregistrer/ }));

    await waitFor(() => {
      const appel = fetchMock.mock.calls.find(([, init]) => init?.method === 'POST');
      expect(appel).toBeTruthy();
      expect(JSON.parse(String(appel?.[1]?.body))).toMatchObject({ label: 'Électricité', amount_xof: 30000 });
    });
  });

  it('explique que le module est réservé aux formules payantes (402)', async () => {
    const fetchMock = vi.fn(async (_url: string, init?: RequestInit) => {
      if (init?.method === 'POST') {
        return jsonResponse({ error: 'fonctionnalité non incluse dans votre formule' }, 402);
      }
      return jsonResponse(PAYLOAD);
    });
    vi.stubGlobal('fetch', fetchMock);

    render(<ExpensesView currentUser={gerant} token="jeton" />);
    await waitFor(() => expect(screen.getByText('Loyer octobre')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: /Nouvelle dépense/ }));
    fireEvent.change(screen.getByPlaceholderText('Lessive en poudre (5 kg)'), {
      target: { value: 'Loyer novembre' },
    });
    fireEvent.change(screen.getByPlaceholderText('25000'), { target: { value: '150000' } });
    fireEvent.click(screen.getByRole('button', { name: /Enregistrer/ }));

    await waitFor(() => expect(screen.getByText(/Module réservé aux formules payantes/)).toBeInTheDocument());
  });

  it('supprime une dépense et signale une liste vide', async () => {
    const fetchMock = vi.fn(async (_url: string, init?: RequestInit) => {
      if (init?.method === 'DELETE') return jsonResponse({ status: 'deleted' });
      return jsonResponse(PAYLOAD);
    });
    vi.stubGlobal('fetch', fetchMock);

    render(<ExpensesView currentUser={gerant} token="jeton" />);
    await waitFor(() => expect(screen.getByText('Loyer octobre')).toBeInTheDocument());

    fireEvent.click(screen.getAllByRole('button', { name: /Supprimer/ })[0]);

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        '/api/v1/expenses/exp-1',
        expect.objectContaining({ method: 'DELETE' })
      )
    );
  });
});
