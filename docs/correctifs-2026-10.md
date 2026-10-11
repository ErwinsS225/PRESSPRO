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

## 7. Reste à faire (backlog priorisé)

| Priorité | Sujet |
|---|---|
| 🟡 Moyenne | ESLint/Prettier côté frontend ; couverture Vitest étendue aux vues métier |
| 🟠 Haute | Modules payants restants : `branches.multi_site` (multi-établissements), `reports.*`, `expenses.*` — routes à créer puis garder avec `RequireFeature`/`RequireFeatureQuota` |
| 🟡 Moyenne | `analyse.md` et `rapports_audit_consolide.md` : à archiver (chiffres non reproductibles) |
| 🟡 Moyenne | Gestion des établissements (branches / multi-sites) dans l'interface (`branches.multi_site`) |
| 🟢 Basse | Notifications WhatsApp/SMS, rapports avancés, impression ESC/POS native, mode hors-ligne |

---

## 8. Vérification finale de bout en bout — 34/34 ✅

Parcours automatisé exécuté sur le stack réel (PostgreSQL 16 + binaire Go + proxy Express) :

| Bloc | Vérifications |
|---|---|
| Santé & version | `/health` 200 · `/ready` 200 · version `0.5.0-hardening` |
| Onboarding d'un nouveau pressing | inscription 201 · 49 prestations · grille principale + version figée à 49 entrées · 1 abonnement d'essai 7 j |
| Cycle métier complet | client 201 · commande 201 · liste/fiche 200 · changement de statut 200 · ticket 80 mm 200 · vérification QR publique 200 · **surpaiement 400** · paiement exact 200 → `paid`, reste dû 0 |
| Sécurité | inter-tenant 404 · superadmin sans contexte **403** (plus de 500) · superadmin avec établissement ciblé 200 · `X-Tenant-ID` forgé 403 · sans jeton 401 · `/admin/stats` 403 pour un gérant / 200 pour le superadmin · force brute **429** · mot de passe correct accepté malgré le plafond |
| Quotas | essai actif · reste de quota jamais négatif |

Script reproductible : `/tmp/e2e_final.sh` (34 assertions).

## 9. Comment reproduire les vérifications

```bash
# Backend : tests unitaires + analyse statique
cd backend && go vet ./... && go test ./...

# Test d'intégration sur PostgreSQL 16 (migrations appliquées)
TEST_DATABASE_URL="postgres://pressing_app:...@localhost:5432/pressing_os?sslmode=disable" \
  go test -tags=integration -v ./internal/catalog/... ./internal/auth/...

# Module Équipe de bout en bout (inscription, création, plafond, désactivation)
python3 /tmp/verif_equipe.py

# Frontend
npm ci --legacy-peer-deps && npm run lint && npm run build

# Stack complet (proxy 3000 → API interne 8085)
npm run dev
```
