import {
  ArrowRight,
  ArrowUpDown,
  Check,
  ChevronDown,
  ChevronUp,
  MessageCircle,
  Printer,
  Search,
} from 'lucide-react';
import * as React from 'react';

import { dateDepot } from '@/lib/analytique';
import { Badge, type BadgeProps } from '@/components/reui/badge';
import { Button } from '@/components/ui/button';
import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableEmpty,
  TableFooter,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import type { Order, OrderStatus, PaymentStatus } from '../../types';

/** Étapes d'atelier : numéro, libellés et couleur de badge. */
export const ETAPES_COMMANDE: Record<
  OrderStatus,
  { numero: number; label: string; court: string; badge: BadgeProps['variant'] }
> = {
  received: { numero: 1, label: '1. Réceptionné', court: 'Réception', badge: 'secondary' },
  washing: { numero: 2, label: '2. En lavage', court: 'Lavage', badge: 'info-light' },
  ironing: { numero: 3, label: '3. Repassage', court: 'Repassage', badge: 'warning-light' },
  ready: { numero: 4, label: '4. Prêt au comptoir', court: 'Prêt', badge: 'success-light' },
  delivered: { numero: 5, label: '5. Livré', court: 'Livré', badge: 'outline' },
};

/** Action d'atelier qui fait avancer un dépôt (null lorsqu'il est clôturé). */
export const PROCHAINE_ETAPE: Record<OrderStatus, string | null> = {
  received: 'Passer au lavage',
  washing: 'Passer au repassage',
  ironing: 'Marquer prêt',
  ready: 'Confirmer la livraison',
  delivered: null,
};

/** État du règlement d'un dépôt. */
export const ETAT_REGLEMENT: Record<PaymentStatus, { label: string; badge: BadgeProps['variant'] }> = {
  paid: { label: 'Soldé', badge: 'success-outline' },
  partially_paid: { label: 'Acompte versé', badge: 'warning-outline' },
  unpaid: { label: 'À encaisser', badge: 'destructive-outline' },
};

export type ColonneCommande =
  'ticket' | 'client' | 'articles' | 'echeance' | 'statut' | 'reglement' | 'montant' | 'actions';

/** Colonnes triables : celles dont la valeur se compare (nombres, dates, textes). */
type ColonneTriable = Extract<ColonneCommande, 'ticket' | 'client' | 'articles' | 'echeance' | 'montant'>;

const LIBELLES: Record<ColonneCommande, string> = {
  ticket: 'N° ticket',
  client: 'Client & contact',
  articles: 'Contenu du dépôt',
  echeance: 'Date prévue',
  statut: 'Statut atelier',
  reglement: 'Règlement',
  montant: 'Montant',
  actions: 'Actions',
};

const A_DROITE: ColonneCommande[] = ['montant', 'actions'];

const TOUTES_COLONNES: ColonneCommande[] = [
  'ticket',
  'client',
  'articles',
  'echeance',
  'statut',
  'reglement',
  'montant',
  'actions',
];

export interface OrdersTableProps {
  orders: Order[];
  /** Colonnes affichées, dans l'ordre du tableau (toutes par défaut). */
  columns?: ColonneCommande[];
  /** Barre d'outils : recherche plein texte et filtre par statut, avec compteurs. */
  toolbar?: boolean;
  /** Lignes par palier d'affichage ; 0 affiche tout d'un coup. */
  pageSize?: number;
  onOpenTicket?: (order: Order) => void;
  onNotify?: (order: Order) => void;
  onAdvance?: (orderId: string) => void;
  emptyMessage?: string;
  caption?: React.ReactNode;
}

/** Retire accents et casse pour comparer les recherches libres. */
function normalise(valeur: string): string {
  return valeur
    .toLowerCase()
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '');
}

const fcfa = (valeur: number) => `${valeur.toLocaleString('fr-FR')} F`;

