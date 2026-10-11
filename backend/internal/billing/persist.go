package billing

import (
	"context"
	"time"

	"pressing-os/backend/internal/database"
)

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

func (c *dbCounters) CountUsers(ctx context.Context, tenantID string) (int64, error) {
	var n int64
	row := c.database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users WHERE tenant_id = $1 AND status = 'active'`, tenantID)
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
	var actor interface{}
	if actorUserID != "" {
		actor = actorUserID
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
