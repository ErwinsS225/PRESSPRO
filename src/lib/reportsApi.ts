/**
 * Module Rapports — fonctionnalité payante « reports.advanced ».
 *
 * Agrégats réels calculés côté serveur : chiffre d'affaires encaissé,
 * commandes, dépenses par poste, marge et meilleures prestations.
 * La consolidation par établissement dépend de « reports.consolidated »
 * (formule BUSINESS).
 */

export interface ReportPeriod {
  from: string;
  to: string;
}

export interface ReportServiceTotal {
  label: string;
  count: number;
  revenue_xof: number;
}

export interface ReportBranchTotal {
  branch_id: string;
  name: string;
  orders_count: number;
  revenue_xof: number;
  expenses_xof: number;
  margin_xof: number;
}

export interface ReportExpenseCategory {
  category: string;
  total_xof: number;
  count: number;
}

export interface ReportSummary {
  period: ReportPeriod;
  data_source: string;
  revenue_xof: number;
  orders_count: number;
  orders_by_status: Record<string, number>;
  average_ticket_xof: number;
  expenses_xof: number;
  margin_xof: number;
  expenses_by_category: ReportExpenseCategory[];
  top_services: ReportServiceTotal[];
  consolidated: boolean;
  by_branch?: ReportBranchTotal[];
  plan_hint?: string;
}

function authHeaders(token?: string): Record<string, string> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    Accept: 'application/json',
  };
  if (token) headers['Authorization'] = `Bearer ${token}`;
  return headers;
}

/** Charge le rapport d'activité de la période (dates incluses, AAAA-MM-JJ). */
export async function fetchReportSummary(
  period: { from: string; to: string },
  token?: string
): Promise<{ ok: boolean; report?: ReportSummary; error?: string; planLocked?: boolean }> {
  const params = new URLSearchParams({ from: period.from, to: period.to });
  try {
    const res = await fetch(`/api/v1/reports/summary?${params.toString()}`, { headers: authHeaders(token) });
    const body = (await res.json().catch(() => ({}))) as ReportSummary & { error?: string };
    if (res.ok) return { ok: true, report: body };
    return {
      ok: false,
      error: body.error ?? `erreur HTTP ${res.status}`,
      planLocked: res.status === 402,
    };
  } catch (err) {
    return { ok: false, error: err instanceof Error ? err.message : 'réseau indisponible' };
  }
}

/** Bornes de la période courante : « ce mois », « mois dernier » ou « 30 jours ». */
export function periodBounds(kind: 'mois' | 'mois_dernier' | 'trente_jours'): ReportPeriod {
  const aujourdhui = new Date();
  const iso = (d: Date) => d.toISOString().slice(0, 10);

  if (kind === 'mois') {
    const debut = new Date(Date.UTC(aujourdhui.getUTCFullYear(), aujourdhui.getUTCMonth(), 1));
    return { from: iso(debut), to: iso(aujourdhui) };
  }
  if (kind === 'mois_dernier') {
    const debut = new Date(Date.UTC(aujourdhui.getUTCFullYear(), aujourdhui.getUTCMonth() - 1, 1));
    const fin = new Date(Date.UTC(aujourdhui.getUTCFullYear(), aujourdhui.getUTCMonth(), 0));
    return { from: iso(debut), to: iso(fin) };
  }
  const debut = new Date(aujourdhui.getTime() - 29 * 24 * 3600 * 1000);
  return { from: iso(debut), to: iso(aujourdhui) };
}

/** Libellé lisible d'un statut de commande. */
export function orderStatusLabel(status: string): string {
  switch (status) {
    case 'received':
      return 'Reçue';
    case 'washing':
      return 'En lavage';
    case 'ironing':
      return 'Repassage';
    case 'ready':
      return 'Prête';
    case 'delivered':
      return 'Livrée';
    case 'cancelled':
      return 'Annulée';
    default:
      return status;
  }
}
