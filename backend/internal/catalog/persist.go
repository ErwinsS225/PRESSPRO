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
}

// dbStore executes catalog queries against PostgreSQL.
type dbStore struct {
	database *database.Store
}

// NewStore builds a catalog backed by PostgreSQL.
func NewStore(db *database.Store) *Service {
	return &Service{store: &dbStore{database: db}}
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

func (s *dbStore) GetService(ctx context.Context, tenantID, id string) (*ServiceItem, error) {
	query := `SELECT id, tenant_id, name, article, treatment, category, unit_price, unit, pricing_model,
	                 estimated_hours, is_active, is_express_available, express_surcharge,
	                 is_pickup_available, is_outsourced, requires_quote, description,
	                 created_at, updated_at
	          FROM services WHERE tenant_id = $1 AND id = $2`
	row := s.database.QueryRowContext(ctx, query, tenantID, id)
	return scanService(row)
}
