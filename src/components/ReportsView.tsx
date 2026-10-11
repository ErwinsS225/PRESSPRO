import React, { useCallback, useEffect, useState } from 'react';
import {
  BarChart3,
  Building2,
  CreditCard,
  Download,
  FileSpreadsheet,
  FileText,
  Loader2,
  Lock,
  RefreshCw,
  ShoppingBag,
  TrendingDown,
  TrendingUp,
  WifiOff,
} from 'lucide-react';
import { User } from '../types';
import { formatXOF } from '../lib/adminApi';
import { categoryLabel } from '../lib/expensesApi';
import {
  ReportExportFormat,
  ReportPeriod,
  ReportSummary,
  exportReport,
  fetchReportSummary,
  orderStatusLabel,
  periodBounds,
} from '../lib/reportsApi';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';

interface ReportsViewProps {
  currentUser: User;
  token?: string;
  showToast?: (message: string) => void;
}

const PERIODES: { id: 'mois' | 'mois_dernier' | 'trente_jours'; label: string }[] = [
  { id: 'mois', label: 'Ce mois' },
  { id: 'mois_dernier', label: 'Mois dernier' },
  { id: 'trente_jours', label: '30 derniers jours' },
];

/**
 * ReportsView — module Rapports (fonctionnalité payante « reports.advanced »).
 *
 * Tous les chiffres proviennent des agrégats serveur : encaissements réels
 * (journal des paiements), commandes, dépenses et marge. La consolidation par
 * établissement n'apparaît que si la formule inclut « reports.consolidated »
 * (BUSINESS) — sinon une invitation commerciale remplace le tableau.
 */
