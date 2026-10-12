-- Migration 000016 (retour arrière) : suppression de l'index du quota de commandes.
DROP INDEX IF EXISTS idx_orders_tenant_created;
