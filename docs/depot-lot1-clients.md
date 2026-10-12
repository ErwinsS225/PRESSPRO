# Dépôts — lot 1 : identité client et transaction

Date : 2026-10-12. Branche : `arena/4127deca-presspro`. Décisions : [phase 1](depot-phase1-decisions.md). Suivi : [MiseJourClient.md](../MiseJourClient.md).

## Périmètre livré sur la branche

**Premier lot clients uniquement, pas un parcours caisse terminé.** Les règles de paiement intégral, de monnaie à rendre et de restitution à solde nul sont confirmées mais seront implémentées dans le lot financier suivant. Aucun tarif, quota ou droit d'abonnement modifié. Aucun commit, push, PR ou fusion réalisé pendant ce lot.

### Backend

- Téléphone obligatoire et non vide ; suppression de la fabrication de numéros `anonyme-*` au dépôt. Validation commune CRUD/dépôt des longueurs existantes (100 caractères prénom/nom, 50 téléphone, 255 email), en caractères Unicode plutôt qu'en octets.
- `customer_id` est réellement utilisé : lecture filtrée par tenant et ID, verrou de lecture dans la transaction. Les coordonnées envoyées avec un ID ne modifient jamais la fiche. Les IDs inconnus/intertenant/malformés renvoient le même 404.
- `new_customer` réutilise `customer.CreateCustomerRequest` : prénoms et nom distincts, sans découpage arbitraire. Création dans la transaction existante du dépôt, avec ses articles, numéro, quota et éventuel paiement initial.
- Aucun `ON CONFLICT (tenant_id, phone) DO UPDATE` dans le code des dépôts. Numéros partagés autorisés sur des fiches distinctes.
- Compatibilité de l'ancien couple nom/téléphone pour un numéro sans correspondance. Si ce numéro existe déjà dans le tenant, erreur **409 `customer_selection_required`** : choix explicite nécessaire, jamais de fusion. Correspondance exacte après trim seulement, pas de normalisation E.164 ajoutée implicitement.
- Contrats ambigus (`customer_id` + `new_customer`, ou objet structuré + anciens champs) refusés 400.
- En mémoire, sélection par ID refusée lorsque la fiche ne peut être vérifiée. La création mémoire reste non durable : ce lot ne prétend pas la transformer en persistance.
- Les erreurs techniques de création/modification client ne sont plus renvoyées brutes comme 400 : réponse 500 générique via `httperr`, détails dans les logs serveur. Le conflit d'un ancien schéma unique reste reconnu spécifiquement, pas toute violation d'unicité.

### Migration 000018

