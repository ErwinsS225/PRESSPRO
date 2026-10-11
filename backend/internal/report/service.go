package report

import (
	"context"
	"math"
	"sort"
	"time"

	"pressing-os/backend/internal/database"
	"pressing-os/backend/internal/expense"
	"pressing-os/backend/internal/order"
)

// Store est le contrat de persistance des rapports (agrégats SQL).
type Store interface {
	Summary(ctx context.Context, tenantID string, from, to time.Time, consolidated bool) (*Summary, error)
}

// OrderSource alimente le rapport en mode mémoire (mode démonstration).
type OrderSource interface {
	GetOrders(ctx context.Context, tenantID, statusFilter, search string) ([]*order.Order, error)
}

// ExpenseSource alimente le rapport en mode mémoire.
type ExpenseSource interface {
	GetExpenses(ctx context.Context, tenantID string, f expense.Filter) ([]*expense.Expense, error)
}

// Service produit les rapports d'activité (module payant reports.advanced).
type Service struct {
	store    Store
	orders   OrderSource
	expenses ExpenseSource
}

// NewService crée le service de rapports.
func NewService() *Service { return &Service{} }

// NewStore construit un service de rapports adossé à PostgreSQL.
func NewStore(db *database.Store) *Service {
	return &Service{store: &dbStore{database: db}}
}

// SetStore branche la persistance sur un service existant.
func (s *Service) SetStore(store Store) { s.store = store }

// SetSources branche les sources mémoire (repli sans base de données).
func (s *Service) SetSources(orders OrderSource, expenses ExpenseSource) {
	s.orders = orders
	s.expenses = expenses
}

// parsePeriod valide les bornes et normalise les valeurs par défaut
// (mois courant lorsque rien n'est fourni).
func parsePeriod(from, to string) (time.Time, time.Time, error) {
	maintenant := time.Now().UTC()
	if from == "" {
		from = time.Date(maintenant.Year(), maintenant.Month(), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	}
	if to == "" {
		to = maintenant.Format("2006-01-02")
	}
	debut, err := time.Parse("2006-01-02", from)
	if err != nil {
		return time.Time{}, time.Time{}, ErrInvalidPeriod
	}
	fin, err := time.Parse("2006-01-02", to)
	if err != nil || fin.Before(debut) {
		return time.Time{}, time.Time{}, ErrInvalidPeriod
	}
	return debut, fin, nil
}

// Summary calcule le rapport d'activité de la période.
func (s *Service) Summary(ctx context.Context, tenantID string, from, to string, consolidated bool) (*Summary, error) {
	debut, fin, err := parsePeriod(from, to)
	if err != nil {
		return nil, err
	}

	if s.store != nil {
		return s.store.Summary(ctx, tenantID, debut, fin, consolidated)
	}
	if s.orders == nil && s.expenses == nil {
		return nil, ErrReportUnavailable
	}
	return s.summaryMemoire(ctx, tenantID, debut, fin, consolidated)
}

// summaryMemoire calcule le rapport à partir des services en mémoire (mode démo).
func (s *Service) summaryMemoire(ctx context.Context, tenantID string, debut, fin time.Time, consolidated bool) (*Summary, error) {
	finExclusive := fin.AddDate(0, 0, 1)
	dansPeriode := func(t time.Time) bool {
		return !t.Before(debut) && t.Before(finExclusive)
	}

	rapport := &Summary{
		Period:         Period{From: debut.Format("2006-01-02"), To: fin.Format("2006-01-02")},
		DataSource:     "memoire",
		OrdersByStatus: map[string]int{},
	}

	var totalCommandes float64
	services := map[string]*ServiceTotal{}

	if s.orders != nil {
		commandes, err := s.orders.GetOrders(ctx, tenantID, "", "")
		if err != nil {
			return nil, err
		}
		for _, o := range commandes {
			if !dansPeriode(o.CreatedAt) {
				continue
			}
			rapport.OrdersCount++
			rapport.OrdersByStatus[o.Status]++
			totalCommandes += o.TotalAmount
			for _, p := range o.Payments {
				if dansPeriode(p.CreatedAt) {
					rapport.RevenueXOF += p.Amount
				}
			}
			for _, it := range o.Items {
				cumul, ok := services[it.ItemLabel]
				if !ok {
					cumul = &ServiceTotal{Label: it.ItemLabel}
					services[it.ItemLabel] = cumul
				}
				cumul.Count += it.Quantity
				cumul.RevenueXOF += it.Subtotal
			}
		}
	}
	if rapport.OrdersCount > 0 {
		rapport.AverageTicketXOF = arrondi(totalCommandes / float64(rapport.OrdersCount))
	}
	rapport.TopServices = topServices(services)

	if s.expenses != nil {
		depenses, err := s.expenses.GetExpenses(ctx, tenantID, expense.Filter{
			From: debut.Format("2006-01-02"), To: fin.Format("2006-01-02"),
		})
		if err != nil {
			return nil, err
		}
		rapport.ExpensesXOF = expense.Total(depenses)
		for _, c := range expense.ParCategorie(depenses) {
			rapport.ExpensesByCategory = append(rapport.ExpensesByCategory, CategoryTotalReport{
				Category: c.Category, TotalXOF: c.TotalXOF, Count: c.Count,
			})
		}
	}
	if rapport.ExpensesByCategory == nil {
		rapport.ExpensesByCategory = []CategoryTotalReport{}
	}

	rapport.MarginXOF = rapport.RevenueXOF - rapport.ExpensesXOF
	rapport.Consolidated = consolidated
	if !consolidated {
		rapport.PlanHint = "La consolidation multi-sites est incluse dans la formule BUSINESS."
		return rapport, nil
	}
	// Mode mémoire : toutes les commandes sont rattachées au site principal.
	rapport.ByBranch = []BranchTotal{{
		BranchID:    "branch-principal",
		Name:        "Établissement principal",
		OrdersCount: rapport.OrdersCount,
		RevenueXOF:  rapport.RevenueXOF,
		ExpensesXOF: rapport.ExpensesXOF,
		MarginXOF:   rapport.MarginXOF,
	}}
	return rapport, nil
}

// topServices classe les prestations par chiffre d'affaires décroissant (5 max).
func topServices(index map[string]*ServiceTotal) []ServiceTotal {
	out := make([]ServiceTotal, 0, len(index))
	for _, s := range index {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RevenueXOF == out[j].RevenueXOF {
			return out[i].Count > out[j].Count
		}
		return out[i].RevenueXOF > out[j].RevenueXOF
	})
	if len(out) > 5 {
		out = out[:5]
	}
	return out
}

// arrondi arrondit au franc CFA (pas de centimes en XOF).
func arrondi(v float64) float64 { return math.Round(v) }
