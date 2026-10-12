# PRESSPRO — Clôture du plan d'abonnement (phases 0 à 8)

Branche : `arena/a06916de-presspro` (base `main` @ `7da3ff8`). Ce document clôt le plan
d'abonnement. Il ne fusionne rien : la fusion reste une décision explicite du propriétaire.

> **Limite de ce document.** Le `PLAN` (sections 3.5, 8 et 9) n'est pas présent dans le dépôt. Les
> critères d'acceptation ci-dessous sont repris du suivi de session et regroupés par thème ; le
> texte officiel des 12 critères doit être recoupé avec le `PLAN` avant fusion.

---

## 1. Phases et commits

| Phase | Objet | Commits | État |
|---|---|---|---|
| 0 | État de référence | `5e89b6d` | Terminé — `docs/subscription-phase0-baseline.md` |
| 1–2 | Tests de caractérisation, expiration d'essai persistée (atomique, idempotente, journalisée) | `8c8e6f7`, `1236301` | Terminé — `docs/subscription-phase1-2-report.md` |
| 3 | Quota de commandes glissant sur 30 jours, concurrence-safe | `2970bc2`, `f61b05e`, `b682d97` | Terminé — `docs/subscription-phase3-report.md` |
| 4 | Matrice des routes et isolation multitenant | `01e2bb9`, `99193d6`, `50f0851` | Terminé — `docs/subscription-phase4-report.md`, `docs/subscription-phase4-matrix.md` |
| 5 | API et frontend reflétant les droits effectifs | `c131e1a` | Terminé — `docs/subscription-phase5-report.md` |
| 6 | Diagnostic de persistance des prix | `68e1891` | Terminé — `docs/subscription-phase6-prix.md` |
| 6 bis | Correction des tarifs (validée par le propriétaire) | `66bd3a8` | Terminé — voir §3 |
| 7 | Validation finale et CI | `f56c355` | Terminé |
| 8 | Documentation et pull request | `f3b7389` + PR | En cours de revue (non fusionnée) |

## 2. Critères d'acceptation (PLAN §8, regroupés)

| Thème | Preuve | Statut |
|---|---|---|
| Initialisation de l'essai idempotente ; un second appel ne prolonge pas l'essai | Tests de cycle de vie (phase 2) | Atteint |
| Expiration vers FREE côté serveur, déterministe, sans confirmation de paiement, journalisée dans `subscription_events` | Phase 2 ; `subscription_baseline_test.go` | Atteint |
| Données intactes après expiration ou rétrogradation | Phase 2 et 4 : aucune suppression ; D6 (§4) | Atteint |
| Quota de commandes : fenêtre glissante de 30 jours ancrée sur l'abonnement, limites issues des droits de la formule | Phase 3 ; `fenetre_test.go`, `quota_window_integration_test.go` | Atteint |
| Quotas sûrs en concurrence | Phase 3 | Atteint |
| Isolation multitenant avec tests négatifs | Phase 4 : 17 tentatives d'accès croisé refusées en base ; matrice complète | Atteint (mode mémoire : limites §5) |
| L'API expose les droits effectifs | Phase 5 : `/me/entitlements`, `/billing/plans` | Atteint |
| Pas de prix codé en dur dans le frontend | Phase 6 (contrôle) ; `adminApi.ts`, `SuperAdminView.tsx`, `billingApi.ts` | Atteint |
| **Prix inchangés** | **Conflit résolu explicitement : voir §3** | Remplacé par une correction validée |
| Migrations, tests et builds validés | Phase 7 (base vierge, 17 migrations) ; §6 | Atteint |
| Documentation à jour | Phase 8 : `api.md`, `architecture.md`, rapports | Atteint |
| Pull request avec preuves | §7 | Ouverte, non fusionnée |

## 3. Conflit « prix inchangés » : réconciliation

Le critère « prix inchangés » du PLAN ne peut pas être tenu à la lettre : le propriétaire a
validé la correction des tarifs. Décision prise le 2026-10-12 :

- **PRO : 45 000 → 10 000 XOF/mois ; BUSINESS : 75 000 → 25 000 XOF/mois.** Le mapping suit
  l'ordre croissant des montants fournis. Confirmé par le propriétaire.
- Le changement passe par la **migration `000017`** (correction unique, gardée sur l'ancienne
  valeur) et par le **nouvel endpoint SuperAdmin** `PATCH /api/v1/admin/plans/{code}/price`,
  journalisé. Pas de SQL manuel.
