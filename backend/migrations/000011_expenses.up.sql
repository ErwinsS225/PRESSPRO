-- Module Dépenses (fonctionnalité payante « expenses.advanced »)
-- Suivi des charges d'exploitation d'un pressing : matières premières,
-- loyer, énergie, salaires… par établissement et par mois.
CREATE TABLE IF NOT EXISTS expenses (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id       UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    branch_id       UUID REFERENCES branches(id) ON DELETE SET NULL,
    category        VARCHAR(60) NOT NULL,
    label           VARCHAR(255) NOT NULL,
    amount_xof      NUMERIC(12, 2) NOT NULL CHECK (amount_xof > 0),
    payment_method  VARCHAR(50),
    spent_at        DATE NOT NULL DEFAULT CURRENT_DATE,
    note            TEXT,
    created_by      UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Isolation multi-tenant + lecture par période (rapports) et par catégorie.
CREATE INDEX IF NOT EXISTS idx_expenses_tenant_date ON expenses(tenant_id, spent_at DESC);
CREATE INDEX IF NOT EXISTS idx_expenses_tenant_category ON expenses(tenant_id, category);
