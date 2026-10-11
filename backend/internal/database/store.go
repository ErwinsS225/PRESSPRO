package database

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store is the PostgreSQL persistence layer used by the business services.
// It exposes the minimal pgxpool operations with a homogeneous call signature.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore creates a Store from an existing pgxpool connection pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Pool exposes the underlying pgxpool.Pool (useful for advanced queries).
func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}

// ExecContext executes a statement that does not return rows (INSERT/UPDATE/DELETE/DROP).
func (s *Store) ExecContext(ctx context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	return s.pool.Exec(ctx, query, args...)
}

// QueryContext executes a query that returns rows (SELECT).
func (s *Store) QueryContext(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error) {
	return s.pool.Query(ctx, query, args...)
}

// QueryRowContext executes a query that is expected to return at most one row.
func (s *Store) QueryRowContext(ctx context.Context, query string, args ...interface{}) pgx.Row {
	return s.pool.QueryRow(ctx, query, args...)
}
