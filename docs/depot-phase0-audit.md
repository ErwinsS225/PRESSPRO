# PRESSPRO — Phase 0 : audit clients, linge, dépôts, paiements et tickets

Date : **12 octobre 2026** (Africa/Abidjan). Référence de suivi : [MiseJourClient.md](../MiseJourClient.md).

> **Suivi ultérieur :** les décisions D1/D2 et la condition de restitution ont été confirmées dans [la phase 1](depot-phase1-decisions.md). Le blocage Go/PostgreSQL a ensuite été levé et le [lot clients](depot-lot1-clients.md) a été testé localement. Le présent audit conserve les constats de son état initial.

## 1. Périmètre, méthode et état vérifié

**Audit de code et validation initiale frontend réalisés ; validation locale Go/PostgreSQL bloquée par l'environnement. Aucune implémentation métier dans cette phase.** Les anomalies décrites ci-dessous proviennent de la lecture des fonctions citées, sauf indication explicite d'un test exécuté. Aucun essai sur des données de production.

- Accès GitHub effectif : `git ls-remote --heads origin main`, `git fetch origin main`, `gh pr list` et `gh run view` ont abouti.
- Branche conservée : `arena/4127deca-presspro`, HEAD `7e432a4f3661df8bb26a08371e8f9512fd688079`.
- `main` distant : `ff15988da596fbb9866e3a61619e98f9ab812dd5`. `git rev-list --left-right --count HEAD...origin/main` donne `0 1` ; `git diff --stat HEAD origin/main` est vide : même arbre suivi, un commit de fusion supplémentaire sur `main`.
- À l'ouverture : seul `MiseJourClient.md` est non suivi ; aucune modification métier préexistante. Aucune PR ouverte lors du contrôle. Pas de changement de branche, commit, push ou fusion effectué.
- Aucun `AGENTS.md` ni README trouvé dans le dépôt avant installation des dépendances. Instructions et références consultées : `PLAN`, `MiseJourClient.md`, `docs/architecture.md`, `docs/database.md`, `docs/installation.md`, `docs/api.md`, rapports/matrice/clôture d'abonnement, fichiers de configuration et CI.
- Environnement : Node `v22.22.3`, npm `10.9.8`. `go` absent ; `psql` et `docker` introuvables dans le PATH. Aucune base de test locale mise à disposition pendant cet audit.
- 17 paires de migrations SQL (`000001` à `000017`) ; examen des tables, contraintes et évolutions relatives au périmètre. Aucune migration appliquée localement.

### Divergences de documentation à ne pas prendre pour des garanties

- `docs/architecture.md` contient encore un schéma « Next.js App Router » et des modules « futurs » : le frontend constaté est **React/Vite**, et les commandes existent.
- L'affirmation « tenant_id dans toutes les tables métier » ne correspond pas à `order_items`, dont l'isolation passe par `order_id`.
- `docs/database.md` décrit encore `order_items.service_id NOT NULL` ; la migration `000008` le rend nullable.
- `docs/subscription-closure.md` indique encore que `PLAN` est absent et la PR non fusionnée : `PLAN` existe et le travail figure dans `main`.
- `PLAN` conserve les anciens prix ; `000017_plan_prix_journal.up.sql` et le rapport de clôture documentent leur correction. Ne pas rétablir les anciens prix.
- Le guide d'installation mentionne Go 1.22+ ; `backend/go.mod` et la CI requièrent **Go 1.25**.

## 2. Architecture réellement constatée

```text
React 19 + TypeScript 6 + Vite 8 (SPA)
  App.tsx : session et listes en état React
  composants : NewOrderModal / CustomersView / OrdersView / ThermalTicketModal
  clients HTTP : ordersApi / customersApi / servicesApi
        ↓ fetch relatif /api/v1/*, Authorization: Bearer
Express (server.ts, port 3000 par défaut)
  Vite en développement / dist en production
  proxy HTTP vers le binaire backend/bin/api (port interne 8085)
        ↓
Go 1.25, Chi, JWT, appartenance, abonnement et périmètre de branche
  handler → service métier → store pgx
        ↓
PostgreSQL 16 : base partagée, filtrage tenant_id, FK, NUMERIC(12,2), TIMESTAMPTZ
  ou repli mémoire décidé au démarrage si Ping échoue
```

Sources : `package.json`, `server.ts::startGoBackend/forwardToGo/startServer`, `backend/cmd/api/main.go::main`, `backend/internal/http/router.go::NewRouter`, `backend/internal/database/store.go`.

Les domaines existants comprennent auth, customer, order, catalog, pricelist, billing, branch, expense, report et notification. **Conserver ce monolithe modulaire.** Les commandes utilisent un service possédant un store SQL facultatif ; les clients exposent une interface `customer.Store`. Les comportements mémoire et SQL ne sont pas équivalents.

### Monnaie et temps existants

