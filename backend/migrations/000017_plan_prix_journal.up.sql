-- ============================================================================
-- PressingOS — Migration 000017 : journal des prix des formules (UP)
-- Phase 6 / 8 : traçabilité des changements de tarif (R-P6-4) et correction
-- tarifaire validée par le propriétaire (PRO 45 000 -> 10 000 ; BUSINESS
-- 75 000 -> 25 000 XOF). Chaque modification de plans.price_monthly_xof est
-- journalisée : ancien et nouveau prix, acteur (si connu), motif, date.
-- Aucun prix d'abonnement n'est copié : les abonnés suivent le catalogue.
-- ============================================================================

CREATE TABLE IF NOT EXISTS plan_price_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    plan_id UUID NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
    plan_code VARCHAR(50) NOT NULL,
    old_price_xof NUMERIC(12, 2),
    new_price_xof NUMERIC(12, 2) NOT NULL CHECK (new_price_xof >= 0),
    changed_by UUID REFERENCES users(id) ON DELETE SET NULL,
    reason TEXT NOT NULL DEFAULT '',
    changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_plan_price_history_plan ON plan_price_history(plan_id, changed_at);

-- Le trigger lit l'acteur et le motif dans des paramètres de session positionnés
-- par l'application (set_config, dans la transaction de la modification). Un
-- acteur absent ou non UUID est journalisé comme NULL.
CREATE OR REPLACE FUNCTION journaliser_prix_plan() RETURNS trigger AS $$
DECLARE
    acteur TEXT := NULLIF(current_setting('presspro.actor_user_id', true), '');
    motif TEXT := COALESCE(NULLIF(current_setting('presspro.price_reason', true), ''), 'changement de prix');
BEGIN
    IF NEW.price_monthly_xof IS DISTINCT FROM OLD.price_monthly_xof THEN
        INSERT INTO plan_price_history (plan_id, plan_code, old_price_xof, new_price_xof, changed_by, reason)
        VALUES (
            NEW.id, NEW.code, OLD.price_monthly_xof, NEW.price_monthly_xof,
            CASE WHEN acteur ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
                 THEN acteur::uuid END,
            motif
        );
        NEW.updated_at := NOW();
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_plans_journal_prix ON plans;
CREATE TRIGGER trg_plans_journal_prix
    BEFORE UPDATE OF price_monthly_xof ON plans
    FOR EACH ROW EXECUTE FUNCTION journaliser_prix_plan();

-- Correction validée par le propriétaire. Gardée : ne s'applique qu'aux valeurs
-- de départ, donc sans effet si la migration est rejouée sur une base corrigée.
DO $$
BEGIN
    PERFORM set_config('presspro.actor_user_id', '', false);
    PERFORM set_config('presspro.price_reason', 'Correction tarifaire validée par le propriétaire (phase 6)', false);
END $$;
UPDATE plans SET price_monthly_xof = 10000 WHERE code = 'PRO' AND price_monthly_xof = 45000;
UPDATE plans SET price_monthly_xof = 25000 WHERE code = 'BUSINESS' AND price_monthly_xof = 75000;
DO $$
BEGIN
    PERFORM set_config('presspro.price_reason', '', false);
END $$;
