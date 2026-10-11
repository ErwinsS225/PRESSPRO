import { describe, expect, it, vi, afterEach } from 'vitest';
import {
  ESC,
  GS,
  LARGEUR_TICKET,
  aligneGaucheDroite,
  buildTicketBytes,
  commandes,
  connectEscPosPrinter,
  disconnectEscPosPrinter,
  encodeEscPos,
  impressionDirecteSupportee,
  imprimanteConnectee,
  renderEscPosPreview,
  ticketDepuisCommande,
  type TicketEscPos,
} from './escpos';
import type { Order, Tenant } from '../types';

const ticket: TicketEscPos = {
  tenantName: 'Pressing Ivoire',
  tenantCity: 'Abidjan',
  tenantPhone: '+225 07 00 00 00 00',
  orderNumber: 'CMD-2026-0042',
  receivedAt: '11/10/2026 09:30',
  dueDate: '13/10/2026 12:00',
  customerName: 'Awa Koné',
  customerPhone: '+225 05 00 00 00 00',
  items: [
    { serviceName: 'Chemise propre', quantity: 2, subtotal: 1500 },
    { serviceName: 'Costume 3 pièces (nettoyage à sec)', quantity: 1, subtotal: 7000 },
  ],
  totalAmount: 8500,
  paidAmount: 5000,
  verifyUrl: 'https://pressing.test/verify/tenant/order',
};

function octets(flux: Uint8Array): number[] {
  return Array.from(flux);
}

describe('flux ESC/POS', () => {
  it('encadre le ticket par l’initialisation, le jeu de caractères et la coupe', () => {
    const flux = buildTicketBytes(ticket);
    const octetsFlux = octets(flux);

    expect(octetsFlux.slice(0, 2)).toEqual([ESC, 0x40]);
    expect(octetsFlux).toContain(0x74); // ESC t (page de codes)
    // Fin de flux : avance papier puis coupe partielle.
    expect(octetsFlux.slice(-7)).toEqual([...commandes.avancer(3), ...commandes.couper()]);
    expect(octetsFlux.at(-4)).toBe(GS);
    expect(octetsFlux.at(-3)).toBe(0x56);
  });

  it('peut produire un ticket sans coupe (imprimante à déchirure)', () => {
    const flux = octets(buildTicketBytes(ticket, { couper: false }));
    // Aucune séquence GS V (coupe) ne doit subsister.
    const coupe = flux.some((o, i) => o === GS && flux[i + 1] === 0x56);
    expect(coupe).toBe(false);
    expect(flux.slice(-3)).toEqual(commandes.avancer(3));
  });

  it('met le nom du pressing en double taille au centre', () => {
    const flux = octets(buildTicketBytes(ticket));
    const indexCentre = flux.findIndex((o, i) => o === ESC && flux[i + 1] === 0x61 && flux[i + 2] === 1);
    const indexDouble = flux.findIndex((o, i) => o === GS && flux[i + 1] === 0x21 && flux[i + 2] === 0x11);

    expect(indexCentre).toBeGreaterThan(-1);
    expect(indexDouble).toBeGreaterThan(indexCentre);
    // Le nom suit immédiatement la mise en double taille.
    expect(flux.slice(indexDouble + 3, indexDouble + 3 + 8)).toEqual(encodeEscPos('PRESSING'));
  });

  it('aligne les montants sur la largeur utile du papier', () => {
    const ligne = aligneGaucheDroite('TOTAL', '8 500 FCFA');
    expect(ligne).toHaveLength(LARGEUR_TICKET);
    expect(ligne.startsWith('TOTAL')).toBe(true);
    expect(ligne.endsWith('8 500 FCFA')).toBe(true);
  });

  it('tronque les libellés trop longs sans casser l’alignement', () => {
    const flux = renderEscPosPreview(
      buildTicketBytes({
        ...ticket,
        items: [
          {
            serviceName: 'Costume trois pièces avec doublure et boutons nacrés',
            quantity: 1,
            subtotal: 7000,
          },
        ],
      })
    );
    const ligneArticle = flux.split('\n').find((l) => l.includes('FCFA') && l.includes('1 x'));
    expect(ligneArticle).toBeDefined();
    expect(ligneArticle!.length).toBeLessThanOrEqual(LARGEUR_TICKET);
    expect(ligneArticle!.endsWith('7 000 FCFA')).toBe(true);
  });
});

