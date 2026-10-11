import { ServiceItem } from '../types';
import { DEMO_SERVICES } from './demoData';

const STORAGE_KEY = 'pressingos_catalog_services_v2';
const LAST_SYNC_KEY = 'pressingos_catalog_last_sync';

// Récupérer le token d'authentification valide
export function getAuthHeader(token?: string): Record<string, string> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    'Accept': 'application/json',
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }
  return headers;
}

// Obtenir la date de dernière synchronisation formatée
export function getLastSyncTime(): string {
  try {
    const raw = localStorage.getItem(LAST_SYNC_KEY);
    if (!raw) return 'Synchronisé en continu';
    return new Date(raw).toLocaleTimeString('fr-FR', {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
  } catch {
    return 'Récemment';
  }
}

export function saveLastSyncTime() {
  try {
    localStorage.setItem(LAST_SYNC_KEY, new Date().toISOString());
  } catch {
    // ignore
  }
}

// Sauvegarde locale de secours
export function saveLocalServices(services: ServiceItem[]) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(services));
  } catch {
    // ignore
  }
}

// Chargement local
export function getLocalServices(): ServiceItem[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed) && parsed.length > 0) {
        return parsed;
      }
    }
  } catch {
    // ignore
  }
  return DEMO_SERVICES;
}

/**
 * Récupère la liste des prestations depuis l'API backend Go (ou cache local sécurisé)
 */
export async function fetchServices(token?: string): Promise<{ services: ServiceItem[]; fromBackend: boolean; error?: string }> {
  try {
    const res = await fetch('/api/v1/services', {
      method: 'GET',
      headers: getAuthHeader(token),
    });

    if (res.ok) {
      const data = await res.json();
      if (Array.isArray(data) && data.length > 0) {
        const mapped: ServiceItem[] = data.map((d: any) => ({
          id: d.id,
          name: d.name,
          category: d.category || 'Lavage & Repassage',
          price: Number(d.unit_price) || 0,
          unit: d.unit || 'pièce',
          durationHours: Number(d.estimated_hours) || 24,
          isActive: d.is_active !== false,
          description: d.description || '',
        }));

        saveLocalServices(mapped);
        saveLastSyncTime();
        return { services: mapped, fromBackend: true };
      }
    }
  } catch (err: any) {
    console.warn('[ServicesApi] Impossible de joindre /api/v1/services, bascule cache local:', err.message);
  }

  // Fallback persistant local
  const local = getLocalServices();
  return { services: local, fromBackend: false };
}

/**
 * Crée une nouvelle prestation dans la base de données et dans le cache local
 */
export async function createServiceOnBackend(
  newService: Omit<ServiceItem, 'id'>,
  token?: string
): Promise<{ service: ServiceItem; synced: boolean; error?: string }> {
  const tempId = `srv-${Date.now()}`;
  const localItem: ServiceItem = {
    ...newService,
    id: tempId,
    isActive: newService.isActive !== false,
  };

  try {
    const res = await fetch('/api/v1/services', {
      method: 'POST',
      headers: getAuthHeader(token),
      body: JSON.stringify({
        name: newService.name,
        category: newService.category,
        unit_price: Number(newService.price),
        unit: newService.unit,
        estimated_hours: Number(newService.durationHours),
        is_active: newService.isActive !== false,
      }),
    });

    if (res.ok) {
      const data = await res.json();
      const confirmedItem: ServiceItem = {
        id: data.id || tempId,
        name: data.name || newService.name,
        category: data.category || newService.category,
        price: Number(data.unit_price) || newService.price,
        unit: data.unit || newService.unit,
        durationHours: Number(data.estimated_hours) || newService.durationHours,
        isActive: data.is_active !== false,
        description: newService.description,
      };

      // Mettre à jour le cache local
      const current = getLocalServices();
      const updated = [confirmedItem, ...current.filter((s) => s.id !== confirmedItem.id)];
      saveLocalServices(updated);
      saveLastSyncTime();

      return { service: confirmedItem, synced: true };
    }
  } catch (err: any) {
    console.warn('[ServicesApi] Échec appel POST /api/v1/services:', err.message);
  }

  // Sauvegarde locale si le serveur est indisponible
  const current = getLocalServices();
  const updated = [localItem, ...current];
  saveLocalServices(updated);

  return { service: localItem, synced: false };
}

/**
 * Met à jour une prestation existante dans la base de données
 */
export async function updateServiceOnBackend(
  service: ServiceItem,
  token?: string
): Promise<{ service: ServiceItem; synced: boolean; error?: string }> {
  try {
    const res = await fetch(`/api/v1/services/${service.id}`, {
      method: 'PUT',
      headers: getAuthHeader(token),
      body: JSON.stringify({
        name: service.name,
        category: service.category,
        unit_price: Number(service.price),
        unit: service.unit,
        estimated_hours: Number(service.durationHours),
        is_active: service.isActive !== false,
      }),
    });

    if (res.ok) {
      const data = await res.json();
      const confirmedItem: ServiceItem = {
        ...service,
        name: data.name || service.name,
        category: data.category || service.category,
        price: Number(data.unit_price) || service.price,
        unit: data.unit || service.unit,
        durationHours: Number(data.estimated_hours) || service.durationHours,
        isActive: data.is_active !== false,
      };

      const current = getLocalServices();
      const updated = current.map((s) => (s.id === confirmedItem.id ? confirmedItem : s));
      saveLocalServices(updated);
      saveLastSyncTime();

      return { service: confirmedItem, synced: true };
    }
  } catch (err: any) {
    console.warn('[ServicesApi] Échec appel PUT /api/v1/services:', err.message);
  }

  // Fallback local
  const current = getLocalServices();
  const updated = current.map((s) => (s.id === service.id ? service : s));
  saveLocalServices(updated);

  return { service, synced: false };
}

/**
 * Supprime une prestation de la base de données
 */
export async function deleteServiceOnBackend(
  serviceId: string,
  token?: string
): Promise<{ success: boolean; synced: boolean }> {
  let synced = false;
  try {
    const res = await fetch(`/api/v1/services/${serviceId}`, {
      method: 'DELETE',
      headers: getAuthHeader(token),
    });
    if (res.ok) {
      synced = true;
      saveLastSyncTime();
    }
  } catch (err: any) {
    console.warn('[ServicesApi] Échec appel DELETE /api/v1/services:', err.message);
  }

  // Toujours nettoyer le stockage local
  const current = getLocalServices();
  const updated = current.filter((s) => s.id !== serviceId);
  saveLocalServices(updated);

  return { success: true, synced };
}

/**
 * Ajustement de tarif en masse (ex: +10% pour l'inflation, ou +200 FCFA fixe)
 */
export async function bulkAdjustCategoryPrices(
  category: string,
  percentage: number,
  services: ServiceItem[],
  token?: string
): Promise<{ updatedServices: ServiceItem[]; count: number }> {
  let count = 0;
  const updated = await Promise.all(
    services.map(async (item) => {
      if (category !== 'all' && item.category !== category) {
        return item;
      }
      count++;
      // Calcul du nouveau prix arrondi à la cinquantaine de FCFA la plus proche
      let newPrice = Math.round((item.price * (1 + percentage / 100)) / 50) * 50;
      if (newPrice < 100) newPrice = 100;

      const updatedItem: ServiceItem = { ...item, price: newPrice };
      await updateServiceOnBackend(updatedItem, token);
      return updatedItem;
    })
  );

  saveLocalServices(updated);
  saveLastSyncTime();
  return { updatedServices: updated, count };
}
