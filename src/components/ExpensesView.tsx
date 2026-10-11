import React, { useCallback, useEffect, useState } from 'react';
import { Coins, Loader2, Lock, Plus, RefreshCw, Trash2, TrendingDown, Wallet, X } from 'lucide-react';
import { User } from '../types';
import {
  CreateExpenseInput,
  EXPENSE_CATEGORIES,
  Expense,
  ExpensesPayload,
  categoryLabel,
  createExpense,
  deleteExpense,
  fetchExpenses,
  monthBounds,
} from '../lib/expensesApi';
import { formatXOF } from '../lib/adminApi';

interface ExpensesViewProps {
  currentUser: User;
  token?: string;
  showToast?: (message: string) => void;
}

const moisCourant = () => new Date().toISOString().slice(0, 7);

const FORM_VIDE = (): CreateExpenseInput => ({
  category: 'matieres_premieres',
  label: '',
  amount_xof: 0,
  payment_method: 'cash',
  spent_at: new Date().toISOString().slice(0, 10),
});

/**
 * ExpensesView — module Dépenses (fonctionnalité payante « expenses.advanced »).
 *
 * Suivi des charges par mois civil : total, répartition par poste, saisie et
 * suppression. Un établissement gratuit reçoit un refus 402 explicite invitant
 * à passer à une formule supérieure.
 */
