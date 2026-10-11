-- ============================================================================
-- PressingOS — Migration 000007: Fondations Multi-Tenant (DOWN)
-- Rollback additif : retire les objets ajoutés en 000007, dans un ordre sûr
-- (les dépendances FK sont détachées avant les DROP).
-- ============================================================================

-- 1) Détacher les commandes des branches AVANT toute suppression de branches
ALTER TABLE orders DROP COLUMN IF EXISTS branch_id;

-- 2) Retirer la traçabilité de cette migration
DELETE FROM schema_migrations WHERE version = '000007';

-- 3) Supprimer les tables (ordre : dépendances d'abord)
DROP TABLE IF EXISTS branch_memberships;
DROP TABLE IF EXISTS tenant_memberships;
DROP TABLE IF EXISTS branches;

-- NOTE : schema_migrations est conservée (traçabilité des autres migrations).
-- Pour un rollback total du suivi : DROP TABLE IF EXISTS schema_migrations;
