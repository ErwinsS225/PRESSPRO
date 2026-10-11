/**
 * Espace SuperAdmin (éditeur SaaS) — appels réels à l'API Go.
 *
 * Toutes les données proviennent de PostgreSQL : indicateurs de la plateforme,
 * annuaire des établissements, et décisions de l'éditeur (suspension /
 * réactivation d'un abonnement).
 */

export interface PlatformStats {
  total_registered_pressings: number;
  total_users: number;
  active_subscriptions: number;
  trialing_subscriptions: number;
  monthly_recurring_mrr_xof: number;
  subscriptions_by_status: Record<string, number>;
  subscriptions_by_plan: Record<string, number>;
  /** "postgresql" (données persistées) ou "memoire" (mode démonstration). */
  data_source: string;
}

export interface TenantOverview {
  tenant_id: string;
  name: string;
  slug: string;
  city: string;
  phone: string;
  owner_email: string;
  plan_code: string;
  plan_name: string;
  price_monthly_xof: number;
  status: string;
  trial_ends_at?: string;
  created_at: string;
  active: boolean;
}

export interface RegisterTenantInput {
  tenant_name: string;
  city: string;
  phone: string;
  owner_full_name: string;
  owner_email: string;
  password: string;
}

function authHeaders(token?: string): Record<string, string> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    Accept: 'application/json',
  };
  if (token) headers['Authorization'] = `Bearer ${token}`;
  return headers;
}

/** Indicateurs consolidés de la plateforme. */
export async function fetchPlatformStats(token?: string): Promise<PlatformStats | null> {
  try {
    const res = await fetch('/api/v1/admin/stats', { headers: authHeaders(token) });
    if (!res.ok) return null;
    return (await res.json()) as PlatformStats;
  } catch {
    return null;
  }
}

/** Annuaire réel des établissements (abonnement + compte propriétaire). */
export async function fetchTenants(token?: string): Promise<TenantOverview[]> {
  try {
    const res = await fetch('/api/v1/admin/tenants', { headers: authHeaders(token) });
    if (!res.ok) return [];
    const json = (await res.json()) as { tenants?: TenantOverview[] };
    return json.tenants ?? [];
  } catch {
    return [];
  }
}

/** Suspend ou réactive l'abonnement d'un établissement (décision de l'éditeur). */
export async function setTenantStatus(
  tenantId: string,
  status: 'SUSPENDED' | 'ACTIVE',
  token?: string,
  reason?: string
): Promise<{ ok: boolean; error?: string }> {
  try {
    const res = await fetch(`/api/v1/admin/tenants/${tenantId}/status`, {
      method: 'PATCH',
      headers: authHeaders(token),
      body: JSON.stringify({ status, reason }),
    });
    if (res.ok) return { ok: true };
    const body = (await res.json().catch(() => ({}))) as { error?: string };
    return { ok: false, error: body.error ?? `erreur HTTP ${res.status}` };
  } catch (err) {
    return { ok: false, error: err instanceof Error ? err.message : 'réseau indisponible' };
  }
}

/** Inscrit un nouvel établissement partenaire (catalogue et grille créés d'office). */
export async function registerTenant(input: RegisterTenantInput): Promise<{ ok: boolean; error?: string }> {
  try {
    const res = await fetch('/api/v1/auth/register-tenant', {
      method: 'POST',
      headers: authHeaders(),
      body: JSON.stringify(input),
    });
    if (res.ok) return { ok: true };
    const body = (await res.json().catch(() => ({}))) as { error?: string };
    return { ok: false, error: body.error ?? `erreur HTTP ${res.status}` };
  } catch (err) {
    return { ok: false, error: err instanceof Error ? err.message : 'réseau indisponible' };
  }
}

/** Libellé lisible du statut d'abonnement. */
export function subscriptionLabel(status: string): string {
  switch (status.toUpperCase()) {
    case 'ACTIVE':
      return 'Actif';
    case 'TRIALING':
      return "Période d'essai";
    case 'PAST_DUE':
      return 'Impayé';
    case 'CANCELED':
      return 'Résilié';
    case 'SUSPENDED':
      return 'Suspendu';
    default:
      return status;
  }
}

export function formatXOF(amount: number): string {
  return `${Math.round(amount).toLocaleString('fr-FR')} FCFA`;
}
