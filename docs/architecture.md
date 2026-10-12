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
3. `Ping` échoué → **bascule complète en repli mémoire** (aucune entreprise fictive pré-chargée : les écrans restent vides, aucune écriture persistée) et `/ready` renvoie `503`. Les logs l'annoncent explicitement ; `/health` reste `200` (processus vivant).
4. Sans `DATABASE_URL` joignable, l'API reste utilisable (mode dégradé) au lieu de renvoyer des erreurs 500 ; aucune entreprise ni donnée de démonstration n'est injectée pour autant (les écrans restent en état vide).

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
| Commandes sur 30 jours glissants | `RequireFeatureQuota(orders.monthly.max)` | `COUNT(orders)` non annulées sur la fenêtre de 30 jours ancrée sur l'abonnement (début d'essai, sinon création de l'abonnement, sinon création de l'établissement) |
| Rapports avancés / consolidation | `RequireFeature(reports.advanced)` / `reports.consolidated` | droit booléen de la formule |
| Dépenses d'exploitation | `RequireFeature(expenses.advanced)` | droit booléen de la formule |
| Notifications clients (envoi) | `RequireFeature(notifications.advanced)` | droit booléen de la formule — la **lecture du journal reste libre** |

Chaque compteur doit être **branché sur un cas explicite** de `billing.Service.FeatureUsage` : une clé oubliée renvoie une consommation nulle et désactive silencieusement le plafond (voir `docs/correctifs-2026-10.md`, session 4).

### Module Notifications clients (paquet `internal/notification`)

| Élément | Rôle |
|---|---|
| `Service` | Compose et journalise les messages ; sources injectées : `SourceCommandes` (`GetOrderByID` — isolation par pressing), `Passerelle` (facultatif), `SetDroitVerificateur(billing, notifications.advanced)`, `SetNomPressing` (nom du pressing lu en base, repli « votre pressing ») |
| `Passerelle` | Interface d'envoi réel ; `NewHTTPPasserelle(url, jeton, timeout)` effectue un POST JSON (`to`, `channel`, `body`, `order_id`, `tenant_id`) avec `Authorization: Bearer` lorsque le jeton est configuré, délai 5 s par défaut |
| `Store` / `dbStore` | Journal persisté (table `notifications`) ; repli mémoire si la base n'est pas prête |
| `Handler` | `GET /api/v1/notifications` (journal, filtres), `POST /api/v1/notifications` (composition + envoi, gardé par `RequireFeature`) |
| Hook `order.SetHookStatut` | Les transitions `ready`/`delivered` déclenchent `NotifierTransition` en **goroutine isolée** (10 s maximum, panne avalée, `recover`) : l'accusé de réception du comptoir n'attend jamais la passerelle |

Sans passerelle configurée (`NOTIFICATIONS_GATEWAY_URL` absente), le message reste
**journalisé** (`status: "simule"`) et l'interface du comptoir propose l'ouverture manuelle
de WhatsApp / SMS : la fonctionnalité reste utile en formule d'essai comme en local, et un
échec de la passerelle est tracé (`status: "echec"`, `detail`) sans bloquer l'atelier.

### Espace SuperAdmin
`GET /admin/stats`, `GET /admin/tenants`, `PATCH /admin/tenants/{id}/status` et `DELETE /admin/tenants/{id}` travaillent sur les données réelles (`billing.Service.AdminStats`, `TenantsOverview`, `SetTenantStatus`, `DeleteTenant`). En mode dégradé, les indicateurs sont calculés sur les abonnements en mémoire et signalés par `data_source: "memoire"` ; aucune entreprise fictive n'est pré-enregistrée, les compteurs démarrent donc à zéro. La suppression (`DeleteTenant`) est transactionnelle : les commandes, leurs lignes et les paiements sont purgés explicitement (ces tables ne cascadent pas depuis `tenants`), le reste suit les `ON DELETE CASCADE`.

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

