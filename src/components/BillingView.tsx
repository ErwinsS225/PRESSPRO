import React, { useEffect, useState } from 'react';
import {
  AlertTriangle,
  Building2,
  Check,
  Crown,
  Info,
  Loader2,
  Mail,
  RefreshCw,
  ShieldCheck,
  Sparkles,
  Store,
  Zap,
} from 'lucide-react';
import {
  BillingError,
  EntitlementsResponse,
  PlanCatalogue,
  BillingResult,
  fetchEntitlements,
  fetchPlans,
  formatDate,
  formatXOF,
  libelleDroit,
  libelleStatut,
  valeurDroit,
} from '../lib/billingApi';

/**
 * BillingView — écran « Mon abonnement ».
 *
 * Tout ce qui est affiché vient de l'API : formules et tarifs (GET /api/v1/billing/plans),
 * droits effectifs, essai, fenêtre de commandes (GET /api/v1/me/entitlements). Aucun
 * prix ni plafond n'est codé ici, et il n'existe pas de données de repli : une erreur
 * de chargement est signalée, avec une action pour réessayer.
 *
 * Le changement de formule n'est pas encore branché sur un paiement : l'écran ne
 * prétend donc pas enregistrer de demande. Il indique comment la traiter.
 */

/** Adresse de contact de l'éditeur, optionnelle (VITE_BILLING_CONTACT_EMAIL). */
const CONTACT_EDITEUR: string = String(import.meta.env.VITE_BILLING_CONTACT_EMAIL ?? '').trim();

const ICONES: Record<string, React.ElementType> = {
  FREE: Store,
  PRO: Zap,
  BUSINESS: Building2,
};

const CODE_ORDRES = 'orders.monthly.max';

const MESSAGE_ERREUR_TITRE: Record<BillingError['kind'], string> = {
  auth: 'Session expirée',
  forbidden: 'Accès refusé',
  network: 'Connexion impossible',
  server: 'Service indisponible',
  invalid: 'Réponse inattendue',
};

function Erreur({
  erreur,
  onReessayer,
  titre,
}: {
  erreur: BillingError;
  onReessayer: () => void;
  titre: string;
}) {
  return (
    <div
      role="alert"
      className="flex flex-col gap-3 rounded-2xl border border-red-200 bg-red-50 p-4 text-sm text-red-800 dark:border-red-900/50 dark:bg-red-950/30 dark:text-red-200 sm:flex-row sm:items-center sm:justify-between"
    >
      <div className="flex items-start gap-2">
        <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
        <div>
          <p className="font-semibold">
            {titre} — {MESSAGE_ERREUR_TITRE[erreur.kind]}
          </p>
          <p className="text-xs">{erreur.message}</p>
        </div>
      </div>
      <button
        type="button"
        onClick={onReessayer}
        className="inline-flex w-fit items-center gap-1.5 rounded-lg border border-red-300 bg-white px-3 py-1.5 text-xs font-medium text-red-700 hover:bg-red-100 dark:border-red-800 dark:bg-neutral-900 dark:text-red-300"
      >
        <RefreshCw className="h-3.5 w-3.5" />
        Réessayer
      </button>
    </div>
  );
}

function ListeDroits({ entitlements }: { entitlements: EntitlementsResponse['entitlements'] }) {
  if (entitlements.length === 0) {
    return <p className="text-xs text-neutral-500 dark:text-neutral-400">Aucun droit n&apos;est listé.</p>;
  }
  return (
    <dl className="grid grid-cols-1 gap-x-6 gap-y-1.5 text-xs sm:grid-cols-2">
      {entitlements.map((e) => (
        <div
          key={e.key}
          className="flex items-center justify-between gap-3 border-b border-neutral-100 pb-1 dark:border-neutral-800"
        >
          <dt className="text-neutral-600 dark:text-neutral-300">{libelleDroit(e.key)}</dt>
          <dd
            className={`font-medium tabular-nums ${e.enabled ? 'text-neutral-900 dark:text-neutral-100' : 'text-neutral-400 dark:text-neutral-500'}`}
          >
            {valeurDroit(e)}
          </dd>
        </div>
      ))}
    </dl>
  );
}

