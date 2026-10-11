import { describe, expect, it } from 'vitest';
import {
  cleJour,
  cleMois,
  dateDepot,
  montantCourt,
  pipelineAtelier,
  recouvrement,
  serieJournaliere,
  serieMensuelle,
} from './analytique';
import type { Order } from '../types';

function commande(partiel: Partial<Order> & Pick<Order, 'orderNumber' | 'receivedAt'>): Order {
  return {
    id: partiel.orderNumber,
    customerName: 'Client Test',
    customerPhone: '+225 07 00 00 00 00',
    items: [{ id: 'it-1', serviceName: 'Chemise', quantity: 2, unitPrice: 1000, subtotal: 2000 }],
    status: 'received',
    paymentStatus: 'unpaid',
    totalAmount: 2000,
    paidAmount: 0,
    dueDate: '12/10/2026 17:00',
    ...partiel,
  };
}

describe('dateDepot — formats produits par l’application', () => {
  it('lit la date de l’API (« jj/mm/aaaa hh:mm »)', () => {
    const date = dateDepot('11/10/2026 09:30');
    expect(date).not.toBeNull();
    expect(cleJour(date!)).toBe('2026-10-11');
    expect(date!.getHours()).toBe(9);
  });

  it('lit une date ISO et ignore ce qu’il ne comprend pas', () => {
    expect(cleJour(dateDepot('2026-09-02T08:00:00Z')!)).toBe('2026-09-02');
    expect(dateDepot('')).toBeNull();
    expect(dateDepot('date inconnue')).toBeNull();
  });

  it('lit les libellés de la vitrine de démonstration', () => {
    const reference = new Date(2026, 9, 11, 12, 0); // 11 octobre 2026
    expect(cleJour(dateDepot('Aujourd’hui à 08:30', reference)!)).toBe('2026-10-11');
    expect(cleJour(dateDepot('Hier à 14:00', reference)!)).toBe('2026-10-10');
    expect(cleJour(dateDepot('09 Octobre à 10:00', reference)!)).toBe('2026-10-09');
  });
});

describe('serieMensuelle', () => {
  const commandes = [
    commande({
      orderNumber: 'CMD-1',
      receivedAt: '02/09/2026 08:00',
      totalAmount: 6000,
      paidAmount: 6000,
      paymentStatus: 'paid',
    }),
    commande({
      orderNumber: 'CMD-2',
      receivedAt: '11/10/2026 09:30',
      totalAmount: 8500,
      paidAmount: 5000,
      paymentStatus: 'partially_paid',
    }),
    commande({ orderNumber: 'CMD-3', receivedAt: '11/10/2026 15:00', totalAmount: 3000, paidAmount: 3000 }),
  ];

  it('produit des mois consécutifs jusqu’au dépôt le plus récent', () => {
    const serie = serieMensuelle(commandes, 6);
    expect(serie).toHaveLength(6);
    expect(serie.map((point) => point.mois)).toEqual([
      '2026-05',
      '2026-06',
      '2026-07',
      '2026-08',
      '2026-09',
      '2026-10',
    ]);
    expect(serie.at(-1)!.label).toBe('oct. 26');
  });

  it('cumule encaissements, facturation et nombre de dépôts par mois', () => {
    const serie = serieMensuelle(commandes, 6);
    const septembre = serie.find((point) => point.mois === '2026-09')!;
    const octobre = serie.at(-1)!;
    expect(septembre).toMatchObject({ encaisse: 6000, facture: 6000, depots: 1 });
    expect(octobre).toMatchObject({ encaisse: 8000, facture: 11500, depots: 2 });
  });

  it('renvoie une série vide quand aucune date n’est exploitable', () => {
    expect(serieMensuelle([])).toEqual([]);
    expect(serieMensuelle([commande({ orderNumber: 'CMD-X', receivedAt: 'Bientôt' })])).toEqual([]);
  });
});

