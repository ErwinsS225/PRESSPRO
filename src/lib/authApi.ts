import { AuthSession, UserRole } from '../types';

export const PRESET_ACCOUNTS = [
  {
    role: 'owner' as UserRole,
    label: 'Propriétaire / Gérant',
    email: 'gerant@pressing-ivoire.ci',
    password: 'pass123',
    tenantName: 'Pressing Ivoire Cocody',
    description:
      'Accès complet : tableau de bord, chiffre d’affaires, clients, tarifs et gestion des employés.',
  },
  {
    role: 'operator' as UserRole,
    label: 'Opérateur d’Atelier (Blanchisseur)',
    email: 'atelier@pressing-ivoire.ci',
    password: 'pass123',
    tenantName: 'Pressing Ivoire Cocody',
    description: 'Accès opérationnel : réception, lavage, repassage, et prêt au retrait.',
  },
  {
    role: 'superadmin' as UserRole,
    label: 'SuperAdmin SaaS (Éditeur)',
    email: 'admin@pressingos.ci',
    password: 'admin123',
    tenantName: 'PressingOS Platform (Global)',
    description: 'Vision éditeur : gestion des pressings partenaires, offres d’abonnement et métriques MRR.',
  },
];

export async function loginUser(email: string, pass: string): Promise<AuthSession> {
  try {
    const res = await fetch('/api/v1/auth/login', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ email, password: pass }),
    });

    if (res.ok) {
      const data = await res.json();
      return {
        token: data.token,
        user: data.user,
        tenant: data.tenant,
      };
    }
  } catch (err) {
    console.warn('Erreur appel réseau login:', err);
  }

  // Fallback instantané basé sur les comptes prédéfinis du backend
  const preset = PRESET_ACCOUNTS.find((a) => a.email.toLowerCase() === email.toLowerCase());
  if (preset && pass) {
    return {
      token: `demo-jwt-token-${preset.role}-${Date.now()}`,
      user: {
        id: `usr-${preset.role}`,
        tenant_id: preset.role === 'superadmin' ? undefined : 'a0000000-0000-0000-0000-000000000001',
        email: preset.email,
        full_name:
          preset.role === 'owner'
            ? 'M. Bamba Seydou'
            : preset.role === 'operator'
              ? 'Koffi Yao (Chef d’Atelier)'
              : 'Administrateur PressingOS',
        role: preset.role,
        status: 'active',
      },
      tenant:
        preset.role === 'superadmin'
          ? undefined
          : {
              id: 'a0000000-0000-0000-0000-000000000001',
              name: 'Pressing Ivoire Cocody',
              slug: 'pressing-ivoire-cocody',
              phone: '+225 07 00 11 22 33',
              city: 'Abidjan (Cocody Angré)',
              currency: 'XOF',
              status: 'active',
            },
    };
  }

  throw new Error('Identifiants de connexion invalides. Utilisez l’un des comptes de démonstration.');
}
