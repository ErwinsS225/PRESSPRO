# PressingOS — Architecture Technique

Ce document détaille l'architecture globale, les choix technologiques et la stratégie d'isolation multi-tenant de **PressingOS**.

---

## 1. Vision et Modèle Produit

PressingOS est un logiciel ERP en mode SaaS (Software as a Service) conçu spécifiquement pour moderniser et fiabiliser la gestion quotidienne des pressings, blanchisseries et buanderies en Côte d'Ivoire.

Dans ce modèle :
- Chaque pressing indépendant constitue un **Tenant** (établissement isolé).
- L'éditeur du logiciel (PressingOS) dispose d'une vision globale (Super Administration) pour gérer les souscriptions, l'activation des comptes et les métriques de la plateforme.

---

## 2. Principes Fondamentaux : Monolithe Modulaire

Pour respecter l'échéance d'un prototype fonctionnel en 5 jours tout en garantissant une base maintenable et évolutive, nous avons retenu une architecture en **Monolithe Modulaire** :

- **Pas de microservices prématurés** : évite la complexité de synchronisation réseau, de déploiement multi-conteneurs et de latence superflue.
- **Séparation stricte des responsabilités** : chaque domaine métier est encapsulé dans son propre module (`internal/`).
- **Communication par interfaces et contrats clairs** : simplifie le test unitaire et le remplacement de composants.

```
                           +-------------------------------------+
                           |            Navigateur Client        |
                           |   (Next.js App Router / TypeScript) |
                           +-------------------------------------+
                                              |
                                              | HTTPS / JSON (REST)
                                              v
                           +-------------------------------------+
                           |            API REST Go              |
                           |     (Chi Router + Middlewares)      |
                           +-------------------------------------+
                               /              |               \
                              v               v                v
                       +-------------+ +-------------+ +-------------+
                       |   Health    | |   Tenants   | |   Orders    |
                       |  /ready     | |   & Users   | |  & Paiements|
                       | (Socle Actif)| |   (Futur)   | |   (Futur)   |
                       +-------------+ +-------------+ +-------------+
                                              |
                                              | pgxpool (Connexions SQL)
                                              v
                           +-------------------------------------+
                           |        PostgreSQL 16 (Docker)       |
                           |   (Schéma Relationnel Multi-Tenant) |
                           +-------------------------------------+
```

---

## 3. Stratégie Multi-Tenant (Isolation des Données)

### A. Modèle retenu : Base partagée avec discrimination par `tenant_id`
- **Pourquoi ?** Un modèle à base unique par client compliquerait inutilement les migrations et la gestion des ressources en phase de lancement.
- **Règle absolue** : Toutes les tables métier (`customers`, `services`, `orders`, `order_items`, `payments`, `users`) contiennent obligatoirement une colonne clé étrangère `tenant_id NOT NULL REFERENCES tenants(id)`.

### B. Sécurité et Contrôle d'Accès Côté Serveur
1. **Extraction du Tenant** : Le `tenant_id` n'est JAMAIS accepté aveuglément depuis les paramètres d'URL de manière non authentifiée.
2. **Contexte de Requête** : Lors de l'authentification (future partie 2), le `tenant_id` sera extrait du jeton de session vérifié et injecté dans le contexte HTTP (`context.Context`).
3. **Filtres SQL Systématiques** : Chaque requête SQL (`SELECT`, `UPDATE`, `DELETE`) applique obligatoirement `WHERE tenant_id = $1`.
4. **Index Dédiés** : Des index composites (ex: `CREATE INDEX idx_orders_tenant_status ON orders(tenant_id, status)`) garantissent que les requêtes ne scannent jamais les données des autres pressings.

---

## 4. Structure du Code Backend Go

