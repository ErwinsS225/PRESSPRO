-- ============================================================================
-- PressingOS — Migration 000007: Fondations Multi-Tenant (UP)
-- Additif & idempotent (IF NOT EXISTS + ON CONFLICT DO NOTHING).
-- Sépare tenant (organisation) / branch (établissement) / user (identité),
-- ajoute les memberships et rattache les commandes à un établissement.
-- Aucune donnée existante n'est supprimée (backfill automatique).
-- ============================================================================

-- 0) Traçabilité des migrations appliquées
CREATE TABLE IF NOT EXISTS schema_migrations (
    version    VARCHAR(20) PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 1) Établissements physiques (multi-sites) appartenant à un tenant
CREATE TABLE IF NOT EXISTS branches (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id  UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name       VARCHAR(255) NOT NULL,
    code       VARCHAR(50),
    address    TEXT,
    phone      VARCHAR(50),
    city       VARCHAR(100),
    timezone   VARCHAR(64) NOT NULL DEFAULT 'Africa/Abidjan',
    is_primary BOOLEAN NOT NULL DEFAULT false,
    status     VARCHAR(50) NOT NULL DEFAULT 'active'
               CHECK (status IN ('active', 'suspended', 'archived')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_branch_tenant_name UNIQUE (tenant_id, name)
);
CREATE INDEX IF NOT EXISTS idx_branches_tenant ON branches(tenant_id);

-- 2) Rattachement utilisateur ↔ tenant avec rôle (isolation d'appartenance)
CREATE TABLE IF NOT EXISTS tenant_memberships (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id  UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role       VARCHAR(50) NOT NULL
               CHECK (role IN ('superadmin', 'owner', 'manager', 'cashier', 'operator')),
    status     VARCHAR(50) NOT NULL DEFAULT 'active'
               CHECK (status IN ('active', 'suspended')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_tenant_membership UNIQUE (tenant_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_tm_user ON tenant_memberships(user_id);
CREATE INDEX IF NOT EXISTS idx_tm_tenant ON tenant_memberships(tenant_id);

-- 3) Périmètre d'établissements accordé à un utilisateur
CREATE TABLE IF NOT EXISTS branch_memberships (
    id         UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id  UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id  UUID NOT NULL REFERENCES branches(id) ON DELETE CASCADE,
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_branch_membership UNIQUE (branch_id, user_id)
);
CREATE INDEX IF NOT EXISTS idx_bm_user ON branch_memberships(user_id);
CREATE INDEX IF NOT EXISTS idx_bm_branch ON branch_memberships(branch_id);

-- 4) Les commandes sont rattachées à un établissement (ops par site)
ALTER TABLE orders ADD COLUMN IF NOT EXISTS branch_id UUID REFERENCES branches(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_orders_branch ON orders(branch_id);

-- ============================ BACKFILL (idempotent) ========================
-- Une branche principale par tenant qui n'en possède pas encore.
INSERT INTO branches (tenant_id, name, is_primary, status)
SELECT t.id, 'Établissement principal', true, 'active'
FROM tenants t
WHERE NOT EXISTS (SELECT 1 FROM branches b WHERE b.tenant_id = t.id)
ON CONFLICT DO NOTHING;

-- Un membership par utilisateur rattaché à un tenant (le superadmin global exclu).
INSERT INTO tenant_memberships (tenant_id, user_id, role, status)
SELECT u.tenant_id, u.id, u.role, 'active'
FROM users u
WHERE u.tenant_id IS NOT NULL
  AND NOT EXISTS (
      SELECT 1 FROM tenant_memberships tm
      WHERE tm.user_id = u.id AND tm.tenant_id = u.tenant_id
  )
ON CONFLICT DO NOTHING;

-- Accès par défaut de chaque membre à la branche principale de son tenant.
INSERT INTO branch_memberships (tenant_id, branch_id, user_id)
SELECT tm.tenant_id, b.id, tm.user_id
FROM tenant_memberships tm
JOIN branches b ON b.tenant_id = tm.tenant_id AND b.is_primary = true
ON CONFLICT DO NOTHING;

-- Rattache les commandes existantes à la branche principale de leur tenant.
UPDATE orders o
SET branch_id = b.id
FROM branches b
WHERE o.tenant_id = b.tenant_id AND b.is_primary = true AND o.branch_id IS NULL;

-- Enregistre la version (000001→000007 désormais suivies formellement).
INSERT INTO schema_migrations (version) VALUES
    ('000001'), ('000002'), ('000003'), ('000004'), ('000005'), ('000006'), ('000007')
ON CONFLICT (version) DO NOTHING;
