/**
 * Module Abonnement — lecture des droits effectifs et de la grille tarifaire.
 *
 * Source de vérité : le backend. Aucun tarif, aucun plafond et aucune date n'est
 * codé ici : l'écran affiche uniquement ce que l'API renvoie. En cas d'échec, le
 * module renvoie une erreur typée ; il n'existe pas de données de repli.
 */

export interface Entitlement {
  key: string;
  enabled: boolean;
  limit?: number | null;
  usage?: number | null;
  remaining?: number | null;
  source: string;
}

/** Intervalle semi-ouvert [start, end[ (end exclu), en ISO 8601. */
export interface Periode {
  start: string;
  end: string;
}

export interface EntitlementsResponse {
  tenant_id: string;
  /** Formule effective : celle dont les droits s'appliquent (BUSINESS pendant l'essai). */
  plan_code: string;
  /** Formule réellement souscrite (FREE pendant l'essai). Sert à marquer la formule en cours. */
  subscribed_plan_code: string;
  plan_name: string;
  status: string;
  on_trial: boolean;
  trial_ends_at?: string | null;
  days_left_trial?: number | null;
  entitlements: Entitlement[];
  /** Tarif mensuel de la formule souscrite, lu en base (absent en mode dégradé). */
  price_monthly_xof?: number | null;
  /** Fenêtre de 30 jours sur laquelle le quota de commandes est compté. */
  orders_period?: Periode | null;
}

export interface PlanCatalogue {
  code: string;
  name: string;
  price_monthly_xof: number | null;
  entitlements: Entitlement[];
}

export type BillingErrorKind = 'auth' | 'forbidden' | 'network' | 'server' | 'invalid';

export interface BillingError {
  kind: BillingErrorKind;
  message: string;
  status?: number;
}

export type BillingResult<T> = { ok: true; data: T } | { ok: false; error: BillingError };

const MESSAGES: Record<BillingErrorKind, string> = {
  auth: 'Votre session a expiré. Reconnectez-vous pour consulter votre abonnement.',
  forbidden: "Accès refusé : cet abonnement n'est pas accessible avec votre session.",
  network: 'Vérifiez votre réseau puis réessayez.',
  server: "Le service d'abonnement est momentanément indisponible. Réessayez dans un instant.",
  invalid: 'Réponse inattendue du serveur. Réessayez ou contactez le support.',
};

function authHeaders(token?: string): Record<string, string> {
  const headers: Record<string, string> = { Accept: 'application/json' };
  if (token) headers.Authorization = `Bearer ${token}`;
  return headers;
}

/** Traduit une réponse HTTP non réussie en erreur typée. */
function erreurHttp(status: number): BillingError {
  if (status === 401) return { kind: 'auth', message: MESSAGES.auth, status };
  if (status === 403) return { kind: 'forbidden', message: MESSAGES.forbidden, status };
  return { kind: 'server', message: MESSAGES.server, status };
}

async function lire<T>(url: string, token?: string): Promise<BillingResult<T>> {
  let res: Response;
  try {
    res = await fetch(url, { headers: authHeaders(token) });
  } catch {
    return { ok: false, error: { kind: 'network', message: MESSAGES.network } };
  }
  if (!res.ok) return { ok: false, error: erreurHttp(res.status) };
  try {
    const data = (await res.json()) as T;
    return { ok: true, data };
  } catch {
    return { ok: false, error: { kind: 'invalid', message: MESSAGES.invalid, status: res.status } };
  }
}

/** Droits effectifs de l'établissement connecté (formule, essai, quotas, fenêtre). */
export function fetchEntitlements(token?: string): Promise<BillingResult<EntitlementsResponse>> {
  return lire<EntitlementsResponse>('/api/v1/me/entitlements', token).then((r) => {
    if (!r.ok) return r;
    if (
      !r.data ||
      typeof r.data.plan_code !== 'string' ||
      typeof r.data.subscribed_plan_code !== 'string' ||
      !Array.isArray(r.data.entitlements)
    ) {
      return { ok: false, error: { kind: 'invalid', message: MESSAGES.invalid } };
    }
    return r;
  });
}

/** Grille des formules publiques (tarif en base et droits). */
export function fetchPlans(token?: string): Promise<BillingResult<PlanCatalogue[]>> {
  return lire<{ plans?: PlanCatalogue[] }>('/api/v1/billing/plans', token).then((r) => {
    if (!r.ok) return r;
    const plans = r.data?.plans;
    if (!Array.isArray(plans)) {
      return { ok: false, error: { kind: 'invalid', message: MESSAGES.invalid } };
    }
    return { ok: true, data: plans };
  });
}

const LIBELLES_DROITS: Record<string, string> = {
  'orders.create': 'Création de commandes',
  'orders.monthly.max': 'Commandes (fenêtre de 30 jours)',
  'users.max': 'Comptes employés',
  'catalog.price_lists': 'Grilles tarifaires',
  'branches.multi_site': 'Établissements',
  'reports.advanced': 'Rapports avancés',
  'reports.consolidated': 'Rapports consolidés',
  'expenses.advanced': 'Dépenses avancées',
  'notifications.advanced': 'Notifications avancées',
  'pricing.pro': 'Tarification PRO',
  'audit.advanced': 'Audit avancé',
};

/** Libellé français d'un droit ; à défaut, la clé technique telle quelle. */
export function libelleDroit(key: string): string {
  return LIBELLES_DROITS[key] ?? key;
}

/** Valeur affichable d'un droit : « non inclus », « illimité », un nombre ou « inclus ». */
export function valeurDroit(e: Entitlement): string {
  if (!e.enabled) return 'non inclus';
  if (e.limit === null || e.limit === undefined) {
    // Un plafond « .max » sans valeur signifie qu'il n'y a pas de plafond.
    return e.key.endsWith('.max') ? 'illimité' : 'inclus';
  }
  return String(e.limit);
}

const STATUTS: Record<string, string> = {
  TRIALING: 'Essai gratuit',
  ACTIVE: 'Abonnement actif',
  PAST_DUE: 'Paiement en retard',
  CANCELED: 'Résilié',
  SUSPENDED: 'Suspendu',
};

/** Libellé du statut d'abonnement ; à défaut, le code brut. */
export function libelleStatut(status: string): string {
  return STATUTS[status] ?? status;
}

const formatDateFr = new Intl.DateTimeFormat('fr-FR', {
  day: '2-digit',
  month: 'long',
  year: 'numeric',
  hour: '2-digit',
  minute: '2-digit',
  timeZone: 'Africa/Abidjan',
});

/** Date et heure en français (fuseau d'Abidjan), ou null si la valeur est invalide. */
export function formatDate(iso?: string | null): string | null {
  if (!iso) return null;
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return null;
  return formatDateFr.format(d);
}

/** Montant en francs CFA ; null si le montant n'est pas connu. */
export function formatXOF(montant: number | null | undefined): string | null {
  if (montant === null || montant === undefined || Number.isNaN(montant)) return null;
  return `${montant.toLocaleString('fr-FR')} FCFA`;
}
