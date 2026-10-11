-- DOWN matrice : supprime les entitlements générés + l'abonnement démo.
DELETE FROM subscription_events
WHERE tenant_id = 'a0000000-0000-0000-0000-000000000001' AND reason = 'trial_started';
DELETE FROM subscriptions WHERE tenant_id = 'a0000000-0000-0000-0000-000000000001';
DELETE FROM plan_entitlements;
