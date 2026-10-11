import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { TeamView } from './TeamView';
import { User } from '../types';

const gerant: User = {
  id: 'usr-gerant-001',
  email: 'gerant@pressing-ivoire.ci',
  full_name: 'Gérant Pressing Ivoire',
  role: 'owner',
  status: 'active',
};

const MEMBRES = [
  {
    id: 'usr-gerant-001',
    email: 'gerant@pressing-ivoire.ci',
    full_name: 'Gérant Pressing Ivoire',
    role: 'owner',
    status: 'active',
  },
  {
    id: 'usr-awa-002',
    email: 'awa@lagune.ci',
    full_name: 'Awa Koné',
    role: 'cashier',
    status: 'active',
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

describe('TeamView', () => {
  it('liste les comptes et protège le compte courant', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(MEMBRES)));

    render(<TeamView currentUser={gerant} token="jeton-gerant" />);

    await waitFor(() => expect(screen.getByText('Awa Koné')).toBeInTheDocument());
    expect(screen.getByText('gerant@pressing-ivoire.ci')).toBeInTheDocument();
    // Deux comptes actifs ; le compte courant n'est pas désactivable.
    expect(screen.getByText(/2 compte\(s\) actif\(s\)/)).toBeInTheDocument();
    expect(screen.getByText('compte courant')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Désactiver/ })).toBeInTheDocument();
  });

  it('désactive un employé puis rafraîchit la liste', async () => {
    const fetchMock = vi.fn(async (url: string, _init?: RequestInit) => {
      if (String(url).endsWith('/status')) {
        return jsonResponse({ id: 'usr-awa-002', status: 'inactive' });
      }
      return jsonResponse(MEMBRES);
    });
    vi.stubGlobal('fetch', fetchMock);

    render(<TeamView currentUser={gerant} token="jeton-gerant" />);
    await waitFor(() => expect(screen.getByText('Awa Koné')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: /Désactiver/ }));

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        '/api/v1/users/usr-awa-002/status',
        expect.objectContaining({ method: 'PATCH' })
      )
    );
  });

  it('affiche le plafond de la formule quand la création est refusée (402)', async () => {
    const fetchMock = vi.fn(async (_url: string, init?: RequestInit) => {
      if (init?.method === 'POST') {
        return jsonResponse({ error: 'plafond atteint (2)', feature: 'users.max' }, 402);
      }
      return jsonResponse(MEMBRES);
    });
    vi.stubGlobal('fetch', fetchMock);

    render(<TeamView currentUser={gerant} token="jeton-gerant" />);
    await waitFor(() => expect(screen.getByText('Awa Koné')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: /Nouvel employé/ }));
    fireEvent.change(screen.getByPlaceholderText('Awa Koné'), { target: { value: 'Koffi Yao' } });
    fireEvent.change(screen.getByPlaceholderText('awa@pressing-ivoire.ci'), {
      target: { value: 'koffi@pressing-ivoire.ci' },
    });
    fireEvent.change(screen.getByPlaceholderText('••••••••'), {
      target: { value: 'motdepasse123' },
    });
    fireEvent.click(screen.getByRole('button', { name: /Créer le compte/ }));

    await waitFor(() => expect(screen.getByText(/formule supérieure/)).toBeInTheDocument());
  });
});
