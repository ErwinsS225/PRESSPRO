import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { BillingView } from './BillingView';

const GRILLE = [
  { code: 'FREE', name: 'Gratuit', price_monthly_xof: 0, entitlements: [] },
  {
    code: 'PRO',
    name: 'Pro',
    price_monthly_xof: 45000,
    entitlements: [{ key: 'users.max', enabled: true, limit: 5, source: 'plan' }],
  },
  {
    code: 'BUSINESS',
    name: 'Business',
    price_monthly_xof: 75000,
    entitlements: [{ key: 'users.max', enabled: true, limit: 15, source: 'plan' }],
  },
];

/** Essai en cours : droits BUSINESS, formule souscrite FREE. */
const DROITS_ESSAI = {
  tenant_id: 'ten-1',
  plan_code: 'BUSINESS',
  subscribed_plan_code: 'FREE',
  plan_name: 'Gratuit',
  status: 'TRIALING',
  on_trial: true,
  trial_ends_at: '2026-10-19T10:00:00Z',
  days_left_trial: 6,
  entitlements: [
    { key: 'orders.monthly.max', enabled: true, limit: 50, usage: 12, remaining: 38, source: 'trial' },
    { key: 'users.max', enabled: true, limit: 15, usage: 2, remaining: 13, source: 'trial' },
    { key: 'reports.advanced', enabled: true, source: 'trial' },
  ],
  price_monthly_xof: 0,
  orders_period: { start: '2026-10-01T00:00:00Z', end: '2026-10-31T00:00:00Z' },
};

/** Abonnement payant actif : PRO, sans essai. */
const DROITS_PRO = {
  tenant_id: 'ten-2',
  plan_code: 'PRO',
  subscribed_plan_code: 'PRO',
  plan_name: 'Pro',
  status: 'ACTIVE',
  on_trial: false,
  entitlements: [
    { key: 'orders.monthly.max', enabled: true, limit: 500, usage: 40, remaining: 460, source: 'plan' },
  ],
  price_monthly_xof: 45000,
  orders_period: { start: '2026-10-05T00:00:00Z', end: '2026-11-04T00:00:00Z' },
};

function reponse(body: unknown, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => body,
  } as Response;
}

