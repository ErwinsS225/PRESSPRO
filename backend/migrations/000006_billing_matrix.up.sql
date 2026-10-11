-- Matrice des droits (plafonds de départ ajustables après pilotes)
WITH p AS (SELECT id, code FROM plans),
     f AS (SELECT id, key FROM features)
INSERT INTO plan_entitlements (plan_id, feature_id, is_enabled, limit_value)
SELECT p.id, f.id,
    CASE
        WHEN f.key = 'reports.advanced' AND p.code = 'FREE' THEN false
        WHEN f.key = 'reports.consolidated' AND p.code IN ('FREE', 'PRO') THEN false
        ELSE true
    END,
    CASE
        WHEN f.key = 'orders.monthly.max' AND p.code = 'FREE' THEN 50
        WHEN f.key = 'users.max' AND p.code = 'FREE' THEN 2
        WHEN f.key = 'users.max' AND p.code = 'PRO' THEN 5
        WHEN f.key = 'users.max' AND p.code = 'BUSINESS' THEN 15
        WHEN f.key = 'catalog.price_lists' AND p.code = 'FREE' THEN 1
        WHEN f.key = 'catalog.price_lists' AND p.code = 'PRO' THEN 3
        WHEN f.key = 'catalog.price_lists' AND p.code = 'BUSINESS' THEN 10
        WHEN f.key = 'branches.multi_site' AND p.code IN ('FREE', 'PRO') THEN 1
        WHEN f.key = 'branches.multi_site' AND p.code = 'BUSINESS' THEN 3
        ELSE NULL
    END
FROM p CROSS JOIN f
ON CONFLICT (plan_id, feature_id) DO NOTHING;

-- Tenant démo : essai gratuit 7 jours (tout débloqué niveau BUSINESS).
WITH free_plan AS (SELECT id FROM plans WHERE code = 'FREE')
INSERT INTO subscriptions (tenant_id, plan_id, status, trial_started_at, trial_ends_at)
SELECT 'a0000000-0000-0000-0000-000000000001', free_plan.id, 'TRIALING', NOW(), NOW() + INTERVAL '7 days'
FROM free_plan
ON CONFLICT (tenant_id) DO NOTHING;
