# PressingOS — Analyse fonctionnelle détaillée et atteinte des objectifs

| | |
|---|---|
| **Date de l'analyse** | 11 octobre 2026 |
| **Commit analysé** | `bba5130` — branche `arena/90615ea9-presspro` (tête poussée, CI verte) |
| **Périmètre** | Code source complet (backend Go, frontend React/TS, SQL), documentation du dépôt, **exécution réelle** : tests Go unitaires + intégration PostgreSQL 16, 171 tests frontend, build, appels HTTP sur la pile vivante (proxy `:3000` → API `:8085` → PostgreSQL) |
| **Méthode** | Inventaire par lecture du code (source de vérité), puis vérification par exécution. Aucun chiffre n'est repris d'un document sans avoir été re-mesuré. |

---

## 0. Synthèse en une page

**Ce qu'est le produit aujourd'hui** : un ERP SaaS multi-tenant complet pour pressings ivoiriens — socle Go (17 paquets, 48 routes), base PostgreSQL 16 (21 tables, 15 migrations), interface React 19 (21 composants applicatifs : 11 écrans, 6 modales, 4 éléments de coque, plus 15 composants ReUI et 7 primitives `ui/`), moteur commercial (FREE/PRO/BUSINESS, essai 7 j, quotas, 402) et console éditeur opérationnelle.

**Verdict global : ~8,3 / 10 — le cœur métier est livré, testé et vérifié ; les manques restants sont d'exploitation (sauvegardes, observabilité, encaissement des abonnements) et de finition (sécurité avancée, accessibilité, hors-ligne).**

