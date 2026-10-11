-- ==============================================================================
-- PressingOS — Migration 000002: Catalogue enrichi (DOWN)
-- ==============================================================================
ALTER TABLE services
    DROP COLUMN IF EXISTS description,
    DROP COLUMN IF EXISTS requires_quote,
    DROP COLUMN IF EXISTS is_outsourced,
    DROP COLUMN IF EXISTS is_pickup_available,
    DROP COLUMN IF EXISTS express_surcharge,
    DROP COLUMN IF EXISTS is_express_available,
    DROP COLUMN IF EXISTS pricing_model,
    DROP COLUMN IF EXISTS treatment,
    DROP COLUMN IF EXISTS article;
