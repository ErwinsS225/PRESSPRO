# RAPPORT DE PROJET — PressingOS

**Plateforme SaaS multi-tenant de gestion des pressings et buanderies — Côte d'Ivoire**

| | |
|---|---|
| **Dépôt** | `ErwinsS225/PRESSPRO` |
| **Branche analysée** | `main` — commit `58f1da7` (11/10/2026, *« Standardisation du service catalog (logger, helpers, isolation, tests) + corrections diverses »*) |
| **Date du rapport** | 11 octobre 2026 |
| **Méthode** | Lecture intégrale du code + exécution réelle (build, tests, base PostgreSQL 16, tests d'API bout-en-bout) |
| **Nature du rapport** | Audit technique complet : architecture, fonctionnel, qualité, sécurité, anomalies, recommandations |

---

## 0. Résumé exécutif

**PressingOS** est un ERP SaaS en **monolithe modulaire** destiné aux pressings ivoiriens. Il associe :

- un **backend Go 1.25** (Chi router, pgx/PostgreSQL 16, JWT) organisé en 10 modules métier,
- un **frontend React 19 + TypeScript + Vite 8 + Tailwind 4** (15 composants, mode sombre, tickets thermiques 80 mm),
- un **proxy Express** (`server.ts`) qui sert le SPA et relaie `/health`, `/ready`, `/api/*` vers le binaire Go,
- un **moteur commercial** (plans FREE/PRO/BUSINESS, essai 7 jours, quotas, matrice de droits) et un **moteur d'autorisation à 4 contrôles** (JWT → appartenance → rôle → périmètre branche).

### Verdict global : **7,5 / 10 — projet solide et cohérent, mais entaché de 2 défauts bloquants en mode production**

| Axe | Note | Constat en une phrase |
|---|---|---|
| Architecture & organisation | 8,5/10 | Pattern identique sur tous les modules (handler/service/persist/model/test), séparation nette des responsabilités. |
| Modèle de données | 9/10 | 19 tables, 10 migrations up/down idempotentes, isolation `tenant_id` systématique, index composites. |
| Sécurité multi-tenant | 8/10 | Isolation prouvée par tests d'intrusion réels (401/403/404 corrects) ; faiblesse sur le secret JWT par défaut. |
| Qualité / tests | 6/10 | 32 tests Go tous verts, mais couverture hétérogène (health 80 %, orders 22,6 %, customer/pricelist/database 0 %) ; zéro test frontend. |
| Fonctionnel livré | 8/10 | Commandes, tickets, caisse, catalogue, clients, grilles tarifaires, abonnement, super-admin : le cœur est là et fonctionne. |
| Robustesse en production | **4/10** | **Le catalogue est inaccessible (HTTP 500) dès que PostgreSQL est la source de données** ; le « repli mémoire » annoncé ne s'active pas si la base tombe après le démarrage. |
| Documentation | 7/10 | 4 documents de qualité (`docs/`) mais **partiellement désynchronisés** du code (versions, ports, chiffres de tests). |

### Les 3 points à retenir

1. 🔴 **BUG BLOQUANT #1 — `GET /api/v1/services` renvoie 500 en mode PostgreSQL.** `catalog.NewStore()` construit le service sans initialiser son logger (`nil`), or `GetServices` et `GetServiceByID` l'utilisent en première ligne → panique *nil pointer dereference*, rattrapée par le middleware `Recoverer` qui renvoie un 500 vide. Conséquence produit : l'écran catalogue (1 878 lignes de frontend) et le calcul de la grille tarifaire ne peuvent pas lire les données réelles ; l'application retombe silencieusement sur les données de démonstration. **Les écritures (POST/PUT/DELETE) fonctionnent : seule la lecture est cassée.**

2. 🔴 **BUG BLOQUANT #2 — pas de repli mémoire quand PostgreSQL est présent mais injoignable.** `main.go` décide du mode persistance via `dbPool != nil && dbPool.Pool != nil` (simple création du pool, sans vérifier le `Ping`). Résultat vérifié : avec une base arrêtée, les logs annoncent « Catalogue/Commandes/Clients adossés à PostgreSQL », `/health` répond 200, la connexion JWT fonctionne… mais `/api/v1/orders`, `/api/v1/services` et `/api/v1/me/entitlements` renvoient **500** au lieu de basculer sur les données seed mémoire annoncées par les commentaires du code.

3. 🟠 **Anomalie financière — le surpaiement est accepté.** Un paiement de **999 999 XOF** sur une commande de **1 500 XOF** a été accepté (HTTP 200), enregistré dans la table `payments`, avec `paid_amount` bloqué à 1 500 → écriture comptable incohérente (le journal des encaissements contredit l'état de la commande).

---

## 1. Fiche d'identité du projet

### 1.1 Produit

| Élément | Valeur |
|---|---|
| Nom produit | **PressingOS** (métadonnée repo : « PressingOS ») |
| Domaine | ERP / SaaS pour pressings, blanchisseries et buanderies |
| Marché cible | Côte d'Ivoire — devise **XOF (FCFA)**, fuseau `Africa/Abidjan`, mobile money (Wave, Orange Money, MTN MoMo) |
| Modèle économique | Abonnement SaaS B2B : FREE (0 F) / PRO (45 000 F/mois) / BUSINESS (75 000 F/mois), essai 7 jours |
| Positionnement | « Monolithe modulaire », multi-tenant à base partagée, isolation par `tenant_id` |

### 1.2 Volumétrie du code (mesurée)

| Zone | Fichiers | Lignes |
|---|---|---|
| Backend Go (`backend/`) | 51 | **7 902** |
| Migrations SQL (`backend/migrations/`) | 20 (10 up + 10 down) | 596 |
| Frontend TypeScript/TSX (`src/`) | 26 (+2 racine : `server.ts`, `vite.config.ts`) | **8 830** |
| Documentation Markdown | 6 | — |
| Tests Go (fichiers `*_test.go`) | 11 | 32 fonctions de test |

### 1.3 Pile technique

**Backend** : Go 1.25 · `go-chi/chi/v5` (routeur) · `go-chi/cors` · `jackc/pgx/v5` + `pgxpool` · `joho/godotenv` · `stretchr/testify` · `golang.org/x/crypto/bcrypt`.

**Base de données** : PostgreSQL 16 (image `postgres:16-alpine` via Docker Compose), extension `uuid-ossp`, migrations SQL numérotées up/down, volume persistant `pressingos_postgres_data`, exposition `127.0.0.1:5432` uniquement.

**Frontend** : React 19 · TypeScript 7 · Vite 8 · Tailwind CSS 4 · `motion` (animations) · `lucide-react` (icônes) · `@google/genai` (dépendance déclarée).

**Full-stack local** : `tsx server.ts` — Express 4 sur le port 3000 (mode middleware Vite) qui **spawn** `backend/bin/api` sur le port interne 8085 puis proxifie `/health`, `/ready` et `/api/*`.

### 1.4 Commandes du projet

| Script (`package.json`) | Effet |
|---|---|
| `npm run dev` / `npm start` | `tsx server.ts` — Full-stack (frontend + binaire Go embarqué) |
| `npm run build` | `vite build` → `dist/` |
| `npm run lint` | `tsc --noEmit` |
| `npm run clean` | Supprime `dist` et `server.js` |
| `make run` / `build` / `test` / `fmt` / `vet` / `tidy` (backend) | Cycle de vie Go |

---

## 2. Architecture

### 2.1 Vue d'ensemble

```
                        Navigateur (React 19 SPA)
                                  │  HTTPS/JSON (fetch relatif /api/…)
                                  ▼
        ┌────────────────────────────────────────────────────────┐
        │  server.ts — Express 4 (port 3000)                     │
        │   • middleware Vite (dev) ou dist/ statique (prod)     │
        │   • routage SPA : /verify/:tenant/:order → index.html  │
        │   • proxy transparent → 127.0.0.1:8085                 │
        └────────────────────────────────────────────────────────┘
                                  │ (spawn du binaire)
                                  ▼
        ┌────────────────────────────────────────────────────────┐
        │  API REST Go — Chi (port 8085 en dev / 8080 par défaut)│
        │  Middlewares : RequestID · RealIP · Logger · Recoverer │
        │  Timeout 15 s · CORS · JWT · Appartenance · RBAC ·     │
        │  Périmètre branche · Garde-fou quota (402)             │
        ├────────────────────────────────────────────────────────┤
        │  Modules métier (internal/)                            │
        │  auth · catalog · order · customer · pricelist ·       │
        │  billing · health · config · database · http           │
        └────────────────────────────────────────────────────────┘
                                  │ pgxpool
                                  ▼
                   PostgreSQL 16 — 19 tables (tenant_id partout)
```

### 2.2 Structure du dépôt

```
PRESSPRO/
├── backend/                    # API Go (monolithe modulaire)
│   ├── cmd/api/main.go         # Entrée : config → pool → services → routeur → graceful shutdown
│   ├── internal/
│   │   ├── auth/               # JWT, bcrypt, login, inscription tenant (9 fichiers)
│   │   ├── billing/            # Plans, droits, quotas, matrice (9 fichiers)
│   │   ├── catalog/            # Prestations & tarifs (6 fichiers)
│   │   ├── config/             # Lecture typée de l'environnement (2)
│   │   ├── customer/           # Fichier clients (4)
│   │   ├── database/           # Pool pgx + helpers d'exécution (2)
│   │   ├── health/             # /health & /ready (2)
│   │   ├── http/               # Routeur, 4 middlewares de sécurité (6)
│   │   ├── order/              # Commandes, paiements, ticket thermique (6)
│   │   └── pricelist/          # Grilles tarifaires versionnées (4)
│   ├── migrations/             # 000001 → 000010 (up/down)
│   ├── Makefile · go.mod · go.sum
├── src/                        # Frontend React
│   ├── App.tsx (453 l.)        # Orchestrateur : session, routage par état, données
│   ├── components/             # 15 composants : écrans, modales, navigation (voir §6)
│   ├── lib/                    # 5 clients API + données de démo
│   ├── context/ThemeContext    # Thème clair/sombre
│   └── types/index.ts          # Modèles TypeScript
├── docs/                       # architecture.md · api.md · database.md · installation.md
├── compose.yaml                # PostgreSQL 16 local
├── server.ts                   # Serveur full-stack Express + proxy
├── analyse.md                  # Analyse d'origine du service order (héritage)
└── rapports_audit_consolide.md # Audit interne antérieur (héritage)
```

### 2.3 Choix d'architecture documentés (et respectés dans le code)

- **Monolithe modulaire assumé** plutôt que microservices : justifié explicitement par la contrainte « prototype en 5 jours ».
- **Séparation stricte handler / service / persist / model** — vérifiée sur les 10 modules.
- **Persistance duale** : repli mémoire (démonstration) ou PostgreSQL (production), décidé au démarrage.
- **Arrêt gracieux** : `SIGINT`/`SIGTERM` → arrêt des acceptations, 10 s de grâce, fermeture du pool pgx. **Vérifié en exécution** (log « Serveur arrêté proprement »).
- **Timeouts stricts** : lecture/écriture/idle configurables, middleware `Timeout(15 s)`.

---

## 3. Backend Go en détail

### 3.1 Modules et responsabilités

| Module | Rôle | Persistance | Tests |
|---|---|---|---|
| `auth` | Login, inscription d'un pressing, JWT HMAC-SHA256, bcrypt, gestion des utilisateurs | DB + mémoire | 5 tests (JWT ×3, bcrypt, service ×2) |
| `order` | Cycle de vie des commandes (création → statut → paiement → retrait), numérotation `CMD-AAAA-NNN` atomique, ticket ESC/POS 80 mm, vérification publique QR | DB + mémoire | 8 tests dont **isolation multi-tenant** |
| `catalog` | Prestations (article/traitement/catégorie), modèles tarifaires, options express/collecte/sous-traitance | DB + mémoire | 6 tests dont **isolation** |
| `customer` | Fichier client, unicité `(tenant_id, phone)` | DB + mémoire | **0 test** |
| `pricelist` | Grilles tarifaires versionnées (snapshot JSONB immuable, statuts draft/active/superseded) | DB + mémoire | **0 test** |
| `billing` | Plans, droits effectifs, essai, quotas mensuels, compteurs d'usage | DB + mémoire | 3 tests (essai, expiration, borne) |
| `http` | Routeur Chi + 4 middlewares de sécurité + garde-fou quota | — | 4 tests |
| `health` | `/health` (liveness) et `/ready` (readiness DB) | — | 3 tests |
| `config` | Chargement/validation de l'environnement, URL DB masquée | — | 2 tests |
| `database` | Pool pgxpool, `Pinger`, store d'exécution | — | **0 test** |

### 3.2 Catalogue d'API REST (vérifié sur le code du routeur)

**Public**

| Méthode | Route | Description |
|---|---|---|
| GET | `/health` | Liveness |
| GET | `/ready` | Readiness + état PostgreSQL (200/503) |
| GET | `/api/v1/system/info` | Métadonnées du monolithe |
| POST | `/api/v1/auth/login` | Connexion (JWT + profil + établissement) |
| POST | `/api/v1/auth/register-tenant` | Onboarding d'un pressing |
| GET | `/api/v1/public/verify/{tenantID}/{orderID}` | Vérification d'authenticité d'un ticket scanné (sans PII sensible) |

**Protégé (JWT obligatoire)**

| Méthode | Route | Contrôle d'accès |
|---|---|---|
| GET | `/api/v1/auth/me` | JWT |
| GET | `/api/v1/auth/users` | JWT (liste filtrée par tenant) |
| GET | `/api/v1/tenant/current` | JWT + appartenance |
| GET/POST | `/api/v1/services` | Lecture membre · écriture owner/manager/superadmin |
| GET/PUT/DELETE | `/api/v1/services/{id}` | Lecture membre · mutation owner/manager/superadmin |
| GET/POST | `/api/v1/customers` | Lecture & création membres |
| GET/PUT/DELETE | `/api/v1/customers/{id}` | Mutation owner/manager/superadmin |
| GET/POST | `/api/v1/orders` | JWT + appartenance + **périmètre branche** + **quota 402** |
| GET | `/api/v1/orders/{id}` | Idem |
| PATCH | `/api/v1/orders/{id}/status` | Idem |
| POST | `/api/v1/orders/{id}/payments` | Idem |
| GET | `/api/v1/orders/{id}/ticket` | Ticket thermique 80 mm (texte) |
| GET/POST | `/api/v1/price-lists` | Lecture membre · création owner/manager/superadmin |
| GET | `/api/v1/price-lists/{id}` | Lecture membre |
| POST | `/api/v1/price-lists/{id}/versions` | Figeage de version (owner/manager/superadmin) |
| GET | `/api/v1/me/entitlements` | Droits effectifs + usage du mois |
| GET | `/api/v1/admin/stats` | **SuperAdmin uniquement** |

### 3.3 Chaîne de middlewares (ordre réel)

1. `RequestID` → 2. `RealIP` → 3. `Logger` → 4. `Recoverer` → 5. `Timeout(15 s)` → 6. `CORS` (origines issues de `ALLOWED_ORIGINS`) → 7. `AuthenticatorMiddleware` (JWT + anti-usurpation `X-Tenant-ID`) → 8. `RequireTenantMembership` (contrôle #2) → 9. [`RequireRole`] (RBAC) → 10. `RequireBranchScope` (contrôle #4, routes commandes).

**Anti-usurpation** : si un client forge `X-Tenant-ID` ≠ `tenant_id` du jeton (et n'est pas superadmin) → **403** immédiat. *Vérifié en direct.*

**Appartenance (`MembershipChecker`)** — sémantique à 3 modes :
- pool présent → **fail-CLOSED** (appartenance absente/suspendue = refus) ;
- identifiant utilisateur non-UUID (compte de démo mémoire) → autorisé via JWT signé ;
- pool `nil` → **fail-OPEN** (mode dégradé).

**Quota commercial** : `QuotaGuard.CheckOrdersCreation` avant toute création de commande ; refus → **HTTP 402** `{"error":"plafond atteint (50) — passez à une formule supérieure","upgrade_required":"true"}`. *Vérifié en direct.*

### 3.4 Code mort / non branché

- `http.RequireFeature(svc, key)` (billing, refus 402 si fonctionnalité absente du plan) est **défini mais jamais utilisé** dans `router.go` : le verrouillage fonctionnel par plan n'est donc pas appliqué aux endpoints (hors quota de commandes).
- `catalog.seedDefaultServices()` est un squelette : commentaire « *logique de seed similaire à NewService()* » sans implémentation → un nouveau tenant en mode mémoire n'obtient **aucun** service par défaut.
- `quota` en mode mémoire : compteur en RAM (remis à zéro au redémarrage) — acceptable en dev, à ne pas confondre avec le compteur SQL.

---

## 4. Modèle de données

### 4.1 Les 19 tables (après application des 10 migrations — vérifié)

| Domaine | Tables |
|---|---|
| Multi-tenant | `tenants`, `branches` (établissements physiques, `is_primary`, `timezone`), `tenant_memberships`, `branch_memberships` |
| Identité | `users` (rôles *superadmin/owner/manager/cashier/operator*) |
| Exploitation | `customers`, `services`, `orders`, `order_items`, `payments`, `order_sequences` |
| Tarification | `price_lists`, `price_list_versions` (snapshot JSONB, `draft/active/superseded`) |
| Abonnement | `plans`, `features`, `plan_entitlements`, `subscriptions`, `subscription_events` |
| Technique | `schema_migrations` |

### 4.2 Inventaire des migrations

| # | Objet | Points notables |
|---|---|---|
| 000001 | Schéma initial | `CREATE EXTENSION uuid-ossp`, UUID PK partout, `UNIQUE(tenant_id, phone)` clients, `UNIQUE(tenant_id, order_number)` commandes, index composites `(tenant_id, status)` |
| 000002 | Catalogue enrichi | `article`, `treatment`, `pricing_model`, express, collecte, sous-traitance, devis |
| 000003 | Facturation | `plans`, `features`, `plan_entitlements`, `subscriptions`, `subscription_events` |
| 000004 | Seed plans | FREE 0 · PRO 45 000 · BUSINESS 75 000 XOF |
| 000005 | Registre features | 10 clés (`orders.create`, `orders.monthly.max`, `users.max`, `catalog.price_lists`, `branches.multi_site`, `reports.advanced`, `expenses.advanced`, `pricing.pro`, `reports.consolidated`, `audit.advanced`) |
| 000006 | Matrice des droits | FREE : 50 commandes/mois, 2 utilisateurs, 1 grille, 1 site, pas de rapports avancés ; PRO : 5 utilisateurs, 3 grilles ; BUSINESS : 15 utilisateurs, 10 grilles, 3 sites, rapports consolidés. Essai 7 jours du tenant démo |
| 000007 | Fondations multi-tenant | `branches`, `tenant_memberships`, `branch_memberships`, `orders.branch_id`, **backfill idempotent** (branche principale, memberships, rattachement des commandes), démarrage du suivi `schema_migrations` |
| 000008 | `order_items` flexible | `service_id` nullable + `ON DELETE SET NULL` (accepte les dépôts historiques/démo sans casser l'historique) |
| 000009 | Séquences de numéros | `order_sequences` par tenant — incrément atomique via `INSERT … ON CONFLICT DO UPDATE … RETURNING` (pas de collision sur `UNIQUE(tenant_id, order_number)`) |
| 000010 | Grilles versionnées | `price_lists` (+ index unique `(tenant_id, name)`), `price_list_versions` (`UNIQUE(price_list_id, version)`), backfill « Grille principale » v1 active avec snapshot du catalogue |

**Qualité du schéma** : migrations additives et idempotentes (`IF NOT EXISTS`, `ON CONFLICT DO NOTHING`), rollback fourni pour chacune, backfills non destructifs. C'est l'un des points les plus soignés du projet. **Les 10 migrations ont été appliquées sans erreur sur une instance PostgreSQL 16.2 réelle** (hors `CREATE EXTENSION`, indisponible dans l'instance de test sandbox — l'extension est présente dans l'image Docker du projet).

### 4.3 Stratégie multi-tenant

- **Base partagée + discrimination `tenant_id`**, refusée en base-par-client (justification : migrations et ressources).
- `tenant_id` **jamais accepté du client** : extrait du JWT vérifié et injecté dans le `context.Context`.
- Filtrage `WHERE tenant_id = $1` sur **toutes** les requêtes SQL (vérifié dans `auth`, `order`, `catalog`, `customer`, `pricelist`, `billing`).
- Isolation mémoire : vérification `item.TenantID == tenantID` + **copie profonde** avant retour (empêche toute mutation externe des structures partagées).
- Index composites `(tenant_id, statut)` pour éviter tout balayage inter-tenant.

---

## 5. Moteur commercial (billing, droits, quotas)

- **Plans** : `FREE` (gratuit permanent), `PRO`, `BUSINESS`. Statuts : `TRIALING`, `ACTIVE`, `PAST_DUE`, `CANCELED`, `SUSPENDED`.
- **Essai** : `TrialDurationDays = 7`, l'essai ouvre **le niveau BUSINESS** (tout débloqué) ; à l'expiration, retour automatique au plan FREE **sans purge de données** (test unitaire dédié).
- **Droits effectifs** : `EffectiveEntitlements` fusionne plan + essai + compteurs d'usage réels (`orders.monthly.max` = commandes du mois civil, `users.max` = comptes du tenant), exposant `limit`, `usage`, `remaining` et `source` (`plan` ou `trial`).
- **API** : `GET /api/v1/me/entitlements` alimente l'écran « Mon abonnement ».

**Comportement vérifié en direct** (base réelle, essai expiré artificiellement) :

```
plan: FREE | essai: False | statut: ACTIVE
  orders.create        actif=True  limite=-     usage=-   reste=-
  orders.monthly.max   actif=True  limite=50    usage=56  reste=-6
  users.max            actif=True  limite=2     usage=0   reste=2
  catalog.price_lists  actif=True  limite=1
  branches.multi_site  actif=True  limite=1
  reports.advanced     actif=False
```

→ Le moteur de facturation est **fonctionnel de bout en bout** ; la 56ᵉ commande du mois est bien refusée en **402**. *(Défaut cosmétique : `remaining` peut devenir négatif, -6 ici.)*

---

## 6. Frontend React

### 6.1 Écrans et composants (26 fichiers dans `src/`, 8 830 lignes)

| Composant | Lignes | Rôle fonctionnel |
|---|---|---|
| `ServicesView` | **1 878** | Écran catalogue le plus riche : 5 familles de prestations (A. quotidien, B. traditionnel/cérémoniel, C. linge de maison, D. services complémentaires, E. spécialisé/luxe), ajustement de grille, express, collecte Abidjan (+1 500 F), activation/désactivation, CRUD complet |
| `demoData.ts` | 1 085 | Jeu de données de démonstration (commandes, clients, prestations, tenant courant) servant de repli hors-ligne |
| `CatalogManagerModal` | 887 | Création/édition avancée d'une prestation (prix, modèle de facturation, options) |
| `CounterReceiptModal` | 683 | Reçu comptoir / encaissement |
| `App.tsx` | 453 | Session, routage par état, chargement des données backend avec repli démo, toasts |
| `ThermalTicketModal` | 395 | Aperçu du ticket thermique 80 mm (ESC/POS textuel) |
| `SuperAdminView` | 389 | Espace éditeur : pressings souscrits, MRR, statuts, inscription d'un nouveau pressing |
| `BillingView` | 387 | « Mon abonnement » : plan, essai, droits, consommation du mois |
| `servicesApi.ts` | 336 | Client API catalogue + cache local + horodatage de synchronisation |
| `NewOrderModal` | 257 | Prise de dépôt (client, articles, acompte, moyen de paiement, échéance, notes) |
| `TicketVerificationView` | 255 | Page publique `/verify/:tenant/:order` (QR scanné) |
| `DashboardView` | 242 | KPI : dépôts en traitement, prêts au retrait, encaissements du jour, statut atelier, socle Go/PostgreSQL, recherche client |
| `Sidebar` / `Header` | 226 / 116 | Navigation (dashboard, commandes, clients, services, abonnement, superadmin) + recherche + thème |
| `OrdersView` | 218 | Liste des dépôts, filtres par statut, actions atelier, ticket |
| `CustomersView` | 193 | Fichier clients, solde dû, historique, dernière visite |
| `LoginModal` | 187 | Connexion (appel `/api/v1/auth/login`) |
| `ordersApi` / `customersApi` / `authApi` / `api` / `permissions` | 154/79/78/59/13 | Clients d'API et règle d'accès superadmin |

### 6.2 Gestion du thème et de l'UX

- `ThemeContext` (66 l.) : thème clair/sombre persistant, `ThemeToggle`.
- Animations `motion/react` (transitions de vues), icônes `lucide-react`.
- Ressources locales : avatar de démonstration, styles Tailwind 4 via plugin Vite.

### 6.3 Intégration front ↔ back (état réel)

| Flux | Branché ? | Repli si l'API échoue |
|---|---|---|
| `/health`, `/ready` (probe du socle) | ✅ | Message indicatif |
| Connexion + session JWT | ✅ (auto-login démo `gerant@pressing-ivoire.ci / pass123`) | Session de démonstration codée en dur |
| Commandes : liste / création / statut / paiement | ✅ | Données de démo |
| Clients : liste / création | ✅ | Données de démo |
| Prestations : liste / création / MAJ / suppression | ✅ | **Cache local + données de démo** (indispensable, voir bug #1) |
| Abonnement (`/me/entitlements`) | ✅ | — |
| Super-admin (`/admin/stats`) | Vue en place ; données d'illustration | — |
| Tickets thermiques | ✅ (génération texte 80 mm côté Go) | Aperçu local |

**Constat clé** : le repli sur données de démonstration masque le bug backend #1 — l'utilisateur voit un catalogue « plein » qui n'est pas celui de PostgreSQL.

### 6.4 Limites frontend

- **Zéro test** (aucun `*.test.tsx`, aucune configuration Vitest/Jest) et **pas de linting au-delà de TypeScript** (pas d'ESLint/Prettier).
- **Bundle non découpé** : 602 kB (160 kB gzip) en un seul chunk, avertissement Vite au build.
- **Pas de routeur** (react-router absent) : navigation par état interne + détection manuelle de `/verify/:tenant/:order` dans `App.tsx` → pas de deep-linking hors ticket.
- Warn Vite : `vite.config.ts` utilise `__dirname` (incompatible avec le futur chargeur de config natif).

---

## 7. Sécurité — état des lieux et tests d'intrusion réels

### 7.1 Mécanismes en place

| Contrôle | Implémentation | Évaluation |
|---|---|---|
| Authentification | JWT HS256 signé, expiration 24 h, claims `sub/tid/tname/email/name/role/exp` | ✅ Solide (secret à durcir — voir 7.3) |
| Mots de passe | **bcrypt** coût par défaut, longueur minimale 6 | ✅ Correct (porter à 8–12 recommandé) |
| Isolation tenant | `tenant_id` issu du JWT, filtres SQL systématiques, copie défensive mémoire | ✅ Vérifié par test d'intrusion |
| Anti-usurpation d'en-tête | `X-Tenant-ID` forgé → **403** | ✅ Vérifié en direct |
| RBAC | `RequireRole` (owner/manager/superadmin pour les mutations, superadmin pour l'admin SaaS) | ✅ Vérifié (403 owner sur `/admin/stats`) |
| Périmètre établissement | `RequireBranchScope` + `branch_memberships` | ⚠️ Fail-open pour les comptes de démo |
| CORS | Liste blanche d'origines (`ALLOWED_ORIGINS`), méthodes et en-têtes explicites | ✅ Correct, ⚠️ défaut `localhost:3000` si variable absente |
| Secrets | `.env` ignoré par Git, `.env.example` sans secret réel, URL de base masquée dans les logs | ✅ Bon |
| Robustesse | `Recoverer`, timeouts 15 s, arrêt gracieux vérifié | ✅ |

### 7.2 Tests d'intrusion exécutés (résultats bruts)

| Test d'attaque | Réponse observée | Verdict |
|---|---|---|
| Requête sans jeton sur `/api/v1/orders` | `401 {"error":"en-tête Authorization manquant…"}` | ✅ |
| En-tête `X-Tenant-ID` d'un autre établissement | `403 {"error":"violation d'isolation multi-tenant…"}` | ✅ |
| Lecture d'une commande d'un autre tenant (jeton tenant B sur ID tenant A) | `404 {"error":"commande introuvable"}` | ✅ (fuite nulle : existence non révélée) |
| Ticket thermique d'un autre tenant | `404` | ✅ |
| Tentative de paiement sur commande d'un autre tenant | `400 {"error":"commande introuvable"}` | ⚠️ Bon refus, **code incorrect** (devrait être 404/403) |
| `/api/v1/admin/stats` avec un jeton `owner` | `403 autorisations insuffisantes` | ✅ |
| `/api/v1/admin/stats` avec le jeton superadmin | `200` | ✅ |
| Mot de passe erroné | `401 identifiants invalides` | ✅ |
| Ticket inexistant (UUID aléatoire) en vérification publique | `404 {"valid":false}` | ✅ |

**Conclusion sécurité** : l'isolation multi-tenant tient face aux tests menés. Aucune fuite inter-tenant détectée.

### 7.3 Faiblesses de sécurité identifiées

| # | Gravité | Constat | Preuve / fichier |
|---|---|---|---|
| S1 | 🟠 Élevée | **Secret JWT de repli codé en dur** : `"pressingos-dev-secret-key-32-bytes-minimum"` (`main.go`) et `"pressingos-default-dev-secret-key-change-in-production-12345"` (`auth/jwt.go:39`). Si `JWT_SECRET` n'est pas défini en production, n'importe qui connaissant le dépôt peut forger des jetons. | `backend/internal/auth/jwt.go:39`, `cmd/api/main.go` |
| S2 | 🟡 Moyenne | **Aucun rate limiting** sur `/api/v1/auth/login` ni sur aucun endpoint (pas de `httprate`, pas de compte par IP) → brute-force de mots de passe facilité. | `grep throttle/limiter` = 0 occurrence |
| S3 | 🟡 Moyenne | **CORS par défaut permissif si mal configuré** : `AllowCredentials: true` avec repli `http://localhost:3000` — à surveiller si `ALLOWED_ORIGINS` est oublié en production. | `router.go` |
| S4 | 🟡 Moyenne | **Mode dégradé fail-OPEN** : sans PostgreSQL, `RequireTenantMembership` et `RequireBranchScope` autorisent tout porteur d'un JWT valide ; `looksLikeUUID` laisse passer les identités non-UUID. Risque de bascule silencieuse en production si la base est momentanément indisponible. | `access_middleware.go` |
| S5 | 🟡 Moyenne | **Erreurs SQL exposées au client** : `/api/v1/services` et `/api/v1/orders` renvoient le message pgx brut (hôte, utilisateur, base) dans le corps de la réponse 500 → divulgation d'informations d'infrastructure. | Test : `"error":"liste des commandes: failed to connect to host=127.0.0.1 user=pressing_app…"` |
| S6 | 🟢 Faible | Mot de passe minimal de 6 caractères seulement ; comptes de démonstration avec mots de passe triviaux (`pass123`, `admin123`) présents dans le code et actifs en repli. | `auth/service.go` |
| S7 | 🟢 Faible | Endpoint de vérification publique expose le **nom du client** et le détail des articles (`public/verify/...`) — à limiter (initiales/masquage) si le QR circule. | `docs/api.md` §4, test réel |

---

## 8. Qualité, build et tests — mesures réelles

### 8.1 Résultats d'exécution (11/10/2026)

| Vérification | Commande | Résultat |
|---|---|---|
| Compilation backend | `go build ./...` | ✅ **Succès** (binaire ~15,3 Mo) |
| Analyse statique | `go vet ./...` | ✅ Aucun signalement |
| Tests backend | `go test ./...` | ✅ **32/32 tests passent** (packages `auth`, `billing`, `catalog`, `config`, `health`, `http`, `order`) |
| Frontend — typage | `npm run lint` (`tsc --noEmit`) | ✅ **0 erreur** |
| Frontend — build | `npm run build` | ✅ `dist/index.html` 0,92 kB · `index.css` 73,16 kB · `index.js` 602,27 kB (gzip 160,83 kB) en 705 ms |
| Stack complet | `npm run dev` | ✅ Frontend 3000 + API Go 8085 + PostgreSQL, `/health` 200 via proxy |
| Migration SQL | 10 fichiers `up.sql` sur PostgreSQL 16.2 | ✅ **10/10 appliquées**, 19 tables créées |

### 8.2 Couverture de tests (mesurée, `go test -cover`)

| Package | Couverture | Commentaire |
|---|---|---|
| `config` | **82,6 %** | Bon |
| `health` | **80,0 %** | Bon |
| `http` (routeur + middlewares) | 43,1 % | Correct (inclut les tests d'isolation) |
| `billing` | 40,9 % | Essai/expiration couverts, quotas partiellement |
| `auth` | 27,5 % | JWT et bcrypt bien couverts, service partiellement |
| `order` | **22,6 %** | Écart majeur avec les « 94 % » annoncés par `rapports_audit_consolide.md` — l'annonce portait sur le seul fichier `service.go` en mémoire, pas sur le package (stores SQL, handlers, ticket, verify non testés) |
| `catalog` | 11,9 % | Idem : la couche PostgreSQL n'est pas couverte |
| `customer`, `pricelist`, `database`, `cmd/api` | **0 %** | Aucun test |

**Aucun test d'intégration HTTP de bout en bout en Go** : les tests `http` utilisent des doublures en mémoire. Les stores PostgreSQL (`persist.go`) ne sont testés nulle part — ce qui explique que le bug bloquant #1 (logger nil dans `catalog.NewStore`) soit passé inaperçu.

### 8.3 Dette technique

- 📄 **Documentation désynchronisée** :
  - `docs/api.md` annonce `v0.3.0-orders`, `/system/info` renvoie `0.4.0-catalog`, le log de démarrage affiche `v0.2.0-auth`.
  - `docs/installation.md` indique « API sur `localhost:8080` » et « frontend sur `localhost:3000` » : c'est vrai en direct, mais le flux `npm run dev` passe par le **port interne 8085** (non documenté).
  - `rapports_audit_consolide.md` présente des chiffres de couverture et de performance (94 %, −65 % de temps de verrouillage, 2 ms → 0,7 ms) **non reproductibles** par la mesure réelle.
  - `analyse.md` est un texte d'analyse généré conservant des extraits tronqués (« …9 lines truncated ») et des blocs non formatés.
- 🧩 **Incohérences mineures** : `/api/v1/orders/{id}/payments` renvoie 400 (au lieu de 404) pour une commande d'un autre tenant ; `remaining` de quota négatif ; `order_items.service_id` vide pour les prestations de démonstration (comportement prévu par la migration 000008).
- 🔁 **Duplication assumée** : les données de démonstration existent à la fois côté Go (seeds mémoire) et côté TypeScript (`demoData.ts`, 1 085  lignes) — risque de divergence silencieuse.
- 🧪 **Absence de CI/CD** : pas de workflow GitHub Actions, pas de `husky`, aucune porte automatique sur `go test`/`tsc`.

---

## 9. Vérification fonctionnelle de bout en bout (protocole et résultats)

**Environnement de test reconstitué pour l'audit** : PostgreSQL 16.2 réel + les 10 migrations du dépôt + binaire Go compilé depuis les sources + les deux tenants (démo et nouvellement inscrit).

| # | Scénario | Résultat |
|---|---|---|
| 1 | `GET /health` | ✅ 200 `{"status":"ok"}` |
| 2 | `GET /ready` (base connectée) | ✅ 200 `database:"connected"` |
| 3 | `GET /ready` (base arrêtée) | ✅ 503 `database:"disconnected"` |
| 4 | `GET /api/v1/system/info` | ✅ 200 (`0.4.0-catalog`, 5 modules) |
| 5 | `POST /auth/login` (gérant / superadmin) | ✅ 200 JWT + profil + établissement |
| 6 | `POST /auth/login` (mauvais mot de passe) | ✅ 401 |
| 7 | `GET /auth/me`, `/tenant/current` | ✅ 200 |
| 8 | `POST /auth/register-tenant` (2ᵉ pressing) | ✅ 201 → tenant `b62709cf…`, JWT owner, **abonnement essai 7 jours créé automatiquement** |
| 9 | Création client | ✅ 201 |
| 10 | **Cycle de commande complet** : création → 201 · `CMD-2026-001` · liste · détail · `PATCH` statut → `washing` · paiement acompte 1 000 F puis 500 F → statut `paid` | ✅ Tous 200, persistance PostgreSQL confirmée |
| 11 | **Ticket thermique 80 mm** | ✅ 200 `text/plain` — en-tête établissement, articles, totaux, acompte, solde, code-barres textuel |
| 12 | **Vérification publique QR** (ticket valide / inexistant) | ✅ 200 `valid:true` / 404 `valid:false` |
| 13 | Isolation inter-tenants (lecture, paiement, ticket) | ✅ 404/400 — aucune fuite |
| 14 | Droits effectifs + **quota 402** après expiration de l'essai | ✅ 402 `"plafond atteint (50)…"` à la 56ᵉ commande du mois |
| 15 | Grilles tarifaires : liste + création | ✅ 200 (backfill « Grille principale » présent) / 201 |
| 16 | RBAC superadmin vs owner sur `/admin/stats` | ✅ 200 / 403 |
| 17 | Validations métier (statut invalide, commande vide, paiement négatif) | ✅ 400 avec messages métier clairs |
| 18 | **`GET /api/v1/services` et `/api/v1/services/{id}`** | ❌ **500 — panique `nil pointer dereference`** (`catalog/service.go:986` et `:1039`, logger non initialisé) |
| 19 | `POST` / `PUT` / `DELETE /api/v1/services` | ✅ 201 / 200 / 200 (les écritures fonctionnent) |
| 20 | **Paiement de 999 999 F sur une commande de 1 500 F** | ❌ **Accepté (200)** — écriture incohérente en base |
| 21 | Application avec la base arrêtée (base « morte mais pool créé ») | ❌ Commandes/services/abonnement en **500** au lieu du repli mémoire annoncé ; `/health` reste 200 |

---

## 10. Anomalies et risques classés

### 🔴 P0 — À corriger avant toute mise en production

| ID | Anomalie | Impact | Correctif suggéré |
|---|---|---|---|
| **A1** | `catalog.NewStore()` renvoie `&Service{store: …}` **sans logger ni map** → panique à la première lecture (`GetServices`, `GetServiceByID`) | **Catalogue illisible en production** ; l'app retombe sur des données de démo ; écran « Grille tarifaire » et sélection de prestation faussés | Construire via `NewService()` puis `SetStore()` (modèle déjà utilisé par `order`/`auth`/`billing`), et ajouter un **test d'intégration sur les stores PostgreSQL** |
| **A2** | Décision « persistance vs repli mémoire » basée sur `dbPool != nil && dbPool.Pool != nil` **sans vérifier le `Ping`** | Base présente mais indisponible ⇒ 100 % des endpoints métier en 500, aucune bascule mémoire, logs mensongers | Décider sur le résultat du `Ping` (ou d'un `SELECT 1` court) ; sinon forcer le mode mémoire ; ajouter un test « DB down » |
| **A3** | **Surpaiement accepté** : 999 999 F encaissés sur une commande de 1 500 F | Incohérence comptable : `payments` ≠ `paid_amount` ; risque de fraude/erreur de caisse et de litige client | Refuser tout `amount > remaining_due` (400) ; option « trop-perçu » explicite si le métier l'exige |

### 🟠 P1 — À traiter à court terme

| ID | Anomalie | Impact | Correctif |
|---|---|---|---|
| **A4** | Secret JWT de repli en dur (voir S1) | Usurpation totale si `JWT_SECRET` oublié en production | **Refuser de démarrer** en `SERVER_ENV=production` sans `JWT_SECRET` fort |
| **A5** | Erreurs internes (pgx/SQL) renvoyées telles quelles au client (S5) | Divulgation d'infrastructure | Journaliser côté serveur, renvoyer un message générique |
| **A6** | Aucun rate limiting sur l'authentification (S2) | Brute-force | `httprate` par IP + verrouillage progressif |
| **A7** | Mode dégradé **fail-open** sur l'appartenance et le périmètre branche (S4) | En cas de panne DB, perte du contrôle d'appartenance | Passer en fail-closed pour les identités UUID, journaliser les contournements |
| **A8** | Couverture faible sur les zones sensibles (`order` 22,6 %, `customer`/`pricelist`/`database` 0 %, stores SQL non testés) | Régressions silencieuses (cf. A1) | Tests d'intégration avec base éphémère (Testcontainers ou base CI) |
| **A9** | Code 400 au lieu de 404/403 sur paiement inter-tenant | Incohérence d'API, fausse piste en supervision | Normaliser les codes de retour |

### 🟡 P2 — Dette / finition

| ID | Anomalie |
|---|---|
| A10 | `remaining` de quota négatif (-6) — clamper à 0 |
| A11 | Documentation désynchronisée (versions 0.2.0 / 0.3.0 / 0.4.0, port 8085 non documenté, chiffres de test invérifiables) |
| A12 | `RequireFeature` défini mais non branché → le verrouillage par plan (rapports, multi-sites, grilles) n'est pas appliqué aux endpoints |
| A13 | `catalog.seedDefaultServices()` est un stub vide : aucun service par défaut pour un nouveau tenant en mode mémoire |
| A14 | Aucun test frontend, pas d'ESLint/Prettier, bundle 602 kB non découpé, `__dirname` dans `vite.config.ts` |
| A15 | Pas de CI/CD (aucune exécution automatique des tests) |
| A16 | Endpoint public de vérification exposant le nom du client (S7) |
| A17 | `analyse.md` / `rapports_audit_consolide.md` obsolètes ou trompeurs (extraits tronqués, chiffres non reproductibles) |
| A18 | `npm install` échoue en résolution stricte (conflit `vite@8` ↔ `esbuild@^0.25` en devDependency) : `npm ci`/`npm install` nécessitent `--legacy-peer-deps` |

---

## 11. Écarts entre spécification/roadmap et réalité

| Fonctionnalité | Statut |
|---|---|
| Multi-tenant, JWT, RBAC, isolation | ✅ Livré et vérifié |
| Commandes, paiements, tickets 80 mm, vérification QR | ✅ Livré et vérifié |
| Catalogue enrichi + options (express, collecte, sous-traitance, devis) | ✅ Livré (lecture cassée en mode DB — A1) |
| Clients, unicité par tenant | ✅ Livré |
| Grilles tarifaires versionnées (snapshots JSONB) | ✅ Livré (backend + UI partielle) |
| Abonnement, plans, essai, quotas, matrice de droits | ✅ Livré (gating par endpoint non branché — A12) |
| Espace SuperAdmin (MRR, souscriptions) | 🟡 UI en place, données d'illustration non issues de la base |
| Multi-établissements (branches, memberships) | ✅ Schéma + middlewares livrés ; UI de gestion des branches absente |
| Notifications **WhatsApp / SMS** (Orange/MTN/Wave) | ❌ Non implémenté (planifié dans `rapports_audit_consolide.md` §6) |
| Rapports avancés / consolidés, exports | ❌ Non implémenté (droits déjà modélisés dans la matrice) |
| Dépenses, journal d'audit avancé, comptabilité | ❌ Non implémenté |
| Mode hors-ligne, impression ESC/POS matérielle | ❌ Non implémenté (ticket = texte 80 mm) |

---

## 12. Recommandations et feuille de route

### Sprint 0 — Correctifs bloquants (1 à 2 jours)

1. **A1** : faire construire `catalog.Service` par `NewService()` + `SetStore()` (comme `order`), ajouter un **test d'intégration lecture catalogue sur PostgreSQL**.
2. **A2** : conditionner le mode persistance au succès du `Ping` ; en échec ⇒ mode mémoire + log d'alerte ; test automatisé « base injoignable ».
3. **A3** : refuser les paiements supérieurs au solde restant dû (400 explicite).
4. **A4** : refuser le démarrage en production sans `JWT_SECRET` (≥ 32 octets).
5. **A5/A9** : erreurs génériques côté client, journalisation détaillée côté serveur, codes HTTP normalisés.

### Sprint 1 — Consolidation sécurité & qualité (1 semaine)

6. Rate limiting sur `/auth/login` (**A6**) et journalisation des échecs (IP, e-mail, horodatage).
7. Bascule des contrôles d'accès en fail-closed hors mode démo explicite (**A7**).
8. **Tests d'intégration** Go (HTTP + PostgreSQL réel) sur `order`, `catalog`, `customer`, `pricelist` ; viser ≥ 70 % sur les lecteurs/écrivains critiques.
9. **CI GitHub Actions** : `go vet`, `go test -race`, `tsc --noEmit`, `vite build` à chaque push (**A15**).
10. Brancher `RequireFeature` sur les routes concernées (**A12**) + clamp des quotas (**A10**).

### Sprint 2 — Frontend & exploitation (1 à 2 semaines)

11. Découpage du bundle (imports dynamiques), ESLint/Prettier, **premiers tests** de composants (Vitest + Testing Library), introduction d'un routeur (react-router) pour le deep-linking.
12. Écran d'administration des établissements (branches) et des utilisateurs (déjà adossés en base).
13. SuperAdmin connecté aux données réelles (`/admin/stats` alimenté par les tables `tenants`/`subscriptions`).
14. Observabilité : métriques Prometheus (latence, taux d'erreur, 402/403), logs structurés JSON, `request_id` propagé.
15. Sauvegardes PostgreSQL automatisées + restauration testée ; procédure de migration versionnée (outil type `golang-migrate` plutôt que `docker-entrypoint-initdb.d`).

### Sprint 3 — Fonctionnalités manquantes (backlog produit)

16. Notifications **WhatsApp Business API / SMS** (Orange CI, MTN) + statuts d'envoi dans la commande.
17. Rapports avancés/consolidés et exports (droits déjà prévus par la matrice).
18. Module dépenses & journal d'audit avancé.
19. Impression ESC/POS native (imprimante thermique réseau/USB) et mode hors-ligne (PWA + file d'attente de synchronisation).

### Hygiène documentaire

20. Réconcilier `docs/` avec le code (versions, ports, endpoints, couverture réelle), **supprimer ou archiver** `analyse.md` et `rapports_audit_consolide.md` (chiffres non reproductibles), et documenter le flux de développement réel (`npm run dev` → port interne 8085, `--legacy-peer-deps`).

---

## 13. Conclusion

PressingOS est un projet **bien architecturé, ambitieux et déjà riche fonctionnellement** : le socle Go est propre et cohérent, le schéma de base de données est sérieux (19 tables, migrations idempotentes, isolation multi-tenant réelle), le moteur commercial (plans, essai, quotas, droits) fonctionne de bout en bout, et les tests d'intrusion menés n'ont révélé **aucune fuite inter-tenant**.

Mais l'application **n'est aujourd'hui pas exploitable en configuration PostgreSQL** : deux défauts bloquants (lecture du catalogue en panique, absence du repli mémoire annoncé) la condamnent à fonctionner « en apparence » sur des données de démonstration, tandis qu'une anomalie financière (surpaiement accepté) peut créer des écritures comptables incohérentes. Ces trois points sont tous corrigibles en quelques jours — le premier par un simple alignement de constructeur, le deuxième par une vérification de `Ping` au démarrage, le troisième par une validation métier.

**Avec ces correctifs, la couverture d'intégration qui manque (stores SQL) et une CI, le projet change de catégorie** : il passerait d'un prototype de démonstration convaincant à un SaaS multi-tenant prêt pour un pilote client réel en Côte d'Ivoire.

---

## Annexe A — Commandes de reproduction utilisées pour cet audit

```bash
# --- Frontend ---
cd /home/user/PRESSPRO
npm install --legacy-peer-deps        # npm ci/install échoue en résolution stricte (A18)
npm run lint                          # tsc --noEmit  → 0 erreur
npm run build                         # vite build    → dist/ (602 kB JS)

# --- Backend ---
cd backend
go vet ./...                          # aucun signalement
go test ./...                         # 32/32 tests OK
go test -cover ./...                  # couverture par package (§8.2)
go build -o bin/api ./cmd/api/main.go

# --- Base de données (équivalent du docker compose du projet) ---
psql -f migrations/000001_init_schema.up.sql   # … jusqu'à 000010 (ordre croissant)

# --- Stack complet ---
cd .. && DATABASE_URL="postgres://pressing_app:…@127.0.0.1:5432/pressing_os?sslmode=disable" npm run dev
curl http://localhost:3000/health     # relais du proxy Express vers le binaire Go
```

## Annexe B — Matrice des plans (extraite de la migration 000006, confirmée par l'API)

| Droit | FREE | PRO | BUSINESS | Essai 7 j |
|---|---|---|---|---|
| `orders.create` | ✅ | ✅ | ✅ | ✅ |
| `orders.monthly.max` | **50 / mois** | illimité | illimité | illimité |
| `users.max` | 2 | 5 | 15 | 15 |
| `catalog.price_lists` | 1 | 3 | 10 | 10 |
| `branches.multi_site` | 1 | 1 | 3 | 3 |
| `reports.advanced` | ❌ | ✅ | ✅ | ✅ |
| `reports.consolidated` | ❌ | ❌ | ✅ | ✅ |
| `expenses.advanced`, `pricing.pro`, `audit.advanced` | selon plan | — | — | ✅ |
| Prix mensuel | 0 F | 45 000 F | 75 000 F | — |

## Annexe C — Les 32 tests Go (tous verts)

- **auth** (6) : génération/vérification JWT · jeton expiré · signature invalide · hachage bcrypt · login · inscription tenant
- **order** (8) : initialisation · commande vide · création réussie · commande introuvable · statut invalide · changement de statut · ajout de paiement · **isolation multi-tenant**
- **catalog** (6) : initialisation · liste par défaut · service introuvable · **isolation multi-tenant** · filtre catégorie · recherche
- **billing** (3) : l'essai ouvre le niveau BUSINESS · expiration de l'essai sans purge · borne de l'essai
- **http** (4) : authentification + isolation tenant · contrôle de rôle · routes · quota FREE après essai
- **health** (3) : liveness · readiness OK · readiness en échec
- **config** (2) : valeurs par défaut · validation

---

*Rapport produit par analyse statique du dépôt et exécution réelle de la pile (Go 1.25.5, PostgreSQL 16.2, Node 22, Vite 8). Tous les résultats marqués « vérifié » l'ont été en exécution le 11 octobre 2026.*
