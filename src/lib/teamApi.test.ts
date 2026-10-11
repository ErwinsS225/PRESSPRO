import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  branchStatusLabel,
  createTeamMember,
  fetchMemberBranches,
  fetchTeam,
  roleLabel,
  setMemberBranches,
  setTeamMemberStatus,
  statusLabel,
} from './teamApi';

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
    status: 'inactive',
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

describe('fetchTeam', () => {
  it('retourne les comptes de l’établissement avec le jeton', async () => {
    const fetchMock = vi.fn(async (_url: string, _init?: RequestInit) => jsonResponse(MEMBRES));
    vi.stubGlobal('fetch', fetchMock);

    const membres = await fetchTeam('jeton-gerant');

    expect(membres).toHaveLength(2);
    expect(membres?.[1].full_name).toBe('Awa Koné');
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/users', {
      headers: expect.objectContaining({ Authorization: 'Bearer jeton-gerant' }),
    });
  });

  it('accepte la forme { users: [...] }', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse({ users: MEMBRES }))
    );
    await expect(fetchTeam('jeton')).resolves.toHaveLength(2);
  });

  it('retourne null quand l’accès est refusé (403)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse({ error: 'accès refusé' }, 403))
    );
    await expect(fetchTeam('jeton')).resolves.toBeNull();
  });

  it('retourne null quand le réseau est indisponible', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => {
        throw new Error('réseau indisponible');
      })
    );
    await expect(fetchTeam()).resolves.toBeNull();
  });
});

describe('createTeamMember', () => {
  const entree = {
    full_name: 'Koffi Yao',
    email: 'koffi@pressing-ivoire.ci',
    role: 'cashier',
    password: 'motdepasse123',
  };

  it('crée le compte et retourne sa fiche', async () => {
    const fetchMock = vi.fn(async (_url: string, _init?: RequestInit) =>
      jsonResponse({ id: 'usr-koffi-003', ...entree, status: 'active' }, 201)
    );
    vi.stubGlobal('fetch', fetchMock);

    const resultat = await createTeamMember(entree, 'jeton-gerant');

    expect(resultat.ok).toBe(true);
    expect(resultat.member?.id).toBe('usr-koffi-003');
    const [, init] = fetchMock.mock.calls[0];
    expect(init?.method).toBe('POST');
    expect(JSON.parse(String(init?.body))).toEqual(entree);
  });

  it('signale le plafond de la formule (402) pour proposer une mise à niveau', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        jsonResponse(
          {
            error: 'plafond atteint (2) — passez à une formule supérieure pour ajouter des utilisateurs',
            feature: 'users.max',
            upgrade_required: true,
          },
          402
        )
      )
    );

    const resultat = await createTeamMember(entree, 'jeton-gerant');

    expect(resultat.ok).toBe(false);
    expect(resultat.quotaReached).toBe(true);
    expect(resultat.error).toContain('plafond atteint');
  });

  it('remonte le doublon d’email (409)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse({ error: 'cette adresse email est déjà utilisée' }, 409))
    );

    const resultat = await createTeamMember(entree, 'jeton-gerant');

    expect(resultat.ok).toBe(false);
    expect(resultat.quotaReached).toBe(false);
    expect(resultat.error).toContain('déjà utilisée');
  });
});

describe('setTeamMemberStatus', () => {
  it('désactive un compte via PATCH', async () => {
    const fetchMock = vi.fn(async (_url: string, _init?: RequestInit) =>
      jsonResponse({ id: 'usr-awa-002', status: 'inactive' })
    );
    vi.stubGlobal('fetch', fetchMock);

    const resultat = await setTeamMemberStatus('usr-awa-002', 'inactive', 'jeton-gerant');

    expect(resultat.ok).toBe(true);
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/users/usr-awa-002/status', {
      method: 'PATCH',
      headers: expect.objectContaining({ Authorization: 'Bearer jeton-gerant' }),
      body: JSON.stringify({ status: 'inactive' }),
    });
  });

  it('refuse la modification de son propre compte (400)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        jsonResponse({ error: 'vous ne pouvez pas modifier le statut de votre propre compte' }, 400)
      )
    );

    const resultat = await setTeamMemberStatus('usr-gerant-001', 'inactive', 'jeton');

    expect(resultat.ok).toBe(false);
    expect(resultat.error).toContain('propre compte');
  });
});

describe('libellés', () => {
  it('traduit les rôles et statuts', () => {
    expect(roleLabel('cashier')).toBe('Caissier');
    expect(roleLabel('role-inconnu')).toBe('role-inconnu');
    expect(statusLabel('active')).toBe('Actif');
    expect(statusLabel('inactive')).toBe('Désactivé');
  });
});

describe('fetchMemberBranches / setMemberBranches', () => {
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

  it('liste les établissements du pressing et l’affectation du compte', async () => {
    const fetchMock = vi.fn(async (_url: string, _init?: RequestInit) =>
      jsonResponse({ user_id: 'usr-awa-002', branches: SITES })
    );
    vi.stubGlobal('fetch', fetchMock);

    const resultat = await fetchMemberBranches('usr-awa-002', 'jeton-gerant');

    expect(resultat.ok).toBe(true);
    expect(resultat.branches?.map((site) => site.name)).toHaveLength(2);
    expect(resultat.branches?.[0].is_primary).toBe(true);
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/users/usr-awa-002/branches', {
      headers: expect.objectContaining({ Authorization: 'Bearer jeton-gerant' }),
    });
  });

  it('remplace l’affectation par les sites envoyés (PUT branch_ids)', async () => {
    const fetchMock = vi.fn(async (_url: string, init?: RequestInit) => {
      expect(init?.method).toBe('PUT');
      expect(JSON.parse(String(init?.body))).toEqual({ branch_ids: ['br-principal', 'br-lagune'] });
      return jsonResponse({
        user_id: 'usr-awa-002',
        branches: SITES.map((site) => ({ ...site, assigned: true })),
      });
    });
    vi.stubGlobal('fetch', fetchMock);

    const resultat = await setMemberBranches('usr-awa-002', ['br-principal', 'br-lagune'], 'jeton-gerant');

    expect(resultat.ok).toBe(true);
    expect(resultat.branches?.every((site) => site.assigned)).toBe(true);
  });

  it('relaie le refus d’une sélection vide (fail-closed côté serveur)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse({ error: 'au moins un établissement doit être affecté' }, 400))
    );

    const resultat = await setMemberBranches('usr-awa-002', [], 'jeton-gerant');

    expect(resultat.ok).toBe(false);
    expect(resultat.error).toContain('au moins un établissement');
  });

  it('remonte le cas d’un compte hors du pressing (404)', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => jsonResponse({ error: 'utilisateur introuvable' }, 404))
    );

    const resultat = await fetchMemberBranches('usr-inconnu', 'jeton-gerant');

    expect(resultat.ok).toBe(false);
    expect(resultat.error).toBe('utilisateur introuvable');
  });

  it('retourne une erreur explicite quand le réseau est indisponible', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => {
        throw new Error('Failed to fetch');
      })
    );

    expect((await fetchMemberBranches('usr-awa-002')).error).toBe('Failed to fetch');
    expect((await setMemberBranches('usr-awa-002', ['br-1'])).error).toBe('Failed to fetch');
  });

  it('libelle les statuts d’établissement', () => {
    expect(branchStatusLabel('active')).toBe('actif');
    expect(branchStatusLabel('archived')).toBe('archivé');
  });
});
