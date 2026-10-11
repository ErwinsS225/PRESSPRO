import React, { useMemo } from 'react';
import {
  TrendingUp,
  Shirt,
  CheckCircle,
  Clock,
  ArrowRight,
  Database,
  PlusCircle,
  ListOrdered,
  BarChart3,
} from 'lucide-react';
import { Order, HealthCheckResult, ViewMode } from '../types';
import {
  cleMois,
  dateDepot,
  pipelineAtelier,
  recouvrement,
  serieJournaliere,
  serieMensuelle,
} from '@/lib/analytique';
import { CollectionRadialChart } from '@/components/reui/collection-radial-chart';
import { DepositsBarChart } from '@/components/reui/deposits-bar-chart';
import { DepositsCarousel } from '@/components/reui/deposits-carousel';
import { OrdersPipelineChart } from '@/components/reui/orders-pipeline-chart';
import { RevenueTrendChart } from '@/components/reui/revenue-trend-chart';
import { OrdersStatusChart } from '@/components/reui/orders-status-chart';
import { OrdersTable } from '@/components/reui/orders-table';
import { QuickActionCard } from '@/components/reui/quick-action-card';
import { RevenueDonutChart } from '@/components/reui/revenue-donut-chart';
import { StatCard } from '@/components/reui/stat-card';

interface DashboardViewProps {
  orders: Order[];
  health: HealthCheckResult | null;
  onNavigate: (view: ViewMode) => void;
  onNewOrder: () => void;
  /** Ouvre le ticket thermique d'un dépôt (fourni par l'application). */
  onOpenTicket?: (order: Order) => void;
}