- Base : `NUMERIC(12,2)` pour tarifs, commandes, lignes et paiements (`000001`). Go : `float64` ; TypeScript : `number`. Les calculs applicatifs ne sont donc pas encore exacts malgré le type SQL décimal.
- Les tickets texte et ESC/POS arrondissent à l'entier, alors que le schéma accepte deux décimales (`order.Handler.ThermalReceipt`, `escpos.montantFCFA`). Ne pas imposer silencieusement une nouvelle convention.
- Base : `TIMESTAMPTZ` et `NOW()` ; mémoire : dates de création en UTC. `branches.timezone` a pour défaut `Africa/Abidjan` (`000007`), mais le mapping frontend utilise le fuseau du navigateur sans paramètre `timeZone`.

## 3. Flux actuel de création et de paiement

### 3.1 Dépôt depuis le comptoir

1. `NewOrderModal` initialise un client par **nom complet**, pas par ID, et une ligne de quantité 2. Il utilise des valeurs de secours fictives si client/catalogue absent. `handleAddItem` accède directement à `services[0].name` : catalogue vide susceptible de provoquer une exception.
2. `handleSubmit` construit un objet `Order` déjà présenté comme une commande : ID temporaire, numéro aléatoire `CMD-2026-*`, réception « À l'instant », retrait « Dans 2 jours ». Le choix « acompte » calcule **50 %** ; pas de montant réellement reçu saisi.
3. Le composant appelle `onSubmit(newOrder)` puis **ferme immédiatement**, sans attendre la promesse.
4. `ordersApi.createOrderOnBackend` envoie nom/téléphone, lignes avec `service_id: ''`, prix fournis par le navigateur, paiement initial et mode `cash`. Il n'envoie pas `customer_id` ; la date de retrait est remplacée par maintenant + 2 jours.
5. `POST /api/v1/orders` : JWT → appartenance → abonnement → contrôle de branche demandé → `order.Handler.CreateOrder`. Le tenant vient du contexte authentifié. Validation HTTP minimale : nom non vide et au moins une ligne.
6. `Service.CreateOrder` délègue à `dbStore.creerCommande` en mode SQL : lecture du plafond → transaction → verrou/comptage quota → numéro atomique → résolution client → branche → commande → lignes → paiement initial → commit → relecture complète.
7. **Résolution client problématique** : `upsertCustomerTx` ignore `req.CustomerID`, choisit par téléphone et remplace prénom/nom en cas de conflit. Sans téléphone, il crée une valeur `anonyme-*`. La transaction existe, mais la décision d'identité est incorrecte pour le nouveau cahier des charges.
8. L'API retourne une commande complète avec lignes et paiements. `mapBackendOrder` en conserve seulement une partie : historique des paiements, `customer_id`, `service_id`, notes par ligne et dates brutes sont perdus.
9. `App.handleAddOrder` utilise la réponse canonique en cas de succès, **sinon l'objet fictif**, l'ajoute à la liste et ouvre le même ticket officiel. Ni le quota refusé ni une erreur réseau n'empêchent ce faux dépôt local.

### 3.2 Encaissement ultérieur

- `ThermalTicketModal.handleSettle` transmet **tout le solde**, sans montant libre ni historique ; modes affichés : espèces, Wave, Orange Money, MTN MoMo. `card` existe côté backend mais n'est pas proposé ici.
- `App.handleSettleBalance` lance un POST seulement si l'ID ressemble à un UUID, sans attendre le résultat. Il incrémente localement le payé, impose `paymentStatus: 'paid'` et affiche un succès même après refus serveur.
- L'API `AddPayment` accepte plusieurs paiements, rejette `amount <= 0` et les montants supérieurs au solde ; SQL utilise une transaction et `SELECT ... FOR UPDATE`. Le verrou est à conserver.
- `payments.received_by` existe dans le schéma, mais les INSERT de création et d'encaissement ne le renseignent pas et `PaymentRecord` ne l'expose pas.
- Les paiements sont des encaissements enregistrés au comptoir : aucune confirmation automatique d'une transaction Wave/carte par fournisseur ne doit être présumée.

### 3.3 Atelier et ticket

- `App.handleAdvanceOrderStatus` avance réception → lavage → repassage → prêt → remis, puis met à jour localement sans attendre l'API ; le toast de notification ne prouve pas l'envoi effectif.
- `UpdateOrderStatus` valide seulement l'appartenance du nouveau statut à une liste, **pas la transition depuis l'ancien**. Pas de journal des transitions ni de date de remise dédiée dans les migrations examinées.
- Trois rendus à distinguer : ticket de dépôt React (`ThermalTicketModal`), flux ESC/POS (`src/lib/escpos.ts`), ticket texte API (`GET /orders/{id}/ticket`). `CounterReceiptModal` imprime catalogue/devis et ne doit pas devenir un second mécanisme de dépôt.

## 4. Inventaire : acquis, partiel et manquant

