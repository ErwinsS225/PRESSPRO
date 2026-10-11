import { AuthSession, UserRole } from '../types';

export const PRESET_ACCOUNTS = [
  {
    role: 'owner' as UserRole,
    label: 'Propriétaire / Gérant',
    email: 'gerant@pressing-ivoire.ci',
    password: 'pass123',
    tenantName: 'Compte de démonstration',
    description:
      'Accès complet : tableau de bord, chiffre d’affaires, clients, tarifs et gestion des employés.',
  },
  {
    role: 'operator' as UserRole,
    label: 'Opérateur d’Atelier (Blanchisseur)',
    email: 'atelier@pressing-ivoire.ci',
    password: 'pass123',
    tenantName: 'Compte de démonstration',
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
  const res = await fetch('/api/v1/auth/login', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ email, password: pass }),
  }).catch((err) => {
    console.warn('Erreur appel réseau login:', err);
    throw new Error(
      'Serveur PressingOS injoignable : vérifiez la connexion puis réessayez (aucune session hors-ligne n’est créée).'
    );
  });

  if (res.status === 401 || res.status === 403) {
    throw new Error('Identifiants refusés par le serveur. Vérifiez votre e-mail et votre mot de passe.');
  }
  if (!res.ok) {
    throw new Error(`Connexion refusée par le serveur (HTTP ${res.status}).`);
  }

  const data = await res.json();
  // La session provient intégralement du serveur : aucun jeton, compte ni
  // établissement de démonstration n'est fabriqué côté navigateur.
  if (!data || typeof data.token !== 'string' || !data.user) {
    throw new Error('Réponse de connexion incomplète : session non établie.');
  }
  return {
    token: data.token,
    user: data.user,
    tenant: data.tenant || undefined,
  };
}
