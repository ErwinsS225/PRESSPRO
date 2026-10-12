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
- **`RequireFeatureQuota`** (plafond consommé) : la consommation réelle est mesurée sur les données (`orders.monthly.max`, `users.max`, `catalog.price_lists`, `branches.multi_site`) et comparée à `plan_entitlements.limit_value` → `402` avec le message métier et `usage`.

Les **lectures ne sont jamais payantes** : une rétrogradation de formule ne rend pas l'historique inaccessible. Le superadmin est dispensé de ces garde-fous.

### Abonnement : droits effectifs, grille et fenêtre de commandes

Le backend est la seule source des droits. Le frontend ne code ni prix ni plafond.

- **`GET /api/v1/me/entitlements`** (authentifié) : formule effective (`plan_code`, droits pendant l'essai : BUSINESS), formule réellement souscrite (`subscribed_plan_code` : FREE pendant l'essai), `plan_name`, `status`, `on_trial`, `trial_ends_at`, `days_left_trial`, `price_monthly_xof` (tarif de la formule souscrite, lu en base ; **absent en mode mémoire**), `orders_period` (`start`, `end` ; fin exclue) et `entitlements[]` (`key`, `enabled`, `limit`, `usage`, `remaining`, `source`).
- **`GET /api/v1/billing/plans`** (authentifié) : grille des formules `{ "plans": [{code, name, price_monthly_xof, entitlements[]}] }`. Tarifs lus dans `plans.price_monthly_xof` ; **absents en mode mémoire**.
- **Essai** : 7 jours avec les droits BUSINESS, initialisé une seule fois (un second appel ne prolonge pas l'essai). À l'échéance, le passage à FREE est fait côté serveur, persisté et journalisé dans `subscription_events`, sans suppression de données.
- **Fenêtre de commandes** : quota glissant de 30 jours. L'ancre est le début de l'essai, sinon la création de l'abonnement, sinon la création de l'établissement. Les commandes annulées ne comptent pas.

État actuel du câblage :
- `POST /api/v1/price-lists` (plafond de grilles : 1 gratuite, 3 en PRO, 10 en BUSINESS) et `POST /api/v1/price-lists/{id}/versions` ;
- `POST /api/v1/users` (plafond de comptes : 2 en gratuit, 5 en PRO, 15 en BUSINESS — établissement d'essai : 15) ;
- `POST /api/v1/branches` (plafond d'établissements : 1 en gratuit et PRO, 3 en BUSINESS) ;
- `GET /api/v1/reports/summary` (`reports.advanced`) et `POST`/`DELETE /api/v1/expenses` (`expenses.advanced`) ;
- `POST /api/v1/notifications` (`notifications.advanced`) — la composition et la **lecture du journal** restent ouvertes à tous.

### Abonnement suspendu
Lorsqu'un éditeur suspend un abonnement (`PATCH /api/v1/admin/tenants/{id}/status`), les **modules métier** de l'établissement (catalogue, commandes, clients, grilles) répondent `402` avec `{"error":"abonnement suspendu…","subscription_status":"SUSPENDED"}`. Les routes de compte (`/tenant/current`, `/me/entitlements`) et l'authentification restent ouvertes afin que le pressing puisse constater sa situation ; le superadmin conserve l'accès complet.

### Supervision éditeur (SuperAdmin)
| Méthode | Route | Rôle | Description |
|---|---|---|---|
| GET | `/api/v1/admin/stats` | superadmin | Indicateurs consolidés réels : établissements, utilisateurs actifs, abonnements actifs/en essai, MRR (somme des formules ACTIVE ; `null` en mode mémoire, où aucun tarif n'est stocké), répartitions par statut et par formule, `data_source` (`postgresql` ou `memoire`). |
| GET | `/api/v1/admin/tenants` | superadmin | Annuaire des établissements : nom, ville, téléphone, compte propriétaire, formule, prix (`null` en mode mémoire : tarif inconnu, jamais 0 inventé), statut, fin d'essai. Aucune donnée client ni commande n'est exposée. |
| PATCH | `/api/v1/admin/tenants/{id}/status` | superadmin | Suspend / réactive un abonnement. Corps : `{"status":"SUSPENDED"\|"ACTIVE"\|"PAST_DUE"\|"CANCELED","reason":"…"}`. La décision et sa trace (`subscription_events`) sont écrites dans **une seule transaction** : `404` si l'établissement n'a pas d'abonnement, `400` si le statut est invalide. |
| PATCH | `/api/v1/admin/plans/{code}/price` | superadmin | **Modifie le tarif mensuel d'une formule payante** (PRO, BUSINESS). Corps : `{\"price_monthly_xof\":10000,\"reason\":\"…\"}` (entier XOF de 0 à 100 000 000, motif ≤ 500 caractères). Réponse `200` `{\"code\",\"price_monthly_xof\"}`. Chaque changement effectif est journalisé dans `plan_price_history` (ancien et nouveau prix, acteur, motif, date) par trigger SQL ; un montant identique n'ajoute aucune ligne. Les abonnements suivent le catalogue (pas de tarif figé par abonnement) et les commandes déjà créées ne sont pas modifiées. `400` pour la formule gratuite, un montant non entier ou hors bornes, ou un motif trop long ; `404` pour une formule inconnue ; `503` en mode mémoire (aucun tarif stocké : refus plutôt que simulation). |
| DELETE | `/api/v1/admin/tenants/{id}` | superadmin | **Retrait définitif** d'un établissement et de toutes ses données : comptes, clients, commandes, lignes de commande, paiements, catalogue, grilles, dépenses, établissements secondaires, notifications et abonnement. **Transaction unique** (les tables `orders` et `payments` ne cascadent pas depuis `tenants` : elles sont purgées explicitement) → `204` sans corps, `404` si l'établissement n'existe plus, identifiant non UUID refusé de la même façon. Action **irréversible** ; l'annuaire du frontend n'appelle cette route qu'après confirmation explicite. |

### Suppression d'un établissement (nettoyage des environnements de démonstration)
`DELETE /api/v1/admin/tenants/{id}` retire l'établissement **et tout son contenu** : le schéma
supprime en cascade les comptes, clients, prestations, dépenses, notifications et abonnements ;
les commandes, leurs lignes et les paiements sont retirés explicitement dans la même transaction
avant l'établissement (aucune ligne orpheline possible, même si une étape échoue). La route sert
notamment à purger les entreprises de test d'un environnement, sans script SQL ponctuel.

### Module Équipe (comptes utilisateurs, fonctionnalité payante `users.max`)
| Méthode | Route | Rôle | Description |
|---|---|---|---|
| GET | `/api/v1/users` | owner, manager (superadmin) | Liste les comptes de l'établissement (isolation par `tenant_id` du jeton). La gestion d'équipe remplace l'ancienne route `GET /api/v1/auth/users`, retirée. |
| POST | `/api/v1/users` | owner, manager (superadmin) | Crée un compte employé dans **une seule transaction** (`users` + `tenant_memberships` + `branch_memberships` sur l'établissement principal). Corps : `{"full_name","email","phone","role","password"}`. Rôles attribuables : `manager`, `cashier`, `operator` (le rôle `superadmin` est refusé `400`). `201` avec la fiche créée (email normalisé en minuscules, mot de passe ≥ 6 caractères). |
| GET | `/api/v1/users/{id}/branches` | owner, manager (superadmin) | Établissements du pressing avec l'affectation du compte : `{"user_id":"…","branches":[{"id","name","is_primary","status","assigned"}]}`. |
| PUT | `/api/v1/users/{id}/branches` | owner, manager (superadmin) | **Remplace** l'affectation du compte. Corps : `{"branch_ids":["…","…"]}`. Réponse identique au `GET` (état après enregistrement). |
| PATCH | `/api/v1/users/{id}/status` | owner, manager (superadmin) | Active (`active`) ou désactive (`inactive`) un compte. Corps : `{"status":"active"\|"inactive"}`. La mise à jour du compte **et** de l'appartenance est transactionnelle : la désactivation ferme immédiatement les routes métier, y compris pour un jeton déjà émis. |

Réponses : `402` `{"error":"plafond atteint (N) — passez à une formule supérieure","feature":"users.max","usage":N,"upgrade_required":"true"}` à la création au-delà du plafond de la formule ; `409` si l'email est déjà utilisé ; `404` si l'identifiant est inconnu **ou appartient à un autre établissement** ; `400` pour un rôle/statut invalide, un mot de passe trop court ou une auto-désactivation (un gérant ne peut pas se fermer l'accès lui-même) ; `403` pour un caissier ou un opérateur, et `403` pour un gérant qui crée un compte propriétaire (`role: owner`) ou modifie le statut d'un propriétaire : ces opérations sont réservées au propriétaire.

### Affectation d'un employé à un établissement (périmètre d'exploitation)
Un compte n'opère que sur les établissements qui lui sont affectés (`branch_memberships`) :
à la création, il est rattaché à l'**établissement principal** ; tant qu'aucune sélection
explicite n'existe, ce site principal reste son périmètre par défaut.

Sémantique du `PUT` :
- **remplacement exclusif** — les sites absents de `branch_ids` sont retirés, les nouveaux ajoutés (transaction unique) ;
- `400` `{"error":"au moins un établissement doit être affecté"}` pour une sélection vide (fail-closed : un employé sans site ne peut ni encaisser ni saisir) ;
- `400` pour un site inconnu du pressing **ou archivé** ;
- `404` si le compte visé appartient à un autre pressing ;
- `403` pour un caissier ou un opérateur (gestion réservée au propriétaire/gérant).

Effet immédiat : le middleware `RequireBranchScope` valide l'en-tête `X-Branch-ID`
contre ces affectations ; retirer un site ferme donc l'accès aux commandes qui y sont
rattachées dès l'enregistrement, sans reconnexion.

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
- **Réponse** : `order_number`, `status`, `payment_status`, `total_amount`, `paid_amount`, `remaining_due`, `due_date`, `items[]` (le nom du client n'est **pas** exposé sur cette route publique)

---

## 5. Endpoints Catalogue & Persistance PostgreSQL (v0.4.0+)

La table `services` est alignée sur le modèle Go via la migration `000002_catalog_enrichi` :
`article`, `treatment`, `pricing_model`, `is_express_available`, `express_surcharge`,
`is_pickup_available`, `is_outsourced`, `requires_quote`, `description`.

Au démarrage, le catalogue bascule sur PostgreSQL quand le pool est joignable,
avec repli mémoire (seed) sinon. `GET /api/v1/services` accepte `category` et `q`
avec pagination SQL `ORDER BY name ASC`.

---

## 6. Endpoints Établissements, Dépenses & Rapports (modules payants)

Trois modules complets sont ajoutés au groupe des **modules métier** : ils sont donc
également gelés (`402`) lorsqu'un abonnement est suspendu, et **les lectures ne sont
jamais payantes** (une rétrogradation de formule ne rend pas l'historique inaccessible).

### Module Établissements (multi-sites) — plafond `branches.multi_site`
| Méthode | Route | Rôle | Description |
|---|---|---|---|
| GET | `/api/v1/branches` | tous les membres | Liste les établissements du pressing (id, nom, code, adresse, ville, téléphone, `is_primary`, `status`, `orders_count`). L'établissement principal est créé à l'inscription. |
| POST | `/api/v1/branches` | owner, manager (superadmin) | Crée un établissement secondaire. Corps : `{"name","code","address","phone","city"}`. Gardé par `RequireFeatureQuota(branches.multi_site)` : 1 site en gratuit et PRO, 3 en BUSINESS (essai actif = niveau BUSINESS) → `402` avec `feature: branches.multi_site` et `usage`. Les membres actifs du pressing sont rattachés au nouveau site (`branch_memberships`) pour pouvoir y opérer. |
| PATCH | `/api/v1/branches/{id}` | owner, manager (superadmin) | Modification **partielle** : seuls les champs fournis changent (`name`, `code`, `address`, `phone`, `city`, `status`). Statuts : `active`, `suspended`, `archived`. L'établissement principal ne peut être ni suspendu ni archivé (`400`). |

Réponses : `409` si un établissement porte déjà ce nom (comparaison insensible à la
casse et aux espaces superflus) ; `404` si l'identifiant est inconnu ou appartient à
un autre pressing ; `400` pour un nom manquant (120 caractères maximum) ou un statut
invalide.

### Module Dépenses — fonctionnalité `expenses.advanced`
| Méthode | Route | Rôle | Description |
|---|---|---|---|
| GET | `/api/v1/expenses` | tous les membres | Dépenses de la période : `from`, `to` (`AAAA-MM-JJ`, défaut = mois courant), `category`, `branch_id`. Réponse : `{"expenses":[…],"count":N,"total_xof":N,"by_category":[…],"period":{…}}` — la lecture reste ouverte même en formule gratuite. |
| POST | `/api/v1/expenses` | owner, manager (superadmin) | Enregistre une charge. Corps : `{"category","label","amount_xof","spent_at","branch_id","payment_method","note"}` (`spent_at` par défaut = aujourd'hui ; `branch_id` déduit de l'en-tête `X-Branch-ID` s'il est omis). Gardé par `RequireFeature(expenses.advanced)`. |
| DELETE | `/api/v1/expenses/{id}` | owner, manager (superadmin) | Supprime une charge du pressing. `404` pour une dépense d'un autre établissement. Gardé par `RequireFeature(expenses.advanced)` : en formule gratuite (après rétrogradation), la suppression répond `402` et les dépenses existantes restent conservées (décision D6). |

Catégories du référentiel : `matieres_premieres`, `produits_entretien`, `loyer`,
`energie_eau`, `salaires`, `transport`, `equipement`, `maintenance`, `taxes`, `divers`.
`400` pour un libellé vide, un montant ≤ 0, une catégorie inconnue ou une date
invalide.

### Module Notifications clients — fonctionnalité `notifications.advanced`

| Méthode | Route | Rôle | Description |
|---|---|---|---|
| GET | `/api/v1/notifications` | tous les membres | **Journal des messages** du pressing, du plus récent au plus ancien. Filtres : `order_id`, `channel` (`whatsapp`\|`sms`), `status` (`simule`\|`envoye`\|`echec`), `limit` (défaut 50, maximum 200). Réponse : `{"notifications":[…],"count":N,"channels":["whatsapp","sms"],"templates":[…]}`. La lecture reste ouverte même en formule gratuite : l'historique appartient au pressing. |
| POST | `/api/v1/notifications` | tous les membres | Compose puis envoie un message client. Corps : `{"order_id","channel","template","message","recipient"}` → `201` `{"notification":…,"status":…,"message":…}`. Gardé par `RequireFeature(notifications.advanced)` : `402` en formule gratuite, l'interface propose alors l'ouverture manuelle. |

**Canaux** : `whatsapp`, `sms`. **Modèles** : `commande_prete`, `commande_livree`,
`paiement_recu`, `rappel_retrait` — chacun compose un texte français avec le nom du
pressing (issu du jeton, jamais du client), le numéro de commande, le nom du client, le
solde restant et l'échéance. `message` remplace le modèle ; un `message` vide (ou blanc)
retombe sur `template`, lui-même par défaut `commande_prete`.

**Destinataire** : `recipient` sinon le téléphone du client de la commande. `400` pour un
canal ou un modèle inconnu ou un téléphone manquant ; `404` « commande introuvable » pour
un identifiant inconnu, mal formé ou appartenant à un autre pressing.

**Envoi réel ou journalisé** : avec `NOTIFICATIONS_GATEWAY_URL`, le message est transmis à
la passerelle (POST JSON `{"to","channel","body","order_id","tenant_id"}` et, si
`NOTIFICATIONS_GATEWAY_TOKEN` est défini, l'en-tête `Authorization: Bearer <jeton>`, délai
5 s) ; la notification est alors enregistrée avec `status: "envoye"` et
`provider: "passerelle"`. Sans passerelle — ou si elle échoue — le message reste
**journalisé** (`status: "simule"` ou `"echec"`, `detail` explicite) : l'interface propose
toujours « Ouvrir dans WhatsApp / SMS » pour envoyer depuis le téléphone du comptoir.

**Déclenchement automatique** : lorsqu'une commande passe à `ready` ou `delivered`, le
service des commandes notifie le client en tâche de fond (hors du chemin critique, 10 s
maximum, panne avalée) **si la formule inclut `notifications.advanced`** ; la notification
porte alors `created_by` vide. Les autres transitions n'envoient rien.

**Variable d'environnement** : `NOTIFICATIONS_GATEWAY_URL` (facultative) active l'envoi
réel ; `NOTIFICATIONS_GATEWAY_TOKEN` (facultative) ajoute le jeton Bearer. Sans URL, tout
reste dans le journal local.

### Module Rapports — fonctionnalités `reports.advanced` et `reports.consolidated`
| Méthode | Route | Rôle | Description |
|---|---|---|---|
| GET | `/api/v1/reports/summary?from=AAAA-MM-JJ&to=AAAA-MM-JJ` | owner, manager (superadmin) | Rapport d'activité de la période (défaut : mois courant). Calcule sur les données réelles : CA **encaissé**, nombre de commandes, `orders_by_status`, ticket moyen, dépenses de la période et leur répartition par poste, **marge = CA encaissé − dépenses**, top 5 des prestations. `data_source` indique `postgresql` ou `memoire`. `400` (`ErrInvalidPeriod`) si `from` > `to` ou si une date est illisible. |
| GET | `/api/v1/reports/export?format=csv\|pdf&from=AAAA-MM-JJ&to=AAAA-MM-JJ` | owner, manager (superadmin) | **Export du rapport** (`format` par défaut `csv`). Mêmes chiffres que `/summary` (dépenses et consolidation incluses). Réponse en pièce jointe : `Content-Disposition: attachment; filename="rapport-<slug>-<from>_<to>.<ext>"` (slug dérivé du nom du pressing). `400` pour un format inconnu ou une période invalide ; `402` (`reports.advanced`) en formule gratuite, comme l'écran. |

**Formats d'export** : le **CSV** utilise le séparateur `;` et un BOM UTF-8 (ouverture
directe dans Excel FR), avec CRLF et des en-têtes français ; le **PDF** est au format 1.4
(Helvetica/WinAnsi, A4, pagination automatique au-delà de 46 lignes) et se compose sans
aucune dépendance externe. Les montants y sont formatés comme à l'écran (espaces fines) et
les métadonnées (période, établissement, source de données) rappellent le contexte.

Consolidation multi-sites : quand la formule inclut `reports.consolidated` (BUSINESS,
essai actif inclus), la réponse porte `consolidated: true` et `by_branch` (CA, dépenses
et marge par établissement). Sinon le rapport reste complet et `plan_hint` rappelle
que « La consolidation multi-sites est incluse dans la formule BUSINESS. » — l'interface
affiche une invitation commerciale à la place du tableau consolidé.