| Domaine | Déjà implémenté et réutilisable | Partiel / à compléter | Références |
| --- | --- | --- | --- |
| Clients | CRUD SQL isolé par tenant ; recherche nom/prénom/téléphone/email ; conflit téléphone 409 | Formulaire dépôt fermé, noms non séparés dans le front, création intégrée absente, sélection par ID non exploitée, aucune alerte de correspondance | `customer.Service`, `customer.dbStore.GetCustomers/GetCustomer/CreateCustomer`, `CustomersView`, `customersApi` |
| Téléphone | Règle explicite obligatoire dans le CRUD et `NOT NULL` + unicité en SQL | Dépôt contourne la règle avec faux téléphone ; facultativité et partage d'un numéro à faire décider | `customer.Service.CreateCustomer/UpdateCustomer`, `order.upsertCustomerTx`, migration `000001` |
| Catalogue | Articles, traitements, catégories, unités, prix, activation, durée, express, devis ; grilles versionnées en JSONB | Dépôt réduit tout à un libellé, sans résolution fiable du service/tarif ; kg et m² existent au catalogue mais quantité de dépôt est un entier | `catalog.ServiceItem`, `pricelist.PriceListEntry`, migrations `000002`, `000010` |
| Articles déposés | Plusieurs lignes, quantité, prix, sous-total, `notes` SQL/API ; prix et libellé figés | Pas de couleur/état structurés ; notes de ligne non saisies ni mappées ; prestation/article/catégorie non distingués dans l'instantané | `order.OrderItem/CreateOrderItemInput`, `createOrderItems`, `creerCommande`, types TS |
| Total et numéro | Calcul serveur à partir des lignes ; transaction ; séquence SQL par tenant ; unicité `(tenant_id, order_number)` | Montants navigateur non contrôlés contre catalogue ; calcul flottant ; séquence mémoire volatile et année 2026 en dur | `creerCommande`, `nextOrderNumberTx`, `Service.CreateOrder`, migrations `000001`, `000009` |
| Paiements | Initial et successifs ; journal SQL ; séparation des deux statuts ; refus de dépassement sur paiements ultérieurs ; verrou de ligne | Initial tronqué silencieusement ; acteur absent ; saisie libre/historique UI absents ; mode non validé au service ; correction traçable absente | `order.AddPayment`, `PaymentRecord`, `ThermalTicketModal`, `000001.payments` |
| Dates | Création SQL fiable ; échéance nullable en base | Échéance artificielle ; API renvoie date lisible non ISO ; format invalide ignoré ; aucune remise réelle dédiée | `parseFlexibleDate`, `dueDateString`, `scanOrder`, `ordersApi.mapBackendOrder` |
| Atelier | Six statuts backend dont `cancelled`, notifications après mise à jour | Aucun graphe de transitions ni journal ; `cancelled` absent du type frontend ; pas de statut distinct « attente » | `order.model.go`, `UpdateOrderStatus`, `signalerStatut`, `src/types/index.ts` |
| Ticket | Nom pressing/client, numéro, dates, quantités, sous-totaux, total/payé/solde ; CSS print ; ESC/POS/WebUSB | Prix unitaire absent, libellés tronqués, police très petite, conditions non configurables, parité des rendus et origine persistée non garanties | `ThermalTicketModal`, `escpos.buildTicketBytes`, `Handler.ThermalReceipt`, `src/index.css` |
| Abonnements | Moteur unique de droits, essai 7 jours, expiration, quotas transactionnels, matrice de routes testée | Ne couvre pas à lui seul les rôles financiers, les références croisées des lignes ni les faux succès frontend | `billing/*`, `http/feature_middleware.go`, `order/quota_integration_test.go` |

## 5. Risques prioritaires, sécurité et intégrité

**Bloquant** signifie : à résoudre avant de déclarer le parcours caisse fiable. Il ne signifie pas qu'une exploitation a été exécutée pendant l'audit.

