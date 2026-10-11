import React from 'react';
import {
  TrendingUp,
  Shirt,
  CheckCircle,
  Clock,
  ArrowRight,
  Database,
} from 'lucide-react';
import { Order, HealthCheckResult, ViewMode } from '../types';

interface DashboardViewProps {
  orders: Order[];
  health: HealthCheckResult | null;
  onNavigate: (view: ViewMode) => void;
  onNewOrder: () => void;
}

export const DashboardView: React.FC<DashboardViewProps> = ({
  orders,
  health,
  onNavigate,
  onNewOrder,
}) => {
  const totalRevenue = orders.reduce((sum, o) => sum + o.paidAmount, 0);
  const activeCount = orders.filter((o) => o.status !== 'delivered').length;
  const readyCount = orders.filter((o) => o.status === 'ready').length;

  const statusLabels: Record<Order['status'], { label: string; count: number }> = {
    received: { label: '1. Réceptionné', count: orders.filter((o) => o.status === 'received').length },
    washing: { label: '2. En lavage', count: orders.filter((o) => o.status === 'washing').length },
    ironing: { label: '3. Repassage', count: orders.filter((o) => o.status === 'ironing').length },
    ready: { label: '4. Prêt comptoir', count: orders.filter((o) => o.status === 'ready').length },
    delivered: { label: '5. Livré', count: orders.filter((o) => o.status === 'delivered').length },
  };

  return (
    <div className="space-y-6">
      {/* Top Welcome & Architecture Strip */}
      <div className="flex flex-col gap-3 rounded-lg border border-neutral-200 bg-white p-5 shadow-2xs dark:border-neutral-800 dark:bg-[#111111] sm:flex-row sm:items-center sm:justify-between">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-lg font-semibold text-neutral-900 tracking-tight dark:text-neutral-100">
              Tableau de bord — Pressing Ivoire Cocody
            </h1>
            <span className="font-mono text-[10px] bg-neutral-100 text-neutral-700 px-2 py-0.5 rounded border border-neutral-200 dark:bg-neutral-800 dark:text-neutral-300 dark:border-neutral-700">
              Établissement Actif
            </span>
          </div>
          <p className="mt-1 text-xs text-neutral-500 dark:text-neutral-400">
            Aperçu en temps réel du flux de linge, des encaissements Mobile Money & espèces à Abidjan.
          </p>
        </div>

        <div className="flex items-center gap-2 self-start sm:self-auto">
          <button
            onClick={onNewOrder}
            className="flex items-center gap-1.5 rounded-md bg-neutral-900 px-3 py-1.5 text-xs font-medium text-white hover:bg-neutral-800 dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-white transition-colors"
          >
            <Shirt className="h-3.5 w-3.5" />
            <span>Enregistrer Dépôt</span>
          </button>
        </div>
      </div>

      {/* KPI Cards Grid */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {/* KPI 1: Chiffre d'Affaires */}
        <div className="rounded-lg border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-[#111111]">
          <div className="flex items-center justify-between text-xs text-neutral-500 dark:text-neutral-400">
            <span>Encaissements (Jour)</span>
            <TrendingUp className="h-4 w-4 text-neutral-400 dark:text-neutral-500" />
          </div>
          <div className="mt-2 flex items-baseline gap-1.5">
            <span className="font-mono text-2xl font-semibold tracking-tight text-neutral-900 dark:text-neutral-100 tabular-nums">
              {totalRevenue.toLocaleString('fr-FR')}
            </span>
            <span className="font-mono text-xs text-neutral-500 dark:text-neutral-400 font-medium">FCFA</span>
          </div>
          <div className="mt-1.5 flex items-center gap-1 text-[11px] text-neutral-500 dark:text-neutral-400">
            <span>Espèces + Wave + Orange Money</span>
          </div>
        </div>

        {/* KPI 2: Commandes en cours */}
        <div className="rounded-lg border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-[#111111]">
          <div className="flex items-center justify-between text-xs text-neutral-500 dark:text-neutral-400">
            <span>Dépôts en Traitement</span>
            <Clock className="h-4 w-4 text-neutral-400 dark:text-neutral-500" />
          </div>
          <div className="mt-2 flex items-baseline gap-1.5">
            <span className="font-mono text-2xl font-semibold tracking-tight text-neutral-900 dark:text-neutral-100 tabular-nums">
              {activeCount}
            </span>
            <span className="text-xs text-neutral-500 dark:text-neutral-400">tickets actifs</span>
          </div>
          <div className="mt-1.5 flex items-center gap-1 text-[11px] text-neutral-500 dark:text-neutral-400">
            <span>Sur 1 établissement connecté</span>
          </div>
        </div>

        {/* KPI 3: Linge Prêt au Retrait */}
        <div className="rounded-lg border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-[#111111]">
          <div className="flex items-center justify-between text-xs text-neutral-500 dark:text-neutral-400">
            <span>Prêts au Retrait</span>
            <CheckCircle className="h-4 w-4 text-neutral-400 dark:text-neutral-500" />
          </div>
          <div className="mt-2 flex items-baseline gap-1.5">
            <span className="font-mono text-2xl font-semibold tracking-tight text-neutral-900 dark:text-neutral-100 tabular-nums">
              {readyCount}
            </span>
            <span className="text-xs text-neutral-500 dark:text-neutral-400">au comptoir</span>
          </div>
          <div className="mt-1.5 flex items-center gap-1 text-[11px] text-neutral-500 dark:text-neutral-400">
            <span>Notification WhatsApp envoyée</span>
          </div>
        </div>

        {/* KPI 4: Infrastructure Backend */}
        <div className="rounded-lg border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-[#111111]">
          <div className="flex items-center justify-between text-xs text-neutral-500 dark:text-neutral-400">
            <span>Socle Go & PostgreSQL</span>
            <Database className="h-4 w-4 text-neutral-400 dark:text-neutral-500" />
          </div>
          <div className="mt-2 flex items-baseline gap-1.5">
            <span className="font-mono text-base font-semibold tracking-tight text-neutral-900 dark:text-neutral-100">
              Chi REST / pgx
            </span>
          </div>
          <div className="mt-1.5 flex items-center gap-1 text-[11px] text-emerald-700 dark:text-emerald-400 font-medium">
            <span>{health?.latencyMs ? `Latence probe: ${health.latencyMs}ms` : 'Endpoint opérationnel'}</span>
          </div>
        </div>
      </div>

      {/* Laundry Lifecycle Flow Tracker */}
      <div className="rounded-lg border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-[#111111]">
        <div className="flex items-center justify-between border-b border-neutral-100 pb-3 dark:border-neutral-800">
          <div>
            <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
              Cycle de Traitement du Linge en Atelier
            </h2>
            <p className="text-xs text-neutral-500 dark:text-neutral-400">
              Étapes séquentielles d’un vêtement de la réception jusqu’à la remise au client
            </p>
          </div>
          <span className="text-[11px] text-neutral-400 dark:text-neutral-500 font-mono">
            Pipeline temps réel
          </span>
        </div>

        <div className="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-5">
          {Object.entries(statusLabels).map(([key, item]) => (
            <div
              key={key}
              className="flex flex-col justify-between rounded-md border border-neutral-200/80 bg-neutral-50/50 p-3 dark:border-neutral-800 dark:bg-neutral-900/60"
            >
              <div className="text-[11px] font-medium text-neutral-600 dark:text-neutral-400">{item.label}</div>
              <div className="mt-2 flex items-baseline justify-between">
                <span className="font-mono text-xl font-semibold text-neutral-900 dark:text-neutral-100 tabular-nums">
                  {item.count}
                </span>
                <span className="text-[10px] text-neutral-400 dark:text-neutral-500 font-mono">dépôts</span>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Recent Orders Table */}
      <div className="rounded-lg border border-neutral-200 bg-white dark:border-neutral-800 dark:bg-[#111111]">
        <div className="flex items-center justify-between border-b border-neutral-200 px-5 py-3.5 dark:border-neutral-800">
          <div>
            <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
              Dépôts Récents & Suivi des Tickets
            </h2>
            <p className="text-xs text-neutral-500 dark:text-neutral-400">
              Commandes enregistrées au comptoir avec statut de traitement
            </p>
          </div>
          <button
            onClick={() => onNavigate('orders')}
            className="flex items-center gap-1 text-xs font-medium text-neutral-700 hover:text-neutral-900 dark:text-neutral-300 dark:hover:text-white"
          >
            <span>Voir toutes les commandes</span>
            <ArrowRight className="h-3.5 w-3.5" />
          </button>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs">
            <thead>
              <tr className="border-b border-neutral-100 bg-neutral-50/75 text-[11px] text-neutral-500 font-medium dark:border-neutral-800 dark:bg-neutral-900/60 dark:text-neutral-400">
                <th className="px-5 py-2.5">Ticket</th>
                <th className="px-4 py-2.5">Client & Téléphone</th>
                <th className="px-4 py-2.5">Articles</th>
                <th className="px-4 py-2.5">Statut Atelier</th>
                <th className="px-4 py-2.5">Règlement</th>
                <th className="px-5 py-2.5 text-right">Montant (FCFA)</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-neutral-100 dark:divide-neutral-800 font-normal">
              {orders.slice(0, 5).map((order) => (
                <tr key={order.id} className="hover:bg-neutral-50/50 dark:hover:bg-neutral-800/40 transition-colors">
                  <td className="px-5 py-3 font-mono font-medium text-neutral-900 dark:text-neutral-100">
                    {order.orderNumber}
                  </td>
                  <td className="px-4 py-3">
                    <div className="font-medium text-neutral-900 dark:text-neutral-100">{order.customerName}</div>
                    <div className="font-mono text-[11px] text-neutral-500 dark:text-neutral-400">{order.customerPhone}</div>
                  </td>
                  <td className="px-4 py-3 text-neutral-600 dark:text-neutral-400">
                    {order.items.map((it) => `${it.quantity}x ${it.serviceName}`).join(', ')}
                  </td>
                  <td className="px-4 py-3">
                    <span className="font-mono text-[11px] text-neutral-700 dark:text-neutral-300">
                      {order.status === 'received' && '1. Réceptionné'}
                      {order.status === 'washing' && '2. En lavage'}
                      {order.status === 'ironing' && '3. Repassage'}
                      {order.status === 'ready' && '4. Prêt au comptoir'}
                      {order.status === 'delivered' && '5. Livré au client'}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <span className="font-mono text-[11px] text-neutral-700 dark:text-neutral-300">
                      {order.paymentStatus === 'paid' && 'Payé intégral'}
                      {order.paymentStatus === 'partially_paid' && `Acompte (${order.paidAmount} F)`}
                      {order.paymentStatus === 'unpaid' && 'À payer au retrait'}
                    </span>
                  </td>
                  <td className="px-5 py-3 text-right font-mono font-medium text-neutral-900 dark:text-neutral-100 tabular-nums">
                    {order.totalAmount.toLocaleString('fr-FR')} F
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};
