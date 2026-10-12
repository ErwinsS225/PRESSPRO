import { afterEach, describe, expect, it, vi } from 'vitest';
import { createOrderOnBackend, fetchOrders } from './ordersApi';
import type { Order } from '../types';

const draft: Order = {
  id: 'temporary',
  customerId: 'client-b',
  orderNumber: '',
  customerName: 'Awa Kouadio',
  customerPhone: '0700000000',
  items: [],
  status: 'received',
  paymentStatus: 'paid',
  totalAmount: 1000,
  paidAmount: 1000,
  amountReceived: 2000,
  changeGiven: 1000,
  paymentMethod: 'cash',
  receivedAt: '',
  dueDate: '',
};
const response = {
  id: 'persisted-order',
  customer_id: 'client-b',
  customer_name: 'Awa Kouadio',
  customer_phone: '0700000000',
  order_number: 'CMD-2026-001',
  items: [],
  payments: [
    {
      id: 'pay-1',
      amount: 1000,
      amount_received: 2000,
      change_given: 1000,
      payment_method: 'cash',
    },
  ],
};

afterEach(() => vi.unstubAllGlobals());

describe('identité client dans les commandes', () => {
  it('envoie customer_id et conserve l’identifiant canonique de la réponse', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify(response), { status: 201 }));
    vi.stubGlobal('fetch', fetchMock);
    const result = await createOrderOnBackend(draft, 'token');
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toMatchObject({ customer_id: 'client-b' });
    expect(result.order?.customerId).toBe('client-b');
  });

  it('conserve customer_id au rechargement de la liste', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify([response]))));
    const result = await fetchOrders('token');
    expect(result.orders[0].customerId).toBe('client-b');
  });

  it('transmet somme remise et récupère la monnaie rendue calculée par le serveur', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify(response), { status: 201 }));
    vi.stubGlobal('fetch', fetchMock);
    const result = await createOrderOnBackend(draft, 'token', 'idemp-key-1');
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toMatchObject({
      amount_received: 2000,
      initial_paid: 1000,
    });
    expect(fetchMock.mock.calls[0][1].headers['Idempotency-Key']).toBe('idemp-key-1');
    expect(result.order?.changeGiven).toBe(1000);
    expect(result.order?.amountReceived).toBe(2000);
  });

  it('propage le refus de choix ambigu sans fabriquer une réponse persistée', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(
          JSON.stringify({ error: 'Sélectionnez un client', code: 'customer_selection_required' }),
          {
            status: 409,
          }
        )
      )
    );
    expect(await createOrderOnBackend(draft, 'token')).toEqual({
      order: null,
      synced: false,
      error: 'Sélectionnez un client',
    });
  });
});