| ID | Niveau | Constat et impact | Preuve de code / opération |
| --- | --- | --- | --- |
| R01 | Bloquant | Faux dépôt/ticket après échec API ; saisie perdue ; aucune prévention des doubles envois | `App.handleAddOrder`, `NewOrderModal.handleSubmit` |
| R02 | Bloquant | Paiement et statut localement confirmés avant réponse ; solde affiché potentiellement faux | `App.handleSettleBalance/handleAdvanceOrderStatus`, `ordersApi.addOrderPaymentOnBackend/updateOrderStatusOnBackend` |
| R03 | Bloquant | Fusion implicite par téléphone et remplacement de nom, y compris depuis un rôle ne pouvant pas modifier la fiche client via PUT ; `customer_id` ignoré | `order.upsertCustomerTx/creerCommande`, rôles des routes `/customers/{id}` dans `NewRouter` |
| R04 | Bloquant | UUID de service accepté sans vérification du tenant ou de l'activité ; prix arbitraire accepté. La FK simple vérifie l'existence, pas le tenant | `creerCommande` → `nullUUID`, migrations `000001`/`000008` ; à reproduire avec deux tenants en intégration |
| R05 | Élevé | FK simples client/dépôt/paiement/branche n'imposent pas la concordance des tenants en SQL ; `order_items` n'a pas de tenant. Les lectures de détails reposent sur le parent préalablement filtré | `000001`, `000007`, `loadOrderDetails`, `orderSelect` ; ce dernier joint le client par ID uniquement |
| R06 | Bloquant | Quantité <= 0 corrigée en 1, libellé vide remplacé en SQL, prix non validé au service, pas de bornes métier. Paiement initial négatif ou excédentaire corrigé sans erreur | `createOrderItems`, `Service.CreateOrder`, `dbStore.creerCommande` ; certains prix invalides ne sont rejetés que par CHECK SQL |
| R07 | Élevé | Calcul en `float64` puis arrondis SQL/ESC-POS : risque de désaccord entre somme des lignes, total et solde | `OrderItem`, `PaymentRecord`, `creerCommande`, `AddPayment`, `escpos.montantFCFA` |
| R08 | Bloquant | Toutes les mutations de commande, y compris paiement, sont ouvertes à tout membre actif ; aucun `RequireRole` dédié. Le contrôle de branche lit header/query, pas le corps, et ne compare pas la branche de la commande consultée | `NewRouter` groupe `/orders`, `RequireBranchScope`, `resolveBranchTx`, `GetOrderByID/GetOrders/AddPayment` ; ne pas confondre isolement intertenant et restriction interne multi-sites |
| R09 | Élevé | Transitions arbitraires entre statuts autorisés, y compris réactivation d'une annulation ; pas de journal/acteur/date de remise. Réactivation susceptible de contourner le quota excluant les annulées | `UpdateOrderStatus`, `billing.requeteCompteFenetre` ; scénario concurrence/quota à tester |
| R10 | Élevé | Pas d'idempotence de création/paiement repérée dans API, service ou schéma. Commit puis relecture peuvent échouer après écriture réussie, entraînant une répétition dangereuse | `creerCommande`, `dbStore.AddPayment`, `router.go`, migrations ; recherche `idempot*` limitée aux essais/catalogue, pas aux encaissements |
| R11 | Élevé | Succès HTTP possible en mémoire sans persistance durable ; le front annonce PostgreSQL sur tout succès. Clients mémoire renvoyés sans ID et non conservés | `main` choix `dbReady`, `customer.Service.CreateCustomer/GetCustomers`, `App.handleAddOrder/handleAddCustomer` |
| R12 | Élevé | Date prévue inventée ou perdue : repli +48h, format `02 Jan à 15:04` sans année, puis parsing JS ; aucune politique d'affichage explicite du fuseau | `dueDateString`, `Service.CreateOrder`, `parseFlexibleDate`, `ordersApi` |
| R13 | Élevé | Changement de compte sans purge/rechargement des commandes, clients, catalogue et ticket actif : ancien tenant encore visible dans le navigateur, voire ticket combiné au nouvel établissement | `App.handleLoginSuccess`, effet d'initialisation `useEffect(..., [])`, rendu `ThermalTicketModal` ; scénario navigateur à automatiser |
| R14 | Élevé | Projection publique d'un ticket expose montants et lignes sur connaissance tenant/commande ; pas de jeton de partage ni expiration. QR navigateur envoie l'URL à un tiers | `order.VerifyTicket`, route `/public/verify/{tenantID}/{orderID}`, `ThermalTicketModal` effet QR ; exception publique existante, décision D7 à clarifier, pas un accès JWT |
| R15 | Élevé | `server.ts::startGoBackend` force `SERVER_ENV=development` même lorsque le front tourne en production, contournant l'exigence de secret de production du binaire | `server.ts`, `backend/cmd/api/main.go` contrôle JWT ; aucun secret ajouté ni reproduit dans ce rapport |
| R16 | Moyen | Erreurs techniques client renvoyées brutes comme 400, ou masquées comme 404 ; classification API insuffisante | `customer.Handler.CreateCustomer/UpdateCustomer/customerErrorStatus/GetCustomer` ; réutiliser `httperr` déjà utilisé par les commandes |
| R17 | Moyen | App essaie encore une connexion automatique à un compte de démonstration codé en dur ; catalogue local de secours non indexé par tenant | `App.initSessionAndServices`, `servicesApi.STORAGE_KEY/getLocalServices/saveLocalServices` ; supprimer le comportement de démo en parcours réel, revoir les consommateurs du cache |
| R18 | Élevé | Montage de tout `backend/migrations` dans l'init PostgreSQL inclut aussi les `.down.sql`, contrairement à la CI qui ne joue que les `.up.sql` | `compose.yaml::db.volumes`, `.github/workflows/ci.yml` ; risque d'initialisation locale à reproduire sur une base jetable avant usage, pas sur une base réelle |

### Garanties existantes à conserver, sans les surévaluer

- `AuthenticatorMiddleware` vérifie le JWT et refuse l'usurpation `X-Tenant-ID` par un utilisateur standard ; l'override tenant du superadmin est une exception explicite.
- `MembershipChecker.IsMember` refuse les membres absents/inactifs en mode PostgreSQL ; les routes clients et commandes filtrent le tenant à la lecture/écriture du parent.
- `AddPayment` sérialise les encaissements SQL et refuse le dépassement du solde. Les modes SQL autorisés sont `cash`, `wave`, `orange_money`, `mtn_momo`, `card`. Le service ne possède cependant pas encore la validation correspondante : SQL refuse par CHECK, mémoire accepte une chaîne arbitraire.
- Les prix/libellés des lignes sont conservés, non recalculés depuis le catalogue à la lecture. En revanche, identité et téléphone client sont joints depuis la fiche vivante : une modification de client affecte l'affichage des anciens tickets.
- Aucun effacement des articles ou paiements n'accompagne le passage à `delivered`. Aucun solde nul exigé actuellement.
- Aucune route de modification/suppression de paiement n'a été trouvée dans `NewRouter`. Ajouter une correction traçable, pas un effacement. La suppression globale d'un tenant par superadmin est une opération distincte déjà existante.

