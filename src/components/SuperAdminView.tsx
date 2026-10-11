import React, { useCallback, useEffect, useState } from 'react';
import {
  Building2,
  TrendingUp,
  Shield,
  Plus,
  CheckCircle,
  Search,
  RefreshCw,
  AlertTriangle,
  Loader2,
} from 'lucide-react';
import { User } from '../types';
import { canAccessSuperadmin } from '../lib/permissions';
import {
  fetchPlatformStats,
  fetchTenants,
  setTenantStatus,
  registerTenant,
  subscriptionLabel,
  formatXOF,
  PlatformStats,
  TenantOverview,
} from '../lib/adminApi';

interface SuperAdminViewProps {
  currentUser: User;
  token?: string;
  showToast?: (message: string) => void;
  /** Conservé pour compatibilité : la console éditeur n'en a plus besoin. */
  onSelectTenantView?: (tenantId: string) => void;
}

export const SuperAdminView: React.FC<SuperAdminViewProps> = ({
  currentUser,
  token,
  showToast,
}) => {
  const [stats, setStats] = useState<PlatformStats | null>(null);
  const [tenants, setTenants] = useState<TenantOverview[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [busyTenant, setBusyTenant] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [newTenantName, setNewTenantName] = useState('');
  const [newCity, setNewCity] = useState('Abidjan (Cocody)');
  const [newPhone, setNewPhone] = useState('');
  const [newOwnerName, setNewOwnerName] = useState('');
  const [newEmail, setNewEmail] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  const isSuperadmin = canAccessSuperadmin(currentUser);

  const load = useCallback(async () => {
    setLoading(true);
    const [platformStats, directory] = await Promise.all([
      fetchPlatformStats(token),
      fetchTenants(token),
    ]);
    setStats(platformStats);
    setTenants(directory);
    setError(
      platformStats === null
        ? "Impossible de charger les indicateurs de la plateforme (API ou base indisponible)."
        : null
    );
    setLoading(false);
  }, [token]);

  useEffect(() => {
    if (isSuperadmin) void load();
  }, [isSuperadmin, load]);

  if (!isSuperadmin) {
    return (
      <div className="flex h-full flex-col items-center justify-center rounded-lg border border-neutral-200 bg-white p-6 text-center dark:border-neutral-800 dark:bg-[#0B0B0B]">
        <Shield className="h-8 w-8 text-neutral-400 dark:text-neutral-500" />
        <h1 className="mt-3 text-lg font-semibold text-neutral-900 dark:text-neutral-100">
          Accès refusé
        </h1>
        <p className="mt-1 text-xs text-neutral-500 dark:text-neutral-400">
          Vous n&apos;avez pas l&apos;autorisation requise pour accéder à
          l&apos;Espace Éditeur SaaS. Seuls les comptes de super-utilisateur de la
          plateforme peuvent y accéder.
        </p>
      </div>
    );
  }

  const isMemoryMode = stats?.data_source === 'memoire';

  const toggleStatus = async (tenant: TenantOverview) => {
    const cible = tenant.active ? 'SUSPENDED' : 'ACTIVE';
    setBusyTenant(tenant.tenant_id);
    const result = await setTenantStatus(
      tenant.tenant_id,
      cible,
      token,
      cible === 'SUSPENDED' ? 'suspension décidée depuis la console éditeur' : 'réactivation éditeur'
    );
    setBusyTenant(null);
    if (!result.ok) {
      showToast?.(`Échec de la mise à jour : ${result.error}`);
      return;
    }
    showToast?.(
      cible === 'SUSPENDED'
        ? `Abonnement de ${tenant.name} suspendu (opérations gelées)`
        : `Abonnement de ${tenant.name} réactivé`
    );
    await load();
  };

  const handleCreateTenant = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newTenantName || !newEmail || !newPassword) return;
    setCreating(true);
    setCreateError(null);
    const result = await registerTenant({
      tenant_name: newTenantName,
      city: newCity,
      phone: newPhone || '+225 07 00 00 00 00',
      owner_full_name: newOwnerName || newTenantName,
      owner_email: newEmail,
      password: newPassword,
    });
    setCreating(false);
    if (!result.ok) {
      setCreateError(result.error ?? 'inscription refusée');
      return;
    }
    showToast?.(
      `${newTenantName} inscrit — catalogue, grille tarifaire et essai 7 jours créés`
    );
    setNewTenantName('');
    setNewEmail('');
    setNewPhone('');
    setNewOwnerName('');
    setNewPassword('');
    setIsModalOpen(false);
    await load();
  };

  const filteredTenants = tenants.filter(
    (t) =>
      t.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      t.city.toLowerCase().includes(searchQuery.toLowerCase()) ||
      t.owner_email.toLowerCase().includes(searchQuery.toLowerCase())
  );

  const actifs = tenants.filter((t) => t.active).length;
  const essais = tenants.filter((t) => t.status.toUpperCase() === 'TRIALING').length;

  return (
    <div className="space-y-6">
      {/* Header Strip */}
      <div className="flex flex-col gap-3 rounded-lg border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-[#111111] sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-neutral-900 text-white dark:bg-neutral-100 dark:text-neutral-900 shadow-xs">
            <Shield className="h-5 w-5" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="text-lg font-semibold tracking-tight text-neutral-900 dark:text-neutral-100">
                Espace Éditeur SaaS — Super Administration Globale
              </h1>
              <span className="font-mono text-[10px] bg-neutral-900 text-white dark:bg-neutral-100 dark:text-neutral-900 px-2 py-0.5 rounded font-medium">
                SuperAdmin
              </span>
            </div>
            <p className="mt-0.5 text-xs text-neutral-500 dark:text-neutral-400">
              Gestion de l&apos;ensemble des pressings souscrits en Côte
              d&apos;Ivoire, facturation et abonnements.
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2 self-start sm:self-auto">
          <button
            onClick={() => void load()}
            className="flex items-center gap-1.5 rounded-md border border-neutral-200 px-3 py-1.5 text-xs font-medium text-neutral-700 hover:bg-neutral-50 dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800 transition-colors"
          >
            <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
            <span>Actualiser</span>
          </button>
          <button
            onClick={() => setIsModalOpen(true)}
            className="flex items-center gap-1.5 rounded-md bg-neutral-900 px-3 py-1.5 text-xs font-medium text-white hover:bg-neutral-800 dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-white transition-colors"
          >
            <Plus className="h-3.5 w-3.5" />
            <span>Enregistrer un Pressing</span>
          </button>
        </div>
      </div>

      {isMemoryMode && (
        <div className="flex items-start gap-2 rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-900 dark:border-amber-900/60 dark:bg-amber-950/40 dark:text-amber-200">
          <AlertTriangle className="mt-0.5 h-3.5 w-3.5 flex-shrink-0" />
          <span>
            Mode démonstration : aucune base de données n&apos;est connectée, les
            chiffres proviennent des abonnements en mémoire et ne sont pas
            persistés.
          </span>
        </div>
      )}

      {error && (
        <div className="flex items-start gap-2 rounded-lg border border-rose-200 bg-rose-50 p-3 text-xs text-rose-800 dark:border-rose-900/60 dark:bg-rose-950/40 dark:text-rose-200">
          <AlertTriangle className="mt-0.5 h-3.5 w-3.5 flex-shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {/* Global SaaS KPIs — données réelles */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <div className="rounded-lg border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-[#111111]">
          <div className="flex items-center justify-between text-xs text-neutral-500 dark:text-neutral-400">
            <span>Pressings Souscrits</span>
            <Building2 className="h-4 w-4 text-neutral-400 dark:text-neutral-500" />
          </div>
          <div className="mt-2 flex items-baseline gap-1.5">
            <span className="font-mono text-2xl font-semibold tracking-tight text-neutral-900 dark:text-neutral-100 tabular-nums">
              {loading && !stats ? '—' : tenants.length}
            </span>
            <span className="text-xs text-neutral-500 dark:text-neutral-400">établissements</span>
          </div>
          <div className="mt-1 text-[11px] text-neutral-500 dark:text-neutral-400">
            Côte d&apos;Ivoire (XOF)
          </div>
        </div>

        <div className="rounded-lg border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-[#111111]">
          <div className="flex items-center justify-between text-xs text-neutral-500 dark:text-neutral-400">
            <span>Revenu Mensuel (MRR)</span>
            <TrendingUp className="h-4 w-4 text-neutral-400 dark:text-neutral-500" />
          </div>
          <div className="mt-2 flex items-baseline gap-1.5">
            <span className="font-mono text-2xl font-semibold tracking-tight text-neutral-900 dark:text-neutral-100 tabular-nums">
              {stats ? formatXOF(stats.monthly_recurring_mrr_xof).replace(' FCFA', '') : '—'}
            </span>
            <span className="font-mono text-xs text-neutral-500 dark:text-neutral-400 font-medium">
              FCFA / mois
            </span>
          </div>
          <div className="mt-1 text-[11px] text-neutral-500 dark:text-neutral-400">
            {stats ? `${stats.active_subscriptions} abonnement(s) payant(s)` : 'abonnements actifs'}
          </div>
        </div>

        <div className="rounded-lg border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-[#111111]">
          <div className="flex items-center justify-between text-xs text-neutral-500 dark:text-neutral-400">
            <span>Comptes Actifs</span>
            <CheckCircle className="h-4 w-4 text-neutral-400 dark:text-neutral-500" />
          </div>
          <div className="mt-2 flex items-baseline gap-1.5">
            <span className="font-mono text-2xl font-semibold tracking-tight text-neutral-900 dark:text-neutral-100 tabular-nums">
              {loading && tenants.length === 0 ? '—' : actifs}
            </span>
            <span className="text-xs text-neutral-500 dark:text-neutral-400">en production</span>
          </div>
          <div className="mt-1 text-[11px] text-neutral-500 dark:text-neutral-400">
            {essais} en période d&apos;essai
          </div>
        </div>

        <div className="rounded-lg border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-[#111111]">
          <div className="flex items-center justify-between text-xs text-neutral-500 dark:text-neutral-400">
            <span>Utilisateurs Plateforme</span>
            <Shield className="h-4 w-4 text-neutral-400 dark:text-neutral-500" />
          </div>
          <div className="mt-2 flex items-baseline gap-1.5">
            <span className="font-mono text-2xl font-semibold tracking-tight text-neutral-900 dark:text-neutral-100 tabular-nums">
              {stats ? stats.total_users : '—'}
            </span>
            <span className="text-xs text-neutral-500 dark:text-neutral-400">comptes actifs</span>
          </div>
          <div className="mt-1 text-[11px] text-neutral-500 dark:text-neutral-400">
            Isolation multi-tenant vérifiée par middleware Go Chi
          </div>
        </div>
      </div>

      {/* Tenants Table */}
      <div className="rounded-lg border border-neutral-200 bg-white dark:border-neutral-800 dark:bg-[#111111]">
        <div className="flex flex-col gap-3 border-b border-neutral-200 px-5 py-3.5 dark:border-neutral-800 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
              Répertoire des Pressings Partenaires
            </h2>
            <p className="text-xs text-neutral-500 dark:text-neutral-400">
              Contrôle d&apos;accès et souscriptions des clients de PressingOS.
            </p>
          </div>

          <div className="relative w-full sm:w-64">
            <Search className="absolute left-2.5 top-2.5 h-3.5 w-3.5 text-neutral-400 dark:text-neutral-500" />
            <input
              type="text"
              placeholder="Rechercher pressing, ville..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full rounded-md border border-neutral-200 bg-white pl-8 pr-3 py-1.5 text-xs text-neutral-900 placeholder:text-neutral-400 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:placeholder:text-neutral-500 dark:focus:border-neutral-100"
            />
          </div>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs">
            <thead>
              <tr className="border-b border-neutral-100 bg-neutral-50/75 text-[11px] text-neutral-500 font-medium dark:border-neutral-800 dark:bg-neutral-900/60 dark:text-neutral-400">
                <th className="px-5 py-2.5">Établissement</th>
                <th className="px-4 py-2.5">Ville & Contact</th>
                <th className="px-4 py-2.5">Compte Propriétaire</th>
                <th className="px-4 py-2.5">Offre Souscrite</th>
                <th className="px-4 py-2.5">Statut</th>
                <th className="px-5 py-2.5 text-right">Actions Éditeur</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-neutral-100 dark:divide-neutral-800">
              {loading && tenants.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-5 py-10 text-center text-neutral-500 dark:text-neutral-400">
                    <Loader2 className="mx-auto h-5 w-5 animate-spin" />
                    <div className="mt-2">Chargement de l&apos;annuaire…</div>
                  </td>
                </tr>
              )}

              {!loading && filteredTenants.length === 0 && (
                <tr>
                  <td colSpan={6} className="px-5 py-10 text-center text-neutral-500 dark:text-neutral-400">
                    Aucun établissement trouvé.
                  </td>
                </tr>
              )}

              {filteredTenants.map((t) => (
                <tr
                  key={t.tenant_id}
                  className="hover:bg-neutral-50/50 dark:hover:bg-neutral-800/40 transition-colors"
                >
                  <td className="px-5 py-3.5">
                    <div className="font-semibold text-neutral-900 dark:text-neutral-100">
                      {t.name || 'Établissement sans nom'}
                    </div>
                    <div className="font-mono text-[10px] text-neutral-400 dark:text-neutral-500">
                      {t.tenant_id}
                    </div>
                  </td>
                  <td className="px-4 py-3.5 text-neutral-600 dark:text-neutral-400">
                    <div>{t.city || '—'}</div>
                    <div className="font-mono text-[11px] text-neutral-400 dark:text-neutral-500">
                      {t.phone || '—'}
                    </div>
                  </td>
                  <td className="px-4 py-3.5 font-mono text-neutral-700 dark:text-neutral-300">
                    {t.owner_email || '—'}
                  </td>
                  <td className="px-4 py-3.5 font-medium text-neutral-800 dark:text-neutral-200">
                    {t.plan_name}
                    {t.price_monthly_xof > 0 && (
                      <span className="block font-mono text-[11px] text-neutral-500 dark:text-neutral-400">
                        {formatXOF(t.price_monthly_xof)} / mois
                      </span>
                    )}
                  </td>
                  <td className="px-4 py-3.5">
                    {t.status.toUpperCase() === 'ACTIVE' && (
                      <span className="font-mono text-[11px] text-emerald-700 dark:text-emerald-400 font-medium">
                        Actif
                      </span>
                    )}
                    {t.status.toUpperCase() === 'TRIALING' && (
                      <span className="font-mono text-[11px] text-amber-700 dark:text-amber-400 font-medium">
                        {subscriptionLabel(t.status)}
                        {t.trial_ends_at
                          ? ` → ${new Date(t.trial_ends_at).toLocaleDateString('fr-FR')}`
                          : ''}
                      </span>
                    )}
                    {['PAST_DUE', 'CANCELED', 'SUSPENDED'].includes(t.status.toUpperCase()) && (
                      <span className="font-mono text-[11px] text-rose-700 dark:text-rose-400 font-medium">
                        {subscriptionLabel(t.status)}
                      </span>
                    )}
                  </td>
                  <td className="px-5 py-3.5 text-right">
                    <button
                      onClick={() => void toggleStatus(t)}
                      disabled={busyTenant === t.tenant_id}
                      className={`inline-flex items-center gap-1.5 rounded px-2.5 py-1 text-[11px] font-medium border transition-colors disabled:opacity-50 ${
                        t.active
                          ? 'border-neutral-200 text-neutral-700 hover:bg-neutral-100 dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800'
                          : 'border-emerald-200 bg-emerald-50 text-emerald-800 hover:bg-emerald-100 dark:border-emerald-800 dark:bg-emerald-950/60 dark:text-emerald-300'
                      }`}
                    >
                      {busyTenant === t.tenant_id && <Loader2 className="h-3 w-3 animate-spin" />}
                      {t.active ? 'Suspendre' : 'Réactiver'}
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Add Tenant Modal */}
      {isModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-neutral-900/40 dark:bg-black/70 p-4 backdrop-blur-xs">
          <div className="w-full max-w-sm rounded-lg border border-neutral-200 bg-white p-5 shadow-lg dark:border-neutral-800 dark:bg-[#111111]">
            <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
              Inscrire un Nouveau Pressing
            </h2>
            <p className="mt-0.5 text-xs text-neutral-500 dark:text-neutral-400">
              L&apos;inscription crée l&apos;établissement, son compte propriétaire,
              ses 49 prestations et sa grille tarifaire, avec 7 jours d&apos;essai.
            </p>

            <form onSubmit={handleCreateTenant} className="mt-4 space-y-3">
              <div>
                <label className="block text-[11px] font-medium text-neutral-700 dark:text-neutral-300">
                  Nom du Pressing
                </label>
                <input
                  type="text"
                  required
                  placeholder="Ex: Pressing de la Lagune"
                  value={newTenantName}
                  onChange={(e) => setNewTenantName(e.target.value)}
                  className="mt-1 w-full rounded-md border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100"
                />
              </div>

              <div>
                <label className="block text-[11px] font-medium text-neutral-700 dark:text-neutral-300">
                  Ville / Commune
                </label>
                <input
                  type="text"
                  required
                  placeholder="Ex: Abidjan (Treichville)"
                  value={newCity}
                  onChange={(e) => setNewCity(e.target.value)}
                  className="mt-1 w-full rounded-md border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100"
                />
              </div>

              <div>
                <label className="block text-[11px] font-medium text-neutral-700 dark:text-neutral-300">
                  Nom du propriétaire
                </label>
                <input
                  type="text"
                  placeholder="Ex: Mme Awa Traoré"
                  value={newOwnerName}
                  onChange={(e) => setNewOwnerName(e.target.value)}
                  className="mt-1 w-full rounded-md border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100"
                />
              </div>

              <div>
                <label className="block text-[11px] font-medium text-neutral-700 dark:text-neutral-300">
                  Email Propriétaire
                </label>
                <input
                  type="email"
                  required
                  placeholder="contact@pressing-lagune.ci"
                  value={newEmail}
                  onChange={(e) => setNewEmail(e.target.value)}
                  className="mt-1 w-full rounded-md border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100"
                />
              </div>

              <div>
                <label className="block text-[11px] font-medium text-neutral-700 dark:text-neutral-300">
                  Mot de passe initial (6 caractères minimum)
                </label>
                <input
                  type="text"
                  required
                  minLength={6}
                  placeholder="À communiquer au propriétaire"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  className="mt-1 w-full rounded-md border border-neutral-200 px-3 py-1.5 font-mono text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100"
                />
              </div>

              <div>
                <label className="block text-[11px] font-medium text-neutral-700 dark:text-neutral-300">
                  Téléphone (+225)
                </label>
                <input
                  type="text"
                  placeholder="+225 07 12 34 56 78"
                  value={newPhone}
                  onChange={(e) => setNewPhone(e.target.value)}
                  className="mt-1 w-full rounded-md border border-neutral-200 px-3 py-1.5 text-xs text-neutral-900 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:focus:border-neutral-100"
                />
              </div>

              {createError && (
                <div className="rounded-md border border-rose-200 bg-rose-50 px-3 py-2 text-[11px] text-rose-800 dark:border-rose-900/60 dark:bg-rose-950/40 dark:text-rose-200">
                  {createError}
                </div>
              )}

              <div className="mt-5 flex justify-end gap-2 pt-2 border-t border-neutral-100 dark:border-neutral-800">
                <button
                  type="button"
                  onClick={() => setIsModalOpen(false)}
                  className="rounded-md border border-neutral-200 px-3 py-1.5 text-xs font-medium text-neutral-700 hover:bg-neutral-50 dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800"
                >
                  Annuler
                </button>
                <button
                  type="submit"
                  disabled={creating}
                  className="inline-flex items-center gap-1.5 rounded-md bg-neutral-900 px-3 py-1.5 text-xs font-medium text-white hover:bg-neutral-800 disabled:opacity-60 dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-white"
                >
                  {creating && <Loader2 className="h-3 w-3 animate-spin" />}
                  Créer le Pressing
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
