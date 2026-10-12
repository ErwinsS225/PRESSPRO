import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
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
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse(MEMBRES))
    );

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

  it('affecte un employé à un autre établissement depuis l’écran Équipe', async () => {
    const SITES = [
      {
        id: 'br-principal',
        name: 'Pressing Ivoire — Plateau',
        is_primary: true,
        status: 'active',
        assigned: true,
      },
      {
        id: 'br-lagune',
        name: 'Pressing Ivoire — Cocody',
        is_primary: false,
        status: 'active',
        assigned: false,
      },
    ];
    let affectations = ['br-principal'];

    const fetchMock = vi.fn(async (url: string, init?: RequestInit) => {
      const cible = String(url);
      if (cible.includes('/branches')) {
        if (init?.method === 'PUT') {
          affectations = JSON.parse(String(init.body)).branch_ids as string[];
        }
        return jsonResponse({
          user_id: 'usr-awa-002',
          branches: SITES.map((site) => ({ ...site, assigned: affectations.includes(site.id) })),
        });
      }
      return jsonResponse(MEMBRES);
    });
    vi.stubGlobal('fetch', fetchMock);

    const toasts: string[] = [];
    render(<TeamView currentUser={gerant} token="jeton-gerant" showToast={(m) => toasts.push(m)} />);
    await waitFor(() => expect(screen.getByText('Awa Koné')).toBeInTheDocument());

    fireEvent.click(
      within(screen.getByText('Awa Koné').closest('tr') as HTMLElement).getByRole('button', {
        name: 'Affecter',
      })
    );
    await waitFor(() => expect(screen.getByText('Pressing Ivoire — Cocody')).toBeInTheDocument());
    expect(screen.getByText('Pressing Ivoire — Plateau')).toBeInTheDocument();
    expect(screen.getByLabelText('Pressing Ivoire — Plateau')).toBeChecked();

    fireEvent.click(screen.getByLabelText('Pressing Ivoire — Cocody'));
    expect(screen.getByLabelText('Pressing Ivoire — Cocody')).toBeChecked();

    fireEvent.click(screen.getByRole('button', { name: /Enregistrer l’affectation/ }));

    await waitFor(() =>
      expect(toasts.some((m) => m.includes('agit désormais sur 2 établissements'))).toBe(true)
    );
    expect(affectations).toEqual(['br-principal', 'br-lagune']);
    expect(screen.queryByText('Cochez les sites sur lesquels ce compte peut encaisser')).toBeNull();
  });

  it('refuse de laisser un employé sans établissement', async () => {
    const SITE_UNIQUE = [
      {
        id: 'br-principal',
        name: 'Pressing Ivoire — Plateau',
        is_primary: true,
        status: 'active',
        assigned: true,
      },
    ];
    vi.stubGlobal(
      'fetch',
      vi.fn(async (url: string) => {
        if (String(url).includes('/branches')) {
          return jsonResponse({ user_id: 'usr-awa-002', branches: SITE_UNIQUE });
        }
        return jsonResponse(MEMBRES);
      })
    );

    render(<TeamView currentUser={gerant} token="jeton-gerant" />);
    await waitFor(() => expect(screen.getByText('Awa Koné')).toBeInTheDocument());

    fireEvent.click(
      within(screen.getByText('Awa Koné').closest('tr') as HTMLElement).getByRole('button', {
        name: 'Affecter',
      })
    );
    await waitFor(() => expect(screen.getByLabelText('Pressing Ivoire — Plateau')).toBeChecked());

    fireEvent.click(screen.getByLabelText('Pressing Ivoire — Plateau'));
    fireEvent.click(screen.getByRole('button', { name: /Enregistrer l’affectation/ }));

    await waitFor(() => expect(screen.getByText(/au moins un établissement/i)).toBeInTheDocument());
    expect(screen.getByText(/un employé sans site ne peut rien saisir/)).toBeInTheDocument();
  });

  it('un gérant ne peut ni verrouiller ni modifier un compte propriétaire', async () => {
    const gerantActeur: User = {
      id: 'usr-manager-003',
      email: 'manager@pressing-ivoire.ci',
      full_name: 'Yao Manager',
      role: 'manager',
      status: 'active',
    };
    const equipe = [
      ...MEMBRES,
      {
        id: 'usr-manager-003',
        email: 'manager@pressing-ivoire.ci',
        full_name: 'Yao Manager',
        role: 'manager',
        status: 'active',
      },
    ];
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse(equipe))
    );

    render(<TeamView currentUser={gerantActeur} token="jeton-manager" />);
    await waitFor(() => expect(screen.getByText('Awa Koné')).toBeInTheDocument());

    // La ligne du propriétaire est protégée ; seul l’employé est désactivable.
    expect(screen.getByText('compte protégé')).toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: /Désactiver/ })).toHaveLength(1);
  });

  it('un propriétaire peut verrouiller un autre propriétaire', async () => {
    const autreProprietaire = {
      id: 'usr-proprio-004',
      email: 'associee@pressing-ivoire.ci',
      full_name: 'Adjoua Associée',
      role: 'owner',
      status: 'active',
    };
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse([...MEMBRES, autreProprietaire]))
    );

    render(<TeamView currentUser={gerant} token="jeton-gerant" />);
    await waitFor(() => expect(screen.getByText('Adjoua Associée')).toBeInTheDocument());

    expect(screen.queryByText('compte protégé')).toBeNull();
    // Employé + autre propriétaire ; le compte courant reste protégé.
    expect(screen.getAllByRole('button', { name: /Désactiver/ })).toHaveLength(2);
  });

  it('ne propose pas le rôle propriétaire à la création d’un employé', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse(MEMBRES))
    );

    const { container } = render(<TeamView currentUser={gerant} token="jeton-gerant" />);
    await waitFor(() => expect(screen.getByText('Awa Koné')).toBeInTheDocument());

    fireEvent.click(screen.getByRole('button', { name: /Nouvel employé/ }));
    expect(container.querySelector('option[value="owner"]')).toBeNull();
  });
});
