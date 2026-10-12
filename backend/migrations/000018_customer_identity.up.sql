-- Téléphone obligatoire, mais partagé possible entre personnes distinctes.
-- Déployer avec le backend sans upsert par téléphone (lot clients).
-- Ne modifie ni ne fusionne aucune fiche historique.
BEGIN;

ALTER TABLE customers DROP CONSTRAINT IF EXISTS uq_tenant_customer_phone;
CREATE INDEX IF NOT EXISTS idx_customers_tenant_phone ON customers(tenant_id, phone);

-- Défense en profondeur : une commande ne peut référencer un client étranger.
-- La validation échoue et annule la migration si des incohérences préexistent.
ALTER TABLE customers ADD CONSTRAINT uq_customers_tenant_id UNIQUE (tenant_id, id);
ALTER TABLE orders ADD CONSTRAINT orders_tenant_customer_fkey
    FOREIGN KEY (tenant_id, customer_id) REFERENCES customers(tenant_id, id) ON DELETE RESTRICT
    NOT VALID;
ALTER TABLE orders VALIDATE CONSTRAINT orders_tenant_customer_fkey;

INSERT INTO schema_migrations (version) VALUES ('000018');
COMMIT;
