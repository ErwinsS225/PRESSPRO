import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { BranchesView } from './BranchesView';
import { User } from '../types';

const gerant: User = {
  id: 'usr-gerant-001',
  email: 'gerant@pressing-ivoire.ci',
  full_name: 'Gérant Pressing Ivoire',
  role: 'owner',
  status: 'active',
};

const caissier: User = { ...gerant, id: 'usr-caissier-002', role: 'cashier' };

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
    city: 'Abidjan',
    is_primary: false,
    status: 'active',
    orders_count: 4,
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

describe('BranchesView', () => {
  it('liste les établissements et protège le site principal', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse(SITES))
    );

    render(<BranchesView currentUser={gerant} token="jeton" />);

    await waitFor(() => expect(screen.getByText('Site Yopougon')).toBeInTheDocument());
    expect(screen.getByText('Établissement principal')).toBeInTheDocument();
    expect(screen.getByText('2 établissement(s)')).toBeInTheDocument();
    // Un seul bouton « Suspendre » : celui du site secondaire.
    expect(screen.getAllByRole('button', { name: /Suspendre/ })).toHaveLength(1);
    expect(screen.getByText(/ne peut être ni suspendu ni archivé/)).toBeInTheDocument();
  });

  it('cache les actions d’écriture à un caissier', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse(SITES))
    );

    render(<BranchesView currentUser={caissier} token="jeton" />);

    await waitFor(() => expect(screen.getByText('Site Yopougon')).toBeInTheDocument());
    expect(screen.queryByRole('button', { name: /Nouvel établissement/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /Suspendre/ })).not.toBeInTheDocument();
  });

  it('suspend un site secondaire puis recharge la liste', async () => {
    const fetchMock = vi.fn(async (url: string, init?: RequestInit) => {
      if (init?.method === 'PATCH') return jsonResponse({ id: 'b-2', status: 'suspended' });
      return jsonResponse(SITES);
    });
    vi.stubGlobal('fetch', fetchMock);

    render(<BranchesView currentUser={gerant} token="jeton" />);
    await waitFor(() => expect(screen.getByText('Site Yopougon')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: /Suspendre/ }));

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        '/api/v1/branches/b-2',
        expect.objectContaining({ method: 'PATCH', body: JSON.stringify({ status: 'suspended' }) })
      )
    );
  });

  it('explique le plafond multi-sites lorsque l’API refuse en 402', async () => {
    const fetchMock = vi.fn(async (_url: string, init?: RequestInit) => {
      if (init?.method === 'POST') {
        return jsonResponse({ error: 'plafond atteint (1)', feature: 'branches.multi_site' }, 402);
      }
      return jsonResponse(SITES);
    });
    vi.stubGlobal('fetch', fetchMock);

    render(<BranchesView currentUser={gerant} token="jeton" />);
    await waitFor(() => expect(screen.getByText('Site Yopougon')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: /Nouvel établissement/ }));
    fireEvent.change(screen.getByPlaceholderText('Pressing Yopougon'), {
      target: { value: 'Site Treichville' },
    });
    fireEvent.click(screen.getByRole('button', { name: /Créer le site/ }));

    await waitFor(() => expect(screen.getByText(/formule BUSINESS autorise 3/)).toBeInTheDocument());
  });
});
