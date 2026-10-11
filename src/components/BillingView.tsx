import React, { useEffect, useState } from 'react';
import { Building2, Check, Crown, Loader2, ShieldCheck, Sparkles, Store, WifiOff, Zap } from 'lucide-react';

/**
 * BillingView — écran « Mon abonnement ».
 *
 * Reflète le moteur d'abonnement centralisé (spec MAJ.md §1-7) :
 *  - état réel du pressing via GET /api/v1/me/entitlements (essai 7 j, plan, quota),
 *  - catalogue des offres FREE / PRO / BUSINESS (config par données côté backend),
 *  - demande d'upgrade (validation manuelle du paiement par l'admin SaaS).
 *
 * Hors-ligne : repli sur un état de démonstration cohérent (essai actif).
 */

interface Entitlement {
  key: string;
  enabled: boolean;
  limit?: number | null;
  usage?: number | null;
  remaining?: number | null;
  source: string;
}

interface EntitlementsResponse {
  tenant_id: string;
  plan_code: string;
  plan_name: string;
  status: string;
  on_trial: boolean;
  trial_ends_at?: string;
  days_left_trial?: number | null;
  entitlements: Entitlement[];
}

interface PlanCard {
  code: string;
  name: string;
  price: number;
  tagline: string;
  icon: React.ElementType;
  popular?: boolean;
  highlights: string[];
}

// Catalogue commercial (prix alignés sur migrations 000004 / matrice 000006).
const PLANS: PlanCard[] = [
  {
    code: 'FREE',
    name: 'Gratuit',
    price: 0,
    tagline: 'Pour démarrer et tester sans engagement',
    icon: Store,
    highlights: [
      'Commandes & tickets thermiques 80 mm',
      'Fichier clients & suivi du linge',
      '50 commandes / mois',
      '2 utilisateurs · 1 grille tarifaire · 1 site',
    ],
  },
  {
    code: 'PRO',
    name: 'Pro',
    price: 45000,
    tagline: 'Pour un pressing qui tourne à plein régime',
    icon: Zap,
    popular: true,
    highlights: [
      'Tout le plan Gratuit',
      'Commandes illimitées',
      '5 utilisateurs · 3 grilles tarifaires',
      'Rapports avancés & tarification PRO',
    ],
  },
  {
    code: 'BUSINESS',
    name: 'Business',
    price: 75000,
    tagline: 'Multi-sites & consolidation du groupe',
    icon: Building2,
    highlights: [
      'Tout le plan Pro',
      'Commandes illimitées',
      '15 utilisateurs · 10 grilles tarifaires',
      '3 sites multi-établissements',
      'Rapports consolidés & audit avancé',
    ],
  },
];

const FALLBACK: EntitlementsResponse = {
  tenant_id: '',
  plan_code: 'FREE',
  plan_name: 'Gratuit permanent',
  status: 'TRIALING',
  on_trial: true,
  days_left_trial: 7,
  entitlements: [],
};

const STATUS_LABELS: Record<string, string> = {
  TRIALING: 'Essai gratuit',
  ACTIVE: 'Abonnement actif',
  PAST_DUE: 'Paiement en retard',
  CANCELED: 'Résilié',
  SUSPENDED: 'Suspendu',
};

const formatXOF = (n: number) => `${n.toLocaleString('fr-FR')} FCFA`;

