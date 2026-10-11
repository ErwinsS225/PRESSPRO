# PressingOS — Journal des correctifs et améliorations

**Session du 11 octobre 2026** — suite au rapport `RAPPORT_PROJET_PRESSPRO.md`.
Chaque correctif a été vérifié en exécution réelle (PostgreSQL 16, binaire Go compilé, appels HTTP).

---

## 1. Correctifs bloquants (P0)

### A1 — Catalogue inaccessible en mode PostgreSQL (`GET /api/v1/services` → 500) ✅
- **Cause** : `catalog.NewStore()` construisait `&Service{store: …}` sans logger ni index mémoire ; `GetServices`/`GetServiceByID` journalisaient en première ligne → panique *nil pointer dereference*, rattrapée par le middleware `Recoverer` (500 vide).
- **Correctif** :
  - fabriques uniques `newService()` / `defaultSeedItems(tenantID)` ; `NewStore()` et `NewService()` passent désormais par elles (`backend/internal/catalog/service.go`, `persist.go`) ;
  - correction identique appliquée par précaution à `pricelist.NewStore()`.
- **Vérification** : `GET /api/v1/services` et `GET /api/v1/services/{id}` → **200** avec les données PostgreSQL réelles (5 prestations du tenant de démo, 49 pour un nouveau pressing).
- **Tests** : `TestNewStore_RuntimeInitialization` (non-régression, échoue sans le correctif), `TestIntegration_DBReadServices` (PostgreSQL réel).

### A2 — Le repli mémoire annoncé ne s'activait jamais ✅
- **Cause** : `main.go` décidait du mode persistance sur `dbPool != nil && dbPool.Pool != nil` (création du pool, sans `Ping`). Base présente mais injoignable → 500 sur tous les endpoints métier, logs mensongers.
- **Correctif** : décision prise sur le résultat du `Ping` (`dbReady`) ; en échec, bascule complète en mode mémoire + message explicite.
- **Vérification** (binaire lancé avec une base arrêtée) : `/health` 200, `/ready` 503, `GET /api/v1/orders` **200 (données mémoire)**, `/api/v1/services` **200**, `/me/entitlements` **200** — au lieu de 500 partout.

### A3 — Surpaiement accepté (écriture comptable incohérente) ✅
- **Cause** : le solde était plafonné au total (`paid_amount = total`) mais l'encaissement réel était enregistré intégralement dans `payments` (999 999 F encaissés sur une commande de 1 500 F).
- **Correctif** : nouvelle erreur métier `ErrPaymentExceedsDue` ; tout encaissement supérieur au solde dû est refusé **avant** écriture, en mémoire (`order/service.go`) comme en SQL (`order/persist.go`, sous verrou `FOR UPDATE`).
- **Vérification** : paiement de 999 999 F → **400** `« le montant dépasse le solde restant dû de la commande »` ; paiement du solde exact → 200 ; aucun mouvement enregistré après un refus.
- **Test** : `TestAddPayment_ExceedsRemainingDue`.

---

## 2. Sécurité

### A4 — Secret JWT obligatoire en production ✅
`SERVER_ENV=production` + `JWT_SECRET` absent ou < 32 caractères → **le serveur refuse de démarrer** (vérifié). Hors production, un secret de développement est utilisé avec un avertissement dans les logs.

### A5 — Fuite d'informations dans les erreurs ✅
Nouveau paquet `internal/httperr` : messages métier conservés (`400`/`402`/`409`), erreurs techniques (SQL, réseau) journalisées côté serveur et remplacées par un `500` générique. Appliqué à `order`, `catalog`, `customer`, `pricelist`, `billing`, `auth`.
- **Avant** : `500 {"error":"liste des commandes: failed to connect to host=127.0.0.1 user=pressing_app …"}`
- **Après** : `500 {"error":"impossible de récupérer les commandes"}` (+ détail dans les logs serveur).

### A6 — Aucune protection contre la force brute ✅
Nouveau paquet `internal/ratelimit` (fenêtre glissante, sans dépendance) branché sur `POST /api/v1/auth/login` : 10 tentatives / 5 min par couple (IP, email), 40 / 5 min par IP → **429 + `Retry-After`**. Une connexion réussie réinitialise le compteur, et **un mot de passe correct reste accepté même plafond atteint** (l'utilisateur légitime n'est jamais enfermé dehors).
- **Vérification** : 10 échecs → 401, 11ᵉ → **429** ; autre compte non pénalisé ; mot de passe correct → **200** malgré le plafond.
- **Tests** : `TestLoginRateLimiting`, `TestLoginRateLimiting_PerIP`, plus 4 tests unitaires du limiteur.

### A9 — Codes HTTP normalisés ✅
Un accès à une commande d'un autre établissement renvoie désormais **404 « commande introuvable »** (au lieu de 400 sur les paiements), sans révéler l'existence de la ressource. Les erreurs techniques remontent en 500 au lieu d'être maquillées en 404.