export const ExpensesView: React.FC<ExpensesViewProps> = ({ currentUser, token, showToast }) => {
  const [mois, setMois] = useState<string>(moisCourant());
  const [payload, setPayload] = useState<ExpensesPayload | null>(null);
  const [loading, setLoading] = useState(true);
  const [fromApi, setFromApi] = useState(false);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [form, setForm] = useState<CreateExpenseInput>(FORM_VIDE);
  const [formError, setFormError] = useState<string | null>(null);
  const [planLocked, setPlanLocked] = useState(false);
  const [saving, setSaving] = useState(false);
  const [busyId, setBusyId] = useState<string | null>(null);

  const gestionnaire = ['owner', 'manager', 'superadmin'].includes(currentUser.role);

  const load = useCallback(async () => {
    setLoading(true);
    const data = await fetchExpenses(monthBounds(mois), token);
    if (data) {
      setPayload(data);
      setFromApi(true);
    }
    setLoading(false);
  }, [mois, token]);

  useEffect(() => {
    void load();
  }, [load]);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setFormError(null);
    if (!form.label.trim()) {
      setFormError('Le libellé de la dépense est obligatoire.');
      return;
    }
    if (!form.amount_xof || form.amount_xof <= 0) {
      setFormError('Le montant doit être supérieur à zéro.');
      return;
    }
    setSaving(true);
    const result = await createExpense(form, token);
    setSaving(false);

    if (!result.ok) {
      setPlanLocked(Boolean(result.planLocked));
      setFormError(result.error ?? 'enregistrement impossible');
      return;
    }
    setIsModalOpen(false);
    setForm(FORM_VIDE());
    showToast?.(`Dépense « ${form.label} » enregistrée`);
    await load();
  };

  const handleDelete = async (expense: Expense) => {
    setBusyId(expense.id);
    const result = await deleteExpense(expense.id, token);
    setBusyId(null);
    if (!result.ok) {
      showToast?.(result.error ?? 'suppression impossible');
      return;
    }
    showToast?.(`Dépense « ${expense.label} » supprimée`);
    await load();
  };

  const depenses = payload?.expenses ?? [];
  const total = payload?.total_xof ?? 0;
  const repartition = payload?.by_category ?? [];

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="flex items-center gap-2 text-lg font-semibold tracking-tight text-neutral-900 dark:text-neutral-100">
            <Wallet className="h-5 w-5 text-rose-500" />
            Dépenses
          </h1>
          <p className="mt-0.5 text-xs text-neutral-500 dark:text-neutral-400">
            Charges d'exploitation du pressing : matières, loyer, énergie, salaires…
          </p>
        </div>
        <div className="flex items-center gap-2">
          <input
            type="month"
            value={mois}
            onChange={(e) => setMois(e.target.value || moisCourant())}
            className="rounded-lg border border-neutral-200 bg-white px-3 py-2 text-xs text-neutral-700 outline-none focus:border-rose-400 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-200"
            aria-label="Mois"
          />
          <button
            type="button"
            onClick={() => void load()}
            className="inline-flex items-center gap-1.5 rounded-lg border border-neutral-200 bg-white px-3 py-2 text-xs font-medium text-neutral-700 transition-colors hover:bg-neutral-50 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-200 dark:hover:bg-neutral-800"
          >
            <RefreshCw className="h-3.5 w-3.5" />
            Actualiser
          </button>
          {gestionnaire && (
            <button
              type="button"
              onClick={() => {
                setFormError(null);
                setForm(FORM_VIDE());
                setIsModalOpen(true);
              }}
              className="inline-flex items-center gap-1.5 rounded-lg bg-rose-600 px-3 py-2 text-xs font-medium text-white transition-colors hover:bg-rose-700"
            >
              <Plus className="h-3.5 w-3.5" />
              Nouvelle dépense
            </button>
          )}
        </div>
      </div>

      {!fromApi && !loading && (
        <div className="rounded-2xl border border-amber-200 bg-amber-50 p-4 text-xs text-amber-800 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-300">
          API injoignable — les dépenses du mois ne peuvent pas être affichées.
        </div>
      )}

      <div className="grid gap-4 md:grid-cols-3">
        <div className="rounded-2xl border border-neutral-200 bg-white p-5 shadow-xs dark:border-neutral-800 dark:bg-neutral-900">
          <div className="flex items-center gap-2 text-[11px] font-medium uppercase tracking-wide text-neutral-500 dark:text-neutral-400">
            <TrendingDown className="h-3.5 w-3.5 text-rose-500" />
            Total du mois
          </div>
          <p className="mt-2 text-2xl font-semibold tracking-tight text-neutral-900 dark:text-neutral-100">
            {loading ? '…' : formatXOF(total)}
          </p>
          <p className="mt-1 text-[11px] text-neutral-500 dark:text-neutral-400">
            {depenses.length} dépense(s) enregistrée(s)
          </p>
        </div>

        <div className="rounded-2xl border border-neutral-200 bg-white p-5 shadow-xs md:col-span-2 dark:border-neutral-800 dark:bg-neutral-900">
          <div className="flex items-center gap-2 text-[11px] font-medium uppercase tracking-wide text-neutral-500 dark:text-neutral-400">
            <Coins className="h-3.5 w-3.5 text-amber-500" />
            Répartition par poste
          </div>
          {repartition.length === 0 ? (
            <p className="mt-3 text-xs text-neutral-500 dark:text-neutral-400">
              Aucune dépense sur la période.
            </p>
          ) : (
            <ul className="mt-3 space-y-2">
              {repartition.map((poste) => {
                const part = total > 0 ? Math.round((poste.total_xof / total) * 100) : 0;
                return (
                  <li key={poste.category} className="text-xs">
                    <div className="flex items-center justify-between text-neutral-700 dark:text-neutral-200">
                      <span>
                        {categoryLabel(poste.category)}{' '}
                        <span className="text-neutral-400 dark:text-neutral-500">({poste.count})</span>
                      </span>
                      <span className="font-medium">
                        {formatXOF(poste.total_xof)} · {part}%
                      </span>
                    </div>
                    <div className="mt-1 h-1.5 w-full overflow-hidden rounded-full bg-neutral-100 dark:bg-neutral-800">
                      <div className="h-full rounded-full bg-rose-400" style={{ width: `${part}%` }} />
                    </div>
                  </li>
                );
              })}
            </ul>
          )}
        </div>
      </div>

      <div className="overflow-hidden rounded-2xl border border-neutral-200 bg-white shadow-xs dark:border-neutral-800 dark:bg-neutral-900">
        {loading ? (
          <div className="flex items-center gap-2 p-5 text-sm text-neutral-500 dark:text-neutral-400">
            <Loader2 className="h-4 w-4 animate-spin" />
            Chargement des dépenses…
          </div>
        ) : depenses.length === 0 ? (
          <div className="p-8 text-center">
            <Wallet className="mx-auto h-8 w-8 text-neutral-300 dark:text-neutral-600" />
            <p className="mt-3 text-sm font-medium text-neutral-700 dark:text-neutral-200">
              Aucune dépense ce mois-ci
            </p>
            <p className="mt-1 text-xs text-neutral-500 dark:text-neutral-400">
              Enregistrez vos charges pour connaître votre marge réelle.
            </p>
          </div>
        ) : (
          <table className="w-full text-left text-sm">
            <thead className="border-b border-neutral-200 bg-neutral-50 text-[11px] uppercase tracking-wide text-neutral-500 dark:border-neutral-800 dark:bg-neutral-950/40 dark:text-neutral-400">
              <tr>
                <th className="px-4 py-3 font-medium">Date</th>
                <th className="px-4 py-3 font-medium">Libellé</th>
                <th className="px-4 py-3 font-medium">Poste</th>
                <th className="px-4 py-3 text-right font-medium">Montant</th>
                {gestionnaire && <th className="px-4 py-3 text-right font-medium">Action</th>}
              </tr>
            </thead>
            <tbody>
              {depenses.map((expense) => (
                <tr
                  key={expense.id}
                  className="border-b border-neutral-100 last:border-0 dark:border-neutral-800/60"
                >
                  <td className="px-4 py-3 text-xs text-neutral-500 dark:text-neutral-400">
                    {expense.spent_at}
                  </td>
                  <td className="px-4 py-3">
                    <div className="font-medium text-neutral-900 dark:text-neutral-100">{expense.label}</div>
                    {expense.note && (
                      <div className="text-[11px] text-neutral-500 dark:text-neutral-400">{expense.note}</div>
                    )}
                  </td>
                  <td className="px-4 py-3 text-xs text-neutral-600 dark:text-neutral-300">
                    {categoryLabel(expense.category)}
                  </td>
                  <td className="px-4 py-3 text-right font-medium text-neutral-900 dark:text-neutral-100">
                    {formatXOF(expense.amount_xof)}
                  </td>
                  {gestionnaire && (
                    <td className="px-4 py-3 text-right">
                      <button
                        type="button"
                        disabled={busyId === expense.id}
                        onClick={() => void handleDelete(expense)}
                        className="inline-flex items-center gap-1.5 rounded-lg border border-neutral-200 px-2.5 py-1.5 text-[11px] font-medium text-neutral-600 transition-colors hover:bg-neutral-50 disabled:opacity-50 dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800"
                      >
                        {busyId === expense.id ? (
                          <Loader2 className="h-3 w-3 animate-spin" />
                        ) : (
                          <Trash2 className="h-3 w-3" />
                        )}
                        Supprimer
                      </button>
                    </td>
                  )}
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {isModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-neutral-900/60 p-4 backdrop-blur-xs">
          <div className="w-full max-w-md rounded-2xl border border-neutral-200 bg-white p-5 shadow-xl dark:border-neutral-800 dark:bg-neutral-900">
            <div className="flex items-start justify-between">
              <div className="flex items-center gap-2">
                <Wallet className="h-5 w-5 text-rose-500" />
                <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
                  Nouvelle dépense
                </h2>
              </div>
              <button
                type="button"
                onClick={() => setIsModalOpen(false)}
                className="rounded-lg p-1 text-neutral-400 transition-colors hover:bg-neutral-100 hover:text-neutral-600 dark:hover:bg-neutral-800"
                aria-label="Fermer"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            {planLocked ? (
              <div className="mt-4 rounded-lg border border-amber-200 bg-amber-50 px-3 py-3 text-xs text-amber-800 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-300">
                <div className="flex items-center gap-1.5 font-medium">
                  <Lock className="h-3.5 w-3.5" />
                  Module réservé aux formules payantes
                </div>
                <p className="mt-1">
                  Le suivi des dépenses est inclus dans les formules PRO et BUSINESS. Passez à une formule
                  supérieure depuis « Mon abonnement » pour l'activer.
                </p>
              </div>
            ) : (
              <form onSubmit={handleCreate} className="mt-4 space-y-3">
                <div>
                  <label className="text-[11px] font-medium text-neutral-600 dark:text-neutral-300">
                    Libellé
                  </label>
                  <input
                    value={form.label}
                    onChange={(e) => setForm({ ...form, label: e.target.value })}
                    className="mt-1 w-full rounded-lg border border-neutral-200 bg-white px-3 py-2 text-sm text-neutral-900 outline-none focus:border-rose-400 dark:border-neutral-700 dark:bg-neutral-950 dark:text-neutral-100"
                    placeholder="Lessive en poudre (5 kg)"
                  />
                </div>
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="text-[11px] font-medium text-neutral-600 dark:text-neutral-300">
                      Montant (FCFA)
                    </label>
                    <input
                      type="number"
                      min={1}
                      value={form.amount_xof || ''}
                      onChange={(e) => setForm({ ...form, amount_xof: Number(e.target.value) })}
                      className="mt-1 w-full rounded-lg border border-neutral-200 bg-white px-3 py-2 text-sm text-neutral-900 outline-none focus:border-rose-400 dark:border-neutral-700 dark:bg-neutral-950 dark:text-neutral-100"
                      placeholder="25000"
                    />
                  </div>
                  <div>
                    <label className="text-[11px] font-medium text-neutral-600 dark:text-neutral-300">
                      Date
                    </label>
                    <input
                      type="date"
                      value={form.spent_at}
                      onChange={(e) => setForm({ ...form, spent_at: e.target.value })}
                      className="mt-1 w-full rounded-lg border border-neutral-200 bg-white px-3 py-2 text-sm text-neutral-900 outline-none focus:border-rose-400 dark:border-neutral-700 dark:bg-neutral-950 dark:text-neutral-100"
                    />
                  </div>
                </div>
                <div>
                  <label className="text-[11px] font-medium text-neutral-600 dark:text-neutral-300">
                    Poste de charge
                  </label>
                  <select
                    value={form.category}
                    onChange={(e) => setForm({ ...form, category: e.target.value })}
                    className="mt-1 w-full rounded-lg border border-neutral-200 bg-white px-3 py-2 text-sm text-neutral-900 outline-none focus:border-rose-400 dark:border-neutral-700 dark:bg-neutral-950 dark:text-neutral-100"
                  >
                    {EXPENSE_CATEGORIES.map((categorie) => (
                      <option key={categorie.value} value={categorie.value}>
                        {categorie.label}
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="text-[11px] font-medium text-neutral-600 dark:text-neutral-300">
                    Note (optionnel)
                  </label>
                  <input
                    value={form.note ?? ''}
                    onChange={(e) => setForm({ ...form, note: e.target.value })}
                    className="mt-1 w-full rounded-lg border border-neutral-200 bg-white px-3 py-2 text-sm text-neutral-900 outline-none focus:border-rose-400 dark:border-neutral-700 dark:bg-neutral-950 dark:text-neutral-100"
                    placeholder="Fournisseur, référence de facture…"
                  />
                </div>

                {formError && (
                  <div className="rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-xs text-rose-700 dark:border-rose-900/50 dark:bg-rose-950/30 dark:text-rose-300">
                    {formError}
                  </div>
                )}

                <div className="flex justify-end gap-2 pt-1">
                  <button
                    type="button"
                    onClick={() => setIsModalOpen(false)}
                    className="rounded-lg border border-neutral-200 px-3 py-2 text-xs font-medium text-neutral-600 transition-colors hover:bg-neutral-50 dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800"
                  >
                    Annuler
                  </button>
                  <button
                    type="submit"
                    disabled={saving}
                    className="inline-flex items-center gap-1.5 rounded-lg bg-rose-600 px-3 py-2 text-xs font-medium text-white transition-colors hover:bg-rose-700 disabled:opacity-60"
                  >
                    {saving ? (
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    ) : (
                      <Plus className="h-3.5 w-3.5" />
                    )}
                    Enregistrer
                  </button>
                </div>
              </form>
            )}
          </div>
        </div>
      )}
    </div>
  );
};
