/**
 * Calculs d'analyse du tableau de bord : séries temporelles, recouvrement des
 * règlements et parcours du linge dans l'atelier.
 *
 * Les fonctions sont pures et reçoivent les commandes telles que l'interface les
 * manipule (`Order.receivedAt` est déjà une chaîne lisible). Elles acceptent les
 * trois formats produits par l'application : date + heure de l'API
 * (« 11/10/2026 09:30 »), date ISO, et libellés de démonstration
 * (« Aujourd'hui à 08:30 », « Hier à 14:00 », « 09 Octobre à 10:00 »).
 */
import { Order, OrderStatus } from '../types';

const MOIS_COURTS = [
  'janv.',
  'févr.',
  'mars',
  'avr.',
  'mai',
  'juin',
  'juil.',
  'août',
  'sept.',
  'oct.',
  'nov.',
  'déc.',
];

const MOIS_LONGS = [
  'janvier',
  'février',
  'mars',
  'avril',
  'mai',
  'juin',
  'juillet',
  'août',
  'septembre',
  'octobre',
  'novembre',
  'décembre',
];

/** Supprime accents et majuscules pour comparer des libellés de mois. */
function sansAccent(valeur: string): string {
  return valeur
    .toLowerCase()
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '');
}

/** Clé de mois « aaaa-mm ». */
export function cleMois(date: Date): string {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}`;
}

/** Clé de jour « aaaa-mm-jj ». */
export function cleJour(date: Date): string {
  return `${cleMois(date)}-${String(date.getDate()).padStart(2, '0')}`;
}

/**
 * Convertit la date de dépôt en `Date` locale. Renvoie `null` si le format
 * n'est pas reconnu — la commande est alors ignorée des séries.
 */
export function dateDepot(receivedAt: string, reference: Date = new Date()): Date | null {
  const valeur = (receivedAt ?? '').trim();
  if (!valeur) return null;

  const numerique = valeur.match(/^(\d{2})\/(\d{2})\/(\d{4})(?:[ T](\d{2}):(\d{2}))?/);
  if (numerique) {
    return new Date(
      +numerique[3],
      +numerique[2] - 1,
      +numerique[1],
      +(numerique[4] ?? 0),
      +(numerique[5] ?? 0)
    );
  }

  const iso = valeur.match(/^(\d{4})-(\d{2})-(\d{2})(?:[ T](\d{2}):(\d{2}))?/);
  if (iso) {
    return new Date(+iso[1], +iso[2] - 1, +iso[3], +(iso[4] ?? 0), +(iso[5] ?? 0));
  }

  const heures = valeur.match(/(\d{1,2}):(\d{2})/);
  const h = +(heures?.[1] ?? 0);
  const mn = +(heures?.[2] ?? 0);
  const normalise = sansAccent(valeur);

  if (normalise.startsWith('aujourd')) {
    const jour = new Date(reference);
    jour.setHours(h, mn, 0, 0);
    return jour;
  }
  if (normalise.startsWith('hier')) {
    const jour = new Date(reference);
    jour.setDate(jour.getDate() - 1);
    jour.setHours(h, mn, 0, 0);
    return jour;
  }

  const jourMois = normalise.match(/^(\d{1,2})\s+([a-z]+)/);
  if (jourMois) {
    const index = MOIS_LONGS.map(sansAccent).indexOf(jourMois[2]);
    if (index >= 0) return new Date(reference.getFullYear(), index, +jourMois[1], h, mn);
  }

  return null;
}

export interface PointMois {
  /** Clé « aaaa-mm ». */
  mois: string;
  /** Libellé court affiché sur l'axe : « oct. 26 ». */
  label: string;
  /** Encaissements de la période (somme des montants réglés). */
  encaisse: number;
  /** Montant facturé (somme des totaux des dépôts). */
  facture: number;
  /** Nombre de dépôts enregistrés. */
  depots: number;
}

/**
 * Encaissements et facturation des derniers mois **clos ou en cours** par rapport
 * au dépôt le plus récent : les mois sans activité restent affichés à zéro pour
 * que l'axe du temps reste continu.
 */
export function serieMensuelle(orders: Order[], nbMois = 6): PointMois[] {
  const dates = orders
    .map((commande) => dateDepot(commande.receivedAt))
    .filter((date): date is Date => date !== null);
  if (dates.length === 0) return [];

  const dernier = new Date(Math.max(...dates.map((date) => date.getTime())));
  const points = new Map<string, PointMois>();
  for (let recul = nbMois - 1; recul >= 0; recul -= 1) {
    const mois = new Date(dernier.getFullYear(), dernier.getMonth() - recul, 1);
    const cle = cleMois(mois);
    points.set(cle, {
      mois: cle,
      label: `${MOIS_COURTS[mois.getMonth()]} ${String(mois.getFullYear()).slice(2)}`,
      encaisse: 0,
      facture: 0,
      depots: 0,
    });
  }

  for (const commande of orders) {
    const date = dateDepot(commande.receivedAt);
    if (!date) continue;
    const point = points.get(cleMois(date));
    if (!point) continue;
    point.encaisse += commande.paidAmount;
    point.facture += commande.totalAmount;
    point.depots += 1;
  }

  return [...points.values()];
}

export interface PointJour {
  /** Clé « aaaa-mm-jj ». */
  jour: string;
  /** Libellé court affiché sur l'axe : « 11/10 ». */
  label: string;
  /** Dépôts enregistrés ce jour-là. */
  depots: number;
  /** Pièces déposées (somme des quantités d'articles). */
  pieces: number;
  /** Encaissements du jour. */
  encaisse: number;
}

/**
 * Activité des derniers **jours d'activité** : on retient les journées ayant au
 * moins un dépôt plutôt que les derniers jours calendaires, afin que le
 * graphique reste lisible quand l'établissement ferme le dimanche ou que la
 * vitrine de démonstration ne couvre pas toute la semaine.
 */
export function serieJournaliere(orders: Order[], jours = 7): PointJour[] {
  const parJour = new Map<string, PointJour>();
  for (const commande of orders) {
    const date = dateDepot(commande.receivedAt);
    if (!date) continue;
    const cle = cleJour(date);
    const point = parJour.get(cle) ?? {
      jour: cle,
      label: `${String(date.getDate()).padStart(2, '0')}/${String(date.getMonth() + 1).padStart(2, '0')}`,
      depots: 0,
      pieces: 0,
      encaisse: 0,
    };
    point.depots += 1;
    point.pieces += commande.items.reduce((somme, item) => somme + item.quantity, 0);
    point.encaisse += commande.paidAmount;
    parJour.set(cle, point);
  }

  return [...parJour.values()].sort((a, b) => a.jour.localeCompare(b.jour)).slice(-jours);
}

export interface TrancheReglement {
  etat: 'solde' | 'acompte' | 'reste';
  label: string;
  /** Montant en FCFA. */
  value: number;
}

export interface Recouvrement {
  tranches: TrancheReglement[];
  /** Total encaissé (dépôts réglés + acomptes). */
  encaisse: number;
  /** Total facturé (somme des tickets). */
  facture: number;
  /** Solde restant à percevoir au comptoir. */
  reste: number;
  /** Part encaissée, en pourcentage arrondi au dixième. */
  taux: number;
}

/** Répartition des règlements : dépôts soldés, acomptes versés et reste à percevoir. */
export function recouvrement(orders: Order[]): Recouvrement {
  let solde = 0;
  let acompte = 0;
  let facture = 0;
  let encaisse = 0;

  for (const commande of orders) {
    facture += commande.totalAmount;
    encaisse += commande.paidAmount;
    if (commande.paymentStatus === 'paid') solde += commande.paidAmount;
    else if (commande.paymentStatus === 'partially_paid') acompte += commande.paidAmount;
  }

  const reste = Math.max(0, facture - encaisse);
  return {
    tranches: [
      { etat: 'solde', label: 'Dépôts soldés', value: solde },
      { etat: 'acompte', label: 'Acomptes versés', value: acompte },
      { etat: 'reste', label: 'Reste à percevoir', value: reste },
    ],
    encaisse,
    facture,
    reste,
    taux: facture > 0 ? Math.round((encaisse / facture) * 1000) / 10 : 0,
  };
}

export interface EtapePipeline {
  etape: OrderStatus | 'depot';
  label: string;
  /** Nombre de dépôts ayant atteint l'étape (cumul depuis la réception). */
  value: number;
}

const ORDRE_STATUTS: OrderStatus[] = ['received', 'washing', 'ironing', 'ready', 'delivered'];

const LIBELLES_ETAPES: Record<OrderStatus, string> = {
  received: 'Dépôts réceptionnés',
  washing: 'Passés au lavage',
  ironing: 'Repassés et finis',
  ready: 'Prêts au comptoir',
  delivered: 'Livrés au client',
};

/**
 * Entonnoir du parcours du linge : à partir du statut courant de chaque dépôt,
 * on cumule les commandes ayant atteint chaque étape. La dernière marche donne
 * le taux de conversion réception → livraison.
 */
export function pipelineAtelier(orders: Order[]): EtapePipeline[] {
  return ORDRE_STATUTS.map((statut, index) => ({
    etape: statut,
    label: LIBELLES_ETAPES[statut],
    value: orders.filter((commande) => ORDRE_STATUTS.indexOf(commande.status) >= index).length,
  }));
}

/** Montant compact pour les axes : « 12 k » plutôt que « 12 000 FCFA ». */
export function montantCourt(valeur: number): string {
  if (Math.abs(valeur) >= 1_000_000)
    return `${(Math.round(valeur / 100_000) / 10).toLocaleString('fr-FR')} M`;
  if (Math.abs(valeur) >= 1000) return `${Math.round(valeur / 1000).toLocaleString('fr-FR')} k`;
  return valeur.toLocaleString('fr-FR');
}
