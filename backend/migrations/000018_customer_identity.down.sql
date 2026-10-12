-- Aucun dédoublonnage destructif : refuser le retour arrière si l'unicité
-- historique est devenue impossible. À exécuter en fenêtre de maintenance.
BEGIN;
LOCK TABLE customers IN ACCESS EXCLUSIVE MODE;
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM customers GROUP BY tenant_id, phone HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'Retour arrière 000018 impossible : des clients partagent un téléphone. Aucune fiche supprimée.';
    END IF;
END $$;

ALTER TABLE orders DROP CONSTRAINT orders_tenant_customer_fkey;
ALTER TABLE customers DROP CONSTRAINT uq_customers_tenant_id;
ALTER TABLE customers ADD CONSTRAINT uq_tenant_customer_phone UNIQUE (tenant_id, phone);
DROP INDEX IF EXISTS idx_customers_tenant_phone;
DELETE FROM schema_migrations WHERE version = '000018';
COMMIT;
