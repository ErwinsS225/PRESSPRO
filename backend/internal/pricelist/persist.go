package pricelist

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"pressing-os/backend/internal/database"
)

// ErrNotFound est renvoyé quand la grille ou la version est introuvable pour ce tenant.
var ErrNotFound = errors.New("grille tarifaire introuvable")

// Store est le contrat de persistance des grilles tarifaires.
type Store interface {
	ListPriceLists(ctx context.Context, tenantID string) ([]*PriceList, error)
	GetPriceList(ctx context.Context, tenantID, id string) (*PriceList, error)
	ListVersions(ctx context.Context, tenantID, priceListID string) ([]*PriceListVersion, error)
	CreatePriceList(ctx context.Context, tenantID string, req CreatePriceListRequest) (*PriceList, error)
	InsertVersion(ctx context.Context, tenantID, priceListID string, snapshot []PriceListEntry, note string) (*PriceListVersion, error)
	NextVersionNumber(ctx context.Context, priceListID string) (int, error)
	SupersedeActiveVersion(ctx context.Context, priceListID string) error
	// CatalogSnapshot fige le catalogue de prestations courant du tenant (base du snapshot de version).
	CatalogSnapshot(ctx context.Context, tenantID string) ([]PriceListEntry, error)
}

// dbStore exécute les requêtes grilles tarifaires contre PostgreSQL.
type dbStore struct {
	database *database.Store
}

// NewStore construit un service de grilles tarifaires adossé à PostgreSQL.
// Le Service est bâti via NewService() pour garantir l'initialisation des maps
// internes (même précaution que le catalogue : jamais de Service partiel).
func NewStore(db *database.Store) *Service {
	s := NewService()
	s.store = &dbStore{database: db}
	return s
}

func (s *dbStore) ListPriceLists(ctx context.Context, tenantID string) ([]*PriceList, error) {
	rows, err := s.database.QueryContext(ctx, `
		SELECT id, tenant_id, COALESCE(branch_id::text, ''), name, COALESCE(description, ''),
		       is_default, status, created_at, updated_at
		FROM price_lists WHERE tenant_id = $1 ORDER BY is_default DESC, name ASC`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("liste des grilles: %w", err)
	}
	defer rows.Close()

	var out []*PriceList
	for rows.Next() {
		pl := &PriceList{}
		if err := rows.Scan(&pl.ID, &pl.TenantID, &pl.BranchID, &pl.Name, &pl.Description,
			&pl.IsDefault, &pl.Status, &pl.CreatedAt, &pl.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan grille: %w", err)
		}
		out = append(out, pl)
	}
	return out, rows.Err()
}

func (s *dbStore) GetPriceList(ctx context.Context, tenantID, id string) (*PriceList, error) {
	row := s.database.QueryRowContext(ctx, `
		SELECT id, tenant_id, COALESCE(branch_id::text, ''), name, COALESCE(description, ''),
		       is_default, status, created_at, updated_at
		FROM price_lists WHERE id = $1 AND tenant_id = $2`, id, tenantID)

	pl := &PriceList{}
	if err := row.Scan(&pl.ID, &pl.TenantID, &pl.BranchID, &pl.Name, &pl.Description,
		&pl.IsDefault, &pl.Status, &pl.CreatedAt, &pl.UpdatedAt); err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("lecture grille: %w", err)
	}
	return pl, nil
}

