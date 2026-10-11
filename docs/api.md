# PressingOS — Spécification de l'API REST (v0.5.0-hardening)

Documentation complète des endpoints HTTP du serveur Go Chi, incluant l'authentification JWT, l'isolation multi-tenant, les commandes de pressing et la génération de tickets thermiques (80mm).

## 0. Conventions transverses

### Format des erreurs
Toutes les erreurs respectent le format `{"error": "message"}` :
- **messages métier** (validation, transition interdite, plafond atteint) : lisibles et destinés à l'interface ;
- **erreurs techniques** (SQL, réseau, transaction) : journalisées côté serveur, **jamais** exposées au client (réponse 500 générique) ;
- **codes** : `400` validation · `401` authentification · `402` quota/plan · `403` rôle ou isolation · `404` ressource inconnue **ou appartenant à un autre établissement** (l'existence n'est jamais révélée) · `409` conflit (email déjà utilisé, nom de grille déjà existant pour l'établissement) · `429` trop de tentatives · `500` erreur interne.

### Limitation de débit
`POST /api/v1/auth/login` est protégé contre la force brute :
- 10 tentatives par couple (IP, email) et 40 tentatives par adresse IP, sur une fenêtre de 5 minutes ;
- au-delà, réponse `429` avec en-tête `Retry-After` (secondes) ;
- une connexion **réussie** remet le compteur à zéro : un utilisateur légitime n'est jamais bloqué par ses fautes de frappe.

### Fonctionnalités payantes (formules)
Deux garde-fous distincts protègent les modules payants :
- **`RequireFeature`** (droit booléen) : la fonctionnalité doit être incluse dans la formule → `402` `{"error","feature","upgrade_required":"true"}` ;
- **`RequireFeatureQuota`** (plafond consommé) : la consommation réelle est mesurée sur les données (`orders.monthly.max`, `users.max`, `catalog.price_lists`) et comparée à `plan_entitlements.limit_value` → `402` avec le message métier et `usage`.

Les **lectures ne sont jamais payantes** : une rétrogradation de formule ne rend pas l'historique inaccessible. Le superadmin est dispensé de ces garde-fous.

État actuel du câblage : `POST /api/v1/price-lists` (plafond de grilles : 1 gratuite, 3 en PRO, 10 en BUSINESS), `POST /api/v1/price-lists/{id}/versions` et `POST /api/v1/users` (plafond de comptes : 2 en gratuit, 5 en PRO, 15 en BUSINESS — établissement d'essai : 15).

### Abonnement suspendu
Lorsqu'un éditeur suspend un abonnement (`PATCH /api/v1/admin/tenants/{id}/status`), les **modules métier** de l'établissement (catalogue, commandes, clients, grilles) répondent `402` avec `{"error":"abonnement suspendu…","subscription_status":"SUSPENDED"}`. Les routes de compte (`/tenant/current`, `/me/entitlements`) et l'authentification restent ouvertes afin que le pressing puisse constater sa situation ; le superadmin conserve l'accès complet.

### Supervision éditeur (SuperAdmin)
| Méthode | Route | Rôle | Description |
|---|---|---|---|
| GET | `/api/v1/admin/stats` | superadmin | Indicateurs consolidés réels : établissements, utilisateurs actifs, abonnements actifs/en essai, MRR (somme des formules ACTIVE), répartitions par statut et par formule, `data_source` (`postgresql` ou `memoire`). |
| GET | `/api/v1/admin/tenants` | superadmin | Annuaire des établissements : nom, ville, téléphone, compte propriétaire, formule, prix, statut, fin d'essai. Aucune donnée client ni commande n'est exposée. |
| PATCH | `/api/v1/admin/tenants/{id}/status` | superadmin | Suspend / réactive un abonnement. Corps : `{"status":"SUSPENDED"\|"ACTIVE"\|"PAST_DUE"\|"CANCELED","reason":"…"}`. La décision et sa trace (`subscription_events`) sont écrites dans **une seule transaction** : `404` si l'établissement n'a pas d'abonnement, `400` si le statut est invalide. |

### Module Équipe (comptes utilisateurs, fonctionnalité payante `users.max`)
| Méthode | Route | Rôle | Description |
|---|---|---|---|
| GET | `/api/v1/users` | owner, manager (superadmin) | Liste les comptes de l'établissement (isolation par `tenant_id` du jeton). La gestion d'équipe remplace l'ancienne route `GET /api/v1/auth/users`, retirée. |
| POST | `/api/v1/users` | owner, manager (superadmin) | Crée un compte employé dans **une seule transaction** (`users` + `tenant_memberships` + `branch_memberships` sur l'établissement principal). Corps : `{"full_name","email","phone","role","password"}`. Rôles attribuables : `manager`, `cashier`, `operator` (le rôle `superadmin` est refusé `400`). `201` avec la fiche créée (email normalisé en minuscules, mot de passe ≥ 6 caractères). |
| PATCH | `/api/v1/users/{id}/status` | owner, manager (superadmin) | Active (`active`) ou désactive (`inactive`) un compte. Corps : `{"status":"active"\|"inactive"}`. La mise à jour du compte **et** de l'appartenance est transactionnelle : la désactivation ferme immédiatement les routes métier, y compris pour un jeton déjà émis. |

Réponses : `402` `{"error":"plafond atteint (N) — passez à une formule supérieure","feature":"users.max","usage":N,"upgrade_required":"true"}` à la création au-delà du plafond de la formule ; `409` si l'email est déjà utilisé ; `404` si l'identifiant est inconnu **ou appartient à un autre établissement** ; `400` pour un rôle/statut invalide, un mot de passe trop court ou une auto-désactivation (un gérant ne peut pas se fermer l'accès lui-même) ; `403` pour un caissier ou un opérateur.

Le module est placé dans le groupe des **modules métier** : un abonnement suspendu le gèle également (`402`, message d'abonnement), tandis que `/tenant/current` et `/me/entitlements` restent ouverts.

### Onboarding d'un nouvel établissement
`POST /api/v1/auth/register-tenant` crée, en une transaction : l'organisation, l'établissement principal, le compte propriétaire, les appartenances (tenant + branche), l'abonnement d'essai 7 jours, **la grille tarifaire par défaut (catalogue)** et **la première version figée de la grille tarifaire**. Un pressing fraîchement inscrit est donc immédiatement opérationnel.

---

## 1. Endpoints Publics de Santé & Diagnostic

### `GET /health`
Vérifie la vivacité (Liveness) du processus backend Go.
- **Authentification** : Aucune (Public)
- **Code HTTP** : `200 OK`

### `GET /ready`
Vérifie l'état de préparation opérationnelle (Readiness) incluant la connectivité active à PostgreSQL.
- **Authentification** : Aucune (Public)
- **Codes HTTP** : `200 OK` (connecté) ou `503 Service Unavailable` (déconnecté)

### `GET /api/v1/system/info`
Métadonnées du monolithe modulaire.

---

## 2. Endpoints d'Authentification & Onboarding (Partie 2)

### `POST /api/v1/auth/login`
Connexion utilisateur par email et mot de passe.
- **Authentification** : Aucune (Public)
- **Retourne** : Jeton JWT signé HMAC-SHA256, profil utilisateur et établissement.
- **Compte désactivé** : `403` `{"error":"compte désactivé — contactez le responsable de votre établissement"}` ; la désactivation (module Équipe) suspend aussi l'appartenance, donc les jetons déjà émis perdent l'accès aux routes métier (`403`).

### `POST /api/v1/auth/register-tenant`
Inscription d'un nouveau pressing (Création de l'établissement et du compte propriétaire).

### `GET /api/v1/auth/me`
Retourne le profil de l'utilisateur connecté d'après son jeton.
- **En-tête requis** : `Authorization: Bearer <token>`

---

## 3. Endpoints Protégés & Multi-Tenant

### `GET /api/v1/tenant/current`
Retourne le contexte de l'établissement actif associé au jeton.
- **Sécurité d'isolation** : Si le client tente de forger `X-Tenant-ID: <autre_id>`, le middleware renvoie immédiatement **403 Forbidden**.

### `GET /api/v1/admin/stats` (SuperAdmin Uniquement)
Retourne les métadonnées d'abonnements globaux de la plateforme.
- **Rôle requis** : `superadmin`

---

## 4. Endpoints Commandes, Flux Linge & Tickets Thermiques (Partie 3)

### `GET /api/v1/orders`
Liste les commandes appartenant strictement au `tenant_id` authentifié.
- **En-tête requis** : `Authorization: Bearer <token>`
- **Paramètres de requête** :
  - `status` (optionnel) : `received`, `washing`, `ironing`, `ready`, `delivered`
  - `q` (optionnel) : recherche par nom, numéro de ticket ou téléphone

### `POST /api/v1/orders`
Enregistre un nouveau dépôt de linge.
- **En-tête requis** : `Authorization: Bearer <token>`
- **Corps de requête (JSON)** :
```json
{
  "customer_name": "Kouassi Jean-Marc",
  "customer_phone": "+225 07 48 12 90 11",
  "items": [
    { "service_id": "srv-1", "item_label": "Chemise classique", "quantity": 3, "unit_price": 1000 },
    { "service_id": "srv-2", "item_label": "Pantalon costume", "quantity": 2, "unit_price": 1500 }
  ],
  "initial_paid": 6000,
  "payment_method": "wave",
  "due_date": "Demain à 17:00",
  "notes": "Amidon léger sur cols"
}
```
- **Code HTTP** : `201 Created`
- **Génération automatique** : Numéro séquentiel au format `CMD-2026-XXX`.

### `PATCH /api/v1/orders/{id}/status`
Avance l'état d'avancement du vêtement dans le cycle d'atelier.
- **En-tête requis** : `Authorization: Bearer <token>`
- **Corps de requête** : `{"status": "washing" | "ironing" | "ready" | "delivered"}`
- **Code HTTP** : `200 OK`

### `POST /api/v1/orders/{id}/payments`
Enregistre le versement d'un acompte ou le règlement du solde restant.
- **En-tête requis** : `Authorization: Bearer <token>`
- **Corps de requête** :
```json
{
  "amount": 3500,
  "payment_method": "wave",
  "reference": "WAVE-CI-998877"
}
```

### `GET /api/v1/orders/{id}/ticket`
Génère le ticket de caisse thermique standard au format 80mm ESC/POS textuel.
- **En-tête requis** : `Authorization: Bearer <token>`
- **Content-Type** : `text/plain; charset=utf-8`
- **Contenu** : En-tête pressing, dates, nom du client, tableau des articles, total, acompte, solde restant dû et code-barres textuel.

### `GET /api/v1/public/verify/{tenantID}/{orderID}`
Vérifie l'authenticité d'un ticket scanné depuis le QR du reçu thermique 80mm.
- **Authentification** : Aucune (Public — projection limitée sans PII sensible)
- **Codes HTTP** : `200 OK` (`{"valid": true, ...}`) ou `404` (`{"valid": false}`)
- **Réponse** : `order_number`, `status`, `payment_status`, `total_amount`, `paid_amount`, `remaining_due`, `due_date`, `customer_name`, `items[]`

---

## 5. Endpoints Catalogue & Persistance PostgreSQL (v0.4.0+)

La table `services` est alignée sur le modèle Go via la migration `000002_catalog_enrichi` :
`article`, `treatment`, `pricing_model`, `is_express_available`, `express_surcharge`,
`is_pickup_available`, `is_outsourced`, `requires_quote`, `description`.

Au démarrage, le catalogue bascule sur PostgreSQL quand le pool est joignable,
avec repli mémoire (seed) sinon. `GET /api/v1/services` accepte `category` et `q`
avec pagination SQL `ORDER BY name ASC`.
