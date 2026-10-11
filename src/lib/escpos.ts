/**
 * Impression thermique native ESC/POS (imprimantes 80 mm : Epson TM-T20,
 * Xprinter XP-80, clones génériques…).
 *
 * Deux parties bien séparées :
 *  1. la CONSTRUCTION du flux d'octets est pure et testable hors navigateur
 *     (`buildTicketBytes`, `encodeEscPos`) — c'est elle qui définit le rendu ;
 *  2. le TRANSPORT utilise WebUSB (`sendToEscPosPrinter`) quand le navigateur
 *     l'expose (Chrome/Edge sur desktop), sinon l'application retombe sur
 *     l'impression système du ticket HTML.
 *
 * `renderEscPosPreview` relit le flux comme le ferait l'imprimante : c'est
 * l'aperçu affiché à l'écran et la base des tests de rendu.
 */

import type { Order, Tenant } from '../types';

export const ESC = 0x1b;
export const GS = 0x1d;

/** Largeur utile d'un ticket 80 mm en police A (42 colonnes). */
export const LARGEUR_TICKET = 42;

/**
 * Table CP437 (page de codes standard des imprimantes ESC/POS) pour les
 * caractères français : les imprimantes thermiques ne parlent pas UTF-8.
 */
const CP437: Record<string, number> = {
  Ç: 0x80,
  ü: 0x81,
  é: 0x82,
  â: 0x83,
  ä: 0x84,
  à: 0x85,
  å: 0x86,
  ç: 0x87,
  ê: 0x88,
  ë: 0x89,
  è: 0x8a,
  ï: 0x8b,
  î: 0x8c,
  ì: 0x8d,
  Ä: 0x8e,
  Å: 0x8f,
  É: 0x90,
  æ: 0x91,
  Æ: 0x92,
  ô: 0x93,
  ö: 0x94,
  ò: 0x95,
  û: 0x96,
  ù: 0x97,
  ÿ: 0x98,
  Ö: 0x99,
  Ü: 0x9a,
  á: 0xa0,
  í: 0xa1,
  ó: 0xa2,
  ú: 0xa3,
  ñ: 0xa4,
  Ñ: 0xa5,
  '¿': 0xa8,
  '°': 0xf8,
  '·': 0xfa,
};

/**
 * Ponctuation typographique française : ramenée à son équivalent ASCII, car
 * les imprimantes thermiques n'embarquent pas ces signes.
 */
const PONCTUATION_ASCII: Record<string, number> = {
  '’': 0x27,
  '‘': 0x27,
  '“': 0x22,
  '”': 0x22,
  '«': 0x22,
  '»': 0x22,
  '–': 0x2d,
  '—': 0x2d,
  '…': 0x2e,
  '\u202f': 0x20,
  '\u00a0': 0x20,
};

/**
 * encodeEscPos traduit un texte en octets CP437. Trois replis successifs
 * évitent tout caractère parasite sur le papier : la ponctuation typographique,
 * puis la perte d'accent (À → A, È → E…), enfin le remplacement par « ? ».
 */
export function encodeEscPos(texte: string): number[] {
  const octets: number[] = [];
  for (const caractere of Array.from(texte)) {
    const code = caractere.codePointAt(0) ?? 0x3f;
    if (code < 128) {
      octets.push(code);
      continue;
    }
    if (CP437[caractere] !== undefined) {
      octets.push(CP437[caractere]);
      continue;
    }
    if (PONCTUATION_ASCII[caractere] !== undefined) {
      octets.push(PONCTUATION_ASCII[caractere]);
      continue;
    }
    // Perte d'accent : « À » n'existe pas en CP437, mais « A » s'imprime.
    const sansAccent = caractere.normalize('NFD').replace(/\p{M}/gu, '');
    if (sansAccent.length === 1 && (sansAccent.codePointAt(0) ?? 0) < 128) {
      octets.push(sansAccent.codePointAt(0) as number);
      continue;
    }
    if (caractere === '€') {
      octets.push(...Array.from('EUR').map((c) => c.charCodeAt(0)));
      continue;
    }
    octets.push(0x3f); // '?'
  }
  return octets;
}

