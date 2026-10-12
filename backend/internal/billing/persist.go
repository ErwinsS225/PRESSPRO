package billing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"pressing-os/backend/internal/database"
	"pressing-os/backend/internal/pgutil"
)

// uuidPourColonne retourne l'identifiant exploitable dans une colonne UUID, ou
// nil. Les comptes de démonstration (identifiants mémoire de forme libre) ne
// peuvent pas alimenter une clé étrangère : ils sont alors consignés dans le
// motif de l'événement plutôt que rejetés par PostgreSQL.
func uuidPourColonne(id string) interface{} { return pgutil.NullableUUID(id) }

// DeleteTenant retire l'établissement et ses données. Le schéma supprime en
// cascade les comptes, clients, catalogue, dépenses, abonnements et notifications ;
// seuls les paiements et les commandes référencent l'établissement sans cascade,
// ils sont donc retirés d'abord, dans la même transaction : aucune ligne orpheline
// ne peut subsister, même si une étape échoue.
func (s *dbStore) DeleteTenant(ctx context.Context, tenantID string) error {
	if !pgutil.IsUUID(tenantID) {
		return ErrTenantNotFound
	}

	tx, err := s.database.Pool().Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, requete := range []string{
		`DELETE FROM payments WHERE tenant_id = $1`,
		`DELETE FROM orders WHERE tenant_id = $1`,
	} {
		if _, err := tx.Exec(ctx, requete, tenantID); err != nil {
			return err
		}
	}

	etiquette, err := tx.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenantID)
	if err != nil {
		return err
	}
	if etiquette.RowsAffected() == 0 {
		return ErrTenantNotFound
	}

	return tx.Commit(ctx)
}

// dbStore exécute les requêtes d'abonnement sur PostgreSQL.
type dbStore struct {
	database *database.Store
}

// NewStore construit un moteur adossé à PostgreSQL.
func NewStore(db *database.Store) *Service {
	s := NewService()
	s.store = &dbStore{database: db}
	s.counters = &dbCounters{database: db}
	return s
}

// NewDBUsageCounter expose les compteurs SQL (quotas mensuels réels).
func NewDBUsageCounter(db *database.Store) UsageCounter {
	return &dbCounters{database: db}
}

// dbCounters compte les usages réels (quotas mensuels).
type dbCounters struct {
	database *database.Store
}

// requeteFenetre : ancre de la fenêtre glissante de commandes. Priorité au début
// d'essai, sinon à la création de l'abonnement, sinon à la création de
// l'établissement (tenants.created_at est NOT NULL, donc toujours défini).
// La fenêtre est ouverte par tranches de OrdersPeriodDays jours depuis cette
// ancre : elle se renouvelle sans job, et un changement de formule ne la remet
// pas à zéro (l'ancre ne dépend pas du plan).
const requeteFenetre = `
	SELECT COALESCE(s.trial_started_at, s.created_at, t.created_at) AS ancre
	FROM tenants t
	LEFT JOIN subscriptions s ON s.tenant_id = t.id
	WHERE t.id = $1`

// requeteCompteFenetre compte les commandes de la fenêtre courante, hors annulées.
// $1 = tenant, $2 = instant de calcul. L'index idx_orders_tenant_created (migration
// 000016) couvre le filtre par établissement et par date.
var requeteCompteFenetre = fmt.Sprintf(`
	WITH ancre AS (%s),
	bornes AS (
		SELECT ancre.ancre + make_interval(days => %d * GREATEST(0,
			FLOOR(EXTRACT(EPOCH FROM ($2::timestamptz - ancre.ancre)) / (%d * 86400))::int)) AS debut
		FROM ancre
	)
	SELECT COUNT(*) FROM orders o, bornes
	WHERE o.tenant_id = $1
	  AND o.status <> 'cancelled'
	  AND o.created_at >= bornes.debut
	  AND o.created_at < bornes.debut + make_interval(days => %d)`,
	requeteFenetre, OrdersPeriodDays, OrdersPeriodDays, OrdersPeriodDays)

