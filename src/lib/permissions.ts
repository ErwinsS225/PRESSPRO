import { User } from '../types';

/**
 * Règle d'accès à l'Espace Éditeur SaaS (Super Administration Globale).
 *
 * Seuls les comptes avec le rôle `superadmin` (super-utilisateur de la plateforme
 * SaaS, distinct des rôles `owner`/`manager`/`cashier`/`operator` du tenant) peuvent
 * ouvrir cette vue. Contrairement au système de rôles de tenant, il ne s'agit
 * pas d'un attribut d'administration d'établissement : c'est l'identité globale
 * de l'éditeur du logiciel.
 */
export function canAccessSuperadmin(user: User | null | undefined): boolean {
  return Boolean(user && user.role === 'superadmin');
}