/** Commandes ESC/POS élémentaires. */
export const commandes = {
  init: () => [ESC, 0x40],
  pageDeCodes437: () => [ESC, 0x74, 0x00],
  aligner: (mode: 'gauche' | 'centre' | 'droite') => [
    ESC,
    0x61,
    mode === 'centre' ? 1 : mode === 'droite' ? 2 : 0,
  ],
  gras: (actif: boolean) => [ESC, 0x45, actif ? 1 : 0],
  doubleTaille: (actif: boolean) => [GS, 0x21, actif ? 0x11 : 0x00],
  avancer: (lignes: number) => [ESC, 0x64, lignes],
  couper: () => [GS, 0x56, 0x42, 0x00], // coupe partielle avec avance
  souligne: (actif: boolean) => [ESC, 0x2d, actif ? 1 : 0],
};

/** Texte + saut de ligne, encodé CP437. */
function ligne(texte: string): number[] {
  return [...encodeEscPos(texte), 0x0a];
}

/**
 * aligneGaucheDroite compose une ligne à deux colonnes de largeur fixe :
 * « Article                    1 500 ».
 */
export function aligneGaucheDroite(gauche: string, droite: string, largeur = LARGEUR_TICKET): string {
  const espace = Math.max(1, largeur - gauche.length - droite.length);
  return gauche + ' '.repeat(espace) + droite;
}

/** Séparateur en tirets sur toute la largeur utile. */
export function separateur(largeur = LARGEUR_TICKET): string {
  return '-'.repeat(largeur);
}

/**
 * Montant lisible (franc CFA entier, séparateur de milliers). Les espaces
 * fines insécables de `toLocaleString` (U+202F / U+00A0) sont ramenées à
 * l'espace simple : la page de codes CP437 ne sait pas les imprimer.
 */
export function montantFCFA(valeur: number): string {
  const montant = Math.round(valeur)
    .toLocaleString('fr-FR')
    .replace(/[\u202f\u00a0]/g, ' ');
  return `${montant} FCFA`;
}

/** Données nécessaires au rendu du ticket thermique. */
export interface TicketEscPos {
  tenantName: string;
  tenantCity?: string;
  tenantPhone?: string;
  orderNumber: string;
  receivedAt?: string;
  dueDate?: string;
  customerName: string;
  customerPhone?: string;
  items: { serviceName: string; quantity: number; subtotal: number }[];
  totalAmount: number;
  paidAmount: number;
  verifyUrl?: string;
  footer?: string;
}

/** Convertit une commande de l'application en données de ticket. */
export function ticketDepuisCommande(order: Order, tenant: Tenant, verifyUrl?: string): TicketEscPos {
  return {
    tenantName: tenant.name,
    tenantCity: tenant.city,
    tenantPhone: tenant.phone,
    orderNumber: order.orderNumber,
    receivedAt: order.receivedAt,
    dueDate: order.dueDate,
    customerName: order.customerName,
    customerPhone: order.customerPhone,
    items: order.items.map((item) => ({
      serviceName: item.serviceName,
      quantity: item.quantity,
      subtotal: item.subtotal,
    })),
    totalAmount: order.totalAmount,
    paidAmount: order.paidAmount,
    verifyUrl,
  };
}

/**
 * buildTicketBytes assemble le flux ESC/POS complet d'un ticket de dépôt :
 * en-tête du pressing en double hauteur, références, articles, totaux, reste à
 * payer, URL de vérification, avance papier et coupe.
 */
