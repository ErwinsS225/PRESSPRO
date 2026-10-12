# Dépôts — lot 2 : finances, monnaie et restitution à solde nul

Date : 2026-10-12. Branche : `arena/4127deca-presspro`. Décisions : [phase 1](depot-phase1-decisions.md). Suivi : [MiseJourClient.md](../MiseJourClient.md).

## Périmètre livré sur la branche

Ce lot met en œuvre et valide les règles financières strictes décidées lors de la phase de cadrage.

### 1. Paiement intégral obligatoire au dépôt
- Tout nouveau dépôt (`POST /api/v1/orders`) exige désormais un règlement couvrant l'intégralité du montant de la commande (`initial_paid` ou `amount_received >= total_amount`).
- Les acomptes partiels ou dépôts à 0 F sont strictement rejetés (`400 Bad Request`, code d'erreur `ErrFullPaymentRequired`).
- Les commandes historiques partiellement payées ou impayées restent consultables et réglables sans altération rétroactive.

### 2. Somme remise & calcul exact de la monnaie
- Saisie facultative ou explicite d'une somme remise (`amount_received`).
- Seul le montant exact dû est inscrit dans la colonne `amount` de la table `payments` et dans le chiffre d'affaires. L'excédent est calculé et persisté dans `change_given`, sans constituer une créance ni un crédit client.
- Tous les calculs s'effectuent en centièmes entiers (`int64`), évitant toute dérive de virgule flottante.

### 3. Restitution strictement subordonnée au solde nul
- La transition d'état vers `delivered` (`PATCH /api/v1/orders/{id}/status`) vérifie en transaction sous verrou `FOR UPDATE` que `total_amount == paid_amount`.
- Si `remaining_due > 0`, l'opération est refusée avec le code `409 Conflict` (`ErrUnpaidDelivery`).
- Une fois le solde encaissé via `POST /api/v1/orders/{id}/payments`, le passage à `delivered` est immédiatement débloqué.

### 4. Idempotence et protection contre les doubles encaissements
- Prise en charge de l'en-tête `Idempotency-Key` au dépôt et à l'encaissement de solde.
- Table `order_mutations` avec verrou applicatif tenant-safe (`pg_advisory_xact_lock`) pour sécuriser les réessais réseaux lors des coupures de connexion au comptoir.
- En cas de double clic ou de réessai identique : renvoi du résultat sans duplication de ticket ni de paiement. En cas de réutilisation de la même clé avec un payload différent : refus `409 Conflict`.

### 5. Migration PostgreSQL 000019
- Ajout des colonnes `amount_received` et `change_given` sur la table `payments`.
- Création de la table `order_mutations` avec index et contrainte unique par établissement.
- Migration aller-retour testée : le `down.sql` protège l'intégrité en refusant la suppression si des règlements comportant de la monnaie rendue existent.

### 6. Raccordement UI & Comptoir
- Modale `NewOrderModal` mise à jour : suppression de l'option d'acompte arbitraire à 50 %, affichage du mode d'encaissement, champ de saisie de la somme remise avec calcul en direct de la monnaie à rendre.
- Blocage du bouton de validation si la somme remise est insuffisante.
- Comptoir (`App.tsx`) : interception et alerte claire si une tentative de restitution est demandée sur une commande non soldée. Encaissement de solde synchronisé avec le serveur avant validation.

---

## Preuves et résultats vérifiés

| Suite de tests / Vérification | Commande exécutée | Résultat |
|---|---|---|
| Tests unitaires Go finances | `go test -v ./internal/order -run TestFinance_` | Passé (calcul exact, monnaie, rejet commande gratuite avec somme) |
| Tests intégration PostgreSQL finances | `go test -tags=integration -run TestFinance_Integration_` | Passé (dépôt, monnaie, refus restitution impayée, concurrence) |
| Tests d'isolation et quotas Go | `go test -tags=integration -race ./internal/order/...` | Passé (détection de courses propre, multi-tenancy garanti) |
| Suite complète intégration Go | `go test -tags=integration -count=1 ./...` | **178 tests de premier niveau + 73 sous-tests réussis** |
| Migrations 000001 à 000019 | Aller-retour `000019_finance_idempotency.{up,down}.sql` | Réussi (protection contre perte de monnaie active) |
| Analyse statique backend | `go vet ./...` et `go vet -tags=integration ./...` | 0 anomalie |
| Compilation binaire | `go build ./cmd/api` et `go mod verify` | Réussi, dépendances vérifiées |
| Suite de tests Vitest frontend | `npm test` | **201 tests réussis (27 fichiers)** |
| Analyse TypeScript & ESLint | `npm run lint` & `npm run lint:eslint` | 0 erreur, 49 avertissements de typage existants |
| Contrôle de formatage | `npm run format:check` | Conforme |
| Build de production frontend | `npm run build` | Réussi en 991 ms |

---

## Prochain lot

- **Lot 3 : Ticket de caisse thermique 80mm & reçu officiel** : alignement de l'impression physique et de la modale sur le ticket officiel avec mention du montant payé, de la somme remise, de la monnaie rendue, et mention claire si le reçu est provisoire ou définitif.
