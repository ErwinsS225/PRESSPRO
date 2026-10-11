-- Rétablit la matrice initiale (module accessible à toutes les formules).
UPDATE plan_entitlements pe
SET is_enabled = true
FROM plans p, features f
WHERE pe.plan_id = p.id
  AND pe.feature_id = f.id
  AND f.key = 'expenses.advanced'
  AND p.code = 'FREE';
