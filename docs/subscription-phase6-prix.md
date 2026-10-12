# Phase 6 — Diagnostic de persistance des prix

**Périmètre :** prix des formules d'abonnement (`plans.price_monthly_xof`), leur copie
éventuelle sur l'abonnement, leur lecture (droits, grille, annuaire SuperAdmin, MRR) et
leur traçabilité. **Aucun prix n'a été modifié.** Aucune migration ni aucun code de
comportement n'a été changé ; seuls un test de caractérisation et ce document ont été
ajoutés.

Le `PLAN` cité par la Phase 0 (§3.5) n'est pas dans le dépôt. Le diagnostic s'appuie sur
la lecture qu'en fait `docs/subscription-phase0-baseline.md` (risque R4).

## 1. Cartographie : où un prix est écrit, stocké, lu

| Étape | Emplacement | Constat |
|---|---|---|
| Écriture initiale | `backend/migrations/000003_billing.up.sql` (`plans.price_monthly_xof NUMERIC(12,2) ≥ 0`) | Colonne sur le catalogue uniquement |
| Valeurs de départ | `backend/migrations/000004_billing_seed.up.sql` : FREE 0, PRO 45 000, BUSINESS 75 000 | `ON CONFLICT (code) DO NOTHING` |
| Écriture applicative | aucune | Aucun endpoint ni code Go ne fait `UPDATE plans` ou `INSERT INTO plans` (hors seed) |
| Copie sur l'abonnement | aucune | `subscriptions` n'a pas de colonne prix (`plan_id` seul) |
| Lecture abonnement | `billing/persist.go` `GetSubscription` : `JOIN plans` | Prix courant du catalogue |
| Lecture MRR (base) | `billing/persist.go` `PlatformStats` : `SUM(p.price_monthly_xof)` sur `status = 'ACTIVE'` | Prix courant du catalogue |
| Lecture grille | `billing/persist.go` `ListPlans` ; `GET /api/v1/billing/plans` | Prix courant du catalogue |
| Lecture annuaire SuperAdmin (base) | `billing/persist.go` : `COALESCE(p.price_monthly_xof, 0)` | Prix courant du catalogue |
| Traçabilité | `plans.updated_at`, `subscription_events` | Voir R-P6-4 |
| Mode mémoire | `entitlements.go`, `admin.go`, `service.go` | Voir R-P6-1 et R-P6-2 |

Changements de formule : `persist.go` lignes 254 (`ON CONFLICT … DO UPDATE SET plan_id`) et
322 (`SET plan_id = (SELECT id FROM plans WHERE code = …)`). Ils n'enregistrent aucun prix.

## 2. Constats

### R-P6-3 — Pas de prix contractuel : un changement de catalogue touche les abonnés existants (confirme R4)

**Gravité : haute (règle de facturation).** Un abonné ACTIVE n'a pas de prix propre ; son
tarif est celui du catalogue au moment de la lecture.

**Reproduction** (transaction annulée, base `pressing_os` du sandbox, aucune donnée conservée) :

| Étape | Prix lu pour l'abonné PRO | MRR plateforme |
|---|---|---|
| Avant | 45 000 | 45 000 |
| `UPDATE plans SET price_monthly_xof = 50000 WHERE code='PRO'` | 50 000 | 50 000 |
| `ROLLBACK` | catalogue revenu à 0 / 45 000 / 75 000 | — |

Un simple changement de catalogue modifie donc le prix payé et affiché par tous les
établissements PRO, sans action sur leur abonnement et sans trace.

### R-P6-4 — Aucune trace d'un changement de prix

**Gravité : haute (auditabilité).** Vérifié dans la même transaction :
- `plans.updated_at` reste à sa valeur de création : aucun trigger et aucun code ne le met à jour.
- Aucune table d'historique de prix ; `subscription_events` ne porte aucun prix.
- Aucun trigger sur `plans` ni `subscriptions` (requête sur `pg_trigger`).

### R-P6-5 — Seul chemin de modification : migration de seed ou SQL manuel

**Gravité : moyenne.** Il n'existe aucun endpoint de gestion des formules. Le seed
(`000004`) est marqué « à ajuster après pilotes », mais :
- son `ON CONFLICT (code) DO NOTHING` signifie qu'une base existante garde ses prix même si
  le fichier change : deux environnements peuvent diverger ;
- une modification de prix exige donc une nouvelle migration (convention de migrations
  immuables) ou un SQL manuel non tracé.

### R-P6-1 — Mode mémoire : l'annuaire SuperAdmin affiche « 0 FCFA / mois » (défaut)

**Gravité : moyenne (mode dégradé, mais affichage trompeur).** `TenantsOverview` en mémoire
ne renseigne pas `PriceMonthlyXOF` : la valeur vaut 0 pour tout établissement, y compris
PRO. `SuperAdminView` formate ce champ directement (`formatXOF(t.price_monthly_xof)`), donc
un établissement PRO s'affiche « 0 FCFA / mois ». Ce 0 est un prix inventé, contraire à la
règle « aucun prix fabriqué ».

