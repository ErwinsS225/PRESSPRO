-- =============================================================================
-- PressingOS — Migration 000008: Commandes — service_id flexible (UP)
-- Rend order_items.service_id nullable + ON DELETE SET NULL pour accepter les
-- dépôts legacy/démo (IDs non-UUID type "srv-1") sans casser l'historique.
-- Le détail commercial reste figé via item_label + unit_price (historique figé).
-- =============================================================================

-- 1) Relâcher la contrainte NOT NULL (opération additive, non destructrice).
ALTER TABLE order_items
    ALTER COLUMN service_id DROP NOT NULL;

-- 2) Remplacer le FK ON DELETE RESTRICT par ON DELETE SET NULL (robustesse :
--    supprimer une prestation du catalogue ne casse pas l'historique des dépôts).
DO $$
DECLARE
    con text;
BEGIN
    SELECT tc.constraint_name INTO con
    FROM information_schema.table_constraints tc
    JOIN information_schema.key_column_usage kcu
      ON tc.constraint_name = kcu.constraint_name
    WHERE tc.table_name = 'order_items'
      AND tc.constraint_type = 'FOREIGN KEY'
      AND kcu.column_name = 'service_id'
    LIMIT 1;

    IF con IS NOT NULL THEN
        EXECUTE format('ALTER TABLE order_items DROP CONSTRAINT %I', con);
    END IF;
END $$;

ALTER TABLE order_items
    ADD CONSTRAINT order_items_service_id_fkey
    FOREIGN KEY (service_id) REFERENCES services(id) ON DELETE SET NULL;

-- Suivi de migration (idempotent)
INSERT INTO schema_migrations (version) VALUES ('000008')
ON CONFLICT (version) DO NOTHING;