- **Backend** : `go vet ./...` puis `go test ./...` (tests unitaires, dont les modules métier) et `go test -tags=integration` sur un **PostgreSQL 16 réel** (migrations `000001` → `000015` appliquées) ; `gofmt` imposé.
- **Frontend** : `npm run lint` (TypeScript, mode `--noEmit`), `npm run lint:eslint` (ESLint 9, configuration plate + `eslint-config-prettier`), `npm run format:check` (Prettier), `npm test` (Vitest + Testing Library — **21 fichiers / 162 tests**, vues métier, clients API, analyses du tableau de bord, dessin réel des graphiques, modale de notification client, garde-fou du jeu de styles, fumée de l'application et flux d'impression ESC/POS) et `npm run build`.
- **Intégration continue** (`.github/workflows/ci.yml`) : les deux chaînes tournent à chaque push et pull request, avec un service PostgreSQL 16 pour les tests d'intégration Go.
- Les vérifications **live** (proxy `:3000` → API `:8085` → PostgreSQL) complètent les tests : parcours métier complet, plafonds de formule, isolation inter-établissements, export des rapports, périmètre d'exploitation par établissement (`X-Branch-ID`) et envoi réel des notifications via une passerelle de test (payload, jeton, journal, déclenchement automatique `ready`/`delivered`).
### Bibliothèque d'interface : primitives shadcn/ui et composants ReUI

L'interface s'appuie sur des **primitives locales** (`src/components/ui/`) — bouton, carte,
tableau, séparateur, menu déroulant, carrousel Embla, enveloppe de graphiques — et sur des
**composants ReUI** (`src/components/reui/`) : badge multi-variantes, `Frame`/`FramePanel`,
carte d'indicateur, carte de raccourci, tableau d'abonnements, anneau de répartition,
répartition par statut, entonnoir de conversion et carrousel des dépôts récents.
Le chemin `@/…` est résolu vers `src/` (Vite, Vitest et TypeScript), suivant la convention
shadcn — aucune autre partie du code n'utilisait cet alias.

Les **jetons de design** shadcn (`--background`, `--card`, `--muted-foreground`,
`--destructive`, `--chart-1` … `--chart-5`) sont déclarés dans `src/index.css` pour les deux
thèmes puis exposés en utilitaires Tailwind v4 via `@theme inline` (`bg-card`,
`text-muted-foreground`, `border-border`, `fill-foreground`…). Le thème sombre réutilise la
classe `.dark` déjà en place : les composants ReUI suivent donc automatiquement le basculement.

Dépendances ajoutées : `recharts` (graphiques), `embla-carousel-react` (carrousel),
`class-variance-authority`, `clsx`, `tailwind-merge` (variantes et fusion de classes) et
`@radix-ui/react-{slot,dropdown-menu,separator}` (primitives accessibles). Les ponts
`matchMedia`, `ResizeObserver` et `IntersectionObserver` sont émulés dans
`src/test/setup.ts` : sans eux, le carrousel et les graphiques échouent au montage en jsdom.

Contextes d'utilisation retenus :
| Composant | Modèle | Emplacement |
|---|---|---|
| Carte d'indicateur (variation + menu) | c-card-15 | Tableau de bord : CA encaissé, encaissé du mois |
| Carte de raccourci | c-card-17 | Tableau de bord : dépôt, comptoir, rapports |
| Carrousel + vignettes | c-carousel-10 | Tableau de bord : derniers dépôts au comptoir |
| Anneau de répartition (dégradés, total au centre) | c-chart-19 | Tableau de bord : encaissements par prestation |
| Répartition hachurée + légende | c-chart-21 | Tableau de bord : suivi de l'atelier |
| Entonnoir de conversion | c-chart-22 | Espace éditeur : formules souscrites |
| Tableau des abonnements | c-table-10 | Espace éditeur : répertoire des pressings souscrits |
| Badges (pleines, `-light`, `-outline`) | c-badge-4 / c-badge-9 | Statuts d'atelier, formules, états d'abonnement |
| Évolution en aires dégradées | c-chart area | Tableau de bord : encaissements et facturation des 6 derniers mois |
| Barres groupées | c-chart bar | Tableau de bord : dépôts et pièces reçues par jour d'activité |
| Anneaux concentriques (radial) | c-chart radial | Tableau de bord : taux de recouvrement des règlements, pourcentage au centre |
| Entonnoir du parcours du linge | c-chart funnel | Tableau de bord : conversion réception → livraison, dépôts encore en atelier |

### Tableaux de données

