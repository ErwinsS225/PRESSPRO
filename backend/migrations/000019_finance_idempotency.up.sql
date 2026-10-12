-- Migration 000019 : monnaie rendue, traçabilité des encaissements et idempotence
-- Conserve scrupuleusement les colonnes et paiements historiques existants.

BEGIN;

ALTER TABLE payments
    ADD COLUMN IF NOT EXISTS amount_received NUMERIC(12, 2) CHECK (amount_received IS NULL OR amount_received >= amount),
    ADD COLUMN IF NOT EXISTS change_given NUMERIC(12, 2) CHECK (change_given IS NULL OR change_given >= 0.00);

-- Table des mutations pour l'idempotence des dépôts et règlements (tenant-safe)
CREATE TABLE IF NOT EXISTS order_mutations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    operation VARCHAR(50) NOT NULL,
    mutation_key VARCHAR(128) NOT NULL,
    request_hash VARCHAR(64) NOT NULL,
    response JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_tenant_operation_key UNIQUE (tenant_id, operation, mutation_key)
);

CREATE INDEX IF NOT EXISTS idx_order_mutations_lookup
    ON order_mutations (tenant_id, operation, mutation_key);

INSERT INTO schema_migrations (version) VALUES ('000019');

COMMIT;
