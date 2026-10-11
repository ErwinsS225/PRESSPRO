import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  CheckCircle2,
  Clock,
  Copy,
  Loader2,
  Lock,
  MessageCircle,
  Send,
  Smartphone,
  WifiOff,
  X,
} from 'lucide-react';
import { Order, User } from '../types';
import { formatXOF } from '../lib/adminApi';
import {
  NOTIFICATION_CHANNELS,
  NOTIFICATION_TEMPLATES,
  NotificationChannel,
  ClientNotification,
  channelLabel,
  fetchNotifications,
  sendClientNotification,
  smsLink,
  statusLabel,
  templateLabel,
  whatsappLink,
} from '../lib/notificationsApi';

interface ClientNotificationModalProps {
  order: Order;
  currentUser: User;
  token?: string;
  onClose: () => void;
  showToast?: (message: string) => void;
}

/** Aperçu local du message : le serveur reste l'autorité, mais le comptoir
 * doit voir ce qui sera transmis avant de valider. */
function soldeDu(order: Order): number {
  return Math.max(0, (order.totalAmount ?? 0) - (order.paidAmount ?? 0));
}

function apercuMessage(order: Order, template: string, pressing: string): string {
  const prenom = (order.customerName ?? '').trim().split(/\s+/)[0] || 'cher client';
  const reste = soldeDu(order) > 0 ? ` Reste à payer : ${formatXOF(soldeDu(order))}.` : '';
  switch (template) {
    case 'commande_livree':
      return `Bonjour ${prenom}, votre commande n° ${order.orderNumber} a bien été remise. Merci de votre confiance — ${pressing}.`;
    case 'paiement_recu':
      return `Bonjour ${prenom}, nous avons bien reçu un règlement pour la commande n° ${order.orderNumber}.${reste} Merci — ${pressing}.`;
    case 'rappel_retrait':
      return `Rappel : votre linge (commande n° ${order.orderNumber}) vous attend au comptoir de ${pressing}.${reste}`;
    default:
      return `Bonjour ${prenom}, votre linge est prêt au comptoir (${pressing}, commande n° ${order.orderNumber}).${reste} À très vite !`;
  }
}

/**
 * ClientNotificationModal — notifier un client (WhatsApp / SMS) depuis une
 * commande : choix du modèle, aperçu, envoi (formule payante) et historique.
 * En formule gratuite (ou sans passerelle), le message reste journalisé et
 * l'écran propose un envoi manuel via WhatsApp ou la copie du texte.
 */
