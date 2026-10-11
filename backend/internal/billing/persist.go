package billing

import (
	"context"
	"strings"
	"time"

	"pressing-os/backend/internal/database"
)

// uuidPourColonne retourne l'identifiant exploitable dans une colonne UUID, ou
// nil. Les comptes de démonstration (identifiants mémoire de forme libre) ne
// peuvent pas alimenter une clé étrangère : ils sont alors consignés dans le
// motif de l'événement plutôt que rejetés par PostgreSQL.
func uuidPourColonne(id string) interface{} {
	if len(id) != 36 {
		return nil
	}
	for i := 0; i < len(id); i++ {
		switch i {
		case 8, 13, 18, 23:
			if id[i] != '-' {
				return nil
			}
		default:
			c := id[i]
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return nil
			}
		}
	}
	return id
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

func (c *dbCounters) CountMonthlyOrders(ctx context.Context, tenantID string) (int64, error) {
	var n int64
	row := c.database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM orders WHERE tenant_id = $1 AND date_trunc('month', created_at) = date_trunc('month', NOW())`,
		tenantID)
	if err := row.Scan(&n); err != nil {
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
		return nil, nil
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
	_, err := s.database.ExecContext(ctx, `
		INSERT INTO subscriptions (tenant_id, plan_id, status, trial_started_at, trial_ends_at)
		VALUES ($1, $2, 'TRIALING', NOW(), $3)
		ON CONFLICT (tenant_id) DO NOTHING`, tenantID, planID, trialEndsAt)
	if err != nil {
		return err
	}
	_, err = s.database.ExecContext(ctx, `
		INSERT INTO subscription_events (tenant_id, to_plan_code, to_status, reason)
		VALUES ($1, 'FREE', 'TRIALING', 'trial_started')`, tenantID)
	return err
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
