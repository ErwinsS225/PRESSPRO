package branch

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"pressing-os/backend/internal/database"
)

// dbStore exécute les requêtes multi-sites contre PostgreSQL.
type dbStore struct {
	database *database.Store
}

const branchColumns = `b.id, b.tenant_id, b.name, COALESCE(b.code, ''), COALESCE(b.address, ''),
	COALESCE(b.phone, ''), COALESCE(b.city, ''), b.timezone, b.is_primary, b.status,
	(SELECT COUNT(*) FROM orders o WHERE o.branch_id = b.id), b.created_at, b.updated_at`

func scanBranch(row pgx.Row) (*Branch, error) {
	var b Branch
	err := row.Scan(&b.ID, &b.TenantID, &b.Name, &b.Code, &b.Address, &b.Phone, &b.City,
		&b.Timezone, &b.IsPrimary, &b.Status, &b.OrdersCount, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// traduireErreurBranche transforme les violations SQL en erreurs métier.
func traduireErreurBranche(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrNameExists
	}
	return err
}

// GetBranches liste les établissements du pressing (principal en tête, puis
// les plus récents).
func (s *dbStore) GetBranches(ctx context.Context, tenantID string) ([]*Branch, error) {
	rows, err := s.database.QueryContext(ctx, `
		SELECT `+branchColumns+`
		FROM branches b
		WHERE b.tenant_id = $1 AND b.status <> 'archived'
		ORDER BY b.is_primary DESC, b.created_at ASC`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Branch
	for rows.Next() {
		b, err := scanBranch(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// GetBranch retourne un établissement du pressing (isolation par tenant_id).
func (s *dbStore) GetBranch(ctx context.Context, tenantID, id string) (*Branch, error) {
	row := s.database.QueryRowContext(ctx, `
		SELECT `+branchColumns+`
		FROM branches b
		WHERE b.tenant_id = $1 AND b.id = $2`, tenantID, id)
	b, err := scanBranch(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrBranchNotFound
		}
		return nil, err
	}
	return b, nil
}

// CreateBranch insère un établissement secondaire (jamais principal : un seul
// site principal par pressing, contrainte métier appliquée en amont).
func (s *dbStore) CreateBranch(ctx context.Context, tenantID string, req CreateBranchRequest) (*Branch, error) {
	// Unicité métier : le nom identifie un site pour les équipes, la casse et
	// les espaces ne doivent pas permettre de créer un doublon.
	var existe bool
	if err := s.database.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM branches
			WHERE tenant_id = $1 AND lower(btrim(name)) = lower($2) AND status <> 'archived'
		)`, tenantID, req.Name).Scan(&existe); err != nil {
		return nil, err
	}
	if existe {
		return nil, ErrNameExists
	}

	var id string
	err := s.database.QueryRowContext(ctx, `
		INSERT INTO branches (tenant_id, name, code, address, phone, city, is_primary, status)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''), false, 'active')
		RETURNING id`,
		tenantID, req.Name, req.Code, req.Address, req.Phone, req.City).Scan(&id)
	if err != nil {
		return nil, traduireErreurBranche(err)
	}

	// Périmètre : le site créé est rattaché à tous les membres actifs du
	// pressing. Sans cette appartenance de branche, le contrôle de périmètre
	// (RequireBranchScope) refuserait à un caissier d'y enregistrer un dépôt —
	// seul le propriétaire, qui couvre par nature tous ses sites, passerait.
	if _, err := s.database.ExecContext(ctx, `
		INSERT INTO branch_memberships (tenant_id, branch_id, user_id)
		SELECT tm.tenant_id, $2, tm.user_id
		FROM tenant_memberships tm
		WHERE tm.tenant_id = $1 AND tm.status = 'active'
		ON CONFLICT (branch_id, user_id) DO NOTHING`, tenantID, id); err != nil {
		return nil, err
	}

	return s.GetBranch(ctx, tenantID, id)
}

// UpdateBranch applique les champs fournis. Le statut de l'établissement
// principal est verrouillé (point d'ancrage des commandes et appartenances).
func (s *dbStore) UpdateBranch(ctx context.Context, tenantID, id string, req UpdateBranchRequest) (*Branch, error) {
	actuel, err := s.GetBranch(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if actuel.IsPrimary && req.Status != nil && *req.Status != StatusActive {
		return nil, ErrPrimaryProtected
	}

	nom, code := actuel.Name, actuel.Code
	adresse, tel, ville := actuel.Address, actuel.Phone, actuel.City
	statut := actuel.Status
	if req.Name != nil {
		nom = *req.Name
	}
	if req.Code != nil {
		code = *req.Code
	}
	if req.Address != nil {
		adresse = *req.Address
	}
	if req.Phone != nil {
		tel = *req.Phone
	}
	if req.City != nil {
		ville = *req.City
	}
	if req.Status != nil {
		statut = *req.Status
	}

	if req.Name != nil && *req.Name != actuel.Name {
		var existe bool
		if err := s.database.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM branches
				WHERE tenant_id = $1 AND id <> $2 AND lower(btrim(name)) = lower($3) AND status <> 'archived'
			)`, tenantID, id, *req.Name).Scan(&existe); err != nil {
			return nil, err
		}
		if existe {
			return nil, ErrNameExists
		}
	}

	_, err = s.database.ExecContext(ctx, `
		UPDATE branches SET name = $3, code = NULLIF($4, ''), address = NULLIF($5, ''),
		       phone = NULLIF($6, ''), city = NULLIF($7, ''), status = $8, updated_at = NOW()
		WHERE tenant_id = $1 AND id = $2`,
		tenantID, id, nom, code, adresse, tel, ville, statut)
	if err != nil {
		return nil, traduireErreurBranche(err)
	}
	return s.GetBranch(ctx, tenantID, id)
}
