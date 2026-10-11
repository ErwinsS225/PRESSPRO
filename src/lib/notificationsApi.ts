/**
 * Module Notifications clients — messages WhatsApp / SMS adressés au client.
 *
 * La composition et le journal restent accessibles à tous les pressings ;
 * l'envoi réel par la passerelle est réservé aux formules payantes
 * (« notifications.advanced », PRO et BUSINESS — l'essai actif les inclut).
 * Lorsqu'aucune passerelle n'est configurée, le message est journalisé avec le
 * statut « simule » : l'écran propose alors de l'ouvrir dans WhatsApp ou de le
 * copier pour l'envoyer depuis le comptoir.
 */

export type NotificationChannel = 'whatsapp' | 'sms';
export type NotificationStatus = 'simule' | 'envoye' | 'echec';

export interface ClientNotification {
  id: string;
  tenant_id: string;
  order_id?: string;
  channel: string;
  template?: string;
  recipient: string;
  body: string;
  status: string;
  provider?: string;
  detail?: string;
  attempts: number;
  created_by?: string;
  created_at: string;
  sent_at?: string;
}

export interface NotificationTemplate {
  value: string;
  label: string;
}

/** Modèles proposés au comptoir (alignés sur le backend). */
export const NOTIFICATION_TEMPLATES: NotificationTemplate[] = [
  { value: 'commande_prete', label: 'Commande prête' },
  { value: 'commande_livree', label: 'Commande livrée' },
  { value: 'paiement_recu', label: 'Paiement reçu' },
  { value: 'rappel_retrait', label: 'Rappel de retrait' },
];

export const NOTIFICATION_CHANNELS: { value: NotificationChannel; label: string }[] = [
  { value: 'whatsapp', label: 'WhatsApp' },
  { value: 'sms', label: 'SMS' },
];

export function channelLabel(channel: string): string {
  return NOTIFICATION_CHANNELS.find((c) => c.value === channel)?.label ?? channel;
}

export function templateLabel(template?: string): string {
  if (!template) return 'Message libre';
  return NOTIFICATION_TEMPLATES.find((t) => t.value === template)?.label ?? template;
}

export function statusLabel(status: string): string {
  switch (status) {
    case 'simule':
      return 'Journalisé (sans envoi)';
    case 'envoye':
      return 'Envoyé';
    case 'echec':
      return 'Échec';
    default:
      return status;
  }
}

/** Numéro exploitable par les liens « wa.me » et « sms: » (chiffres seuls). */
export function phoneDigits(phone: string): string {
  return (phone ?? '').replace(/[^0-9]/g, '');
}

/**
 * Lien d'ouverture directe du message dans WhatsApp — repli gratuit lorsque
 * l'envoi serveur n'est pas disponible (formule gratuite ou passerelle absente).
 */
export function whatsappLink(phone: string, message: string): string {
  const numero = phoneDigits(phone);
  const base = numero ? `https://wa.me/${numero}` : 'https://wa.me/';
  return `${base}?text=${encodeURIComponent(message)}`;
}

/** Lien SMS natif (mobile / tablette). */
export function smsLink(phone: string, message: string): string {
  const numero = phoneDigits(phone);
  return `sms:${numero ? `+${numero}` : ''}?&body=${encodeURIComponent(message)}`;
}

function authHeaders(token?: string): Record<string, string> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    Accept: 'application/json',
  };
  if (token) headers['Authorization'] = `Bearer ${token}`;
  return headers;
}

export interface SendNotificationInput {
  order_id: string;
  channel: NotificationChannel;
  template?: string;
  message?: string;
  recipient?: string;
}

export interface SendNotificationResult {
  ok: boolean;
  notification?: ClientNotification;
  error?: string;
  planLocked?: boolean;
}

/** Compose (et envoie si la passerelle le permet) un message au client. */
export async function sendClientNotification(
  input: SendNotificationInput,
  token?: string
): Promise<SendNotificationResult> {
  try {
    const res = await fetch('/api/v1/notifications', {
      method: 'POST',
      headers: authHeaders(token),
      body: JSON.stringify(input),
    });
    const body = (await res.json().catch(() => ({}))) as {
      notification?: ClientNotification;
      error?: string;
    };
    if (res.ok) return { ok: true, notification: body.notification };
    return {
      ok: false,
      error: body.error ?? `erreur HTTP ${res.status}`,
      planLocked: res.status === 402,
    };
  } catch (err) {
    return { ok: false, error: err instanceof Error ? err.message : 'réseau indisponible' };
  }
}

/** Journal du pressing, filtrable par commande (le plus récent d'abord). */
export async function fetchNotifications(
  options: { orderId?: string; limit?: number } = {},
  token?: string
): Promise<{ ok: boolean; notifications?: ClientNotification[]; error?: string }> {
  const params = new URLSearchParams();
  if (options.orderId) params.set('order_id', options.orderId);
  if (options.limit) params.set('limit', String(options.limit));
  const requete = params.toString();
  try {
    const res = await fetch(`/api/v1/notifications${requete ? `?${requete}` : ''}`, {
      headers: authHeaders(token),
    });
    const body = (await res.json().catch(() => ({}))) as {
      notifications?: ClientNotification[];
      error?: string;
    };
    if (!res.ok) return { ok: false, error: body.error ?? `erreur HTTP ${res.status}` };
    return { ok: true, notifications: body.notifications ?? [] };
  } catch (err) {
    return { ok: false, error: err instanceof Error ? err.message : 'réseau indisponible' };
  }
}