- Suppression de l'unicité du téléphone ; maintien de `NOT NULL`.
- Index non unique `(tenant_id, phone)`.
- Clé unique `(tenant_id, id)` sur clients, FK composite depuis commandes, validation des lignes existantes.
- Aucune fusion, suppression ou modification de client historique.
- Up atomique. Si une relation intertenant existe déjà, la validation échoue et annule la migration : corriger avec revue des données, pas en supprimant silencieusement les dépôts.
- Down atomique et verrouillé : **refuse** lorsque des clients partagent un numéro ; aucune fiche sacrifiée pour reconstruire l'unicité.
- Migration versionnée à appliquer une fois, pas un script de synchronisation rejouable sans suivi. Ancien backend incompatible après retrait de la contrainte utilisée par son upsert : prévoir un déploiement coordonné. Ne pas utiliser le montage Compose non corrigé pour ce déploiement (voir R18 de l'audit).

Précontrôle conseillé avant up sur une base existante :

```sql
SELECT o.id, o.tenant_id, o.customer_id
FROM orders o JOIN customers c ON c.id = o.customer_id
WHERE o.tenant_id <> c.tenant_id;
```

Le résultat doit être vide. Après introduction de numéros partagés, privilégier une correction en avant ; tout retour à l'ancien modèle doit faire l'objet d'un traitement métier explicite.

### Adaptation minimale frontend de compatibilité

Le formulaire existant sélectionne et transmet désormais **l'ID**, pas le nom. `Order.customerId` et le mapping HTTP conservent cet ID au retour/rechargement. Ce raccordement évite de casser les dépôts de clients existants lorsque le backend refuse désormais l'ancienne résolution ambiguë par téléphone ; deux homonymes peuvent être distingués techniquement même s'ils partagent le numéro.

**Ce n'est pas la livraison du nouveau formulaire** : recherche, création inline, avertissement de correspondance, libellés de différenciation visuelle, saisie du montant réel et gestion fiable des erreurs restent à faire. Le sélecteur affiche encore le nom seul. Les faux succès locaux de `App.tsx` restent une anomalie connue : ne pas mettre ce lot seul en production comme parcours officiel.

## Fichiers modifiés ou ajoutés dans ce lot

Backend :

- `backend/internal/customer/model.go`
- `backend/internal/customer/service.go`
- `backend/internal/customer/persist.go`
- `backend/internal/customer/handler.go`
- `backend/internal/customer/service_test.go` (nouveau)
- `backend/internal/order/model.go`
- `backend/internal/order/service.go`
- `backend/internal/order/persist.go`
- `backend/internal/order/handler.go`
- `backend/internal/order/customer.go` (nouveau)
- `backend/internal/order/customer_test.go` (nouveau)
- `backend/internal/order/customer_integration_test.go` (nouveau)
- `backend/internal/order/service_test.go` : retrait d'IDs fictifs sans fiche correspondante dans les fixtures de création ; tests d'isolation/finances conservés.
- `backend/migrations/000018_customer_identity.up.sql` (nouveau)
- `backend/migrations/000018_customer_identity.down.sql` (nouveau)

Frontend :

- `src/types/index.ts`
- `src/lib/ordersApi.ts`
- `src/components/NewOrderModal.tsx`
- `src/lib/ordersApi.test.ts` (nouveau, 3 tests)
- `src/components/NewOrderModal.test.tsx` (nouveau, 1 test)

Documentation :

- `MiseJourClient.md` : décisions et progression, sans cocher la livraison financière.
- `docs/depot-phase1-decisions.md` (nouveau).
- `docs/depot-lot1-clients.md` (ce rapport).
- `docs/depot-phase0-audit.md` : lien de suivi ; constats de l'audit initial conservés.
- `docs/api.md`, `docs/database.md` : contrat et migration clients.

`go.mod`, `go.sum`, `package-lock.json` inchangés. Outils, serveur SQL, bases, caches et binaire de test hors Git.

## Environnement de test : blocage initial levé

- Node 22.22.3, npm 10.9.8.
- Go **1.25.5**, installé hors dépôt via la distribution PyPI `go-bin==1.25.5`.
- PostgreSQL **16.2**, installé hors dépôt via `pgserver==0.1.4` ; base jetable `presspro_lot1` sur socket Unix local, aucun accès réseau exposé.
- Extension `uuid-ossp` absente de cette distribution : compilée depuis le code PostgreSQL `REL_16_2` récupéré via GitHub, avec l'en-tête libuuid de util-linux `v2.39` et la bibliothèque système. Aucun remplacement factice de `uuid_generate_v4`.
- Dépendances Go des versions déjà inscrites dans `go.sum` récupérées via les archives GitHub et servies par un proxy fichier local. `GOSUMDB=off` pour éviter une résolution externe indisponible ; **sommes existantes `go.sum` conservées et vérifiées**, `go mod verify` réussi. Aucun ajout de dépendance au produit.
- Migrations 000001–000017 appliquées avant tests de caractérisation, puis 000018 avant tests corrigés. Autre base jetable `presspro_migration18` pour les essais aller-retour.

## Preuves : avant et après

### Régressions reproduites avant correction

Les cinq premiers tests PostgreSQL, écrits avant changement métier, ont tous échoué sur le code initial :

1. `TestDepotClient_SelectionSansEcrasement` : l'ID choisi n'était pas conservé.
2. `TestDepotClient_RefuseIDAutreTenant` : la référence étrangère était ignorée au lieu d'être refusée.
3. `TestDepotClient_TelephoneObligatoire` : absence de téléphone acceptée.
4. `TestDepotClient_AncienContratNeFusionnePas` : aucune erreur, upsert effectué.
5. `TestDepotClient_CRUDNumeroPartage` : numéro partagé rejeté par l'ancienne unicité.

Tous passent après correction et migration. Tests ajoutés ensuite : ID seul, IDs invalides/inconnus, nom composé et recherche de numéros partagés, isolation de recherche, rollback client/paiement/commande/séquence, rejet SQL d'une relation tenant-client étrangère. Les tests de quotas concurrents et multi-instances existants passent également.

### Commandes exécutées et résultats

Les commandes Go ci-dessous ont été exécutées depuis `backend`. Variables locales de validation :

```sh
export PATH=/home/user/.local/presspro-tools/go/bin:$PATH
export GOPROXY=file:///home/user/.local/presspro-proxy
export GOSUMDB=off
export TEST_DATABASE_URL='postgres://user@/presspro_lot1?host=/home/user/.local/presspro-pgdata&port=55432&sslmode=disable'
```

Cette URL désigne uniquement une base jetable locale sans mot de passe, pas un accès production.

| Commande / opération | Résultat vérifié |
| --- | --- |
| `go test -race -cover ./...` avant modification (après correction du PATH) | réussite de la baseline ; certains résultats issus du cache du premier passage |
| `go test -tags=integration -count=1 -run TestDepotClient_ ./internal/order/...` avant correction | 5 échecs reproduits, code 1 |
| `go test -race -count=1 ./...` après correction | tous paquets testés réussis, code 0 |
| `go test -race -tags=integration -count=1 -v ./internal/order/...` | 26 tests de premier niveau réussis, dont 9 nouveaux tests SQL clients et les quotas concurrents, code 0 |
| `go test -tags=integration -count=1 -json ./...` | **172 tests de premier niveau + 73 sous-tests réussis**, aucun échec ; **1 sous-test ignoré**, `TestSuspension_GeleLesModulesMetier/mémoire`, code 0 |
| `go vet ./...` et `go vet -tags=integration ./...` | réussite |
| `go build -o /home/user/.local/presspro-api ./cmd/api` | réussite |
| `go mod verify` | `all modules verified` |
| `gofmt -w` sur fichiers Go du lot | appliqué ; contrôle final des fichiers modifiés sans écart |
| Migrations 000001–000018, `psql -v ON_ERROR_STOP=1 -f ...` | 18 up appliquées sur base jetable |
| Down/up 000018 avec une fiche | réussite, fiche conservée |
| Down 000018 après création d'une seconde fiche au même numéro | **refus attendu** ; les 2 fiches, la FK composite et la version 000018 restent présentes |
| `npm test`, premier passage en parallèle des validations Go | 199 réussis, 1 échec : test existant de notification dans `App.test.tsx` ne trouve pas le bouton à temps |
| `npm test`, second passage sans autres tests lourds et sans modification du test | **200 tests réussis, 27 fichiers**, code 0 |
| `npm run lint` | TypeScript réussi |
| `npm run lint:eslint` | 0 erreur, 48 avertissements, code 0 |
| `npm run format:check` | réussi ; Markdown/backend exclus par la configuration existante |
| `npm run build` | réussi ; avertissement existant de chunk > 500 kB |
| `git diff --check` | réussi |

Comptage Go effectué depuis les événements JSON `pass/skip` portant un champ `Test`, en distinguant les noms avec `/` (sous-tests). Les paquets sans tests ne sont pas comptés comme tests ignorés. La suite intégration complète n'a pas été exécutée avec `-race` ; en revanche la suite unitaire complète et l'intégration commandes l'ont été.

### Échecs intermédiaires et limites

- Premier lancement Go par chemin absolu : erreurs `exec: go: executable file not found in PATH` pour certains paquets sans tests lors du calcul de couverture. PATH corrigé, commande relancée avec succès.
- Première compilation des tests de caractérisation : import inutilisé dans le nouveau test, retiré avant d'obtenir les cinq échecs métier ci-dessus.
- Échec Vitest existant reproduit une fois puis absent au second passage ; sensibilité au timing suspectée, **cause non démontrée**, aucun test désactivé ni assertion assouplie.
- Pas de nouvelle CI GitHub lancée ; ces résultats sont locaux. Pas de tests navigateur d'impression, matériel ou parcours complet de caisse.
- Les droits d'encaissement, l'idempotence, le service/tarif tenant-safe et les faux succès UI identifiés dans l'audit ne sont pas résolus par ce lot.
- Anciennes valeurs `anonyme-*` inchangées pour préserver l'historique ; leur nettoyage nécessite une décision dédiée.
- Les avertissements visuels de correspondance client restent à réaliser, avant de considérer le parcours client terminé.

Logs complets hors Git : `/home/user/audit-presspro-lot1/` (`clients-before-fixed.log`, `order-after.log`, `backend-after.log`, `integration-all.jsonl`, `migration18.log`, `frontend.log`, `frontend-rerun.log`, contrôles npm). Les outils/volumes ne sont pas des livrables.

## Prochain lot et garde de livraison

1. Implémenter les décisions financières confirmées (somme remise distincte, montant affecté, monnaie, montant intégral et restitution à solde nul), avec calcul exact, acteur et tests de concurrence/idempotence.
2. Achever les protections de références catalogue et autorisations avant de finaliser l'interface.
3. Nouveau formulaire client/dépôt et correction des faux succès UI, puis ticket 80 mm et recette complète.
4. PR de revue avant toute intégration ; **aucune fusion automatique dans main**.
