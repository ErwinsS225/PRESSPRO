import React, { useState, useEffect } from 'react';
import { CheckCircle, XCircle, MapPin, Phone, Calendar } from 'lucide-react';
import { Order, Tenant } from '../types';
import { DEMO_ORDERS, CURRENT_TENANT } from '../lib/demoData';

interface TicketVerificationViewProps {
  tenantId: string;
  orderId: string;
}

interface VerifyApiItem {
  label: string;
  quantity: number;
  unit_price: number;
  subtotal: number;
}

interface VerifyApiResponse {
  valid: boolean;
  tenant_id: string;
  order_id: string;
  order_number: string;
  status: string;
  payment_status: string;
  total_amount: number;
  paid_amount: number;
  remaining_due: number;
  due_date: string;
  customer_name: string;
  items: VerifyApiItem[];
}

export const TicketVerificationView: React.FC<TicketVerificationViewProps> = ({ tenantId, orderId }) => {
  const [order, setOrder] = useState<Order | null>(null);
  const [tenant, setTenant] = useState<Tenant | null>(null);
  const [loading, setLoading] = useState(true);
  const [isValid, setIsValid] = useState(false);

  useEffect(() => {
    let cancelled = false;

    // 1. Source de vérité : API publique Go (QR scanné par le client).
    fetch(`/api/v1/public/verify/${encodeURIComponent(tenantId)}/${encodeURIComponent(orderId)}`)
      .then(async (res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        const data = (await res.json()) as VerifyApiResponse;
        if (cancelled || !data.valid) throw new Error('invalid');
        const mapped: Order = {
          id: data.order_id,
          orderNumber: data.order_number,
          customerName: data.customer_name,
          customerPhone: '',
          items: (data.items || []).map((it, idx) => ({
            id: `verify-${idx}`,
            serviceName: it.label,
            quantity: it.quantity,
            unitPrice: it.unit_price,
            subtotal: it.subtotal,
          })),
          status: (data.status as Order['status']) || 'received',
          paymentStatus: (data.payment_status as Order['paymentStatus']) || 'unpaid',
          totalAmount: data.total_amount,
          paidAmount: data.paid_amount,
          receivedAt: '',
          dueDate: data.due_date,
        };
        const mappedTenant: Tenant =
          tenantId === CURRENT_TENANT.id
            ? CURRENT_TENANT
            : {
                id: data.tenant_id,
                name: 'Pressing partenaire',
                slug: 'pressing-partenaire',
                phone: '',
                city: 'Côte d’Ivoire',
                currency: 'XOF',
                status: 'active',
              };
        setOrder(mapped);
        setTenant(mappedTenant);
        setIsValid(true);
        setLoading(false);
      })
      .catch(() => {
        if (cancelled) return;
        // 2. Repli local (mode démo hors-ligne / backend indisponible).
        const foundOrder = DEMO_ORDERS.find((o) => o.id === orderId);
        const foundTenant = tenantId === CURRENT_TENANT.id ? CURRENT_TENANT : null;
        if (foundOrder && foundTenant) {
          setOrder(foundOrder);
          setTenant(foundTenant);
          setIsValid(true);
        } else {
          setIsValid(false);
        }
        setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [tenantId, orderId]);

  // Formater les dates (ISO backend ou libellé démo type « Demain à 17:00 »).
  const formatDate = (dateStr: string) => {
    const date = new Date(dateStr);
    if (!isNaN(date.getTime())) {
      return date.toLocaleDateString('fr-FR', {
        day: '2-digit',
        month: '2-digit',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      });
    }
    return dateStr || '—';
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-gray-50 dark:bg-gray-900 flex items-center justify-center">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mx-auto"></div>
          <p className="mt-4 text-gray-600 dark:text-gray-400">Vérification de votre ticket...</p>
        </div>
      </div>
    );
  }

  if (!isValid || !order || !tenant) {
    return (
      <div className="min-h-screen bg-gray-50 dark:bg-gray-900 flex items-center justify-center p-4">
        <div className="max-w-md w-full bg-white dark:bg-gray-800 rounded-xl shadow-lg p-8 text-center">
          <XCircle className="h-16 w-16 text-red-500 mx-auto" />
          <h1 className="mt-4 text-2xl font-bold text-gray-900 dark:text-white">Ticket invalide</h1>
          <p className="mt-2 text-gray-600 dark:text-gray-400">
            Ce ticket n'existe pas ou a expiré. Veuillez contacter votre pressing.
          </p>
        </div>
      </div>
    );
  }

  // Ticket valide - affichage des informations
  const remainingDue = order.totalAmount - order.paidAmount;
  const isFullyPaid = remainingDue <= 0;

  return (
    <div className="min-h-screen bg-gray-50 dark:bg-gray-900 flex items-center justify-center p-4">
      <div className="max-w-md w-full bg-white dark:bg-gray-800 rounded-xl shadow-lg p-6">
        {/* Header de validation */}
        <div className="text-center">
          <CheckCircle className="h-20 w-20 text-green-500 mx-auto" />
          <h1 className="mt-4 text-2xl font-bold text-gray-900 dark:text-white">Ticket Valide !</h1>
          <p className="mt-1 text-gray-600 dark:text-gray-400">Votre commande a été vérifiée avec succès</p>
        </div>

        <div className="mt-6 border-t border-gray-200 dark:border-gray-700 pt-6">
          {/* Informations du pressing */}
          <div className="bg-blue-50 dark:bg-blue-900/30 rounded-lg p-4 mb-4">
            <h3 className="font-semibold text-blue-900 dark:text-blue-100">{tenant.name}</h3>
            <div className="mt-2 space-y-1 text-sm text-blue-800 dark:text-blue-200">
              <div className="flex items-center gap-2">
                <MapPin className="h-4 w-4" />
                <span>{tenant.city}, Côte d'Ivoire</span>
              </div>
              <div className="flex items-center gap-2">
                <Phone className="h-4 w-4" />
                <span>{tenant.phone}</span>
              </div>
            </div>

            {/* Horaires d'ouverture sur la page de vérification */}
            {tenant.openingHours && (
              <div className="mt-3 pt-3 border-t border-blue-200 dark:border-blue-700">
                <div className="text-xs font-medium text-blue-900 dark:text-blue-100 mb-2">
                  Horaires d'ouverture
                </div>
                <div className="grid grid-cols-2 gap-x-4 gap-y-1 text-xs text-blue-800 dark:text-blue-200">
                  {tenant.openingHours.map((day, idx) => (
                    <div key={idx} className="flex justify-between">
                      <span>{day.day}:</span>
                      <span className={day.isClosed ? 'text-red-600 font-medium' : ''}>
                        {day.isClosed ? 'Fermé' : `${day.open}-${day.close}`}
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            )}
          </div>

          {/* Informations de la commande */}
          <div className="space-y-4">
            <div className="flex justify-between items-center">
              <span className="text-gray-500 dark:text-gray-400">N° Ticket</span>
              <span className="font-mono font-bold text-gray-900 dark:text-white">{order.orderNumber}</span>
            </div>

            <div className="flex items-center gap-2">
              <Calendar className="h-4 w-4 text-gray-500" />
              <span className="text-sm text-gray-500 dark:text-gray-400">
                Date de retrait : {formatDate(order.dueDate).split(' ')[0]}
              </span>
            </div>

            {/* Statut de paiement */}
            <div
              className={`rounded-lg p-3 ${isFullyPaid ? 'bg-green-50 dark:bg-green-900/30' : 'bg-amber-50 dark:bg-amber-900/30'}`}
            >
              <div className="flex items-center justify-between">
                <span
                  className={`font-medium ${isFullyPaid ? 'text-green-800 dark:text-green-200' : 'text-amber-800 dark:text-amber-200'}`}
                >
                  {isFullyPaid ? '✅ Completement payé' : '⚠️ Solde à payer'}
                </span>
                <span
                  className={`font-bold ${isFullyPaid ? 'text-green-700 dark:text-green-300' : 'text-amber-700 dark:text-amber-300'}`}
                >
                  {isFullyPaid ? '0 FCFA' : `${remainingDue.toLocaleString('fr-FR')} FCFA`}
                </span>
              </div>
            </div>

            {/* Liste des articles */}
            <div className="border-t border-gray-200 dark:border-gray-700 pt-4">
              <h4 className="font-semibold text-gray-900 dark:text-white mb-3">Vos articles</h4>
              <div className="space-y-2">
                {order.items.map((item, idx) => (
                  <div key={idx} className="flex justify-between text-sm">
                    <span className="text-gray-600 dark:text-gray-400">
                      {item.quantity}x {item.serviceName}
                    </span>
                    <span className="font-medium text-gray-900 dark:text-white">
                      {item.subtotal.toLocaleString('fr-FR')} FCFA
                    </span>
                  </div>
                ))}
              </div>
            </div>

            {/* Total */}
            <div className="border-t border-gray-200 dark:border-gray-700 pt-4">
              <div className="flex justify-between items-center">
                <span className="font-bold text-gray-900 dark:text-white">Total</span>
                <span className="font-bold text-xl text-gray-900 dark:text-white">
                  {order.totalAmount.toLocaleString('fr-FR')} FCFA
                </span>
              </div>
            </div>
          </div>
        </div>

        {/* Pied de page */}
        <div className="mt-8 text-center text-xs text-gray-500 dark:text-gray-400">
          <p>Powered by PressingOS Côte d'Ivoire</p>
          <p className="mt-1">Ticket vérifié le {new Date().toLocaleDateString('fr-FR')}</p>
        </div>
      </div>
    </div>
  );
};