## 6. Abonnements et quotas : règles réellement présentes

Sources : `billing/model.go`, `service.go::defaultMatrix/EffectivePlan`, `entitlements.go::GetSubscription/EffectiveEntitlements`, `quota.go`, `persist.go`, migrations `000003`–`000006`, `000012`, `000014`, `000016`, `000017`, `http/feature_middleware.go`, `cmd/api/main.go` câblage du garde.

| Droit | FREE | PRO | BUSINESS / essai actif |
| --- | --- | --- | --- |
| `orders.create` | activé | activé | activé |
| `orders.monthly.max` | 50 | sans plafond | sans plafond |
| `users.max` | 2 | 5 | 15 |
| `catalog.price_lists` | 1 | 3 | 10 |
| `branches.multi_site` | 1 | 1 | 3 |
| Rapports avancés, dépenses avancées, pricing pro, envoi notifications | désactivés | activés | activés |
| Rapports consolidés | désactivés | désactivés | activés |
| `audit.advanced` | activé | activé | activé |

- Prix issus des migrations après correction `000017` : FREE 0, PRO 10 000, BUSINESS 25 000 XOF/mois. **Ce sont les valeurs du dépôt, pas une lecture de la base de production. Aucun tarif à modifier.** Le catalogue SQL reste autorité ; mémoire ne fournit pas de prix fictif.
- Essai de 7 jours donnant les droits BUSINESS ; expiration atomique vers FREE/ACTIVE, journalisée en SQL, sans purge (`GetSubscription`/`ExpireTrial`).
- Fenêtre quota : tranches de 30 jours ancrées sur `trial_started_at`, sinon création de l'abonnement, sinon du tenant. Ce n'est ni le mois civil ni une fenêtre mobile « maintenant moins 30 jours ». Renouvellement de fenêtre déterministe sans remise à zéro au changement de plan.
- `QuotaGuard.VerifierCommandeTx` applique verrou consultatif par tenant et comptage transactionnel. Le garde est branché sur le service de commandes, pas un contrôle frontend.
- Le moteur de création contrôle `orders.monthly.max` ; `orders.create` existe mais n'est pas vérifié séparément sur la route. Aucune différence actuelle de matrice pour cette clé : ne pas inventer un nouveau droit de dépôt/paiement/ticket.
- Les statuts financiers des dépôts ne sont **pas** les statuts d'abonnement. Aucun renouvellement commercial par paiement fournisseur/webhook n'a été repéré dans les routes ; ne pas inventer une confirmation de paiement SaaS.
- Une rétrogradation ne purge pas les données ; les lectures historiques ne sont pas soumises au quota de création. Une **suspension** est différente : `RequireActiveSubscription` englobe aussi les lectures métier. `IsTenantSuspended` laisse passer une erreur de lecture de l'abonnement (fail-open) : risque à tracer, ne pas présenter ce garde comme fail-closed.
- La suite tronquée de la section 12 du nouveau prompt reste à préciser. Le `PLAN` existant §3.3 aide à cadrer la conservation après rétrogradation, mais ne remplace pas une confirmation de la consigne incomplète.

## 7. Ticket 80 mm : examen et limites de validation

- `ThermalTicketModal` imprime nom du pressing, ville, téléphone, numéro, dates, client, lignes, notes globales, total/payé/reste et QR. Adresse complète non affichée ; téléphone du pressing remplacé par un placeholder si absent. Plusieurs tailles de texte sont de 8–10 px.
- Le tableau ne montre pas le **prix unitaire**. `truncate` coupe les désignations longues au lieu de les reporter sur plusieurs lignes.
- `src/index.css::@media print` masque globalement via `visibility`, rend les conteneurs reçus visibles et impose 80 mm avec padding 2 mm/3 mm. `@page` reste `size: auto` ; les contraintes des ancêtres de modale (hauteur/overflow) et la pagination longue doivent être testées en navigateur.
- ESC/POS : largeur de 42 colonnes, CP437, aperçu texte et WebUSB déjà disponibles. Le flux tronque les libellés, arrondit les montants, n'embarque ni prix unitaires ni notes du dépôt, et imprime une URL de vérification plutôt qu'une commande QR graphique.
- API texte : relit la commande côté backend, mais tronque le libellé à 20 **octets** (risque UTF-8), sans prix unitaires ni notes ni configuration des conditions.
- `CounterReceiptModal` est un rendu tarifaire/devis distinct avec conditions et coordonnées de démonstration en dur ; aucune configuration tenant de conditions de conservation/restitution identifiée dans le schéma examiné.
- Les **16 tests `escpos.test.ts` ont passé** dans la suite frontend. Ils ne prouvent ni l'impression navigateur, ni la compatibilité physique d'une imprimante.
- **Aucun rendu navigateur imprimé/PDF, aucune impression physique et aucun test WebUSB matériel exécutés dans cette phase.** Le QR tiers n'a pas été sollicité.

