package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"pressing-os/backend/internal/database"
)

// dbStore exécute les requêtes d'authentification sur PostgreSQL.
type dbStore struct {
	database *database.Store
}

// NewStore construit le store d'authentification persistant.
func NewStore(db *database.Store) *dbStore {
	return &dbStore{database: db}
}

// SetStore branche la persistance PostgreSQL sur un service existant.
// Appelé une seule fois au démarrage (avant de servir les requêtes).
func (s *Service) SetStore(st *dbStore) {
	s.store = st
}

// nullIfEmpty renvoie nil pour une chaîne vide (colonne NULL en base).
func nullIfEmpty(v string) *string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	cp := v
	return &cp
}

// scanUserRow lit une ligne users (colonnes nullables : tenant_id, phone).
func scanUserRow(row pgx.Row) (*User, error) {
	var (
		id, email, passwordHash, fullName, role, status string
		tenantID, phone                                 *string
		createdAt, updatedAt                            time.Time
	)
	if err := row.Scan(&id, &tenantID, &email, &passwordHash, &fullName, &phone, &role, &status, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	u := &User{
		ID: id, Email: email, PasswordHash: passwordHash,
		FullName: fullName, Role: role, Status: status,
		CreatedAt: createdAt, UpdatedAt: updatedAt,
	}
	if tenantID != nil {
		u.TenantID = *tenantID
	}
	if phone != nil {
		u.Phone = *phone
	}
	return u, nil
}

// FindUserByEmail renvoie l'utilisateur par email, ou (nil, nil) si absent.
func (s *dbStore) FindUserByEmail(ctx context.Context, email string) (*User, error) {
	row := s.database.QueryRowContext(ctx, `
		SELECT id, tenant_id, email, password_hash, full_name, phone, role, status, created_at, updated_at
		FROM users WHERE email = $1`, email)
	u, err := scanUserRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

// FindUserByID renvoie l'utilisateur par identifiant, ou (nil, nil) si absent.
func (s *dbStore) FindUserByID(ctx context.Context, userID string) (*User, error) {
	row := s.database.QueryRowContext(ctx, `
		SELECT id, tenant_id, email, password_hash, full_name, phone, role, status, created_at, updated_at
		FROM users WHERE id = $1`, userID)
	u, err := scanUserRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return u, err
}

// FindUsersByTenant renvoie tous les utilisateurs d'un tenant spécifique.
func (s *dbStore) FindUsersByTenant(ctx context.Context, tenantID string) ([]*User, error) {
	rows, err := s.database.QueryContext(ctx, `
		SELECT id, tenant_id, email, password_hash, full_name, phone, role, status, created_at, updated_at
		FROM users WHERE tenant_id = $1 ORDER BY full_name`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		u, err := scanUserRow(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, u)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

// FindTenant renvoie le résumé d'un établissement, ou (nil, nil) si absent.
func (s *dbStore) FindTenant(ctx context.Context, tenantID string) (*TenantSummary, error) {
	var (
		id, name, slug, status string
		city, currency         *string
	)
	row := s.database.QueryRowContext(ctx, `
		SELECT id, name, slug, city, currency, status
		FROM tenants WHERE id = $1`, tenantID)
	if err := row.Scan(&id, &name, &slug, &city, &currency, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	t := &TenantSummary{ID: id, Name: name, Slug: slug, Status: status}
	if city != nil {
		t.City = *city
	}
	if currency != nil {
		t.Currency = *currency
	}
	return t, nil
}

// FindPrimaryBranch renvoie la branche principale d'un tenant, ou "" si absente.
func (s *dbStore) FindPrimaryBranch(ctx context.Context, tenantID string) (string, error) {
	var branchID string
	row := s.database.QueryRowContext(ctx, `
		SELECT id FROM branches WHERE tenant_id = $1 AND is_primary = true LIMIT 1`, tenantID)
	if err := row.Scan(&branchID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}
	return branchID, nil
}

// EmailExists indique si un email est déjà pris (contrainte UNIQUE users.email).
func (s *dbStore) EmailExists(ctx context.Context, email string) (bool, error) {
	var exists bool
	err := s.database.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, email).Scan(&exists)
	return exists, err
}

// RegisterTenant crée, en une seule transaction atomique, la séquence
// complète d'onboarding multi-tenant : tenant → branche principale → compte
// propriétaire → memberships (tenant + branche) → abonnement TRIALING (7 jours).
// En cas d'échec, tout est annulé (aucun tenant orphelin).
func (s *dbStore) RegisterTenant(ctx context.Context, req RegisterTenantRequest) (*User, *TenantSummary, error) {
	normEmail := strings.ToLower(strings.TrimSpace(req.OwnerEmail))

	hash, err := HashPassword(req.Password)
	if err != nil {
		return nil, nil, err
	}

	trialEnds := time.Now().Add(7 * 24 * time.Hour)

	tx, err := s.database.Pool().Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 1) Organisation cliente (tenant)
	slug := uniqueSlug(req.TenantName, time.Now().Unix())
	var tenantID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO tenants (name, slug, phone, city, status)
		VALUES ($1, $2, $3, $4, 'trial') RETURNING id`,
		req.TenantName, slug, nullIfEmpty(req.Phone), nullIfEmpty(req.City),
	).Scan(&tenantID); err != nil {
		return nil, nil, translateRegisterErr(err)
	}

	// 2) Établissement physique principal
	var branchID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO branches (tenant_id, name, is_primary, city, phone, status)
		VALUES ($1, 'Établissement principal', true, $2, $3, 'active') RETURNING id`,
		tenantID, nullIfEmpty(req.City), nullIfEmpty(req.Phone),
	).Scan(&branchID); err != nil {
		return nil, nil, err
	}

	// 3) Compte propriétaire (owner)
	var userID string
	if err := tx.QueryRow(ctx, `
		INSERT INTO users (tenant_id, email, password_hash, full_name, phone, role, status)
		VALUES ($1, $2, $3, $4, $5, 'owner', 'active') RETURNING id`,
		tenantID, normEmail, hash, req.OwnerFullName, nullIfEmpty(req.Phone),
	).Scan(&userID); err != nil {
		return nil, nil, translateRegisterErr(err)
	}

	// 4) Appartenance au tenant (rôle owner)
	if _, err := tx.Exec(ctx, `
		INSERT INTO tenant_memberships (tenant_id, user_id, role, status)
		VALUES ($1, $2, 'owner', 'active')`, tenantID, userID); err != nil {
		return nil, nil, err
	}

	// 5) Périmètre d'accès à l'établissement principal
	if _, err := tx.Exec(ctx, `
		INSERT INTO branch_memberships (tenant_id, branch_id, user_id)
		VALUES ($1, $2, $3)`, tenantID, branchID, userID); err != nil {
		return nil, nil, err
	}

	// 6) Abonnement d'essai (plan FREE, droits BUSINESS via EffectivePlan pendant J7)
	if err := startTrialTx(ctx, tx, tenantID, trialEnds); err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, err
	}

	now := time.Now().UTC()
	user := &User{
		ID: userID, TenantID: tenantID, Email: normEmail, PasswordHash: hash,
		FullName: req.OwnerFullName, Phone: req.Phone, Role: RoleOwner, Status: "active",
		CreatedAt: now, UpdatedAt: now,
	}
	tenant := &TenantSummary{
		ID: tenantID, Name: req.TenantName, Slug: slug,
		City: req.City, Currency: "XOF", Status: "trial",
	}
	return user, tenant, nil
}

// uniqueSlug normalise un nom commercial en slug URL, suffixé pour éviter les collisions.
func uniqueSlug(name string, salt int64) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	prevDash := false
	for _, r := range lower {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		slug = "pressing"
	}
	return fmt.Sprintf("%s-%d", slug, salt)
}

// startTrialTx démarre l'abonnement d'essai (plan FREE, statut TRIALING) + journalise l'événement.
func startTrialTx(ctx context.Context, tx pgx.Tx, tenantID string, trialEndsAt time.Time) error {
	var planID string
	if err := tx.QueryRow(ctx, `SELECT id FROM plans WHERE code = 'FREE'`).Scan(&planID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO subscriptions (tenant_id, plan_id, status, trial_started_at, trial_ends_at)
		VALUES ($1, $2, 'TRIALING', NOW(), $3)`, tenantID, planID, trialEndsAt); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO subscription_events (tenant_id, to_plan_code, to_status, reason)
		VALUES ($1, 'FREE', 'TRIALING', 'trial_started')`, tenantID)
	return err
}

// translateRegisterErr convertit une violation d'unicité PostgreSQL en erreur métier explicite.
func translateRegisterErr(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch {
		case strings.Contains(pgErr.ConstraintName, "users_email"), strings.Contains(pgErr.ConstraintName, "email"):
			return ErrEmailAlreadyExists
		case strings.Contains(pgErr.ConstraintName, "slug"):
			return fmt.Errorf("ce nom d'établissement est déjà utilisé")
		}
	}
	return err
}
