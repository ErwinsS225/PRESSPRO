-- Annulation 000019 : suppression de la table des mutations et des colonnes de monnaie.
-- Refuse si des colonnes de monnaie sont renseignées, afin d'éviter toute perte de traçabilité.

BEGIN;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM payments
        WHERE amount_received IS NOT NULL OR change_given IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'Retour arrière 000019 impossible : des règlements avec traçabilité de monnaie existent.';
    END IF;
END $$;

DROP TABLE IF EXISTS order_mutations;
ALTER TABLE payments DROP COLUMN IF EXISTS change_given;
ALTER TABLE payments DROP COLUMN IF EXISTS amount_received;

DELETE FROM schema_migrations WHERE version = '000019';

COMMIT;
