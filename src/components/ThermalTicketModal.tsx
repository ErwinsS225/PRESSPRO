import React, { useState } from 'react';
import { X, Printer, CheckCircle, CreditCard, ArrowRight, QrCode } from 'lucide-react';
import { Order } from '../types';

interface ThermalTicketModalProps {
  order: Order;
  tenantName?: string;
  onClose: () => void;
  onSettleBalance: (orderId: string, amount: number, method: string) => void;
}

export const ThermalTicketModal: React.FC<ThermalTicketModalProps> = ({
  order,
  tenantName = 'Pressing Ivoire Cocody',
  onClose,
  onSettleBalance,
}) => {
  const [paymentMethod, setPaymentMethod] = useState<'wave' | 'orange_money' | 'mtn_momo' | 'cash'>('wave');
  const [isSettling, setIsSettling] = useState(false);
  const remainingDue = order.totalAmount - order.paidAmount;

  const handlePrint = () => {
    window.print();
  };

  const handleSettle = () => {
    if (remainingDue <= 0) return;
    onSettleBalance(order.id, remainingDue, paymentMethod);
    setIsSettling(false);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-neutral-900/60 dark:bg-black/80 p-4 backdrop-blur-xs">
      <div className="w-full max-w-md rounded-lg border border-neutral-200 bg-white p-5 shadow-2xl dark:border-neutral-800 dark:bg-[#111111] max-h-[95vh] overflow-y-auto">
        {/* Header bar */}
        <div className="flex items-center justify-between border-b border-neutral-100 dark:border-neutral-800 pb-3">
          <div className="flex items-center gap-2">
            <Printer className="h-4 w-4 text-neutral-500 dark:text-neutral-400" />
            <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
              Aperçu Ticket Thermique (Format 80mm)
            </h2>
          </div>
          <button
            onClick={onClose}
            className="rounded p-1 text-neutral-400 hover:text-neutral-700 dark:text-neutral-500 dark:hover:text-neutral-200"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        {/* Thermal Receipt Paper simulation */}
        <div
          id="printable-thermal-ticket"
          className="my-4 mx-auto w-full max-w-[340px] rounded border border-neutral-300 bg-neutral-50/80 p-5 font-mono text-[11px] text-neutral-900 shadow-inner dark:border-neutral-700 dark:bg-white dark:text-neutral-900"
        >
          {/* Header Pressing */}
          <div className="text-center">
            <div className="font-bold text-sm tracking-tight">{tenantName}</div>
            <div className="text-[10px] text-neutral-600">Cocody Angré 8ème Tranche, Abidjan</div>
            <div className="text-[10px] text-neutral-600">Tél : +225 07 00 11 22 33</div>
            <div className="my-2 border-b border-dashed border-neutral-400" />
          </div>

          {/* Ticket ID & Dates */}
          <div className="space-y-0.5">
            <div className="flex justify-between font-bold text-xs">
              <span>TICKET :</span>
              <span className="tracking-wide">{order.orderNumber}</span>
            </div>
            <div className="flex justify-between text-[10px] text-neutral-600">
              <span>Date Dépôt :</span>
              <span>{order.receivedAt}</span>
            </div>
            <div className="flex justify-between text-[10px] text-neutral-600">
              <span>Retrait Prévu :</span>
              <span className="font-semibold text-neutral-900">{order.dueDate}</span>
            </div>
          </div>

          <div className="my-2 border-b border-dashed border-neutral-400" />

          {/* Customer Info */}
          <div>
            <div className="text-[10px] text-neutral-600">Client :</div>
            <div className="font-semibold">{order.customerName}</div>
            <div className="text-[10px]">{order.customerPhone}</div>
          </div>

          <div className="my-2 border-b border-dashed border-neutral-400" />

          {/* Items breakdown */}
          <div className="space-y-1">
            <div className="flex justify-between text-[10px] font-bold text-neutral-500 uppercase">
              <span>Articles</span>
              <span>Total</span>
            </div>
            {order.items.map((item, idx) => (
              <div key={idx} className="flex justify-between items-baseline">
                <span className="truncate pr-2">
                  {item.quantity}x {item.serviceName}
                </span>
                <span className="font-medium whitespace-nowrap tabular-nums">
                  {item.subtotal.toLocaleString('fr-FR')} F
                </span>
              </div>
            ))}
          </div>

          {order.notes && (
            <div className="mt-2 text-[10px] italic text-neutral-600">
              Instructions : {order.notes}
            </div>
          )}

          <div className="my-2 border-b border-dashed border-neutral-400" />

          {/* Totals & Balances */}
          <div className="space-y-1 text-xs">
            <div className="flex justify-between">
              <span>TOTAL :</span>
              <span className="font-bold tabular-nums">{order.totalAmount.toLocaleString('fr-FR')} FCFA</span>
            </div>
            <div className="flex justify-between text-[11px] text-neutral-600">
              <span>Acompte Versé :</span>
              <span className="tabular-nums">{order.paidAmount.toLocaleString('fr-FR')} FCFA</span>
            </div>
            <div className="flex justify-between font-bold border-t border-neutral-300 pt-1 text-xs">
              <span>RESTE À PAYER :</span>
              <span className="tabular-nums text-neutral-900">
                {remainingDue > 0 ? `${remainingDue.toLocaleString('fr-FR')} FCFA` : 'SOLDÉ (0 F)'}
              </span>
            </div>
          </div>

          <div className="my-3 border-b border-dashed border-neutral-400" />

          {/* Barcode simulation */}
          <div className="flex flex-col items-center justify-center text-center">
            <div className="h-7 w-48 bg-[repeating-linear-gradient(90deg,#000,#000_2px,transparent_2px,transparent_4px)]" />
            <span className="mt-1 font-mono text-[9px] tracking-widest">{order.orderNumber}</span>
            <div className="mt-2 text-[9px] text-neutral-500 italic">
              Merci de votre confiance · Ticket à présenter au retrait
            </div>
          </div>
        </div>

        {/* Actions Zone */}
        <div className="space-y-3 pt-2">
          {remainingDue > 0 && !isSettling && (
            <div className="rounded-md border border-amber-200 bg-amber-50 p-2.5 dark:border-amber-900 dark:bg-amber-950/40">
              <div className="flex items-center justify-between text-xs">
                <span className="font-medium text-amber-800 dark:text-amber-300">
                  Solde dû au retrait : {remainingDue.toLocaleString('fr-FR')} FCFA
                </span>
                <button
                  onClick={() => setIsSettling(true)}
                  className="rounded bg-neutral-900 px-2.5 py-1 text-[11px] font-medium text-white hover:bg-neutral-800 dark:bg-neutral-100 dark:text-neutral-900"
                >
                  Encaisser le Solde
                </button>
              </div>
            </div>
          )}

          {isSettling && (
            <div className="rounded-md border border-neutral-200 bg-neutral-50 p-3 dark:border-neutral-800 dark:bg-neutral-900/60 space-y-2">
              <span className="text-xs font-semibold text-neutral-900 dark:text-neutral-100">
                Moyen de Règlement du Solde ({remainingDue.toLocaleString('fr-FR')} FCFA)
              </span>
              <div className="grid grid-cols-2 gap-2 text-xs">
                {[
                  { id: 'wave', label: 'Wave Côte d’Ivoire' },
                  { id: 'orange_money', label: 'Orange Money' },
                  { id: 'mtn_momo', label: 'MTN Mobile Money' },
                  { id: 'cash', label: 'Espèces au comptoir' },
                ].map((m) => (
                  <button
                    key={m.id}
                    type="button"
                    onClick={() => setPaymentMethod(m.id as any)}
                    className={`rounded border p-2 text-left text-[11px] font-medium transition-colors ${
                      paymentMethod === m.id
                        ? 'border-neutral-900 bg-white font-semibold dark:border-neutral-100 dark:bg-neutral-800 text-neutral-900 dark:text-neutral-100'
                        : 'border-neutral-200 hover:bg-white dark:border-neutral-700 dark:hover:bg-neutral-800 text-neutral-600 dark:text-neutral-400'
                    }`}
                  >
                    {m.label}
                  </button>
                ))}
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setIsSettling(false)}
                  className="rounded px-2.5 py-1 text-xs text-neutral-600 hover:underline dark:text-neutral-400"
                >
                  Annuler
                </button>
                <button
                  type="button"
                  onClick={handleSettle}
                  className="rounded bg-emerald-700 px-3 py-1 text-xs font-medium text-white hover:bg-emerald-800"
                >
                  Valider l'Encaissement
                </button>
              </div>
            </div>
          )}

          <div className="flex items-center justify-between border-t border-neutral-100 dark:border-neutral-800 pt-3">
            <button
              type="button"
              onClick={onClose}
              className="rounded-md border border-neutral-200 px-3 py-1.5 text-xs font-medium text-neutral-700 hover:bg-neutral-50 dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800"
            >
              Fermer
            </button>
            <button
              type="button"
              onClick={handlePrint}
              className="flex items-center gap-1.5 rounded-md bg-neutral-900 px-3 py-1.5 text-xs font-medium text-white hover:bg-neutral-800 dark:bg-neutral-100 dark:text-neutral-900 dark:hover:bg-white"
            >
              <Printer className="h-3.5 w-3.5" />
              <span>Imprimer Ticket (80mm)</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};
