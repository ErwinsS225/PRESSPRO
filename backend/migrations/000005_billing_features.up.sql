-- Seed registre des fonctionnalités (matrice spec MAJ.md section 2)
INSERT INTO features (key, name, description, type) VALUES
    ('orders.create', 'Création et suivi des commandes', 'Créer et suivre les dépôts de linge', 'boolean'),
    ('orders.monthly.max', 'Plafond de commandes par mois', 'Commandes créées par mois civil', 'limit'),
    ('users.max', 'Nombre de comptes utilisateurs', 'Comptes employés du pressing', 'limit'),
    ('catalog.price_lists', 'Grilles tarifaires', 'Grilles tarifaires configurables', 'limit'),
    ('branches.multi_site', 'Multi-établissements', 'Établissements rattachés', 'limit'),
    ('reports.advanced', 'Rapports détaillés', 'Rapports avancés et exports', 'boolean'),
    ('expenses.advanced', 'Dépenses avancées', 'Gestion avancée des dépenses', 'boolean'),
    ('pricing.pro', 'Tarification professionnelle', 'Tarification pro / facturation avancée', 'tier'),
    ('reports.consolidated', 'Rapports consolidés multi-sites', 'Consolidation inter-établissements', 'boolean'),
    ('audit.advanced', 'Journal audit avancé', 'Traçabilité avancée des actions', 'boolean')
ON CONFLICT (key) DO NOTHING;
