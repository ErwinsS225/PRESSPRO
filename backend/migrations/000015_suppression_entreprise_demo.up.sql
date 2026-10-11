-- ==============================================================================
-- PressingOS — Migration 000015: Retrait de l'entreprise de démonstration
-- ==============================================================================
-- La migration 000001 livrait une société fictive (« Pressing Ivoire Cocody »,
-- id a0000000-0000-0000-0000-000000000001) et ses 5 prestations, afin de peupler
-- l'interface de démonstration. Le produit ne doit plus démarrer avec des
-- données d'entreprise inventées : chaque établissement naît de sa propre
-- inscription (`POST /api/v1/auth/register-tenant`), qui crée son catalogue, sa
-- grille tarifaire et son essai 7 jours.
--
-- Les migrations restant immuables, le seed initial n'est pas modifié : il est
-- annulé ici. La suppression emporte en cascade les comptes, clients, commandes,
-- lignes de commande, paiements, dépenses, établissements, abonnements et
-- notifications rattachés à cette société. Les seuls comptes de démonstration
-- conservés sont ceux embarqués dans le service d'authentification.

DELETE FROM order_items
WHERE order_id IN (
    SELECT id FROM orders WHERE tenant_id = 'a0000000-0000-0000-0000-000000000001'
);

DELETE FROM payments WHERE tenant_id = 'a0000000-0000-0000-0000-000000000001';
DELETE FROM orders WHERE tenant_id = 'a0000000-0000-0000-0000-000000000001';

-- Le reste des tables enfants référence `tenants` avec ON DELETE CASCADE
-- (users, customers, services, subscriptions, subscription_events, branches,
-- tenant_memberships, branch_memberships, order_sequences, price_lists,
-- price_list_versions, expenses, notifications).
DELETE FROM tenants WHERE id = 'a0000000-0000-0000-0000-000000000001';
