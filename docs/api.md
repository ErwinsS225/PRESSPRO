# PressingOS — Spécification de l'API REST (v0.3.0-orders)

Documentation complète des endpoints HTTP du serveur Go Chi, incluant l'authentification JWT, l'isolation multi-tenant, les commandes de pressing et la génération de tickets thermiques (80mm).

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
