-- =============================================================================
-- PressingOS — Migration 000010: Grilles tarifaires versionnées (UP)
-- Spec MULTI tenant §3/§4 : price_lists (grille au niveau tenant, portée
-- facultative par établissement) + price_list_versions (historique immuable
-- des prix sous forme de snapshot JSONB figé à chaque version).
-- Additive et idempotente : aucun objet existant n'est modifié.
-- =============================================================================

CREATE TABLE IF NOT EXISTS price_lists (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    -- Portée facultative par établissement (prix communs par défaut, override possible)
    branch_id UUID REFERENCES branches(id) ON DELETE SET NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    is_default BOOLEAN NOT NULL DEFAULT false,
    status VARCHAR(50) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Idempotence du backfill (une seule "Grille principale" par tenant)
CREATE UNIQUE INDEX IF NOT EXISTS uq_tenant_pricelist_name
    ON price_lists(tenant_id, name);
CREATE INDEX IF NOT EXISTS idx_price_lists_tenant ON price_lists(tenant_id);
CREATE INDEX IF NOT EXISTS idx_price_lists_branch ON price_lists(branch_id);

CREATE TABLE IF NOT EXISTS price_list_versions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    price_list_id UUID NOT NULL REFERENCES price_lists(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    version INT NOT NULL DEFAULT 1,
    status VARCHAR(50) NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'active', 'superseded')),
    -- Snapshot figé : [{service_id, name, article, treatment, category,
    --                   unit_price, unit, pricing_model, is_active}]
    snapshot JSONB NOT NULL DEFAULT '[]'::jsonb,
    note TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_price_list_version UNIQUE (price_list_id, version)
);
CREATE INDEX IF NOT EXISTS idx_plv_tenant ON price_list_versions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_plv_list ON price_list_versions(price_list_id);

-- ---------------------------------------------------------------------------
-- Backfill : une "Grille principale" par tenant existant (aucun tenant ne
-- doit se retrouver sans grille).
-- ---------------------------------------------------------------------------
INSERT INTO price_lists (tenant_id, name, description, is_default)
SELECT t.id, 'Grille principale', 'Grille tarifaire initiale du catalogue', true
FROM tenants t
WHERE NOT EXISTS (
    SELECT 1 FROM price_lists pl
    WHERE pl.tenant_id = t.id AND pl.name = 'Grille principale'
);

-- Version 1 "active" : snapshot du catalogue de prestations du tenant.
INSERT INTO price_list_versions (price_list_id, tenant_id, version, status, snapshot, note)
SELECT
    pl.id,
    pl.tenant_id,
    1,
    'active',
    COALESCE((
        SELECT jsonb_agg(jsonb_build_object(
            'service_id',   s.id,
            'name',         s.name,
            'article',      s.article,
            'treatment',    s.treatment,
            'category',     s.category,
            'unit_price',   s.unit_price,
            'unit',         s.unit,
            'pricing_model', s.pricing_model,
            'is_active',    s.is_active
        ) ORDER BY s.name)
        FROM services s
        WHERE s.tenant_id = pl.tenant_id
    ), '[]'::jsonb),
    'Version initiale générée automatiquement à partir du catalogue'
FROM price_lists pl
WHERE pl.is_default = true
AND NOT EXISTS (
    SELECT 1 FROM price_list_versions v WHERE v.price_list_id = pl.id
);