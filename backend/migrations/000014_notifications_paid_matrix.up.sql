-- ==============================================================================
-- PressingOS — Migration 000014: Envoi réel des notifications = formule payante
-- ==============================================================================
-- Le journal et la composition des messages restent ouverts à tous, mais
-- l'envoi réel par la passerelle (WhatsApp Business API / agrégateur SMS) est
-- réservé aux formules PRO et BUSINESS — cohérent avec « reports.advanced » et
-- « expenses.advanced ». L'essai actif vaut BUSINESS.

INSERT INTO features (key, name, description, type)
VALUES (
    'notifications.advanced',
    'Notifications clients (WhatsApp / SMS)',
    'Envoi réel des messages de suivi (commande prête, livrée, paiement reçu, rappel de retrait).',
    'boolean'
)
ON CONFLICT (key) DO NOTHING;

-- PRO et BUSINESS : envoi réel activé.
INSERT INTO plan_entitlements (plan_id, feature_id, is_enabled, limit_value)
SELECT p.id, f.id, true, NULL
FROM plans p, features f
WHERE f.key = 'notifications.advanced' AND p.code IN ('PRO', 'BUSINESS')
ON CONFLICT (plan_id, feature_id) DO UPDATE SET is_enabled = true;

-- Formule gratuite : composition et journal conservés, envoi réel désactivé.
INSERT INTO plan_entitlements (plan_id, feature_id, is_enabled, limit_value)
SELECT p.id, f.id, false, NULL
FROM plans p, features f
WHERE f.key = 'notifications.advanced' AND p.code = 'FREE'
ON CONFLICT (plan_id, feature_id) DO UPDATE SET is_enabled = false;