/**
 * Tableau des dépôts (modèle c-table-10 enrichi) : tri par colonne, filtre par
 * étape d'atelier, recherche libre (ticket, client, téléphone, article), actions
 * de comptoir et pied récapitulatif. Le même composant sert au comptoir (colonnes
 * d'action) et au tableau de bord (version condensée, sans barre d'outils).
 */
export function OrdersTable({
  orders,
  columns = TOUTES_COLONNES,
  toolbar = false,
  pageSize = 8,
  onOpenTicket,
  onNotify,
  onAdvance,
  emptyMessage = 'Aucun dépôt ne correspond aux critères.',
  caption,
}: OrdersTableProps) {
  const [tri, setTri] = React.useState<{ colonne: ColonneTriable; sens: 'asc' | 'desc' } | null>(null);
  const [filtre, setFiltre] = React.useState<'all' | OrderStatus>('all');
  const [recherche, setRecherche] = React.useState('');
  const [paliers, setPaliers] = React.useState(1);

  const compteurs = React.useMemo(() => {
    const parStatut = new Map<OrderStatus, number>();
    for (const commande of orders) {
      parStatut.set(commande.status, (parStatut.get(commande.status) ?? 0) + 1);
    }
    return parStatut;
  }, [orders]);

  const lignes = React.useMemo(() => {
    const requete = normalise(recherche.trim());
    const filtrees = orders.filter((commande) => {
      if (filtre !== 'all' && commande.status !== filtre) return false;
      if (!requete) return true;
      const champs = [
        commande.orderNumber,
        commande.customerName,
        commande.customerPhone,
        ...commande.items.map((item) => item.serviceName),
      ];
      return champs.some((champ) => normalise(champ ?? '').includes(requete));
    });

    if (!tri) return filtrees;
    const facteur = tri.sens === 'asc' ? 1 : -1;
    const valeur = (commande: Order): string | number => {
      switch (tri.colonne) {
        case 'ticket':
          return commande.orderNumber;
        case 'client':
          return commande.customerName;
        case 'articles':
          return commande.items.reduce((total, item) => total + item.quantity, 0);
        case 'echeance':
          return dateDepot(commande.dueDate)?.getTime() ?? 0;
        case 'montant':
          return commande.totalAmount;
      }
    };

    return [...filtrees].sort((a, b) => {
      const va = valeur(a);
      const vb = valeur(b);
      if (typeof va === 'string' || typeof vb === 'string') {
        return facteur * String(va).localeCompare(String(vb), 'fr', { numeric: true });
      }
      return facteur * (va - vb);
    });
  }, [orders, filtre, recherche, tri]);

  const parPalier = pageSize > 0 ? pageSize : lignes.length || 1;
  const visibles = lignes.slice(0, parPalier * paliers);
  const reste = lignes.length - visibles.length;

  const totaux = React.useMemo(
    () =>
      lignes.reduce(
        (cumul, commande) => ({
          facture: cumul.facture + commande.totalAmount,
          encaisse: cumul.encaisse + commande.paidAmount,
          pieces: cumul.pieces + commande.items.reduce((total, item) => total + item.quantity, 0),
          enAtelier: cumul.enAtelier + (commande.status === 'delivered' ? 0 : 1),
          soldes: cumul.soldes + (commande.paymentStatus === 'paid' ? 1 : 0),
        }),
        { facture: 0, encaisse: 0, pieces: 0, enAtelier: 0, soldes: 0 }
      ),
    [lignes]
  );

  const basculerTri = (colonne: ColonneTriable) => {
    setTri((actuel) =>
      actuel?.colonne === colonne
        ? { colonne, sens: actuel.sens === 'asc' ? 'desc' : 'asc' }
        : { colonne, sens: 'asc' }
    );
    setPaliers(1);
  };

  const enTete = (colonne: ColonneCommande) => {
    const triable: ColonneTriable[] = ['ticket', 'client', 'articles', 'echeance', 'montant'];
    const alignement = A_DROITE.includes(colonne) ? 'text-right' : '';
    if (!triable.includes(colonne as ColonneTriable)) {
      return (
        <TableHead key={colonne} className={alignement}>
          {LIBELLES[colonne]}
        </TableHead>
      );
    }

    const cle = colonne as ColonneTriable;
    const actif = tri?.colonne === cle;
    const icone = !actif ? (
      <ArrowUpDown className="size-3 opacity-50" aria-hidden="true" />
    ) : tri.sens === 'asc' ? (
      <ChevronUp className="size-3" aria-hidden="true" />
    ) : (
      <ChevronDown className="size-3" aria-hidden="true" />
    );

    return (
      <TableHead
        key={colonne}
        className={alignement}
        aria-sort={actif ? (tri.sens === 'asc' ? 'ascending' : 'descending') : 'none'}
      >
        <button
          type="button"
          onClick={() => basculerTri(cle)}
          className="inline-flex items-center gap-1 rounded-sm font-medium transition-colors hover:text-neutral-950 dark:hover:text-neutral-100"
          aria-label={`Trier par ${LIBELLES[colonne].toLowerCase()}`}
        >
          {LIBELLES[colonne]}
          {icone}
        </button>
      </TableHead>
    );
  };

  const contenuPied: Record<ColonneCommande, React.ReactNode> = {
    ticket: `${lignes.length} dépôt${lignes.length > 1 ? 's' : ''}`,
    client: '',
    articles: `${totaux.pieces} pièce${totaux.pieces > 1 ? 's' : ''}`,
    echeance: '',
    statut: `${totaux.enAtelier} en atelier`,
    reglement: `${totaux.soldes} soldé${totaux.soldes > 1 ? 's' : ''}`,
    montant: fcfa(totaux.facture),
    actions: `${fcfa(totaux.encaisse)} encaissés`,
  };

  return (
    <div className="flex flex-col">
      {toolbar && (
        <div className="flex flex-col gap-3 px-5 py-4 sm:flex-row sm:items-center sm:justify-between">
          <div
            role="group"
            aria-label="Filtrer par étape d’atelier"
            className="flex flex-wrap items-center gap-1 rounded-lg border border-neutral-200/60 bg-neutral-100 p-1 dark:border-neutral-800 dark:bg-neutral-900"
          >
            {(
              [
                { id: 'all' as const, label: 'Tous', compte: orders.length },
                ...(['received', 'washing', 'ironing', 'ready', 'delivered'] as OrderStatus[]).map(
                  (statut) => ({
                    id: statut,
                    label: ETAPES_COMMANDE[statut].court,
                    compte: compteurs.get(statut) ?? 0,
                  })
                ),
              ] as { id: 'all' | OrderStatus; label: string; compte: number }[]
            ).map((onglet) => (
              <button
                key={onglet.id}
                type="button"
                aria-pressed={filtre === onglet.id}
                onClick={() => {
                  setFiltre(onglet.id);
                  setPaliers(1);
                }}
                className={`shrink-0 rounded-md px-3 py-1 text-xs font-medium whitespace-nowrap transition-colors ${
                  filtre === onglet.id
                    ? 'bg-white font-semibold text-neutral-900 shadow-2xs dark:bg-neutral-800 dark:text-neutral-100'
                    : 'text-neutral-600 hover:text-neutral-950 dark:text-neutral-400 dark:hover:text-neutral-200'
                }`}
              >
                {onglet.label}
                <span className="ml-1 font-mono text-[10px] text-neutral-400 dark:text-neutral-500">
                  {onglet.compte}
                </span>
              </button>
            ))}
          </div>

          <div className="relative w-full sm:w-64">
            <Search
              className="absolute top-2.5 left-2.5 h-3.5 w-3.5 text-neutral-400 dark:text-neutral-500"
              aria-hidden="true"
            />
            <input
              type="search"
              aria-label="Rechercher un dépôt"
              placeholder="Rechercher ticket, client, article…"
              value={recherche}
              onChange={(evenement) => {
                setRecherche(evenement.target.value);
                setPaliers(1);
              }}
              className="w-full rounded-md border border-neutral-200 bg-white py-1.5 pr-3 pl-8 text-xs text-neutral-900 placeholder:text-neutral-400 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:placeholder:text-neutral-500 dark:focus:border-neutral-100"
            />
          </div>
        </div>
      )}

      <Table>
        <TableHeader>
          <TableRow className="hover:bg-transparent dark:hover:bg-transparent">
            {columns.map((colonne) => enTete(colonne))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {visibles.length === 0 ? (
            <TableEmpty colSpan={columns.length}>{emptyMessage}</TableEmpty>
          ) : (
            visibles.map((commande) => {
              const etape = ETAPES_COMMANDE[commande.status];
              const reglement = ETAT_REGLEMENT[commande.paymentStatus];
              const soldeDu = Math.max(0, commande.totalAmount - commande.paidAmount);
              const prochaine = PROCHAINE_ETAPE[commande.status];

              return (
                <TableRow key={commande.id}>
                  {columns.map((colonne) => {
                    const alignement = A_DROITE.includes(colonne) ? 'text-right' : '';
                    switch (colonne) {
                      case 'ticket':
                        return (
                          <TableCell key={colonne} className="px-5 py-3.5 font-mono font-semibold">
                            {onOpenTicket ? (
                              <button
                                type="button"
                                onClick={() => onOpenTicket(commande)}
                                title="Voir / imprimer le ticket thermique"
                                className="flex items-center gap-1.5 font-mono font-semibold text-neutral-900 transition-colors hover:text-neutral-950 dark:text-neutral-100 dark:hover:text-neutral-300"
                              >
                                <Printer className="h-3 w-3 text-neutral-400" aria-hidden="true" />
                                <span>{commande.orderNumber}</span>
                              </button>
                            ) : (
                              commande.orderNumber
                            )}
                          </TableCell>
                        );
                      case 'client':
                        return (
                          <TableCell key={colonne} className="py-3.5">
                            <div className="font-medium text-neutral-900 dark:text-neutral-100">
                              {commande.customerName}
                            </div>
                            <div className="font-mono text-[11px] text-neutral-500 dark:text-neutral-400">
                              {commande.customerPhone}
                            </div>
                          </TableCell>
                        );
                      case 'articles':
                        return (
                          <TableCell key={colonne} className="py-3.5 text-neutral-600 dark:text-neutral-400">
                            <div>
                              {commande.items.length > 0
                                ? commande.items
                                    .map((item) => `${item.quantity}x ${item.serviceName}`)
                                    .join(', ')
                                : 'Aucun article'}
                            </div>
                            {commande.notes ? (
                              <div className="mt-0.5 text-[11px] text-neutral-400 italic dark:text-neutral-500">
                                Note : {commande.notes}
                              </div>
                            ) : null}
                          </TableCell>
                        );
                      case 'echeance':
                        return (
                          <TableCell
                            key={colonne}
                            className="py-3.5 font-mono text-[11px] text-neutral-600 dark:text-neutral-400"
                          >
                            {commande.dueDate || '—'}
                          </TableCell>
                        );
                      case 'statut':
                        return (
                          <TableCell key={colonne} className="py-3.5">
                            <Badge variant={etape.badge} size="sm">
                              {etape.label}
                            </Badge>
                          </TableCell>
                        );
                      case 'reglement':
                        return (
                          <TableCell key={colonne} className="py-3.5">
                            <Badge variant={reglement.badge} size="sm">
                              {reglement.label}
                            </Badge>
                            {soldeDu > 0 ? (
                              <div className="mt-1 font-mono text-[11px] text-amber-700 dark:text-amber-400">
                                Reste {fcfa(soldeDu)}
                              </div>
                            ) : null}
                          </TableCell>
                        );
                      case 'montant':
                        return (
                          <TableCell
                            key={colonne}
                            className="py-3.5 text-right font-mono font-medium text-neutral-900 tabular-nums dark:text-neutral-100"
                          >
                            {fcfa(commande.totalAmount)}
                          </TableCell>
                        );
                      case 'actions':
                        return (
                          <TableCell key={colonne} className={`py-3.5 ${alignement}`}>
                            <div className="flex items-center justify-end gap-1.5">
                              {onOpenTicket && (
                                <button
                                  type="button"
                                  onClick={() => onOpenTicket(commande)}
                                  title="Imprimer le ticket 80 mm"
                                  aria-label={`Imprimer le ticket ${commande.orderNumber}`}
                                  className="rounded p-1 text-neutral-400 transition-colors hover:text-neutral-950 dark:text-neutral-500 dark:hover:text-neutral-200"
                                >
                                  <Printer className="h-3.5 w-3.5" aria-hidden="true" />
                                </button>
                              )}
                              {onNotify && (
                                <button
                                  type="button"
                                  onClick={() => onNotify(commande)}
                                  title="Notifier le client (WhatsApp / SMS)"
                                  aria-label={`Notifier ${commande.customerName}`}
                                  className="rounded p-1 text-neutral-400 transition-colors hover:text-emerald-700 dark:text-neutral-500 dark:hover:text-emerald-400"
                                >
                                  <MessageCircle className="h-3.5 w-3.5" aria-hidden="true" />
                                </button>
                              )}
                              {onAdvance && prochaine ? (
                                <Button
                                  variant="outline"
                                  size="sm"
                                  className="h-7 gap-1 text-[11px] ps-elevated-static"
                                  onClick={() => onAdvance(commande.id)}
                                  aria-label={`${prochaine} — ${commande.orderNumber}`}
                                >
                                  <span>{prochaine}</span>
                                  <ArrowRight
                                    className="h-3 w-3 text-neutral-400 dark:text-neutral-500"
                                    aria-hidden="true"
                                  />
                                </Button>
                              ) : prochaine ? null : (
                                <span className="inline-flex items-center gap-1 text-[11px] text-neutral-400 dark:text-neutral-500">
                                  <Check className="h-3 w-3" aria-hidden="true" />
                                  <span>Clôturé</span>
                                </span>
                              )}
                            </div>
                          </TableCell>
                        );
                    }
                  })}
                </TableRow>
              );
            })
          )}
        </TableBody>
        {lignes.length > 0 && (
          <TableFooter>
            <TableRow className="hover:bg-transparent dark:hover:bg-transparent">
              {columns.map((colonne) => (
                <TableCell
                  key={colonne}
                  className={`px-5 py-2.5 text-[11px] whitespace-nowrap ${A_DROITE.includes(colonne) ? 'text-right' : ''} ${colonne === 'montant' ? 'font-mono tabular-nums' : ''} ${colonne === 'ticket' ? 'text-neutral-500 dark:text-neutral-400' : 'text-neutral-600 dark:text-neutral-400'}`}
                >
                  {contenuPied[colonne]}
                </TableCell>
              ))}
            </TableRow>
          </TableFooter>
        )}
        {caption && <TableCaption>{caption}</TableCaption>}
      </Table>

      {reste > 0 && (
        <div className="flex items-center justify-between border-t border-neutral-100 px-5 py-2.5 dark:border-neutral-800">
          <span className="font-mono text-[11px] text-neutral-500 dark:text-neutral-400">
            {visibles.length} sur {lignes.length} dépôts affichés
          </span>
          <Button
            variant="ghost"
            size="sm"
            className="h-7 text-[11px]"
            onClick={() => setPaliers((actuel) => actuel + 1)}
          >
            Afficher plus
            <ChevronDown className="size-3.5" aria-hidden="true" />
          </Button>
        </div>
      )}
    </div>
  );
}
