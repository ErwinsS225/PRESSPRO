/**
 * Module Établissements (multi-sites) — fonctionnalité payante « branches.multi_site ».
 *
 * Plafond de la formule : 1 site en gratuit et PRO, 3 en BUSINESS (l'essai
 * actif vaut BUSINESS). Lecture ouverte aux membres, écritures réservées aux
 * gérants.
 */

export interface Branch {
  id: string;
  tenant_id: string;
  name: string;
  code?: string;
  address?: string;
  phone?: string;
  city?: string;
  timezone?: string;
  is_primary: boolean;
  status: string;
  orders_count: number;
  created_at: string;
  updated_at: string;
}

export interface CreateBranchInput {
  name: string;
  code?: string;
  address?: string;
  phone?: string;
  city?: string;
}

function authHeaders(token?: string): Record<string, string> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    Accept: 'application/json',
  };
  if (token) headers['Authorization'] = `Bearer ${token}`;
  return headers;
}

export function branchStatusLabel(status: string): string {
  switch (status) {
    case 'active':
      return 'Actif';
    case 'suspended':
      return 'Suspendu';
    case 'archived':
      return 'Archivé';
    default:
      return status;
  }
}

/** Liste les établissements du pressing. `null` si l'API est injoignable. */
export async function fetchBranches(token?: string): Promise<Branch[] | null> {
  try {
    const res = await fetch('/api/v1/branches', { headers: authHeaders(token) });
    if (!res.ok) return null;
    const json = (await res.json()) as Branch[] | { branches?: Branch[] };
    return Array.isArray(json) ? json : (json.branches ?? []);
  } catch {
    return null;
  }
}

/** Crée un établissement secondaire (refus 402 si le plafond de la formule est atteint). */
export async function createBranch(
  input: CreateBranchInput,
  token?: string
): Promise<{ ok: boolean; branch?: Branch; error?: string; quotaReached?: boolean }> {
  try {
    const res = await fetch('/api/v1/branches', {
      method: 'POST',
      headers: authHeaders(token),
      body: JSON.stringify(input),
    });
    const body = (await res.json().catch(() => ({}))) as Branch & { error?: string };
    if (res.ok) return { ok: true, branch: body };
    return {
      ok: false,
      error: body.error ?? `erreur HTTP ${res.status}`,
      quotaReached: res.status === 402,
    };
  } catch (err) {
    return { ok: false, error: err instanceof Error ? err.message : 'réseau indisponible' };
  }
}

/** Suspend, réactive ou archive un site secondaire. */
export async function updateBranchStatus(
  branchId: string,
  status: 'active' | 'suspended' | 'archived',
  token?: string
): Promise<{ ok: boolean; branch?: Branch; error?: string }> {
  try {
    const res = await fetch(`/api/v1/branches/${branchId}`, {
      method: 'PATCH',
      headers: authHeaders(token),
      body: JSON.stringify({ status }),
    });
    const body = (await res.json().catch(() => ({}))) as Branch & { error?: string };
    if (res.ok) return { ok: true, branch: body };
    return { ok: false, error: body.error ?? `erreur HTTP ${res.status}` };
  } catch (err) {
    return { ok: false, error: err instanceof Error ? err.message : 'réseau indisponible' };
  }
}
