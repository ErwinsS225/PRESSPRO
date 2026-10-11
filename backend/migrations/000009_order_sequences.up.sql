-- =============================================================================
-- PressingOS — Migration 000009: Séquence de numéros de commande (UP)
-- Table de séquences par tenant pour générer un numéro de ticket ATOMIQUE
-- (CMD-<année>-<séquence>) sans collision sur la contrainte UNIQUE(tenant_id, order_number).
-- L'incrément se fait via INSERT ... ON CONFLICT DO UPDATE ... RETURNING,
-- qui verrouille la ligne du tenant le temps de la transaction.
-- =============================================================================
CREATE TABLE IF NOT EXISTS order_sequences (
    tenant_id UUID PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
    last_seq  INT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Suivi de migration (idempotent)
INSERT INTO schema_migrations (version) VALUES ('000009')
ON CONFLICT (version) DO NOTHING;
