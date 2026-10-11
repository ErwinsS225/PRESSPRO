import { Customer } from '../types';
import { getAuthHeader } from './servicesApi';

/**
 * Mapping PostgreSQL/Go -> Frontend.
 * Le backend gère first_name/last_name (contrainte NOT NULL) et full_name dérivé.
 */
function mapBackendCustomer(d: any): Customer {
  return {
    id: d.id,
    fullName: d.full_name || [d.first_name, d.last_name].filter(Boolean).join(' ') || 'Client',
    phone: d.phone || '',
    neighborhood: d.address || '',
    totalOrders: 0, // recalculé côté UI si besoin (agrégation non exposée par l'API)
    lastVisit: d.created_at
      ? new Date(d.created_at).toLocaleDateString('fr-FR', { day: '2-digit', month: '2-digit', year: 'numeric' })
      : '',
    balanceDue: 0,
  };
}

/**
 * Charge la liste des clients du tenant depuis PostgreSQL.
 */
export async function fetchCustomers(token?: string): Promise<{ customers: Customer[]; fromBackend: boolean }> {
  try {
    const res = await fetch('/api/v1/customers', {
      method: 'GET',
      headers: getAuthHeader(token),
    });
    if (res.ok) {
      const data = await res.json();
      if (Array.isArray(data)) {
        return { customers: data.map(mapBackendCustomer), fromBackend: true };
      }
    } else {
      console.warn('[CustomersApi] GET /api/v1/customers -> HTTP', res.status);
    }
  } catch (err: any) {
    console.warn('[CustomersApi] Impossible de joindre /api/v1/customers:', err.message);
  }
  return { customers: [], fromBackend: false };
}

/**
 * Crée un client côté backend.
 * La contrainte UNIQUE(tenant_id, phone) renvoie un 409 sur doublon.
 */
export async function createCustomerOnBackend(
  customer: Customer,
  token?: string
): Promise<{ customer: Customer | null; synced: boolean; error?: string }> {
  // Découpage du nom complet : le backend exige first_name + last_name NOT NULL.
  const parts = (customer.fullName || '').trim().split(/\s+/);
  const firstName = parts[0] || customer.fullName || 'Client';
  const lastName = parts.slice(1).join(' ') || firstName;

  const payload = {
    first_name: firstName,
    last_name: lastName,
    phone: customer.phone,
    address: customer.neighborhood || undefined,
  };

  try {
    const res = await fetch('/api/v1/customers', {
      method: 'POST',
      headers: getAuthHeader(token),
      body: JSON.stringify(payload),
    });
    if (res.ok) {
      const data = await res.json();
      return { customer: mapBackendCustomer(data), synced: true };
    }
    const errData = await res.json().catch(() => ({}));
    return { customer: null, synced: false, error: errData.error || `Erreur serveur HTTP ${res.status}` };
  } catch (err: any) {
    return { customer: null, synced: false, error: err.message || 'Erreur réseau vers /api/v1/customers' };
  }
}