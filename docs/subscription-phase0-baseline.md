# PRESSPRO — Phase 0 : vérification initiale et état de référence (abonnements)

Document de suivi de la **PHASE 0** du cahier des charges (`PLAN`). Il ne contient aucune modification
du comportement métier : il consigne l'état du dépôt, les tests de référence exécutés **avant toute
modification**, les composants réellement impliqués et les risques identifiés.

Date des constats : 2026-10-12 (UTC). Commit de référence : `7da3ff8` (`main`, « Update PLAN »).

---

## 1. Phase

**PHASE 0 — Vérification initiale et état de référence.**

## 2. État

**Terminé**, avec les réserves d'environnement décrites à la section 5 (outillage local et substitution
du proxy de modules Go). Aucune phase suivante n'a été commencée.

## 3. Constat — état du dépôt et des PR

| Élément | Constat vérifié |
|---|---|
| Branche de travail | `arena/a06916de-presspro`, créée depuis `main` @ `7da3ff8`, arbre propre au départ |
| Branches distantes | `main` (`7da3ff8`), `feat/subscription-baseline-tests` (`84c0776`), `arena/90615ea9-presspro` (`7af710f`) |
| PR #2 « test(billing): établir les tests de référence des abonnements » | **MERGED** le 2026-10-11 à 23:55:19 UTC, commit de fusion `d6376ac`, ancêtre de `main` (`compare` : la tête de la branche est incluse dans `main`) |
| CI de la PR #2 au moment de la fusion | `Backend Go (tests + PostgreSQL)` SUCCESS, `Frontend React (types + build)` SUCCESS, `SonarCloud Code Analysis` SUCCESS, `CodeRabbit` SUCCESS |
| PR #1 « PressingOS : durcissement P0, monétisation… » (`arena/90615ea9-presspro`) | MERGED (2026-10-11 23:16:28 UTC) |
| PR ouvertes | Aucune |

Le `PLAN` demande de ne pas supposer que la PR #2 est fusionnée ou que ses tests passent : vérifié
sur GitHub, elle est fusionnée et ses checks étaient verts à la fusion. Les tests sont réexécutés
localement ci-dessous.

Le dépôt ne contient ni `AGENTS.md` ni `CONTRIBUTING.md`. Les instructions d'architecture sont dans
`docs/architecture.md`, `docs/api.md`, `docs/database.md`, `docs/correctifs-2026-10.md`.

## 4. Versions et scripts déclarés

| Outil | Déclaré dans le dépôt | Utilisé pour cette Phase 0 |
|---|---|---|
| Go | `backend/go.mod` : `go 1.25` ; CI `go-version: '1.25'` | Go 1.25.4 (linux/amd64) |
| PostgreSQL | `compose.yaml` et CI : `postgres:16-alpine` | PostgreSQL 16 (binaires `16.13.0-beta.17`, voir section 5) |
| Node.js | CI `node-version: '22'` | Node v22.22.3, npm 10.9.8 |
| Backend | `backend/Makefile` : `go test -v -race ./...`, `go vet ./...` | idem + `-cover` comme la CI |
| Frontend | `package.json` : `lint:eslint`, `format:check`, `lint` (= `tsc --noEmit`), `test` (vitest), `build` | idem |

Les migrations `backend/migrations/` comptent **15** paires `up`/`down` (de `000001` à `000015`).
Le fichier `.github/workflows/ci.yml` les applique toutes en ordre sur PostgreSQL 16 avant les tests
d'intégration.

## 5. Environnement de test (réserves)

- **Go** : la sandbox ne peut pas joindre `proxy.golang.org` ni `golang.org` (hôtes non autorisés).
  - Toolchain Go 1.25.4 récupérée depuis le wheel PyPI `go-bin` (installée hors dépôt, dans `/tmp`).
  - Modules : un proxy de modules **local** (`file://`, hors dépôt) a été construit à partir des tags
    GitHub correspondant à chaque version de `go.sum`.
  - `go.mod` et `go.sum` du dépôt **ne sont pas modifiés** : les commandes Go utilisent une copie dans
    `/tmp/gomod/` via `-modfile`.
  - **Réserve** : le hash `h1:` de `github.com/davecgh/go-spew v1.1.1` n'a pas pu être reproduit à
    partir du tag GitHub (dépendance transitive de test, via `testify`). Cette seule ligne a été retirée
    de la copie `/tmp/gomod/go.sum` pour l'exécution locale. Toutes les autres vérifications de hash
    ont réussi. **La CI GitHub, avec le proxy officiel, reste la référence.**
- **PostgreSQL** : la sandbox ne dispose pas de Docker. Un cluster PostgreSQL 16 a été initialisé
  localement à partir des binaires de `@embedded-postgres/linux-x64` (npm), avec le même utilisateur
  (`pressing_app`) et la même base (`pressing_os`) que la CI. `psql` est absent : les 15 migrations
  `up` ont été appliquées en ordre par un script Python (`psycopg`) hors dépôt, sans erreur.