describe('encodage CP437', () => {
  it('traduit les accents français en octets imprimables', () => {
    expect(encodeEscPos('é')).toEqual([0x82]);
    expect(encodeEscPos('É')).toEqual([0x90]);
    expect(encodeEscPos('à')).toEqual([0x85]);
    expect(encodeEscPos('î')).toEqual([0x8c]);
    expect(encodeEscPos('ç')).toEqual([0x87]);
  });

  it('ramène la ponctuation typographique à l’ASCII imprimable', () => {
    expect(encodeEscPos('O’Brien')).toContain(0x27); // apostrophe courbe
    expect(encodeEscPos('« Chemise »')).toEqual(encodeEscPos('" Chemise "'));
    expect(encodeEscPos('5 – 10')).toEqual(encodeEscPos('5 - 10'));
  });

  it('perd l’accent plutôt que d’imprimer un caractère parasite', () => {
    // « À » et « Ê » n'existent pas en CP437 : ils s'impriment sans accent.
    expect(encodeEscPos('RESTE À PAYER')).toEqual(encodeEscPos('RESTE A PAYER'));
    expect(encodeEscPos('PRÊT')).toEqual(encodeEscPos('PRET'));
    expect(encodeEscPos('5 €')).toContain(0x45); // E de « EUR »
  });

  it('signale par « ? » un caractère réellement inimprimable', () => {
    expect(encodeEscPos('priorité ≤ élevée')).toContain(0x3f);
  });
});

describe('aperçu du flux', () => {
  const apercu = renderEscPosPreview(buildTicketBytes(ticket));

  it('restitue le ticket tel qu’il sortira de l’imprimante', () => {
    const lignes = apercu.split('\n');
    // Double taille : chaque caractère du nom est doublé.
    expect(apercu).toContain('PPRREESSSSIINNGG');
    expect(lignes.some((l) => l.includes('N° CMD-2026-0042'))).toBe(true);
    expect(lignes.some((l) => l.includes('Awa Koné'))).toBe(true);
    expect(lignes.some((l) => l.includes('TOTAL') && l.includes('8 500 FCFA'))).toBe(true);
    expect(lignes.some((l) => l.includes('Payé') && l.includes('5 000 FCFA'))).toBe(true);
    // « À » n'existe pas en CP437 : l'aperçu montre ce que l'imprimante sortira
    // réellement, c'est-à-dire la lettre sans accent.
    expect(lignes.some((l) => l.includes('RESTE A PAYER') && l.includes('3 500 FCFA'))).toBe(true);
    expect(lignes.some((l) => l.includes('https://pressing.test/verify/tenant/order'))).toBe(true);
    expect(apercu).toContain('Géré par PressingOS');
  });

  it('remplace la mention de reste par « SOLDÉ » quand tout est réglé', () => {
    const solde = renderEscPosPreview(buildTicketBytes({ ...ticket, paidAmount: ticket.totalAmount }));
    expect(solde).toContain('SOLDÉ');
    expect(solde).not.toContain('RESTE A PAYER');
  });

  it('ne laisse passer aucun octet de contrôle dans le texte imprimé', () => {
    // On teste justement l'absence d'octets de contrôle dans l'aperçu.
    // eslint-disable-next-line no-control-regex
    expect(apercu).not.toMatch(/[\u0000-\u0008\u000b-\u001f]/);
  });

  it('sépare en deux colonnes les articles et leur prix', () => {
    const ligne = apercu.split('\n').find((l) => l.includes('2 x Chemise propre'))!;
    expect(ligne).toContain('1 500 FCFA');
    expect(ligne.trimEnd()).toHaveLength(LARGEUR_TICKET);
  });
});

