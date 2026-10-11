package report_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"pressing-os/backend/internal/expense"
	"pressing-os/backend/internal/order"
	"pressing-os/backend/internal/report"
)

const tenant = "a0000000-0000-0000-0000-000000000001"

// commandesStub alimente le rapport en mode mémoire.
type commandesStub struct{ commandes []*order.Order }

func (c commandesStub) GetOrders(_ context.Context, _ string, _, _ string) ([]*order.Order, error) {
	return c.commandes, nil
}

type depensesStub struct{ depenses []*expense.Expense }

func (d depensesStub) GetExpenses(_ context.Context, _ string, f expense.Filter) ([]*expense.Expense, error) {
	// La doublure applique le même filtre de période que le service Dépenses :
	// le rapport doit transmettre les bornes demandées.
	var out []*expense.Expense
	for _, e := range d.depenses {
		if f.From != "" && e.SpentAt < f.From {
			continue
		}
		if f.To != "" && e.SpentAt > f.To {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

func date(v string) time.Time {
	t, _ := time.Parse(time.RFC3339, v)
	return t
}

// TestSummary_Memoire : chiffre d'affaires encaissé, panier moyen, dépenses,
// marge et top prestations sur la période demandée.
func TestSummary_Memoire(t *testing.T) {
	ctx := context.Background()

	commandes := []*order.Order{
		{
			ID: "cmd-1", Status: "delivered", TotalAmount: 6000, CreatedAt: date("2026-10-03T10:00:00Z"),
			Payments: []order.PaymentRecord{{Amount: 6000, CreatedAt: date("2026-10-03T10:05:00Z")}},
			Items: []order.OrderItem{
				{ItemLabel: "Chemise", Quantity: 2, Subtotal: 4000},
				{ItemLabel: "Costume", Quantity: 1, Subtotal: 2000},
			},
		},
		{
			ID: "cmd-2", Status: "ready", TotalAmount: 3000, CreatedAt: date("2026-10-05T09:00:00Z"),
			Payments: []order.PaymentRecord{{Amount: 1000, CreatedAt: date("2026-10-05T09:02:00Z")}},
			Items:    []order.OrderItem{{ItemLabel: "Chemise", Quantity: 1, Subtotal: 3000}},
		},
		{
			// Hors période : septembre.
			ID: "cmd-3", Status: "delivered", TotalAmount: 9000, CreatedAt: date("2026-09-20T10:00:00Z"),
			Payments: []order.PaymentRecord{{Amount: 9000, CreatedAt: date("2026-09-20T10:00:00Z")}},
		},
	}
	depenses := []*expense.Expense{
		{ID: "exp-1", Category: "loyer", AmountXOF: 150000, SpentAt: "2026-10-01"},
		{ID: "exp-2", Category: "matieres_premieres", AmountXOF: 25000, SpentAt: "2026-10-03"},
		{ID: "exp-3", Category: "loyer", AmountXOF: 150000, SpentAt: "2026-09-01"},
	}

	svc := report.NewService()
	svc.SetSources(commandesStub{commandes}, depensesStub{depenses})

	rapport, err := svc.Summary(ctx, tenant, "2026-10-01", "2026-10-31", false)
	if err != nil {
		t.Fatalf("rapport d'activité: %v", err)
	}

	if rapport.DataSource != "memoire" {
		t.Errorf("source de données attendue memoire (repli), obtenue %q", rapport.DataSource)
	}
	if rapport.RevenueXOF != 7000 {
		t.Errorf("chiffre d'affaires encaissé attendu 7000, obtenu %v", rapport.RevenueXOF)
	}
	if rapport.OrdersCount != 2 {
		t.Errorf("2 commandes attendues sur la période, obtenues %d", rapport.OrdersCount)
	}
	if rapport.OrdersByStatus["delivered"] != 1 || rapport.OrdersByStatus["ready"] != 1 {
		t.Errorf("répartition par statut inattendue: %+v", rapport.OrdersByStatus)
	}
	if rapport.AverageTicketXOF != 4500 {
		t.Errorf("panier moyen attendu 4500, obtenu %v", rapport.AverageTicketXOF)
	}
	if rapport.ExpensesXOF != 175000 {
		t.Errorf("dépenses attendues 175000, obtenues %v", rapport.ExpensesXOF)
	}
	if rapport.MarginXOF != rapport.RevenueXOF-rapport.ExpensesXOF {
		t.Errorf("marge incohérente: %v", rapport.MarginXOF)
	}
	if len(rapport.ExpensesByCategory) != 2 || rapport.ExpensesByCategory[0].Category != "loyer" {
		t.Errorf("répartition des dépenses inattendue: %+v", rapport.ExpensesByCategory)
	}
	if len(rapport.TopServices) != 2 || rapport.TopServices[0].Label != "Chemise" || rapport.TopServices[0].Count != 3 {
		t.Errorf("top prestations inattendu: %+v", rapport.TopServices)
	}
	if rapport.Consolidated {
		t.Error("la consolidation ne doit pas être active sans la formule BUSINESS")
	}
	if rapport.PlanHint == "" {
		t.Error("une note commerciale est attendue lorsque la consolidation est absente")
	}
	if len(rapport.ByBranch) != 0 {
		t.Errorf("aucune ligne de site attendue sans consolidation: %+v", rapport.ByBranch)
	}
}

// TestSummary_Consolide : la consolidation multi-sites ajoute une ligne par site.
func TestSummary_Consolide(t *testing.T) {
	svc := report.NewService()
	svc.SetSources(
		commandesStub{[]*order.Order{{
			ID: "cmd-1", Status: "delivered", TotalAmount: 5000, CreatedAt: date("2026-10-02T10:00:00Z"),
			Payments: []order.PaymentRecord{{Amount: 5000, CreatedAt: date("2026-10-02T10:00:00Z")}},
		}}},
		depensesStub{[]*expense.Expense{{Category: "divers", AmountXOF: 2000, SpentAt: "2026-10-02"}}},
	)

	rapport, err := svc.Summary(context.Background(), tenant, "2026-10-01", "2026-10-31", true)
	if err != nil {
		t.Fatalf("rapport consolidé: %v", err)
	}
	if !rapport.Consolidated || len(rapport.ByBranch) != 1 {
		t.Fatalf("consolidation attendue: %+v", rapport.ByBranch)
	}
	if rapport.ByBranch[0].RevenueXOF != 5000 || rapport.ByBranch[0].MarginXOF != 3000 {
		t.Errorf("ligne de site incohérente: %+v", rapport.ByBranch[0])
	}
}

// TestSummary_Erreurs : période invalide et absence de source de données.
func TestSummary_Erreurs(t *testing.T) {
	ctx := context.Background()

	svc := report.NewService()
	svc.SetSources(commandesStub{}, depensesStub{})
	if _, err := svc.Summary(ctx, tenant, "2026-13-01", "2026-10-31", false); !errors.Is(err, report.ErrInvalidPeriod) {
		t.Errorf("date illisible : attendu ErrInvalidPeriod, obtenu %v", err)
	}
	if _, err := svc.Summary(ctx, tenant, "2026-10-31", "2026-10-01", false); !errors.Is(err, report.ErrInvalidPeriod) {
		t.Errorf("bornes inversées : attendu ErrInvalidPeriod, obtenu %v", err)
	}

	// Sans base ni services branchés, le rapport ne peut pas être calculé.
	nu := report.NewService()
	if _, err := nu.Summary(ctx, tenant, "", "", false); !errors.Is(err, report.ErrReportUnavailable) {
		t.Errorf("attendu ErrReportUnavailable, obtenu %v", err)
	}
}
