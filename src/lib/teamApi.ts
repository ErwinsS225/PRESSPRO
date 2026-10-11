/**
 * Module Équipe — gestion des comptes employés de l'établissement.
 *
 * Fonctionnalité payante « users.max » : la création est refusée par l'API
 * (HTTP 402) lorsque le plafond de la formule est atteint. Les rôles proposés
 * sont ceux d'un pressing : gérant, caissier, opérateur (le rôle superadmin
 * de l'éditeur n'est jamais attribuable ici).
 */

export interface TeamMember {
  id: string;
  tenant_id?: string;
  email: string;
  full_name: string;
  phone?: string;
  role: string;
  status: string;
  created_at?: string;
  updated_at?: string;
}

export interface CreateMemberInput {
  full_name: string;
  email: string;
  phone?: string;
  role: string;
  password: string;
}

/** Rôles attribuables par un pressing (aligné sur le backend). */
export const TEAM_ROLES = [
  { value: 'manager', label: 'Gérant' },
  { value: 'cashier', label: 'Caissier' },
  { value: 'operator', label: 'Opérateur' },
] as const;

/** Établissement du pressing tel que vu depuis l'écran Équipe. */
export interface MemberBranch {
  id: string;
  name: string;
  is_primary: boolean;
  status: string;
  assigned: boolean;
}

const ROLE_LABELS: Record<string, string> = {
  owner: 'Propriétaire',
  manager: 'Gérant',
  cashier: 'Caissier',
  operator: 'Opérateur',
  superadmin: 'Superadmin',
};

export function roleLabel(role: string): string {
  return ROLE_LABELS[role] ?? role;
}

export function statusLabel(status: string): string {
  switch (status) {
    case 'active':
      return 'Actif';
    case 'inactive':
      return 'Désactivé';
    case 'locked':
      return 'Verrouillé';
    default:
      return status;
  }
}

function authHeaders(token?: string): Record<string, string> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    Accept: 'application/json',
  };
  if (token) headers['Authorization'] = `Bearer ${token}`;
  return headers;
}

/** Liste les comptes de l'établissement. `null` si l'API est injoignable. */
export async function fetchTeam(token?: string): Promise<TeamMember[] | null> {
  try {
    const res = await fetch('/api/v1/users', { headers: authHeaders(token) });
    if (!res.ok) return null;
    const json = (await res.json()) as TeamMember[] | { users?: TeamMember[] } | null;
    if (!json) return [];
    return Array.isArray(json) ? json : (json.users ?? []);
  } catch {
    return null;
  }
}

/** Crée un compte employé. Le message d'erreur de l'API est remonté tel quel. */
export async function createTeamMember(
  input: CreateMemberInput,
  token?: string
): Promise<{ ok: boolean; member?: TeamMember; error?: string; quotaReached?: boolean }> {
  try {
    const res = await fetch('/api/v1/users', {
      method: 'POST',
      headers: authHeaders(token),
      body: JSON.stringify(input),
    });
    const body = (await res.json().catch(() => ({}))) as TeamMember & { error?: string };
    if (res.ok) return { ok: true, member: body };
    return {
      ok: false,
      error: body.error ?? `erreur HTTP ${res.status}`,
      quotaReached: res.status === 402,
    };
  } catch (err) {
    return { ok: false, error: err instanceof Error ? err.message : 'réseau indisponible' };
  }
}

/** Active ou désactive un compte employé. */
export async function setTeamMemberStatus(
  memberId: string,
  status: 'active' | 'inactive',
  token?: string
): Promise<{ ok: boolean; error?: string }> {
  try {
    const res = await fetch(`/api/v1/users/${memberId}/status`, {
      method: 'PATCH',
      headers: authHeaders(token),
      body: JSON.stringify({ status }),
    });
    if (res.ok) return { ok: true };
    const body = (await res.json().catch(() => ({}))) as { error?: string };
    return { ok: false, error: body.error ?? `erreur HTTP ${res.status}` };
  } catch (err) {
    return { ok: false, error: err instanceof Error ? err.message : 'réseau indisponible' };
  }
}

/** Libellé du statut d'un établissement dans le sélecteur d'affectation. */
export function branchStatusLabel(status: string): string {
  return status === 'archived' ? 'archivé' : 'actif';
}

/** Liste les établissements du pressing et l'affectation du compte. */
export async function fetchMemberBranches(
  memberId: string,
  token?: string
): Promise<{ ok: boolean; branches?: MemberBranch[]; error?: string }> {
  try {
    const res = await fetch(`/api/v1/users/${memberId}/branches`, { headers: authHeaders(token) });
    const body = (await res.json().catch(() => ({}))) as {
      branches?: MemberBranch[];
      error?: string;
    };
    if (!res.ok) return { ok: false, error: body.error ?? `erreur HTTP ${res.status}` };
    return { ok: true, branches: body.branches ?? [] };
  } catch (err) {
    return { ok: false, error: err instanceof Error ? err.message : 'réseau indisponible' };
  }
}

/**
 * Remplace l'affectation du compte. Un tableau vide est refusé par l'API
 * (l'employé doit toujours avoir accès à au moins un établissement) : l'écran
 * Équipe empêche donc de tout décocher.
 */
export async function setMemberBranches(
  memberId: string,
  branchIds: string[],
  token?: string
): Promise<{ ok: boolean; branches?: MemberBranch[]; error?: string }> {
  try {
    const res = await fetch(`/api/v1/users/${memberId}/branches`, {
      method: 'PUT',
      headers: authHeaders(token),
      body: JSON.stringify({ branch_ids: branchIds }),
    });
    const body = (await res.json().catch(() => ({}))) as {
      branches?: MemberBranch[];
      error?: string;
    };
    if (!res.ok) return { ok: false, error: body.error ?? `erreur HTTP ${res.status}` };
    return { ok: true, branches: body.branches ?? [] };
  } catch (err) {
    return { ok: false, error: err instanceof Error ? err.message : 'réseau indisponible' };
  }
}