describe('conversion depuis une commande', () => {
  it('reprend les informations utiles du ticket HTML', () => {
    const commande = {
      id: 'o1',
      orderNumber: 'CMD-77',
      customerName: 'Awa',
      customerPhone: '+225 01',
      items: [{ id: 'i1', serviceName: 'Chemise', quantity: 1, unitPrice: 500, subtotal: 500 }],
      status: 'ready',
      paymentStatus: 'partially_paid',
      totalAmount: 500,
      paidAmount: 0,
      receivedAt: '2026-10-11',
      dueDate: '2026-10-12',
    } as Order;
    const pressing = {
      id: 't1',
      name: 'Pressing Ivoire',
      slug: 'x',
      phone: '+225 07',
      city: 'Abidjan',
      currency: 'XOF',
      status: 'active',
    } as Tenant;

    const donnees = ticketDepuisCommande(commande, pressing, 'https://x/verify');
    expect(donnees.orderNumber).toBe('CMD-77');
    expect(donnees.items).toEqual([{ serviceName: 'Chemise', quantity: 1, subtotal: 500 }]);
    expect(donnees.verifyUrl).toBe('https://x/verify');
  });
});

describe('transport WebUSB', () => {
  afterEach(async () => {
    await disconnectEscPosPrinter();
    Object.defineProperty(navigator, 'usb', { value: undefined, configurable: true });
  });

  it('signale clairement l’absence de WebUSB', async () => {
    Object.defineProperty(navigator, 'usb', { value: undefined, configurable: true });
    expect(impressionDirecteSupportee()).toBe(false);
    const resultat = await connectEscPosPrinter();
    expect(resultat.ok).toBe(false);
    expect(resultat.error).toMatch(/Chrome ou Edge/);
  });

  it('ouvre l’imprimante puis transmet le flux tel quel', async () => {
    const transferOut = vi.fn().mockResolvedValue({ status: 'ok' });
    interface FauxAppareil {
      opened: boolean;
      configuration: {
        interfaces: {
          interfaceNumber: number;
          alternate: {
            endpoints: {
              endpointNumber: number;
              direction: 'in' | 'out';
              type: 'bulk' | 'interrupt' | 'isochronous';
            }[];
          }[];
        }[];
      } | null;
      open: () => Promise<void>;
      selectConfiguration: (valeur: number) => Promise<void>;
      claimInterface: (numero: number) => Promise<void>;
      releaseInterface: (numero: number) => Promise<void>;
      close: () => Promise<void>;
      transferOut: (numero: number, donnees: Uint8Array) => Promise<{ status: string }>;
    }
    const appareil: FauxAppareil = {
      opened: false,
      configuration: null,
      open: vi.fn(async () => {
        appareil.opened = true;
      }),
      selectConfiguration: vi.fn(async () => {
        appareil.configuration = {
          interfaces: [
            {
              interfaceNumber: 0,
              alternate: [
                { endpoints: [{ endpointNumber: 1, direction: 'out' as const, type: 'bulk' as const }] },
              ],
            },
          ],
        };
      }),
      claimInterface: vi.fn().mockResolvedValue(undefined),
      releaseInterface: vi.fn().mockResolvedValue(undefined),
      close: vi.fn().mockResolvedValue(undefined),
      transferOut,
    };
    Object.defineProperty(navigator, 'usb', {
      value: { requestDevice: vi.fn().mockResolvedValue(appareil) },
      configurable: true,
    });

    expect(impressionDirecteSupportee()).toBe(true);

    const { sendToEscPosPrinter } = await import('./escpos');
    const flux = buildTicketBytes(ticket);
    const resultat = await sendToEscPosPrinter(flux);

    expect(resultat.ok).toBe(true);
    expect(imprimanteConnectee()).toBe(true);
    expect(transferOut).toHaveBeenCalledTimes(1);
    expect(octets(transferOut.mock.calls[0][1] as Uint8Array)).toEqual(octets(flux));
    expect(transferOut.mock.calls[0][0]).toBe(1);
  });
});