export const BillingView: React.FC<{
  token?: string;
  showToast?: (m: string) => void;
}> = ({ token, showToast }) => {
  const [loading, setLoading] = useState(true);
  const [data, setData] = useState<EntitlementsResponse>(FALLBACK);
  const [fromApi, setFromApi] = useState(false);
  const [requested, setRequested] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      try {
        const res = await fetch('/api/v1/me/entitlements', {
          headers: {
            'Content-Type': 'application/json',
            Accept: 'application/json',
            ...(token ? { Authorization: `Bearer ${token}` } : {}),
          },
        });
        if (res.ok) {
          const json = (await res.json()) as EntitlementsResponse;
          if (!cancelled) {
            setData(json);
            setFromApi(true);
          }
        }
      } catch {
        /* hors-ligne : état de démonstration conservé */
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    load();
    return () => {
      cancelled = true;
    };
  }, [token]);

  const ordersEntitlement = data.entitlements.find((e) => e.key === 'orders.monthly.max');
  const ordersLimit = ordersEntitlement?.limit ?? null;

  const handleRequest = (code: string, name: string) => {
    setRequested(code);
    showToast?.(
      `Demande d'abonnement « ${name} » enregistrée. Notre équipe vous contactera pour valider le paiement (Mobile Money / virement).`
    );
  };

  return (
    <div className="space-y-6">
      {/* En-tête */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="flex items-center gap-2 text-lg font-semibold tracking-tight text-neutral-900 dark:text-neutral-100">
            <Crown className="h-5 w-5 text-amber-500" />
            Mon abonnement
          </h1>
          <p className="mt-0.5 text-xs text-neutral-500 dark:text-neutral-400">
            Gérez votre formule et débloquez les fonctionnalités avancées de PressingOS.
          </p>
        </div>
        {!fromApi && (
          <span className="inline-flex w-fit items-center gap-1.5 rounded-full border border-amber-200 bg-amber-50 px-2.5 py-1 text-[11px] font-medium text-amber-700 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-300">
            <WifiOff className="h-3 w-3" />
            Mode hors-ligne — formule par défaut appliquée
          </span>
        )}
      </div>

      {/* État courant */}
      <div className="rounded-2xl border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
        {loading ? (
          <div className="flex items-center gap-2 text-sm text-neutral-500 dark:text-neutral-400">
            <Loader2 className="h-4 w-4 animate-spin" />
            Chargement de votre abonnement…
          </div>
        ) : (
          <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
            <div className="flex items-center gap-4">
              <div
                className={`flex h-14 w-14 items-center justify-center rounded-xl ${
                  data.on_trial
                    ? 'bg-amber-100 text-amber-600 dark:bg-amber-950/40 dark:text-amber-400'
                    : 'bg-emerald-100 text-emerald-600 dark:bg-emerald-950/40 dark:text-emerald-400'
                }`}
              >
                {data.on_trial ? <Sparkles className="h-7 w-7" /> : <ShieldCheck className="h-7 w-7" />}
              </div>
              <div>
                <div className="flex items-center gap-2">
                  <span className="text-base font-semibold text-neutral-900 dark:text-neutral-100">
                    Plan {data.plan_name || data.plan_code}
                  </span>
                  <span
                    className={`rounded-full px-2 py-0.5 text-[11px] font-medium ${
                      data.on_trial
                        ? 'bg-amber-100 text-amber-700 dark:bg-amber-950/50 dark:text-amber-300'
                        : 'bg-emerald-100 text-emerald-700 dark:bg-emerald-950/50 dark:text-emerald-300'
                    }`}
                  >
                    {STATUS_LABELS[data.status] || data.status}
                  </span>
                </div>
                <p className="mt-1 max-w-lg text-xs text-neutral-500 dark:text-neutral-400">
                  {data.on_trial
                    ? `Toutes les fonctionnalités sont débloquées pendant l'essai${
                        typeof data.days_left_trial === 'number'
                          ? ` — ${data.days_left_trial} jour(s) restant(s)`
                          : ''
                      }. Sans action, votre compte repassera au plan Gratuit (aucune donnée supprimée).`
                    : ordersLimit
                      ? `Votre formule inclut ${ordersLimit} commandes par mois.`
                      : 'Commandes illimitées incluses dans votre formule.'}
                </p>
              </div>
            </div>
            {ordersLimit && (
              <div className="w-full md:w-64">
                <div className="mb-1 flex items-center justify-between text-[11px] text-neutral-500 dark:text-neutral-400">
                  <span>Commandes ce mois</span>
                  <span className="font-mono tabular-nums">
                    {ordersEntitlement?.usage ?? 0} / {ordersLimit}
                  </span>
                </div>
                <div className="h-2 overflow-hidden rounded-full bg-neutral-100 dark:bg-neutral-800">
                  <div
                    className={`h-full rounded-full ${
                      (ordersEntitlement?.remaining ?? 1) <= 0
                        ? 'bg-red-500'
                        : (ordersEntitlement?.remaining ?? ordersLimit) <= 10
                          ? 'bg-amber-500'
                          : 'bg-emerald-500'
                    }`}
                    style={{
                      width: `${Math.min(
                        100,
                        Math.round(((ordersEntitlement?.usage ?? 0) / ordersLimit) * 100)
                      )}%`,
                    }}
                  />
                </div>
              </div>
            )}
          </div>
        )}
      </div>
      {/* Titre section */}
      <div>
        <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">Choisir une formule</h2>
        <p className="text-xs text-neutral-500 dark:text-neutral-400">
          Changez à tout moment. Le paiement est validé manuellement par notre équipe.
        </p>
      </div>

      {/* Cartes des formules */}
      <div className="grid grid-cols-1 gap-5 md:grid-cols-3">
        {PLANS.map((plan) => {
          const Icon = plan.icon;
          const isCurrent = data.plan_code === plan.code && !data.on_trial;
          return (
            <div
              key={plan.code}
              className={`relative flex flex-col rounded-2xl border bg-white p-6 transition-shadow dark:bg-neutral-900 ${
                plan.popular
                  ? 'border-neutral-950 shadow-[0_2px_6px_rgba(17,17,17,0.09),0_26px_55px_-22px_rgba(17,17,17,0.6)] ring-1 ring-neutral-900/15 -translate-y-0.5 dark:border-neutral-100 dark:ring-neutral-100/20 dark:shadow-[0_2px_8px_rgba(0,0,0,0.75),0_30px_65px_-24px_rgba(0,0,0,1)]'
                  : 'border-neutral-200 shadow-[0_1px_2px_rgba(17,17,17,0.06),0_10px_30px_-18px_rgba(17,17,17,0.35)] hover:-translate-y-0.5 hover:border-neutral-950 hover:shadow-[0_2px_5px_rgba(17,17,17,0.08),0_22px_48px_-20px_rgba(17,17,17,0.55)] dark:border-neutral-800 dark:shadow-[0_1px_2px_rgba(0,0,0,0.55),0_12px_32px_-18px_rgba(0,0,0,0.85)] dark:hover:border-white/70'
              }`}
            >
              {plan.popular && (
                <span className="absolute -top-3 left-6 rounded-full bg-neutral-900 px-2.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-white dark:bg-neutral-100 dark:text-neutral-900">
                  Le plus choisi
                </span>
              )}
              <div className="flex items-center gap-3">
                <div
                  className={`flex h-10 w-10 items-center justify-center rounded-lg ${
                    plan.popular
                      ? 'bg-neutral-900 text-white dark:bg-neutral-100 dark:text-neutral-900'
                      : 'bg-neutral-100 text-neutral-600 dark:bg-neutral-800 dark:text-neutral-300'
                  }`}
                >
                  <Icon className="h-5 w-5" />
                </div>
                <h3 className="text-base font-semibold text-neutral-900 dark:text-neutral-100">
                  {plan.name}
                </h3>
              </div>

              <p className="mt-3 min-h-[2.5rem] text-xs text-neutral-500 dark:text-neutral-400">
                {plan.tagline}
              </p>

              <div className="mt-2 flex items-baseline gap-1">
                <span className="text-2xl font-bold tabular-nums text-neutral-900 dark:text-neutral-100">
                  {plan.price === 0 ? 'Gratuit' : formatXOF(plan.price)}
                </span>
                {plan.price > 0 && (
                  <span className="text-xs text-neutral-500 dark:text-neutral-400">/ mois</span>
                )}
              </div>

              <ul className="mt-4 flex-1 space-y-2">
                {plan.highlights.map((h) => (
                  <li
                    key={h}
                    className="flex items-start gap-2 text-xs text-neutral-600 dark:text-neutral-300"
                  >
                    <Check className="mt-0.5 h-3.5 w-3.5 shrink-0 text-emerald-500" />
                    <span>{h}</span>
                  </li>
                ))}
              </ul>

              {plan.code === 'FREE' ? (
                <button
                  disabled
                  className="mt-6 w-full cursor-not-allowed rounded-lg bg-neutral-100 px-4 py-2.5 text-xs font-semibold text-neutral-400 dark:bg-neutral-800 dark:text-neutral-500"
                >
                  Formule de base
                </button>
              ) : isCurrent ? (
                <div className="mt-6 flex w-full items-center justify-center gap-1.5 rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-2.5 text-xs font-semibold text-emerald-700 dark:border-emerald-900/50 dark:bg-emerald-950/30 dark:text-emerald-300">
                  <ShieldCheck className="h-4 w-4" />
                  Formule actuelle
                </div>
              ) : (
                <button
                  onClick={() => handleRequest(plan.code, plan.name)}
                  className={`mt-6 w-full rounded-lg px-4 py-2.5 text-xs font-semibold transition-colors ${
                    requested === plan.code
                      ? 'bg-emerald-600 text-white'
                      : plan.popular
                        ? 'bg-neutral-900 text-white hover:bg-black dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-neutral-200'
                        : 'bg-neutral-100 text-neutral-900 hover:bg-neutral-950 hover:text-white dark:bg-neutral-800 dark:text-neutral-100 dark:hover:bg-neutral-700 dark:hover:bg-neutral-100 dark:hover:text-neutral-950'
                  }`}
                >
                  {requested === plan.code ? (
                    <span className="flex items-center justify-center gap-1.5">
                      <Check className="h-4 w-4" /> Demande envoyée
                    </span>
                  ) : (
                    `Passer à ${plan.name}`
                  )}
                </button>
              )}
            </div>
          );
        })}
      </div>

      {/* Pied de page informatif */}
      <div className="rounded-2xl border border-neutral-200 bg-neutral-50 p-4 text-center text-xs text-neutral-500 dark:border-neutral-800 dark:bg-neutral-900/40 dark:text-neutral-400 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
        Besoin d'une formule sur-mesure ou d'une facturation groupée multi-sites ?{' '}
        <span className="font-medium text-neutral-700 dark:text-neutral-200">
          Contactez notre équipe commerciale
        </span>{' '}
        — nous adapterons l'offre à votre réseau de pressings.
      </div>
    </div>
  );
};