export const ClientNotificationModal: React.FC<ClientNotificationModalProps> = ({
  order,
  currentUser,
  token,
  onClose,
  showToast,
}) => {
  const [channel, setChannel] = useState<NotificationChannel>('whatsapp');
  const [template, setTemplate] = useState<string>('commande_prete');
  const [message, setMessage] = useState<string>(() =>
    apercuMessage(order, 'commande_prete', 'votre pressing')
  );
  const [messageEdite, setMessageEdite] = useState(false);
  const [envoiEnCours, setEnvoiEnCours] = useState(false);
  const [planLocked, setPlanLocked] = useState(false);
  const [erreur, setErreur] = useState<string | null>(null);
  const [journal, setJournal] = useState<ClientNotification[]>([]);
  const [chargementJournal, setChargementJournal] = useState(true);

  // Chargement du journal : aucune écriture d'état synchrone (le premier rendu
  // part déjà de `chargementJournal = true`), l'appelant gère son propre retour
  // visuel — l'effet ne fait donc que lancer la requête.
  const chargerJournal = useCallback(async () => {
    const result = await fetchNotifications({ orderId: order.id, limit: 20 }, token);
    setChargementJournal(false);
    if (!result.ok) {
      setErreur(result.error ?? 'journal indisponible');
      return;
    }
    setJournal(result.notifications ?? []);
  }, [order.id, token]);

  useEffect(() => {
    void chargerJournal();
  }, [chargerJournal]);

  const changerTemplate = (valeur: string) => {
    setTemplate(valeur);
    setMessageEdite(false);
    setMessage(apercuMessage(order, valeur, 'votre pressing'));
  };

  const apercu = useMemo(
    () => (messageEdite ? message : apercuMessage(order, template, 'votre pressing')),
    [messageEdite, message, order, template]
  );

  const handleEnvoyer = async () => {
    setEnvoiEnCours(true);
    setErreur(null);
    const result = await sendClientNotification(
      {
        order_id: order.id,
        channel,
        template: messageEdite ? undefined : template,
        message: messageEdite ? apercu : undefined,
      },
      token
    );
    setEnvoiEnCours(false);
    if (!result.ok) {
      setPlanLocked(Boolean(result.planLocked));
      setErreur(
        result.planLocked
          ? 'Envoi serveur réservé aux formules PRO et BUSINESS — le message reste disponible ci-dessous.'
          : (result.error ?? 'envoi impossible')
      );
      return;
    }
    const statut = result.notification?.status;
    showToast?.(
      statut === 'envoye'
        ? `Message transmis à ${order.customerName} (${channelLabel(channel)})`
        : statut === 'echec'
          ? 'Passerelle en échec — le message est journalisé'
          : 'Message journalisé (aucune passerelle configurée)'
    );
    await chargerJournal();
  };

  const copier = async () => {
    try {
      await navigator.clipboard.writeText(apercu);
      showToast?.('Message copié');
    } catch {
      showToast?.('Copie impossible — sélectionnez le texte manuellement');
    }
  };

  const libre = currentUser.role !== 'superadmin';

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-neutral-900/60 p-4 backdrop-blur-xs">
      <div className="w-full max-w-2xl rounded-2xl border border-neutral-200 bg-white p-5 shadow-xl dark:border-neutral-800 dark:bg-neutral-900 shadow-2xl shadow-neutral-950/25 dark:shadow-black/80 ring-1 ring-black/5 dark:ring-white/10">
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-2">
            <MessageCircle className="h-5 w-5 text-emerald-500" />
            <div>
              <h2 className="text-sm font-semibold text-neutral-900 dark:text-neutral-100">
                Notifier {order.customerName}
              </h2>
              <p className="mt-0.5 text-[11px] text-neutral-500 dark:text-neutral-400">
                Commande {order.orderNumber} · {order.customerPhone || 'sans téléphone'} · reste à payer{' '}
                {formatXOF(soldeDu(order))}
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg p-1 text-neutral-400 transition-colors hover:bg-neutral-950 hover:text-white dark:hover:text-neutral-200 dark:hover:bg-neutral-100 dark:hover:text-neutral-950"
            aria-label="Fermer"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        {planLocked && (
          <div className="mt-4 rounded-lg border border-amber-200 bg-amber-50 p-3 text-[11px] text-amber-800 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-300">
            <div className="flex items-center gap-1.5 font-medium">
              <Lock className="h-3.5 w-3.5" />
              Envoi automatique réservé aux formules payantes
            </div>
            <p className="mt-1">
              La composition et le journal restent accessibles. Utilisez « Ouvrir dans WhatsApp » pour envoyer
              le message depuis le comptoir, ou passez à une formule supérieure.
            </p>
          </div>
        )}

        <div className="mt-4 grid gap-4 sm:grid-cols-2">
          <div className="space-y-3">
            <div>
              <span className="text-[11px] font-medium text-neutral-600 dark:text-neutral-300">Canal</span>
              <div className="mt-1 inline-flex rounded-lg border border-neutral-200 bg-white p-0.5 dark:border-neutral-800 dark:bg-neutral-900 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated">
                {NOTIFICATION_CHANNELS.map((option) => (
                  <button
                    key={option.value}
                    type="button"
                    onClick={() => setChannel(option.value)}
                    className={`inline-flex items-center gap-1 rounded-md px-2.5 py-1.5 text-[11px] font-medium transition-colors ${
                      channel === option.value
                        ? 'bg-emerald-600 text-white'
                        : 'text-neutral-600 hover:bg-neutral-900/[0.06] dark:text-neutral-300 dark:hover:bg-neutral-800'
                    }`}
                  >
                    {option.value === 'whatsapp' ? (
                      <MessageCircle className="h-3 w-3" />
                    ) : (
                      <Smartphone className="h-3 w-3" />
                    )}
                    {option.label}
                  </button>
                ))}
              </div>
            </div>

            <div>
              <span className="text-[11px] font-medium text-neutral-600 dark:text-neutral-300">Modèle</span>
              <div className="mt-1 flex flex-wrap gap-1.5">
                {NOTIFICATION_TEMPLATES.map((option) => (
                  <button
                    key={option.value}
                    type="button"
                    onClick={() => changerTemplate(option.value)}
                    className={`rounded-full border px-2.5 py-1 text-[11px] font-medium transition-colors ${
                      !messageEdite && template === option.value
                        ? 'border-emerald-300 bg-emerald-50 text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-300'
                        : 'border-neutral-200 text-neutral-600 hover:bg-neutral-900/[0.06] dark:border-neutral-700 dark:text-neutral-300 dark:hover:bg-neutral-800'
                    }`}
                  >
                    {option.label}
                  </button>
                ))}
              </div>
            </div>

            <div>
              <span className="text-[11px] font-medium text-neutral-600 dark:text-neutral-300">
                Message envoyé
              </span>
              <textarea
                aria-label="Message"
                value={apercu}
                onChange={(e) => {
                  setMessage(e.target.value);
                  setMessageEdite(true);
                }}
                rows={6}
                className="mt-1 w-full rounded-lg border border-neutral-200 bg-white p-2 font-mono text-[11px] leading-relaxed text-neutral-800 dark:border-neutral-700 dark:bg-neutral-950 dark:text-neutral-100 hover:border-neutral-950 dark:hover:border-white/70 ps-elevated"
              />
              <p className="mt-1 text-[10px] text-neutral-500 dark:text-neutral-400">
                {apercu.length} caractères · {messageEdite ? 'message personnalisé' : 'modèle appliqué'}
              </p>
            </div>

            <div className="flex flex-wrap items-center gap-2">
              <button
                type="button"
                onClick={() => void handleEnvoyer()}
                disabled={envoiEnCours}
                className="inline-flex items-center gap-1.5 rounded-lg bg-emerald-700 px-3 py-2 text-xs font-medium text-white transition-colors hover:bg-emerald-800 disabled:opacity-50"
              >
                {envoiEnCours ? (
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Send className="h-3.5 w-3.5" />
                )}
                Envoyer
              </button>
              <a
                href={
                  channel === 'sms'
                    ? smsLink(order.customerPhone, apercu)
                    : whatsappLink(order.customerPhone, apercu)
                }
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center gap-1.5 rounded-lg border border-emerald-200 px-3 py-2 text-xs font-medium text-emerald-800 transition-colors hover:bg-emerald-50 dark:border-emerald-900 dark:text-emerald-300 dark:hover:bg-emerald-950/40"
              >
                {channel === 'sms' ? (
                  <Smartphone className="h-3.5 w-3.5" />
                ) : (
                  <MessageCircle className="h-3.5 w-3.5" />
                )}
                {channel === 'sms' ? 'Ouvrir le SMS' : 'Ouvrir dans WhatsApp'}
              </a>
              <button
                type="button"
                onClick={() => void copier()}
                className="inline-flex items-center gap-1.5 rounded-lg border border-neutral-200 px-3 py-2 text-xs font-medium text-neutral-700 transition-colors hover:bg-neutral-900/[0.06] dark:border-neutral-700 dark:text-neutral-200 dark:hover:bg-neutral-800"
              >
                <Copy className="h-3.5 w-3.5" />
                Copier
              </button>
            </div>
          </div>

          <div className="rounded-lg border border-neutral-200 p-3 dark:border-neutral-800">
            <div className="flex items-center justify-between">
              <span className="text-[11px] font-medium text-neutral-600 dark:text-neutral-300">
                Historique de la commande
              </span>
              {chargementJournal && <Loader2 className="h-3 w-3 animate-spin text-neutral-400" />}
            </div>

            {erreur && (
              <div className="mt-2 flex items-start gap-1.5 rounded-lg border border-rose-200 bg-rose-50 px-2 py-1.5 text-[11px] text-rose-700 dark:border-rose-900/50 dark:bg-rose-950/30 dark:text-rose-300">
                <WifiOff className="h-3.5 w-3.5 shrink-0" />
                {erreur}
              </div>
            )}

            <ul className="mt-2 space-y-2">
              {!chargementJournal && journal.length === 0 && (
                <li className="text-[11px] text-neutral-500 dark:text-neutral-400">
                  Aucun message pour cette commande.
                </li>
              )}
              {journal.map((n) => (
                <li
                  key={n.id}
                  className="rounded-lg border border-neutral-100 px-2 py-1.5 text-[11px] dark:border-neutral-800"
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="flex items-center gap-1 font-medium text-neutral-700 dark:text-neutral-200">
                      {n.status === 'envoye' ? (
                        <CheckCircle2 className="h-3 w-3 text-emerald-500" />
                      ) : (
                        <Clock className="h-3 w-3 text-amber-500" />
                      )}
                      {channelLabel(n.channel)} · {templateLabel(n.template)}
                    </span>
                    <span className="text-[10px] text-neutral-400 dark:text-neutral-500">
                      {new Date(n.created_at).toLocaleString('fr-FR', {
                        day: '2-digit',
                        month: '2-digit',
                        hour: '2-digit',
                        minute: '2-digit',
                      })}
                    </span>
                  </div>
                  <p className="mt-1 line-clamp-2 text-neutral-500 dark:text-neutral-400">{n.body}</p>
                  <p className="mt-0.5 text-[10px] text-neutral-400 dark:text-neutral-500">
                    {statusLabel(n.status)}
                    {!libre && n.provider ? ` · ${n.provider}` : ''}
                  </p>
                </li>
              ))}
            </ul>
          </div>
        </div>
      </div>
    </div>
  );
};