Caractérisé par `TestPrixDiagnostic_MemoireAnnuaireAffiche0Franc`.

### R-P6-2 — Mode mémoire : MRR à 0 pour des abonnements payants (défaut)

**Gravité : moyenne (mode dégradé).** `AdminStats` ajoute `rec.PriceMonthly`, champ jamais
renseigné en mémoire. Un abonnement PRO actif produit un MRR de 0.

Caractérisé par `TestPrixDiagnostic_MemoireMRRVautZeroPourUnAbonnementPayant`.

## 3. Ce qui est correct (vérifié)

- **Valeurs de départ** : `plans` contient FREE 0,00 / PRO 45 000,00 / BUSINESS 75 000,00
  (base vérifiée), conformes au PLAN et à la Phase 0.
- **Aucun prix codé dans le frontend** : recherche des montants dans `src/` (hors tests) ;
  les seuls montants restent dans les fichiers de test. Les écrans lisent l'API.
- **Pas de prix inventé en mémoire pour la grille et les droits** : `PriceMonthlyXOF` est
  absent (`nil`) dans `/billing/plans` et `/me/entitlements` (`fenetre_test.go`,
  `memoire_phase5_test.go`).
- **Précision** : `NUMERIC(12,2)` lu en `float64` ; les montants XOF entiers sont exacts.
- **Essai** : le tarif affiché pendant l'essai est celui de la formule souscrite (FREE, 0),
  pas celui des droits effectifs (BUSINESS).
- **Lignes de commande** : `order_items` conserve `item_label` et `unit_price` figés au
  moment de la commande (`000008`). Un changement de tarif de service ne modifie pas les
  commandes passées.
- **Grilles tarifaires** : chaque version est un instantané JSONB (`000010`).

## 4. Décisions du propriétaire (recueillies le 2026-10-12)

1. **Abonnés existants : suivre le catalogue.** Un changement de tarif s'applique à tous
   les abonnés. Le comportement actuel est donc conservé ; il reste à le **tracer** (R-P6-4).
   Conséquence : la colonne de prix contractuel (option A) n'est pas requise pour l'instant.
2. **Modification des prix : endpoint SuperAdmin avec journal d'audit.** Chaque changement
   passe par une API réservée au superadmin et laisse une trace (qui, quand, ancien et
   nouveau prix). Ce n'est pas une migration ni du SQL manuel.
3. **Prix de départ : à redéfinir.** Le propriétaire fournira les nouveaux montants. Les
   valeurs actuelles (45 000 et 75 000 XOF) restent en vigueur tant qu'elles ne sont pas
   remplacées. Aucun nouveau montant n'a été appliqué ni inventé.

## 5. Correction retenue (à implémenter, non appliquée en Phase 6)

- **Traçabilité (R-P6-4) :** trigger ou écriture applicative qui met à jour `plans.updated_at`
  et journalise chaque changement de prix (ancien, nouveau, acteur, date).
- **Endpoint SuperAdmin (R-P6-5) :** modification d'un prix réservée à `superadmin`, validée
  (montant ≥ 0), auditée, sans effet rétroactif sur les commandes.
- **Mode mémoire (R-P6-1, R-P6-2) :** absence de tarif (`nil`) plutôt que 0 pour l'annuaire
  et le MRR ; affichage « Tarif non communiqué » côté SuperAdmin.
- Les tests de caractérisation `prix_diagnostic_test.go` devront alors être inversés.
- Option A (prix contractuel sur l'abonnement) : écartée pour l'instant par la décision 1.

## 6. Tests

- `backend/internal/billing/prix_diagnostic_test.go` (nouveau) : deux tests de
  caractérisation (R-P6-1, R-P6-2) qui assertent le comportement actuel. **À inverser**
  lors de la correction.
- Existant, à conserver : `TestLifecycle_PrixLuDuCatalogue` (intégration, lecture du
  catalogue), `TestCatalogue_MemoireSansPrix` et `TestMemoire_CatalogueDesFormules` (absence de
  tarif en mémoire).
- La reproduction R-P6-3 est faite en SQL dans une transaction annulée, et non dans un test
  automatisé : modifier `plans` dans un test partagé perturberait les autres paquets
  d'intégration qui lisent les mêmes lignes.

## 7. Résultat

- Phase 6 : diagnostic terminé. 5 défauts consignés (R-P6-1 à R-P6-5), dont 2 de gravité
  haute (R-P6-3, R-P6-4).
- Aucun prix modifié. Décisions du propriétaire recueillies (section 4). Les nouveaux prix sont attendus avant toute modification de tarif.
