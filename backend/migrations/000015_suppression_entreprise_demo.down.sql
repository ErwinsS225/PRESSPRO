-- ==============================================================================
-- PressingOS — Migration 000015: Retrait de l'entreprise de démonstration (DOWN)
-- ==============================================================================
-- Retour arrière : la société fictive est réinsérée telle que la livrait la
-- migration 000001 (sans ses données métier : ni commandes, ni clients, ni
-- paiements). Utile pour rejouer une démonstration sur une base de recette.

INSERT INTO tenants (id, name, slug, phone, email, address, city, currency, status)
VALUES (
    'a0000000-0000-0000-0000-000000000001',
    'Pressing Ivoire Cocody',
    'pressing-ivoire-cocody',
    '+225 07 00 11 22 33',
    'contact@pressing-ivoire.ci',
    'Boulevard Hassan II, Cocody Angré 8ème Tranche',
    'Abidjan',
    'XOF',
    'active'
) ON CONFLICT (id) DO NOTHING;

INSERT INTO services (tenant_id, name, category, unit_price, unit, estimated_hours)
VALUES
    ('a0000000-0000-0000-0000-000000000001', 'Chemise (Lavage + Repassage)', 'Lavage & Repassage', 1000.00, 'pièce', 24),
    ('a0000000-0000-0000-0000-000000000001', 'Pantalon costume', 'Nettoyage à sec', 1500.00, 'pièce', 24),
    ('a0000000-0000-0000-0000-000000000001', 'Costume 2 pièces (Veste + Pantalon)', 'Nettoyage à sec', 3500.00, 'ensemble', 48),
    ('a0000000-0000-0000-0000-000000000001', 'Robe de soirée / Cérémonie', 'Délicat', 4000.00, 'pièce', 48),
    ('a0000000-0000-0000-0000-000000000001', 'Drap 2 places / Couette', 'Linge de maison', 2500.00, 'pièce', 48)
ON CONFLICT DO NOTHING;
