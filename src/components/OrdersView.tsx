import React, { useState } from 'react';
import { Search, Plus, ArrowRight, Check, Printer, MessageCircle } from 'lucide-react';
import { Order, OrderStatus } from '../types';

interface OrdersViewProps {
  orders: Order[];
  onAdvanceStatus: (orderId: string) => void;
  onNewOrder: () => void;
  onOpenThermalTicket: (order: Order) => void;
  onNotifyClient: (order: Order) => void;
}

export const OrdersView: React.FC<OrdersViewProps> = ({
  orders,
  onAdvanceStatus,
  onNewOrder,
  onOpenThermalTicket,
  onNotifyClient,
}) => {
  const [filter, setFilter] = useState<'all' | OrderStatus>('all');
  const [searchQuery, setSearchQuery] = useState('');

  const filteredOrders = orders.filter((order) => {
    const matchesFilter = filter === 'all' || order.status === filter;
    const matchesSearch =
      order.customerName.toLowerCase().includes(searchQuery.toLowerCase()) ||
      order.orderNumber.toLowerCase().includes(searchQuery.toLowerCase()) ||
      order.customerPhone.includes(searchQuery);
    return matchesFilter && matchesSearch;
  });

  const getNextStatusLabel = (status: OrderStatus): string | null => {
    switch (status) {
      case 'received':
        return 'Passer au Lavage';
      case 'washing':
        return 'Passer au Repassage';
      case 'ironing':
        return 'Marquer Prêt';
      case 'ready':
        return 'Confirmer Livraison';
      case 'delivered':
        return null;
    }
  };

  return (
    <div className="space-y-6">
      {/* Top Controls */}
      <div className="flex flex-col gap-3 rounded-lg border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-[#111111] sm:flex-row sm:items-center sm:justify-between hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
        <div>
          <h1 className="text-lg font-semibold tracking-tight text-neutral-900 dark:text-neutral-100">
            Gestion du Linge & Dépôts
          </h1>
          <p className="mt-0.5 text-xs text-neutral-500 dark:text-neutral-400">
            Suivi des vêtements en atelier, impression de ticket thermique (80mm) et encaissements.
          </p>
        </div>

        <button
          onClick={onNewOrder}
          className="flex items-center gap-1.5 self-start rounded-md bg-neutral-900 px-3 py-1.5 text-xs font-medium text-white hover:bg-black dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-white transition-colors sm:self-auto"
        >
          <Plus className="h-3.5 w-3.5" />
          <span>Créer un Dépôt</span>
        </button>
      </div>

      {/* Filter and Search Bar */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        {/* Filter Segmented Control */}
        <div className="flex flex-wrap items-center gap-1 p-1 bg-neutral-100 dark:bg-neutral-900 rounded-lg border border-neutral-200/60 dark:border-neutral-800">
          {(
            [
              { id: 'all', label: 'Toutes' },
              { id: 'received', label: 'Réception' },
              { id: 'washing', label: 'Lavage' },
              { id: 'ironing', label: 'Repassage' },
              { id: 'ready', label: 'Prêt' },
              { id: 'delivered', label: 'Livré' },
            ] as const
          ).map((tab) => (
            <button
              key={tab.id}
              onClick={() => setFilter(tab.id)}
              className={`px-3 py-1 text-xs font-medium rounded-md transition-colors whitespace-nowrap shrink-0 ${
                filter === tab.id
                  ? 'bg-white text-neutral-900 shadow-2xs font-semibold dark:bg-neutral-800 dark:text-neutral-100'
                  : 'text-neutral-600 hover:text-neutral-950 dark:text-neutral-400 dark:hover:text-neutral-200'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>

        {/* Search input */}
        <div className="relative w-full sm:w-64">
          <Search className="absolute left-2.5 top-2.5 h-3.5 w-3.5 text-neutral-400 dark:text-neutral-500" />
          <input
            type="text"
            placeholder="Rechercher ticket, client, tél..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="w-full rounded-md border border-neutral-200 bg-white pl-8 pr-3 py-1.5 text-xs text-neutral-900 placeholder:text-neutral-400 focus:border-neutral-900 focus:outline-hidden dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-100 dark:placeholder:text-neutral-500 dark:focus:border-neutral-100 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated"
          />
        </div>
      </div>

      {/* Orders List Table */}
      <div className="rounded-lg border border-neutral-200 bg-white dark:border-neutral-800 dark:bg-[#111111]">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs">
            <thead>
              <tr className="border-b border-neutral-100 bg-neutral-50/75 text-[11px] text-neutral-500 font-medium dark:border-neutral-800 dark:bg-neutral-900/60 dark:text-neutral-400">
                <th className="px-5 py-2.5">N° Ticket</th>
                <th className="px-4 py-2.5">Client & Contact</th>
                <th className="px-4 py-2.5">Contenu du Dépôt</th>
                <th className="px-4 py-2.5">Date Prévue</th>
                <th className="px-4 py-2.5">Statut Actuel</th>
                <th className="px-4 py-2.5 text-right">Montant</th>
                <th className="px-5 py-2.5 text-right">Action Atelier</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-neutral-100 dark:divide-neutral-800">
              {filteredOrders.length === 0 ? (
                <tr>
                  <td colSpan={7} className="px-5 py-12 text-center text-neutral-500 dark:text-neutral-400">
                    Aucune commande trouvée pour ces critères de recherche.
                  </td>
                </tr>
              ) : (
                filteredOrders.map((order) => {
                  const nextAction = getNextStatusLabel(order.status);
                  const isPaid = order.paymentStatus === 'paid';
                  return (
                    <tr
                      key={order.id}
                      className="hover:bg-neutral-900/[0.06] dark:hover:bg-neutral-800/40 transition-colors"
                    >
                      <td className="px-5 py-3.5">
                        <button
                          onClick={() => onOpenThermalTicket(order)}
                          className="flex items-center gap-1.5 font-mono font-semibold text-neutral-900 hover:text-neutral-950 dark:text-neutral-100 dark:hover:text-neutral-300"
                          title="Voir / Imprimer le ticket thermique"
                        >
                          <Printer className="h-3 w-3 text-neutral-400" />
                          <span>{order.orderNumber}</span>
                        </button>
                      </td>
                      <td className="px-4 py-3.5">
                        <div className="font-medium text-neutral-900 dark:text-neutral-100">
                          {order.customerName}
                        </div>
                        <div className="font-mono text-[11px] text-neutral-500 dark:text-neutral-400">
                          {order.customerPhone}
                        </div>
                      </td>
                      <td className="px-4 py-3.5 text-neutral-600 dark:text-neutral-400">
                        <div>{order.items.map((i) => `${i.quantity}x ${i.serviceName}`).join(', ')}</div>
                        {order.notes && (
                          <div className="text-[11px] text-neutral-400 dark:text-neutral-500 italic mt-0.5">
                            Note: {order.notes}
                          </div>
                        )}
                      </td>
                      <td className="px-4 py-3.5 text-neutral-600 dark:text-neutral-400 font-mono text-[11px]">
                        {order.dueDate}
                      </td>
                      <td className="px-4 py-3.5">
                        <span className="font-mono text-[11px] font-medium text-neutral-800 dark:text-neutral-200">
                          {order.status === 'received' && '1. Réceptionné'}
                          {order.status === 'washing' && '2. En lavage'}
                          {order.status === 'ironing' && '3. Repassage'}
                          {order.status === 'ready' && '4. Prêt au comptoir'}
                          {order.status === 'delivered' && '5. Livré'}
                        </span>
                      </td>
                      <td className="px-4 py-3.5 text-right font-mono font-medium text-neutral-900 dark:text-neutral-100 tabular-nums">
                        {order.totalAmount.toLocaleString('fr-FR')} F
                        <div className="text-[10px] font-normal">
                          {isPaid ? (
                            <span className="text-emerald-700 dark:text-emerald-400">Soldé</span>
                          ) : (
                            <button
                              onClick={() => onOpenThermalTicket(order)}
                              className="text-amber-700 dark:text-amber-400 hover:underline"
                            >
                              Solde dû ({(order.totalAmount - order.paidAmount).toLocaleString('fr-FR')} F)
                            </button>
                          )}
                        </div>
                      </td>
                      <td className="px-5 py-3.5 text-right">
                        <div className="flex items-center justify-end gap-1.5">
                          <button
                            onClick={() => onOpenThermalTicket(order)}
                            className="rounded p-1 text-neutral-400 hover:text-neutral-950 dark:text-neutral-500 dark:hover:text-neutral-200"
                            title="Imprimer le ticket 80mm"
                          >
                            <Printer className="h-3.5 w-3.5" />
                          </button>
                          <button
                            onClick={() => onNotifyClient(order)}
                            className="rounded p-1 text-neutral-400 hover:text-emerald-700 dark:text-neutral-500 dark:hover:text-emerald-400"
                            title="Notifier le client (WhatsApp / SMS)"
                            aria-label={`Notifier ${order.customerName}`}
                          >
                            <MessageCircle className="h-3.5 w-3.5" />
                          </button>
                          {nextAction ? (
                            <button
                              onClick={() => onAdvanceStatus(order.id)}
                              className="inline-flex items-center gap-1 rounded-md border border-neutral-200 bg-neutral-50 px-2.5 py-1 text-[11px] font-medium text-neutral-700 hover:bg-neutral-950 hover:text-white dark:border-neutral-700 dark:bg-neutral-800 dark:text-neutral-300 dark:hover:bg-neutral-700 transition-colors hover:border-neutral-950 dark:hover:border-white/70 ps-elevated dark:hover:bg-neutral-100 dark:hover:text-neutral-950"
                            >
                              <span>{nextAction}</span>
                              <ArrowRight className="h-3 w-3 text-neutral-400 dark:text-neutral-500" />
                            </button>
                          ) : (
                            <span className="inline-flex items-center gap-1 text-[11px] text-neutral-400 dark:text-neutral-500">
                              <Check className="h-3 w-3" />
                              <span>Clôturé</span>
                            </span>
                          )}
                        </div>
                      </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};