La primitive `src/components/ui/table.tsx` (shadcn/ui) porte l'habillage maison : en-tête
discret (`neutral-50/75`, `dark:neutral-900/60`), filets `neutral-100`/`neutral-800`, survol
foncé des lignes et **conteneur défilant intégré** — plus aucune vue ne gère son
`overflow-x`. Elle expose `Table`, `TableHeader`, `TableBody`, `TableFooter`, `TableRow`,
`TableHead`, `TableCell`, `TableCaption` et `TableEmpty` (ligne d'état vide occupant toute la
largeur, qui neutralise le survol).

`src/components/reui/orders-table.tsx` (modèle c-table-10 enrichi) est le tableau des dépôts :
tri par colonne (ticket, client, article, échéance, montant) avec `aria-sort`,
filtre par étape d'atelier **avec compteurs**, recherche libre insensible aux accents
(ticket, client, téléphone, article), actions de comptoir par ligne (imprimer, notifier,
faire avancer), affichage par paliers (« Afficher plus ») et pied récapitulatif aligné sur les
colonnes (facturé, encaissé, pièces, dépôts en atelier, soldés). Il sert au comptoir (avec
barre d'outils) et au tableau de bord (version condensée de cinq lignes). Les métadonnées
d'étape et de règlement (`ETAPES_COMMANDE`, `PROCHAINE_ETAPE`, `ETAT_REGLEMENT`) y sont
exportées pour que les vues partagent les mêmes libellés et badges.

Toutes les vues métier passent par cette primitive — clients, dépenses, rapports,
prestations (deux tableaux), équipe, abonnements de l'espace éditeur et dépôts : plus aucune
balise `<table>` brute dans `src/`.

Les séries qui alimentent ces graphiques sont calculées par des **fonctions pures** dans
`src/lib/analytique.ts` (`serieMensuelle`, `serieJournaliere`, `recouvrement`,
`pipelineAtelier`) : elles lisent les trois formats de date produits par l'application
(date de l'API, date ISO, libellés de la vitrine de démonstration) et sont testées
indépendamment du rendu. Les graphiques sont également vérifiés **au dessin** : les tests
imposent un cadre de 420 × 230 px au conteneur et contrôlent les tracés réellement produits
(aires et dégradés, barres par série, anneaux colorés, trapèzes de l'entonnoir, parts hachurées).

### Jeu de styles : cadres surélevés et survols foncés

Deux utilitaires Tailwind v4 portent l'identité visuelle des surfaces (`src/index.css`) :
- **`ps-elevated`** — cadre surélevé : liseré d'ombre **sur les bords** (`inset 0 0 0 1px`),
  ombre de contact et ombre portée profonde ; au survol le cadre se soulève
  (`translateY(-2px)`), son trait devient **noir** (`hover:border-neutral-950`) et l'ombre
  se densifie. En thème sombre, le liseré devient clair et l'ombre noire est renforcée
  (`.dark .ps-elevated`).
- **`ps-elevated-static`** — même relief, sans soulèvement (panneaux et encarts non
  interactifs).

Les **survols** sont foncés partout : les contrôles (boutons, icônes, entrées de menu,
onglets) passent au **noir plein avec texte blanc** en thème clair — et blanc avec texte
noir en thème sombre ; les lignes de tableau et de liste reçoivent une **tinte noire**
(6 à 16 %) plutôt qu'un gris. Les fenêtres modales conservent leur ombre portée profonde
(`shadow-2xl` teinté) sans effet de survol.

Répartition actuelle : **58 cadres surélevés** (cartes, encarts, boutons d'action, lignes de
synthèse) et **9 surfaces plates** en `ps-elevated-static` — champs de saisie, pastilles et
barre d'en-tête collante, qui gardent le relief d'un panneau sans jamais se soulever. Le
fichier `src/styles.test.ts` verrouille ces règles : utilitaires présents, cadres équipés et
absence de retour aux survols gris clair.

- **Impression des tickets** : le rendu 80 mm est disponible en HTML (modale ticket) et en **flux ESC/POS** (`src/lib/escpos.ts`) — CP437, encodage tolérant aux accents absents de la table, transport **WebUSB** (Chrome/Edge sur ordinateur) avec aperçu du flux avant envoi ; l'impression HTML reste le repli.