export const DashboardView: React.FC<DashboardViewProps> = ({
  orders,
  health,
  onNavigate,
  onNewOrder,
  onOpenTicket,
}) => {
  const totalRevenue = orders.reduce((sum, o) => sum + o.paidAmount, 0);
  const activeCount = orders.filter((o) => o.status !== 'delivered').length;
  const readyCount = orders.filter((o) => o.status === 'ready').length;

  // Analyse du comptoir : encaissements par prestation et variation mensuelle réelle.
  const { tranchesPrestations, variationMois, moisPrecedent, montantMoisCourant } = useMemo(() => {
    const parPrestation = new Map<string, number>();
    for (const commande of orders) {
      for (const item of commande.items) {
        const cle = item.serviceName || 'Autres prestations';
        parPrestation.set(cle, (parPrestation.get(cle) ?? 0) + item.subtotal);
      }
    }
    const classement = [...parPrestation.entries()].sort((a, b) => b[1] - a[1]);
    const tete = classement.slice(0, 4).map(([nom, montant]) => ({
      source: nom
        .toLowerCase()
        .normalize('NFD')
        .replace(/[^a-z0-9]+/g, '-')
        .replace(/^-|-$/g, ''),
      label: nom,
      value: montant,
    }));
    const reste = classement.slice(4).reduce((somme, [, montant]) => somme + montant, 0);
    if (reste > 0) tete.push({ source: 'autres', label: 'Autres prestations', value: reste });

    const parMois = new Map<string, number>();
    for (const commande of orders) {
      const date = dateDepot(commande.receivedAt);
      if (!date) continue;
      const mois = cleMois(date);
      parMois.set(mois, (parMois.get(mois) ?? 0) + commande.paidAmount);
    }
    const moisTries = [...parMois.keys()].sort();
    const courant = moisTries.at(-1);
    const precedent = moisTries.at(-2);
    const montantCourant = courant ? (parMois.get(courant) ?? 0) : 0;
    const montantPrecedent = precedent ? (parMois.get(precedent) ?? 0) : 0;
    const variation =
      montantPrecedent > 0 ? ((montantCourant - montantPrecedent) / montantPrecedent) * 100 : undefined;

    return {
      tranchesPrestations: tete,
      variationMois: variation,
      moisPrecedent: montantPrecedent,
      montantMoisCourant: montantCourant,
    };
  }, [orders]);

  const statusLabels: Record<Order['status'], { label: string; count: number }> = {
    received: { label: '1. Réceptionné', count: orders.filter((o) => o.status === 'received').length },
    washing: { label: '2. En lavage', count: orders.filter((o) => o.status === 'washing').length },
    ironing: { label: '3. Repassage', count: orders.filter((o) => o.status === 'ironing').length },
    ready: { label: '4. Prêt comptoir', count: orders.filter((o) => o.status === 'ready').length },
    delivered: { label: '5. Livré', count: orders.filter((o) => o.status === 'delivered').length },
  };

  const tranchesStatuts = useMemo(
    () =>
      [
        { statut: 'received', label: 'Réceptionné', value: statusLabels.received.count },
        { statut: 'washing', label: 'En lavage', value: statusLabels.washing.count },
        { statut: 'ironing', label: 'Repassage', value: statusLabels.ironing.count, hachure: true },
        { statut: 'ready', label: 'Prêt comptoir', value: statusLabels.ready.count, hachure: true },
        { statut: 'delivered', label: 'Livré', value: statusLabels.delivered.count },
      ].filter((tranche) => tranche.value > 0),
    [orders]
  );

  // Séries analytiques des graphiques (calculs purs : src/lib/analytique.ts).
  const mois = useMemo(() => serieMensuelle(orders), [orders]);
  const journees = useMemo(() => serieJournaliere(orders), [orders]);
  const reglements = useMemo(() => recouvrement(orders), [orders]);
  const pipeline = useMemo(() => pipelineAtelier(orders), [orders]);

  return (
    <div className="space-y-6">
      {/* Top Welcome & Architecture Strip */}
      <div className="flex flex-col gap-3 rounded-lg border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-[#111111] sm:flex-row sm:items-center sm:justify-between hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
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
            className="flex items-center gap-1.5 rounded-md bg-neutral-900 px-3 py-1.5 text-xs font-medium text-white hover:bg-black dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-white transition-colors"
          >
            <Shirt className="h-3.5 w-3.5" />
            <span>Enregistrer Dépôt</span>
          </button>
        </div>
      </div>

      {/* KPI Cards Grid */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {/* KPI 1: Chiffre d'Affaires */}
        <div className="rounded-lg border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-[#111111] hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
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
        <div className="rounded-lg border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-[#111111] hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
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
        <div className="rounded-lg border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-[#111111] hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
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
        <div className="rounded-lg border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-[#111111] hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
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
            <span>
              {health?.latencyMs ? `Latence probe: ${health.latencyMs}ms` : 'Endpoint opérationnel'}
            </span>
          </div>
        </div>
      </div>

      {/* Laundry Lifecycle Flow Tracker */}
      <div className="rounded-lg border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-[#111111] hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
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
              className="flex flex-col justify-between rounded-md border border-neutral-200/80 bg-neutral-50/50 p-3 dark:border-neutral-800 dark:bg-neutral-900/60 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated"
            >
              <div className="text-[11px] font-medium text-neutral-600 dark:text-neutral-400">
                {item.label}
              </div>
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

      {/* Analyse du comptoir — composants ReUI (c-card-15, c-chart-19, c-chart-21) */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
        <div className="flex flex-col gap-4">
          <StatCard
            title="Chiffre d'affaires encaissé"
            value={`${totalRevenue.toLocaleString('fr-FR')} FCFA`}
            delta={variationMois}
            positive={(variationMois ?? 0) >= 0}
            comparison={`Mois précédent : ${moisPrecedent.toLocaleString('fr-FR')} FCFA`}
          />
          <StatCard
            title="Encaissé sur le mois en cours"
            value={`${montantMoisCourant.toLocaleString('fr-FR')} FCFA`}
            comparison={`Reste à encaisser : ${orders
              .reduce((somme, o) => somme + Math.max(0, o.totalAmount - o.paidAmount), 0)
              .toLocaleString('fr-FR')} FCFA`}
          />
        </div>
        <RevenueDonutChart
          title="Encaissements par prestation"
          description="Chiffre d'affaires détaillé par type de linge traité"
          data={tranchesPrestations}
        />
        <OrdersStatusChart
          title="Suivi de l'atelier"
          description="Répartition du linge par étape de traitement"
          data={tranchesStatuts}
        />
      </div>

      {/* Analytique : séries temporelles, recouvrement et entonnoir d'atelier */}
      <div className="grid grid-cols-1 gap-4 xl:grid-cols-3">
        <div className="xl:col-span-2">
          <RevenueTrendChart
            title="Encaissements des 6 derniers mois"
            description="Facturation et règlements encaissés (espèces, Wave, Orange Money) en FCFA"
            data={mois}
          />
        </div>
        <CollectionRadialChart title="Recouvrement des règlements" recouvrement={reglements} />
        <div className="xl:col-span-2">
          <DepositsBarChart
            title="Charge du comptoir"
            description={`Dépôts et pièces reçues sur les ${journees.length} derniers jours d’activité`}
            data={journees}
          />
        </div>
        <OrdersPipelineChart title="Parcours du linge en atelier" data={pipeline} />
      </div>

      {/* Dépôts récents (c-carousel-10) et raccourcis du comptoir (c-card-17) */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-3">
        <div className="rounded-lg border border-neutral-200 bg-white p-4 dark:border-neutral-800 dark:bg-[#111111] hover:border-neutral-950 dark:hover:border-white/70 ps-elevated lg:col-span-2">
          <div className="mb-3 flex items-center justify-between gap-3">
            <div>
              <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
                Derniers dépôts au comptoir
              </h2>
              <p className="text-xs text-neutral-500 dark:text-neutral-400">
                Parcourez les tickets récents et ouvrez le suivi du linge d&apos;un clic.
              </p>
            </div>
            <button
              type="button"
              onClick={() => onNavigate('orders')}
              className="inline-flex items-center gap-1 rounded-md border border-neutral-200 px-2.5 py-1 text-[11px] font-medium text-neutral-700 transition-colors hover:bg-neutral-950 hover:text-white dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-100 dark:hover:text-neutral-950"
            >
              Tout voir
              <ArrowRight className="h-3 w-3" />
            </button>
          </div>
          <DepositsCarousel orders={orders} onSelect={() => onNavigate('orders')} />
        </div>

        <div className="grid grid-cols-1 gap-4">
          <QuickActionCard
            icon={<PlusCircle aria-hidden="true" />}
            title="Nouveau dépôt de linge"
            description="Enregistrez un client, ses articles et l'avance encaissée en une seule saisie, ticket thermique imprimé."
            linkLabel="Enregistrer un dépôt"
            onSelect={onNewOrder}
          />
          <QuickActionCard
            icon={<ListOrdered aria-hidden="true" />}
            title="Suivi du linge en atelier"
            description="Faites avancer les commandes de la réception à la livraison et notifiez le client par WhatsApp ou SMS."
            linkLabel="Ouvrir le comptoir"
            onSelect={() => onNavigate('orders')}
          />
          <QuickActionCard
            icon={<BarChart3 aria-hidden="true" />}
            title="Rapports & exports"
            description="Analysez le chiffre d'affaires, les dépenses et la marge, puis exportez le rapport en CSV ou PDF."
            linkLabel="Consulter les rapports"
            onSelect={() => onNavigate('reports')}
          />
        </div>
      </div>

      {/* Dépôts récents — tableau de données (tri, filtre, pied récapitulatif) */}
      <div className="rounded-lg border border-neutral-200 bg-white dark:border-neutral-800 dark:bg-[#111111]">
        <div className="flex items-center justify-between gap-3 border-b border-neutral-100 px-5 py-3.5 dark:border-neutral-800">
          <div>
            <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
              Dépôts Récents & Suivi des Tickets
            </h2>
            <p className="text-xs text-neutral-500 dark:text-neutral-400">
              Commandes enregistrées au comptoir : triez les colonnes, filtrez une étape, ouvrez un ticket.
            </p>
          </div>
          <button
            onClick={() => onNavigate('orders')}
            className="flex shrink-0 items-center gap-1 text-xs font-medium text-neutral-700 hover:text-neutral-950 dark:text-neutral-300 dark:hover:text-white"
          >
            <span>Voir toutes les commandes</span>
            <ArrowRight className="h-3.5 w-3.5" />
          </button>
        </div>

        <OrdersTable
          orders={orders}
          pageSize={5}
          onOpenTicket={onOpenTicket}
          columns={
            onOpenTicket
              ? ['ticket', 'client', 'articles', 'statut', 'reglement', 'montant', 'actions']
              : ['ticket', 'client', 'articles', 'statut', 'reglement', 'montant']
          }
        />
      </div>
    </div>
  );
};