## 8. Tests existants et validations exécutées

### 8.1 Couverture trouvée, sans confusion avec de nouveaux tests

- `order/service_test.go` : création, liste vide d'articles, statut inconnu, paiement, isolation et dépassement du solde. `TestUpdateOrderStatus_Success` accepte expressément réception → prêt : les nouvelles règles nécessiteront de revoir ce test après décision métier.
- `order/quota_integration_test.go` : dernière place concurrente, plusieurs instances, rollback, isolation du verrou tenant. Le test de prix négatif repose aujourd'hui sur les CHECK SQL, pas sur une validation métier.
- `http/route_matrix_test.go` et `route_matrix_integration_test.go` : inventaire de routes, accès sans JWT, rôles, accès croisé aux ressources, usurpation tenant, suspension, FREE et downgrade ; les cas clients mémoire sont limités car ce module ne stocke rien.
- `billing/*test.go` : essai/expiration, droits, période, quotas, tarifs, catalogue et journal des changements.
- Frontend : 25 fichiers ; `App.test.tsx` ne contient que deux scénarios de fumée/notification. Aucun fichier dédié `NewOrderModal.test.tsx`, `ThermalTicketModal.test.tsx`, `ordersApi.test.ts` ou `customersApi.test.ts` trouvé avant ce chantier. Aucun test dédié dans `backend/internal/customer` ; les clients sont toutefois couverts partiellement par la matrice HTTP.

### 8.2 Résultats de cette session

| Commande réellement exécutée | Résultat | Limites / avertissements |
| --- | --- | --- |
| `npm ci --legacy-peer-deps` | code 0 ; 492 paquets ajoutés, audit npm 0 vulnérabilité signalée | avertissements de dépréciation ; lockfile inchangé |
| `npm test` | code 0 ; **196 tests réussis, 25 fichiers** | avertissements Recharts/jsdom et erreurs réseau simulées attendues dans des tests ; pas de parcours réel PostgreSQL |
| `npm run lint` | code 0 | vérification TypeScript (`tsc --noEmit`) |
| `npm run lint:eslint` | code 0 ; **0 erreur, 48 avertissements** | budget existant de 50 ; ne signifie pas « zéro avertissement » |
| `npm run format:check` | code 0 | `.prettierignore` exclut notamment `backend` et `*.md` |
| `npm run build` | code 0 ; build Vite réussi | avertissement chunk > 500 kB ; ne compile pas le backend |
| `cd backend && go test -race -cover ./...` | **code 127** : `go: command not found` | aucun test Go local exécuté avec succès |
| `gh run view 38216977355 --json headSha,conclusion,event,jobs,url` | run antérieur de `main` : succès, jobs backend + frontend | preuve CI existante, pas une nouvelle CI de cette branche ni une couverture des anomalies nouvellement relevées |

Run vérifié : https://github.com/ErwinsS225/PRESSPRO/actions/runs/38216977355, SHA `ff15988da596fbb9866e3a61619e98f9ab812dd5`. Étapes migrations up, `go vet`, tests avec race detector et intégration PostgreSQL affichées **success** par GitHub. Aucun nombre de tests backend n'est déduit de cette seule réponse.

Extraits reproductibles et commandes : [depot-phase0-validation.md](depot-phase0-validation.md). Logs npm complets conservés hors Git dans `/home/user/audit-presspro-phase0/`.

**Non exécutés localement :** Go build/vet, intégration SQL, migrations up/down, parcours HTTP réel, tests de concurrence des paiements, tests navigateur/impression. Ils restent indispensables avant livraison et ne sont pas cochés comme validations métier réussies.

## 9. Modifications et migrations à prévoir, pas encore réalisées

### 9.1 Fichiers concernés

| Lot | Fichiers à modifier ou compléter | Intention |
| --- | --- | --- |
| Contrat et clients | `customer/{model,service,persist,handler}.go`, `order/{model,service,persist,handler}.go` | sélection explicite par ID ou nouveau client structuré, erreurs métier, téléphone selon décision, transaction sans fusion implicite |
| Validation articles et finances | `order/{model,service,persist}.go`, intégration lecture `catalog`/`pricelist` si nécessaire | validation uniforme, tenant/service actif, prix autorisé, calcul exact, instantanés et limites |
| Sécurité | `http/{router,access_middleware}.go`, `cmd/api/main.go`, `server.ts`, éventuellement `billing/admin.go` | rôles validés, périmètre réel de ressource, refus de faux succès durables, environnement production fiable |
| Historique et reprise | `order/{model,service,persist,handler}.go`, migrations additives | acteur encaissement, idempotence, journal statuts, remise réelle ; correction financière après cadrage |
| Frontend | `src/types/index.ts`, `lib/{ordersApi,customersApi,servicesApi}.ts`, `App.tsx`, `NewOrderModal.tsx`, `CustomersView.tsx`, `OrdersView.tsx`, permissions si nécessaire | contrats typés, saisie conservée, ID client/service, paiement réel, réponses canoniques, purge au changement de tenant |
| Ticket | `ThermalTicketModal.tsx`, `lib/escpos.ts`, `index.css`, `order/handler.go` | source enregistrée, unités/prix/notes, dates brutes, wrapping, lisibilité, cohérence des rendus |
| Validation et docs | nouveaux tests dédiés + suites existantes, `.github/workflows/ci.yml` si nouveau paquet d'intégration, `docs/{api,architecture,database,installation}.md`, `compose.yaml` | preuves de sécurité/transactions, migration sûre, documentation concordante |

