package catalog

import (
	"context"
	"fmt"

	"pressing-os/backend/internal/database"
)

// Store is the persistence backend contract for the catalog.
type Store interface {
	GetServices(ctx context.Context, tenantID, category, search string) ([]*ServiceItem, error)
	GetService(ctx context.Context, tenantID, id string) (*ServiceItem, error)
	CreateService(ctx context.Context, tenantID string, req CreateServiceRequest) (*ServiceItem, error)
	UpdateService(ctx context.Context, tenantID, id string, req UpdateServiceRequest) (*ServiceItem, error)
	DeleteService(ctx context.Context, tenantID, id string) error
	// SeedIfEmpty insère la grille tarifaire par défaut si le catalogue du
	// tenant est vide (onboarding). Retourne le nombre de prestations créées.
	SeedIfEmpty(ctx context.Context, tenantID string, items []ServiceItem) (int, error)
}

// dbStore executes catalog queries against PostgreSQL.
type dbStore struct {
	database *database.Store
}

// NewStore builds a catalog backed by PostgreSQL.
//
// Le Service est construit via newService() afin de garantir l'initialisation
// du logger et de l'index mémoire : un Service partiel provoquerait une panique
// (nil pointer) dès la première lecture du catalogue.
func NewStore(db *database.Store) *Service {
	s := newService()
	s.store = &dbStore{database: db}
	return s
}

func (s *dbStore) GetServices(ctx context.Context, tenantID, category, search string) ([]*ServiceItem, error) {
	query := `SELECT id, tenant_id, name, article, treatment, category, unit_price, unit, pricing_model,
	                 estimated_hours, is_active, is_express_available, express_surcharge,
	                 is_pickup_available, is_outsourced, requires_quote, description,
	                 created_at, updated_at
	          FROM services WHERE tenant_id = $1`
	args := []interface{}{tenantID}
	argIdx := 2
	if category != "" && category != "all" {
		query += fmt.Sprintf(` AND category = $%d`, argIdx)
		args = append(args, category)
		argIdx++
	}
	if search != "" {
		query += fmt.Sprintf(` AND (name ILIKE $%d OR category ILIKE $%d)`, argIdx, argIdx)
		args = append(args, "%"+search+"%")
		argIdx++
	}
	query += ` ORDER BY name ASC`
	rows, err := s.database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items, err := scanServices(rows)
	return items, err
}

// SeedIfEmpty initialise le catalogue d'un nouvel établissement en une seule
// transaction : aucune insertion si le tenant possède déjà des prestations
// (opération idempotente, appelable après une inscription).
func (s *dbStore) SeedIfEmpty(ctx context.Context, tenantID string, items []ServiceItem) (int, error) {
	var existing int
	if err := s.database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM services WHERE tenant_id = $1`, tenantID).Scan(&existing); err != nil {
		return 0, fmt.Errorf("comptage du catalogue: %w", err)
	}
	if existing > 0 {
		return 0, nil
	}

	tx, err := s.database.Pool().Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("ouverture de transaction (seed catalogue): %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	inserted := 0
	for _, it := range items {
		if _, err := tx.Exec(ctx, `
			INSERT INTO services (tenant_id, name, article, treatment, category, unit_price, unit,
			                      pricing_model, estimated_hours, is_active, is_express_available,
			                      express_surcharge, is_pickup_available, is_outsourced, requires_quote, description)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`,
			tenantID, it.Name, it.Article, it.Treatment, it.Category, it.UnitPrice, it.Unit,
			it.PricingModel, it.EstimatedHours, it.IsActive, it.IsExpressAvailable,
			it.ExpressSurcharge, it.IsPickupAvailable, it.IsOutsourced, it.RequiresQuote, it.Description,
		); err != nil {
			return 0, fmt.Errorf("insertion prestation par défaut: %w", err)
		}
		inserted++
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("validation du seed catalogue: %w", err)
	}
	return inserted, nil
}

func (s *dbStore) GetService(ctx context.Context, tenantID, id string) (*ServiceItem, error) {
	query := `SELECT id, tenant_id, name, article, treatment, category, unit_price, unit, pricing_model,
	                 estimated_hours, is_active, is_express_available, express_surcharge,
	                 is_pickup_available, is_outsourced, requires_quote, description,
	                 created_at, updated_at
	          FROM services WHERE tenant_id = $1 AND id = $2`
	row := s.database.QueryRowContext(ctx, query, tenantID, id)
	return scanService(row)
}
