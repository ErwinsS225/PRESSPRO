-- ============================================================================
-- PressingOS — Migration 000017 : journal des prix des formules (DOWN)
-- Rétablit les prix de départ (PRO 45 000, BUSINESS 75 000 XOF) uniquement si
-- les valeurs corrigées sont encore en place, puis retire le journal.
-- ============================================================================

DO $$
BEGIN
    PERFORM set_config('presspro.actor_user_id', '', false);
    PERFORM set_config('presspro.price_reason', 'Retour aux tarifs de départ (annulation de la migration 000017)', false);
END $$;
UPDATE plans SET price_monthly_xof = 45000 WHERE code = 'PRO' AND price_monthly_xof = 10000;
UPDATE plans SET price_monthly_xof = 75000 WHERE code = 'BUSINESS' AND price_monthly_xof = 25000;
DO $$
BEGIN
    PERFORM set_config('presspro.price_reason', '', false);
END $$;

DROP TRIGGER IF EXISTS trg_plans_journal_prix ON plans;
DROP FUNCTION IF EXISTS journaliser_prix_plan();
DROP TABLE IF EXISTS plan_price_history;