- **Journal** : `plan_price_history` (ancien et nouveau prix, acteur, motif, date) alimenté par
  le trigger `trg_plans_journal_prix`. `plans.updated_at` est désormais mis à jour.
- **Abonnements** : ils suivent toujours le catalogue (décision de la phase 6). Les commandes
  déjà créées gardent leur prix figé (`unit_price`).
- **Mémoire** : MRR et tarifs de l'annuaire valent `null` au lieu de 0 ; l'interface affiche
  « Tarif non communiqué ». La modification y est refusée (`503`).

**Impact à vérifier avant fusion :** comme les abonnements suivent le catalogue, **tout
abonnement actif en base voit son tarif et son MRR passer immédiatement à la nouvelle valeur**.
Si un environnement contient de vrais abonnés, il faut en informer le propriétaire avant la
fusion. Le down de la migration rétablit les anciens tarifs seulement s'ils sont encore à leur
nouvelle valeur.

## 4. Décision D6 — suppression d'une dépense en formule gratuite

Confirmée par le propriétaire (phase 4) : `DELETE /api/v1/expenses/{id}` reste **bloqué (402)**
après une rétrogradation en FREE. Les dépenses existantes sont **conservées** et restent
lisibles ; seule la suppression est gardée par `RequireFeature(expenses.advanced)`. Règle
documentée dans `docs/api.md`. Elle devra être reportée dans le `PLAN` (le fichier n'est pas dans
le dépôt).

## 5. Risques résiduels

| Réf. | Risque | Degré | Traitement |
|---|---|---|---|
| R-P4-1 | Clients non persistés en mode mémoire | Faible | Limite documentée, mode dégradé uniquement |
| R-P6-3 | Pas de prix contractuel par abonnement : un changement de catalogue touche tous les abonnés | Élevé (accepté) | Décision propriétaire : les abonnés suivent le catalogue ; traçabilité par le journal. Voir §3 |
| — | `/admin/*` protégé seulement par `RequireRole(superadmin)` dans le groupe d'appartenance (PLAN §3.7 non totalement respecté) | Moyen | À traiter dans un chantier distinct : un contrôle explicite sur les routes admin hors appartenance est recommandé |
| — | `ForcePlanForTest` exporté depuis du code non test (`billing/testing_hook.go`) | Faible | Non atteignable en production par le routage ; à déplacer dans un fichier `_test` lors d'un prochain chantier |
| — | Description du seed `orders.monthly.max` : « par mois civil » (obsolète) | Faible | Migrations immuables : documenté, non modifié |
| — | Mémoire : `/admin/tenants/{id}` répond 400 pour un identifiant non UUID | Faible | Accepté |
| — | Mode mémoire sans abonnement : pas de fenêtre de commandes exposée | Faible | Accepté, mode dégradé |
| — | Migration de `ubuntu-latest` vers Ubuntu 26 le 2026-10-19 (avertissement CI) | Moyen | À surveiller : le workflow peut changer de comportement |
| — | 48 avertissements ESLint (budget 50) | Faible | Sous le budget |

## 6. Validation (dernier passage, phase 8)

**Backend** (base PostgreSQL vierge `pressing_p8`, 17 migrations appliquées sans erreur) :
- `gofmt` propre ; `go vet ./...` et `go vet -tags=integration ./...` OK.
- `go test ./...` OK (tous les paquets).
- Intégration : **218 PASS, 0 FAIL, 1 SKIP** (`TestSuspension_GeleLesModulesMetier/mémoire`, attendu).
- Migration `000017` : aller-retour vérifié (up → down → up) ; les prix suivent et le journal se
  crée et se retire.
- `go test -race` : `internal/billing`, `internal/http` OK.

**Frontend** :
- `npm run lint` (tsc) OK ; ESLint 0 erreur, 48 avertissements (budget 50) ; Prettier OK.
- `npm test` : **196 tests OK** (25 fichiers).
- `npm run build` OK.

**CI GitHub Actions :** `38213920605` sur `f56c355` : succès (backend et frontend). Le run sur
`f3b7389` est à vérifier et à reporter dans la PR (§7).

## 7. Pull request

Ouverte depuis `arena/a06916de-presspro` vers `main`, **non fusionnée**. Elle contient les preuves
ci-dessus. La fusion relève d'une autorisation explicite du propriétaire.
