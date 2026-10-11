# Guide d'Installation et de Démarrage Local (macOS)

> **Deux façons de lancer le projet**
> 1. **Full-stack en une commande** — `npm run dev` : Express sert le SPA sur **http://localhost:3000** et lance lui-même le binaire Go (`backend/bin/api`) sur le port interne **8085**, qu'il proxifie (`/health`, `/ready`, `/api/*`). C'est le mode recommandé.
> 2. **Mode séparé** — `cd backend && go run cmd/api/main.go` expose l'API directement sur le port **8080** (valeur par défaut de `PORT`), et `npm run dev` reste utilisable côté frontend.
>
> Mise en garde production : `SERVER_ENV=production` **exige** `JWT_SECRET` (32 caractères minimum) — le serveur refuse de démarrer sans ce secret.

> Notifications clients : `NOTIFICATIONS_GATEWAY_URL` (et son jeton facultatif
> `NOTIFICATIONS_GATEWAY_TOKEN`) active l'envoi réel WhatsApp/SMS par webhook. Laisser vide
> pour rester en mode journalisé (« simule »), dans lequel l'interface du comptoir propose
> l'ouverture manuelle de WhatsApp / SMS.

Ce guide pas à pas est conçu pour vous accompagner sur votre **MacBook sous macOS** avec **VS Code** et le terminal.

---

## 1. Prérequis sur votre Mac

Avant de commencer, vérifiez que vous disposez des outils nécessaires. Ouvrez l'application **Terminal** (ou le terminal intégré de VS Code avec `Ctrl + ~`).

### A. Outils de développement Apple
```bash
xcode-select --install
```
*Si la commande indique que les outils sont déjà installés, vous pouvez continuer.*

### B. Gestionnaire de paquets Homebrew (recommandé sur Mac)
Si vous n'avez pas Homebrew, installez-le avec :
```bash
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
```

### C. Installation des langages et outils
Installez Go, Node.js et Docker :
```bash
brew install go
brew install node
```
Pour Docker, téléchargez et installez **Docker Desktop pour Mac (Apple Silicon / Intel)** depuis :
[https://www.docker.com/products/docker-desktop/](https://www.docker.com/products/docker-desktop/)

Lancez Docker Desktop une première fois pour qu'il s'initialise.

---

## 2. Vérification des versions
Exécutez dans votre terminal :
```bash
go version         # Doit afficher go1.22 ou supérieur
node -v            # Doit afficher v20 ou v22
npm -v             # Doit afficher v10 ou supérieur
docker --version   # Doit afficher Docker version 24+
docker compose version
```

---

## 3. Configuration du Projet

Dans le dossier racine du projet `pressing-saas/` :

1. **Créer le fichier d'environnement local** :
```bash
cp .env.example .env
```
*(Le fichier `.env` est ignoré par Git pour des raisons de sécurité).*

---

## 4. Démarrage de la Base de Données PostgreSQL

Dans la racine du projet :
```bash
# Lancer PostgreSQL en arrière-plan
docker compose up -d

# Vérifier que le conteneur tourne et que le statut est "healthy"
docker compose ps

# Voir les journaux (logs) de PostgreSQL
docker compose logs -f db
```

Pour arrêter la base de données :
```bash
docker compose down
```
*Remarque : Les données sont sauvegardées dans un volume persistant `postgres_data`. Elles ne seront pas perdues.*

---

## 5. Démarrage du Backend Go

Ouvrez un terminal et rendez-vous dans le dossier `backend` :
```bash
cd backend

# Télécharger les dépendances Go
go mod tidy

# Lancer les tests unitaires
go test ./...

# Lancer le serveur d'API
go run cmd/api/main.go

# Lancer les tests unitaires et d'intégration
go test ./...
TEST_DATABASE_URL="postgres://pressing_app:...@localhost:5432/pressing_os?sslmode=disable" \
  go test -tags=integration ./internal/catalog/...
```
Le terminal affichera :
```
[PressingOS-API] Démarrage du socle backend PressingOS...
[PressingOS-API] Configuration chargée. Environnement: development, Port: 8080
[PressingOS-API] Cible base de données: postgres://pressing_app:********@localhost:5432/pressing_os?sslmode=disable
[PressingOS-API] Serveur HTTP à l'écoute sur http://0.0.0.0:8080
```

Vous pouvez tester immédiatement dans un autre terminal :
```bash
curl http://localhost:8080/health
# Réponse : {"status":"ok","service":"pressingos-api","timestamp":"..."}

curl http://localhost:8080/ready
# Réponse : {"status":"ready","service":"pressingos-api","database":"connected","timestamp":"..."}
```

---

## 6. Démarrage du Frontend

Ouvrez un autre onglet de terminal : si l'API tourne déjà seule sur `8080`, lancez uniquement
le frontend ; sinon `npm run dev` suffit (il lance aussi le binaire Go sur le port interne 8085).
```bash
npm ci --legacy-peer-deps   # npm ci/install strict échoue (conflit vite@8 / esbuild@0.25)
npm run dev
```
Ouvrez votre navigateur web à l'adresse : **`http://localhost:3000`**.

Vous verrez alors le tableau de bord de démonstration PressingOS connecté à votre API locale !