## 6. Résultats des tests AVANT toute modification

| Commande | Résultat |
|---|---|
| `go vet ./...` (backend) | OK (aucune sortie, code 0) |
| `go test -race -cover -count=1 ./...` (backend, tests unitaires) | **OK** — tous les packages testés passent ; `billing` 40.1 %, `auth` 39.5 %, `http` 58.9 %, `order` 21.3 %, `pricelist` 7.5 % de couverture |
| `go test -tags=integration -count=1 -v ./internal/catalog/... ./internal/auth/... ./internal/report/...` (commande exacte de la CI, PostgreSQL réel) | **OK** — `catalog`, `auth`, `report` PASS |
| `go test -tags=integration -count=1 -v ./internal/billing/...` (**non exécuté par la CI**) | **OK** — 14 tests `--- PASS` dont `TestSubscriptionBaseline_TrialIsSevenDaysAndIdempotent`, `TestSubscriptionBaseline_TrialUsesBusinessEntitlements`, `TestSubscriptionBaseline_PlanAndQuotaAreTenantScoped`, `TestTrialGrantsBusinessLevel`, `TestTrialExpiryFallsBackToFreeWithoutPurge`, `TestIntegration_DeleteTenant_RetireToutesLesDonnees` |
| `npm ci --legacy-peer-deps` | OK |
| `npm run lint:eslint` | OK — **0 erreur, 48 avertissements** (seuil `--max-warnings=50`, marge de 2) |
| `npm run format:check` | OK |
| `npm run lint` (`tsc --noEmit`) | OK |
| `npm test` (vitest) | OK — **23 fichiers, 171 tests** passent |
| `npm run build` (vite) | OK |

**Aucun échec préexistant** n'a été observé sur `main` avec cet environnement. Aucun test n'a été
affaibli ni modifié.

## 7. Fichiers et composants réellement impliqués (constat de lecture)

### Backend — moteur d'abonnement
- `backend/internal/billing/service.go` : `OnTrial` (l.138), `EffectivePlan` (l.146) — l'essai est calculé à la lecture (`now < trial_ends_at`), **sans transition persistée** vers FREE.
- `backend/internal/billing/store_contract.go` : `EnsureTrial` (l.31) — chemin mémoire idempotent ; chemin DB délégué à `store.StartTrial`.
- `backend/internal/billing/persist.go` :
  - `dbStore.StartTrial` (l.194) : `INSERT INTO subscriptions … ON CONFLICT (tenant_id) DO NOTHING` **puis** `INSERT INTO subscription_events … 'trial_started'` **sans condition** (voir risque R1).
  - `dbCounters.CountMonthlyOrders` (l.77) : `date_trunc('month', created_at) = date_trunc('month', NOW())` — **mois civil** (voir risque R2).
  - `dbCounters.CountPriceLists` (l.90) et compteurs associés.
- `backend/internal/billing/quota.go` : `QuotaGuard.CheckOrdersCreation` (compteur + `CheckLimit`).
- `backend/internal/billing/entitlements.go` : `EffectiveEntitlements` (l.30), `FeatureUsage` (l.114).
- `backend/internal/billing/handler.go` : handler `Me` (le commentaire mentionne encore `/api/me/entitlements`, la route réelle est `/api/v1/me/entitlements`, voir R7).
- `backend/internal/billing/model.go`, `admin.go`, `testing_hook.go` — non encore relus en détail.
- `backend/internal/http/feature_middleware.go` : `RequireFeature` (l.15), `RequireFeatureQuota` (l.52), `RequireActiveSubscription` (l.109).
- `backend/internal/http/router.go` : groupe `/api/v1` (l.76), route `protected.Get("/me/entitlements", …)` (l.297).
- `backend/internal/order/handler.go` : `SetQuotaGuard` (l.49) — contrôle de quota branché sur la création de commande.
- `backend/internal/auth/persist.go` : transaction d'inscription `register-tenant` (tenant → branche → propriétaire → appartenances → essai).

### Base de données
- `backend/migrations/000003_billing.up.sql` : tables `plans`, `features`, `plan_entitlements`, `subscriptions` (`tenant_id UNIQUE`, `status` ∈ {TRIALING, ACTIVE, PAST_DUE, CANCELED, SUSPENDED}, `trial_started_at`, `trial_ends_at`, `current_period_end`), `subscription_events`.
- `backend/migrations/000004_billing_seed.up.sql` : plans **FREE 0 / PRO 45 000 / BUSINESS 75 000 XOF** (`price_monthly_xof`), conformes au `PLAN` (§3.1).
- `000005` à `000015` : fonctionnalités, matrice d'habilitation, dépenses, notifications, suppression de l'entreprise de démo.

