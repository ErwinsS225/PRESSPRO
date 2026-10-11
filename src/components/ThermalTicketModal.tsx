import React, { useState, useEffect } from 'react';
import { X, Printer, QrCode } from 'lucide-react';
import { Order, Tenant } from '../types';

interface ThermalTicketModalProps {
  order: Order;
  tenant: Tenant; // Informations complètes du pressing (tenant)
  onClose: () => void;
  onSettleBalance: (orderId: string, amount: number, method: string) => void;
}

// Générer le contenu du QR code pour la vérification du ticket
const generateQRContent = (order: Order, tenant: Tenant) => {
  return `${window.location.origin}/verify/${tenant.id}/${order.id}`;
};

export const ThermalTicketModal: React.FC<ThermalTicketModalProps> = ({
  order,
  tenant,
  onClose,
  onSettleBalance,
}) => {
  const [paymentMethod, setPaymentMethod] = useState<'wave' | 'orange_money' | 'mtn_momo' | 'cash'>('wave');
  const [isSettling, setIsSettling] = useState(false);
  const [qrCodeUrl, setQrCodeUrl] = useState('');
  const [customerPhone, setCustomerPhone] = useState<string>(order.customerPhone || '');
  const [isSending, setIsSending] = useState<boolean>(false);
  const remainingDue = order.totalAmount - order.paidAmount;

  // Générer le QR code automatiquement à l'ouverture
  useEffect(() => {
    const qrContent = generateQRContent(order, tenant);
    // Utiliser l'API QR Code gratuite de Google pour générer un QR code scannable
    setQrCodeUrl(`https://api.qrserver.com/v1/create-qr-code/?size=100x100&data=${encodeURIComponent(qrContent)}`);
  }, [order, tenant]);

  // Fonction d'impression optimisée pour les imprimantes thermiques 80mm
  const handlePrint = () => {
    // Ajouter une classe pour masquer les éléments non nécessaires à l'impression
    document.body.classList.add('printing-thermal-ticket');
    window.print();
    // Supprimer la classe après impression
    setTimeout(() => document.body.classList.remove('printing-thermal-ticket'), 1000);
  };

  const handleSettle = () => {
    if (remainingDue <= 0) return;
    onSettleBalance(order.id, remainingDue, paymentMethod);
    setIsSettling(false);
  };

  // Générer le message WhatsApp/SMS à envoyer au client
  const generateShareMessage = () => {
    const verifyUrl = `${window.location.origin}/verify/${tenant.id}/${order.id}`;
    return encodeURIComponent(
      `*${tenant.name}* - Votre commande de pressing est prête !\n\n` +
      `📋 Commande N°${order.orderNumber}\n` +
      `💰 Montant total : ${order.totalAmount.toLocaleString('fr-FR')} FCFA\n` +
      `✅ Payé : ${order.paidAmount.toLocaleString('fr-FR')} FCFA\n` +
      `${remainingDue > 0 ? `💸 Reste à payer : ${remainingDue.toLocaleString('fr-FR')} FCFA\n` : '\n'}` +
      `🔍 Vérifiez l'authenticité de votre ticket :\n${verifyUrl}\n\n` +
      `À bientôt chez ${tenant.name} !`
    );
  };

  // Envoyer par WhatsApp
  const handleSendWhatsApp = () => {
    if (!customerPhone.trim()) {
      alert('Veuillez saisir le numéro de téléphone du client');
      return;
    }
    // Nettoyer le numéro pour le format WhatsApp international
    const cleanPhone = customerPhone.replace(/\s/g, '').replace(/^\+/, '');
    const message = generateShareMessage();
    window.open(`https://wa.me/${cleanPhone}?text=${message}`, '_blank');
  };

  // Envoyer par SMS
  const handleSendSMS = () => {
    if (!customerPhone.trim()) {
      alert('Veuillez saisir le numéro de téléphone du client');
      return;
    }
    const cleanPhone = customerPhone.replace(/\s/g, '').replace(/^\+/, '');
    const message = generateShareMessage();
    window.open(`sms:${cleanPhone}?body=${message}`, '_blank');
  };

  // Formater les dates (ISO backend ou libellé démo type « Demain à 17:00 »).
  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr);
    if (!isNaN(date.getTime())) {
      return date.toLocaleDateString('fr-FR', {
        day: '2-digit',
        month: '2-digit',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit'
      });
    }
    return dateStr || '—';
  };

  // Vérifier si le paiement est 100% soldé
  const isFullyPaid = remainingDue <= 0;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-neutral-900/60 dark:bg-black/80 p-4 backdrop-blur-xs">
      <div className="w-full max-w-md rounded-lg border border-neutral-200 bg-white p-5 shadow-2xl dark:border-neutral-800 dark:bg-[#111111] max-h-[95vh] overflow-y-auto">
        {/* Header bar */}
        <div className="flex items-center justify-between border-b border-neutral-100 dark:border-neutral-800 pb-3">
          <div className="flex items-center gap-2">
            <Printer className="h-4 w-4 text-neutral-500 dark:text-neutral-400" />
            <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
              Ticket Thermique Prêt (80mm)
            </h2>
          </div>
          <button
            onClick={onClose}
            className="rounded p-1 text-neutral-400 hover:text-neutral-700 dark:text-neutral-500 dark:hover:text-neutral-200"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        {/* 🎯 Ticket optimisé pour impression thermique 80mm */}
        <div
          id="printable-thermal-ticket"
          className="my-4 mx-auto w-full max-w-[320px] bg-white p-4 font-mono text-[10px] text-black print:shadow-none print:border-0 print:p-2"
          style={{
            width: '80mm', // Taille exacte d'un ticket thermique 80mm
            minHeight: '150mm',
            printColorAdjust: 'exact',
            WebkitPrintColorAdjust: 'exact'
          }}
        >
          {/* EN-TÊTE DU PRESSING (CENTRÉ, GRAND FORMAT) */}
          <div className="text-center mb-3">
            <div className="font-bold text-base tracking-wide uppercase">{tenant.name}</div>
            <div className="text-[9px] mt-1">{tenant.city}, Côte d'Ivoire</div>
            <div className="text-[9px]">Tél : {tenant.phone || '+225 XX XX XXX XX'}</div>
            <div className="text-[8px] text-neutral-500 mt-1">Géré par PressingOS • Votre pressing digitalisé</div>
            <div className="my-2 border-b border-black border-dashed" />
          </div>

          {/* RÉFÉRENCE TICKET ET DATES */}
          <div className="space-y-1 mb-3">
            <div className="flex justify-between font-bold">
              <span>N° TICKET:</span>
              <span className="tracking-wider">{order.orderNumber}</span>
            </div>
            <div className="flex justify-between text-[9px]">
              <span>Dépôt:</span>
              <span>{formatDate(order.receivedAt)}</span>
            </div>
            <div className="flex justify-between text-[9px]">
              <span>Retrait:</span>
              <span className="font-bold">{formatDate(order.dueDate)}</span>
            </div>
          </div>

          <div className="my-2 border-b border-black border-dashed" />

          {/* INFOS CLIENT */}
          <div className="mb-3">
            <div className="text-[9px] text-neutral-600">Client:</div>
            <div className="font-bold">{order.customerName}</div>
            <div className="text-[9px]">{order.customerPhone}</div>
          </div>

          <div className="my-2 border-b border-black border-dashed" />

          {/* DÉTAILS DES ARTICLES */}
          <div className="mb-3">
            <div className="flex justify-between font-bold uppercase text-[9px] mb-2">
              <span>Article</span>
              <span>Prix</span>
            </div>
            {order.items.map((item, idx) => (
              <div key={idx} className="flex justify-between items-baseline mb-1">
                <span className="truncate pr-2">
                  {item.quantity}x {item.serviceName}
                </span>
                <span className="font-medium whitespace-nowrap">
                  {item.subtotal.toLocaleString('fr-FR')} FCFA
                </span>
              </div>
            ))}
          </div>

          {/* INSTRUCTIONS SPÉCIALES */}
          {order.notes && (
            <div className="mb-3 p-2 border border-neutral-300 text-[9px] italic">
              📝 {order.notes}
            </div>
          )}

          <div className="my-2 border-b border-black border-dashed" />

          {/* TOTAUX */}
          <div className="space-y-2 mb-3">
            <div className="flex justify-between">
              <span className="font-bold">TOTAL:</span>
              <span className="font-bold">{order.totalAmount.toLocaleString('fr-FR')} FCFA</span>
            </div>
            <div className="flex justify-between text-[9px]">
              <span>Déjà payé:</span>
              <span>{order.paidAmount.toLocaleString('fr-FR')} FCFA</span>
            </div>
            <div className={`flex justify-between font-bold border-t-2 border-black pt-2 ${isFullyPaid ? 'text-green-700' : 'text-red-700'}`}>
              <span>{isFullyPaid ? '✅ SOLDÉ' : '⚠️ RESTE:'}</span>
              <span>
                {isFullyPaid ? '0 FCFA' : `${remainingDue.toLocaleString('fr-FR')} FCFA`}
              </span>
            </div>
          </div>

          {/* MOYENS DE PAIEMENT ACCEPTÉS (IVOIRIENS) */}
          <div className="text-center text-[8px] mb-3">
            <div className="font-bold mb-1">Moyens de paiement acceptés:</div>
            <div className="flex justify-center gap-2">
              <span>Wave</span>
              <span>Orange Money</span>
              <span>MTN MoMo</span>
              <span>Espèces</span>
            </div>
          </div>

          <div className="my-2 border-b border-black border-dashed" />

          {/* QR CODE POUR VÉRIFICATION DU TICKET */}
          <div className="flex flex-col items-center justify-center mb-3">
            {qrCodeUrl && (
              <img
                src={qrCodeUrl}
                alt="QR Code de vérification"
                className="w-20 h-20"
              />
            )}
            <div className="text-[8px] mt-2 text-center">
              Scannez pour vérifier votre commande<br />
              Ticket à présenter obligatoirement au retrait
            </div>
          </div>

          {/* HORAIRES D'OUVERTURE */}
          {tenant.openingHours && (
            <div className="text-center mt-3 mb-3 text-[8px]">
              <div className="font-bold mb-1">Nos Horaires d'Ouverture</div>
              <div className="space-y-0.5">
                {tenant.openingHours.map((day, idx) => (
                  <div key={idx} className="flex justify-between px-4">
                    <span>{day.day}</span>
                    <span className={day.isClosed ? 'text-red-600 font-medium' : ''}>
                      {day.isClosed ? 'FERMÉ' : `${day.open} - ${day.close}`}
                    </span>
                  </div>
                ))}
              </div>
              <div className="my-2 border-b border-black border-dashed" />
            </div>
          )}

          {/* PIED DE PAGE */}
          <div className="text-center mt-4 text-[8px] text-neutral-500">
            <div className="font-bold">Merci pour votre confiance ! 🙏</div>
            <div className="mt-1">{tenant.name} - Tous droits réservés</div>
            <div className="mt-1">Powered by PressingOS Côte d'Ivoire</div>
          </div>

          {/* MARGE DE COUPURE POUR L'IMPRIMANTE */}
          <div className="mt-4 border-t-4 border-double border-black" />
        </div>

        {/* Actions Zone */}
        <div className="space-y-3 pt-2">
          {/* Section Envoi WhatsApp/SMS */}
          <div className="rounded-md border border-emerald-200 bg-emerald-50 p-3 dark:border-emerald-900 dark:bg-emerald-950/40">
            <div className="space-y-2">
              <div className="text-xs font-medium text-emerald-800 dark:text-emerald-300">
                📱 Envoyer le ticket au client par WhatsApp/SMS
              </div>
              <input
                type="tel"
                value={customerPhone}
                onChange={(e) => setCustomerPhone(e.target.value)}
                placeholder="Numéro du client (ex: +2250708091011)"
                className="w-full rounded-md border border-neutral-300 px-3 py-2 text-sm dark:border-neutral-700 dark:bg-neutral-900 dark:text-neutral-100 focus:outline-none focus:ring-2 focus:ring-emerald-500"
              />
              <div className="flex gap-2">
                <button
                  onClick={handleSendWhatsApp}
                  disabled={isSending}
                  className="flex-1 flex items-center justify-center gap-1.5 rounded-md bg-green-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-green-700 disabled:opacity-50"
                >
                  <svg className="h-3.5 w-3.5" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M17.472 14.382c-.297-.149-1.758-.867-2.03-.967-.273-.099-.471-.148-.67.15-.197.297-.767.966-.94 1.164-.173.199-.347.223-.644.075-.297-.15-1.255-.463-2.39-1.475-.883-.788-1.48-1.761-1.653-2.059-.173-.297-.018-.458.13-.606.134-.133.298-.347.446-.52.149-.174.198-.298.298-.497.099-.198.05-.371-.025-.52-.075-.149-.669-1.612-.916-2.207-.242-.579-.487-.5-.669-.51-.173-.008-.371-.01-.57-.01-.198 0-.52.074-.792.372-.272.297-1.04 1.016-1.04 2.479 0 1.462 1.065 2.875 1.213 3.074.149.198 2.096 3.2 5.077 4.487.709.306 1.262.489 1.694.625.712.227 1.36.195 1.871.118.571-.085 1.758-.719 2.006-1.413.248-.694.248-1.289.173-1.413-.074-.124-.272-.198-.57-.347m-5.421 7.403h-.004a9.87 9.87 0 01-5.031-1.378l-.361-.214-3.741.982.998-3.648-.235-.374a9.86 9.86 0 01-1.51-5.26c.001-5.45 4.436-9.884 9.888-9.884 2.64 0 5.122 1.03 6.988 2.898a9.825 9.825 0 012.893 6.994c-.003 5.45-4.437 9.884-9.885 9.884m8.413-18.297A11.815 11.815 0 0012.05 0C5.495 0 .16 5.335.157 11.892c0 2.096.547 4.142 1.588 5.945L.057 24l6.305-1.654a11.882 11.882 0 005.683 1.448h.005c6.554 0 11.89-5.335 11.893-11.893a11.821 11.821 0 00-3.48-8.413z" />
                  </svg>
                  WhatsApp
                </button>
                <button
                  onClick={handleSendSMS}
                  disabled={isSending}
                  className="flex-1 flex items-center justify-center gap-1.5 rounded-md bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-700 disabled:opacity-50"
                >
                  <svg className="h-3.5 w-3.5" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M20 2H4c-1.1 0-2 .9-2 2v18l4-4h14c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2z" />
                  </svg>
                  SMS
                </button>
              </div>
            </div>
          </div>

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
                    className={`rounded border p-2 text-left text-[11px] font-medium transition-colors ${paymentMethod === m.id
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