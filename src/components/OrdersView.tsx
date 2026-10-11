import React, { useState } from 'react';
import { Search, Filter, Plus, ArrowRight, Check, Printer, Clock } from 'lucide-react';
import { Order, OrderStatus } from '../types';

interface OrdersViewProps {
  orders: Order[];
  onAdvanceStatus: (orderId: string) => void;
  onNewOrder: () => void;
}

export const OrdersView: React.FC<OrdersViewProps> = ({
  orders,
  onAdvanceStatus,
  onNewOrder,
}) => {
  const [filter, setFilter] = useState<'all' | OrderStatus>('all');
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedOrder, setSelectedOrder] = useState<Order | null>(null);

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
      <div className="flex flex-col gap-3 rounded-lg border border-neutral-200 bg-white p-5 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-lg font-semibold tracking-tight text-neutral-900">
            Gestion du Linge & Dépôts
          </h1>
          <p className="mt-0.5 text-xs text-neutral-500">
            Suivi des vêtements en atelier, impression de ticket et avancement des étapes.
          </p>
        </div>

        <button
          onClick={onNewOrder}
          className="flex items-center gap-1.5 self-start rounded-md bg-neutral-900 px-3 py-1.5 text-xs font-medium text-white hover:bg-neutral-800 transition-colors sm:self-auto"
        >
          <Plus className="h-3.5 w-3.5" />
          <span>Créer un Dépôt</span>
        </button>
      </div>

      {/* Filter and Search Bar */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        {/* Filter Segmented Control */}
        <div className="flex flex-wrap items-center gap-1 p-1 bg-neutral-100 rounded-lg">
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
                  ? 'bg-white text-neutral-900 shadow-2xs font-semibold'
                  : 'text-neutral-600 hover:text-neutral-900'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>

        {/* Search input */}
        <div className="relative w-full sm:w-64">
          <Search className="absolute left-2.5 top-2.5 h-3.5 w-3.5 text-neutral-400" />
          <input
            type="text"
            placeholder="Rechercher ticket, client, tél..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="w-full rounded-md border border-neutral-200 bg-white pl-8 pr-3 py-1.5 text-xs text-neutral-900 placeholder:text-neutral-400 focus:border-neutral-900 focus:outline-hidden"
          />
        </div>
      </div>

      {/* Orders List Table */}
      <div className="rounded-lg border border-neutral-200 bg-white">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs">
            <thead>
              <tr className="border-b border-neutral-100 bg-neutral-50/75 text-[11px] text-neutral-500 font-medium">
                <th className="px-5 py-2.5">N° Ticket</th>
                <th className="px-4 py-2.5">Client & Contact</th>
                <th className="px-4 py-2.5">Contenu du Dépôt</th>
                <th className="px-4 py-2.5">Date Prévue</th>
                <th className="px-4 py-2.5">Statut Actuel</th>
                <th className="px-4 py-2.5 text-right">Montant</th>
                <th className="px-5 py-2.5 text-right">Action Atelier</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-neutral-100">
              {filteredOrders.length === 0 ? (
                <tr>
                  <td colSpan={7} className="px-5 py-12 text-center text-neutral-500">
                    Aucune commande trouvée pour ces critères de recherche.
                  </td>
                </tr>
              ) : (
                filteredOrders.map((order) => {
                  const nextAction = getNextStatusLabel(order.status);
                  return (
                    <tr key={order.id} className="hover:bg-neutral-50/50 transition-colors">
                      <td className="px-5 py-3.5 font-mono font-medium text-neutral-900">
                        {order.orderNumber}
                      </td>
                      <td className="px-4 py-3.5">
                        <div className="font-medium text-neutral-900">{order.customerName}</div>
                        <div className="font-mono text-[11px] text-neutral-500">{order.customerPhone}</div>
                      </td>
                      <td className="px-4 py-3.5 text-neutral-600">
                        <div>{order.items.map((i) => `${i.quantity}x ${i.serviceName}`).join(', ')}</div>
                        {order.notes && (
                          <div className="text-[11px] text-neutral-400 italic mt-0.5">Note: {order.notes}</div>
                        )}
                      </td>
                      <td className="px-4 py-3.5 text-neutral-600 font-mono text-[11px]">
                        {order.dueDate}
                      </td>
                      <td className="px-4 py-3.5">
                        <span className="font-mono text-[11px] font-medium text-neutral-800">
                          {order.status === 'received' && '1. Réceptionné'}
                          {order.status === 'washing' && '2. En lavage'}
                          {order.status === 'ironing' && '3. Repassage'}
                          {order.status === 'ready' && '4. Prêt au comptoir'}
                          {order.status === 'delivered' && '5. Livré'}
                        </span>
                      </td>
                      <td className="px-4 py-3.5 text-right font-mono font-medium text-neutral-900 tabular-nums">
                        {order.totalAmount.toLocaleString('fr-FR')} F
                        <div className="text-[10px] text-neutral-400 font-normal">
                          {order.paymentStatus === 'paid' ? 'Payé' : 'Solde dû'}
                        </div>
                      </td>
                      <td className="px-5 py-3.5 text-right">
                        {nextAction ? (
                          <button
                            onClick={() => onAdvanceStatus(order.id)}
                            className="inline-flex items-center gap-1 rounded-md border border-neutral-200 bg-neutral-50 px-2.5 py-1 text-[11px] font-medium text-neutral-700 hover:bg-neutral-100 transition-colors"
                          >
                            <span>{nextAction}</span>
                            <ArrowRight className="h-3 w-3 text-neutral-400" />
                          </button>
                        ) : (
                          <span className="inline-flex items-center gap-1 text-[11px] text-neutral-400">
                            <Check className="h-3 w-3" />
                            <span>Clôturé</span>
                          </span>
                        )}
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