```
backend/
├── cmd/
│   └── api/
│       └── main.go           # Point d'entrée, initialisation et Graceful Shutdown
├── internal/
│   ├── auth/                 # JWT, bcrypt, inscription d'établissement (onboarding), équipe
│   ├── billing/              # Plans, droits effectifs, quotas, matrice des fonctionnalités
│   ├── branch/               # Établissements multi-sites (branches, rattachement des membres)
│   ├── catalog/              # Catalogue de prestations et grille tarifaire
│   ├── config/               # Lecture typée des variables d'environnement (.env)
│   ├── customer/             # Fichier clients du pressing
│   ├── database/             # Gestion du pool pgxpool et des requêtes
│   ├── expense/              # Dépenses d'exploitation (postes, filtres, totaux)
│   ├── health/               # Endpoints /health (liveness) et /ready (readiness)
│   ├── httperr/              # Réponses d'erreur normalisées (aucune fuite technique)
│   ├── order/                # Commandes, paiements, ticket thermique 80 mm
│   ├── pricelist/            # Grilles tarifaires versionnées (snapshots JSONB)
│   ├── ratelimit/            # Limiteur de débit (anti-force brute sur /auth/login)
│   ├── report/               # Rapports d'activité (encaissements, marge, consolidation)
│   └── http/                 # Routeur Chi, middlewares (CORS, timeouts, JWT, RBAC)
├── migrations/               # Migrations SQL numérotées (up/down)
├── go.mod                    # Définition du module Go et dépendances
└── Makefile                  # Commandes raccourcies de test, build, formatage
```

---

## 5. Démarrage, mode de persistance et onboarding

### Choix du mode de persistance (au démarrage)
1. Le pool `pgxpool` est créé, puis **un `Ping` est exécuté** : la création du pool ne prouve pas que la base répond.
2. `Ping` réussi → tous les services sont adossés à PostgreSQL (`dbReady = true`).
3. `Ping` échoué → **bascule complète en repli mémoire** (données de démonstration, aucune écriture persistée) et `/ready` renvoie `503`. Les logs l'annoncent explicitement ; `/health` reste `200` (processus vivant).
4. Sans `DATABASE_URL` joignable, l'API reste utilisable (mode démonstration) au lieu de renvoyer des erreurs 500.

### Sécurité du démarrage
`SERVER_ENV=production` impose un `JWT_SECRET` d'au moins 32 caractères : sans lui, le processus refuse de démarrer. Un secret de développement n'est utilisé qu'en environnement non-production, avec un avertissement explicite dans les logs.

### Onboarding d'un nouvel établissement
L'inscription (`POST /api/v1/auth/register-tenant`) exécute une transaction unique : tenant → branche principale → compte propriétaire → appartenances (tenant + branche) → abonnement d'essai 7 jours. Ensuite, un **hook d'onboarding** injecté à l'assemblage (`SetOnTenantRegistered`) initialise, de façon idempotente :
- la **grille tarifaire par défaut** du catalogue (prestations courantes du pressing) ;
- la **grille tarifaire principale** (`price_lists`) et sa **première version figée** (snapshot JSONB du catalogue).

Un pressing fraîchement inscrit peut donc enregistrer des commandes immédiatement.