### A16 — Contexte d'établissement vide → 500 au lieu d'erreur propre ✅ *(nouveau, détecté par la vérification finale)*
- **Constat** : un **superadmin** (jeton sans `tenant_id`) accédant à une route multi-tenant déclenchait une erreur SQL `invalid input syntax for type uuid: ""` → **500**. Bug du même type sur toute route exigeant un contexte d'établissement.
- **Correctif** :
  - le middleware accepte désormais un `X-Tenant-ID` **explicite et bien formé** comme contexte de travail pour un superadmin (administration SaaS ciblée), sinon
  - toutes les routes multi-tenant (`order`, `catalog`, `customer`, `pricelist`) répondent **403 « un contexte d'établissement est requis pour cette opération »**.
- **Vérification** : superadmin sans header → **403** (plus de 500) ; avec `X-Tenant-ID` d'un établissement ciblé → **200** (consultation de la commande) ; X-Tenant-ID étranger pour un propriétaire → **403** (inchangé).

---

## 3. Fonctionnalités

### A13 / Onboarding — un nouveau pressing démarre opérationnel ✅
- **Avant** : `seedDefaultServices()` était un stub vide et l'inscription ne créait ni catalogue ni grille tarifaire ; un pressing fraîchement inscrit avait un catalogue vide en production.
- **Après** :
  - seed mémoire réel par tenant, avec identifiants préfixés par établissement (prestations réellement distinctes et attribuables) ;
  - hook d'onboarding `auth.SetOnTenantRegistered(...)` (assemblage dans `main.go`) qui, après inscription :
    1. crée la **grille tarifaire par défaut** du catalogue (`catalog.SeedTenantDefaults`, idempotent, transactionnel) ;
    2. crée la **grille tarifaire principale** (`price_lists`) et fige sa **version 1 active** (snapshot du catalogue).
- **Vérification** (nouveau pressing « Pressing Onboarding Test » en PostgreSQL) : 1 appel `register-tenant` → **49 prestations**, **1 grille « Grille principale »**, **1 version active avec 49 entrées figées**, 1 abonnement d'essai. L'établissement peut enregistrer une commande immédiatement.
- **Test** : `TestSeedDefaultServices_NewTenant`.

### A10 — Quotas lisibles ✅
`remaining` ne peut plus être négatif (affichage « reste 0 » quand le plafond est dépassé, au lieu de « -6 »).

### Bug supplémentaire détecté par le test d'intégration ✅
En mode PostgreSQL, une prestation inconnue (ou appartenant à un autre tenant) remontait l'erreur brute pgx `« no rows in result set »` au lieu de `ErrServiceNotFound` → réponse 500 au lieu de 404. Corrigé dans `catalog/persist_logics.go` (traduction `pgx.ErrNoRows`).

---

## 4. Qualité et industrialisation