describe('serieJournaliere', () => {
  const commandes = [
    commande({ orderNumber: 'CMD-1', receivedAt: '02/09/2026 08:00', paidAmount: 2000 }),
    commande({ orderNumber: 'CMD-2', receivedAt: '11/10/2026 09:30', paidAmount: 5000 }),
    commande({ orderNumber: 'CMD-3', receivedAt: '11/10/2026 15:45', paidAmount: 1000 }),
    commande({ orderNumber: 'CMD-4', receivedAt: '12/10/2026 10:00', paidAmount: 0 }),
  ];

  it('regroupe par jour d’activité et trie du plus ancien au plus récent', () => {
    const serie = serieJournaliere(commandes);
    expect(serie.map((point) => point.label)).toEqual(['02/09', '11/10', '12/10']);
    expect(serie[1]).toMatchObject({ depots: 2, pieces: 4, encaisse: 6000 });
    expect(serie[2].jour).toBe('2026-10-12');
  });

  it('ne conserve que les derniers jours demandés', () => {
    const serie = serieJournaliere(commandes, 2);
    expect(serie.map((point) => point.label)).toEqual(['11/10', '12/10']);
  });
});

describe('recouvrement', () => {
  it('sépare les dépôts soldés, les acomptes et le reste à percevoir', () => {
    const resultat = recouvrement([
      commande({
        orderNumber: 'CMD-1',
        receivedAt: '02/09/2026 08:00',
        totalAmount: 6000,
        paidAmount: 6000,
        paymentStatus: 'paid',
      }),
      commande({
        orderNumber: 'CMD-2',
        receivedAt: '11/10/2026 09:30',
        totalAmount: 8500,
        paidAmount: 5000,
        paymentStatus: 'partially_paid',
      }),
    ]);

    expect(resultat).toMatchObject({ encaisse: 11000, facture: 14500, reste: 3500, taux: 75.9 });
    expect(resultat.tranches).toEqual([
      { etat: 'solde', label: 'Dépôts soldés', value: 6000 },
      { etat: 'acompte', label: 'Acomptes versés', value: 5000 },
      { etat: 'reste', label: 'Reste à percevoir', value: 3500 },
    ]);
  });

  it('reste à zéro sans commande', () => {
    const resultat = recouvrement([]);
    expect(resultat.taux).toBe(0);
    expect(resultat.reste).toBe(0);
    expect(resultat.tranches.every((tranche) => tranche.value === 0)).toBe(true);
  });
});

describe('pipelineAtelier', () => {
  it('cumule les dépôts ayant atteint chaque étape', () => {
    const etapes = pipelineAtelier([
      commande({ orderNumber: 'CMD-1', receivedAt: '01/10/2026 08:00', status: 'delivered' }),
      commande({ orderNumber: 'CMD-2', receivedAt: '01/10/2026 09:00', status: 'delivered' }),
      commande({ orderNumber: 'CMD-3', receivedAt: '02/10/2026 09:00', status: 'ready' }),
      commande({ orderNumber: 'CMD-4', receivedAt: '03/10/2026 09:00', status: 'washing' }),
      commande({ orderNumber: 'CMD-5', receivedAt: '04/10/2026 09:00', status: 'received' }),
    ]);

    expect(etapes.map((etape) => etape.value)).toEqual([5, 4, 3, 3, 2]);
    expect(etapes.at(-1)!.label).toBe('Livrés au client');
  });
});

describe('montantCourt', () => {
  it('abrège les montants pour les axes', () => {
    expect(montantCourt(850)).toBe('850');
    expect(montantCourt(12_000)).toBe('12 k');
    expect(montantCourt(1_500_000)).toBe('1,5 M');
  });
});

describe('clés de période', () => {
  it('respecte les formats « aaaa-mm » et « aaaa-mm-jj »', () => {
    expect(cleMois(new Date(2026, 0, 5))).toBe('2026-01');
    expect(cleJour(new Date(2026, 9, 9))).toBe('2026-10-09');
  });
});
