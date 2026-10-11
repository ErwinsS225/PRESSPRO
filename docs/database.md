# PressingOS — Modèle de Base de Données et Migrations

Ce document détaille le modèle relationnel PostgreSQL 16 retenu pour **PressingOS**, les choix d'indexation, les contraintes d'intégrité et la gestion des migrations SQL.

---

## 1. Vue d'Ensemble des Tables

Le schéma utilise des identifiants **UUID v4** pour garantir l'unicité globale et simplifier les futures intégrations hors-ligne ou multi-serveurs sans collision d'identifiants séquentiels.

### 1. `tenants` (Établissements / Pressings)
| Colonne | Type | Contraintes | Description |
|---|---|---|---|
| `id` | UUID | PRIMARY KEY | Identifiant unique de l'établissement |
| `name` | VARCHAR(255) | NOT NULL | Nom commercial (ex: Pressing Ivoire Cocody) |
| `slug` | VARCHAR(100) | NOT NULL UNIQUE | Identifiant URL convivial (ex: pressing-ivoire) |
| `phone` | VARCHAR(50) | - | Téléphone de contact (format Côte d'Ivoire) |
| `city` | VARCHAR(100) | DEFAULT 'Abidjan' | Ville de domiciliation |
| `currency` | VARCHAR(10) | DEFAULT 'XOF' | Devise monétaire (Franc CFA BCEAO) |
| `status` | VARCHAR(50) | CHECK (active, suspended, trial) | Statut de l'abonnement du pressing |

---

### 2. `users` (Employés & Administrateurs)
| Colonne | Type | Contraintes | Description |
|---|---|---|---|
| `id` | UUID | PRIMARY KEY | Identifiant unique utilisateur |
| `tenant_id` | UUID | REFERENCES tenants(id) | Établissement de rattachement (NULL pour SuperAdmin global) |
| `email` | VARCHAR(255) | UNIQUE NOT NULL | Adresse de connexion |
| `password_hash` | VARCHAR(255) | NOT NULL | Empreinte sécurisée (Argon2id ou bcrypt) |
| `full_name` | VARCHAR(255) | NOT NULL | Prénom et nom |
| `role` | VARCHAR(50) | CHECK (superadmin, owner, manager, cashier, operator) | Rôle dans l'application |

---

### 3. `customers` (Fichier Clients par Pressing)
| Colonne | Type | Contraintes | Description |
|---|---|---|---|
| `id` | UUID | PRIMARY KEY | Identifiant client |
| `tenant_id` | UUID | NOT NULL REFERENCES tenants(id) | Isolation stricte par pressing |
| `first_name` | VARCHAR(100) | NOT NULL | Prénom du client |
| `last_name` | VARCHAR(100) | NOT NULL | Nom de famille |
| `phone` | VARCHAR(50) | NOT NULL | Numéro WhatsApp / GSM |
| `address` | TEXT | - | Quartier / Résidence à Abidjan |

> **Contrainte d'intégrité unique** : `UNIQUE(tenant_id, phone)` : un même client ne peut pas être dupliqué deux fois avec le même numéro dans un même pressing, mais deux pressings distincts peuvent avoir le même numéro client dans leurs bases respectives sans collision.

---

### 4. `services` (Catalogue des Prestations et Tarifs)
| Colonne | Type | Contraintes | Description |
|---|---|---|---|
| `id` | UUID | PRIMARY KEY | Identifiant de prestation |
| `tenant_id` | UUID | NOT NULL REFERENCES tenants(id) | Établissement |
| `name` | VARCHAR(255) | NOT NULL | Nom (ex: Chemise, Costume 2 pièces) |
| `category` | VARCHAR(100) | NOT NULL | Lavage, Repassage, Nettoyage à sec, Délicat |
| `unit_price` | NUMERIC(12,2)| CHECK >= 0 | Prix unitaire en FCFA (XOF) |
| `unit` | VARCHAR(50) | DEFAULT 'pièce' | Pièce, ensemble, kg, mètre carré |
| `estimated_hours` | INT | DEFAULT 24 | Délai moyen estimé de traitement |

---

### 5. `orders` (Commandes & Dépôts de Linge)
| Colonne | Type | Contraintes | Description |
|---|---|---|---|
| `id` | UUID | PRIMARY KEY | Identifiant unique |
| `tenant_id` | UUID | NOT NULL REFERENCES tenants(id) | Établissement |
| `customer_id` | UUID | NOT NULL REFERENCES customers(id) | Client |
| `order_number` | VARCHAR(50) | NOT NULL | Numéro de ticket séquentiel (ex: CMD-2026-001) |
| `status` | VARCHAR(50) | CHECK (received, washing, ironing, ready, delivered, cancelled) | Statut du linge |
| `payment_status` | VARCHAR(50) | CHECK (unpaid, partially_paid, paid) | Statut financier |
| `total_amount` | NUMERIC(12,2)| CHECK >= 0 | Montant total en FCFA |
| `paid_amount` | NUMERIC(12,2)| CHECK >= 0 | Acompte ou somme versée |
| `due_date` | TIMESTAMPTZ | - | Date/heure promise pour le retrait |

---

### 6. `order_items` (Détail des Vêtements / Articles)
| Colonne | Type | Contraintes | Description |
|---|---|---|---|
| `id` | UUID | PRIMARY KEY | Identifiant |
| `order_id` | UUID | NOT NULL REFERENCES orders(id) ON DELETE CASCADE | Commande parente |
| `service_id` | UUID | NOT NULL REFERENCES services(id) | Prestation appliquée |
| `item_label` | VARCHAR(255)| NOT NULL | Libellé au moment du dépôt |
| `quantity` | INT | NOT NULL CHECK > 0 | Nombre d'articles |
| `unit_price` | NUMERIC(12,2)| NOT NULL | Prix unitaire figé au moment de la vente |
| `subtotal` | NUMERIC(12,2)| NOT NULL | Total de la ligne |

---

### 7. `payments` (Encaissements)
| Colonne | Type | Contraintes | Description |
|---|---|---|---|
| `id` | UUID | PRIMARY KEY | Identifiant de paiement |
| `tenant_id` | UUID | NOT NULL REFERENCES tenants(id) | Établissement |
| `order_id` | UUID | NOT NULL REFERENCES orders(id) | Commande associée |
| `amount` | NUMERIC(12,2)| CHECK > 0 | Somme reçue en FCFA |
| `payment_method` | VARCHAR(50) | CHECK (cash, orange_money, mtn_momo, wave, card) | Moyen de paiement ivoirien |
| `reference` | VARCHAR(100)| - | Référence transaction Mobile Money |

---

## 2. Cycle de Vie des Migrations SQL

Les migrations sont stockées dans `backend/migrations/` :
- `000001_init_schema.up.sql` : applique les créations de tables et index.
- `000001_init_schema.down.sql` : rollback complet en cas de besoin.

Lors du lancement via Docker Compose, le script `.up.sql` est automatiquement exécuté par le conteneur PostgreSQL si la base est vierge (`/docker-entrypoint-initdb.d`).
