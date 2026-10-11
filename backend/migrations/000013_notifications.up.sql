-- ==============================================================================
-- PressingOS — Migration 000013: Journal des notifications clients (UP)
-- ==============================================================================
-- Chaque message adressé à un client (WhatsApp / SMS) est journalisé : le
-- pressing garde la trace de ce qui a été composé et transmis, même lorsque
-- l'envoi est simulé (aucune passerelle configurée par l'éditeur).
-- Statuts : « simule » (message composé, aucun envoi réseau), « envoye »
-- (passerelle sollicitée avec succès), « echec » (passerelle injoignable ou en
-- erreur — le détail technique reste consultable).

CREATE TABLE IF NOT EXISTS notifications (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id     UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    order_id      UUID REFERENCES orders(id) ON DELETE SET NULL,
    channel       VARCHAR(20) NOT NULL,
    template      VARCHAR(40),
    recipient     VARCHAR(40) NOT NULL,
    body          TEXT NOT NULL,
    status        VARCHAR(20) NOT NULL,
    provider      VARCHAR(40),
    detail        TEXT,
    attempts      INTEGER NOT NULL DEFAULT 1,
    created_by    UUID,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at       TIMESTAMPTZ,
    CONSTRAINT chk_notifications_channel CHECK (channel IN ('whatsapp', 'sms')),
    CONSTRAINT chk_notifications_status CHECK (status IN ('simule', 'envoye', 'echec'))
);

-- Journal d'un pressing (page la plus récente en premier) et suivi par commande.
CREATE INDEX IF NOT EXISTS idx_notifications_tenant_created
    ON notifications (tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_order
    ON notifications (order_id);

COMMENT ON TABLE notifications IS 'Journal des messages clients (WhatsApp/SMS) : composition, statut d''envoi et détail technique.';
COMMENT ON COLUMN notifications.status IS 'simule = journalisé sans envoi réseau, envoye = passerelle OK, echec = passerelle en erreur.';
