import React from 'react';
import { Plus } from 'lucide-react';
import { OrdersTable } from '@/components/reui/orders-table';
import { Order } from '../types';

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

      {/* Liste des dépôts : tri, filtre par étape et recherche (composant partagé) */}
      <div className="rounded-lg border border-neutral-200 bg-white dark:border-neutral-800 dark:bg-[#111111]">
        <OrdersTable
          orders={orders}
          toolbar
          pageSize={10}
          onOpenTicket={onOpenThermalTicket}
          onNotify={onNotifyClient}
          onAdvance={onAdvanceStatus}
        />
      </div>
    </div>
  );
};
