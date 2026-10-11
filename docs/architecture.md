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
│   ├── config/               # Lecture typée des variables d'environnement (.env)
│   ├── database/             # Gestion du pool pgxpool et des requêtes
│   ├── health/               # Endpoints /health (liveness) et /ready (readiness)
│   └── http/                 # Routeur Chi, middlewares (CORS, timeouts, logger)
├── migrations/               # Migrations SQL numérotées (up/down)
├── go.mod                    # Définition du module Go et dépendances
└── Makefile                  # Commandes raccourcies de test, build, formatage
```

---

## 5. Arrêt Gracieux (Graceful Shutdown)

Lorsqu'un signal d'arrêt est reçu (`SIGINT` via Ctrl+C ou `SIGTERM` via Docker/Kubernetes) :
1. Le serveur cesse d'accepter de nouvelles requêtes entrantes.
2. Un délai de 10 secondes maximum est accordé aux requêtes en cours pour se terminer normalement.
3. Le pool de connexions PostgreSQL `pgxpool` est fermé proprement.
4. Aucun processus orphelin ni transaction interrompue à chaud.
