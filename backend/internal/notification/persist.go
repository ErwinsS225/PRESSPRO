package notification

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"pressing-os/backend/internal/database"
	"pressing-os/backend/internal/pgutil"
)

// dbStore exécute les requêtes du journal des notifications contre PostgreSQL.
type dbStore struct {
	database *database.Store
}

const notificationColumns = `id, tenant_id, COALESCE(order_id::text, ''), channel, COALESCE(template, ''),
	recipient, body, status, COALESCE(provider, ''), COALESCE(detail, ''), attempts,
	COALESCE(created_by::text, ''), created_at, sent_at`

// Create insère une notification et retourne la ligne persistée.
func (s *dbStore) Create(ctx context.Context, n *Notification) error {
	var id string
	err := s.database.QueryRowContext(ctx, `
		INSERT INTO notifications (tenant_id, order_id, channel, template, recipient, body,
			status, provider, detail, attempts, created_by, created_at, sent_at)
		VALUES ($1, NULLIF($2, '')::uuid, $3, NULLIF($4, ''), $5, $6, $7, NULLIF($8, ''),
			NULLIF($9, ''), $10, $11::uuid, $12, $13)
		RETURNING id`,
		n.TenantID, n.OrderID, n.Channel, n.Template, n.Recipient, n.Body,
		n.Status, n.Provider, n.Detail, n.Attempts, pgutil.NullableUUID(n.CreatedBy),
		n.CreatedAt, n.SentAt).Scan(&id)
	if err != nil {
		return err
	}
	n.ID = id
	return nil
}

// List retourne le journal d'un pressing, du plus récent au plus ancien.
func (s *dbStore) List(ctx context.Context, tenantID string, f Filtre) ([]*Notification, error) {
	requete := `SELECT ` + notificationColumns + ` FROM notifications WHERE tenant_id = $1`
	params := []interface{}{tenantID}
	ajouter := func(valeur interface{}, condition string) {
		params = append(params, valeur)
		requete += fmt.Sprintf(condition, len(params))
	}
	if f.OrderID != "" {
		ajouter(f.OrderID, ` AND order_id = $%d::uuid`)
	}
	if f.Channel != "" {
		ajouter(f.Channel, ` AND channel = $%d`)
	}
	if f.Status != "" {
		ajouter(f.Status, ` AND status = $%d`)
	}
	ajouter(f.Limit, ` ORDER BY created_at DESC LIMIT $%d`)

	rows, err := s.database.QueryContext(ctx, requete, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notifications := []*Notification{}
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		notifications = append(notifications, n)
	}
	return notifications, rows.Err()
}

func scanNotification(row pgx.Row) (*Notification, error) {
	var n Notification
	err := row.Scan(&n.ID, &n.TenantID, &n.OrderID, &n.Channel, &n.Template, &n.Recipient,
		&n.Body, &n.Status, &n.Provider, &n.Detail, &n.Attempts, &n.CreatedBy, &n.CreatedAt, &n.SentAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotificationAbsente
		}
		return nil, err
	}
	return &n, nil
}
