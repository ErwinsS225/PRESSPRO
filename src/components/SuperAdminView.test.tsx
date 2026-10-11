import { render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { SuperAdminView } from './SuperAdminView';
import { User } from '../types';

const superadmin: User = {
  id: 'usr-superadmin-000',
  email: 'admin@pressingos.ci',
  full_name: 'Administrateur PressingOS SaaS',
  role: 'superadmin',
  status: 'active',
};

const gerant = { ...superadmin, id: 'usr-gerant-001', role: 'owner' } as User;

const STATS = {
  total_registered_pressings: 2,
  total_users: 3,
  active_subscriptions: 1,
  trialing_subscriptions: 1,
  monthly_recurring_mrr_xof: 45000,
  subscriptions_by_status: { ACTIVE: 1, TRIALING: 1 },
  subscriptions_by_plan: { FREE: 1, PRO: 1 },
  data_source: 'postgresql',
};

const TENANTS = [
  {
    tenant_id: 't-1',
    name: 'Pressing Ivoire Cocody',
    slug: 'pressing-ivoire-cocody',
    city: 'Abidjan (Cocody)',
    phone: '+225 07 00 11 22 33',
    owner_email: 'gerant@pressing-ivoire.ci',
    plan_code: 'PRO',
    plan_name: 'Pro',
    price_monthly_xof: 45000,
    status: 'ACTIVE',
    created_at: '2026-10-01T00:00:00Z',
    active: true,
  },
  {
    tenant_id: 't-2',
    name: 'Pressing de la Lagune',
    slug: 'pressing-lagune',
    city: 'Abidjan (Treichville)',
    phone: '',
    owner_email: 'awa@lagune.ci',
    plan_code: 'FREE',
    plan_name: 'Gratuit',
    price_monthly_xof: 0,
    status: 'TRIALING',
    trial_ends_at: '2026-10-18T00:00:00Z',
    created_at: '2026-10-11T00:00:00Z',
    active: true,
  },
];

function stubApi() {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string) => {
      const body = url.includes('/admin/stats') ? STATS : { tenants: TENANTS, total: TENANTS.length };
      return { ok: true, status: 200, json: async () => body } as Response;
    })
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('SuperAdminView', () => {
  it('refuse l\'accès à un compte non superadmin', () => {
    render(<SuperAdminView currentUser={gerant} token="jeton" />);
    expect(screen.getByText('Accès refusé')).toBeInTheDocument();
    expect(screen.queryByText('Répertoire des Pressings Partenaires')).not.toBeInTheDocument();
  });

  it('affiche les indicateurs et l\'annuaire réels de la plateforme', async () => {
    stubApi();
    render(<SuperAdminView currentUser={superadmin} token="jeton-superadmin" />);

    await waitFor(() => {
      expect(screen.getByText('Pressing Ivoire Cocody')).toBeInTheDocument();
    });
    expect(screen.getByText('Pressing de la Lagune')).toBeInTheDocument();
    // MRR réel issu de l'API (45 000 FCFA) et non une valeur codée en dur.
    expect(screen.getByText('45 000')).toBeInTheDocument();
    // Le compte propriétaire vient de la base, pas d'une liste figée dans le composant.
    expect(screen.getByText('gerant@pressing-ivoire.ci')).toBeInTheDocument();
    expect(screen.getByText('awa@lagune.ci')).toBeInTheDocument();
    expect(screen.getAllByText('Abidjan (Cocody)').length).toBeGreaterThan(0);
  });

  it('signale l\'indisponibilité de l\'API au lieu d\'afficher de faux chiffres', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => {
      throw new Error('réseau indisponible');
    }));
    render(<SuperAdminView currentUser={superadmin} token="jeton" />);

    await waitFor(() => {
      expect(screen.getByText(/Impossible de charger les indicateurs/i)).toBeInTheDocument();
    });
  });
});
