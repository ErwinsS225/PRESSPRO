package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pinger représente le contrat minimal pour vérifier la santé d'une base de données
type Pinger interface {
	Ping(ctx context.Context) error
	Close()
}

// DB encapsule le pool de connexions pgxpool
type DB struct {
	Pool *pgxpool.Pool
}

// ConnectPool initialise le pool PostgreSQL avec un contexte borné
func ConnectPool(ctx context.Context, databaseURL string) (*DB, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("erreur de parsing de la chaîne de connexion: %w", err)
	}

	config.MaxConns = 25
	config.MinConns = 2
	config.MaxConnIdleTime = 5 * time.Minute
	config.MaxConnLifetime = 1 * time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("impossible de créer le pool de connexions: %w", err)
	}

	return &DB{Pool: pool}, nil
}

// Ping teste la réactivité de la base avec un timeout strict
func (d *DB) Ping(ctx context.Context) error {
	if d == nil || d.Pool == nil {
		return fmt.Errorf("pool de base de données non initialisé")
	}
	return d.Pool.Ping(ctx)
}

// Close ferme le pool de connexions proprement
func (d *DB) Close() {
	if d != nil && d.Pool != nil {
		d.Pool.Close()
	}
}