/** Route les appels GET selon l'URL ; chaque réponse peut être une fonction (appel n°). */
function routerApi(droits: () => Response, plans: () => Response) {
  return vi.fn(async (url: string) => {
    if (String(url).endsWith('/me/entitlements')) return droits();
    if (String(url).endsWith('/billing/plans')) return plans();
    throw new Error(`appel inattendu : ${url}`);
  });
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('BillingView — Mon abonnement', () => {
  it('affiche la formule souscrite, les tarifs venus de l’API et la fin de l’essai', async () => {
    vi.stubGlobal(
      'fetch',
      routerApi(
        () => reponse(DROITS_ESSAI),
        () => reponse({ plans: GRILLE })
      )
    );

    render(<BillingView token="jeton" />);

    expect(await screen.findByRole('heading', { name: /Formule souscrite : Gratuit/ })).toBeInTheDocument();
    expect(screen.getByText('Essai gratuit en cours')).toBeInTheDocument();
    expect(screen.getByText(/Fin de l.essai : 19 octobre 2026/)).toBeInTheDocument();
    expect(screen.getByText(/6 jour\(s\) restant\(s\)/)).toBeInTheDocument();

    // Les prix affichés sont ceux de la grille, pas des valeurs codées dans l'écran.
    expect(
      within(screen.getByRole('article', { name: 'Formule Pro' })).getByText(/45\s000 FCFA/)
    ).toBeInTheDocument();
    expect(
      within(screen.getByRole('article', { name: 'Formule Business' })).getByText(/75\s000 FCFA/)
    ).toBeInTheDocument();
  });

  it('marque la formule souscrite (FREE), pas la formule dont les droits sont actifs (BUSINESS) pendant l’essai', async () => {
    vi.stubGlobal(
      'fetch',
      routerApi(
        () => reponse(DROITS_ESSAI),
        () => reponse({ plans: GRILLE })
      )
    );

    render(<BillingView token="jeton" />);
    await screen.findByRole('heading', { name: /Formule souscrite/ });

    expect(
      within(screen.getByRole('article', { name: 'Formule Gratuit' })).getByText('Formule souscrite')
    ).toBeInTheDocument();
    expect(
      within(screen.getByRole('article', { name: 'Formule Business' })).queryByText('Formule souscrite')
    ).toBeNull();
  });

  it('affiche le compteur de commandes de la fenêtre glissante, sans libellé « ce mois »', async () => {
    vi.stubGlobal(
      'fetch',
      routerApi(
        () => reponse(DROITS_PRO),
        () => reponse({ plans: GRILLE })
      )
    );

    render(<BillingView token="jeton" />);
    await screen.findByRole('heading', { name: /Formule souscrite : Pro/ });

    const meter = screen.getByRole('progressbar', { name: 'Commandes sur la fenêtre en cours' });
    expect(meter).toHaveAttribute('aria-valuenow', '40');
    expect(meter).toHaveAttribute('aria-valuemax', '500');
    expect(screen.getByText('40 / 500')).toBeInTheDocument();
    expect(screen.getByText(/Période du .+ au .+ \(fin exclue\)/)).toBeInTheDocument();
    expect(screen.queryByText(/ce mois/i)).toBeNull();
    // Abonnement payant : pas de bandeau d'essai.
    expect(screen.queryByText('Essai gratuit en cours')).toBeNull();
  });

  it('n’affiche aucune formule ni droit fabriqués quand l’API est en erreur', async () => {
    vi.stubGlobal(
      'fetch',
      routerApi(
        () => reponse({}, 401),
        () => reponse({ plans: GRILLE })
      )
    );

    render(<BillingView token="expire" />);

    const alerte = await screen.findByRole('alert');
    expect(within(alerte).getByText(/Session expirée/)).toBeInTheDocument();
    expect(screen.queryByText('Essai gratuit en cours')).toBeNull();
    expect(screen.queryByText(/Formule souscrite :/)).toBeNull();
    // La grille publique reste affichée (prix de l'API), mais aucune formule n'est marquée
    // comme souscrite : l'état du compte est inconnu.
    expect(screen.getAllByRole('article')).toHaveLength(GRILLE.length);
    expect(screen.queryByText('Formule souscrite')).toBeNull();
  });

  it('signale une grille indisponible sans masquer la situation de l’abonnement', async () => {
    vi.stubGlobal(
      'fetch',
      routerApi(
        () => reponse(DROITS_PRO),
        () => reponse({}, 500)
      )
    );

    render(<BillingView token="jeton" />);

    expect(await screen.findByText(/Formule souscrite : Pro/)).toBeInTheDocument();
    expect(await screen.findByText(/Grille tarifaire indisponible/)).toBeInTheDocument();
    expect(screen.queryByRole('article')).toBeNull();
  });

  it('réessaie le chargement après une erreur réseau', async () => {
    let appel = 0;
    const fetchMock = vi.fn(async (url: string) => {
      if (String(url).endsWith('/me/entitlements')) {
        appel += 1;
        if (appel === 1) throw new TypeError('Failed to fetch');
        return reponse(DROITS_PRO);
      }
      return reponse({ plans: GRILLE });
    });
    vi.stubGlobal('fetch', fetchMock);

    render(<BillingView token="jeton" />);

    const alerte = await screen.findByRole('alert');
    expect(within(alerte).getByText(/Connexion impossible/)).toBeInTheDocument();

    fireEvent.click(within(alerte).getByRole('button', { name: /Réessayer/ }));

    expect(await screen.findByText(/Formule souscrite : Pro/)).toBeInTheDocument();
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('annonce le passage à une formule sans enregistrer de demande ni afficher de succès', async () => {
    const fetchMock = routerApi(
      () => reponse(DROITS_PRO),
      () => reponse({ plans: GRILLE })
    );
    vi.stubGlobal('fetch', fetchMock);

    render(<BillingView token="jeton" />);
    await screen.findByRole('heading', { name: /Formule souscrite : Pro/ });

    fireEvent.click(screen.getByRole('button', { name: 'Demander le passage à Business' }));

    const statut = await screen.findByRole('status');
    expect(within(statut).getByText(/n.enregistre aucune demande/)).toBeInTheDocument();
    expect(screen.queryByText(/Demande envoyée/i)).toBeNull();
    // Aucun appel d'écriture : seulement des lectures.
    for (const [, init] of fetchMock.mock.calls as unknown as Array<[string, RequestInit | undefined]>) {
      expect(init?.method ?? 'GET').toBe('GET');
    }
  });

  it('affiche « Tarif non communiqué » quand la base ne fournit pas de prix', async () => {
    const grilleSansPrix = GRILLE.map((p) => (p.code === 'BUSINESS' ? { ...p, price_monthly_xof: null } : p));
    vi.stubGlobal(
      'fetch',
      routerApi(
        () => reponse(DROITS_PRO),
        () => reponse({ plans: grilleSansPrix })
      )
    );

    render(<BillingView token="jeton" />);

    const business = await screen.findByRole('article', { name: 'Formule Business' });
    expect(within(business).getByText('Tarif non communiqué')).toBeInTheDocument();
  });

  it('actualiser relance le chargement des deux sources', async () => {
    const fetchMock = routerApi(
      () => reponse(DROITS_PRO),
      () => reponse({ plans: GRILLE })
    );
    vi.stubGlobal('fetch', fetchMock);

    render(<BillingView token="jeton" />);
    await screen.findByRole('heading', { name: /Formule souscrite : Pro/ });
    const avant = fetchMock.mock.calls.length;

    fireEvent.click(screen.getByRole('button', { name: "Actualiser l'abonnement" }));

    await waitFor(() => expect(fetchMock.mock.calls.length).toBe(avant + 2));
  });
});
