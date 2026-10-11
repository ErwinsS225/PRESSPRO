# PressingOS — Spécification de l'API REST (v0.2.0-auth)

Documentation complète des endpoints HTTP du serveur Go Chi, incluant l'authentification JWT, l'isolation multi-tenant et le contrôle d'accès basé sur les rôles (RBAC).

---

## 1. Endpoints Publics de Santé & Diagnostic

### `GET /health`
Vérifie la vivacité (Liveness) du processus backend Go.
- **Authentification** : Aucune (Public)
- **Code HTTP** : `200 OK`
- **Réponse** :
```json
{
  "status": "ok",
  "service": "pressingos-api",
  "timestamp": "2026-10-11T06:30:00Z"
}
```

### `GET /ready`
Vérifie l'état de préparation opérationnelle (Readiness) incluant la connectivité active à PostgreSQL.
- **Authentification** : Aucune (Public)
- **Codes HTTP** : `200 OK` (connecté) ou `503 Service Unavailable` (déconnecté)
- **Réponse (200)** :
```json
{
  "status": "ready",
  "service": "pressingos-api",
  "database": "connected",
  "timestamp": "2026-10-11T06:30:00Z"
}
```

### `GET /api/v1/system/info`
Métadonnées du monolithe modulaire.
- **Authentification** : Aucune (Public)
- **Code HTTP** : `200 OK`

---

## 2. Endpoints d'Authentification & Onboarding (Partie 2)

### `POST /api/v1/auth/login`
Connexion utilisateur par email et mot de passe.
- **Authentification** : Aucune (Public)
- **Corps de requête (JSON)** :
```json
{
  "email": "gerant@pressing-ivoire.ci",
  "password": "pass123"
}
```
- **Réponse (200 OK)** :
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": "usr-gerant-001",
    "tenant_id": "a0000000-0000-0000-0000-000000000001",
    "email": "gerant@pressing-ivoire.ci",
    "full_name": "M. Bamba Seydou",
    "role": "owner",
    "status": "active"
  },
  "tenant": {
    "id": "a0000000-0000-0000-0000-000000000001",
    "name": "Pressing Ivoire Cocody",
    "slug": "pressing-ivoire-cocody",
    "city": "Abidjan (Cocody)",
    "currency": "XOF",
    "status": "active"
  }
}
```

---

### `POST /api/v1/auth/register-tenant`
Inscription d'un nouveau pressing (Création de l'établissement et du compte propriétaire).
- **Authentification** : Aucune (Public)
- **Corps de requête (JSON)** :
```json
{
  "tenant_name": "Pressing du Plateau",
  "city": "Abidjan (Plateau)",
  "phone": "+225 07 11 22 33 44",
  "owner_full_name": "Kone Amadou",
  "owner_email": "kone@plateau-pressing.ci",
  "password": "SecretMotDePasse2026"
}
```
- **Réponse (201 Created)** : Retourne le jeton JWT et les métadonnées du tenant créé.

---

### `GET /api/v1/auth/me`
Retourne le profil de l'utilisateur connecté d'après son jeton.
- **En-tête requis** : `Authorization: Bearer <token>`
- **Code HTTP** : `200 OK` (ou `401 Unauthorized`)

---

## 3. Endpoints Protégés & Multi-Tenant

### `GET /api/v1/tenant/current`
Retourne le contexte de l'établissement actif associé au jeton.
- **En-tête requis** : `Authorization: Bearer <token>`
- **Sécurité d'isolation** : Si le client tente de forger `X-Tenant-ID: <autre_id>`, le middleware renvoie immédiatement **403 Forbidden**.

### `GET /api/v1/admin/stats` (SuperAdmin Uniquement)
Retourne les métadonnées d'abonnements globaux de la plateforme.
- **Rôle requis** : `superadmin`
- **Code HTTP** : `200 OK` pour superadmin, `403 Forbidden` pour tout autre rôle.

---

## 4. Comptes de Démonstration Pré-Configurés

| Rôle | Email | Mot de passe | Établissement | Périmètre |
|---|---|---|---|---|
| **owner** | `gerant@pressing-ivoire.ci` | `pass123` | Pressing Cocody | Gestion complète du pressing, finances, tarifs |
| **operator** | `atelier@pressing-ivoire.ci` | `pass123` | Pressing Cocody | Suivi des étapes de lavage et repassage en atelier |
| **superadmin**| `admin@pressingos.ci` | `admin123` | SaaS Global | Éditeur de logiciel, liste des pressings et MRR |
