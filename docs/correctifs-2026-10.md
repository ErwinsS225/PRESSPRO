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

## 5. Reste à faire (backlog priorisé)

| Priorité | Sujet |
|---|---|
| 🟠 Haute | Tests frontend (Vitest Testing Library) et ESLint/Prettier ; découpage du bundle 602 kB |
| 🟠 Haute | Brancher `RequireFeature` sur les routes concernées + limitation d'usage (grilles, multi-sites, rapports) |
| 🟠 Haute | Bascule « fail-closed » des contrôles d'appartenance hors mode démo `looksLikeUUID` |
| 🟡 Moyenne | Espace SuperAdmin alimenté par les données réelles (`tenants`, `subscriptions`) au lieu de valeurs d'illustration |
| 🟡 Moyenne | `analyse.md` et `rapports_audit_consolide.md` : à archiver (chiffres non reproductibles) |
| 🟡 Moyenne | Gestion des établissements (branches) et des utilisateurs dans l'interface |
| 🟢 Basse | Notifications WhatsApp/SMS, rapports avancés, impression ESC/POS native, mode hors-ligne |

---

## 6. Vérification finale de bout en bout — 34/34 ✅

Parcours automatisé exécuté sur le stack réel (PostgreSQL 16 + binaire Go + proxy Express) :

| Bloc | Vérifications |
|---|---|
| Santé & version | `/health` 200 · `/ready` 200 · version `0.5.0-hardening` |
| Onboarding d'un nouveau pressing | inscription 201 · 49 prestations · grille principale + version figée à 49 entrées · 1 abonnement d'essai 7 j |
| Cycle métier complet | client 201 · commande 201 · liste/fiche 200 · changement de statut 200 · ticket 80 mm 200 · vérification QR publique 200 · **surpaiement 400** · paiement exact 200 → `paid`, reste dû 0 |
| Sécurité | inter-tenant 404 · superadmin sans contexte **403** (plus de 500) · superadmin avec établissement ciblé 200 · `X-Tenant-ID` forgé 403 · sans jeton 401 · `/admin/stats` 403 pour un gérant / 200 pour le superadmin · force brute **429** · mot de passe correct accepté malgré le plafond |
| Quotas | essai actif · reste de quota jamais négatif |

Script reproductible : `/tmp/e2e_final.sh` (34 assertions).

## 7. Comment reproduire les vérifications

```bash
# Backend : tests unitaires + analyse statique
cd backend && go vet ./... && go test ./...

# Test d'intégration sur PostgreSQL 16 (migrations appliquées)
TEST_DATABASE_URL="postgres://pressing_app:...@localhost:5432/pressing_os?sslmode=disable" \
  go test -tags=integration -v ./internal/catalog/...

# Frontend
npm ci --legacy-peer-deps && npm run lint && npm run build

# Stack complet (proxy 3000 → API interne 8085)
npm run dev
```
