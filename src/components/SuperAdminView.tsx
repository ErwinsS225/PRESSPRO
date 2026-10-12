import React, { useCallback, useEffect, useMemo, useState } from 'react';
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
  Trash2,
} from 'lucide-react';
import { User } from '../types';
import { Badge } from '@/components/reui/badge';
import { Frame, FramePanel } from '@/components/reui/frame';
import { PlansFunnelChart } from '@/components/reui/plans-funnel-chart';
import { SubscriptionsTable, type SubscriptionRow } from '@/components/reui/subscriptions-table';
import { canAccessSuperadmin } from '../lib/permissions';
import {
  fetchPlatformStats,
  fetchTenants,
  setTenantStatus,
  deleteTenant,
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

interface LigneSouscription extends SubscriptionRow {
  origine: TenantOverview;
  actif: boolean;
}

export const SuperAdminView: React.FC<SuperAdminViewProps> = ({ currentUser, token, showToast }) => {
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
  // Retrait définitif d'un établissement (données de démonstration incluses) :
  // l'action n'est exécutée qu'après confirmation explicite de l'éditeur.
  const [tenantASupprimer, setTenantASupprimer] = useState<TenantOverview | null>(null);
  const [confirmationSuppression, setConfirmationSuppression] = useState(false);
  const [suppressionEnCours, setSuppressionEnCours] = useState(false);
  const [erreurSuppression, setErreurSuppression] = useState<string | null>(null);

  const isSuperadmin = canAccessSuperadmin(currentUser);

  const load = useCallback(async () => {
    setLoading(true);
    const [platformStats, directory] = await Promise.all([fetchPlatformStats(token), fetchTenants(token)]);
    setStats(platformStats);
    setTenants(directory);
    setError(
      platformStats === null
        ? 'Impossible de charger les indicateurs de la plateforme (API ou base indisponible).'
        : null
    );
    setLoading(false);
  }, [token]);

  useEffect(() => {
    if (isSuperadmin) void load();
  }, [isSuperadmin, load]);

  /** Répartition des pressings par formule, pour l'anneau de conversion (c-chart-22). */
  const repartitionFormules = useMemo(() => {
    const ordre = ['FREE', 'PRO', 'BUSINESS'];
    const parFormule = new Map<string, { label: string; value: number }>();
    for (const t of tenants) {
      const code = (t.plan_code || 'FREE').toUpperCase();
      const entree = parFormule.get(code) ?? { label: t.plan_name || code, value: 0 };
      entree.value += 1;
      parFormule.set(code, entree);
    }
    return [...parFormule.entries()]
      .sort((a, b) => ordre.indexOf(a[0]) - ordre.indexOf(b[0]))
      .map(([plan, valeur]) => ({ plan: plan.toLowerCase(), label: valeur.label, value: valeur.value }));
  }, [tenants]);

  if (!isSuperadmin) {
    return (
      <div className="flex h-full flex-col items-center justify-center rounded-lg border border-neutral-200 bg-white p-6 text-center dark:border-neutral-800 dark:bg-[#0B0B0B] hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
        <Shield className="h-8 w-8 text-neutral-400 dark:text-neutral-500" />
        <h1 className="mt-3 text-lg font-semibold text-neutral-900 dark:text-neutral-100">Accès refusé</h1>
        <p className="mt-1 text-xs text-neutral-500 dark:text-neutral-400">
          Vous n&apos;avez pas l&apos;autorisation requise pour accéder à l&apos;Espace Éditeur SaaS. Seuls
          les comptes de super-utilisateur de la plateforme peuvent y accéder.
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

  /** Ouvre la confirmation de suppression (aucun appel réseau à ce stade). */
  const demanderSuppression = (tenant: TenantOverview) => {
    setTenantASupprimer(tenant);
    setConfirmationSuppression(false);
    setErreurSuppression(null);
  };

  /** Exécute la suppression définitive décidée par l'éditeur. */
  const handleDeleteTenant = async () => {
    if (!tenantASupprimer || !confirmationSuppression) return;
    setSuppressionEnCours(true);
    setErreurSuppression(null);
    const cible = tenantASupprimer;
    const result = await deleteTenant(cible.tenant_id, token);
    setSuppressionEnCours(false);
    if (!result.ok) {
      setErreurSuppression(result.error ?? 'suppression refusée');
      return;
    }
    showToast?.(`${cible.name} supprimé avec toutes ses données`);
    setTenantASupprimer(null);
    setConfirmationSuppression(false);
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
    showToast?.(`${newTenantName} inscrit — catalogue, grille tarifaire et essai 7 jours créés`);
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
  const suspendus = tenants.filter((t) =>
    ['PAST_DUE', 'CANCELED', 'SUSPENDED'].includes(t.status.toUpperCase())
  ).length;
  const payants = tenants.filter((t) => t.plan_code.toUpperCase() !== 'FREE').length;

  /** Lignes du tableau des souscriptions (ReUI c-table-10). */
  const lignesSouscriptions: LigneSouscription[] = filteredTenants.map((t) => ({
    service: t.name,
    subtitle: (
      <>
        <span>{t.city}</span>
        <span className="mx-1">·</span>
        <span>{t.owner_email}</span>
      </>
    ),
    plan: t.plan_code,
    planVariant:
      t.plan_code.toUpperCase() === 'BUSINESS'
        ? 'info'
        : t.plan_code.toUpperCase() === 'PRO'
          ? 'default'
          : 'secondary',
    billing:
      t.price_monthly_xof === null ? 'Tarif non communiqué' : `${formatXOF(t.price_monthly_xof)} / mois`,
    status: subscriptionLabel(t.status),
    statusVariant: t.active ? 'success-light' : 'destructive-light',
    statusDetail: t.trial_ends_at
      ? `Fin d'essai : ${new Date(t.trial_ends_at).toLocaleDateString('fr-FR')}`
      : undefined,
    origine: t,
    actif: t.active,
  }));

  return (
    <div className="space-y-6">
      {/* Header Strip */}
      <div className="flex flex-col gap-3 rounded-lg border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-[#111111] sm:flex-row sm:items-center sm:justify-between hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
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
              Gestion de l&apos;ensemble des pressings souscrits en Côte d&apos;Ivoire, facturation et
              abonnements.
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2 self-start sm:self-auto">
          <button
            onClick={() => void load()}
            className="flex items-center gap-1.5 rounded-md border border-neutral-200 px-3 py-1.5 text-xs font-medium text-neutral-700 hover:bg-neutral-900/[0.06] dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800 transition-colors"
          >
            <RefreshCw className={`h-3.5 w-3.5 ${loading ? 'animate-spin' : ''}`} />
            <span>Actualiser</span>
          </button>
          <button
            onClick={() => setIsModalOpen(true)}
            className="flex items-center gap-1.5 rounded-md bg-neutral-900 px-3 py-1.5 text-xs font-medium text-white hover:bg-black dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-white transition-colors"
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
            Mode démonstration : aucune base de données n&apos;est connectée, les chiffres proviennent des
            abonnements en mémoire et ne sont pas persistés.
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
        <div className="rounded-lg border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-[#111111] hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
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

        <div className="rounded-lg border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-[#111111] hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
          <div className="flex items-center justify-between text-xs text-neutral-500 dark:text-neutral-400">
            <span>Revenu Mensuel (MRR)</span>
            <TrendingUp className="h-4 w-4 text-neutral-400 dark:text-neutral-500" />
          </div>
          <div className="mt-2 flex items-baseline gap-1.5">
            <span className="font-mono text-2xl font-semibold tracking-tight text-neutral-900 dark:text-neutral-100 tabular-nums">
              {stats && stats.monthly_recurring_mrr_xof !== null
                ? formatXOF(stats.monthly_recurring_mrr_xof).replace(' FCFA', '')
                : '—'}
            </span>
            <span className="font-mono text-xs text-neutral-500 dark:text-neutral-400 font-medium">
              FCFA / mois
            </span>
          </div>
          <div className="mt-1 text-[11px] text-neutral-500 dark:text-neutral-400">
            {stats ? `${stats.active_subscriptions} abonnement(s) payant(s)` : 'abonnements actifs'}
          </div>
        </div>

        <div className="rounded-lg border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-[#111111] hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
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

        <div className="rounded-lg border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-[#111111] hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
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

      {/* Répertoire des souscriptions (composant ReUI c-table-10) */}
      <div className="rounded-lg border border-neutral-200 bg-white dark:border-neutral-800 dark:bg-[#111111] hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
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

        <div className="px-2 py-2">
          <SubscriptionsTable
            rows={lignesSouscriptions}
            busyKey={busyTenant}
            manageLabel={(row) => (row.actif ? 'Suspendre' : 'Réactiver')}
            onManage={(row) => void toggleStatus(row.origine)}
            onDelete={(row) => demanderSuppression(row.origine)}
            deleteLabel="Supprimer"
            deleteBusyKey={suppressionEnCours ? (tenantASupprimer?.tenant_id ?? null) : null}
          />
        </div>

        {lignesSouscriptions.length === 0 && (
          <p className="border-t border-neutral-200 px-5 py-6 text-center text-xs text-neutral-500 dark:border-neutral-800 dark:text-neutral-400">
            Aucun pressing ne correspond à cette recherche.
          </p>
        )}
      </div>

      {/* Répartition des formules (composant ReUI c-chart-22) et état des abonnements */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
        <PlansFunnelChart
          title="Formules souscrites"
          description={`${((payants / Math.max(1, tenants.length)) * 100).toFixed(1)} % des pressings sont sur une formule payante`}
          data={repartitionFormules}
          freePlan="FREE"
          centerLabel="Pressings payants"
        />

        <Frame className="lg:col-span-2">
          <FramePanel className="flex items-center justify-between gap-3">
            <div>
              <h3 className="text-sm font-medium text-neutral-900 dark:text-neutral-100">
                État des abonnements
              </h3>
              <p className="text-xs text-neutral-500 dark:text-neutral-400">
                Suivi des souscriptions, des essais et des suspensions de la plateforme.
              </p>
            </div>
            <Badge variant="success" size="sm">
              {actifs} actif(s)
            </Badge>
          </FramePanel>
          <FramePanel className="flex flex-wrap items-center gap-2">
            <Badge variant="success-light" size="sm">
              {actifs} abonnement(s) actif(s)
            </Badge>
            <Badge variant="info-light" size="sm">
              {essais} en essai BUSINESS
            </Badge>
            <Badge variant="destructive-light" size="sm">
              {suspendus} suspendu(s)
            </Badge>
            <Badge variant="destructive-outline" size="sm">
              {tenants.length - actifs - essais - suspendus} autre(s) état
            </Badge>
          </FramePanel>
          <FramePanel className="text-xs text-neutral-500 dark:text-neutral-400">
            MRR suivi :{' '}
            <span className="font-mono font-medium text-neutral-900 tabular-nums dark:text-neutral-100">
              {stats
                ? stats.monthly_recurring_mrr_xof === null
                  ? 'Tarif non communiqué'
                  : formatXOF(stats.monthly_recurring_mrr_xof)
                : '—'}
            </span>{' '}
            · forme d&apos;essai équivalente au niveau BUSINESS pendant 7 jours.
          </FramePanel>
        </Frame>
      </div>

      {/* Add Tenant Modal */}
      {isModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-neutral-900/40 dark:bg-black/70 p-4 backdrop-blur-xs">
          <div className="w-full max-w-sm rounded-lg border border-neutral-200 bg-white p-5 shadow-lg dark:border-neutral-800 dark:bg-[#111111] shadow-2xl shadow-neutral-950/25 dark:shadow-black/80 ring-1 ring-black/5 dark:ring-white/10">
            <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
              Inscrire un Nouveau Pressing
            </h2>
            <p className="mt-0.5 text-xs text-neutral-500 dark:text-neutral-400">
              L&apos;inscription crée l&apos;établissement, son compte propriétaire, ses 49 prestations et sa
              grille tarifaire, avec 7 jours d&apos;essai.
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
                  className="rounded-md border border-neutral-200 px-3 py-1.5 text-xs font-medium text-neutral-700 hover:bg-neutral-900/[0.06] dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800"
                >
                  Annuler
                </button>
                <button
                  type="submit"
                  disabled={creating}
                  className="inline-flex items-center gap-1.5 rounded-md bg-neutral-900 px-3 py-1.5 text-xs font-medium text-white hover:bg-black disabled:opacity-60 dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-white"
                >
                  {creating && <Loader2 className="h-3 w-3 animate-spin" />}
                  Créer le Pressing
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Confirmation de suppression définitive */}
      {tenantASupprimer && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-neutral-900/40 dark:bg-black/70 p-4 backdrop-blur-xs">
          <div className="w-full max-w-sm rounded-lg border border-rose-200 bg-white p-5 shadow-xl dark:border-rose-900/60 dark:bg-[#111111] ring-1 ring-black/5 dark:ring-white/10">
            <div className="flex items-center gap-2">
              <span className="flex h-8 w-8 items-center justify-center rounded-md bg-rose-600 text-white">
                <Trash2 className="h-4 w-4" />
              </span>
              <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
                Supprimer définitivement cet établissement ?
              </h2>
            </div>

            <p className="mt-3 text-xs text-neutral-600 dark:text-neutral-400">
              <span className="font-semibold text-neutral-900 dark:text-neutral-100">
                {tenantASupprimer.name}
              </span>{' '}
              ({tenantASupprimer.slug}) sera retiré de la plateforme avec{' '}
              <span className="font-medium">toutes ses données</span> : comptes employés, clients, commandes,
              paiements, catalogue, dépenses, établissements et abonnement.
            </p>
            <p className="mt-2 text-[11px] text-rose-700 dark:text-rose-300">
              Cette action est irréversible et ne peut pas être annulée.
            </p>

            <label className="mt-4 flex items-start gap-2 rounded-md border border-neutral-200 px-3 py-2 text-[11px] text-neutral-700 dark:border-neutral-800 dark:text-neutral-300">
              <input
                type="checkbox"
                checked={confirmationSuppression}
                onChange={(e) => setConfirmationSuppression(e.target.checked)}
                className="mt-0.5 h-3.5 w-3.5 accent-rose-600"
                aria-label="Je confirme la suppression définitive de cet établissement"
              />
              <span>
                Je confirme la suppression définitive de <strong>{tenantASupprimer.name}</strong> et de ses
                données.
              </span>
            </label>

            {erreurSuppression && (
              <div className="mt-3 rounded-md border border-rose-200 bg-rose-50 px-3 py-2 text-[11px] text-rose-800 dark:border-rose-900/60 dark:bg-rose-950/40 dark:text-rose-200">
                {erreurSuppression}
              </div>
            )}

            <div className="mt-5 flex justify-end gap-2 border-t border-neutral-100 pt-2 dark:border-neutral-800">
              <button
                type="button"
                onClick={() => {
                  setTenantASupprimer(null);
                  setConfirmationSuppression(false);
                  setErreurSuppression(null);
                }}
                className="rounded-md border border-neutral-200 px-3 py-1.5 text-xs font-medium text-neutral-700 hover:bg-neutral-900/[0.06] dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800"
              >
                Annuler
              </button>
              <button
                type="button"
                onClick={() => void handleDeleteTenant()}
                disabled={!confirmationSuppression || suppressionEnCours}
                className="inline-flex items-center gap-1.5 rounded-md bg-rose-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-rose-700 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {suppressionEnCours && <Loader2 className="h-3 w-3 animate-spin" />}
                Supprimer définitivement
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
