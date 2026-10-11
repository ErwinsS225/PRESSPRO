-- DOWN seed plans : ne supprime que les plans sans abonnement rattaché.
DELETE FROM plans WHERE code IN ('FREE', 'PRO', 'BUSINESS')
  AND NOT EXISTS (SELECT 1 FROM subscriptions WHERE subscriptions.plan_id = plans.id);
