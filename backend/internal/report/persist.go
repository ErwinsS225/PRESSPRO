package report

import (
	"context"
	"time"

	"pressing-os/backend/internal/database"
)

// dbStore calcule les rapports par agrégats SQL (PostgreSQL).
type dbStore struct {
	database *database.Store
}

// Summary agrège l'activité de la période :
//   - chiffre d'affaires = paiements réellement encaissés (`payments`) ;
//   - commandes, panier moyen et répartition par statut (`orders`) ;
//   - dépenses par catégorie (`expenses`) et marge ;
//   - prestations les plus vendues (`order_items`) ;
//   - consolidation par établissement (formule BUSINESS uniquement).
func (s *dbStore) Summary(ctx context.Context, tenantID string, from, to time.Time, consolidated bool) (*Summary, error) {
	finExclusive := to.AddDate(0, 0, 1)

	rapport := &Summary{
		Period:         Period{From: from.Format("2006-01-02"), To: to.Format("2006-01-02")},
		DataSource:     "postgresql",
		OrdersByStatus: map[string]int{},
	}

	// 1) Chiffre d'affaires encaissé sur la période (journal des paiements).
	if err := s.database.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount), 0)
		FROM payments
		WHERE tenant_id = $1 AND created_at >= $2 AND created_at < $3`,
		tenantID, from, finExclusive).Scan(&rapport.RevenueXOF); err != nil {
		return nil, err
	}

	// 2) Commandes de la période + panier moyen.
	if err := s.database.QueryRowContext(ctx, `
		SELECT COUNT(*), COALESCE(AVG(total_amount), 0)
		FROM orders
		WHERE tenant_id = $1 AND created_at >= $2 AND created_at < $3`,
		tenantID, from, finExclusive).Scan(&rapport.OrdersCount, &rapport.AverageTicketXOF); err != nil {
		return nil, err
	}
	rapport.AverageTicketXOF = arrondi(rapport.AverageTicketXOF)

	// 3) Répartition des commandes par statut.
	lignes, err := s.database.QueryContext(ctx, `
		SELECT status, COUNT(*)
		FROM orders
		WHERE tenant_id = $1 AND created_at >= $2 AND created_at < $3
		GROUP BY status`, tenantID, from, finExclusive)
	if err != nil {
		return nil, err
	}
	for lignes.Next() {
		var statut string
		var n int
		if err := lignes.Scan(&statut, &n); err != nil {
			lignes.Close()
			return nil, err
		}
		rapport.OrdersByStatus[statut] = n
	}
	lignes.Close()
	if err := lignes.Err(); err != nil {
		return nil, err
	}

	// 4) Dépenses par catégorie (module Dépenses).
	depenses, err := s.database.QueryContext(ctx, `
		SELECT category, COALESCE(SUM(amount_xof), 0), COUNT(*)
		FROM expenses
		WHERE tenant_id = $1 AND spent_at >= $2 AND spent_at <= $3
		GROUP BY category
		ORDER BY 2 DESC`, tenantID, from, to)
	if err != nil {
		return nil, err
	}
	rapport.ExpensesByCategory = []CategoryTotalReport{}
	for depenses.Next() {
		var c CategoryTotalReport
		if err := depenses.Scan(&c.Category, &c.TotalXOF, &c.Count); err != nil {
			depenses.Close()
			return nil, err
		}
		rapport.ExpensesByCategory = append(rapport.ExpensesByCategory, c)
		rapport.ExpensesXOF += c.TotalXOF
	}
	depenses.Close()
	if err := depenses.Err(); err != nil {
		return nil, err
	}

	// 5) Prestations les plus vendues de la période.
	prestations, err := s.database.QueryContext(ctx, `
		SELECT oi.item_label, SUM(oi.quantity), SUM(oi.subtotal)
		FROM order_items oi
		JOIN orders o ON o.id = oi.order_id
		WHERE o.tenant_id = $1 AND o.created_at >= $2 AND o.created_at < $3
		GROUP BY oi.item_label
		ORDER BY 3 DESC
		LIMIT 5`, tenantID, from, finExclusive)
	if err != nil {
		return nil, err
	}
	for prestations.Next() {
		var st ServiceTotal
		if err := prestations.Scan(&st.Label, &st.Count, &st.RevenueXOF); err != nil {
			prestations.Close()
			return nil, err
		}
		rapport.TopServices = append(rapport.TopServices, st)
	}
	prestations.Close()
	if err := prestations.Err(); err != nil {
		return nil, err
	}

	rapport.MarginXOF = rapport.RevenueXOF - rapport.ExpensesXOF
	rapport.Consolidated = consolidated
	if !consolidated {
		rapport.PlanHint = "La consolidation multi-sites est incluse dans la formule BUSINESS."
		return rapport, nil
	}

	// 6) Consolidation multi-sites (formule BUSINESS).
	branches, err := s.database.QueryContext(ctx, `
		SELECT b.id, b.name,
		       (SELECT COUNT(*) FROM orders o WHERE o.branch_id = b.id AND o.created_at >= $2 AND o.created_at < $3),
		       (SELECT COALESCE(SUM(p.amount), 0) FROM payments p
		          JOIN orders o2 ON o2.id = p.order_id
		         WHERE o2.branch_id = b.id AND p.created_at >= $2 AND p.created_at < $3),
		       (SELECT COALESCE(SUM(e.amount_xof), 0) FROM expenses e
		         WHERE e.branch_id = b.id AND e.spent_at >= $2 AND e.spent_at <= $3)
		FROM branches b
		WHERE b.tenant_id = $1 AND b.status <> 'archived'
		ORDER BY b.is_primary DESC, b.created_at ASC`, tenantID, from, finExclusive)
	if err != nil {
		return nil, err
	}
	defer branches.Close()
	for branches.Next() {
		var bt BranchTotal
		if err := branches.Scan(&bt.BranchID, &bt.Name, &bt.OrdersCount, &bt.RevenueXOF, &bt.ExpensesXOF); err != nil {
			return nil, err
		}
		bt.MarginXOF = bt.RevenueXOF - bt.ExpensesXOF
		rapport.ByBranch = append(rapport.ByBranch, bt)
	}
	return rapport, branches.Err()
}