// CountOrdersInPeriod compte la consommation de la fenêtre courante (hors transaction).
func (c *dbCounters) CountOrdersInPeriod(ctx context.Context, tenantID string, now time.Time) (int64, error) {
	if !pgutil.IsUUID(tenantID) {
		return 0, nil
	}
	var n int64
	if err := c.database.QueryRowContext(ctx, requeteCompteFenetre, tenantID, now).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// CountOrdersInPeriodTx compte dans la transaction de création, après le verrou
// de l'établissement : la lecture voit alors les commandes déjà validées.
func (c *dbCounters) CountOrdersInPeriodTx(ctx context.Context, tx pgx.Tx, tenantID string, now time.Time) (int64, error) {
	if !pgutil.IsUUID(tenantID) {
		return 0, nil
	}
	var n int64
	if err := tx.QueryRow(ctx, requeteCompteFenetre, tenantID, now).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (c *dbCounters) CountPriceLists(ctx context.Context, tenantID string) (int64, error) {
	var n int64
	row := c.database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM price_lists WHERE tenant_id = $1`, tenantID)
	if err := row.Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (c *dbCounters) CountUsers(ctx context.Context, tenantID string) (int64, error) {
	var n int64
	row := c.database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE tenant_id = $1 AND status = 'active'`, tenantID)
	if err := row.Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (c *dbCounters) CountBranches(ctx context.Context, tenantID string) (int64, error) {
	var n int64
	row := c.database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM branches WHERE tenant_id = $1 AND status <> 'archived'`, tenantID)
	if err := row.Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (s *dbStore) GetSubscription(ctx context.Context, tenantID string) (*SubscriptionRecord, error) {
	var (
		planCode, status, planName string
		price                      float64
		trialStart, trialEnd       *time.Time
		periodEnd                  *time.Time
	)
	row := s.database.QueryRowContext(ctx, `
		SELECT p.code, sub.status, p.name, p.price_monthly_xof,
		       sub.trial_started_at, sub.trial_ends_at, sub.current_period_end
		FROM subscriptions sub JOIN plans p ON p.id = sub.plan_id
		WHERE sub.tenant_id = $1`, tenantID)
	if err := row.Scan(&planCode, &status, &planName, &price, &trialStart, &trialEnd, &periodEnd); err != nil {
		// Absence d'abonnement : état normal (formule FREE par défaut).
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		// Toute autre erreur est une panne de lecture : jamais un état FREE silencieux.
		return nil, fmt.Errorf("lecture de l'abonnement: %w", err)
	}
	return &SubscriptionRecord{
		PlanCode: planCode, Status: status,
		TrialStartedAt: trialStart, TrialEndsAt: trialEnd,
		PlanName: planName, PriceMonthly: price,
	}, nil
}

func (s *dbStore) ListEntitlements(ctx context.Context, planCode string) ([]Entitlement, error) {
	rows, err := s.database.QueryContext(ctx, `
		SELECT f.key, pe.is_enabled, pe.limit_value
		FROM plan_entitlements pe
		JOIN plans p ON p.id = pe.plan_id
		JOIN features f ON f.id = pe.feature_id
		WHERE p.code = $1`, planCode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entitlement
	for rows.Next() {
		var key string
		var enabled bool
		var limit *int64
		if err := rows.Scan(&key, &enabled, &limit); err != nil {
			return nil, err
		}
		out = append(out, Entitlement{Key: key, Enabled: enabled, Limit: limit, Source: "plan"})
	}
	return out, nil
}

func (s *dbStore) SetPlan(ctx context.Context, tenantID, planCode, status, actorUserID, reason string) error {
	var planID string
	row := s.database.QueryRowContext(ctx, `SELECT id FROM plans WHERE code = $1`, planCode)
	if err := row.Scan(&planID); err != nil {
		return err
	}
	var fromPlan, fromStatus *string
	r := s.database.QueryRowContext(ctx, `
		SELECT p.code, sub.status FROM subscriptions sub
		JOIN plans p ON p.id = sub.plan_id WHERE sub.tenant_id = $1`, tenantID)
	_ = r.Scan(&fromPlan, &fromStatus)
	_, err := s.database.ExecContext(ctx, `
		INSERT INTO subscriptions (tenant_id, plan_id, status)
		VALUES ($1, $2, $3)
		ON CONFLICT (tenant_id) DO UPDATE SET plan_id = $2, status = $3, updated_at = NOW()`,
		tenantID, planID, status)
	if err != nil {
		return err
	}
	actor := uuidPourColonne(actorUserID)
	if actor == nil && actorUserID != "" {
		reason = reason + " (acteur: " + actorUserID + ")"
	}
	_, err = s.database.ExecContext(ctx, `
		INSERT INTO subscription_events (tenant_id, from_plan_code, to_plan_code, from_status, to_status, reason, actor_user_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		tenantID, fromPlan, planCode, fromStatus, status, reason, actor)
	return err
}

func (s *dbStore) StartTrial(ctx context.Context, tenantID string, trialEndsAt time.Time) error {
	var planID string
	row := s.database.QueryRowContext(ctx, `SELECT id FROM plans WHERE code = 'FREE'`)
	if err := row.Scan(&planID); err != nil {
		return err
	}
	// Une seule instruction : l'événement n'est journalisé que si l'abonnement a
	// réellement été créé. Une seconde initialisation (conflit sur tenant_id)
	// ne crée ni essai, ni événement en double.
	_, err := s.database.ExecContext(ctx, `
		WITH cree AS (
			INSERT INTO subscriptions (tenant_id, plan_id, status, trial_started_at, trial_ends_at)
			VALUES ($1, $2, 'TRIALING', NOW(), $3)
			ON CONFLICT (tenant_id) DO NOTHING
			RETURNING tenant_id
		)
		INSERT INTO subscription_events (tenant_id, to_plan_code, to_status, reason)
		SELECT tenant_id, 'FREE', 'TRIALING', 'trial_started' FROM cree`,
		tenantID, planID, trialEndsAt)
	return err
}

// ExpireTrial applique la transition d'expiration dans une transaction :
// verrouillage de la ligne d'abonnement, contrôle de la date et du statut sous
// verrou, puis mise à jour et événement. Deux appels concurrents ne peuvent donc
// produire qu'une seule transition ; un abonnement payé ou déjà actif n'est jamais touché.
func (s *dbStore) ExpireTrial(ctx context.Context, tenantID string, now time.Time, actorUserID string) (bool, error) {
	tx, err := s.database.Pool().Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var fromPlan, fromStatus string
	var trialEnd *time.Time
	err = tx.QueryRow(ctx, `
		SELECT p.code, sub.status, sub.trial_ends_at
		FROM subscriptions sub JOIN plans p ON p.id = sub.plan_id
		WHERE sub.tenant_id = $1
		FOR UPDATE OF sub`, tenantID).Scan(&fromPlan, &fromStatus, &trialEnd)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("verrouillage de l'abonnement: %w", err)
	}
	if fromStatus != StatusTrialing || trialEnd == nil || now.Before(*trialEnd) {
		return false, nil
	}

	if _, err := tx.Exec(ctx, `
		UPDATE subscriptions
		SET plan_id = (SELECT id FROM plans WHERE code = $2), status = $3, updated_at = NOW()
		WHERE tenant_id = $1`, tenantID, PlanFree, StatusActive); err != nil {
		return false, fmt.Errorf("transition de l'essai: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO subscription_events (tenant_id, from_plan_code, to_plan_code, from_status, to_status, reason, actor_user_id)
		VALUES ($1, $2, $3, $4, $5, 'trial_expired', $6)`,
		tenantID, fromPlan, PlanFree, fromStatus, StatusActive, uuidPourColonne(actorUserID)); err != nil {
		return false, fmt.Errorf("journal de l'expiration: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// ---------- Espace SuperAdmin : indicateurs et annuaire réels ----------

// PlatformStats agrège l'activité de la plateforme à partir des données réelles.
func (s *dbStore) PlatformStats(ctx context.Context) (*PlatformStats, error) {
	stats := &PlatformStats{ByStatus: map[string]int64{}, ByPlan: map[string]int64{}}

	row := s.database.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM tenants),
			(SELECT COUNT(*) FROM users WHERE status = 'active'),
			(SELECT COUNT(*) FROM subscriptions WHERE status = 'ACTIVE'),
			(SELECT COUNT(*) FROM subscriptions WHERE status = 'TRIALING'),
			COALESCE((
				SELECT SUM(p.price_monthly_xof)
				FROM subscriptions sub JOIN plans p ON p.id = sub.plan_id
				WHERE sub.status = 'ACTIVE'
			), 0)`)
	if err := row.Scan(&stats.TotalPressings, &stats.TotalUsers,
		&stats.ActiveSubscriptions, &stats.TrialingCount, &stats.MRR); err != nil {
		return nil, err
	}

	rows, err := s.database.QueryContext(ctx,
		`SELECT status, COUNT(*) FROM subscriptions GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var n int64
		if err := rows.Scan(&status, &n); err != nil {
			return nil, err
		}
		stats.ByStatus[status] = n
	}

	planRows, err := s.database.QueryContext(ctx, `
		SELECT p.code, COUNT(*)
		FROM subscriptions sub JOIN plans p ON p.id = sub.plan_id
		GROUP BY p.code`)
	if err != nil {
		return nil, err
	}
	defer planRows.Close()
	for planRows.Next() {
		var code string
		var n int64
		if err := planRows.Scan(&code, &n); err != nil {
			return nil, err
		}
		stats.ByPlan[code] = n
	}
	return stats, rows.Err()
}

// ListTenantOverviews retourne l'annuaire des pressings, abonnement et compte
// propriétaire compris (aucune donnée métier interne : ni clients ni commandes).
func (s *dbStore) ListTenantOverviews(ctx context.Context) ([]TenantOverview, error) {
	rows, err := s.database.QueryContext(ctx, `
		SELECT t.id, t.name, t.slug, COALESCE(t.city, ''), COALESCE(t.phone, ''),
		       COALESCE(owner.email, ''),
		       COALESCE(p.code, 'FREE'), COALESCE(p.name, 'Gratuit'),
		       COALESCE(p.price_monthly_xof, 0),
		       COALESCE(sub.status, 'ACTIVE'), sub.trial_ends_at, t.created_at
		FROM tenants t
		LEFT JOIN subscriptions sub ON sub.tenant_id = t.id
		LEFT JOIN plans p ON p.id = sub.plan_id
		LEFT JOIN LATERAL (
			SELECT u.email FROM users u
			WHERE u.tenant_id = t.id AND u.role = 'owner'
			ORDER BY u.created_at LIMIT 1
		) owner ON true
		ORDER BY t.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]TenantOverview, 0)
	for rows.Next() {
		var o TenantOverview
		var status string
		if err := rows.Scan(&o.TenantID, &o.Name, &o.Slug, &o.City, &o.Phone,
			&o.OwnerEmail, &o.PlanCode, &o.PlanName, &o.PriceMonthlyXOF,
			&status, &o.TrialEndsAt, &o.CreatedAt); err != nil {
			return nil, err
		}
		o.Status = status
		o.Actif = !strings.EqualFold(status, StatusSuspended)
		out = append(out, o)
	}
	return out, rows.Err()
}

// SetSubscriptionStatus change le statut d'abonnement et journalise l'événement
// dans la MÊME transaction : une décision de l'éditeur n'est jamais appliquée à
// moitié (statut modifié sans trace, ou trace sans effet).
func (s *dbStore) SetSubscriptionStatus(ctx context.Context, tenantID, status, actorUserID, reason string) error {
	tx, err := s.database.Pool().Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var fromStatus, planCode *string
	row := tx.QueryRow(ctx, `
		SELECT sub.status, p.code FROM subscriptions sub
		JOIN plans p ON p.id = sub.plan_id WHERE sub.tenant_id = $1`, tenantID)
	if err := row.Scan(&fromStatus, &planCode); err != nil {
		return ErrTenantSubscriptionNotFound
	}

	if _, err := tx.Exec(ctx, `
		UPDATE subscriptions SET status = $2, updated_at = NOW() WHERE tenant_id = $1`,
		tenantID, status); err != nil {
		return err
	}

	if reason == "" {
		reason = "changement de statut par l'éditeur"
	}
	actor := uuidPourColonne(actorUserID)
	if actor == nil && actorUserID != "" {
		reason = reason + " (acteur: " + actorUserID + ")"
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO subscription_events (tenant_id, from_plan_code, to_plan_code, from_status, to_status, reason, actor_user_id)
		VALUES ($1, $2, $2, $3, $4, $5, $6)`,
		tenantID, planCode, fromStatus, status, reason, actor); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
