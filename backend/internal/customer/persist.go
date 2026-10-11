package customer

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"pressing-os/backend/internal/database"
)

// Store est le contrat de persistance pour le module clients.
type Store interface {
	GetCustomers(ctx context.Context, tenantID, search string) ([]*Customer, error)
	GetCustomer(ctx context.Context, tenantID, id string) (*Customer, error)
	CreateCustomer(ctx context.Context, tenantID string, req CreateCustomerRequest) (*Customer, error)
	UpdateCustomer(ctx context.Context, tenantID, id string, req UpdateCustomerRequest) (*Customer, error)
	DeleteCustomer(ctx context.Context, tenantID, id string) error
}

// dbStore exécute les requêtes clients contre PostgreSQL.
type dbStore struct {
	database *database.Store
}

// NewStore construit un store clients adossé à PostgreSQL.
func NewStore(db *database.Store) *dbStore {
	return &dbStore{database: db}
}

const customerColumns = `id, tenant_id, first_name, last_name, phone, email, address, notes, created_at, updated_at`

// scanCustomer lit une ligne client et reconstitue le FullName.
// Les colonnes nullable (email, address, notes) sont scannées via des
// pointeurs pour tolérer le NULL de PostgreSQL.
func scanCustomer(row pgx.Row) (*Customer, error) {
	var c Customer
	var email, address, notes *string
	err := row.Scan(
		&c.ID, &c.TenantID, &c.FirstName, &c.LastName, &c.Phone,
		&email, &address, &notes, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	c.Email = derefString(email)
	c.Address = derefString(address)
	c.Notes = derefString(notes)
	c.FullName = buildFullName(c.FirstName, c.LastName)
	return &c, nil
}

func scanCustomers(rows pgx.Rows) ([]*Customer, error) {
	var out []*Customer
	for rows.Next() {
		var c Customer
		var email, address, notes *string
		if err := rows.Scan(
			&c.ID, &c.TenantID, &c.FirstName, &c.LastName, &c.Phone,
			&email, &address, &notes, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		c.Email = derefString(email)
		c.Address = derefString(address)
		c.Notes = derefString(notes)
		c.FullName = buildFullName(c.FirstName, c.LastName)
		out = append(out, &c)
	}
	return out, rows.Err()
}

// derefString convertit un pointeur nullable issu de la base en chaîne vide.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func buildFullName(first, last string) string {
	switch {
	case first != "" && last != "":
		return first + " " + last
	case first != "":
		return first
	default:
		return last
	}
}

// translateCustomerErr convertit une violation d'unicité (tenant_id, phone) en erreur métier.
func translateCustomerErr(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrPhoneExists
	}
	return err
}

// GetCustomers liste les clients d'un tenant, avec recherche plein texte optionnelle.
func (s *dbStore) GetCustomers(ctx context.Context, tenantID, search string) ([]*Customer, error) {
	query := `SELECT ` + customerColumns + ` FROM customers WHERE tenant_id = $1`
	args := []interface{}{tenantID}
	if search != "" {
		query += ` AND (first_name ILIKE $2 OR last_name ILIKE $2 OR phone ILIKE $2 OR COALESCE(email,'') ILIKE $2)`
		args = append(args, "%"+search+"%")
	}
	query += ` ORDER BY first_name ASC, last_name ASC`
	rows, err := s.database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCustomers(rows)
}

// GetCustomer récupère un client par son ID au sein du tenant.
func (s *dbStore) GetCustomer(ctx context.Context, tenantID, id string) (*Customer, error) {
	query := `SELECT ` + customerColumns + ` FROM customers WHERE tenant_id = $1 AND id = $2`
	row := s.database.QueryRowContext(ctx, query, tenantID, id)
	c, err := scanCustomer(row)
	if err == pgx.ErrNoRows {
		return nil, ErrCustomerNotFound
	}
	return c, err
}

// CreateCustomer insère un nouveau client (UUID généré côté SQL).
func (s *dbStore) CreateCustomer(ctx context.Context, tenantID string, req CreateCustomerRequest) (*Customer, error) {
	query := `INSERT INTO customers (id, tenant_id, first_name, last_name, phone, email, address, notes)
	          VALUES (uuid_generate_v4(), $1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), NULLIF($7, ''))
	          RETURNING ` + customerColumns
	row := s.database.QueryRowContext(ctx, query,
		tenantID, req.FirstName, req.LastName, req.Phone, req.Email, req.Address, req.Notes,
	)
	c, err := scanCustomer(row)
	if err != nil {
		return nil, translateCustomerErr(err)
	}
	return c, nil
}

// UpdateCustomer met à jour un client existant et renvoie la version persistée.
func (s *dbStore) UpdateCustomer(ctx context.Context, tenantID, id string, req UpdateCustomerRequest) (*Customer, error) {
	query := `UPDATE customers
	          SET first_name = $3, last_name = $4, phone = $5,
	              email = NULLIF($6, ''), address = NULLIF($7, ''), notes = NULLIF($8, ''),
	              updated_at = NOW()
	          WHERE tenant_id = $1 AND id = $2
	          RETURNING ` + customerColumns
	row := s.database.QueryRowContext(ctx, query,
		tenantID, id, req.FirstName, req.LastName, req.Phone, req.Email, req.Address, req.Notes,
	)
	c, err := scanCustomer(row)
	if err == pgx.ErrNoRows {
		return nil, ErrCustomerNotFound
	}
	if err != nil {
		return nil, translateCustomerErr(err)
	}
	return c, nil
}

// DeleteCustomer supprime définitivement un client du tenant.
func (s *dbStore) DeleteCustomer(ctx context.Context, tenantID, id string) error {
	tag, err := s.database.ExecContext(ctx,
		`DELETE FROM customers WHERE tenant_id = $1 AND id = $2`, tenantID, id,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrCustomerNotFound
	}
	return nil
}
