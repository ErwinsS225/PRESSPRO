# PRESSPRO — Rapport des phases 1 et 2 (tests de caractérisation et cycle de vie des abonnements)

Suite de `docs/subscription-phase0-baseline.md`. Périmètre : tests de caractérisation (phase 1) puis
correctifs du cycle de vie des abonnements (phase 2). **La phase 3 (fenêtre de 30 jours, annulations,
concurrence sur le quota) n'est pas implémentée.** Aucun tarif n'a été modifié, aucun paiement n'est
simulé, aucune migration de schéma n'a été ajoutée.

---

## Phase 1 — Tests de référence et de caractérisation

**État :** terminé pour le périmètre A, B, D, E ; les tests de la fenêtre de 30 jours et de la
concurrence du quota (phase 3) sont écrits mais **volontairement exclus de la CI** (tag `phase3`).

**Constat (avant correctifs)** — tests qui échouaient sur le code de `main` :

| Test | Résultat avant correctif | Défaut reproduit |
|---|---|---|
| `TestLifecycle_EnsureTrialDeuxFois_JournaliseUneSeuleFois` | ÉCHEC : 2 événements `trial_started` | R1 : journal non idempotent |
| `TestLifecycle_EssaiExpire_PersisteVersFreeSansAppelFrontend` | ÉCHEC : statut `TRIALING`, 0 événement `trial_expired` | R3 : expiration non persistée à la lecture |
| `TestLifecycle_ExpirationConcurrente_UneSeuleTransition` | ÉCHEC **intermittent** : 12 échecs sur 15 exécutions (2 ou 3 transitions) | Nouveau (concurrence de l'expiration) : lecture puis écriture sans verrou |
| `TestLifecycle_ErreurDeLecture_NonSilencieuse` | ÉCHEC : erreur avalée, `FREE/ACTIVE` renvoyé | Nouveau (non listé en phase 0) : `dbStore.GetSubscription` renvoie `(nil, nil)` sur toute erreur |
| `TestLifecycleMemoire_EssaiExpire_PersisteVersFree` | ÉCHEC : statut `TRIALING` | R3 (mémoire, même cause que R3) |
| `TestLifecycleMemoire_ExpirationConcurrente_UneSeuleTransition` | ÉCHEC **intermittent** : 43 échecs sur 300 + `DATA RACE` | Concurrence de l'expiration (mémoire), lecture de l'enregistrement hors verrou |
| `TestLifecycleMemoire_GardeDeSuspension_*` (première version, « fail-closed ») | ÉCHEC | Voir décision D1 ci-dessous |

Tests de **caractérisation** passant avant correctif (comportement à préserver) : pas de redémarrage
de l'essai, abonnement payé non expiré, données conservées après expiration, fallback FREE sans
abonnement, rétrogradation qui conserve les données, prix lu depuis le catalogue, isolation du quota
entre établissements, erreur de persistance propagée par le moteur.

**Tests de phase 3 (écrits, échouent, exclus de la CI) — tag `phase3` :**

| Test | Résultat | Défaut |
|---|---|---|
| `TestPhase3_CommandeAnnulee_NeConsommePasLeQuota` | ÉCHEC : la commande annulée consomme le quota | `CountMonthlyOrders` compte les annulées |
| `TestPhase3_FenetreDe30Jours_AncreeSurLaPeriode` | ÉCHEC : 2 commandes comptées au lieu de 1 | mois civil (`date_trunc('month', NOW())`) |
| `TestPhase3_CreationsConcurrentes_AuDernierEmplacement` | ÉCHEC : 3 créations acceptées pour 1 place | contrôle « compter puis vérifier » non atomique |

Remarque : le test de fenêtre dépend de la date d'exécution (il ne reproduit l'écart que si la commande
« d'il y a 10 jours » est dans le même mois civil). La phase 3 remplacera ce point par une date
de référence explicite.

**Fichiers ajoutés :**
- `backend/internal/billing/subscription_lifecycle_integration_test.go` (tag `integration`, PostgreSQL réel)
- `backend/internal/billing/subscription_lifecycle_memory_test.go` (tests unitaires du mode mémoire)
- `backend/internal/billing/quota_window_pending_test.go` (tag `integration && phase3`)

---

## Phase 2 — Correction du cycle de vie des abonnements

**État :** terminé pour le cycle de vie (essai, expiration, journalisation, erreurs de persistance).
La fenêtre de 30 jours et le verrouillage du quota restent à faire (phase 3).

**Correctifs :**

1. **Expiration persistée à la lecture (R3).** `Service.GetSubscription` détecte un essai terminé
   (`now >= trial_ends_at`, même borne que `OnTrial`) et déclenche la transition côté serveur. Elle ne
   dépend plus de `GET /me/entitlements` ni d'un cron : le middleware, le quota et `/me` passent tous par
   cette lecture.
2. **Transition atomique et idempotente (R3b).**
   - PostgreSQL : `dbStore.ExpireTrial` ouvre une transaction, verrouille la ligne (`FOR UPDATE OF sub`),
     recontrôle le statut et la date sous verrou, puis met à jour et journalise dans la même transaction.
   - Mémoire : `applyTrialExpiry` vérifie et modifie sous le même verrou `sync.RWMutex`.
   - Un seul appel obtient `true` ; un abonnement payé ou déjà actif n'est jamais touché.
3. **Journal sans doublon (R1).** `StartTrial` est une seule instruction SQL (CTE) : l'événement
   `trial_started` n'est inséré que si l'abonnement est réellement créé.
4. **Erreurs de persistance explicites (défaut nouveau, non listé en phase 0).** `dbStore.GetSubscription` ne renvoie `nil` que pour
   `pgx.ErrNoRows` (absence d'abonnement) ; toute autre erreur remonte.
5. **Erreurs cohérentes côté HTTP.** Les refus métier restent en **402** avec le même message
   (`ErrFeatureNotIncluded`, `ErrLimitReached`, `billing.IsRefusMetier`). Une panne de lecture répond
   **500** générique, sans détail technique :
   - `RequireFeature` : 402 remplacé par 500 si les droits sont illisibles ;
   - `RequireFeatureQuota` et la création de commande : une erreur de compteur ou de droits ne fuit
     plus son texte SQL dans une réponse 402.
6. **Correction de course dans le mode mémoire.** `GetSubscription` renvoie une copie sous verrou (la
   détection `go test -race` signalait une lecture concurrente de l'enregistrement).
7. **CI.** `.github/workflows/ci.yml` lance désormais `./internal/billing/...` dans l'étape des tests
   d'intégration PostgreSQL (les tests d'intégration billing n'étaient exécutés par aucune étape).

**Résultats après correctifs (environnement local, voir section Environnement) :**

| Commande | Résultat |
|---|---|
| `go vet ./...` | OK |
| `go test -race -cover -count=1 ./...` | OK — 13 paquets `ok`, aucun échec |
| `go test -tags=integration -count=1 -v ./internal/catalog/... ./internal/auth/... ./internal/report/... ./internal/billing/...` | OK — 64 `--- PASS`, aucun échec |
| Stress : expiration concurrente PostgreSQL, 15 exécutions | 0 échec (avant : 12/15) |
| Stress : expiration concurrente mémoire, `-race`, 300 exécutions | 0 échec, 0 `DATA RACE` (avant : 43 échecs) |
| `go test -tags='integration phase3' -run Phase3` | ÉCHEC attendu (3 tests, phase 3 non implémentée) |

Aucun test préexistant n'a été modifié. Le seul test réécrit en cours de phase est un test ajouté pendant
cette phase (garde de suspension, devenu test de politique documentée). Aucune assertion n'a été assouplie.

**Fichiers modifiés :**
- `backend/internal/billing/entitlements.go` — `GetSubscription`, `readSubscription`, `applyTrialExpiry`, `trialExpired`, `CheckLimit` (refus typés), `ExpireTrialIfNeeded`.
- `backend/internal/billing/persist.go` — `GetSubscription` (erreurs), `StartTrial` (CTE), `ExpireTrial` (transaction verrouillée).
- `backend/internal/billing/store_contract.go` — méthode `ExpireTrial` ajoutée à l'interface `Store`.
- `backend/internal/billing/errors.go` (nouveau) — `ErrFeatureNotIncluded`, `ErrLimitReached`, `ErrUnknownFeature`, `IsRefusMetier`.
- `backend/internal/http/feature_middleware.go` — `RequireFeature`, `RequireFeatureQuota`.
- `backend/internal/order/handler.go` — garde-fou de quota à la création de commande.
- `.github/workflows/ci.yml` — ajout de `./internal/billing/...` aux tests d'intégration.
- Tests : `subscription_lifecycle_integration_test.go`, `subscription_lifecycle_memory_test.go`, `quota_window_pending_test.go` (voir phase 1).

**Aucune migration de schéma** n'a été nécessaire pour ces correctifs.

---

## Décisions et points à confirmer

- **D1 — Garde de suspension en cas d'erreur de lecture (fail-open, existant).** `IsTenantSuspended`
  laisse passer l'établissement si son état ne peut pas être lu. Le code le documente comme un choix
  délibéré (une panne transitoire ne doit pas bloquer tout un pressing). **Ce n'est pas une règle
  approuvée dans le PLAN** : un établissement suspendu peut donc opérer pendant une panne de lecture.
  Je n'ai pas modifié ce comportement. Le test `TestLifecycleMemoire_GardeDeSuspension_PolitiqueDocumentee`
  le fige. **À confirmer par le propriétaire du projet** : fail-open (actuel) ou fail-closed.
- **D2 — Expiration « à la lecture ».** Une lecture (`GET`) peut écrire la transition FREE, une seule
  fois par essai. C'est une écriture déclenchée par une requête, prévue par le PLAN (« côté serveur, pas
  de cron seul »). Si la base est en lecture seule, la lecture échoue alors en erreur explicite (500).
- **D3 — Décalage de la borne.** L'essai est considéré terminé à `now >= trial_ends_at` (cohérent avec
  `OnTrial`). L'ancien code utilisait `now > trial_ends_at`.
- **D4 — Aucune route de changement de plan.** Il n'existe pas d'API d'administration pour passer un
  établissement à PRO ou BUSINESS (`SetPlan` n'est appelé que par l'expiration). La rétrogradation n'est
  testée qu'en SQL direct. À traiter avant toute mise en production commerciale.
- **D5 — Règles de la phase 3** (réponses reçues) : début de période = `trial_started_at`, à défaut
  `created_at` ; fenêtre glissante de 30 jours ; les commandes annulées (`status = 'cancelled'`) ne
  comptent pas.

## Environnement et limites

- Sandbox sans Docker. PostgreSQL 16 provient des binaires npm `@embedded-postgres/linux-x64`
  (`16.13.0-beta.17`), avec le même utilisateur et la même base que la CI. Les 15 migrations `up` ont été
  appliquées en ordre par script (pas de `psql`).
- Go 1.25.4 via le wheel PyPI `go-bin`. Le proxy de modules officiel n'est pas joignable : un proxy local
  (`file://`, hors dépôt) a été construit à partir des tags GitHub. **Hors dépôt, `go.mod` et `go.sum`
  inchangés.** Une ligne `go.sum` (`go-spew v1.1.1`, dépendance de test transitive) n'a pas pu être
  reproduite : elle a été retirée d'une copie `/tmp` pour l'exécution locale uniquement. **La CI GitHub
  reste la référence.**
- Le frontend n'a pas été modifié pendant ces phases.
- Les tests de concurrence sont probabilistes : un passage vert ne prouve pas l'absence de course. Les
  reproductions ci-dessus utilisent des répétitions (15 et 300 exécutions).