export function buildTicketBytes(
  ticket: TicketEscPos,
  options?: { largeur?: number; couper?: boolean }
): Uint8Array {
  const largeur = options?.largeur ?? LARGEUR_TICKET;
  const octets: number[] = [...commandes.init(), ...commandes.pageDeCodes437()];

  // En-tête : nom du pressing en double taille, centré.
  octets.push(...commandes.aligner('centre'), ...commandes.doubleTaille(true));
  octets.push(...ligne(ticket.tenantName.toUpperCase()));
  octets.push(...commandes.doubleTaille(false));
  const coordonnees = [ticket.tenantCity, ticket.tenantPhone].filter(Boolean).join(' · ');
  if (coordonnees) octets.push(...ligne(coordonnees));
  octets.push(...ligne(separateur(largeur)));

  // Références de la commande.
  octets.push(...commandes.aligner('gauche'), ...commandes.gras(true));
  octets.push(...ligne(`N° ${ticket.orderNumber}`));
  octets.push(...commandes.gras(false));
  if (ticket.receivedAt) octets.push(...ligne(`Dépôt    : ${ticket.receivedAt}`));
  if (ticket.dueDate) octets.push(...ligne(`Retrait  : ${ticket.dueDate}`));
  octets.push(...ligne(separateur(largeur)));

  // Client.
  octets.push(...ligne(ticket.customerName));
  if (ticket.customerPhone) octets.push(...ligne(`Tél : ${ticket.customerPhone}`));
  octets.push(...ligne(separateur(largeur)));

  // Articles : libellé tronqué pour que le montant reste aligné.
  for (const item of ticket.items) {
    const debut = `${item.quantity} x ${item.serviceName}`;
    const libelle = debut.length > largeur - 12 ? `${debut.slice(0, largeur - 13)}.` : debut;
    octets.push(...ligne(aligneGaucheDroite(libelle, montantFCFA(item.subtotal), largeur)));
  }
  octets.push(...ligne(separateur(largeur)));

  // Totaux.
  octets.push(...ligne(aligneGaucheDroite('TOTAL', montantFCFA(ticket.totalAmount), largeur)));
  octets.push(...ligne(aligneGaucheDroite('Payé', montantFCFA(ticket.paidAmount), largeur)));
  const reste = ticket.totalAmount - ticket.paidAmount;
  octets.push(...commandes.gras(true));
  octets.push(
    ...ligne(
      aligneGaucheDroite(reste > 0 ? 'RESTE À PAYER' : 'SOLDÉ', montantFCFA(Math.max(0, reste)), largeur)
    )
  );
  octets.push(...commandes.gras(false));
  octets.push(...ligne(separateur(largeur)));

  // Vérification du ticket (QR code scannable ou URL).
  if (ticket.verifyUrl) {
    octets.push(...commandes.aligner('centre'), ...ligne('Vérification du ticket'));
    octets.push(...ligne(ticket.verifyUrl));
    octets.push(...commandes.aligner('gauche'));
  }

  octets.push(...ligne(ticket.footer ?? 'Merci de votre confiance !'), ...ligne('Géré par PressingOS'));

  // Avance papier puis coupe (désactivable pour une imprimante à déchirure).
  octets.push(...commandes.avancer(3));
  if (options?.couper !== false) octets.push(...commandes.couper());

  return new Uint8Array(octets);
}

/**
 * renderEscPosPreview relit le flux comme le ferait l'imprimante et renvoie le
 * rendu texte : aperçu affiché à l'écran et vérification automatisée du rendu.
 */
export function renderEscPosPreview(flux: Uint8Array): string {
  const inverse: Record<number, string> = {};
  for (const [caractere, code] of Object.entries(CP437)) inverse[code] = caractere;

  const sortie: string[] = [];
  let courante = '';
  let doubleTaille = false;

  const pousser = () => {
    sortie.push(doubleTaille ? rendreDoubleTaille(courante) : courante);
    courante = '';
  };

  for (let i = 0; i < flux.length; i += 1) {
    const octet = flux[i];
    if (octet === ESC) {
      const commande = flux[i + 1];
      if (commande === 0x40) {
        i += 1; // init : rien à afficher
      } else if (commande === 0x74) {
        i += 2; // page de codes
      } else if (commande === 0x61 || commande === 0x45 || commande === 0x2d) {
        i += 2; // alignement / gras / soulignement
      } else if (commande === 0x64) {
        const lignes = flux[i + 2] ?? 0;
        for (let n = 0; n < lignes; n += 1) pousser();
        i += 2;
      }
      continue;
    }
    if (octet === GS) {
      const commande = flux[i + 1];
      if (commande === 0x21) {
        doubleTaille = (flux[i + 2] ?? 0) !== 0;
        i += 2;
      } else if (commande === 0x56) {
        i += 3; // coupe
      }
      continue;
    }
    if (octet === 0x0a) {
      pousser();
      continue;
    }
    courante += inverse[octet] ?? String.fromCharCode(octet);
  }
  if (courante) pousser();
  return sortie.join('\n');
}

/**
 * rendreDoubleTaille imite l'élargissement des caractères par l'imprimante :
 * chaque caractère occupe deux colonnes.
 */
function rendreDoubleTaille(ligne: string): string {
  return ligne
    .split('')
    .map((c) => `${c}${c}`)
    .join('');
}

// --- Transport WebUSB -------------------------------------------------------

interface EscPosUsbEndpoint {
  endpointNumber: number;
  direction: 'in' | 'out';
  type: 'bulk' | 'interrupt' | 'isochronous';
}

