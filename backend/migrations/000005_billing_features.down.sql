-- DOWN registre features : conserve celles référencées par des entitlements.
DELETE FROM features
WHERE key IN ('orders.create', 'orders.monthly.max', 'users.max', 'catalog.price_lists',
              'branches.multi_site', 'reports.advanced', 'expenses.advanced',
              'pricing.pro', 'reports.consolidated', 'audit.advanced')
  AND NOT EXISTS (SELECT 1 FROM plan_entitlements WHERE plan_entitlements.feature_id = features.id);
