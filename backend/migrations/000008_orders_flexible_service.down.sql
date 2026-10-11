-- =============================================================================
-- PressingOS — Migration 000008: Commandes — service_id flexible (DOWN)
-- Rétablit service_id NOT NULL + ON DELETE RESTRICT.
-- ⚠️ Ne rejoue que si aucune ligne n'a service_id IS NULL (sinon échec assumé).
-- =============================================================================

DELETE FROM order_items WHERE service_id IS NULL;

ALTER TABLE order_items
    DROP CONSTRAINT IF EXISTS order_items_service_id_fkey;

ALTER TABLE order_items
    ADD CONSTRAINT order_items_service_id_fkey
    FOREIGN KEY (service_id) REFERENCES services(id) ON DELETE RESTRICT;

ALTER TABLE order_items
    ALTER COLUMN service_id SET NOT NULL;
