-- Matrice commerciale : le suivi avancé des dépenses (« expenses.advanced »)
-- devient réellement payant — inclus en PRO et BUSINESS, retiré du plan
-- gratuit (cohérent avec « reports.advanced », déjà réservé aux formules
-- payantes). Les établissements en essai conservent le niveau BUSINESS.
UPDATE plan_entitlements pe
SET is_enabled = false
FROM plans p, features f
WHERE pe.plan_id = p.id
  AND pe.feature_id = f.id
  AND f.key = 'expenses.advanced'
  AND p.code = 'FREE';