### Frontend (à auditer en phase 5)
- `src/components/BillingView.tsx`, `src/lib/permissions.ts`, `src/types/index.ts`, `src/lib/adminApi.ts`, `src/components/SuperAdminView.tsx`, `src/components/OrdersView.tsx`.

### Tests existants
- Unitaires : `backend/internal/billing/{service_test,entitlements_test,admin_test,subscription_baseline_test}.go`, `backend/internal/http/feature_gate_test.go`, `admin_tenants_test.go`, `backend/internal/order/service_test.go`.
- Intégration PostgreSQL : `backend/internal/billing/persist_integration_test.go` (tag `integration`), et ceux de `auth`, `catalog`, `report`.

### CI
- `.github/workflows/ci.yml` : job backend (`go vet`, `go test -race -cover ./...`, migrations via `psql`, intégration `catalog`/`auth`/`report`), job frontend (`npm ci --legacy-peer-deps`, `lint:eslint`, `format:check`, `lint`, `test`, `build`).

## 8. Risques identifiés (Phase 0, à confirmer par les tests des phases suivantes)

- **R1 — Journalisation non idempotente de l'essai.** `dbStore.StartTrial` insère une ligne `subscription_events` (`trial_started`) à chaque appel, même si l'abonnement existe déjà (`ON CONFLICT DO NOTHING` ne s'applique qu'à `subscriptions`). Un second appel d'initialisation ne prolonge pas l'essai, mais **duplique l'événement**. *Hypothèse issue de la lecture du code, à reproduire en phase 1 (test PostgreSQL).*
- **R2 — Quota de commandes sur le mois civil.** `CountMonthlyOrders` utilise `date_trunc('month', …)`, alors que le `PLAN` (§3.4) impose une fenêtre de 30 jours ancrée sur la période d'abonnement. Le schéma n'a que `current_period_end` : **pas de `current_period_start`**. Une migration additive et une règle de transition pour les abonnements existants seront nécessaires. *Règle non tranchée : voir la question « Décisions à valider » ci-dessous.*
- **R3 — Expiration non persistée.** L'essai expiré n'est pas transformé en plan FREE en base : `status` reste `TRIALING` et le plan effectif est recalculé à la lecture. Le `PLAN` (§3.2 point 6) exige une transition persistée, idempotente et atomique.
- **R4 — Pas de prix contractuel sur l'abonnement.** `subscriptions` ne stocke que `plan_id` ; le prix est lu dans `plans.price_monthly_xof` au moment de la lecture. Une modification du catalogue se répercuterait sur tous les abonnements (§3.5). Aucun paiement n'est traité dans le code (conforme au périmètre, mais à documenter).
- **R5 — Concurrence du quota.** `CheckOrdersCreation` fait « compter puis vérifier » sans verrou : deux créations simultanées autour de la dernière place peuvent dépasser le plafond (§3.4, critère 5). À reproduire en phase 3.
- **R6 — Couverture CI incomplète.** Le test d'intégration `billing` (`persist_integration_test.go`, tag `integration`) **n'est pas lancé par la CI** : seules les paquets `catalog`, `auth`, `report` le sont. Il faudra l'ajouter à la phase 7.
- **R7 — Documentation et commentaires désynchronisés.** Le commentaire de `billing/handler.go` cite `/api/me/entitlements` alors que la route est `/api/v1/me/entitlements`. La doc `docs/architecture.md` décrit le quota « du mois » (cohérent avec R2 mais contraire au `PLAN`).
- **R8 — Marge ESLint faible.** 48 avertissements pour un seuil à 50 : toute nouvelle dérive frontend fera échouer le lint.
- **R9 — Environnement non identique à la CI.** Les résultats de cette Phase 0 viennent d'un environnement local (PostgreSQL 16 beta, proxy Go local, une ligne `go.sum` contournée). Ils doivent être confirmés par la CI GitHub.

## 9. Décisions à valider avant la phase 2/3 (non tranchées — aucune règle inventée)

1. **Début de la période de 30 jours** pour les abonnements existants qui n'ont pas de `current_period_start` : (a) date de première souscription/essai, (b) date de la migration, (c) autre règle métier.
2. **Commandes annulées/supprimées/archivées** : comptent-elles dans le quota ? (le schéma `orders` doit être relu avant décision).
3. **Quota FREE** : le plafond de 50 commandes reste-t-il « 50 par période de 30 jours » ?

## 10. Phase suivante

**PHASE 1 — Tests de référence et tests de caractérisation.** Objectif : ajouter les tests manquants
(essai, plans et droits, quotas 30 jours, multitenant, prix), en reproduisant R1 et les autres défauts
**sans affaiblir** les assertions. Critère de sortie : nouveaux tests exécutés, résultats consignés,
défauts reproduits listés.

Blocage éventuel : les décisions de la section 9 ne bloquent que les tests de la fenêtre de 30 jours
(phase 3) ; les tests d'essai, de droits et de multitenant peuvent avancer.
