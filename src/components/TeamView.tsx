import React, { useCallback, useEffect, useState } from 'react';
import {
  BadgeCheck,
  Building2,
  CircleSlash,
  Loader2,
  Plus,
  RefreshCw,
  ShieldAlert,
  Store,
  UserPlus,
  Users,
  WifiOff,
  X,
} from 'lucide-react';
import { User } from '../types';
import {
  CreateMemberInput,
  MemberBranch,
  TEAM_ROLES,
  TeamMember,
  branchStatusLabel,
  createTeamMember,
  fetchMemberBranches,
  fetchTeam,
  roleLabel,
  setMemberBranches,
  setTeamMemberStatus,
  statusLabel,
} from '../lib/teamApi';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';

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

  // Affectation d'un employé à un ou plusieurs établissements.
  const [affectationPour, setAffectationPour] = useState<TeamMember | null>(null);
  const [sites, setSites] = useState<MemberBranch[]>([]);
  const [selectionSites, setSelectionSites] = useState<string[]>([]);
  const [chargementSites, setChargementSites] = useState(false);
  const [enregistrementSites, setEnregistrementSites] = useState(false);
  const [erreurSites, setErreurSites] = useState<string | null>(null);

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

  /**
   * Règle alignée sur le backend : seul un propriétaire peut verrouiller ou
   * réactiver un compte propriétaire. Un gérant voit ces lignes protégées.
   */
  const modifiable = (member: TeamMember) => {
    if (member.id === currentUser.id || member.role === 'superadmin') return false;
    if (member.role === 'owner') return currentUser.role === 'owner';
    return true;
  };

  /** Ouvre le sélecteur d'établissements d'un employé et charge son affectation. */
  const ouvrirAffectation = async (member: TeamMember) => {
    setAffectationPour(member);
    setErreurSites(null);
    setSites([]);
    setSelectionSites([]);
    setChargementSites(true);
    const result = await fetchMemberBranches(member.id, token);
    setChargementSites(false);
    if (!result.ok) {
      setErreurSites(result.error ?? 'affectation indisponible');
      return;
    }
    const liste = result.branches ?? [];
    setSites(liste);
    setSelectionSites(liste.filter((site) => site.assigned).map((site) => site.id));
  };

  const basculerSite = (siteId: string) => {
    setSelectionSites((courante) =>
      courante.includes(siteId) ? courante.filter((id) => id !== siteId) : [...courante, siteId]
    );
  };

  /** Enregistre l'affectation : l'employé ne pourra agir que sur ces sites. */
  const enregistrerAffectation = async () => {
    if (!affectationPour) return;
    if (selectionSites.length === 0) {
      setErreurSites('Sélectionnez au moins un établissement : un employé sans site ne peut rien saisir.');
      return;
    }
    setEnregistrementSites(true);
    setErreurSites(null);
    const result = await setMemberBranches(affectationPour.id, selectionSites, token);
    setEnregistrementSites(false);
    if (!result.ok) {
      setErreurSites(result.error ?? 'enregistrement impossible');
      return;
    }
    const liste = result.branches ?? [];
    setSites(liste);
    setSelectionSites(liste.filter((site) => site.assigned).map((site) => site.id));
    const noms = liste.filter((site) => site.assigned).map((site) => site.name);
    showToast?.(
      noms.length === 1
        ? `${affectationPour.full_name} agit désormais sur « ${noms[0]} »`
        : `${affectationPour.full_name} agit désormais sur ${noms.length} établissements`
    );
    setAffectationPour(null);
  };

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
            className="inline-flex items-center gap-1.5 rounded-lg border border-neutral-200 bg-white px-3 py-2 text-xs font-medium text-neutral-700 transition-colors hover:bg-neutral-900/[0.06] dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-200 dark:hover:bg-neutral-800 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated"
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
        <span className="inline-flex items-center gap-1.5 rounded-full border border-neutral-200 bg-neutral-50 px-2.5 py-1 text-[11px] text-neutral-600 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-400 ps-elevated-static">
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
          <Table className="w-full text-left text-sm">
            <TableHeader className="border-b border-neutral-200 bg-neutral-50 text-[11px] uppercase tracking-wide text-neutral-500 dark:border-neutral-800 dark:bg-neutral-950/40 dark:text-neutral-400">
              <TableRow>
                <TableHead className="px-4 py-3 font-medium">Employé</TableHead>
                <TableHead className="px-4 py-3 font-medium">Rôle</TableHead>
                <TableHead className="px-4 py-3 font-medium">Statut</TableHead>
                <TableHead className="px-4 py-3 font-medium">Établissements</TableHead>
                <TableHead className="px-4 py-3 text-right font-medium">Action</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {members.map((member) => (
                <TableRow
                  key={member.id}
                  className="border-b border-neutral-100 last:border-0 dark:border-neutral-800/60"
                >
                  <TableCell className="px-4 py-3">
                    <div className="font-medium text-neutral-900 dark:text-neutral-100">
                      {member.full_name}
                      {member.id === currentUser.id && (
                        <span className="ml-2 rounded-full bg-neutral-100 px-2 py-0.5 text-[10px] font-medium text-neutral-600 dark:bg-neutral-800 dark:text-neutral-300">
                          vous
                        </span>
                      )}
                    </div>
                    <div className="text-xs text-neutral-500 dark:text-neutral-400">{member.email}</div>
                  </TableCell>
                  <TableCell className="px-4 py-3 text-xs text-neutral-600 dark:text-neutral-300">
                    {roleLabel(member.role)}
                  </TableCell>
                  <TableCell className="px-4 py-3">
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
                  </TableCell>
                  <TableCell className="px-4 py-3">
                    <button
                      type="button"
                      onClick={() => void ouvrirAffectation(member)}
                      className="inline-flex items-center gap-1.5 rounded-lg border border-neutral-200 px-2.5 py-1.5 text-[11px] font-medium text-neutral-700 transition-colors hover:bg-neutral-900/[0.06] dark:border-neutral-700 dark:text-neutral-200 dark:hover:bg-neutral-800"
                      title="Choisir les établissements sur lesquels cet employé peut travailler"
                    >
                      <Store className="h-3 w-3" />
                      Affecter
                    </button>
                  </TableCell>
                  <TableCell className="px-4 py-3 text-right">
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
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </div>

      {/* Modal d'affectation aux établissements */}
      {affectationPour && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-neutral-900/60 p-4 backdrop-blur-xs">
          <div className="w-full max-w-md rounded-2xl border border-neutral-200 bg-white p-5 shadow-xl dark:border-neutral-800 dark:bg-neutral-900 shadow-2xl shadow-neutral-950/25 dark:shadow-black/80 ring-1 ring-black/5 dark:ring-white/10">
            <div className="flex items-start justify-between">
              <div className="flex items-center gap-2">
                <Building2 className="h-5 w-5 text-sky-500" />
                <div>
                  <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
                    Établissements de {affectationPour.full_name}
                  </h2>
                  <p className="mt-0.5 text-[11px] text-neutral-500 dark:text-neutral-400">
                    Cochez les sites sur lesquels ce compte peut encaisser et saisir des commandes.
                  </p>
                </div>
              </div>
              <button
                type="button"
                onClick={() => setAffectationPour(null)}
                className="rounded-lg p-1 text-neutral-400 transition-colors hover:bg-neutral-950 hover:text-white dark:hover:text-neutral-200 dark:hover:bg-neutral-100 dark:hover:text-neutral-950"
                aria-label="Fermer"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            {chargementSites && (
              <div className="mt-4 flex items-center gap-2 text-xs text-neutral-500 dark:text-neutral-400">
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
                Chargement des établissements…
              </div>
            )}

            {!chargementSites && erreurSites && (
              <div className="mt-4 flex items-center gap-2 rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-[11px] text-rose-700 dark:border-rose-900/50 dark:bg-rose-950/30 dark:text-rose-300">
                <WifiOff className="h-3.5 w-3.5" />
                {erreurSites}
              </div>
            )}

            {!chargementSites && !erreurSites && (
              <div className="mt-4 space-y-2">
                {sites.length === 0 && (
                  <p className="text-xs text-neutral-500 dark:text-neutral-400">
                    Aucun établissement déclaré pour ce pressing.
                  </p>
                )}
                {sites.map((site) => (
                  <label
                    key={site.id}
                    className={`flex cursor-pointer items-center justify-between gap-3 rounded-lg border px-3 py-2 text-xs transition-colors ${
                      selectionSites.includes(site.id)
                        ? 'border-sky-300 bg-sky-50 dark:border-sky-900 dark:bg-sky-950/30'
                        : 'border-neutral-200 hover:bg-neutral-900/[0.06] dark:border-neutral-700 dark:hover:bg-neutral-800'
                    }`}
                  >
                    <span className="flex items-center gap-2">
                      <input
                        type="checkbox"
                        className="h-3.5 w-3.5"
                        aria-label={site.name}
                        checked={selectionSites.includes(site.id)}
                        onChange={() => basculerSite(site.id)}
                      />
                      <span className="font-medium text-neutral-800 dark:text-neutral-100">{site.name}</span>
                      {site.is_primary && (
                        <span className="rounded-full bg-neutral-100 px-2 py-0.5 text-[10px] font-medium text-neutral-600 dark:bg-neutral-800 dark:text-neutral-300">
                          principal
                        </span>
                      )}
                    </span>
                    <span className="text-[10px] uppercase tracking-wide text-neutral-400 dark:text-neutral-500">
                      {branchStatusLabel(site.status)}
                    </span>
                  </label>
                ))}
                <p className="text-[11px] text-neutral-500 dark:text-neutral-400">
                  {selectionSites.length} établissement{selectionSites.length > 1 ? 's' : ''} sélectionné
                  {selectionSites.length > 1 ? 's' : ''} · un site retiré ferme immédiatement l’accès aux
                  commandes qui y sont rattachées.
                </p>
              </div>
            )}

            <div className="mt-5 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => setAffectationPour(null)}
                className="rounded-lg border border-neutral-200 px-3 py-2 text-xs font-medium text-neutral-700 transition-colors hover:bg-neutral-900/[0.06] dark:border-neutral-700 dark:text-neutral-200 dark:hover:bg-neutral-800"
              >
                Annuler
              </button>
              <button
                type="button"
                onClick={() => void enregistrerAffectation()}
                disabled={enregistrementSites || chargementSites || Boolean(erreurSites)}
                className="inline-flex items-center gap-1.5 rounded-lg bg-sky-600 px-3 py-2 text-xs font-medium text-white transition-colors hover:bg-sky-700 disabled:opacity-50"
              >
                {enregistrementSites ? (
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Building2 className="h-3.5 w-3.5" />
                )}
                Enregistrer l’affectation
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Modal de création */}
      {isModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-neutral-900/60 p-4 backdrop-blur-xs">
          <div className="w-full max-w-md rounded-2xl border border-neutral-200 bg-white p-5 shadow-xl dark:border-neutral-800 dark:bg-neutral-900 shadow-2xl shadow-neutral-950/25 dark:shadow-black/80 ring-1 ring-black/5 dark:ring-white/10">
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
                className="rounded-lg p-1 text-neutral-400 transition-colors hover:bg-neutral-950 hover:text-white dark:hover:bg-neutral-100 dark:hover:text-neutral-950"
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
                  className="rounded-lg border border-neutral-200 px-3 py-2 text-xs font-medium text-neutral-600 transition-colors hover:bg-neutral-900/[0.06] dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800"
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
