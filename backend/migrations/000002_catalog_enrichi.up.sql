-- ==============================================================================
-- PressingOS — Migration 000002: Catalogue enrichi (UP)
-- Aligne la table services sur le modèle Go (article, traitement, modèle
-- tarifaire, options express / collecte / sous-traitance / devis, description).
-- ==============================================================================
ALTER TABLE services
    ADD COLUMN IF NOT EXISTS article VARCHAR(255),
    ADD COLUMN IF NOT EXISTS treatment VARCHAR(255),
    ADD COLUMN IF NOT EXISTS pricing_model VARCHAR(50) NOT NULL DEFAULT 'piece',
    ADD COLUMN IF NOT EXISTS is_express_available BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS express_surcharge NUMERIC(12, 2) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS is_pickup_available BOOLEAN NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS is_outsourced BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS requires_quote BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS description TEXT;