func (s *dbStore) ListVersions(ctx context.Context, tenantID, priceListID string) ([]*PriceListVersion, error) {
	rows, err := s.database.QueryContext(ctx, `
		SELECT id, price_list_id, tenant_id, version, status, snapshot, COALESCE(note, ''), created_at
		FROM price_list_versions
		WHERE tenant_id = $1 AND price_list_id = $2
		ORDER BY version DESC`, tenantID, priceListID)
	if err != nil {
		return nil, fmt.Errorf("liste des versions: %w", err)
	}
	defer rows.Close()

	var out []*PriceListVersion
	for rows.Next() {
		v := &PriceListVersion{}
		var snapshotRaw []byte
		if err := rows.Scan(&v.ID, &v.PriceListID, &v.TenantID, &v.Version, &v.Status,
			&snapshotRaw, &v.Note, &v.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan version: %w", err)
		}
		v.Snapshot = []PriceListEntry{}
		if len(snapshotRaw) > 0 {
			_ = json.Unmarshal(snapshotRaw, &v.Snapshot)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *dbStore) CreatePriceList(ctx context.Context, tenantID string, req CreatePriceListRequest) (*PriceList, error) {
	var branchID *string
	if req.BranchID != "" {
		branchID = &req.BranchID
	}
	row := s.database.QueryRowContext(ctx, `
		INSERT INTO price_lists (tenant_id, branch_id, name, description, is_default)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5)
		RETURNING id, tenant_id, COALESCE(branch_id::text, ''), name, COALESCE(description, ''),
		          is_default, status, created_at, updated_at`,
		tenantID, branchID, req.Name, req.Description, req.IsDefault)

	pl := &PriceList{}
	if err := row.Scan(&pl.ID, &pl.TenantID, &pl.BranchID, &pl.Name, &pl.Description,
		&pl.IsDefault, &pl.Status, &pl.CreatedAt, &pl.UpdatedAt); err != nil {
		return nil, fmt.Errorf("création grille: %w", err)
	}
	return pl, nil
}

// NextVersionNumber retourne le prochain numéro de version (monotone par grille).
func (s *dbStore) NextVersionNumber(ctx context.Context, priceListID string) (int, error) {
	var next int
	err := s.database.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(version), 0) + 1 FROM price_list_versions WHERE price_list_id = $1`,
		priceListID).Scan(&next)
	if err != nil {
		return 0, fmt.Errorf("numéro de version: %w", err)
	}
	return next, nil
}

// SupersedeActiveVersion marque les versions actives comme dépassées avant insertion de la nouvelle.
func (s *dbStore) SupersedeActiveVersion(ctx context.Context, priceListID string) error {
	_, err := s.database.ExecContext(ctx, `
		UPDATE price_list_versions SET status = 'superseded'
		WHERE price_list_id = $1 AND status = 'active'`, priceListID)
	if err != nil {
		return fmt.Errorf("supersession version: %w", err)
	}
	return nil
}

// InsertVersion insère le snapshot figé de la nouvelle version (statut active).
func (s *dbStore) InsertVersion(ctx context.Context, tenantID, priceListID string, snapshot []PriceListEntry, note string) (*PriceListVersion, error) {
	next, err := s.NextVersionNumber(ctx, priceListID)
	if err != nil {
		return nil, err
	}
	// Atomicité : la supersession et l'insertion partagent la même requête CTE.
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("encodage snapshot: %w", err)
	}

	tx, err := s.database.Pool().Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("début transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
		UPDATE price_list_versions SET status = 'superseded'
		WHERE price_list_id = $1 AND status = 'active'`, priceListID); err != nil {
		return nil, fmt.Errorf("supersession: %w", err)
	}

	v := &PriceListVersion{}
	err = tx.QueryRow(ctx, `
		INSERT INTO price_list_versions (price_list_id, tenant_id, version, status, snapshot, note)
		VALUES ($1, $2, $3, 'active', $4::jsonb, NULLIF($5, ''))
		RETURNING id, price_list_id, tenant_id, version, status, snapshot, COALESCE(note, ''), created_at`,
		priceListID, tenantID, next, string(payload), note).Scan(
		&v.ID, &v.PriceListID, &v.TenantID, &v.Version, &v.Status, &[]byte{}, &v.Note, &v.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insertion version: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit version: %w", err)
	}
	v.Snapshot = snapshot
	if v.Snapshot == nil {
		v.Snapshot = []PriceListEntry{}
	}
	return v, nil
}

// CatalogSnapshot lit le catalogue de prestations actif du tenant pour figer le snapshot.
func (s *dbStore) CatalogSnapshot(ctx context.Context, tenantID string) ([]PriceListEntry, error) {
	rows, err := s.database.QueryContext(ctx, `
		SELECT id::text, name, COALESCE(article, ''), COALESCE(treatment, ''), category,
		       unit_price, unit, pricing_model, is_active
		FROM services
		WHERE tenant_id = $1 AND is_active = true
		ORDER BY name ASC`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("snapshot catalogue: %w", err)
	}
	defer rows.Close()

	out := []PriceListEntry{}
	for rows.Next() {
		var e PriceListEntry
		if err := rows.Scan(&e.ServiceID, &e.Name, &e.Article, &e.Treatment, &e.Category,
			&e.UnitPrice, &e.Unit, &e.PricingModel, &e.IsActive); err != nil {
			return nil, fmt.Errorf("scan snapshot: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
