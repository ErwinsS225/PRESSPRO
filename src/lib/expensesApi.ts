/**
 * Module Dépenses — fonctionnalité payante « expenses.advanced ».
 *
 * Suivi des charges d'exploitation (matières premières, loyer, énergie,
 * salaires…), par mois et par établissement. Les écritures sont réservées aux
 * gérants et refusées en 402 lorsque la formule ne l'inclut pas.
 */

export interface Expense {
  id: string;
  tenant_id: string;
  branch_id?: string;
  category: string;
  label: string;
  amount_xof: number;
  payment_method?: string;
  spent_at: string;
  note?: string;
  created_by?: string;
  created_at: string;
  updated_at: string;
}

export interface ExpenseCategoryTotal {
  category: string;
  total_xof: number;
  count: number;
}

export interface ExpensesPayload {
  expenses: Expense[];
  count: number;
  total_xof: number;
  by_category: ExpenseCategoryTotal[];
  period: { from: string; to: string };
}

export interface CreateExpenseInput {
  category: string;
  label: string;
  amount_xof: number;
  payment_method?: string;
  spent_at?: string;
  branch_id?: string;
  note?: string;
}

/** Référentiel des postes de charge (clés alignées sur le backend). */
export const EXPENSE_CATEGORIES = [
  { value: 'matieres_premieres', label: 'Matières premières' },
  { value: 'produits_entretien', label: 'Produits d’entretien' },
  { value: 'loyer', label: 'Loyer' },
  { value: 'energie_eau', label: 'Énergie & eau' },
  { value: 'salaires', label: 'Salaires' },
  { value: 'transport', label: 'Transport' },
  { value: 'equipement', label: 'Équipement' },
  { value: 'maintenance', label: 'Maintenance' },
  { value: 'taxes', label: 'Taxes & impôts' },
  { value: 'divers', label: 'Divers' },
] as const;

export function categoryLabel(category: string): string {
  return EXPENSE_CATEGORIES.find((c) => c.value === category)?.label ?? category;
}

function authHeaders(token?: string): Record<string, string> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    Accept: 'application/json',
  };
  if (token) headers['Authorization'] = `Bearer ${token}`;
  return headers;
}

/** Bornes du mois « AAAA-MM » (incluses). */
export function monthBounds(month: string): { from: string; to: string } {
  const [annee, mois] = month.split('-').map((v) => parseInt(v, 10));
  const dernierJour = new Date(Date.UTC(annee, mois, 0)).getUTCDate();
  return { from: `${month}-01`, to: `${month}-${String(dernierJour).padStart(2, '0')}` };
}

/** Liste les dépenses d'une période (défaut : mois courant côté API). */
export async function fetchExpenses(
  period: { from?: string; to?: string; category?: string },
  token?: string
): Promise<ExpensesPayload | null> {
  const params = new URLSearchParams();
  if (period.from) params.set('from', period.from);
  if (period.to) params.set('to', period.to);
  if (period.category) params.set('category', period.category);
  const query = params.toString();
  try {
    const res = await fetch(`/api/v1/expenses${query ? `?${query}` : ''}`, { headers: authHeaders(token) });
    if (!res.ok) return null;
    return (await res.json()) as ExpensesPayload;
  } catch {
    return null;
  }
}

/** Enregistre une dépense (gérant / propriétaire). */
export async function createExpense(
  input: CreateExpenseInput,
  token?: string,
  branchId?: string
): Promise<{ ok: boolean; expense?: Expense; error?: string; planLocked?: boolean }> {
  try {
    const res = await fetch('/api/v1/expenses', {
      method: 'POST',
      headers: { ...authHeaders(token), ...(branchId ? { 'X-Branch-ID': branchId } : {}) },
      body: JSON.stringify(input),
    });
    const body = (await res.json().catch(() => ({}))) as Expense & { error?: string };
    if (res.ok) return { ok: true, expense: body };
    return {
      ok: false,
      error: body.error ?? `erreur HTTP ${res.status}`,
      planLocked: res.status === 402,
    };
  } catch (err) {
    return { ok: false, error: err instanceof Error ? err.message : 'réseau indisponible' };
  }
}

/** Supprime une dépense saisie par erreur. */
export async function deleteExpense(
  expenseId: string,
  token?: string
): Promise<{ ok: boolean; error?: string }> {
  try {
    const res = await fetch(`/api/v1/expenses/${expenseId}`, {
      method: 'DELETE',
      headers: authHeaders(token),
    });
    if (res.ok) return { ok: true };
    const body = (await res.json().catch(() => ({}))) as { error?: string };
    return { ok: false, error: body.error ?? `erreur HTTP ${res.status}` };
  } catch (err) {
    return { ok: false, error: err instanceof Error ? err.message : 'réseau indisponible' };
  }
}
