package expense

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"

	"pressing-os/backend/internal/database"
)

// dbStore exécute les requêtes dépenses contre PostgreSQL.
type dbStore struct {
	database *database.Store
}

const expenseColumns = `id, tenant_id, COALESCE(branch_id::text, ''), category, label, amount_xof,
	COALESCE(payment_method, ''), spent_at, COALESCE(note, ''), COALESCE(created_by::text, ''),
	created_at, updated_at`

// uuidRE valide le format d'un identifiant UUID tel que stocké en base.
var uuidRE = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// uuidPourColonne retourne l'identifiant exploitable dans une colonne UUID, ou
// nil. Les comptes de démonstration (identifiants mémoire de forme libre, par
// exemple « usr-gerant-001 ») n'existent pas en base : sans cette conversion, la
// dépense était rejetée par PostgreSQL (22P02). Elle reste enregistrée, sans
// attribution d'auteur.
func uuidPourColonne(id string) interface{} {
	if !uuidRE.MatchString(strings.TrimSpace(id)) {
		return nil
	}
	return strings.TrimSpace(id)
}

func scanExpense(row pgx.Row) (*Expense, error) {
	var e Expense
	var date interface{}
	err := row.Scan(&e.ID, &e.TenantID, &e.BranchID, &e.Category, &e.Label, &e.AmountXOF,
		&e.PaymentMethod, &date, &e.Note, &e.CreatedBy, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return nil, err
	}
	e.SpentAt = formatDate(date)
	return &e, nil
}

// formatDate convertit une colonne DATE (time.Time ou chaîne selon le pilote)
// au format AAAA-MM-JJ attendu par l'API.
func formatDate(v interface{}) string {
	switch d := v.(type) {
	case nil:
		return ""
	case string:
		if len(d) >= 10 {
			return d[:10]
		}
		return d
	default:
		if t, ok := v.(interface{ Format(string) string }); ok {
			return t.Format("2006-01-02")
		}
		return ""
	}
}

// GetExpenses liste les dépenses du pressing sur la période demandée.
func (s *dbStore) GetExpenses(ctx context.Context, tenantID string, f Filter) ([]*Expense, error) {
	rows, err := s.database.QueryContext(ctx, `
		SELECT `+expenseColumns+`
		FROM expenses
		WHERE tenant_id = $1
		  AND ($2 = '' OR spent_at >= $2::date)
		  AND ($3 = '' OR spent_at <= $3::date)
		  AND ($4 = '' OR category = $4)
		  AND ($5 = '' OR branch_id::text = $5)
		ORDER BY spent_at DESC, created_at DESC`,
		tenantID, f.From, f.To, f.Category, f.BranchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*Expense
	for rows.Next() {
		e, err := scanExpense(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// GetExpense retourne une dépense du pressing (isolation par tenant_id).
func (s *dbStore) GetExpense(ctx context.Context, tenantID, id string) (*Expense, error) {
	row := s.database.QueryRowContext(ctx,
		`SELECT `+expenseColumns+` FROM expenses WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	e, err := scanExpense(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrExpenseNotFound
		}
		return nil, err
	}
	return e, nil
}

// CreateExpense insère une dépense rattachée au pressing (et, si fourni, au site).
func (s *dbStore) CreateExpense(ctx context.Context, tenantID, userID string, req CreateExpenseRequest) (*Expense, error) {
	var id string
	err := s.database.QueryRowContext(ctx, `
		INSERT INTO expenses (tenant_id, branch_id, category, label, amount_xof, payment_method, spent_at, note, created_by)
		VALUES ($1, NULLIF($2, '')::uuid, $3, $4, $5, NULLIF($6, ''), $7::date, NULLIF($8, ''), $9::uuid)
		RETURNING id`,
		tenantID, req.BranchID, req.Category, req.Label, req.AmountXOF,
		req.PaymentMethod, req.SpentAt, req.Note, uuidPourColonne(userID)).Scan(&id)
	if err != nil {
		return nil, err
	}
	return s.GetExpense(ctx, tenantID, id)
}

// DeleteExpense supprime une dépense de l'établissement.
func (s *dbStore) DeleteExpense(ctx context.Context, tenantID, id string) error {
	tag, err := s.database.ExecContext(ctx, `DELETE FROM expenses WHERE tenant_id = $1 AND id = $2`, tenantID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrExpenseNotFound
	}
	return nil
}
