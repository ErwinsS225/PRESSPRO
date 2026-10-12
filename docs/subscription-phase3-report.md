# PRESSPRO — Rapport de la phase 3 (quota de commandes sur 30 jours, concurrence)

Suite de `docs/subscription-phase1-2-report.md`. Périmètre : plafond de commandes du plan, fenêtre
glissante de 30 jours ancrée sur l'abonnement, exclusion des commandes annulées, contrôle
concurrent-safe à la création. Aucun tarif modifié, aucun paiement ou webhook simulé.

---

## Phase 3 — Quota de commandes sur 30 jours, concurrence

**État :** terminé dans le périmètre de la phase 3 (code, migration, tests, stress). Les points
de l'exigence « concurrence » sont couverts par des tests d'intégration PostgreSQL réels. Commit
dédié, poussé sur la branche de session (voir section GitHub).

**Constat (avant correctif, mesuré par les tests de caractérisation de la phase 1) :**

| Défaut | Preuve | Cause |
|---|---|---|
| Quota sur le mois civil | `TestPhase3_FenetreDe30Jours` : 2 comptées au lieu de 1 | `date_trunc('month', NOW())` |
| Commande annulée comptée | `TestPhase3_CommandeAnnulee` : la 50e refusée à tort | aucun filtre sur `status` |
| Création simultanée non atomique | `TestPhase3_CreationsConcurrentes` : 3 créations acceptées pour 1 place | contrôle « compter puis créer » dans le handler, hors transaction |
| Mode mémoire sans contrôle au niveau service | lecture du code | le contrôle n'existait que dans le handler HTTP |

Constat complémentaire : `nextOrderNumberTx` sérialise déjà les créations d'un établissement via le
verrou de ligne sur `order_sequences`. Ce verrou **ne suffit pas** au quota, car le comptage a lieu
avant la prise de ce verrou. Preuve par mutation (voir Tests) : sans le verrou consultatif, jusqu'à
5 créations sont acceptées pour 1 place.

**Décisions d'implémentation :**

1. **Ancre de fenêtre.** `COALESCE(subscriptions.trial_started_at, subscriptions.created_at,
   tenants.created_at)`. `tenants.created_at` est `NOT NULL`, donc l'ancre existe toujours. La fenêtre
   courante commence à `ancre + k × 30 jours`, avec `k = max(0, floor((maintenant − ancre) / 30 j))`.
   Elle se renouvelle sans job et ne dépend pas du plan : un changement de formule ne la remet pas à
   zéro.
2. **Bornes.** Début inclus, fin exclue, au niveau de la seconde.
3. **Commandes annulées.** `status <> 'cancelled'` : elles libèrent leur place (décision utilisateur).
4. **Contrôle dans la transaction de création.** `creerCommande` :
   - lit le plafond du plan **avant** `Begin` (aucune seconde connexion pendant le verrou) ;
   - ouvre la transaction ;
   - prend `pg_advisory_xact_lock(hashtextextended('quota-commandes:' || tenant, 0))` ;
   - compte la fenêtre dans la même transaction, après le verrou ;
   - insère la commande ; le verrou est libéré au `COMMIT` ou au `ROLLBACK`.
   Une création échouée ne consomme donc aucune place.
5. **Verrou par établissement.** Un établissement n'en bloque pas un autre. Le verrou vit dans
   PostgreSQL : la garantie tient entre plusieurs instances du backend.
6. **Mode mémoire.** Le contrôle est fait sous le verrou du service, donc atomiquement avec
   l'insertion. Le compteur mémoire reste nul par défaut (voir Limites).
7. **Contrôle au service, plus au handler.** `Service.CreateOrder` porte le garde-fou. Le handler
   n'a plus de pré-contrôle. `respondServiceError` traduit tout refus métier en **402** avec
   `upgrade_required`, comme avant.
8. **Migration 000016 (additive).** `idx_orders_tenant_created ON orders(tenant_id, created_at)`.
   Le retour arrière supprime seulement cet index. Aucune donnée modifiée.
9. **Renommage.** `CountMonthlyOrders` devient `CountOrdersInPeriod(ctx, tenantID, now)`. Tous les
   appelants sont mis à jour (`entitlements.go`, `billing/persist.go`, `router_test.go`, tests).

**Fichiers modifiés :**
- `backend/internal/billing/quota.go` — réécrit : `QuotaGuard`, `PlafondCommandes` (hors transaction),
  `VerifierCommande` (mode mémoire), `VerifierCommandeTx` (verrou + comptage dans la transaction),
  `verifierPlafond` (règle partagée avec `CheckLimit`).
- `backend/internal/billing/persist.go` — `requeteFenetre` (ancre), `requeteCompteFenetre` (fenêtre
  glissante, hors annulées), `CountOrdersInPeriod` et `CountOrdersInPeriodTx` ; UUID non valide → 0.
- `backend/internal/billing/service.go`, `entitlements.go`, `model.go` — interface `UsageCounter`,
  `OrdersPeriodDays = 30`, `CheckLimit` appelle `verifierPlafond`.
- `backend/internal/order/service.go` — `SetQuotaGuard` ; contrôle du plafond en mode mémoire sous
  verrou ; délégation `creerCommande` avec le garde-fou.
- `backend/internal/order/persist.go` — `creerCommande(ctx, tenant, req, quota)` : plafond lu avant la
  transaction, contrôle après le verrou.