### 9.2 Migrations candidates (après `000017`, numéros définitifs au moment de l'implémentation)

1. **Clients** : si téléphone facultatif approuvé, autoriser absence réelle (NULL) et adapter les scans ; définir si plusieurs clients peuvent partager un téléphone. Maintenir ou remplacer l'unicité en conséquence, sans fusion ni nettoyage aveugle des anciens `anonyme-*`. Nom/prénoms existent déjà : ne pas créer un second fichier clients.
2. **Intégrité tenant** : clés uniques/composites et FK tenant-client, tenant-commande-paiement, tenant-branche ; stratégie de tenant des lignes et référence tenant-service. Préserver la possibilité historique `service_id NULL` et la sémantique de suppression catalogue. Précontrôler les lignes historiques avant validation des contraintes.
3. **Articles et dates** : ajouter seulement les attributs nécessaires aux instantanés (article/prestation/catégorie/couleur/état), `delivered_at` si retenu. `due_date` et notes existent déjà. Ne pas inventer la date réelle des anciens dépôts : garder inconnue.
4. **Journal de statut** : table de transitions avec tenant/dépôt, ancien/nouveau statut, acteur, instant et observation ; pas d'historique passé inventé.
5. **Idempotence** : table/contrainte par tenant + opération + clé, empreinte de requête et référence du résultat, écrites dans la même transaction ; rejouer la même réponse, refuser une clé réutilisée avec contenu différent. Couvrir création et paiements. Politique de conservation à définir.
6. **Paiements** : réutiliser `received_by` avant d'ajouter une colonne équivalente ; acteur historique inconnu conservé NULL. Contrepassation/annulation liée à l'original, motif et auteur si correction autorisée. Adapter rapports et calcul du cumul valide, pas seulement l'écran.
7. **Montants** : privilégier calcul exact à l'échelle SQL existante (décimal ou entiers internes de centièmes avec contrôle de dépassement), sans modifier silencieusement le JSON ou les montants historiques. Une migration de type monétaire n'est pas présumée nécessaire ; les contraintes additionnelles dépendent des règles validées.
8. **Paramètres ticket** : vérifier la source des coordonnées tenant et définir les seuls paramètres réellement nécessaires pour les conditions ; pas de texte juridique inventé.

Toutes migrations : préaudit des données, sauvegarde selon procédure de déploiement, test sur base jetable, compatibility des lecteurs, contrôle up/down et stratégie explicite pour les données nouvellement ajoutées. Ne pas modifier les migrations historiques en place.

## 10. Plan de tests à ajouter et risques de régression

### Backend / SQL / sécurité

- Choix client existant sans écrasement ; nouveau client structuré ; homonymes ; téléphone partagé/absent selon règle ; refus de client d'un autre tenant ; absence de client orphelin si dépôt échoue.
- Quantités 0/négatives/hors limite, lignes vides, prix négatifs/trop précis/hors bornes ; totaux exacts ; service inactif ou d'un autre tenant ; prix historique inchangé après modification catalogue.
- Initial zéro/partiel/intégral ; refus explicite initial excédentaire/négatif ; tous les modes autorisés et mode invalide ; plusieurs règlements avec acteur ; concurrence et journal égal au cumul valide ; rollback complet.
- Deux requêtes de même clé d'idempotence, succès suivi de réponse perdue, contenu différent sous même clé, clés identiques de tenants différents ; aucune consommation supplémentaire de quota/numéro/paiement.
- Numérotation concurrente, rechargement après redémarrage ; retour date ISO nullable et fuseau ; transitions autorisées/interdites, observation et acteur, restitution avec dette selon règle, refus ou contrôle de réactivation d'annulation.
- Refus intertenant sur toutes opérations ; branche du corps/header/ressource ; changement de rôle, rôle opérateur et caisse ; tests SQL des contraintes composites.
- FREE/PRO/BUSINESS/essai/expiration/suspension : quota inchangé, lectures historiques, encaissement d'un dépôt existant après quota atteint/downgrade selon règle ; aucun droit/tarif commercial inventé.
- Mode mémoire/non prêt : aucun document faussement présenté comme persistant ; panne post-commit traitée sans doublon.

### Frontend / impression

- Formulaire avec catalogue/client vides, recherche/sélection par ID, création client en ligne, avertissement de correspondance et conservation des articles.
- Montant réellement encaissé (dont 0), pas de règle de 50 %, historique ; succès canonique uniquement après API.
- Validation/401/403/402/409/500/réseau : pas d'ajout local, pas d'impression officielle, saisie conservée, double clic bloqué, reprise idempotente.
- Changement de tenant : suppression des anciens états/caches, annulation ou invalidation des réponses en vol, aucun ticket mélangeant deux tenants.
- Ticket : noms longs, nombreuses lignes, notes, PU/sous-total, téléphone et date absents, cohérence React/API/ESC-POS, impression navigateur/PDF 80 mm sans menus ; matériel ensuite.