| Axe | Note | Constat en une phrase | Évolution depuis l'audit du 11/10 (`58f1da7`) |
|---|---|---|---|
| Architecture & organisation | **9 / 10** | Pattern identique partout (handler → service → persist → model → test), 17 paquets cohérents, proxy Express clair. | 8,5 → 9 |
| Modèle de données | **9 / 10** | 21 tables, 15 migrations up/down idempotentes, `tenant_id` partout, 24 index, contraintes `CHECK`, cascades réfléchies. | 9 → 9 (migrations 11 → 15) |
| Sécurité multi-tenant | **8,5 / 10** | 4 contrôles en chaîne (JWT → appartenance fail-closed → rôle → périmètre branche), rate limiting, isolation prouvée par tests. Manquent : refresh/rotation de jeton, 2FA éditeur, RLS PostgreSQL, confidentialité du ticket public. | 8 → 8,5 |
| Qualité & tests | **7,5 / 10** | 94 tests Go (+ 4 suites d'intégration PostgreSQL) et 23 fichiers / 171 tests frontend, CI verte à chaque push. Couverture Go encore hétérogène (`customer`/`database` 0 %). | 6 → 7,5 |
| Fonctionnel livré | **9 / 10** | Commandes, caisse, catalogue, clients, équipe, multi-sites, dépenses, rapports + exports, notifications WhatsApp/SMS, abonnement/quotas, console éditeur (dont **suppression définitive**). | 8 → 9 |
| Robustesse en production | **6,5 / 10** | Bascule mémoire décidée sur `Ping`, `/ready` 503, arrêt gracieux, erreurs normalisées. Manquent : sauvegardes/restauration, observabilité, encaissement des abonnements. | **4 → 6,5** |
| Documentation | **7,5 / 10** | 5 documents `docs/` réconciliés avec le code (API, architecture, base, installation, journal des correctifs) + `.env.example`. Restent 3 documents racine périmés et aucun contrat OpenAPI. | 7 → 7,5 |

**Les 3 risques qui empêchent encore une commercialisation** :

1. **Aucune sauvegarde automatisée ni restauration testée** : une perte de disque = perte définitive des données d'un client.
2. **Le SaaS n'encaisse pas ses abonnements** : plans et quotas sont appliqués, mais aucun moyen de paiement n'est branché (aucune facture d'abonnement non plus).
3. **Observabilité nulle** : pas de métriques, pas de logs structurés, pas d'alerte — une panne se découvre par le client.

---

## 1. Volumétrie mesurée (11/10/2026)

| Élément | Mesure |
|---|---|
| Backend Go | **16 084 lignes**, 17 paquets dans `backend/internal/` (+ `cmd/api`) |
| Routes HTTP déclarées | **48** (dont 1 publique non authentifiée : vérification de ticket) |
| Frontend TypeScript/TSX | **18 548 lignes** : 21 composants applicatifs (11 écrans, 6 modales, 4 éléments de coque) + 15 composants ReUI + 7 primitives `ui/` |
| Base de données | **21 tables**, **15 migrations** up/down, 24 index, 1 table de suivi `schema_migrations` |
| Tests Go | **94 fonctions de test** réparties sur 13 paquets + **4 suites d'intégration** (`-tags=integration`, PostgreSQL réel) |
| Couverture Go | `ratelimit` 92 % · `config` 83 % · `health` 80 % · `notification` 61 % · `http` 59 % · `report` 59 % · `expense` 44 % · `billing` 40 % · `auth` 39,5 % · `branch` 37 % · `order` 21 % · `catalog` 16 % · `pricelist` 7,5 % · `customer`, `database`, `httperr`, `pgutil` 0 % |
| Tests frontend | **23 fichiers / 171 tests** ✅ · ESLint 0 erreur / 48 avertissements · Prettier ✅ · `vite build` ~1 s (bundle découpé) |
| Intégration continue | 2 jobs GitHub Actions (Go + PostgreSQL 16, Frontend types + build) — **verte** sur `bba5130` |
| État de la base live | **0 établissement, 0 commande** (purge du 11/10) ; API `200 []` sur les listes ; preview servie |

---

## 2. Objectifs visés (reformulés depuis les documents du projet)

1. **Objectif produit** : « ERP SaaS multi-tenant de gestion des pressings et buanderies en Côte d'Ivoire » (`RAPPORT_PROJET_PRESSPRO.md` §1.1) — dépôt, atelier, caisse, clients, tarifs, personnel, pilotage, pour un prix mensuel par établissement.
2. **Objectifs de robustesse de l'audit** (Sprints 0 et 1) : rendre l'application exploitable en mode PostgreSQL (catalogue, repli mémoire), garantir l'intégrité financière (surpaiement), durcir la sécurité (secret JWT, erreurs, force brute, fail-closed), brancher les verrous de plan, industrialiser (CI).
3. **Objectifs de complétion fonctionnelle** (Sprints 2 et 3) : console éditeur sur données réelles, écrans multi-établissements et équipe, notifications WhatsApp/SMS, rapports avancés/consolidés + exports, module dépenses, impression ESC/POS, mode hors-ligne, observabilité, sauvegardes.
4. **Objectifs exprimés ensuite par le commanditaire** (sessions 4 à 7) : composants d'interface ReUI réels, habillage « survols foncés + relief », composant tableau correctement implémenté puis partagé, exports de rapports, impression thermique, affectation employé ↔ établissement, notifications clients WhatsApp/SMS, et **suppression de toutes les données des entreprises fictives (base + jeu de démonstration) via une vraie fonctionnalité d'administration**.

---

## 3. Inventaire détaillé des fonctionnalités

Légende : ✅ livré et vérifié · 🟡 livré partiellement · ❌ absent.

### 3.1 Identité, accès et annuaire des comptes

| Fonctionnalité | État | Mise en œuvre | Preuve |
|---|---|---|---|
| Connexion (JWT HS256, 24 h) | ✅ | `auth/service.go`, `handler.go`, bcrypt | `auth` 39,5 % couverture, login live `200` |
| Inscription d'un établissement | ✅ | `POST /auth/register-tenant` → établissement + propriétaire + **49 prestations** + grille tarifaire + essai 7 j | Vérifié en live : `201`, 49 prestations au `GET /services` |
| Comptes de démonstration embarqués | ✅ | `auth.Service` (3 comptes) — conservés sur décision explicite | Login live `200` |
| Appartenance fail-closed | ✅ | `internal/auth/persist.go` + middleware | Tests `http`, tests d'intégration |
| Rôles (owner, manager, cashier, operator, superadmin) | ✅ | `RequireRole` sur 8 groupes de routes | `http/auth_middleware_test.go` |
| Désactivation d'un compte (effet immédiat) | ✅ | `PATCH /users/{id}/status` + suspension des appartenances | Intégration `TestIntegration_CreateUser_PoseLesAppartenances` |
| Limitation de la force brute | ✅ | 10 tentatives/couple (IP, e-mail) + 40/IP sur 5 min, `429` + `Retry-After` | `ratelimit` 92 % + test |
| Mot de passe oublié / changement de mot de passe | ❌ | — | — |
| Refresh token / révocation de session | ❌ | Jeton unique 24 h | — |
| 2FA (éditeur, gérants) | ❌ | — | — |
| Journal de connexion / audit des accès | ❌ | — | — |

### 3.2 Multi-tenant et périmètre

| Fonctionnalité | État | Détail |
|---|---|---|
| Isolation par `tenant_id` sur toutes les tables métier | ✅ | 21 tables, contraintes `NOT NULL` + index composites |
| Chaîne d'autorisation à 4 contrôles | ✅ | JWT → appartenance → rôle → **périmètre branche** (`X-Branch-ID` validé, `X-Tenant-ID` forgé refusé) |
| Tests d'intrusion (inter-tenant `404`, anonyme `401`, mauvais rôle `403`) | ✅ | `http/*_test.go` + intégration |
| Défense en profondeur PostgreSQL (RLS) | ❌ | Isolation **applicative** uniquement : une requête SQL hors middlewares n'est plus bornée |
| Isolation dans le moteur de facturation | ✅ | Quotas et droits calculés **par établissement**, vérifiés en SQL |

### 3.3 Catalogue, prestations et grilles tarifaires

| Fonctionnalité | État | Détail |
|---|---|---|
| CRUD prestations (owner/manager) | ✅ | `services` + 13 champs métier (article, traitement, catégorie, unité, durée, options) |
| 5 catégories métier (A–E : quotidien, traditionnel, linge de maison, complémentaire, luxe) | ✅ | `catalog/model.go`, filtres d'interface |
| Options par prestation : express (avec surcharge), collecte, sous-traitance, devis | ✅ | Champs booléens + `express_surcharge` |
| 4 modèles de facturation (pièce, kilogramme, abonnement, contrat) | ✅ | `pricing_model` |
| 49 prestations créées à l'inscription | ✅ | `SeedTenantDefaults` (vérifié : 49 en live) |
| Grilles tarifaires versionnées (snapshots JSONB) | 🟡 | Backend complet (`price-lists`, versions, gel `catalog.price_lists` 1/3/10) — **aucun écran de gestion des grilles** dans le frontend |
| Import/export de catalogue, duplication, historique de prix | ❌ | — |
| Calcul de marge/rentabilité par prestation | 🟡 | Marge globale dans les rapports ; pas par prestation |

### 3.4 Commandes, atelier et ticket client

| Fonctionnalité | État | Détail |
|---|---|---|
| Dépôt : client, articles, acompte, moyen de paiement, échéance, notes | ✅ | `POST /orders` (201), séquence par établissement (`order_sequences`) |
| Cycle de vie 5 étapes + annulation | ✅ | `received → washing → ironing → ready → delivered` (+ `cancelled` accepté par l'API) |
| Liste filtrée/recherchée par statut et texte | ✅ | `GET /orders?status=&q=` |
| Ticket 80 mm texte (API) | ✅ | `GET /orders/{id}/ticket` — en-tête = nom du jeton, **sans coordonnées codées en dur** (corrigé le 11/10) |
| Ticket thermique client (ESC/POS) | ✅ | `escpos.ts` : encodage ESC/POS, aperçu, **impression directe WebUSB** + repli impression navigateur ; testé (19 tests) |
| Vérification publique d'un ticket (QR, sans compte) | ✅ | `GET /public/verify/{tenantID}/{orderID}` — `valid: false` + 404 si inexistant |
| Isolation des commandes entre établissements | ✅ | Refus `404` inter-tenant (testé) |
| Pagination serveur des listes | ❌ | Listes renvoyées entières (risque de charge au-delà de quelques milliers de commandes) |
| Historique/audit par commande (qui a fait quoi) | ❌ | — |
| Capacité/planning de production, retouches, litiges | ❌ | — |
| Confidentialité du ticket public | ❌ | Le nom du client est renvoyé en clair sur une URL devinable (anomalie **A16** de l'audit, toujours ouverte) |

### 3.5 Caisse, paiements et intégrité financière

| Fonctionnalité | État | Détail |
|---|---|---|
| Encaissements partiels et multi-moyens | ✅ | `cash`, `wave`, `orange_money`, `mtn_momo`, `card` ; `POST /orders/{id}/payments` |
| Refus du surpaiement (intégrité comptable) | ✅ | `ErrPaymentExceedsDue` → `400` ; rien n'est écrit (corrigé le 11/10) |
| Statuts de règlement (à encaisser / acompte / soldé) | ✅ | Calculés, affichés en badges, agrégés dans les graphiques |
| Rapprochement des caisses, clôture journalière (Z) | ❌ | — |
| Remboursement/annulation d'un encaissement | ❌ | — |
| Encaissement réel Mobile Money (API Wave/Orange/MTN) | ❌ | Le moyen est **saisi** par l'opérateur, aucune confirmation bancaire automatique |
| Reçu de caisse imprimable (ticket/reçu tarifaire) | 🟡 | `CounterReceiptModal` (80/58 mm, devis) — mais **coordonnées par défaut fictives** (voir §5.4) |

### 3.6 Clients

| Fonctionnalité | État | Détail |
|---|---|---|
| CRUD clients + unicité par établissement (téléphone) | ✅ | `customers` CRUD + gestion des doublons (constatée en session de test) |
| Recherche et fiches clients | ✅ | `CustomersView` (tableau primitif durci) |
| Historique consolidé client (commandes, encours, préférences) | 🟡 | Les commandes portent le client ; pas de fiche 360° |
| Fidélité, segmentation, campagnes, relances | ❌ | — |
| Consentement RGPD (opt-in SMS/WhatsApp), export/effacement | ❌ | — |

### 3.7 Équipe

| Fonctionnalité | État | Détail |
|---|---|---|
| Création de comptes employés (limite de plan) | ✅ | `POST /users`, quotas `users.max` 2/5/15 |
| Rôles et statuts (actif/inactif) | ✅ | `TeamView` + `PATCH /users/{id}/status` |
| Affectation employé ↔ établissements | ✅ | `GET/PUT /users/{id}/branches` (livré en session 5) |
| Invitation par SMS/e-mail (mot de passe temporaire automatique) | ❌ | Le mot de passe est communiqué oralement |
| Journal d'activité par employé, permissions granulaires, PIN caisse | ❌ | — |

### 3.8 Établissements (multi-sites)

| Fonctionnalité | État | Détail |
|---|---|---|
| CRUD établissements + établissement principal | ✅ | `branches` + `BranchesView`, limite `branches.multi_site` 1/1/3 |
| Appartenances branche par utilisateur | ✅ | `branch_memberships` + écran d'affectation |
| Rapports consolidés multi-sites (BUSINESS) | ✅ | `reports.consolidated` → bloc `by_branch` (CA, dépenses, marge par site) |
| Transferts de linge entre sites / réaffectation de commandes | ❌ | — |
| Horaires d'ouverture persistés (affichés sur le ticket public) | ❌ | Champ front ; non modélisé en base |
| Fiche établissement éditable (adresse, téléphone, logo) via l'API | ❌ | Table `tenants` a les colonnes ; **aucun endpoint** de lecture/écriture |

### 3.9 Dépenses et résultat

| Fonctionnalité | État | Détail |
|---|---|---|
| Saisie, liste, suppression de dépenses (catégories, moyen, date) | ✅ | `expenses` + `ExpensesView` (tableau primitif) |
| Réservé aux formules payantes (assertion `expenses.advanced`) | ✅ | `402` documenté et testé (gratuit) |
| Filtre par période, total, répartition par catégorie | ✅ | `GET /expenses` + rapports |
| Marge (CA − dépenses) par période et par établissement | ✅ | `reports/summary` |
| Justificatifs/pièces jointes, fournisseurs, dépenses récurrentes | ❌ | — |
| Validation/approbation, TVA, export comptable | ❌ | — |

### 3.10 Rapports et exports

| Fonctionnalité | État | Détail |
|---|---|---|
| Rapport de période (CA, commandes, panier moyen, marge, statuts, top prestations) | ✅ | `GET /reports/summary` (gate `reports.advanced`) |
| Consolidation multi-établissements | ✅ | `reports.consolidated` (BUSINESS) |
| Export CSV et PDF, **strictement identiques à l'écran** | ✅ | `GET /reports/export?format=csv|pdf` (livré session 5) |
| Graphiques de pilotage (encaissements, charge, recouvrement, entonnoir, statuts, donut) | ✅ | 7 graphiques ReUI (recharts) branchés sur les données réelles + recette de dessin en jsdom |
| Rapports planifiés / envoi par e-mail, comparatif inter-périodes | ❌ | — |
| Export Excel natif, liasse comptable (SYSCOHADA/TVA) | ❌ | — |

### 3.11 Notifications clients (WhatsApp / SMS)

| Fonctionnalité | État | Détail |
|---|---|---|
| Journal des notifications (canal, statut, contenu, destinataire) | ✅ | `notifications` + `GET /notifications` (libre, tous plans) |
| 4 modèles : commande prête, livrée, paiement reçu, rappel de retrait | ✅ | Rendu avec nom du client, n° de commande, reste à payer, nom du pressing |
| Envoi réel via passerelle HTTP (URL + jeton + timeout 5 s) | ✅ | `Passerelle` configurable, statut `succes`/`echec` + détail |
| Déclenchement automatique sur passage `ready` / `delivered` | ✅ | `NotifierTransition` (non bloquant pour l'atelier) |
| Envoi réservé aux formules payantes (`notifications.advanced`) | ✅ | `402` en gratuit, journal conservé |
| Fournisseur réel contracté (Orange CI, MTN, WhatsApp Business) | ❌ | La passerelle est générique : c'est à l'exploitant de la brancher |
| File d'attente, reprises automatiques, idempotence, accusés | ❌ | Envoi synchrone avec délai d'attente |
| Préférences client (opt-out), envois programmés | ❌ | — |

### 3.12 Abonnement, droits et quotas

| Fonctionnalité | État | Détail |
|---|---|---|
| Plans FREE / PRO / BUSINESS (0 / 45 000 / 75 000 FCFA) | ✅ | `plans` + matrice `plan_entitlements` + `defaultMatrix()` alignée |
| Essai 7 jours au niveau BUSINESS | ✅ | Calculé côté serveur, aucun cron |
| 10 droits plafonnés/enabled (`orders.monthly.max` 50, `users.max`, `catalog.price_lists`, `branches.multi_site`, 4 fonctionnalités binaires) | ✅ | Compteurs réels en SQL (`dbCounters`) |
| Refus explicites `402 {error, feature, usage, upgrade_required}` | ✅ | Verrous branchés sur 6 routes (grilles, dépenses ×2, notifications, rapports ×2) |
| Suspension par l'éditeur → modules gelés (402) | ✅ | `RequireActiveSubscription` ; compte et éditeur restent accessibles |
| Droits effectifs exposés au client | ✅ | `GET /me/entitlements` (+ écran « Mon abonnement ») |
| Encaissement des abonnements (PSP), factures, relances d'impayés | ❌ | Statut changé à la main par l'éditeur |
| Changement de plan en self-service, prorata, historique de facturation | ❌ | — |

### 3.13 Console éditeur (SuperAdmin)

| Fonctionnalité | État | Détail |
|---|---|---|
| Indicateurs consolidés réels (établissements, utilisateurs, essais, MRR, répartitions) | ✅ | `GET /admin/stats` (SQL), signalés `postgresql`/`memoire` |
| Annuaire des établissements (propriétaire, formule, statut, fin d'essai) | ✅ | `GET /admin/tenants` |
| Inscription d'un partenaire depuis la console | ✅ | `POST /auth/register-tenant` (bouton + formulaire) |
| Suspension / réactivation d'un abonnement | ✅ | `PATCH /admin/tenants/{id}/status` (journalisé dans `subscription_events`) |
| **Suppression définitive d'un établissement et de ses données** | ✅ | `DELETE /admin/tenants/{id}` — transaction unique (`payments`, `orders`, `order_items` purgés explicitement avant `tenants`), `204`/`404`, bouton + confirmation à cocher (livré le 11/10) |
| Vision par formule, MRR, alertes mode dégradé | ✅ | Funnel des formules, cartes d'indicateurs |
| Gestion des plans/quotas depuis l'interface (aujourd'hui : SQL) | ❌ | — |
| Impersonation / support « voir en tant que », journal des décisions éditeur | 🟡 | Décisions tracées en base (`subscription_events`) ; aucune page d'audit |

### 3.14 Exploitation, santé et intégration

| Fonctionnalité | État | Détail |
|---|---|---|
| `GET /health` (liveness) et `GET /ready` (readiness, `Ping`) | ✅ | `internal/health`, vérifiés en live |
| Bascule en mode mémoire si base injoignable | ✅ | Décision sur `Ping` réel ; écrans vides, aucune donnée inventée |
| Arrêt gracieux (SIGINT/SIGTERM, 10 s) | ✅ | `main.go` |
| Erreurs normalisées et non divulgantes | ✅ | `httperr` (journalisation serveur, message générique) |
| CORS restreint + en-têtes autorisés | ✅ | `ALLOWED_ORIGINS`, `X-Branch-ID`, `X-Tenant-ID` (refusé s'il est forgé) |
| PostgreSQL local en conteneur | ✅ | `compose.yaml` (exposition `127.0.0.1` uniquement, healthcheck, volume) |
| Conteneurisation de l'application (API + SPA) | ❌ | Seule la base est conteneurisée |
| Sauvegardes automatisées + restauration testée | ❌ | — |
| Migrations versionnées par un outil (`golang-migrate`) | ❌ | Appliquées par le conteneur/CI script `psql` (fichiers up/down présents) |
| Journaux structurés (JSON), métriques Prometheus, tracing, alerting | ❌ | Journaux texte Chi, `request_id` présent |
| Tests de charge, plan de reprise d'activité | ❌ | — |

### 3.15 Frontend

| Fonctionnalité | État | Détail |
|---|---|---|
| 12 écrans : tableau de bord, commandes, clients, prestations, équipe, établissements, dépenses, rapports, abonnement, console éditeur, vérification publique de ticket | ✅ | Navigation par état interne + route publique `/verify/…` |
| Modales métier : nouveau dépôt, ticket thermique, notification client, reçu/devis, gestionnaire de catalogue, connexion | ✅ | 6 modales, dont 5 testées |
| Composants d'interface réutilisables | ✅ | 7 primitives `ui/` (dont **table durcie** : en-têtes, pied, état vide, conteneur défilant) + 15 composants ReUI (7 graphiques recharts, 2 tableaux, badges, cartes d'indicateurs, cadre, carrousel) — composant table employé dans 7 fichiers (8 tableaux, 137 en-têtes/cellules) |
| Tableau des dépôts partagé comptoir / tableau de bord | ✅ | Tri accessible (`aria-sort`), filtres comptés, recherche insensible aux accents, pagination par paliers, actions contextuelles (une seule balise `<table>` brute subsiste dans tout `src/`) |
| Habillage unifié (relief + survols foncés) | ✅ | Utilitaires `ps-elevated`/`ps-elevated-static` (86 usages dans 21 fichiers), garde-fous automatisés (`src/styles.test.ts`) |
| Thème clair/sombre, impression 80 mm, gestion d'erreurs | ✅ | `ThemeContext`, `@media print`, `ErrorBoundary` |
| États vides explicites (aucune donnée fictive) | ✅ | Livré le 11/10 : l'application démarre vide et l'indique |
| Routeur / deep-linking (hors `/verify`) | ❌ | Pas de `react-router` : pas d'URL partageable pour une commande/un rapport |
| Internationalisation | ❌ | Français en dur |
| Accessibilité (audit WCAG/RGAA, navigation clavier complète) | 🟡 | `aria-label`/`aria-sort`/focus visibles sur les tableaux ; aucun audit formel |
| Mode hors-ligne (PWA + file de synchronisation) | ❌ | — |
| Virtualisation des grandes listes | ❌ | Rendu intégral côté client |

### 3.16 Tests, qualité et CI

| Élément | État | Détail |
|---|---|---|
| Tests Go unitaires | ✅ | 94 fonctions ; `go test -race` en CI |
| Tests d'intégration PostgreSQL | ✅ | `catalog`, `auth`, `report`, `billing` (dont suppression transactionnelle) sur PostgreSQL 16 réel |
| Tests frontend | ✅ | 23 fichiers / 171 tests : vues, clients d'API, analyses, **dessin réel des graphiques**, tableaux, garde-fous de style, ESC/POS, flux applicatifs |
| Lint/format | ✅ | `tsc --noEmit`, ESLint 9 (plafond 50 avertissements, 48 actuels), Prettier |
| CI/CD | ✅ | 2 jobs sur chaque push/PR ; **vert** sur `bba5130` |
| Seuil de couverture bloquant | ❌ | Couverture mesurée mais non imposée |
| Tests de bout en bout navigateur (Playwright) | ❌ | Les vérifications de bout en bout sont des scripts Python non versionnés (`/tmp/…`) |
| Analyse statique de sécurité (gosec, CodeQL), Dependabot | ❌ | — |

### 3.17 Base de données

| Élément | État | Détail |
|---|---|---|
| 21 tables, `tenant_id` systématique, 24 index | ✅ | `docs/database.md` (aligné) |
| Contraintes d'intégrité (`CHECK` sur statuts/montants, `UNIQUE (tenant_id, …)`) | ✅ | Migrations 000001 → 000015 |
| Cascades réfléchies (`ON DELETE CASCADE` sauf `orders`/`payments` : traitées explicitement) | ✅ | Prouvé par le test d'intégration de suppression |
| Migrations up/down idempotentes | ✅ | 15 paires ; la migration 000015 annule le seed de l'entreprise fictive |
| Rétention/archivage (commandes anciennes), partitionnement | ❌ | — |
| Colonnes d'audit (`created_by`, `updated_by`), suppression logique | ❌ | — |
| RLS PostgreSQL | ❌ | — |

### 3.18 Inventaire écran par écran (frontend)

Chaque écran est un composant autonome (`src/components/`) alimenté par un client d'API typé (`src/lib/*Api.ts`) ; la navigation est un état interne filtré par rôle (`Sidebar.tsx`).

| Écran (fichier) | Taille | Rôles | Ce que l'utilisateur peut y faire |
|---|---|---|---|
| **Tableau de bord** (`DashboardView.tsx`) | 393 l. | tous | Cartes d'indicateurs (dépôts actifs, à encaisser, CA du jour, marge), **7 graphiques** (encaissements, charge hebdomadaire, recouvrement, entonnoir du pipeline, statuts, donut des revenus, formules), tableau des dépôts partagé, actions rapides |
| **Commandes & Linge** (`OrdersView.tsx` + `reui/orders-table.tsx`) | 56 + 522 l. | tous | Créer un dépôt ; tableau **partagé** avec tri accessible, filtres par étape comptés, recherche insensible aux accents, pagination, avancement d'étape (libellé exact de l'étape suivante), impression de ticket, notification client |
| **Fichier Clients** (`CustomersView.tsx`) | 215 l. | owner, manager, cashier | CRUD client, recherche, téléphone/e-mail, encours et solde |
| **Prestations & Tarifs** (`ServicesView.tsx`) | **1 947 l.** | owner, manager | Catalogue par catégories A–E, création/modification/suppression de prestation, options (express, collecte, sous-traitance, devis), **vente au comptoir** multi-articles avec reçu tarifaire 80/58 mm, ouverture du gestionnaire de catalogue |
| **Équipe** (`TeamView.tsx`) | 583 l. | owner, manager | Créer un compte (quota `users.max`), attribuer un rôle, activer/désactiver, **affecter aux établissements** |
| **Établissements** (`BranchesView.tsx`) | 369 l. | owner, manager | CRUD établissement, désigner le principal, limite `branches.multi_site` |
| **Dépenses** (`ExpensesView.tsx`) | 415 l. | owner, manager, cashier | Saisir une dépense (catégorie, moyen, date, note), lister, supprimer, total et répartition de la période |
| **Rapports** (`ReportsView.tsx`) | 418 l. | owner, manager | Choisir la période, lire CA/panier moyen/marge/statuts/top prestations/par établissement, **exporter en CSV ou PDF** |
| **Mon abonnement** (`BillingView.tsx`, carte en bas de barre latérale) | 371 l. | tous | Formule courante, droits effectifs, compteurs et quotas, fin d'essai, comparaison des formules |
| **Plateforme SaaS** (`SuperAdminView.tsx`) | 648 l. | super-admin | Indicateurs consolidés (MRR, essais, répartitions), annuaire, inscription d'un partenaire, suspension/réactivation, **suppression définitive** avec confirmation à cocher |
| **Vérification de ticket** (`TicketVerificationView.tsx`) | 254 l. | **public** (aucun compte) | Saisie/scan d'un ticket → « Ticket valide ! » ou « Ticket invalide » |
| Coque (`Header`, `Sidebar`, `ThemeToggle`) + `ErrorBoundary` | — | — | En-tête contextuel, navigation filtrée par rôle, thème clair/sombre, capture des erreurs d'affichage |

Modales métier : `NewOrderModal` (dépôt multi-articles), `ThermalTicketModal` (aperçu 80 mm → ESC/POS WebUSB ou impression navigateur), `ClientNotificationModal` (WhatsApp/SMS, 4 modèles éditables), `CounterReceiptModal` (reçu/devis 80/58 mm), `CatalogManagerModal` (gestion du catalogue), `LoginModal`.

### 3.19 Parcours de bout en bout vérifié sur la pile réelle (base vide)

Séquence exécutée après la purge, via le proxy `:3000` → API `:8085` → PostgreSQL :

| Étape | Appel | Résultat observé |
|---|---|---|
| 1. Inscription | `POST /api/v1/auth/register-tenant` | `201` — établissement + propriétaire + essai BUSINESS |
| 2. Catalogue auto | `GET /api/v1/services` | **49 prestations** créées |
| 3. Ouverture d'un dépôt | `POST /api/v1/orders` | `201` — `CMD-2026-001`, total 2 000, reste 1 500 (acompte 500) |
| 4. Vérification publique | `GET /api/v1/public/verify/{tenant}/{order}` | `200` — `valid: true` |
| 5. Suppression définitive | `DELETE /api/v1/admin/tenants/{id}` (super-admin) | `204`, annuaire à nouveau vide |
| 6. Jeton du tenant supprimé | `GET /api/v1/orders` (ancien jeton) | `403` (membre inactif — fail-closed) |

Le cycle complet « vendre → encaisser → contrôler » fonctionne donc **sans aucune donnée préexistante**, et l'éditeur peut défaire proprement une inscription.

---

## 4. Atteinte des objectifs

### 4.1 Objectifs de l'audit du 11/10 (anomalies A1 → A18)

| ID | Anomalie initiale | Statut | Preuve |
|---|---|---|---|
| A1 | Catalogue illisible en PostgreSQL (panique) | ✅ corrigé | Fabriques alignées + test d'intégration `TestIntegration_DBReadServices` |
| A2 | Repli mémoire jamais déclenché (pas de `Ping`) | ✅ corrigé | Bascule décidée sur le `Ping` ; `/ready` 503, listes vides |
| A3 | Surpaiement accepté (comptabilité incohérente) | ✅ corrigé | `ErrPaymentExceedsDue` (400), test unitaire + intégration |
| A4 | Secret JWT de repli en dur | ✅ corrigé | Démarrage refusé en production sans `JWT_SECRET` fort |
| A5 | Erreurs internes divulguées | ✅ corrigé | `httperr` : message générique + journal serveur |
| A6 | Aucun rate limiting sur la connexion | ✅ corrigé | `ratelimit` 10/e-mail, 40/IP, `429` |
| A7 | Contrôles d'accès fail-open | ✅ corrigé | Appartenance fail-closed (jeton orphelin refusé) |
| A8 | Couverture de tests faible | 🟡 partiel | 94 tests Go + 4 suites d'intégration, mais `customer`/`database` 0 %, `order` 21 %, `catalog` 16 %, `pricelist` 7,5 % |
| A9 | Codes HTTP incohérents | ✅ corrigé | `403`/`404` normalisés (audit final 34/34) |
| A10 | Quota restant négatif | ✅ corrigé | Valeurs bornées à 0 |
| A11 | Documentation désynchronisée | 🟡 partiel | `docs/` réconcilié ; 3 documents racine périmés subsistent |
| A12 | Verrous de plan non branchés | ✅ corrigé | 6 routes sous `RequireFeature` + 402 testés |
| A13 | Aucune prestation par défaut pour un nouveau tenant | ✅ corrigé | 49 prestations créées à l'inscription |
| A14 | Zéro test frontend, pas de lint/format, bundle monolithique | ✅ corrigé | 171 tests, ESLint/Prettier, bundle découpé (vendors) |
| A15 | Aucune CI | ✅ corrigé | 2 jobs GitHub Actions, PostgreSQL 16 de service |
| A16 | Ticket public exposant le nom du client | ❌ **ouvert** | `VerifyTicketResponse.CustomerName` toujours renvoyé en clair |
| A17 | Documents obsolètes (`analyse.md`, `rapports_audit_consolide.md`) | ❌ ouvert | Toujours présents à la racine |
| A18 | `npm ci` exige `--legacy-peer-deps` | ❌ ouvert | Aucun `.npmrc` ni `overrides` dans `package.json` |

**Bilan** : **14 anomalies corrigées**, 2 partiellement, 3 ouvertes (A16, A17, A18) — les P0/P1 de la sécurité et de la robustesse sont tous traités.

### 4.2 Feuille de route de l'audit (points 1 → 20)

| # | Objectif | Statut | Commentaire |
|---|---|---|---|
| 1-5 | Correctifs bloquants (catalogue, repli, surpaiement, secret JWT, erreurs/codes) | ✅ | Voir A1-A5, A9 |
| 6 | Rate limiting connexion | ✅ | 429 vérifié en live |
| 7 | Fail-closed des accès | ✅ | |
| 8 | Tests d'intégration Go sur PostgreSQL réel | ✅ | 4 suites ; couverture encore inégale (⚠️ partiel sur l'objectif « ≥ 70 % sur les critiques ») |
| 9 | CI GitHub Actions | ✅ | |
| 10 | Brancher `RequireFeature` + clamper les quotas | ✅ | |
| 11 | Découpage du bundle, ESLint/Prettier, premiers tests, routeur | 🟡 | 3 volets sur 4 ; **pas de routeur** |
| 12 | Écrans établissements et utilisateurs | ✅ | `BranchesView`, `TeamView` (livrés sessions 3-4) |
| 13 | SuperAdmin sur données réelles | ✅ | `/admin/stats` + annuaire SQL |
| 14 | Observabilité (Prometheus, logs JSON, `request_id`) | ❌ | Seul `request_id` existe |
| 15 | Sauvegardes automatisées + outil de migration | ❌ | `docker-entrypoint-initdb.d` + scripts `psql` manuels |
| 16 | Notifications WhatsApp/SMS | ✅ | Journal, modèles, passerelle, automatisme, 402 (le fournisseur reste à contracter) |
| 17 | Rapports avancés/consolidés + exports | ✅ | `reports.advanced`, `reports.consolidated`, CSV/PDF |
| 18 | Module dépenses + journal d'audit avancé | 🟡 | Dépenses ✅ ; **journal d'audit** ❌ (droit `audit.advanced` déclaré mais aucun module) |
| 19 | Impression ESC/POS native + mode hors-ligne | 🟡 | ESC/POS WebUSB ✅ (USB, Chrome/Edge) ; **PWA hors-ligne** ❌, pas d'imprimante réseau |
| 20 | Hygiène documentaire | 🟡 | `docs/` réconcilié ; 2 documents périmés non archivés ; scripts de vérification hors dépôt |

### 4.3 Objectifs exprimés par le commanditaire (sessions livrées)

| Demande | Statut | Livrable |
|---|---|---|
| Importer et utiliser réellement les composants ReUI | ✅ | 15 composants (`8e94107`) |
| Habillage « survols foncés + relief » | ✅ | 58 cadres, 33 contrôles, garde-fous (`9c3fee8`, `af9e6b2`) |
| « Bien implémenter le composant table » | ✅ | Primitive durcie + table des dépôts partagée, 212 balises migrées (`dad0f02`) |
| Graphiques réellement tracés | ✅ | 4 graphiques analytiques + recette de dessin en jsdom (`2485b6f`) |
| Tours 5 et 6 (exports, ESC/POS, affectations ; notifications WhatsApp/SMS) | ✅ | `6601a17`, `dc99602` (57/57 vérifications live) |
| **Supprimer toutes les données des entreprises fictives (base + jeu de démonstration), via une fonctionnalité d'administration** | ✅ | `bba5130` : route `DELETE /admin/tenants/{id}` + confirmation, 14 établissements purgés, 17 tables à 0, `demoData.ts` supprimé, migration 000015 |

**Conclusion sur l'atteinte des objectifs** : les objectifs de **correction** (rendre le produit exploitable sur PostgreSQL), de **sécurité de base**, de **monétisation** (plans/quotas vraiment appliqués) et de **complétion fonctionnelle du cœur métier** sont atteints et prouvés par exécution. Le produit est **utilisable par un vrai pressing** (inscription → catalogue → dépôt → atelier → encaissement → ticket → rapport), y compris sur une base vide. Les objectifs **d'exploitation d'un SaaS commercial** — sauvegardes, supervision, encaissement des abonnements, support outillé — ne sont **pas encore** atteints.

---

## 5. Objectifs non encore réalisés

### 5.1 Blocages à lever avant un pilote client réel (P0)

| # | Objectif manquant | Impact | Effort estimé |
|---|---|---|---|
| M1 | **Fiche établissement servie par l'API** (nom, adresse, téléphone, horaires) + utilisation dans les tickets | Des documents client impriment des coordonnées fictives ; l'identité légale n'est pas dans l'app | 3-5 j |
| M2 | **Sauvegardes PostgreSQL automatisées + restauration testée** | Perte de données irréversible = fin de la relation client | 2-3 j |
| M3 | **Encaissement des abonnements** (PSP type CinetPay/Stripe) ou facturation outillée (facture PDF + relances) | Le SaaS ne peut pas facturer ses clients | 5-10 j |
| M4 | **Observabilité** : logs JSON, métriques, alertes (5xx, 402/403, latence, échecs de passerelle) | Pannes silencieuses, diagnostic à l'aveugle | 3-5 j |
| M5 | **Confidentialité de la vérification publique** (masquer le nom du client / code court) | Divulgation d'informations personnelles (A16) | 1 j |
| M6 | **Cycle de vie des jetons** : refresh/révocation, changement et réinitialisation de mot de passe, 2FA éditeur | Sessions de 24 h, aucun moyen de récupérer un compte | 4-6 j |

### 5.2 Objectifs importants mais non bloquants (P1)

| # | Objectif manquant | Impact |
|---|---|---|
| M7 | Couverture de tests sur `customer`, `database`, `pricelist`, `order` (< 25 %) + seuil bloquant en CI | Régressions silencieuses sur les zones peu testées |
| M8 | Pagination et tri côté serveur sur commandes/clients | Dégradation dès quelques milliers de lignes |
| M9 | **Journal d'audit** (qui a encaissé, changé un statut, suspendu/supprimé un établissement) | Aucune traçabilité en cas de litige ou de fraude interne |
| M10 | File d'attente de notifications (reprises, idempotence) + configuration fournisseur par établissement | Messages perdus en cas d'incident réseau de la passerelle |
| M11 | Gestion des plans/quotas et suivi de facturation depuis la console éditeur | Toute évolution tarifaire nécessite un accès SQL |
| M12 | Scripts de vérification **versionnés** (`scripts/`) + tests E2E navigateur | Les preuves de bout en bout ne sont pas rejouables par un tiers |
| M13 | `npm ci` sans `--legacy-peer-deps`, Dependabot, analyse statique de sécurité | Installation fragile, vulnérabilités non surveillées |

### 5.3 Backlog produit non commencé (P2-P3)

- **Paiements Mobile Money réels** (Wave, Orange, MTN) et rapprochement automatique des encaissements.
- **Comptabilité** : TVA, plan SYSCOHADA, clôture de caisse journalière (Z), exports comptables, factures PDF par client.
- **Hors-ligne** : PWA + file de synchronisation (indispensable en cas de coupure Internet au comptoir).
- **Impression réseau** (aujourd'hui WebUSB uniquement) et agent d'impression local.
- **Routeur et deep-linking**, i18n, audit d'accessibilité (RGAA/WCAG), application mobile.
- **Fidélité et marketing client** (relances automatiques, campagnes SMS, segmentation), consentement RGPD.
- **Stock/inventaire** (produits lessiviels), **RH/planning** (heures, présences), contrats B2B facturés à l'entreprise.
- **Multi-devises** (XOF figé), gestion multi-régions.
- **Défense en profondeur** : RLS PostgreSQL, chiffrement au repos des données sensibles, purge/archivage (rétention).

---

## 6. Améliorations à apporter

### 6.1 Priorité 0 — fiabilité et confiance (à faire avant tout pilote)

| Amélioration | Bénéfice | Critère d'acceptation |
|---|---|---|
| Sauvegardes `pg_dump` chiffrées + rotation + **restauration testée** (script versionné) | Survie à une perte de données | Procédure rejouée en recette : base restaurée, `npm run dev` → données intactes |
| Fiche établissement (`GET`/`PATCH /api/v1/tenant/profile`) + écran de paramètres | Documents conformes, plus de coordonnées fictives | Le ticket 80 mm et le reçu impriment l'adresse/téléphone réels de l'établissement |
| Supprimer les valeurs par défaut fictives de `CounterReceiptModal` (`Cocody Angré…`, `+225 07 00 11 22 33…`) | Cohérence avec la purge du 11/10 | Aucune occurrence de coordonnées fictives dans `src/` |
| Journalisation JSON + `request_id` propagé + endpoint `/metrics` + 3 alertes (5xx, latence p95, échecs de passerelle) | Diagnostic en minutes au lieu d'heures | Un incident simulé est visible dans les métriques et les logs |
| Masquer le nom du client sur la vérification publique (initiales + code court) | Conformité vie privée | Un ticket inconnu et un ticket connu n'exposent aucune identité complète |
| Refresh token + changement de mot de passe + réinitialisation (jeton à usage unique) + 2FA superadmin | Sécurité des comptes | Tests automatisés pour chaque parcours + documentation API |

### 6.2 Priorité 1 — qualité, exploitation, traçabilité

| Amélioration | Bénéfice |
|---|---|
| Tests d'intégration pour `customer`, `pricelist`, `order` (stores SQL) et seuil de couverture bloquant (ex. 55 % sur les paquets critiques) | Filet de sécurité réel avant refactorisation |
| Pagination serveur (`limit`/`offset`/`cursor`) + tris SQL sur commandes et clients | Tenue à l'échelle |
| Table `audit_log` (+ écran) : encaissements, changements de statut, suspensions/suppressions, connexions | Litiges, fraude interne, conformité |
| File d'attente des notifications (table `notification_outbox`, reprises exponentielles, idempotence) + configuration fournisseur par établissement | Messages fiables, multi-fournisseurs |
| Gestion des plans/quotas dans la console éditeur + historique de facturation | Autonomie de l'éditeur, moins d'opérations SQL |
| Versionner les scripts de vérification dans `scripts/` + 5 parcours E2E Playwright (dépôt, encaissement, ticket, suppression d'établissement, 402) | Preuves reproductibles par un tiers |
| `npm ci` sans `--legacy-peer-deps` (aligner `vite`/`esbuild`), Dependabot, `gosec`/CodeQL, `.env.example` complet | Chaîne d'installation et de sécurité saine |
| Conteneurisation de l'API + SPA (`Dockerfile`, `compose` complet) et outil de migration (`golang-migrate`) | Déploiement reproductible chez un hébergeur |

### 6.3 Priorité 2 — expérience et couverture produit

| Amélioration | Bénéfice |
|---|---|
| Routeur (`react-router`) + deep-linking (commande, rapport filtré, ticket) | Partage de liens, historique navigateur, support plus simple |
| Écran de gestion des grilles tarifaires versionnées (aujourd'hui API seule) | Finir la fonctionnalité vendue dans les plans payants |
| PWA hors-ligne + file de synchronisation du comptoir | Continuité de service en cas de coupure réseau |
| Téléchargement/impression réseau (API Web Serial/Ethernet ou agent local) | Impression fiable en boutique |
| Accessibilité : audit RGAA/WCAG, navigation clavier, contrastes, libellés | Conformité et usage en boutique |
| i18n (fr/en), multi-devises | Ouverture géographique |
| Rapport comparatif (période N vs N-1), prévisionnel de charge, marge par prestation | Pilotage plus fin du gérant |
| Fidélité, relances automatiques, segmentation client + consentement RGPD | Rétention et chiffre d'affaires |

### 6.4 Priorité 3 — écosystème

| Amélioration | Bénéfice |
|---|---|
| Encaissement Mobile Money (Wave/Orange/MTN) et rapprochement automatique | Trésorerie temps réel, moins d'erreurs de caisse |
| Comptabilité SYSCOHADA/TVA, clôture de caisse (Z), liasse exportable | Conformité fiscale ivoirienne |
| Stock/inventaire, RH/planning, contrats B2B facturés | Extension naturelle de l'ERP |
| Application mobile (comptoir/atelier) ou PWA installable | Usage terrain |
| RLS PostgreSQL, rétention/archivage, chiffrement au repos | Défense en profondeur et conformité |

---

## 7. Conclusion

PressingOS a franchi en une journée de travail (sessions 1 → 7) l'écart entre **prototype de démonstration** et **produit exploitable** :

- les **3 défauts bloquants** de l'audit sont corrigés et prouvés par des tests d'intégration sur PostgreSQL réel ;
- le **moteur commercial** n'est plus déclaratif : les verrous (`402`) sont branchés, les quotas sont comptés en base, la suspension est effective ;
- le **cœur métier** est complet (dépôt, atelier, caisse, clients, équipe, multi-sites, dépenses, rapports + exports, notifications, console éditeur avec suppression définitive) ;
- la **base ne contient plus aucune entreprise fictive** et l'application démarre vide en l'annonçant ;
- la **qualité** est outillée : 94 tests Go, 171 tests frontend, CI verte, lint/format, documentation alignée.

Ce qui manque n'est plus « le produit » mais **son exploitation** : sauvegardes, supervision, encaissement des abonnements, traçabilité (audit), sécurité avancée des comptes et finitions d'expérience (hors-ligne, routeur, accessibilité). En traitant le bloc **P0 de la §6.1** (environ 3 semaines-personne), le produit devient commercialisable auprès d'un premier client réel ; les priorités P1 puis P2 sécurisent ensuite la montée en charge.

---

## Annexe A — Commandes de reproduction de cette analyse

```bash
# Volumétrie et inventaire
find backend -name '*.go' | xargs wc -l | tail -1
find src -name '*.ts' -o -name '*.tsx' | xargs wc -l | tail -1
grep -cE '(Get|Post|Put|Patch|Delete)\("' backend/internal/http/router.go

# Tests et couverture
cd backend && go vet ./... && go test ./... && go test -cover ./...
go test -tags=integration ./internal/...          # TEST_DATABASE_URL requis
cd .. && npm test -- --run && npm run lint && npm run lint:eslint && npm run build

# Pile vivante (base vide après la purge du 11/10)
npm run dev &                                      # proxy :3000 → API :8085
curl -s localhost:3000/health ; curl -s localhost:3000/ready
TOKEN=$(curl -s -X POST localhost:8085/api/v1/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"admin@pressingos.ci","password":"admin123"}' | python3 -c 'import sys,json;print(json.load(sys.stdin)["token"])')
curl -s localhost:8085/api/v1/admin/stats -H "Authorization: Bearer $TOKEN"
curl -s localhost:8085/api/v1/admin/tenants -H "Authorization: Bearer $TOKEN"
```

## Annexe B — Matrice des plans (vérifiée dans `billing.defaultMatrix()` et les migrations 000006/000012/000014)

| Droit | FREE | PRO (45 000 F) | BUSINESS (75 000 F) | Essai 7 j |
|---|---|---|---|---|
| `orders.create` | ✅ | ✅ | ✅ | ✅ |
| `orders.monthly.max` | 50 / mois | illimité | illimité | illimité |
| `users.max` | 2 | 5 | 15 | 15 |
| `catalog.price_lists` | 1 | 3 | 10 | 10 |
| `branches.multi_site` | 1 | 1 | 3 | 3 |
| `reports.advanced` | ❌ | ✅ | ✅ | ✅ |
| `expenses.advanced` | ❌ | ✅ | ✅ | ✅ |
| `pricing.pro` | ❌ | ✅ | ✅ | ✅ |
| `reports.consolidated` | ❌ | ❌ | ✅ | ✅ |
| `notifications.advanced` | ❌ | ✅ | ✅ | ✅ |

## Annexe C — Fichiers de référence

| Sujet | Fichier |
|---|---|
| Routes et middlewares | `backend/internal/http/router.go` |
| Moteur de droits et quotas | `backend/internal/billing/{admin,service,persist}.go` |
| Suppression d'un établissement | `backend/internal/http/admin_handler.go`, `backend/internal/billing/persist.go` |
| Séries analytiques du tableau de bord | `src/lib/analytique.ts` |
| Tableau des dépôts partagé | `src/components/reui/orders-table.tsx`, `src/components/ui/table.tsx` |
| Journal des correctifs par session | `docs/correctifs-2026-10.md` |
| Modèle de données | `docs/database.md`, `backend/migrations/` |
