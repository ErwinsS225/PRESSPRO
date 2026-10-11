-- ==============================================================================
-- PressingOS — Migration 000001: Initial Multi-Tenant Schema (UP)
-- ==============================================================================

-- Extension pour la génération des UUID natifs
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Table des établissements / pressings (Multi-Tenant)
CREATE TABLE IF NOT EXISTS tenants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(100) NOT NULL UNIQUE,
    legal_name VARCHAR(255),
    phone VARCHAR(50),
    email VARCHAR(255),
    address TEXT,
    city VARCHAR(100) DEFAULT 'Abidjan',
    country VARCHAR(100) DEFAULT 'Côte d''Ivoire',
    currency VARCHAR(10) DEFAULT 'XOF',
    status VARCHAR(50) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'trial', 'pending')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Table des utilisateurs (reliés à un tenant)
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID REFERENCES tenants(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(255) NOT NULL,
    phone VARCHAR(50),
    role VARCHAR(50) NOT NULL CHECK (role IN ('superadmin', 'owner', 'manager', 'cashier', 'operator')),
    status VARCHAR(50) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'locked')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Table des clients du pressing (isolés par tenant_id)
CREATE TABLE IF NOT EXISTS customers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    first_name VARCHAR(100) NOT NULL,
    last_name VARCHAR(100) NOT NULL,
    phone VARCHAR(50) NOT NULL,
    email VARCHAR(255),
    address TEXT,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_tenant_customer_phone UNIQUE (tenant_id, phone)
);

-- Table du catalogue des prestations & tarifs par pressing
CREATE TABLE IF NOT EXISTS services (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    category VARCHAR(100) NOT NULL DEFAULT 'Lavage & Repassage',
    unit_price NUMERIC(12, 2) NOT NULL CHECK (unit_price >= 0),
    unit VARCHAR(50) NOT NULL DEFAULT 'pièce',
    estimated_hours INT DEFAULT 24,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Table des commandes de pressing
CREATE TABLE IF NOT EXISTS orders (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE RESTRICT,
    order_number VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'received' CHECK (status IN ('received', 'washing', 'ironing', 'ready', 'delivered', 'cancelled')),
    payment_status VARCHAR(50) NOT NULL DEFAULT 'unpaid' CHECK (payment_status IN ('unpaid', 'partially_paid', 'paid')),
    total_amount NUMERIC(12, 2) NOT NULL DEFAULT 0.00 CHECK (total_amount >= 0),
    paid_amount NUMERIC(12, 2) NOT NULL DEFAULT 0.00 CHECK (paid_amount >= 0),
    due_date TIMESTAMPTZ,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_tenant_order_number UNIQUE (tenant_id, order_number)
);

-- Lignes d'articles d'une commande
CREATE TABLE IF NOT EXISTS order_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    service_id UUID NOT NULL REFERENCES services(id) ON DELETE RESTRICT,
    item_label VARCHAR(255) NOT NULL,
    quantity INT NOT NULL DEFAULT 1 CHECK (quantity > 0),
    unit_price NUMERIC(12, 2) NOT NULL CHECK (unit_price >= 0),
    subtotal NUMERIC(12, 2) NOT NULL CHECK (subtotal >= 0),
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Table des paiements encaissés
CREATE TABLE IF NOT EXISTS payments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE RESTRICT,
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    amount NUMERIC(12, 2) NOT NULL CHECK (amount > 0),
    payment_method VARCHAR(50) NOT NULL DEFAULT 'cash' CHECK (payment_method IN ('cash', 'orange_money', 'mtn_momo', 'wave', 'card')),
    reference VARCHAR(100),
    received_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index d'isolation et d'optimisation multi-tenant
CREATE INDEX IF NOT EXISTS idx_users_tenant ON users(tenant_id);
CREATE INDEX IF NOT EXISTS idx_customers_tenant ON customers(tenant_id);
CREATE INDEX IF NOT EXISTS idx_services_tenant ON services(tenant_id);
CREATE INDEX IF NOT EXISTS idx_orders_tenant_status ON orders(tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_orders_customer ON orders(customer_id);
CREATE INDEX IF NOT EXISTS idx_order_items_order ON order_items(order_id);
CREATE INDEX IF NOT EXISTS idx_payments_tenant_order ON payments(tenant_id, order_id);

-- Données de démonstration initiales (Seed)
INSERT INTO tenants (id, name, slug, phone, email, address, city, currency, status)
VALUES (
    'a0000000-0000-0000-0000-000000000001',
    'Pressing Ivoire Cocody',
    'pressing-ivoire-cocody',
    '+225 07 00 11 22 33',
    'contact@pressing-ivoire.ci',
    'Boulevard Hassan II, Cocody Angré 8ème Tranche',
    'Abidjan',
    'XOF',
    'active'
) ON CONFLICT (id) DO NOTHING;

INSERT INTO services (tenant_id, name, category, unit_price, unit, estimated_hours)
VALUES
    ('a0000000-0000-0000-0000-000000000001', 'Chemise (Lavage + Repassage)', 'Lavage & Repassage', 1000.00, 'pièce', 24),
    ('a0000000-0000-0000-0000-000000000001', 'Pantalon costume', 'Nettoyage à sec', 1500.00, 'pièce', 24),
    ('a0000000-0000-0000-0000-000000000001', 'Costume 2 pièces (Veste + Pantalon)', 'Nettoyage à sec', 3500.00, 'ensemble', 48),
    ('a0000000-0000-0000-0000-000000000001', 'Robe de soirée / Cérémonie', 'Délicat', 4000.00, 'pièce', 48),
    ('a0000000-0000-0000-0000-000000000001', 'Drap 2 places / Couette', 'Linge de maison', 2500.00, 'pièce', 48)
ON CONFLICT DO NOTHING;
