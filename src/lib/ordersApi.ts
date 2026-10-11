import { Order, OrderItem, OrderStatus, PaymentStatus } from '../types';
import { getAuthHeader } from './servicesApi';

/**
 * Mapping PostgreSQL/Go -> Frontend
 * Le backend renvoie snake_case (created_at, order_number...), le frontend utilise camelCase.
 */
function mapBackendOrder(d: any): Order {
  const items: OrderItem[] = Array.isArray(d.items)
    ? d.items.map((it: any) => ({
        id: it.id,
        serviceName: it.item_label,
        quantity: Number(it.quantity) || 0,
        unitPrice: Number(it.unit_price) || 0,
        subtotal: Number(it.subtotal) || 0,
      }))
    : [];

  // Les dates sont stockées en TIMESTAMPTZ ; on les rend lisibles (l'UI les affiche telles quelles).
  const receivedAt = d.created_at ? new Date(d.created_at).toLocaleString('fr-FR', {
    day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit',
  }) : '';

  let dueDate = '';
  if (d.due_date) {
    const dt = new Date(d.due_date);
    if (!isNaN(dt.getTime())) {
      dueDate = dt.toLocaleString('fr-FR', {
        day: '2-digit', month: '2-digit', year: 'numeric', hour: '2-digit', minute: '2-digit',
      });
    }
  }

  return {
    id: d.id,
    orderNumber: d.order_number,
    customerName: d.customer_name,
    customerPhone: d.customer_phone,
    items,
    status: (d.status as OrderStatus) || 'received',
    paymentStatus: (d.payment_status as PaymentStatus) || 'unpaid',
    totalAmount: Number(d.total_amount) || 0,
    paidAmount: Number(d.paid_amount) || 0,
    receivedAt,
    dueDate,
    notes: d.notes || undefined,
  };
}

/**
 * Charge la liste des commandes depuis PostgreSQL (via le backend Go).
 */
export async function fetchOrders(token?: string): Promise<{ orders: Order[]; fromBackend: boolean }> {
  try {
    const res = await fetch('/api/v1/orders', {
      method: 'GET',
      headers: getAuthHeader(token),
    });
    if (res.ok) {
      const data = await res.json();
      if (Array.isArray(data)) {
        return { orders: data.map(mapBackendOrder), fromBackend: true };
      }
    } else {
      console.warn('[OrdersApi] GET /api/v1/orders -> HTTP', res.status);
    }
  } catch (err: any) {
    console.warn('[OrdersApi] Impossible de joindre /api/v1/orders:', err.message);
  }
  return { orders: [], fromBackend: false };
}

/**
 * Crée une commande côté backend : numéro de ticket atomique, upsert client, lignes, paiement initial.
 * On remplace l'objet local (id/orderNumber fictifs) par la version persistée.
 */
export async function createOrderOnBackend(
  order: Order,
  token?: string
): Promise<{ order: Order | null; synced: boolean; error?: string }> {
  const payload = {
    customer_name: order.customerName,
    customer_phone: order.customerPhone,
    // service_id nullable (migration 000008) : l'historique est figé via item_label.
    items: order.items.map((it) => ({
      service_id: '',
      item_label: it.serviceName,
      quantity: it.quantity,
      unit_price: it.unitPrice,
    })),
    initial_paid: order.paidAmount,
    payment_method: order.paymentStatus === 'unpaid' ? undefined : 'cash',
    due_date: new Date(Date.now() + 2 * 24 * 3600 * 1000).toISOString(),
    notes: order.notes,
  };

  try {
    const res = await fetch('/api/v1/orders', {
      method: 'POST',
      headers: getAuthHeader(token),
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const data = await res.json();
      return { order: mapBackendOrder(data), synced: true };
    }
    const errData = await res.json().catch(() => ({}));
    return { order: null, synced: false, error: errData.error || `Erreur serveur HTTP ${res.status}` };
  } catch (err: any) {
    return { order: null, synced: false, error: err.message || 'Erreur réseau vers /api/v1/orders' };
  }
}

/**
 * Fait avancer le statut d'une commande (réception -> lavage -> repassage -> prêt -> remis).
 */
export async function updateOrderStatusOnBackend(
  orderId: string,
  status: OrderStatus,
  token?: string
): Promise<boolean> {
  try {
    const res = await fetch(`/api/v1/orders/${orderId}/status`, {
      method: 'PATCH',
      headers: getAuthHeader(token),
      body: JSON.stringify({ status }),
    });
    return res.ok;
  } catch (err: any) {
    console.warn('[OrdersApi] Échec PATCH status:', err.message);
    return false;
  }
}

/**
 * Encaisse un paiement (solde ou acompte) sur une commande.
 */
export async function addOrderPaymentOnBackend(
  orderId: string,
  amount: number,
  method: string,
  token?: string
): Promise<boolean> {
  try {
    const res = await fetch(`/api/v1/orders/${orderId}/payments`, {
      method: 'POST',
      headers: getAuthHeader(token),
      body: JSON.stringify({ amount, payment_method: method }),
    });
    return res.ok;
  } catch (err: any) {
    console.warn('[OrdersApi] Échec POST payments:', err.message);
    return false;
  }
}