-- ==============================================================================
-- PressingOS — Migration 000016 : index du quota de commandes
-- ==============================================================================
-- Le quota de commandes (fenêtre glissante de 30 jours) filtre les commandes par
-- établissement et par date de création. Cet index couvre ce filtre. Migration
-- strictement additive : aucune donnée n'est modifiée.
CREATE INDEX IF NOT EXISTS idx_orders_tenant_created ON orders(tenant_id, created_at);
