# PressingOS — Modèle de Base de Données et Migrations

Ce document détaille le modèle relationnel PostgreSQL 16 retenu pour **PressingOS**, les choix d'indexation, les contraintes d'intégrité et la gestion des migrations SQL.

---

## 1. Vue d'Ensemble des Tables

Le schéma utilise des identifiants **UUID v4** pour garantir l'unicité globale et simplifier les futures intégrations hors-ligne ou multi-serveurs sans collision d'identifiants séquentiels.

### 1. `tenants` (Établissements / Pressings)
| Colonne | Type | Contraintes | Description |
|---|---|---|---|
| `id` | UUID | PRIMARY KEY | Identifiant unique de l'établissement |
| `name` | VARCHAR(255) | NOT NULL | Nom commercial (ex: Pressing de la Lagune) |
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

> **Depuis la migration 000018** : téléphone obligatoire, mais plusieurs personnes peuvent partager un numéro dans un même pressing. Un index non unique `(tenant_id, phone)` remplace l’unicité historique. L’identité repose sur `id` ; aucun upsert/fusion par téléphone. La clé unique `(tenant_id, id)` permet une FK composite depuis `orders(tenant_id, customer_id)`, en complément des FK simples. La migration valide les données existantes et échoue sans les modifier si elles sont incohérentes. Son down refuse les numéros partagés au lieu de supprimer des clients. Déployer avec le nouveau backend, jamais avec l’ancien upsert. Voir [le rapport du lot clients](depot-lot1-clients.md).

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

### 8. `notifications` (Journal des messages clients — session 6)
| Colonne | Type | Contraintes | Description |
|---|---|---|---|
| `id` | UUID | PRIMARY KEY, `DEFAULT uuid_generate_v4()` | Identifiant du message |
| `tenant_id` | UUID | NOT NULL REFERENCES tenants(id) | Établissement émetteur |
| `order_id` | UUID | NULL REFERENCES orders(id) ON DELETE SET NULL | Commande concernée (facultative) |
| `channel` | VARCHAR(20) | NOT NULL, CHECK (`whatsapp`, `sms`) | Canal d'envoi |
| `template` | VARCHAR(40) | - | Modèle appliqué (`commande_prete`, `commande_livree`, `paiement_recu`, `rappel_retrait`) — NULL pour un message libre |
| `recipient` | VARCHAR(40) | NOT NULL | Numéro destinataire (téléphone du client par défaut) |
| `body` | TEXT | NOT NULL | Message tel qu'envoyé (composé ou personnalisé) |
| `status` | VARCHAR(20) | NOT NULL, CHECK (`simule`, `envoye`, `echec`) | `envoye` = transmis à la passerelle ; `simule` = journalisé sans passerelle ; `echec` = passerelle en erreur |
| `provider` | VARCHAR(40) | - | `passerelle` ou vide (journal local) |
| `detail` | TEXT | - | Motif d'échec renvoyé par la passerelle |
| `attempts` | INTEGER | NOT NULL DEFAULT 1 | Nombre de tentatives d'envoi |
| `created_by` | UUID | NULL REFERENCES users(id) | Auteur humain ; NULL pour un envoi automatique (`ready`/`delivered`) |
| `created_at` / `sent_at` | TIMESTAMPTZ | NOT NULL / NULL | Création et envoi effectif |

Index : `(tenant_id, created_at DESC)` pour le journal, `(order_id)` pour l'historique par
commande. Une commande supprimée n'efface pas le journal (`ON DELETE SET NULL`), une
commande d'un autre pressing est refusée en amont (`404`).

---

## 2. Cycle de Vie des Migrations SQL

Les migrations sont stockées dans `backend/migrations/` :
- `000001_init_schema.up.sql` : applique les créations de tables et index.
- `000001_init_schema.down.sql` : rollback complet en cas de besoin.
- `000013_notifications.up.sql` / `000014_notifications_paid_matrix.up.sql` : journal des
  notifications clients puis droit `notifications.advanced` (faux en gratuit, vrai en PRO et
  BUSINESS), avec leurs `down.sql` respectifs — la matrice par défaut de
  `billing.defaultMatrix()` doit rester alignée sur ces entrées.
- `000015_suppression_entreprise_demo.up.sql` : annule le seed de la société fictive livrée par
  `000001` (établissement `a0000000-…-0001` et ses 5 prestations). Les migrations restant
  immuables, le seed n'est pas édité : il est annulé après coup, si bien qu'une base vierge
  finit avec **0 établissement**. Les commandes/paiements sont retirés avant l'établissement
  (`orders` et `payments` référencent `tenants` **sans** `ON DELETE CASCADE`) ; le fichier
  `down.sql` repose la société pour rejouer une démonstration.

Lors du lancement via Docker Compose, le script `.up.sql` est automatiquement exécuté par le conteneur PostgreSQL si la base est vierge (`/docker-entrypoint-initdb.d`).


---

## 7. Migration 000019 — Finances, Monnaie et Idempotence

- Table `payments` : ajout des colonnes `amount_received NUMERIC(12, 2)` et `change_given NUMERIC(12, 2)`.
  - Intégrité vérifiée : `amount_received >= amount` et `change_given >= 0`.
  - Traçabilité complète du mode d'encaissement et de l'opérateur de caisse (`received_by`).
- Table `order_mutations` :
  - Clé composite unique `(tenant_id, operation, mutation_key)`.
  - Stockage de l'empreinte `request_hash` (SHA-256) et de la réponse sérialisée JSONB.
  - Garantit l'idempotence des dépôts et des règlements de caisse, évitant tout encaissement ou ticket en double lors des micro-coupures réseau.
- Rétro-compatibilité : les commandes et paiements historiques sans monnaie conservent des colonnes `NULL` valides.
