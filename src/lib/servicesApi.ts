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
          article: d.article || d.name,
          treatment: d.treatment || 'Lavage & Soin',
          category: d.category || 'Vêtements du quotidien',
          price: Number(d.unit_price) || 0,
          unit: d.unit || 'pièce',
          pricingModel: d.pricing_model || 'piece',
          durationHours: Number(d.estimated_hours) || 24,
          isActive: d.is_active !== false,
          isExpressAvailable: d.is_express_available === true,
          expressSurcharge: Number(d.express_surcharge) || 0,
          isPickupAvailable: d.is_pickup_available !== false,
          isOutsourced: d.is_outsourced === true,
          requiresQuote: d.requires_quote === true,
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
        article: newService.article || newService.name,
        treatment: newService.treatment || 'Lavage & Soin',
        category: newService.category,
        unit_price: Number(newService.price),
        unit: newService.unit,
        pricing_model: newService.pricingModel || 'piece',
        estimated_hours: Number(newService.durationHours),
        is_active: newService.isActive !== false,
        is_express_available: newService.isExpressAvailable === true,
        express_surcharge: Number(newService.expressSurcharge) || 0,
        is_pickup_available: newService.isPickupAvailable !== false,
        is_outsourced: newService.isOutsourced === true,
        requires_quote: newService.requiresQuote === true,
        description: newService.description,
      }),
    });

    if (res.ok) {
      const data = await res.json();
      const confirmedItem: ServiceItem = {
        id: data.id || tempId,
        name: data.name || newService.name,
        article: data.article || newService.article || newService.name,
        treatment: data.treatment || newService.treatment || 'Lavage & Soin',
        category: data.category || newService.category,
        price: Number(data.unit_price) || newService.price,
        unit: data.unit || newService.unit,
        pricingModel: data.pricing_model || newService.pricingModel || 'piece',
        durationHours: Number(data.estimated_hours) || newService.durationHours,
        isActive: data.is_active !== false,
        isExpressAvailable: data.is_express_available === true,
        expressSurcharge: Number(data.express_surcharge) || 0,
        isPickupAvailable: data.is_pickup_available !== false,
        isOutsourced: data.is_outsourced === true,
        requiresQuote: data.requires_quote === true,
        description: data.description || newService.description,
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
        article: service.article,
        treatment: service.treatment,
        category: service.category,
        unit_price: Number(service.price),
        unit: service.unit,
        pricing_model: service.pricingModel || 'piece',
        estimated_hours: Number(service.durationHours),
        is_active: service.isActive !== false,
        is_express_available: service.isExpressAvailable === true,
        express_surcharge: Number(service.expressSurcharge) || 0,
        is_pickup_available: service.isPickupAvailable !== false,
        is_outsourced: service.isOutsourced === true,
        requires_quote: service.requiresQuote === true,
        description: service.description,
      }),
    });

    if (res.ok) {
      const data = await res.json();
      const confirmedItem: ServiceItem = {
        ...service,
        name: data.name || service.name,
        article: data.article || service.article,
        treatment: data.treatment || service.treatment,
        category: data.category || service.category,
        price: Number(data.unit_price) || service.price,
        unit: data.unit || service.unit,
        pricingModel: data.pricing_model || service.pricingModel,
        durationHours: Number(data.estimated_hours) || service.durationHours,
        isActive: data.is_active !== false,
        isExpressAvailable: data.is_express_available === true,
        expressSurcharge: Number(data.express_surcharge) || 0,
        isPickupAvailable: data.is_pickup_available !== false,
        isOutsourced: data.is_outsourced === true,
        requiresQuote: data.requires_quote === true,
        description: data.description || service.description,
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
 * Supprime une prestation de la base de données via requête DELETE vers l'API Go
 */
export async function deleteServiceOnBackend(
  serviceId: string,
  token?: string
): Promise<{ success: boolean; synced: boolean; error?: string }> {
  let synced = false;
  let apiError: string | undefined = undefined;

  try {
    const res = await fetch(`/api/v1/services/${serviceId}`, {
      method: 'DELETE',
      headers: getAuthHeader(token),
    });

    if (res.ok) {
      synced = true;
      saveLastSyncTime();
    } else {
      const errorData = await res.json().catch(() => ({}));
      if (res.status === 404) {
        // Prestation absente de PostgreSQL (ex: service local non encore synchronisé ou déjà supprimé)
        console.warn(`[ServicesApi] Prestation ${serviceId} introuvable sur le serveur (404), suppression locale.`);
        synced = false;
      } else {
        apiError = errorData.error || `Erreur serveur HTTP ${res.status}`;
      }
    }
  } catch (err: any) {
    console.warn('[ServicesApi] Échec appel DELETE /api/v1/services:', err.message);
    apiError = err.message || 'Erreur réseau de communication avec l\'API Go';
  }

  if (apiError) {
    return { success: false, synced: false, error: apiError };
  }

  // Nettoyer le stockage local en cas de succès ou de ressource non trouvée sur le backend
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
