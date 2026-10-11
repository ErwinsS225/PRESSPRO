-- =============================================================================
-- PressingOS — Migration 000010: Grilles tarifaires versionnées (DOWN)
-- Rollback complet (les tables n'existentent que pour ce modèle).
-- =============================================================================
DROP TABLE IF EXISTS price_list_versions;
DROP TABLE IF EXISTS price_lists;