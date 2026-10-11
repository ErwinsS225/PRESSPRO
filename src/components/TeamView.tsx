import React, { useCallback, useEffect, useState } from 'react';
import {
  BadgeCheck,
  CircleSlash,
  Loader2,
  Plus,
  RefreshCw,
  ShieldAlert,
  UserPlus,
  Users,
  WifiOff,
  X,
} from 'lucide-react';
import { User } from '../types';
import {
  CreateMemberInput,
  TEAM_ROLES,
  TeamMember,
  createTeamMember,
  fetchTeam,
  roleLabel,
  setTeamMemberStatus,
  statusLabel,
} from '../lib/teamApi';

interface TeamViewProps {
  currentUser: User;
  token?: string;
  showToast?: (message: string) => void;
}

const FORM_VIDE: CreateMemberInput = {
  full_name: '',
  email: '',
  phone: '',
  role: 'cashier',
  password: '',
};

/**
 * TeamView — module Équipe (fonctionnalité payante « users.max »).
 *
 * Permet au propriétaire ou au gérant de créer des comptes employés, de les
 * activer ou de les désactiver. Le plafond de la formule est rappelé et
 * l'API répond 402 (formule supérieure requise) lorsqu'il est atteint.
 */
export const TeamView: React.FC<TeamViewProps> = ({ currentUser, token, showToast }) => {
  const [members, setMembers] = useState<TeamMember[]>([]);
  const [loading, setLoading] = useState(true);
  const [fromApi, setFromApi] = useState(false);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [form, setForm] = useState<CreateMemberInput>(FORM_VIDE);
  const [formError, setFormError] = useState<string | null>(null);
  const [quotaReached, setQuotaReached] = useState(false);
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    const data = await fetchTeam(token);
    if (data) {
      setMembers(data);
      setFromApi(true);
    }
    setLoading(false);
  }, [token]);

  useEffect(() => {
    void load();
  }, [load]);

  const actifs = members.filter((m) => m.status === 'active').length;

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setFormError(null);
    if (!form.full_name.trim() || !form.email.trim() || form.password.length < 6) {
      setFormError('Nom, email et mot de passe (6 caractères minimum) sont obligatoires.');
      return;
    }
    setSaving(true);
    const result = await createTeamMember({ ...form, phone: form.phone?.trim() || undefined }, token);
    setSaving(false);

    if (!result.ok) {
      setQuotaReached(Boolean(result.quotaReached));
      setFormError(result.error ?? 'création impossible');
      return;
    }
    setIsModalOpen(false);
    setForm(FORM_VIDE);
    setQuotaReached(false);
    showToast?.(`Compte créé pour ${form.full_name} (${roleLabel(form.role)})`);
    await load();
  };

  const handleToggleStatus = async (member: TeamMember) => {
    const cible = member.status === 'active' ? 'inactive' : 'active';
    setBusyId(member.id);
    const result = await setTeamMemberStatus(member.id, cible, token);
    setBusyId(null);
    if (!result.ok) {
      showToast?.(result.error ?? 'modification impossible');
      return;
    }
    showToast?.(
      cible === 'active'
        ? `Compte de ${member.full_name} réactivé`
        : `Compte de ${member.full_name} désactivé — connexion refusée`
    );
    await load();
  };

  const modifiable = (member: TeamMember) =>
    member.id !== currentUser.id && member.role !== 'owner' && member.role !== 'superadmin';

  return (
    <div className="space-y-6">
      {/* En-tête */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="flex items-center gap-2 text-lg font-semibold tracking-tight text-neutral-900 dark:text-neutral-100">
            <Users className="h-5 w-5 text-violet-500" />
            Équipe
          </h1>
          <p className="mt-0.5 text-xs text-neutral-500 dark:text-neutral-400">
            Comptes employés de votre pressing : création, activation et désactivation.
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
          <button
            type="button"
            onClick={() => {
              setFormError(null);
              setQuotaReached(false);
              setForm(FORM_VIDE);
              setIsModalOpen(true);
            }}
            className="inline-flex items-center gap-1.5 rounded-lg bg-violet-600 px-3 py-2 text-xs font-medium text-white transition-colors hover:bg-violet-700"
          >
            <Plus className="h-3.5 w-3.5" />
            Nouvel employé
          </button>
        </div>
      </div>

      {/* Rappel du plafond + état de la source */}
      <div className="flex flex-wrap items-center gap-3">
        <span className="inline-flex items-center gap-1.5 rounded-full border border-violet-200 bg-violet-50 px-2.5 py-1 text-[11px] font-medium text-violet-700 dark:border-violet-900/50 dark:bg-violet-950/30 dark:text-violet-300">
          <Users className="h-3 w-3" />
          {actifs} compte(s) actif(s) sur {members.length}
        </span>
        <span className="inline-flex items-center gap-1.5 rounded-full border border-neutral-200 bg-neutral-50 px-2.5 py-1 text-[11px] text-neutral-600 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-400">
          Plafond selon la formule : 2 (Gratuit) · 5 (PRO) · 15 (BUSINESS)
        </span>
        {!fromApi && !loading && (
          <span className="inline-flex items-center gap-1.5 rounded-full border border-amber-200 bg-amber-50 px-2.5 py-1 text-[11px] font-medium text-amber-700 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-300">
            <WifiOff className="h-3 w-3" />
            API injoignable — liste indisponible
          </span>
        )}
      </div>

      {/* Liste */}
      <div className="overflow-hidden rounded-2xl border border-neutral-200 bg-white shadow-xs dark:border-neutral-800 dark:bg-neutral-900">
        {loading ? (
          <div className="flex items-center gap-2 p-5 text-sm text-neutral-500 dark:text-neutral-400">
            <Loader2 className="h-4 w-4 animate-spin" />
            Chargement de l'équipe…
          </div>
        ) : members.length === 0 ? (
          <div className="p-8 text-center">
            <Users className="mx-auto h-8 w-8 text-neutral-300 dark:text-neutral-600" />
            <p className="mt-3 text-sm font-medium text-neutral-700 dark:text-neutral-200">
              Aucun compte employé
            </p>
            <p className="mt-1 text-xs text-neutral-500 dark:text-neutral-400">
              Créez un compte pour que vos caissiers et opérateurs accèdent au pressing.
            </p>
          </div>
        ) : (
          <table className="w-full text-left text-sm">
            <thead className="border-b border-neutral-200 bg-neutral-50 text-[11px] uppercase tracking-wide text-neutral-500 dark:border-neutral-800 dark:bg-neutral-950/40 dark:text-neutral-400">
              <tr>
                <th className="px-4 py-3 font-medium">Employé</th>
                <th className="px-4 py-3 font-medium">Rôle</th>
                <th className="px-4 py-3 font-medium">Statut</th>
                <th className="px-4 py-3 text-right font-medium">Action</th>
              </tr>
            </thead>
            <tbody>
              {members.map((member) => (
                <tr
                  key={member.id}
                  className="border-b border-neutral-100 last:border-0 dark:border-neutral-800/60"
                >
                  <td className="px-4 py-3">
                    <div className="font-medium text-neutral-900 dark:text-neutral-100">
                      {member.full_name}
                      {member.id === currentUser.id && (
                        <span className="ml-2 rounded-full bg-neutral-100 px-2 py-0.5 text-[10px] font-medium text-neutral-600 dark:bg-neutral-800 dark:text-neutral-300">
                          vous
                        </span>
                      )}
                    </div>
                    <div className="text-xs text-neutral-500 dark:text-neutral-400">{member.email}</div>
                  </td>
                  <td className="px-4 py-3 text-xs text-neutral-600 dark:text-neutral-300">
                    {roleLabel(member.role)}
                  </td>
                  <td className="px-4 py-3">
                    <span
                      className={`inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium ${
                        member.status === 'active'
                          ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950/50 dark:text-emerald-300'
                          : 'bg-rose-100 text-rose-700 dark:bg-rose-950/50 dark:text-rose-300'
                      }`}
                    >
                      {member.status === 'active' ? (
                        <BadgeCheck className="h-3 w-3" />
                      ) : (
                        <CircleSlash className="h-3 w-3" />
                      )}
                      {statusLabel(member.status)}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-right">
                    {modifiable(member) ? (
                      <button
                        type="button"
                        disabled={busyId === member.id}
                        onClick={() => void handleToggleStatus(member)}
                        className={`inline-flex items-center gap-1.5 rounded-lg border px-2.5 py-1.5 text-[11px] font-medium transition-colors disabled:opacity-50 ${
                          member.status === 'active'
                            ? 'border-rose-200 text-rose-700 hover:bg-rose-50 dark:border-rose-900/50 dark:text-rose-300 dark:hover:bg-rose-950/30'
                            : 'border-emerald-200 text-emerald-700 hover:bg-emerald-50 dark:border-emerald-900/50 dark:text-emerald-300 dark:hover:bg-emerald-950/30'
                        }`}
                      >
                        {busyId === member.id ? (
                          <Loader2 className="h-3 w-3 animate-spin" />
                        ) : (
                          <ShieldAlert className="h-3 w-3" />
                        )}
                        {member.status === 'active' ? 'Désactiver' : 'Réactiver'}
                      </button>
                    ) : (
                      <span className="text-[11px] text-neutral-400 dark:text-neutral-500">
                        {member.id === currentUser.id ? 'compte courant' : 'compte protégé'}
                      </span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Modal de création */}
      {isModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-neutral-900/60 p-4 backdrop-blur-xs">
          <div className="w-full max-w-md rounded-2xl border border-neutral-200 bg-white p-5 shadow-xl dark:border-neutral-800 dark:bg-neutral-900">
            <div className="flex items-start justify-between">
              <div className="flex items-center gap-2">
                <UserPlus className="h-5 w-5 text-violet-500" />
                <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
                  Nouvel employé
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
                  Nom complet
                </label>
                <input
                  value={form.full_name}
                  onChange={(e) => setForm({ ...form, full_name: e.target.value })}
                  className="mt-1 w-full rounded-lg border border-neutral-200 bg-white px-3 py-2 text-sm text-neutral-900 outline-none focus:border-violet-400 dark:border-neutral-700 dark:bg-neutral-950 dark:text-neutral-100"
                  placeholder="Awa Koné"
                />
              </div>
              <div>
                <label className="text-[11px] font-medium text-neutral-600 dark:text-neutral-300">
                  Email
                </label>
                <input
                  type="email"
                  value={form.email}
                  onChange={(e) => setForm({ ...form, email: e.target.value })}
                  className="mt-1 w-full rounded-lg border border-neutral-200 bg-white px-3 py-2 text-sm text-neutral-900 outline-none focus:border-violet-400 dark:border-neutral-700 dark:bg-neutral-950 dark:text-neutral-100"
                  placeholder="awa@pressing-ivoire.ci"
                />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="text-[11px] font-medium text-neutral-600 dark:text-neutral-300">
                    Téléphone (optionnel)
                  </label>
                  <input
                    value={form.phone}
                    onChange={(e) => setForm({ ...form, phone: e.target.value })}
                    className="mt-1 w-full rounded-lg border border-neutral-200 bg-white px-3 py-2 text-sm text-neutral-900 outline-none focus:border-violet-400 dark:border-neutral-700 dark:bg-neutral-950 dark:text-neutral-100"
                    placeholder="+225 …"
                  />
                </div>
                <div>
                  <label className="text-[11px] font-medium text-neutral-600 dark:text-neutral-300">
                    Rôle
                  </label>
                  <select
                    value={form.role}
                    onChange={(e) => setForm({ ...form, role: e.target.value })}
                    className="mt-1 w-full rounded-lg border border-neutral-200 bg-white px-3 py-2 text-sm text-neutral-900 outline-none focus:border-violet-400 dark:border-neutral-700 dark:bg-neutral-950 dark:text-neutral-100"
                  >
                    {TEAM_ROLES.map((role) => (
                      <option key={role.value} value={role.value}>
                        {role.label}
                      </option>
                    ))}
                  </select>
                </div>
              </div>
              <div>
                <label className="text-[11px] font-medium text-neutral-600 dark:text-neutral-300">
                  Mot de passe (6 caractères minimum)
                </label>
                <input
                  type="password"
                  value={form.password}
                  onChange={(e) => setForm({ ...form, password: e.target.value })}
                  className="mt-1 w-full rounded-lg border border-neutral-200 bg-white px-3 py-2 text-sm text-neutral-900 outline-none focus:border-violet-400 dark:border-neutral-700 dark:bg-neutral-950 dark:text-neutral-100"
                  placeholder="••••••••"
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
                  {quotaReached
                    ? `${formError} — passez à une formule supérieure pour ajouter d'autres employés.`
                    : formError}
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
                  className="inline-flex items-center gap-1.5 rounded-lg bg-violet-600 px-3 py-2 text-xs font-medium text-white transition-colors hover:bg-violet-700 disabled:opacity-60"
                >
                  {saving ? (
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <Plus className="h-3.5 w-3.5" />
                  )}
                  Créer le compte
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
