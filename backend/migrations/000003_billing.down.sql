-- ============================================================================
-- PressingOS — Migration 000003: Abonnements & Entitlements (DOWN)
-- ============================================================================
DROP TABLE IF EXISTS subscription_events CASCADE;
DROP TABLE IF EXISTS subscriptions CASCADE;
DROP TABLE IF EXISTS plan_entitlements CASCADE;
DROP TABLE IF EXISTS features CASCADE;
DROP TABLE IF EXISTS plans CASCADE;