### Politique de réponse d'erreur (paquet `httperr`)
- erreurs métier → `400`/`402`/`409` avec message fonctionnel ;
- ressource inconnue **ou appartenant à un autre établissement** → `404` (aucune fuite d'existence) ;
- erreurs techniques (SQL, réseau, transaction) → journalisées côté serveur, `500` avec message générique.

### Contrôle d'appartenance (fail-closed)
Le middleware `MembershipChecker` refuse par défaut dès qu'une base est fournie :
- appartenance absente ou non `active` en base → refus ;
- utilisateur inconnu de la base **et** non déclaré comme compte de démonstration → refus (jeton orphelin) ;
- identifiant hors format UUID (jeton forgé) → refus, sans requête ;
- erreur de base → refus.

Les comptes de la vitrine embarquée (`auth.Service.IsDemoAccount`) sont autorisés **nominativement** : le mode démonstration est préservé sans jamais ouvrir la porte à un identifiant arbitraire. Sans base connectée, aucune vérification n'est possible et le jeton signé portant le `tenant_id` vaut appartenance.

### Garde-fous commerciaux
`RequireFeature` (droit booléen) et `RequireFeatureQuota` (plafond consommé, mesuré sur les données réelles) protègent les modules payants ; `RequireActiveSubscription` gèle les modules métier d'un établissement suspendu par l'éditeur. Les lectures restent accessibles, et le superadmin est dispensé des deux premiers contrôles.

Modules gardés et compteurs associés :

| Module | Garde-fou | Mesure réelle |
|---|---|---|
| Équipe (comptes employés) | `RequireFeatureQuota(users.max)` | `COUNT(users) WHERE status = 'active'` (propriétaire inclus) |
| Multi-sites (établissements) | `RequireFeatureQuota(branches.multi_site)` | `COUNT(branches) WHERE status <> 'archived'` |
| Grilles tarifaires | `RequireFeatureQuota(catalog.price_lists)` | `COUNT(price_lists)` |
| Commandes du mois | `RequireFeatureQuota(orders.monthly.max)` | `COUNT(orders)` sur le mois courant |
| Rapports avancés / consolidation | `RequireFeature(reports.advanced)` / `reports.consolidated` | droit booléen de la formule |
| Dépenses d'exploitation | `RequireFeature(expenses.advanced)` | droit booléen de la formule |

Chaque compteur doit être **branché sur un cas explicite** de `billing.Service.FeatureUsage` : une clé oubliée renvoie une consommation nulle et désactive silencieusement le plafond (voir `docs/correctifs-2026-10.md`, session 4).

### Espace SuperAdmin
`GET /admin/stats`, `GET /admin/tenants` et `PATCH /admin/tenants/{id}/status` travaillent sur les données réelles (`billing.Service.AdminStats`, `TenantsOverview`, `SetTenantStatus`). En mode dégradé, les indicateurs sont calculés sur les abonnements en mémoire et signalés par `data_source: "memoire"`.

### Limitation de débit
`POST /api/v1/auth/login` est protégé (`internal/ratelimit`) : 10 tentatives par couple (IP, email) et 40 par IP sur 5 minutes, réponse `429` + `Retry-After`. Une connexion réussie réinitialise le compteur. Le limiteur est en mémoire (mono-instance) ; en déploiement multi-instances, le remplacer par un compteur partagé.

---

## 6. Arrêt Gracieux (Graceful Shutdown)

Lorsqu'un signal d'arrêt est reçu (`SIGINT` via Ctrl+C ou `SIGTERM` via Docker/Kubernetes) :
1. Le serveur cesse d'accepter de nouvelles requêtes entrantes.
2. Un délai de 10 secondes maximum est accordé aux requêtes en cours pour se terminer normalement.
3. Le pool de connexions PostgreSQL `pgxpool` est fermé proprement.
4. Aucun processus orphelin ni transaction interrompue à chaud.

---

## 7. Qualité, outillage et intégration continue

- **Backend** : `go vet ./...` puis `go test ./...` (tests unitaires, dont les modules métier) et `go test -tags=integration` sur un **PostgreSQL 16 réel** (migrations `000001` → `000012` appliquées) ; `gofmt` imposé.
- **Frontend** : `npm run lint` (TypeScript, mode `--noEmit`), `npm run lint:eslint` (ESLint 9, configuration plate + `eslint-config-prettier`), `npm run format:check` (Prettier), `npm test` (Vitest + Testing Library — **11 fichiers / 94 tests**, vues métier, clients API et flux d'impression ESC/POS) et `npm run build`.
- **Intégration continue** (`.github/workflows/ci.yml`) : les deux chaînes tournent à chaque push et pull request, avec un service PostgreSQL 16 pour les tests d'intégration Go.
- Les vérifications **live** (proxy `:3000` → API `:8085` → PostgreSQL) complètent les tests : parcours métier complet, plafonds de formule, isolation inter-établissements, export des rapports et périmètre d'exploitation par établissement (`X-Branch-ID`).
- **Impression des tickets** : le rendu 80 mm est disponible en HTML (modale ticket) et en **flux ESC/POS** (`src/lib/escpos.ts`) — CP437, encodage tolérant aux accents absents de la table, transport **WebUSB** (Chrome/Edge sur ordinateur) avec aperçu du flux avant envoi ; l'impression HTML reste le repli.
