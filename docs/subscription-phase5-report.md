# PRESSPRO — Rapport de la phase 5 (API et frontend reflétant les droits effectifs)

Suite de `docs/subscription-phase4-report.md`. Commit : `c131e1a` (poussé sur `arena/a06916de-presspro`).
Rapport rédigé en phase 8 à partir de l'état du dépôt ; les décisions sont reprises telles qu'elles
ont été prises en phase 5 et confirmées par le propriétaire.

---

## Phase 5 — API et frontend reflétant les droits effectifs

**État :** terminé dans le périmètre de la phase 5 (code, tests, CI verte sur `c131e1a`, run
`38212469238`). Aucune migration, aucun tarif modifié, aucun paiement ni webhook simulé.

**Constat :**
- Pendant l'essai, `plan_code` désigne la formule **effective** (BUSINESS), et non la formule
  réellement **souscrite** (FREE). Sans second champ, l'interface ne pouvait pas distinguer
  les deux.
- Les contrôles de plafond en mode mémoire ne s'appliquaient pas (R-P4-3) et la vérification
  d'appartenance d'un compte cible pour les affectations de sites était ouverte en mode mémoire
  (R-P4-2). Le propriétaire a demandé leur correction en phase 5.

**Changements :**
- `GET /api/v1/me/entitlements` : `plan_code` (effective), `subscribed_plan_code` (souscrite),
  `plan_name`, `status`, `on_trial`, `trial_ends_at`, `days_left_trial`, `entitlements[]`
  (`key`, `enabled`, `limit`, `usage`, `remaining`, `source`), `price_monthly_xof` (formule
  souscrite, lu en base), `orders_period` (début inclus, fin exclue).
- `GET /api/v1/billing/plans` : grille `{plans: [{code, name, price_monthly_xof, entitlements}]}`.
- Fenêtre de commandes : quota glissant de 30 jours, ancré sur `trial_started_at`, sinon
  `created_at` de l'abonnement, sinon `created_at` de l'établissement (SQL et Go alignés).
- R-P4-2 corrigé : en mode mémoire, la vérification d'appartenance du compte cible est
  fail-closed avant toute écriture d'affectation (`branch/assignment.go`).
- R-P4-3 corrigé : compteurs mémoire des plafonds (commandes, comptes, grilles, établissements)
  (`billing/comptage_memoire.go`).
- Frontend : `BillingView.tsx` (écran d'abonnement réécrit pour afficher les droits effectifs), `billingApi.ts`,
  `TeamView.tsx`, `TicketVerificationView.tsx`, `App.tsx`. Aucun prix codé en dur dans les
  composants : les tarifs viennent de `/billing/plans` et de `/me/entitlements`.

**Fichiers modifiés (36) :** `backend/cmd/api/main.go`, `backend/internal/auth/{handler,service}.go`,
`backend/internal/billing/{entitlements,handler,model,persist,quota,service,store_contract,testing_hook}.go`,
`backend/internal/billing/comptage_memoire.go` (nouveau), `backend/internal/branch/{assignment,service}.go`,
`backend/internal/http/{router,wiring_memoire}.go`, `backend/internal/order/{service,verify}.go`,
`backend/internal/pricelist/service.go`, tests associés, `src/App.tsx`, `src/components/BillingView.tsx`,
`src/components/TeamView.tsx`, `src/components/TicketVerificationView.tsx`, `src/lib/billingApi.ts`,
tests Vitest, `.env.example`.

**Tests (au moment de la phase 5) :**
- Intégration PostgreSQL : 209 PASS, 0 FAIL, 1 SKIP (`TestSuspension_GeleLesModulesMetier/mémoire`,
  attendu).
- `go test -race` : `internal/http`, `billing`, `order`, `branch` : OK.
- Frontend : `npm run lint` (tsc) OK ; ESLint 0 erreur, 48 avertissements (budget 50) ;
  Prettier OK ; `npm test` 195 tests OK ; `npm run build` OK.

**Sécurité et multitenant :** les droits sont décidés par le backend ; le frontend ne fait
qu'afficher. Les identifiants d'un autre établissement restent refusés (404). En mode mémoire,
l'affectation d'un compte étranger est refusée.

**Décisions :**
- `subscribed_plan_code` ajouté : la formule souscrite n'est plus déduite de `plan_code`.
- R-P4-2 et R-P4-3 corrigés en phase 5 (décision du propriétaire, option A).
- R-P4-1 (clients non persistés en mémoire) reste une limite documentée, sans correctif.

**GitHub :** commit `c131e1a`, poussé. CI `38212469238` : succès.

**Phase suivante :** phase 6 (diagnostic de persistance des prix, sans changement de prix).