| Élément | Avant | Après |
|---|---|---|
| Tests Go | 32 | **41** (unitaires + 2 sur la limitation de débit) + 1 test d'intégration |
| Tests d'intégration PostgreSQL | 0 | `backend/internal/catalog/persist_integration_test.go` (`-tags=integration`, `TEST_DATABASE_URL`) |
| CI/CD | absente | GitHub Actions : `go vet`, `go test -race -cover`, migrations + tests d'intégration sur PostgreSQL 16, `tsc --noEmit`, `vite build` |
| Version applicative | incohérente (0.2.0 / 0.3.0 / 0.4.0) | **0.5.0-hardening** partout (log de démarrage, `/api/v1/system/info`, docs) |
| Documentation | désynchronisée | `docs/api.md` (conventions d'erreur, limitation de débit, onboarding), `docs/installation.md` (ports réels, `JWT_SECRET`, tests, `--legacy-peer-deps`), `docs/architecture.md` (démarrage, `httperr`, `ratelimit`, onboarding) |
| Frontend | avertissement Vite `__dirname` | `import.meta.dirname` (plus d'avertissement au build) |

---

## 5. Session 2 — durcissement, monétisation et console éditeur

### Fail-closed du contrôle d'appartenance
Le contrôle #2 (appartenance) **refuse désormais par défaut** dès qu'une base est fournie :
appartenance absente ou suspendue, utilisateur inconnu de la base **et** non déclaré comme compte de démonstration, identifiant hors format UUID, ou erreur de base → **403**.
Le mode vitrine est préservé sans faille générique : les comptes embarqués sont reconnus **nominativement** (`auth.Service.IsDemoAccount`).
- **Vérification live** : appartenance passée en `suspended` en base → 403 (avant : 200) ; ligne d'appartenance supprimée → 403 ; rétablie → 200 ; comptes de démonstration opérationnels.

### Fonctionnalités payantes réellement gardées
- `RequireFeature` (droit booléen) : `POST /price-lists/{id}/versions`.
- **`RequireFeatureQuota`** (nouveau, plafond consommé mesuré sur les données réelles : `orders.monthly.max`, `users.max`, `catalog.price_lists`) : `POST /price-lists`.
- **Vérification live** : établissement gratuit (1 grille autorisée, 2 en base) → **402** « plafond atteint (1) — passez à une formule supérieure » avec `feature`, `usage`, `upgrade_required` ; établissement en essai (niveau BUSINESS, 10 grilles) → **201**. Les **lectures restent gratuites** après rétrogradation.

### Espace SuperAdmin branché sur les données réelles
| Avant | Après |
|---|---|
| 5 pressings fictifs codés en dur dans le composant | `GET /admin/tenants` — annuaire réel (10 puis 13 établissements en base) |
| MRR « 265 000 » et « 85 % de recouvrement » statiques | `GET /admin/stats` — MRR = somme des formules ACTIVE, répartitions par statut et par formule, `data_source` |
| Bouton Suspendre purement local | `PATCH /admin/tenants/{id}/status` — décision réelle **et effective** |
| Bouton « Enregistrer un Pressing » local | `POST /auth/register-tenant` — inscription réelle (catalogue + grille + essai) |
| Badge « 12 pressings » figé | libellé neutre « Éditeur » |

**Suspension effective** : les modules métier de l'établissement suspendu répondent `402` (catalogue, commandes, clients, grilles), tandis que `/tenant/current`, `/me/entitlements` et l'authentification restent ouverts pour que le pressing constate sa situation. Le superadmin n'est jamais bloqué. La décision et sa trace (`subscription_events`) sont écrites **dans une seule transaction** — un premier essai avait modifié le statut tout en renvoyant 500 (journal en échec) : corrigé.

### Bugs trouvés et corrigés pendant cette session
| Bug | Symptôme | Correctif |
|---|---|---|
| Contexte d'établissement vide | superadmin sans `X-Tenant-ID` → erreur SQL `uuid ""` → **500** | `X-Tenant-ID` explicite accepté pour un superadmin, sinon **403** explicite sur toutes les routes multi-tenant |
| Journal d'audit non-UUID | « Suspendre » → **500** (`invalid input syntax for type uuid`) | identifiants mémoire consignés dans le motif de l'événement, jamais insérés dans une colonne UUID |
| Décision non transactionnelle | statut modifié malgré une réponse 500 | `UPDATE` + trace dans la même transaction |
| Doublon de grille | 2e grille du même nom → **500** `duplicate key … SQLSTATE 23505` | erreur métier `ErrNameExists` → **409** « une grille de ce nom existe déjà » (base et mémoire) |

### Frontend : performance et tests
- **Bundle découpé** : 602 kB en un seul lot → **250 kB applicatif** + `vendor-react` 219 kB + `vendor-motion` 125 kB + `vendor-icons` 13 kB (cache navigateur indépendant, avertissement « > 500 kB » supprimé).
- **Vitest + Testing Library** installés (`npm test`) : **14 tests** — client API éditeur (jeton, 403, erreurs réseau, 409, libellés) et vue SuperAdmin (refus d'accès, affichage des données réelles, message d'indisponibilité au lieu de faux chiffres).
- **CI** : `npm test` ajouté, plus une étape Go ciblant les paquets des correctifs.

**Compteurs** : 50 tests Go (+ 1 test d'intégration PostgreSQL), 14 tests frontend, 34 + 17 + 14 = **65 assertions live** vertes.

## 6. Session 3 — module Équipe : comptes utilisateurs gardés par la formule (`users.max`)

### Endpoints livrés
| Méthode | Route | Rôle | Comportement |
|---|---|---|---|
| GET | `/api/v1/users` | owner, manager | Liste des comptes de l'établissement (isolation par le `tenant_id` du jeton). |
| POST | `/api/v1/users` | owner, manager | Création d'un employé **transactionnelle** : `users` + `tenant_memberships` + `branch_memberships` (établissement principal). Gardée par `RequireFeatureQuota("users.max")`. |
| PATCH | `/api/v1/users/{id}/status` | owner, manager | Activation / désactivation, répercutée sur l'appartenance dans la même transaction. |

Le module est placé dans le groupe **métier** : un abonnement suspendu le gèle (`402`), alors que `/tenant/current` et `/me/entitlements` restent consultables. L'ancienne route `GET /api/v1/auth/users` (accessible à tout compte authentifié) est retirée au profit de la route gardée par rôle.

### Décisions structurantes
- **Fail-closed sur l'appartenance** : la désactivation écrit `tenant_memberships.status = 'suspended'` ; un jeton déjà émis perd donc l'accès aux routes métier (`403`) en plus du refus de connexion (`403` « compte désactivé »).
- **Le quota compte tous les comptes du pressing, propriétaire inclus** : formule gratuite 2, PRO 5, BUSINESS 15 (un établissement en essai dispose du niveau BUSINESS). Dépassement → `402` avec `feature: users.max` et `usage`.
- **Rôles attribuables** : `manager`, `cashier`, `operator` uniquement — le rôle `superadmin` de l'éditeur est refusé (`400`) ; le statut est validé (`active`/`inactive`).
- **Protections d'interface et d'API** : impossible de modifier son propre statut (`400`) ; les comptes `owner`/`superadmin` ne sont pas désactivables depuis le module ; email normalisé en minuscules, doublon → `409`.
- **Onboarding cohérent** : un employé créé sans appartenance serait refusé partout par le contrôle d'appartenance ; c'est pourquoi les trois écritures sont dans une seule transaction.

### Interface Équipe (frontend)
- `src/lib/teamApi.ts` : client typé (liste, création, changement de statut) qui distingue le refus de plafond (`402`) des autres erreurs.
- `src/components/TeamView.tsx` : liste des comptes (rôle, statut, « vous »), création dans une fenêtre modale, bouton désactiver/réactiver, rappel du plafond et message dédié « passez à une formule supérieure » sur `402`.
- Entrée **Équipe** dans la barre latérale, visible pour `owner`, `manager` et `superadmin`.

### Preuves d'exécution
- **24 assertions live** (`/tmp/verif_equipe.py`, pile réelle 3000 → 8085 → PostgreSQL 16) : inscription d'essai, liste initiale, création d'un caissier (email normalisé), connexion de l'employé, accès aux commandes, `403` pour un caissier sur le module, `409` doublon, `400` rôle superadmin / mot de passe court / email invalide, `404` identifiant inconnu, `400` auto-désactivation, plafond `402` (`feature: users.max`, total = limite, propriétaire inclus), désactivation → connexion `403` + **jeton antérieur fermé sur les routes métier**, réactivation → accès rétabli, `401` sans jeton.
- **Tests Go** : `internal/auth/users_test.go` (validation des créations, rôle superadmin refusé, désactivation/reconnexion, isolation inter-établissement), `internal/auth/persist_integration_test.go` (les deux appartenances sont réellement écrites en base ; la désactivation les suspend), `TestRequireFeatureQuota_Users` (402 au plafond, superadmin dispensé).
- **Tests frontend** : `src/lib/teamApi.test.ts` (10) et `src/components/TeamView.test.tsx` (3) → **27 tests Vitest** au total.
- **Non-régression** : 14/14 (fail-closed & quotas), 17/17 (SuperAdmin), 34/34 (E2E) → **89 assertions live** vertes, aucun test rouge.

**Compteurs** : 54 tests Go (+ 2 tests d'intégration PostgreSQL), 27 tests frontend ; bundle applicatif 263 kB (l'écran Équipe ajoute 13 kB), aucun avertissement de taille.

## 7. Session 4 — qualité frontend, garde-fous éprouvés et quota multi-sites réparé

### 1. ESLint 9 + Prettier sur tout le frontend
- `eslint.config.js` (configuration **plate** : `js.configs.recommended`, `typescript-eslint`, `react-hooks`, `react-refresh`, puis `eslint-config-prettier` en dernier), `.prettierrc.json` (guillemets simples, 110 colonnes, virgules ES5) et `.prettierignore`.
- Scripts `package.json` : `lint:eslint` (`eslint . --max-warnings=50`), `format`, `format:check` ; `lint` reste la vérification TypeScript (`tsc --noEmit`).
- Les deux contrôles sont branchés dans la CI (job *Frontend*), après `npm ci --legacy-peer-deps`.
- **TypeScript ramené à `6.0.3`** : `typescript-eslint` 8.71 déclare `peerDependencies typescript >=4.8.4 <6.1.0` et refuse TypeScript 7.
- **Dette résorbée** : `CatalogManagerModal.tsx` appelait 25 hooks **après** un `return null` conditionnel (l'ordre d'appel changeait entre le premier et le second rendu → erreur React en production) ; early-return déplacé après tous les hooks. Imports/props/mutateurs morts supprimés dans `ServicesView`, `Header`, `Sidebar`, `ThermalTicketModal`, `CounterReceiptModal`, `App.tsx` et `src/lib/api.ts`.
- Résultat : **`eslint .` → 0 erreur** (46 avertissements `no-explicit-any` assumés et plafonnés), **`prettier --check .` → conforme** sur l'ensemble du dépôt frontend.

### 2. Couverture Vitest étendue aux vues métier
**10 fichiers / 63 tests** verts : clients `branchesApi` (8), `expensesApi` (7), `reportsApi` (8), `teamApi` (10), `adminApi` (12) ; composants `BranchesView` (4), `ExpensesView` (4), `ReportsView` (5), `TeamView` (3), `SuperAdminView` (2). Les tests vérifient notamment : établissement principal protégé, refus `402` affiché avec l'offre commerciale, isolation des actions selon le rôle (caissier), calculs de période (`monthBounds`, février bissextile), envoi JSON des créations et rechargement au changement de période.

### 3. Bug — le plafond `branches.multi_site` n'était jamais appliqué
`billing.Service.FeatureUsage` ne traitait pas la clé `branches.multi_site` : la consommation mesurée valait **toujours 0**, donc `CheckLimit` laissait passer toutes les créations. Constaté sur la pile réelle : un pressing **gratuit** créait un 2e établissement (plafond 1), un établissement en **essai** en créait un 4e (plafond BUSINESS = 3) et la consolidation affichait 4 sites.
- Correctif : `FeatureUsage` mesure désormais `CountBranches` (`COUNT(branches) WHERE status <> 'archived'`) et `isCountedFeature` déclare la clé, ce qui expose aussi la consommation dans `/me/entitlements`.
- Tests ajoutés : `internal/billing/entitlements_test.go` (mesure, refus au plafond en gratuit **et** en BUSINESS, autorisation sous le plafond, `usage` exposé) et `internal/http/feature_gate_test.go#TestRequireFeatureQuota_MultiSites` (`402` avec `feature: branches.multi_site`, superadmin dispensé). **Les deux échouent si l'on retire le cas de `FeatureUsage`** (vérifié en le supprimant temporairement).

### 4. Bug — `GET /api/v1/users` renvoyait `null` pour une équipe vide
Une tranche nulle Go était sérialisée `null` ; le client frontend en déduisait une erreur de chargement au lieu d'une liste vide.
- Correctif : le handler normalise en tableau vide ; `teamApi.ts` traite explicitement `null`.
- Tests : `auth/users_test.go` — `TestListUsers_TenantSansCompte` (corps strictement `[]`) et `TestListUsers_ComptesDuTenant` (isolation inter-établissements, aucun condensat de mot de passe exposé).

### Preuves d'exécution
- **Modules payants en live** (`/tmp/verif_modules.py`, pile réelle 3000 → 8085 → PostgreSQL 16) : **35/35** (avant correctif : 32/35, les trois échecs étant le quota multi-sites).
- **Module Équipe en live** (`/tmp/verif_equipe.py`) : **24/24**.
- **Non-régression** : 14/14 (fail-closed & quotas), 17/17 (SuperAdmin), 34/34 (E2E) → **89 assertions live** vertes.
- **Backend** : `gofmt` propre, `go vet ./...` muet, `go test ./...` → **12 paquets `ok`**, **67 tests unitaires** (+ 3 tests d'intégration PostgreSQL, dont `TestIntegration_MultiSitesDepensesRapports`).
- **Frontend** : `npm run lint` (TypeScript 6.0.3) OK · `npm run lint:eslint` 0 erreur · `npm run format:check` OK · `npm test` **63 tests** · `npm run build` OK.

**Compteurs** : 67 tests Go (+ 3 d'intégration), 63 tests frontend, **35 + 24 + 89 = 148 assertions live** vertes.

## 8. Session 5 — export des rapports, impression ESC/POS et affectation multi-sites

Trois sujets issus du backlog, livrés avec leurs preuves d'exécution sur la pile réelle.

### 1. Export des rapports (CSV & PDF) — `GET /api/v1/reports/export`
- `backend/internal/report/export.go` produit un **CSV** `;` + **BOM UTF-8** + CRLF (ouverture directe dans Excel FR, accents préservés) ; `backend/internal/report/pdf.go` compose un **PDF 1.4** sans dépendance : Helvetica + encodage WinAnsi, A4 (595 × 842), pagination automatique au-delà de 46 lignes, `startxref`/`%%EOF` valides. Les caractères typographiques français (`–`, `—`, `œ`, `€`) passent par une table CP1252 explicite ; les octets Latin-1 (160-255) sont écrits tels quels.
- La réponse est une pièce jointe : `Content-Disposition: attachment; filename="rapport-<slug>-<from>_<to>.<ext>"`, le slug étant dérivé du nom du pressing (`Pressing Étoile d'Or` → `rapport-pressing-toile-d-or-…`).
- Les chiffres exportés sont **ceux de l'écran** : le même `Summary` alimente `/reports/summary` et l'export (dépenses, marge et consolidation incluses). Le mur payant est identique (`RequireFeature(reports.advanced)`, `402` en formule gratuite), et `400` couvre un format inconnu ou une période invalide (`from > to`, date illisible).
- Interface : boutons **CSV** / **PDF** dans l'en-tête de l'écran Rapports (avec état de chargement, message de confirmation, et désactivation lorsque la formule ne débloque pas les rapports).
- Tests : `internal/report/export_test.go` — **7 tests** (séparateur/BOM/contenu, `NomFichier` avec accents, structure PDF et `startxref`, échappement, pagination `/Count 2`, WinAnsi `activit\xe9`, absence de consolidation hors BUSINESS) ; côté front, `reportsApi.test.ts` (**5 tests** d'export) et `ReportsView.test.tsx` (téléchargement réel via ancre, nom de fichier serveur, message d'erreur non commercial).

### 2. Impression ESC/POS native (WebUSB)
- `src/lib/escpos.ts` construit le **flux binaire** d'une imprimante thermique 80 mm : `ESC @` + `ESC t 0` (CP437), en-tête double taille centrée, séparateurs sur 42 colonnes, articles tronqués, totaux (`TOTAL`, `Payé`, `RESTE A PAYER` / `SOLDÉ`), URL de vérification du ticket, coupe `ESC d 3` + `GS V B`.
- Encodage CP437 avec replis progressifs : ponctuation typographique → ASCII (`’` → `'`, `«»` → `"`, `–—` → `-`), puis perte d'accent (`À` → `A`) ; les montants `fr-FR` (espaces fines U+202F/U+00A0) sont normalisés.
- La modale ticket gagne **« Imprimer en direct (ESC/POS) »** (transport WebUSB : filtre classe 7, endpoint bulk out, messages d'erreur explicites si le navigateur ne supporte pas WebUSB — Chrome/Edge sur ordinateur requis) et un **aperçu du flux** reconstitué (`renderEscPosPreview`) pour vérifier avant impression. L'impression HTML 80 mm reste disponible en repli.
- Tests : `src/lib/escpos.test.ts` — **16 tests** (encodage CP437, perte d'accent, troncature, totaux, coupe, aperçu, transport WebUSB simulé avec transfert du flux, absence d'octet de contrôle).

### 3. Affectation d'un employé à un établissement
- `backend/internal/branch/assignment.go` + SQL transactionnel (`branch/persist.go`) : lecture et **remplacement exclusif** des `branch_memberships` d'un compte (`GET`/`PUT /api/v1/users/{id}/branches`). Une sélection vide est refusée (`400` fail-closed : un employé agit toujours sur au moins un site), un site inconnu ou **archivé** est refusé (`400`), un compte d'un autre pressing est introuvable (`404`).
- Garde-fou de concurrence : `memberBranchesLocked` (le verrou n'est jamais repris par une méthode déjà verrouillée — un interblocage a été détecté puis éliminé par les tests).
- Effet immédiat : `RequireBranchScope` s'appuie sur ces affectations ; retirer un site ferme l'accès aux commandes rattachées (`403`) dès l'enregistrement, sans reconnexion. Vérifié sur la pile réelle.
- Interface : bouton **« Affecter »** par employé dans l'écran Équipe, sélecteur des sites (principal signalé, statut affiché), refus local d'une sélection vide, message de confirmation.
- Tests : `branch/service_test.go` (`TestAffectation_MembreAuxEtablissements`, 6 étapes : défaut, non-affecté, remplacement exclusif, cas limites, isolation entre pressings, archivage) ; front, `teamApi.test.ts` (**6 tests**) et `TeamView.test.tsx` (sélection d'un site, enregistrement, refus de la sélection vide).

### 4. Bug — dépense impossible avec un compte de démonstration
Détecté en alimentant le pressing vitrine : `POST /api/v1/expenses` répondait **400**
`invalid input syntax for type uuid: "usr-gerant-001" (SQLSTATE 22P02)`.
- Cause : `expense/persist.go` castait l'identifiant de l'auteur (`created_by UUID`) sans vérifier sa forme. Les comptes de démonstration portent des identifiants **mémoire** (`usr-gerant-001`), valides pour le jeton JWT mais impossibles à insérer dans une colonne UUID. Les modules `billing` (journal d'abonnement) et `order` avaient déjà leur conversion ; le module Dépenses l'avait oubliée — invisible jusqu'ici car tous les tests de bout en bout créaient des établissements réels (identifiants UUID).
- Correctif : `uuidPourColonne` (même politique que `billing/persist.go`) convertit l'auteur en valeur de colonne : UUID conservé, identifiant mémoire → `NULL`. L'attribution des vrais comptes reste donc intacte et la dépense est enregistrée, simplement sans auteur.
- Tests : `internal/expense/persist_test.go` — `TestUUIDPourColonne` (6 cas : identifiant de démonstration, chaîne vide, non hexadécimal, espaces, majuscules, UUID standard).

### Preuves d'exécution
- **Live (pile réelle 3000 → 8085 → PostgreSQL 16)** : `/tmp/verif_tour5.py` — **30/30** (export CSV/PDF conforme et chiffres identiques à l'écran, garde-fous `400`/`401`/`402`, affectation par défaut puis exclusive, effet réel sur `X-Branch-ID` — `200` sur le site affecté, `403` sur le site retiré).
- **Non-régression** : 35/35 (modules payants), 24/24 (Équipe), 34/34 (E2E), 17/17 (SuperAdmin), 14/14 (fail-closed & quotas) → **154 assertions live** vertes.
- **Correctif dépense/démo** (`/tmp/verif_fix_demo.py`) : **11/11** — attribution conservée pour un compte réel, dépense acceptée avec un compte de démonstration (`created_by` NULL, aucun auteur inventé), export CSV/PDF et affectation toujours opérationnels.
- **Backend** : `gofmt` propre, `go vet ./...` muet, `go test ./...` **12 paquets `ok`**, binaire `backend/bin/api` reconstruit et rechargé par le serveur de développement.
- **Frontend** : `tsc --noEmit` OK · `eslint .` **0 erreur** (46 avertissements plafonnés) · `prettier --check .` conforme · **11 fichiers / 94 tests Vitest** · `npm run build` OK.

**Compteurs** : 67 tests Go (+ 3 d'intégration), 94 tests frontend, **154 assertions live** vertes.

## 9. Session 6 — notifications clients (WhatsApp / SMS)

Objectif : prévenir le client sans quitter le comptoir, et prouver que le message est bien
parti. La composition et le **journal restent gratuits** ; l'**envoi par passerelle** est le
module payant `notifications.advanced` (faux en gratuit, vrai en PRO et BUSINESS).

### 1. Journal et composition ouverts à tous les pressings
- `GET /api/v1/notifications` : journal du pressing, filtres `order_id`, `channel`,
  `status`, `limit` (défaut 50, plafond 200) ; réponse enrichie des canaux et modèles
  disponibles. Lisible même en formule gratuite — l'historique appartient au pressing, et
  une rétrogradation ne rend jamais inaccessible un message déjà envoyé.
- Quatre modèles français : `commande_prete`, `commande_livree`, `paiement_recu`,
  `rappel_retrait`. Le nom du pressing vient du **jeton** (`claims.TenantName`) ou de la
  base (`SetNomPressing`) — jamais du corps de la requête, donc jamais falsifiable.
- L'interface du comptoir (`ClientNotificationModal`) propose le canal, le modèle, un
  **aperçu éditable**, l'envoi, ainsi que « Ouvrir dans WhatsApp / SMS » (liens `wa.me` et
  `sms:`) et « Copier » : le pressing sans passerelle envoie depuis son propre téléphone.

### 2. Envoi réel par passerelle configurable
- `POST /api/v1/notifications` : `201` `{"notification", "status", "message"}`. Gardé par
  `RequireFeature(notifications.advanced)` → `402` en formule gratuite ; l'interface affiche
  alors l'invitation commerciale **et** conserve l'ouverture manuelle.
- Passerelle générique par webhook (`NOTIFICATIONS_GATEWAY_URL`, jeton facultatif
  `NOTIFICATIONS_GATEWAY_TOKEN`) : POST JSON `{to, channel, body, order_id, tenant_id}`,
  `Authorization: Bearer <jeton>`, délai 5 s. Le fournisseur réel (WhatsApp Business API,
  agrégateur SMS local) se branche derrière ce webhook sans toucher au cœur métier.
- Sans passerelle — ou si elle répond en erreur — le message est **journalisé**
  (`simule`, ou `echec` avec le motif dans `detail`) : jamais de perte silencieuse, l'atelier
  n'est jamais bloqué par un fournisseur indisponible.

### 3. Déclenchement automatique sur `ready` et `delivered`
`order.SetHookStatut` branche `notification.NotifierTransition` : lorsque le linge passe à
`ready` (prêt au retrait) ou `delivered` (livré), le client est prévenu automatiquement —
en **goroutine isolée** avec `recover` et un délai de 10 s, donc l'accusé de réception du
comptoir n'attend jamais la passerelle. Les autres transitions n'envoient rien, et le droit
`notifications.advanced` est revérifié à chaque fois.

### 4. Isolation et validations éprouvées
- Une commande d'un autre pressing, inconnue ou d'identifiant mal formé → **404
  « commande introuvable »** (aucune fuite d'existence, aucun message chez le voisin).
- Canal inconnu (`email`), modèle inconnu, téléphone manquant → **400** avec message métier.
- Message blanc → repli sur le modèle (le comptoir ne peut pas envoyer un WhatsApp vide).

### 5. Bug/piège corrigé pendant la session
- `order.Order` n'expose **ni `BranchID` ni `TenantName`** : le périmètre a été retiré du
  modèle de notification et le nom du pressing déplacé côté jeton/base (voir ci-dessus).
- Un utilitaire UUID unique (`pgutil.NullableUUID`, `pgutil.IsUUID`) a été extrait : la
  dépense (session 5) et l'abonnement l'utilisent désormais au lieu de conversions `::uuid`
  divergentes — c'est la classe de bug qui avait produit le `400 22P02` de la session 5.

### 6. Habillage : cadres surélevés et survols foncés
- Nouveaux utilitaires `ps-elevated` / `ps-elevated-static` (`src/index.css`) : liseré
  d'ombre sur les bords, ombre portée profonde, soulèvement au survol avec trait **noir**
  (et variante sombre dédiée).
- **58 cadres surélevés** (cartes, encarts, panneaux, lignes de synthèse) et **9 surfaces
  plates** (`ps-elevated-static`) pour les champs de saisie, pastilles et la barre
  d'en-tête collante — un champ ne se soulève pas au survol ; les **survols** deviennent
  foncés : 33 contrôles passent au noir plein avec texte blanc, 69 bordures réagissent en
  noir, les lignes de liste reçoivent une tinte noire, les modales gardent leur ombre
  profonde sans soulèvement. Plus aucun survol gris clair dans l'application (vérifié par
  test).
- `src/styles.test.ts` verrouille le jeu de styles (utilitaires, cadres équipés, absence de
  régression vers les survols clairs).

### Preuves d'exécution (session 6)
| Preuve | Résultat |
|---|---|
| `go build ./...`, `go vet ./...`, `gofmt -l` | propres |
| `go test ./...` | **13 paquets ok**, dont `notification` (11 tests : composition, validations, passerelle `httptest` — payload, Bearer, échec 429/injoignable, hook automatique, isolation tenant) et `billing` (`notifications.advanced` réservé aux formules payantes) |
| Frontend | `tsc --noEmit`, ESLint, Prettier, **116 tests Vitest** (16 fichiers : `notificationsApi`, `ClientNotificationModal`, fumée de l'application, garde-fou du jeu de styles), build Vite ✅ |
| Live `/tmp/verif_tour6.py` (passerelle webhook de test) | **57/57** — journal vide puis 2 messages (`envoye`, `provider: passerelle`), payload et jeton Bearer reçus par la passerelle, filtres, persistance SQL, refus `400`/`404`, repli du message blanc, messages automatiques `ready`/`delivered` réellement transmis, **isolation inter-pressings**, **402 après la fin de l'essai** avec configuration toujours lisible, non-régression des autres modules |

---

## 10. Reste à faire (backlog priorisé)

| Priorité | Sujet |
|---|---|
| 🟡 Moyenne | `analyse.md` et `rapports_audit_consolide.md` : à archiver (chiffres non reproductibles) |
| 🟢 Basse | Mode hors-ligne (PWA), transfert d'un ticket vers une imprimante réseau, sélection d'un fournisseur WhatsApp/SMS managé côté éditeur |

---

## 11. Vérification finale de bout en bout — 34/34 ✅

Parcours automatisé exécuté sur le stack réel (PostgreSQL 16 + binaire Go + proxy Express) :

| Bloc | Vérifications |
|---|---|
| Santé & version | `/health` 200 · `/ready` 200 · version `0.5.0-hardening` |
| Onboarding d'un nouveau pressing | inscription 201 · 49 prestations · grille principale + version figée à 49 entrées · 1 abonnement d'essai 7 j |
| Cycle métier complet | client 201 · commande 201 · liste/fiche 200 · changement de statut 200 · ticket 80 mm 200 · vérification QR publique 200 · **surpaiement 400** · paiement exact 200 → `paid`, reste dû 0 |
| Sécurité | inter-tenant 404 · superadmin sans contexte **403** (plus de 500) · superadmin avec établissement ciblé 200 · `X-Tenant-ID` forgé 403 · sans jeton 401 · `/admin/stats` 403 pour un gérant / 200 pour le superadmin · force brute **429** · mot de passe correct accepté malgré le plafond |
| Quotas | essai actif · reste de quota jamais négatif |

Script reproductible : `/tmp/e2e_final.sh` (34 assertions).

## 12. Comment reproduire les vérifications

```bash
# Backend : tests unitaires + analyse statique
cd backend && go vet ./... && go test ./...

# Tests d'intégration sur PostgreSQL 16 (migrations appliquées)
TEST_DATABASE_URL="postgres://pressing_app:...@localhost:5432/pressing_os?sslmode=disable" \
  go test -tags=integration -v ./internal/catalog/... ./internal/auth/... ./internal/report/...

# Modules payants de bout en bout (multi-sites, dépenses, rapports, murs 402)
python3 /tmp/verif_modules.py

# Module Équipe de bout en bout (inscription, création, plafond, désactivation)
python3 /tmp/verif_equipe.py

# Tour 5 : export CSV/PDF des rapports et affectation employé ↔ établissement
python3 /tmp/verif_tour5.py

# Correctif « dépense avec un compte de démonstration » (auteur non-UUID)
python3 /tmp/verif_fix_demo.py

# Tour 6 : notifications clients (journal, passerelle webhook, automatisme, 402)
# Prérequis : NOTIFICATIONS_GATEWAY_URL=http://127.0.0.1:9099/webhook et le petit
# récepteur /tmp/passerelle.py lancés, puis npm run dev
python3 /tmp/verif_tour6.py

# Frontend : types, ESLint, formatage, tests (116), build
npm ci --legacy-peer-deps
npm run lint && npm run lint:eslint && npm run format:check && npm test && npm run build

# Stack complet (proxy 3000 → API interne 8085)
npm run dev
```