interface EscPosUsbDevice {
  opened: boolean;
  configuration: {
    interfaces: { interfaceNumber: number; alternate: { endpoints: EscPosUsbEndpoint[] }[] }[];
  } | null;
  open(): Promise<void>;
  selectConfiguration(valeur: number): Promise<void>;
  claimInterface(numero: number): Promise<void>;
  releaseInterface(numero: number): Promise<void>;
  close(): Promise<void>;
  transferOut(numero: number, donnees: Uint8Array): Promise<{ status: string }>;
}

interface EscPosUsbManager {
  requestDevice(options: { filters: unknown[] }): Promise<EscPosUsbDevice>;
  getDevices(): Promise<EscPosUsbDevice[]>;
}

function gestionnaireUsb(): EscPosUsbManager | null {
  if (typeof navigator === 'undefined') return null;
  const usb = (navigator as unknown as { usb?: EscPosUsbManager }).usb;
  return usb ?? null;
}

/** Indique si l'impression directe est possible dans ce navigateur. */
export function impressionDirecteSupportee(): boolean {
  return gestionnaireUsb() !== null;
}

let peripherique: EscPosUsbDevice | null = null;
let pointDeSortie: number | null = null;

/** État courant de la connexion à l'imprimante. */
export function imprimanteConnectee(): boolean {
  return peripherique !== null && pointDeSortie !== null;
}

/**
 * connectEscPosPrinter ouvre une imprimante USB (sélecteur du navigateur) et
 * prépare son canal de sortie. À appeler depuis un geste utilisateur.
 */
export async function connectEscPosPrinter(): Promise<{ ok: boolean; error?: string }> {
  const usb = gestionnaireUsb();
  if (!usb) {
    return {
      ok: false,
      error:
        'Impression directe non prise en charge par ce navigateur (Chrome ou Edge sur ordinateur requis).',
    };
  }
  try {
    // Filtre large : les imprimantes thermiques génériques n'exposent pas
    // toujours de descripteur exploitable, l'utilisateur choisit le sien.
    const appareil = await usb.requestDevice({ filters: [{ classCode: 7 }] });
    if (!appareil.opened) await appareil.open();
    if (!appareil.configuration) await appareil.selectConfiguration(1);

    const interfaceImprimante = appareil.configuration?.interfaces[0];
    if (!interfaceImprimante) {
      await appareil.close();
      return { ok: false, error: 'Imprimante sans interface USB exploitable.' };
    }
    await appareil.claimInterface(interfaceImprimante.interfaceNumber);
    const sortie = interfaceImprimante.alternate[0]?.endpoints.find((e) => e.direction === 'out');
    if (!sortie) {
      await appareil.releaseInterface(interfaceImprimante.interfaceNumber);
      await appareil.close();
      return { ok: false, error: 'Aucun canal de sortie sur cette imprimante.' };
    }

    peripherique = appareil;
    pointDeSortie = sortie.endpointNumber;
    return { ok: true };
  } catch (erreur) {
    peripherique = null;
    pointDeSortie = null;
    return {
      ok: false,
      error: erreur instanceof Error ? erreur.message : 'connexion à l’imprimante annulée',
    };
  }
}

/**
 * sendToEscPosPrinter envoie le flux à l'imprimante connectée. Si aucun
 * périphérique n'est encore ouvert, la connexion est demandée automatiquement.
 */
export async function sendToEscPosPrinter(flux: Uint8Array): Promise<{ ok: boolean; error?: string }> {
  if (!imprimanteConnectee()) {
    const connexion = await connectEscPosPrinter();
    if (!connexion.ok) return connexion;
  }
  try {
    const appareil = peripherique as EscPosUsbDevice;
    const numero = pointDeSortie as number;
    await appareil.transferOut(numero, flux);
    return { ok: true };
  } catch (erreur) {
    return {
      ok: false,
      error: erreur instanceof Error ? erreur.message : 'écriture vers l’imprimante impossible',
    };
  }
}

/** Ferme la connexion (déconnexion explicite ou changement d'imprimante). */
export async function disconnectEscPosPrinter(): Promise<void> {
  if (!peripherique) return;
  try {
    if (peripherique.configuration?.interfaces[0]) {
      await peripherique.releaseInterface(peripherique.configuration.interfaces[0].interfaceNumber);
    }
    await peripherique.close();
  } catch {
    // Une imprimante débranchée ne doit jamais faire échouer l'interface.
  } finally {
    peripherique = null;
    pointDeSortie = null;
  }
}