export const BillingView: React.FC<{ token?: string }> = ({ token }) => {
  const [chargement, setChargement] = useState(true);
  const [droits, setDroits] = useState<BillingResult<EntitlementsResponse> | null>(null);
  const [grille, setGrille] = useState<BillingResult<PlanCatalogue[]> | null>(null);
  const [version, setVersion] = useState(0);
  const [demande, setDemande] = useState<PlanCatalogue | null>(null);

  // Chargement : rejoué à chaque actualisation (version) et au changement de session.
  useEffect(() => {
    let annule = false;
    Promise.all([fetchEntitlements(token), fetchPlans(token)]).then(([d, g]) => {
      if (annule) return;
      setDroits(d);
      setGrille(g);
      setChargement(false);
    });
    return () => {
      annule = true;
    };
  }, [token, version]);

  const actualiser = () => {
    setChargement(true);
    setVersion((v) => v + 1);
  };

  const donnees = droits?.ok ? droits.data : null;
  const meter = donnees?.entitlements.find((e) => e.key === CODE_ORDRES);
  const plafondCommandes = meter && meter.enabled && typeof meter.limit === 'number' ? meter.limit : null;
  const usageCommandes = meter?.usage ?? null;
  const prixSouscrit = formatXOF(donnees?.price_monthly_xof ?? null);
  const finEssai = formatDate(donnees?.trial_ends_at ?? null);
  const fenetre = donnees?.orders_period ?? null;
  const debutFenetre = fenetre ? formatDate(fenetre.start) : null;
  const finFenetre = fenetre ? formatDate(fenetre.end) : null;

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
            Votre formule, votre essai et les droits qui s&apos;y rattachent.
          </p>
        </div>
        <button
          type="button"
          onClick={actualiser}
          disabled={chargement}
          aria-label="Actualiser l'abonnement"
          className="inline-flex w-fit items-center gap-1.5 rounded-lg border border-neutral-200 bg-white px-3 py-1.5 text-xs font-medium text-neutral-700 hover:border-neutral-950 disabled:opacity-50 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-200"
        >
          {chargement ? (
            <Loader2 className="h-3.5 w-3.5 animate-spin" />
          ) : (
            <RefreshCw className="h-3.5 w-3.5" />
          )}
          Actualiser
        </button>
      </div>

      {/* Situation de l'établissement */}
      {droits && !droits.ok ? (
        <Erreur
          erreur={droits.error}
          onReessayer={actualiser}
          titre="Situation de l'abonnement indisponible"
        />
      ) : !donnees ? (
        <div className="flex items-center gap-2 rounded-2xl border border-neutral-200 bg-white p-5 text-sm text-neutral-500 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-400">
          <Loader2 className="h-4 w-4 animate-spin" />
          Chargement de votre abonnement…
        </div>
      ) : (
        <section
          aria-labelledby="situation-abonnement"
          className="space-y-5 rounded-2xl border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900"
        >
          <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
            <div className="flex items-center gap-4">
              <div
                className={`flex h-14 w-14 items-center justify-center rounded-xl ${
                  donnees.on_trial
                    ? 'bg-amber-100 text-amber-600 dark:bg-amber-950/40 dark:text-amber-400'
                    : 'bg-emerald-100 text-emerald-600 dark:bg-emerald-950/40 dark:text-emerald-400'
                }`}
              >
                {donnees.on_trial ? <Sparkles className="h-7 w-7" /> : <ShieldCheck className="h-7 w-7" />}
              </div>
              <div>
                <div className="flex flex-wrap items-center gap-2">
                  <h2
                    id="situation-abonnement"
                    className="text-base font-semibold text-neutral-900 dark:text-neutral-100"
                  >
                    Formule souscrite : {donnees.plan_name || donnees.plan_code}
                  </h2>
                  <span className="rounded-full bg-neutral-100 px-2 py-0.5 text-[11px] font-medium text-neutral-700 dark:bg-neutral-800 dark:text-neutral-200">
                    {libelleStatut(donnees.status)}
                  </span>
                </div>
                {prixSouscrit && (
                  <p className="mt-0.5 text-xs text-neutral-600 dark:text-neutral-300">
                    {prixSouscrit} / mois
                  </p>
                )}
              </div>
            </div>
          </div>

          {donnees.on_trial && (
            <div className="rounded-xl border border-amber-200 bg-amber-50 p-4 text-xs text-amber-900 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-200">
              <p className="font-semibold">Essai gratuit en cours</p>
              <p className="mt-1">
                Pendant l&apos;essai, les droits effectifs sont ceux de la formule la plus complète.
                {typeof donnees.days_left_trial === 'number' &&
                  ` ${donnees.days_left_trial} jour(s) restant(s).`}
              </p>
              {finEssai && <p className="mt-1">Fin de l&apos;essai : {finEssai}.</p>}
              <p className="mt-1">
                Sans passage à une formule payante, le compte repassera à la formule{' '}
                <strong>{donnees.plan_name || donnees.plan_code}</strong> à cette date. Aucune donnée
                n&apos;est supprimée.
              </p>
            </div>
          )}

          {plafondCommandes !== null && (
            <div className="max-w-xl">
              <div className="mb-1 flex items-center justify-between text-[11px] text-neutral-500 dark:text-neutral-400">
                <span>Commandes sur la fenêtre en cours</span>
                <span className="font-mono tabular-nums">
                  {usageCommandes ?? '—'} / {plafondCommandes}
                </span>
              </div>
              <div
                className="h-2 overflow-hidden rounded-full bg-neutral-100 dark:bg-neutral-800"
                role="progressbar"
                aria-label="Commandes sur la fenêtre en cours"
                aria-valuemin={0}
                aria-valuemax={plafondCommandes}
                aria-valuenow={usageCommandes ?? 0}
              >
                <div
                  className={`h-full rounded-full ${
                    (meter?.remaining ?? 1) <= 0
                      ? 'bg-red-500'
                      : (meter?.remaining ?? plafondCommandes) <=
                          Math.max(1, Math.round(plafondCommandes * 0.2))
                        ? 'bg-amber-500'
                        : 'bg-emerald-500'
                  }`}
                  style={{
                    width: `${Math.min(100, Math.round(((usageCommandes ?? 0) / Math.max(1, plafondCommandes)) * 100))}%`,
                  }}
                />
              </div>
              {debutFenetre && finFenetre && (
                <p className="mt-1 text-[11px] text-neutral-500 dark:text-neutral-400">
                  Période du {debutFenetre} au {finFenetre} (fin exclue). Les commandes annulées ne comptent
                  pas.
                </p>
              )}
            </div>
          )}

          <div>
            <h3 className="mb-2 text-xs font-semibold text-neutral-900 dark:text-neutral-100">
              Droits effectifs
            </h3>
            <ListeDroits entitlements={donnees.entitlements} />
          </div>
        </section>
      )}

      {/* Grille des formules */}
      <div>
        <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">Formules disponibles</h2>
        <p className="text-xs text-neutral-500 dark:text-neutral-400">
          Tarifs et droits fournis par l&apos;éditeur. Le passage à une autre formule est traité manuellement.
        </p>
      </div>

      {grille && !grille.ok ? (
        <Erreur erreur={grille.error} onReessayer={actualiser} titre="Grille tarifaire indisponible" />
      ) : !grille ? (
        <div className="flex items-center gap-2 text-sm text-neutral-500 dark:text-neutral-400">
          <Loader2 className="h-4 w-4 animate-spin" />
          Chargement des formules…
        </div>
      ) : grille.data.length === 0 ? (
        <p className="text-sm text-neutral-500 dark:text-neutral-400">
          Aucune formule n&apos;est proposée pour le moment.
        </p>
      ) : (
        <div className="grid grid-cols-1 gap-5 md:grid-cols-3">
          {grille.data.map((plan) => {
            const Icone = ICONES[plan.code] ?? Store;
            // Formule réellement souscrite : pendant l'essai, les droits sont ceux de BUSINESS
            // mais la formule souscrite reste celle du compte.
            const estSouscrite = donnees?.subscribed_plan_code === plan.code;
            const prix = plan.price_monthly_xof;
            const libellePrix =
              prix === null ? 'Tarif non communiqué' : prix === 0 ? 'Gratuit' : formatXOF(prix);
            return (
              <article
                key={plan.code}
                aria-label={`Formule ${plan.name}`}
                className={`relative flex flex-col rounded-2xl border bg-white p-6 dark:bg-neutral-900 ${
                  estSouscrite
                    ? 'border-emerald-300 ring-1 ring-emerald-200 dark:border-emerald-800 dark:ring-emerald-900/50'
                    : 'border-neutral-200 dark:border-neutral-800'
                }`}
              >
                {estSouscrite && (
                  <span className="absolute -top-3 left-6 rounded-full bg-emerald-600 px-2.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-white">
                    Formule souscrite
                  </span>
                )}
                <div className="flex items-center gap-3">
                  <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-neutral-100 text-neutral-600 dark:bg-neutral-800 dark:text-neutral-300">
                    <Icone className="h-5 w-5" />
                  </div>
                  <h3 className="text-base font-semibold text-neutral-900 dark:text-neutral-100">
                    {plan.name}
                  </h3>
                </div>
                <p className="mt-3 text-lg font-bold tabular-nums text-neutral-900 dark:text-neutral-100">
                  {libellePrix}
                  {prix !== null && prix > 0 && (
                    <>
                      {' '}
                      <span className="text-xs font-normal text-neutral-500">/ mois</span>
                    </>
                  )}
                </p>
                <div className="mt-4 flex-1">
                  <ListeDroits entitlements={plan.entitlements} />
                </div>
                {!estSouscrite && (
                  <button
                    type="button"
                    onClick={() => setDemande(plan)}
                    className="mt-6 w-full rounded-lg bg-neutral-900 px-4 py-2.5 text-xs font-semibold text-white hover:bg-black dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-neutral-200"
                  >
                    Demander le passage à {plan.name}
                  </button>
                )}
              </article>
            );
          })}
        </div>
      )}

      {/* Changement de formule : aucune demande n'est enregistrée par l'écran */}
      {demande && (
        <div
          role="status"
          aria-live="polite"
          className="flex flex-col gap-3 rounded-2xl border border-neutral-300 bg-neutral-50 p-4 text-xs text-neutral-700 dark:border-neutral-700 dark:bg-neutral-900/60 dark:text-neutral-200 sm:flex-row sm:items-start sm:justify-between"
        >
          <div className="flex items-start gap-2">
            <Info className="mt-0.5 h-4 w-4 shrink-0" />
            <div>
              <p className="font-semibold">Changement vers {demande.name}</p>
              <p className="mt-1">
                Cet écran n&apos;enregistre aucune demande et n&apos;active aucune formule. Le changement est
                traité par l&apos;éditeur PressingOS après validation manuelle du paiement (Mobile Money ou
                virement).
              </p>
              {CONTACT_EDITEUR ? (
                <a
                  href={`mailto:${CONTACT_EDITEUR}?subject=${encodeURIComponent(`Changement de formule : ${demande.name}`)}`}
                  className="mt-2 inline-flex items-center gap-1.5 font-medium text-neutral-900 underline dark:text-neutral-100"
                >
                  <Mail className="h-3.5 w-3.5" />
                  Écrire à l&apos;éditeur ({CONTACT_EDITEUR})
                </a>
              ) : (
                <p className="mt-2 font-medium">
                  Contactez l&apos;éditeur PressingOS pour finaliser ce changement.
                </p>
              )}
            </div>
          </div>
          <button
            type="button"
            onClick={() => setDemande(null)}
            className="inline-flex w-fit items-center gap-1.5 rounded-lg border border-neutral-300 bg-white px-3 py-1.5 text-xs font-medium text-neutral-700 dark:border-neutral-700 dark:bg-neutral-900 dark:text-neutral-200"
          >
            <Check className="h-3.5 w-3.5" />
            Fermer
          </button>
        </div>
      )}
    </div>
  );
};
