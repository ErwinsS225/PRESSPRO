-- ==============================================================================
-- PressingOS — Migration 000014: Envoi réel des notifications (DOWN)
-- ==============================================================================
DELETE FROM plan_entitlements pe
USING features f
WHERE pe.feature_id = f.id AND f.key = 'notifications.advanced';

DELETE FROM features WHERE key = 'notifications.advanced';