export const ReportsView: React.FC<ReportsViewProps> = ({ currentUser, token, showToast }) => {
  const [periode, setPeriode] = useState<'mois' | 'mois_dernier' | 'trente_jours'>('mois');
  const [bornes, setBornes] = useState<ReportPeriod>(() => periodBounds('mois'));
  const [rapport, setRapport] = useState<ReportSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [planLocked, setPlanLocked] = useState(false);
  const [exportEnCours, setExportEnCours] = useState<ReportExportFormat | null>(null);
  const [messageExport, setMessageExport] = useState<string | null>(null);

  const gestionnaire = ['owner', 'manager', 'superadmin'].includes(currentUser.role);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    const result = await fetchReportSummary(bornes, token);
    setPlanLocked(!result.ok && Boolean(result.planLocked));
    if (result.ok && result.report) {
      setRapport(result.report);
    } else if (!result.ok) {
      setError(result.error ?? 'rapport indisponible');
    }
    setLoading(false);
  }, [bornes, token]);

  useEffect(() => {
    void load();
  }, [load]);

  const changerPeriode = (id: 'mois' | 'mois_dernier' | 'trente_jours') => {
    setPeriode(id);
    setBornes(periodBounds(id));
  };

  /** Télécharge le rapport de la période courante (CSV pour Excel, PDF à partager). */
  const telechargerRapport = async (format: ReportExportFormat) => {
    setExportEnCours(format);
    setMessageExport(null);
    const result = await exportReport(format, bornes, token);
    if (!result.ok || !result.blob) {
      const message = result.error ?? 'export indisponible';
      setMessageExport(message);
      showToast?.(message);
      setPlanLocked(Boolean(result.planLocked));
    } else {
      const url = URL.createObjectURL(result.blob);
      const lien = document.createElement('a');
      lien.href = url;
      lien.download = result.filename ?? `rapport.${format}`;
      document.body.appendChild(lien);
      lien.click();
      document.body.removeChild(lien);
      URL.revokeObjectURL(url);
      const message = `Rapport exporté : ${lien.download}`;
      setMessageExport(message);
      showToast?.(message);
    }
    setExportEnCours(null);
  };

  const statuts = rapport ? Object.entries(rapport.orders_by_status) : [];
  const margePositive = (rapport?.margin_xof ?? 0) >= 0;

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="flex items-center gap-2 text-lg font-semibold tracking-tight text-neutral-900 dark:text-neutral-100">
            <BarChart3 className="h-5 w-5 text-indigo-500" />
            Rapports
          </h1>
          <p className="mt-0.5 text-xs text-neutral-500 dark:text-neutral-400">
            Activité réelle du pressing : encaissements, commandes, dépenses et marge.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <div className="inline-flex rounded-lg border border-neutral-200 bg-white p-0.5 dark:border-neutral-800 dark:bg-neutral-900 ps-elevated-static">
            {PERIODES.map((option) => (
              <button
                key={option.id}
                type="button"
                onClick={() => changerPeriode(option.id)}
                className={`rounded-md px-2.5 py-1.5 text-[11px] font-medium transition-colors ${
                  periode === option.id
                    ? 'bg-indigo-600 text-white'
                    : 'text-neutral-600 hover:bg-neutral-900/[0.06] dark:text-neutral-300 dark:hover:bg-neutral-800'
                }`}
              >
                {option.label}
              </button>
            ))}
          </div>
          <button
            type="button"
            onClick={() => void load()}
            className="inline-flex items-center gap-1.5 rounded-lg border border-neutral-200 bg-white px-3 py-2 text-xs font-medium text-neutral-700 transition-colors hover:bg-neutral-900/[0.06] dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-200 dark:hover:bg-neutral-800 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated"
          >
            <RefreshCw className="h-3.5 w-3.5" />
            Actualiser
          </button>
          <div className="inline-flex items-center gap-1.5">
            <Download className="h-3.5 w-3.5 text-neutral-400" />
            <button
              type="button"
              onClick={() => void telechargerRapport('csv')}
              disabled={planLocked || exportEnCours !== null}
              className="inline-flex items-center gap-1.5 rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-2 text-xs font-medium text-emerald-800 transition-colors hover:bg-emerald-100 disabled:opacity-50 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-300 dark:hover:bg-emerald-950/70"
              title="Exporter la période au format CSV (séparateur « ; » et BOM UTF-8 : s'ouvre directement dans Excel)"
            >
              <FileSpreadsheet className="h-3.5 w-3.5" />
              {exportEnCours === 'csv' ? 'Export…' : 'CSV'}
            </button>
            <button
              type="button"
              onClick={() => void telechargerRapport('pdf')}
              disabled={planLocked || exportEnCours !== null}
              className="inline-flex items-center gap-1.5 rounded-lg border border-rose-200 bg-rose-50 px-3 py-2 text-xs font-medium text-rose-800 transition-colors hover:bg-rose-100 disabled:opacity-50 dark:border-rose-900 dark:bg-rose-950/40 dark:text-rose-300 dark:hover:bg-rose-950/70"
              title="Exporter la période en PDF (imprimable et partageable)"
            >
              <FileText className="h-3.5 w-3.5" />
              {exportEnCours === 'pdf' ? 'Export…' : 'PDF'}
            </button>
          </div>
        </div>
      </div>

      {messageExport && (
        <div
          role="status"
          className="flex items-center gap-2 rounded-2xl border border-indigo-200 bg-indigo-50 px-4 py-2.5 text-[11px] text-indigo-800 dark:border-indigo-900 dark:bg-indigo-950/40 dark:text-indigo-300"
        >
          <Download className="h-3.5 w-3.5" />
          {messageExport}
        </div>
      )}

      <p className="text-[11px] text-neutral-500 dark:text-neutral-400">
        Période analysée : du {bornes.from} au {bornes.to}
        {rapport?.data_source === 'memoire' && ' · données en mémoire (mode démonstration)'}
      </p>

      {planLocked && (
        <div className="rounded-2xl border border-amber-200 bg-amber-50 p-4 dark:border-amber-900/50 dark:bg-amber-950/30">
          <div className="flex items-center gap-2 text-xs font-medium text-amber-800 dark:text-amber-300">
            <Lock className="h-3.5 w-3.5" />
            Rapports avancés réservés aux formules payantes
          </div>
          <p className="mt-1 text-xs text-amber-700 dark:text-amber-300">
            Les rapports détaillés sont inclus dans les formules PRO et BUSINESS. Passez à une formule
            supérieure depuis « Mon abonnement » pour accéder à vos chiffres.
          </p>
        </div>
      )}

      {!planLocked && error && (
        <div className="flex items-center gap-2 rounded-2xl border border-neutral-200 bg-white p-4 text-xs text-neutral-500 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-400 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
          <WifiOff className="h-3.5 w-3.5" />
          {error}
          {gestionnaire && ' — vérifiez que le backend est démarré.'}
        </div>
      )}

      {loading ? (
        <div className="flex items-center gap-2 rounded-2xl border border-neutral-200 bg-white p-5 text-sm text-neutral-500 dark:border-neutral-800 dark:bg-neutral-900 dark:text-neutral-400 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
          <Loader2 className="h-4 w-4 animate-spin" />
          Calcul du rapport…
        </div>
      ) : (
        rapport && (
          <>
            <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
              <div className="rounded-2xl border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
                <div className="flex items-center gap-2 text-[11px] font-medium uppercase tracking-wide text-neutral-500 dark:text-neutral-400">
                  <CreditCard className="h-3.5 w-3.5 text-emerald-500" />
                  Encaissé
                </div>
                <p className="mt-2 text-2xl font-semibold tracking-tight text-neutral-900 dark:text-neutral-100">
                  {formatXOF(rapport.revenue_xof)}
                </p>
                <p className="mt-1 text-[11px] text-neutral-500 dark:text-neutral-400">
                  Paiements réellement perçus
                </p>
              </div>

              <div className="rounded-2xl border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
                <div className="flex items-center gap-2 text-[11px] font-medium uppercase tracking-wide text-neutral-500 dark:text-neutral-400">
                  <ShoppingBag className="h-3.5 w-3.5 text-sky-500" />
                  Commandes
                </div>
                <p className="mt-2 text-2xl font-semibold tracking-tight text-neutral-900 dark:text-neutral-100">
                  {rapport.orders_count}
                </p>
                <p className="mt-1 text-[11px] text-neutral-500 dark:text-neutral-400">
                  Panier moyen {formatXOF(rapport.average_ticket_xof)}
                </p>
              </div>

              <div className="rounded-2xl border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
                <div className="flex items-center gap-2 text-[11px] font-medium uppercase tracking-wide text-neutral-500 dark:text-neutral-400">
                  <TrendingDown className="h-3.5 w-3.5 text-rose-500" />
                  Dépenses
                </div>
                <p className="mt-2 text-2xl font-semibold tracking-tight text-neutral-900 dark:text-neutral-100">
                  {formatXOF(rapport.expenses_xof)}
                </p>
                <p className="mt-1 text-[11px] text-neutral-500 dark:text-neutral-400">
                  {rapport.expenses_by_category.length} poste(s) de charge
                </p>
              </div>

              <div className="rounded-2xl border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
                <div className="flex items-center gap-2 text-[11px] font-medium uppercase tracking-wide text-neutral-500 dark:text-neutral-400">
                  {margePositive ? (
                    <TrendingUp className="h-3.5 w-3.5 text-emerald-500" />
                  ) : (
                    <TrendingDown className="h-3.5 w-3.5 text-rose-500" />
                  )}
                  Marge
                </div>
                <p
                  className={`mt-2 text-2xl font-semibold tracking-tight ${
                    margePositive
                      ? 'text-emerald-600 dark:text-emerald-400'
                      : 'text-rose-600 dark:text-rose-400'
                  }`}
                >
                  {formatXOF(rapport.margin_xof)}
                </p>
                <p className="mt-1 text-[11px] text-neutral-500 dark:text-neutral-400">Encaissé − dépenses</p>
              </div>
            </div>

            <div className="grid gap-4 lg:grid-cols-2">
              <div className="rounded-2xl border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
                <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
                  Commandes par statut
                </h2>
                {statuts.length === 0 ? (
                  <p className="mt-3 text-xs text-neutral-500 dark:text-neutral-400">
                    Aucune commande sur la période.
                  </p>
                ) : (
                  <ul className="mt-3 space-y-2 text-xs">
                    {statuts.map(([statut, nombre]) => (
                      <li key={statut} className="flex items-center justify-between">
                        <span className="text-neutral-600 dark:text-neutral-300">
                          {orderStatusLabel(statut)}
                        </span>
                        <span className="font-medium text-neutral-900 dark:text-neutral-100">{nombre}</span>
                      </li>
                    ))}
                  </ul>
                )}
              </div>

              <div className="rounded-2xl border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
                <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
                  Prestations les plus vendues
                </h2>
                {rapport.top_services.length === 0 ? (
                  <p className="mt-3 text-xs text-neutral-500 dark:text-neutral-400">
                    Aucune prestation facturée sur la période.
                  </p>
                ) : (
                  <ul className="mt-3 space-y-2 text-xs">
                    {rapport.top_services.map((service) => (
                      <li key={service.label} className="flex items-center justify-between">
                        <span className="text-neutral-600 dark:text-neutral-300">
                          {service.label} <span className="text-neutral-400">× {service.count}</span>
                        </span>
                        <span className="font-medium text-neutral-900 dark:text-neutral-100">
                          {formatXOF(service.revenue_xof)}
                        </span>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            </div>

            <div className="rounded-2xl border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
              <div className="flex items-center gap-2">
                <Building2 className="h-4 w-4 text-sky-500" />
                <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
                  Consolidation par établissement
                </h2>
              </div>

              {!rapport.consolidated ? (
                <p className="mt-3 text-xs text-neutral-500 dark:text-neutral-400">
                  {rapport.plan_hint ?? 'La consolidation multi-sites est incluse dans la formule BUSINESS.'}{' '}
                  Le rapport global ci-dessus reste complet.
                </p>
              ) : (
                <Table containerClassName="mt-3" className="w-full text-left text-xs">
                  <TableHeader className="text-[11px] uppercase tracking-wide text-neutral-500 dark:text-neutral-400">
                    <TableRow>
                      <TableHead className="py-2 font-medium">Établissement</TableHead>
                      <TableHead className="py-2 font-medium">Commandes</TableHead>
                      <TableHead className="py-2 text-right font-medium">Encaissé</TableHead>
                      <TableHead className="py-2 text-right font-medium">Dépenses</TableHead>
                      <TableHead className="py-2 text-right font-medium">Marge</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {(rapport.by_branch ?? []).map((ligne) => (
                      <TableRow
                        key={ligne.branch_id}
                        className="border-t border-neutral-100 dark:border-neutral-800/60"
                      >
                        <TableCell className="py-2 font-medium text-neutral-900 dark:text-neutral-100">
                          {ligne.name}
                        </TableCell>
                        <TableCell className="py-2 text-neutral-600 dark:text-neutral-300">
                          {ligne.orders_count}
                        </TableCell>
                        <TableCell className="py-2 text-right text-neutral-600 dark:text-neutral-300">
                          {formatXOF(ligne.revenue_xof)}
                        </TableCell>
                        <TableCell className="py-2 text-right text-neutral-600 dark:text-neutral-300">
                          {formatXOF(ligne.expenses_xof)}
                        </TableCell>
                        <TableCell
                          className={`py-2 text-right font-medium ${
                            ligne.margin_xof >= 0
                              ? 'text-emerald-600 dark:text-emerald-400'
                              : 'text-rose-600 dark:text-rose-400'
                          }`}
                        >
                          {formatXOF(ligne.margin_xof)}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </div>

            <div className="rounded-2xl border border-neutral-200 bg-white p-5 dark:border-neutral-800 dark:bg-neutral-900 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
              <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
                Détail des dépenses par poste
              </h2>
              {rapport.expenses_by_category.length === 0 ? (
                <p className="mt-3 text-xs text-neutral-500 dark:text-neutral-400">
                  Aucune dépense enregistrée sur la période.
                </p>
              ) : (
                <ul className="mt-3 space-y-2 text-xs">
                  {rapport.expenses_by_category.map((poste) => (
                    <li key={poste.category} className="flex items-center justify-between">
                      <span className="text-neutral-600 dark:text-neutral-300">
                        {categoryLabel(poste.category)}{' '}
                        <span className="text-neutral-400">({poste.count})</span>
                      </span>
                      <span className="font-medium text-neutral-900 dark:text-neutral-100">
                        {formatXOF(poste.total_xof)}
                      </span>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          </>
        )
      )}
    </div>
  );
};