- `backend/internal/order/handler.go` — pré-contrôle supprimé ; `SetQuotaGuard` délègue au service ;
  `respondServiceError` traduit les refus métier en 402.
- `backend/migrations/000016_orders_tenant_created_index.up.sql` / `.down.sql` (nouveaux).
- `backend/internal/billing/quota_window_pending_test.go` supprimé ; remplacé par
  `backend/internal/billing/quota_window_integration_test.go` (tag `integration`, exécuté par la CI).
- `backend/internal/order/quota_integration_test.go` (nouveau, tag `integration`).
- `backend/internal/billing/entitlements_test.go`, `backend/internal/http/router_test.go` — fixtures renommées.
- `.github/workflows/ci.yml` — ajout de `./internal/order/...` à l'étape d'intégration PostgreSQL.
- `docs/subscription-phase1-2-report.md` — note de mise à jour (le fichier `phase3` n'existe plus).

**Tests :**

| Commande | Résultat |
|---|---|
| `gofmt -l .` | aucun fichier à reformater |
| `go vet ./...` et `go vet -tags=integration ./...` | OK |
| `go test -race -count=1 ./...` (unitaires) | OK : tous les paquets `ok`, aucun échec |
| `go test -race -tags=integration -count=1 ./...` (PostgreSQL 16 réel) | OK : tous les paquets `ok` |
| `TestQuota_*` (billing, 7 tests) | OK : annulée, bornes à la seconde, roulement sur 2 mois, ancrage `created_at` abonnement, ancrage établissement, changement de formule, plafond FREE/BUSINESS |
| `TestQuotaCommandes_*` (order, 4 tests) | OK : 10 créations simultanées (1 acceptée, 9 refusées) ; 2 instances (pools distincts) ; création échouée sans consommation ; isolement entre établissements |
| Stress `TestQuotaCommandes_` `-race -count=50` | OK : 50 exécutions, 0 échec, 0 `DATA RACE` |
| Stress `TestQuota_` `-race -count=20` | OK : 20 exécutions, 0 échec |
| Mutation 1 : verrou consultatif supprimé | ÉCHEC attendu et observé : jusqu'à 5 acceptées pour 1 place (sur 5 exécutions de la paire de tests) |
| Mutation 2 : borne de fin de fenêtre rendue inclusive | ÉCHEC attendu et observé : 3 comptées au lieu de 2 |
| Migration 000016 : down, puis up | OK : index supprimé puis recréé ; plan de requête exploitable |

Les tests de concurrence sont probabilistes : les répétitions ci-dessus donnent une confiance, pas une
preuve absolue. Les deux mutations montrent que les tests détectent la régression visée.

**Sécurité et multitenant :**
- Toutes les requêtes de comptage et de verrou sont paramétrées par `tenant_id` ; aucune requête
  n'agrège sans filtre d'établissement.
- Un identifiant d'établissement non UUID (comptes de démonstration) donne un compteur à 0, sans
  erreur SQL. La création échoue alors comme avant, à l'insertion.
- Le verrou est propre à chaque établissement (`quota-commandes:<tenant>`) : test
  `TestQuotaCommandes_Isolement_VerrouParEtablissement` (B crée pendant que A tient le verrou).
- Une erreur de lecture (plafond, compteur) répond **500** générique, sans détail SQL ; seul un refus
  métier répond 402.

**Décisions :**
- Ancre : `trial_started_at`, à défaut `created_at` (abonnement), à défaut `created_at` (établissement).
  Décision utilisateur, appliquée.
- Commandes annulées exclues du quota. Décision utilisateur, appliquée.
- Le plafond du plan est lu hors transaction. Un changement de formule concurrent à une création peut
  donc, au pire, laisser passer une commande au-delà du plafond de l'ancien plan ; cette fenêtre est
  d'une seule création et sans conséquence de données. Accepté et consigné.
- Le verrou consultatif est identifié par `hashtextextended`. Une collision entre deux établissements
  ne rend pas le quota incorrect : elle sérialise seulement deux créations.

**Limites connues (non traitées en phase 3) :**
- **Mode mémoire sans PostgreSQL.** Le compteur par défaut est nul : le plafond n'est pas appliqué.
  Ce comportement est identique à `main` ; il ne concerne que le mode dégradé.
- **Libellé frontend.** `src/components/BillingView.tsx` affiche « Commandes ce mois » alors que le
  quota est désormais une fenêtre de 30 jours. Correction prévue en phase 5 (frontend reflétant les
  droits du backend), non traitée ici pour rester dans le périmètre de la phase.
- **Pas de route de changement de plan** (point D4 de la phase 1-2). Le test de changement de formule
  passe par SQL direct.

**GitHub :**
- Branche de travail : `arena/a06916de-presspro`. Aucun merge, aucune PR, aucun déploiement.
- Phase 1 : commit `8c8e6f7` (tests de caractérisation), poussé.
- Phase 2 : commit `1236301` (correctifs du cycle de vie), poussé.
- Phase 3 : commit `2970bc2` (quota de commandes), poussé sur la même branche.
- Aucune PR ouverte : l'ouverture et la fusion attendent l'autorisation explicite du propriétaire.

**Phase suivante :** phase 4 — couverture des routes et matrice d'isolation multitenant (tests négatifs
sur toutes les routes de commandes, clients, catalogue, facturation, établissements). Suivi :
phase 5, qui corrigera le libellé « Commandes ce mois ».
