# PressingOS — ERP SaaS Pressings & Buanderies (Côte d'Ivoire)

Bienvenue dans le dépôt officiel de **PressingOS**, une plateforme SaaS multi-tenant conçue pour moderniser, digitaliser et simplifier la gestion quotidienne des pressings, blanchisseries et buanderies en Côte d'Ivoire.

Ce projet est structuré selon les standards modernes du génie logiciel :
- **Frontend** : Next.js / React avec TypeScript, Tailwind CSS, animations fluides Motion et design épuré Vercel/Linear.
- **Backend** : API REST performante et robuste écrite intégralement en **Go** avec le routeur **Chi** et le pilote PostgreSQL **pgx**.
- **Base de Données** : **PostgreSQL 16** conteneurisé avec Docker Compose, migrations SQL versionnées et isolation multi-tenant.
- **Architecture** : Monolithe Modulaire préparé pour un développement rapide et sécurisé.

---

## 📁 Structure du Projet

```text
pressing-saas/
├── frontend/ (ou src/)       # Interface web Next.js / React
│   ├── components/           # Composants UI (Navigation, Dashboard, Diagnostic)
│   ├── lib/                  # Utilitaires et connecteurs API
│   └── index.html            # Entrée HTML
├── backend/                  # API REST intégrale en Go
│   ├── cmd/api/main.go       # Point d'entrée serveur et arrêt gracieux
│   ├── internal/
│   │   ├── config/           # Configuration typée et validation
│   │   ├── database/         # Pool de connexions PostgreSQL pgx
│   │   ├── health/           # Endpoints de santé /health et /ready
│   │   └── http/             # Routeur Chi, middlewares CORS et requêtes
│   ├── migrations/           # Schémas SQL up/down versionnés
│   ├── go.mod                # Dépendances du module Go
│   └── Makefile              # Commandes de test, formatage et compilation
├── docs/                     # Documentation complète
│   ├── architecture.md       # Principes monolithiques et isolation multi-tenant
│   ├── installation.md       # Guide pas-à-pas débutant pour Mac (macOS / VS Code)
│   ├── database.md           # Schéma relationnel, tables et intégrité
│   └── api.md                # Spécification des routes de l'API REST
├── compose.yaml              # Déploiement local de PostgreSQL avec healthcheck
├── .env.example              # Modèle des variables d'environnement (sans secrets)
├── .gitignore                # Règles d'exclusion Git adaptées à Go, Mac et Node
└── README.md                 # Ce document
```

---

## 🚀 Démarrage Rapide sur macOS (MacBook)

Pour un guide complet détaillé, consultez [docs/installation.md](docs/installation.md).

### 1. Cloner et configurer l'environnement
```bash
# Dans la racine du projet
cp .env.example .env
```

### 2. Démarrer PostgreSQL avec Docker
```bash
docker compose up -d

# Vérifier la santé du conteneur
docker compose ps
```

### 3. Démarrer le Backend Go
Dans un premier terminal :
```bash
cd backend
go test ./...        # Exécuter les tests unitaires
go run cmd/api/main.go
```
*Le serveur répond sur `http://localhost:8080`.*

### 4. Démarrer le Frontend Web
Dans un second terminal (à la racine du projet) :
```bash
npm run dev
```
*L'application est accessible sur `http://localhost:3000`.*

---

## 🧪 Tests et Vérifications Automatisées

### Vérification du Backend Go
```bash
cd backend
go fmt ./...         # Vérifie et formate le code
go vet ./...         # Analyse statique des anomalies
go test -v ./...     # Exécute tous les tests unitaires (config, health, router)
```

### Vérification du Frontend
```bash
npm run lint         # Vérification TypeScript
npm run build        # Compilation de production
```

### Vérification des Endpoints HTTP
```bash
# Vérification de l'écoute du serveur Go
curl -i http://localhost:8080/health

# Vérification de la liaison active à PostgreSQL
curl -i http://localhost:8080/ready

# Métadonnées système
curl -i http://localhost:8080/api/v1/system/info
```

---

## 🛡️ Sécurité et Bonnes Pratiques Multi-Tenant

1. **Aucun secret dans Git** : Tous les mots de passe et clés d'accès sont injectés par variables d'environnement.
2. **Logs sécurisés** : La méthode `RedactedDatabaseURL()` masque les identifiants de base de données dans la console.
3. **Isolation par tenant_id** : Les requêtes SQL associent toujours les données au pressing actif.
4. **Arrêt gracieux (Graceful Shutdown)** : Les transactions et connexions en cours sont libérées proprement sans perte de données.
