import React, { useCallback, useEffect, useState } from 'react';
import {
  BadgeCheck,
  Building2,
  CircleSlash,
  Loader2,
  MapPin,
  Package,
  Phone,
  Plus,
  RefreshCw,
  X,
} from 'lucide-react';
import { User } from '../types';
import {
  Branch,
  CreateBranchInput,
  branchStatusLabel,
  createBranch,
  fetchBranches,
  updateBranchStatus,
} from '../lib/branchesApi';

interface BranchesViewProps {
  currentUser: User;
  token?: string;
  showToast?: (message: string) => void;
}

const FORM_VIDE: CreateBranchInput = { name: '', city: '', phone: '', address: '' };

/**
 * BranchesView — gestion des établissements (multi-sites).
 *
 * Fonctionnalité payante « branches.multi_site » : 1 site en gratuit et PRO,
 * 3 en BUSINESS. Le site principal (posé à l'inscription) est protégé : il ne
 * peut être ni suspendu ni archivé.
 */
export const BranchesView: React.FC<BranchesViewProps> = ({ currentUser, token, showToast }) => {
  const [branches, setBranches] = useState<Branch[]>([]);
  const [loading, setLoading] = useState(true);
  const [fromApi, setFromApi] = useState(false);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [form, setForm] = useState<CreateBranchInput>(FORM_VIDE);
  const [formError, setFormError] = useState<string | null>(null);
  const [quotaReached, setQuotaReached] = useState(false);
  const [saving, setSaving] = useState(false);

  const gestionnaire = ['owner', 'manager', 'superadmin'].includes(currentUser.role);

  const load = useCallback(async () => {
    setLoading(true);
    const data = await fetchBranches(token);
    if (data) {
      setBranches(data);
      setFromApi(true);
    }
    setLoading(false);
  }, [token]);

  useEffect(() => {
    void load();
  }, [load]);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setFormError(null);
    if (!form.name.trim()) {
      setFormError("Le nom de l'établissement est obligatoire.");
      return;
    }
    setSaving(true);
    const result = await createBranch(
      {
        name: form.name,
        city: form.city || undefined,
        phone: form.phone || undefined,
        address: form.address || undefined,
      },
      token
    );
    setSaving(false);

    if (!result.ok) {
      setQuotaReached(Boolean(result.quotaReached));
      setFormError(result.error ?? 'création impossible');
      return;
    }
    setIsModalOpen(false);
    setForm(FORM_VIDE);
    showToast?.(`Établissement « ${form.name} » créé`);
    await load();
  };

  const handleStatus = async (branch: Branch, status: 'active' | 'suspended' | 'archived') => {
    setBusyId(branch.id);
    const result = await updateBranchStatus(branch.id, status, token);
    setBusyId(null);
    if (!result.ok) {
      showToast?.(result.error ?? 'modification impossible');
      return;
    }
    showToast?.(`« ${branch.name} » — ${branchStatusLabel(status).toLowerCase()}`);
    await load();
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="flex items-center gap-2 text-lg font-semibold tracking-tight text-neutral-900 dark:text-neutral-100">
            <Building2 className="h-5 w-5 text-sky-500" />
            Établissements
          </h1>
          <p className="mt-0.5 text-xs text-neutral-500 dark:text-neutral-400">
            Vos sites physiques : chaque dépôt de linge est rattaché à un établissement.
          </p>
        </div>
        <div className="flex items-center gap-2">
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
                setQuotaReached(false);
                setForm(FORM_VIDE);
                setIsModalOpen(true);
              }}
              className="inline-flex items-center gap-1.5 rounded-lg bg-sky-600 px-3 py-2 text-xs font-medium text-white transition-colors hover:bg-sky-700"
            >
              <Plus className="h-3.5 w-3.5" />
              Nouvel établissement
            </button>
          )}
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <span className="inline-flex items-center gap-1.5 rounded-full border border-sky-200 bg-sky-50 px-2.5 py-1 text-[11px] font-medium text-sky-700 dark:border-sky-900/50 dark:bg-sky-950/30 dark:text-sky-300">
          <Building2 className="h-3 w-3" />
          {branches.length} établissement(s)
        </span>
        <span className="inline-flex items-center gap-1.5 rounded-full border border-neutral-200 bg-neutral-50 px-2.5 py-1 text-[11px] text-neutral-600 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-400">
          Plafond selon la formule : 1 site (Gratuit, PRO) · 3 (BUSINESS)
        </span>
      </div>

      {loading ? (
        <div className="flex items-center gap-2 rounded-2xl border border-neutral-200 bg-white p-5 text-sm text-neutral-500 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-400">
          <Loader2 className="h-4 w-4 animate-spin" />
          Chargement des établissements…
        </div>
      ) : !fromApi && branches.length === 0 ? (
        <div className="rounded-2xl border border-amber-200 bg-amber-50 p-5 text-sm text-amber-800 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-300">
          API injoignable — la liste des établissements est indisponible pour le moment.
        </div>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {branches.map((branch) => (
            <div
              key={branch.id}
              className="flex flex-col justify-between rounded-2xl border border-neutral-200 bg-white p-5 shadow-xs dark:border-neutral-800 dark:bg-neutral-900"
            >
              <div>
                <div className="flex items-start justify-between gap-2">
                  <div>
                    <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
                      {branch.name}
                    </h2>
                    <p className="mt-0.5 text-[11px] text-neutral-500 dark:text-neutral-400">
                      {branch.is_primary ? 'Site principal' : 'Site secondaire'}
                    </p>
                  </div>
                  <span
                    className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium ${
                      branch.status === 'active'
                        ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950/50 dark:text-emerald-300'
                        : 'bg-rose-100 text-rose-700 dark:bg-rose-950/50 dark:text-rose-300'
                    }`}
                  >
                    {branch.status === 'active' ? (
                      <BadgeCheck className="h-3 w-3" />
                    ) : (
                      <CircleSlash className="h-3 w-3" />
                    )}
                    {branchStatusLabel(branch.status)}
                  </span>
                </div>

                <dl className="mt-3 space-y-1.5 text-xs text-neutral-600 dark:text-neutral-300">
                  {branch.city && (
                    <div className="flex items-center gap-1.5">
                      <MapPin className="h-3.5 w-3.5 text-neutral-400" />
                      {branch.city}
                    </div>
                  )}
                  {branch.phone && (
                    <div className="flex items-center gap-1.5">
                      <Phone className="h-3.5 w-3.5 text-neutral-400" />
                      {branch.phone}
                    </div>
                  )}
                  <div className="flex items-center gap-1.5">
                    <Package className="h-3.5 w-3.5 text-neutral-400" />
                    {branch.orders_count} commande(s) enregistrée(s)
                  </div>
                </dl>
              </div>

              {gestionnaire && !branch.is_primary && (
                <div className="mt-4 flex gap-2">
                  {branch.status === 'active' ? (
                    <button
                      type="button"
                      disabled={busyId === branch.id}
                      onClick={() => void handleStatus(branch, 'suspended')}
                      className="inline-flex items-center gap-1.5 rounded-lg border border-rose-200 px-2.5 py-1.5 text-[11px] font-medium text-rose-700 transition-colors hover:bg-rose-50 disabled:opacity-50 dark:border-rose-900/50 dark:text-rose-300 dark:hover:bg-rose-950/30"
                    >
                      {busyId === branch.id ? (
                        <Loader2 className="h-3 w-3 animate-spin" />
                      ) : (
                        <CircleSlash className="h-3 w-3" />
                      )}
                      Suspendre
                    </button>
                  ) : (
                    <button
                      type="button"
                      disabled={busyId === branch.id}
                      onClick={() => void handleStatus(branch, 'active')}
                      className="inline-flex items-center gap-1.5 rounded-lg border border-emerald-200 px-2.5 py-1.5 text-[11px] font-medium text-emerald-700 transition-colors hover:bg-emerald-50 disabled:opacity-50 dark:border-emerald-900/50 dark:text-emerald-300 dark:hover:bg-emerald-950/30"
                    >
                      {busyId === branch.id ? (
                        <Loader2 className="h-3 w-3 animate-spin" />
                      ) : (
                        <BadgeCheck className="h-3 w-3" />
                      )}
                      Réactiver
                    </button>
                  )}
                </div>
              )}
              {branch.is_primary && (
                <p className="mt-4 text-[11px] text-neutral-400 dark:text-neutral-500">
                  Site d'ancrage : ne peut être ni suspendu ni archivé.
                </p>
              )}
            </div>
          ))}
        </div>
      )}

      {isModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-neutral-900/60 p-4 backdrop-blur-xs">
          <div className="w-full max-w-md rounded-2xl border border-neutral-200 bg-white p-5 shadow-xl dark:border-neutral-800 dark:bg-neutral-900">
            <div className="flex items-start justify-between">
              <div className="flex items-center gap-2">
                <Building2 className="h-5 w-5 text-sky-500" />
                <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
                  Nouvel établissement
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

            <form onSubmit={handleCreate} className="mt-4 space-y-3">
              <div>
                <label className="text-[11px] font-medium text-neutral-600 dark:text-neutral-300">
                  Nom du site
                </label>
                <input
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  className="mt-1 w-full rounded-lg border border-neutral-200 bg-white px-3 py-2 text-sm text-neutral-900 outline-none focus:border-sky-400 dark:border-neutral-700 dark:bg-neutral-950 dark:text-neutral-100"
                  placeholder="Pressing Yopougon"
                />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="text-[11px] font-medium text-neutral-600 dark:text-neutral-300">
                    Ville
                  </label>
                  <input
                    value={form.city}
                    onChange={(e) => setForm({ ...form, city: e.target.value })}
                    className="mt-1 w-full rounded-lg border border-neutral-200 bg-white px-3 py-2 text-sm text-neutral-900 outline-none focus:border-sky-400 dark:border-neutral-700 dark:bg-neutral-950 dark:text-neutral-100"
                    placeholder="Abidjan"
                  />
                </div>
                <div>
                  <label className="text-[11px] font-medium text-neutral-600 dark:text-neutral-300">
                    Téléphone
                  </label>
                  <input
                    value={form.phone}
                    onChange={(e) => setForm({ ...form, phone: e.target.value })}
                    className="mt-1 w-full rounded-lg border border-neutral-200 bg-white px-3 py-2 text-sm text-neutral-900 outline-none focus:border-sky-400 dark:border-neutral-700 dark:bg-neutral-950 dark:text-neutral-100"
                    placeholder="+225 …"
                  />
                </div>
              </div>
              <div>
                <label className="text-[11px] font-medium text-neutral-600 dark:text-neutral-300">
                  Adresse (optionnel)
                </label>
                <input
                  value={form.address}
                  onChange={(e) => setForm({ ...form, address: e.target.value })}
                  className="mt-1 w-full rounded-lg border border-neutral-200 bg-white px-3 py-2 text-sm text-neutral-900 outline-none focus:border-sky-400 dark:border-neutral-700 dark:bg-neutral-950 dark:text-neutral-100"
                  placeholder="Rue du marché, à côté de la pharmacie"
                />
              </div>

              {formError && (
                <div
                  className={`rounded-lg border px-3 py-2 text-xs ${
                    quotaReached
                      ? 'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-300'
                      : 'border-rose-200 bg-rose-50 text-rose-700 dark:border-rose-900/50 dark:bg-rose-950/30 dark:text-rose-300'
                  }`}
                >
                  {quotaReached ? `${formError} — la formule BUSINESS autorise 3 établissements.` : formError}
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
                  className="inline-flex items-center gap-1.5 rounded-lg bg-sky-600 px-3 py-2 text-xs font-medium text-white transition-colors hover:bg-sky-700 disabled:opacity-60"
                >
                  {saving ? (
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <Plus className="h-3.5 w-3.5" />
                  )}
                  Créer le site
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