### Régressions à surveiller

- Tests qui caractérisent les actuels raccourcis réception → prêt ou IDs mémoire non UUID ; les modifier seulement après validation du contrat cible.
- Rapports et graphiques qui somment `payments.amount`/`orders.paid_amount` : une correction financière exige une définition unique des paiements valides (`report/persist.go`, `src/lib/analytique.ts`).
- Notifications qui consomment les transitions `ready`/`delivered` et les coordonnées ; éviter doublons et faux envois lors des reprises.
- Catalogues kg/m² et prestations gratuites (prix 0 légal actuellement) ; ne pas restreindre arbitrairement le modèle.
- Suppression/édition de service : instantanés historiques et `ON DELETE SET NULL` à préserver.
- Migrations de relations composites sur les données existantes ; vieux clients API qui envoient encore nom/téléphone seuls ; explicitement versionner ou gérer la transition.
- Projection publique de vérification : données et garanties à décider sans casser silencieusement tous les QR déjà imprimés.

## 11. Plan d'exécution ordonné et décisions de phase 1

1. **Phase 0 (ce rapport)** : revue statique, baseline frontend et preuve CI existante ; limites backend local explicites.
2. **Phase 1** : fixer les contrats/règles ci-dessous ; préparer Go 1.25 + PostgreSQL 16 jetable et vérifier le processus de migrations. Écrire les tests de caractérisation/régression ciblés avant corrections.
3. **Phase 2** : backend client/dépôt atomique, sélection tenant-safe, validation catalogue/montants, date fiable, contrat d'idempotence et contraintes additives. Conserver la séquence et le quota transactionnels existants.
4. **Phase 3** : encaissement réel, contrôles initiaux/ultérieurs cohérents, acteur, historique, concurrence et corrections traçables après validation. Exécuter les tests SQL et de sécurité avant finalisation UI.
5. **Phases 4–5** : transitions et journal, rôles, portée branche, abonnements inchangés. Sécurité intégrée dès les lots backend, pas repoussée à une simple vérification finale.
6. **Phase 6** : brancher les formulaires sur contrats canoniques, gérer erreurs/reprises, supprimer le succès local et les états de l'ancien tenant.
7. **Phase 7** : ticket officiel 80 mm enrichi, parité des rendus et test navigateur ; validation physique explicitement distincte.
8. **Phases 8–9** : suites complètes, CI nouvelle de la branche, recette caissier, documentation, PR avec preuves. **Aucune fusion automatique.**

### Décisions métier à confirmer avant les changements concernés

| Réf. | État actuel vérifié | Décision attendue / proposition soumise à validation |
| --- | --- | --- |
| D1 — téléphone | Obligatoire dans CRUD/SQL, contourné au dépôt | Autoriser les clients sans téléphone ? Autoriser plusieurs personnes avec le même numéro ? Dans tous les cas, aucune fusion automatique. |
| D2 — encaissement | Paiement nul/partiel/intégral possible ; trop-perçu refusé seulement après création | Conserver ces trois possibilités et appliquer également un refus explicite de trop-perçu au dépôt ; supprimer les 50 % imposés. Confirmer limites et précision monétaire. |
| D3 — prix appliqué | Prix navigateur accepté, catalogue configurable | Prix catalogue obligatoire ou dérogation autorisée à certains rôles avec motif ? Cas devis, gratuité et unités non entières à cadrer. |
| D4 — atelier/restitution | Remise avec dette possible ; tous sauts acceptés | Valider le graphe réutilisant `received/washing/ironing/ready/delivered`, besoin d'attente distincte, annulation/réouverture ; ne pas bloquer la remise impayée sans accord. |
| D5 — autorisations | Tout membre peut déposer/encaisser/changer le statut | Définir rôles de dépôt, encaissement, correction et atelier ; portée multi-sites obligatoire ou optionnelle selon poste. |
| D6 — date et ticket | +2 jours implicites ; pas de conditions configurées | Échéance libre facultative ou délai métier calculé ? UTC au stockage, affichage Africa/Abidjan ou fuseau de branche proposé. Conditions réelles à fournir. |
| D7 — erreurs et partage | Repli mémoire utilisé comme succès ; vérification publique par IDs | Refuser les dépôts officiels sans DB ; politique de partage QR, historique des QR existants et suppression du tiers QR à décider. |
| D8 — abonnements | Conservation après downgrade déjà documentée, suspension distincte | Compléter la fin tronquée de la section 12 ; conserver les plans/prix existants. |

## 12. Livraison de cette phase

Fichiers documentaires uniquement :

- `docs/depot-phase0-audit.md` : ce rapport.
- `docs/depot-phase0-validation.md` : relevé des validations/extraits.
- `MiseJourClient.md` : suivi de phase 0 et journal mis à jour ; phases d'implémentation non cochées.

Aucun fichier métier, test applicatif, migration, prix ou droit d'abonnement modifié. Les 196 tests existants réussis ne suffisent pas à valider les exigences nouvelles. **Le parcours cible n'est pas encore livré.**
