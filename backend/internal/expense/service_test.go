package expense_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"pressing-os/backend/internal/expense"
)

const tenant = "a0000000-0000-0000-0000-000000000001"

func service(t *testing.T) *expense.Service {
	t.Helper()
	return expense.NewService()
}

// TestCreateExpense_Validation : contrôles avant écriture + valeurs par défaut.
func TestCreateExpense_Validation(t *testing.T) {
	ctx := context.Background()
	svc := service(t)

	// 1. Création nominale : catégorie et date par défaut.
	e, err := svc.CreateExpense(ctx, tenant, "usr-gerant-001", expense.CreateExpenseRequest{
		Label: "  Lessive en poudre  ", AmountXOF: 25000,
	})
	if err != nil {
		t.Fatalf("création d'une dépense: %v", err)
	}
	if e.Label != "Lessive en poudre" {
		t.Errorf("libellé normalisé attendu, obtenu %q", e.Label)
	}
	if e.Category != "divers" {
		t.Errorf("catégorie par défaut « divers » attendue, obtenue %q", e.Category)
	}
	if e.SpentAt != time.Now().UTC().Format("2006-01-02") {
		t.Errorf("date par défaut attendue au jour courant, obtenue %q", e.SpentAt)
	}
	if e.CreatedBy != "usr-gerant-001" {
		t.Errorf("l'auteur de la dépense doit être conservé, obtenu %q", e.CreatedBy)
	}

	// 2. Cas de refus.
	refus := []struct {
		nom string
		req expense.CreateExpenseRequest
	}{
		{"libellé vide", expense.CreateExpenseRequest{AmountXOF: 1000}},
		{"montant nul", expense.CreateExpenseRequest{Label: "Loyer", AmountXOF: 0}},
		{"montant négatif", expense.CreateExpenseRequest{Label: "Loyer", AmountXOF: -500}},
		{"catégorie inconnue", expense.CreateExpenseRequest{Label: "Loyer", AmountXOF: 1000, Category: "vacances"}},
		{"date invalide", expense.CreateExpenseRequest{Label: "Loyer", AmountXOF: 1000, SpentAt: "12/10/2026"}},
	}
	for _, c := range refus {
		t.Run(c.nom, func(t *testing.T) {
			if _, err := svc.CreateExpense(ctx, tenant, "usr-gerant-001", c.req); err == nil {
				t.Errorf("%s : une erreur était attendue", c.nom)
			}
		})
	}
	if _, err := svc.CreateExpense(ctx, tenant, "u", expense.CreateExpenseRequest{Label: "Loyer", AmountXOF: 0}); !errors.Is(err, expense.ErrInvalidAmount) {
		t.Errorf("attendu ErrInvalidAmount, obtenu %v", err)
	}
	if _, err := svc.CreateExpense(ctx, tenant, "u", expense.CreateExpenseRequest{Label: "Loyer", AmountXOF: 1, Category: "vacances"}); !errors.Is(err, expense.ErrInvalidCategory) {
		t.Errorf("attendu ErrInvalidCategory, obtenu %v", err)
	}
}

// TestGetExpenses_FiltresEtAgregats : période, catégorie et totaux.
func TestGetExpenses_FiltresEtAgregats(t *testing.T) {
	ctx := context.Background()
	svc := service(t)

	jeu := []expense.CreateExpenseRequest{
		{Label: "Loyer octobre", AmountXOF: 150000, Category: "loyer", SpentAt: "2026-10-01"},
		{Label: "Lessive", AmountXOF: 25000, Category: "matieres_premieres", SpentAt: "2026-10-03"},
		{Label: "Loyer septembre", AmountXOF: 150000, Category: "loyer", SpentAt: "2026-09-01"},
		{Label: "Lessive (site 2)", AmountXOF: 10000, Category: "matieres_premieres", SpentAt: "2026-10-05", BranchID: "branch-2"},
	}
	for _, req := range jeu {
		if _, err := svc.CreateExpense(ctx, tenant, "usr-gerant-001", req); err != nil {
			t.Fatalf("création %s: %v", req.Label, err)
		}
	}

	// 1. Période du mois d'octobre : 3 dépenses, 185 000 FCFA.
	octobre, err := svc.GetExpenses(ctx, tenant, expense.Filter{From: "2026-10-01", To: "2026-10-31"})
	if err != nil {
		t.Fatalf("dépenses d'octobre: %v", err)
	}
	if len(octobre) != 3 {
		t.Errorf("3 dépenses attendues en octobre, obtenues %d", len(octobre))
	}
	if total := expense.Total(octobre); total != 185000 {
		t.Errorf("total attendu 185000, obtenu %v", total)
	}

	// 2. Filtre par catégorie puis par établissement.
	matieres, _ := svc.GetExpenses(ctx, tenant, expense.Filter{Category: "matieres_premieres"})
	if len(matieres) != 2 {
		t.Errorf("2 dépenses de matières attendues, obtenues %d", len(matieres))
	}
	site2, _ := svc.GetExpenses(ctx, tenant, expense.Filter{BranchID: "branch-2"})
	if len(site2) != 1 || site2[0].Label != "Lessive (site 2)" {
		t.Errorf("filtre par établissement inopérant: %+v", site2)
	}

	// 3. Agrégats : poste le plus lourd en tête.
	agregats := expense.ParCategorie(octobre)
	if len(agregats) != 2 || agregats[0].Category != "loyer" || agregats[0].TotalXOF != 150000 || agregats[0].Count != 1 {
		t.Errorf("agrégats par catégorie inattendus: %+v", agregats)
	}

	// 4. Isolation : un autre pressing ne voit ni ne modifie ces dépenses.
	autres, err := svc.GetExpenses(ctx, "b62709cf-0fbf-4db2-aa56-d861b7930284", expense.Filter{})
	if err != nil || len(autres) != 0 {
		t.Errorf("fuite inter-tenant: %v (%d dépenses)", err, len(autres))
	}
	if err := svc.DeleteExpense(ctx, "b62709cf-0fbf-4db2-aa56-d861b7930284", octobre[0].ID); !errors.Is(err, expense.ErrExpenseNotFound) {
		t.Errorf("attendu ErrExpenseNotFound pour un autre pressing, obtenu %v", err)
	}

	// 5. Suppression par le propriétaire.
	if err := svc.DeleteExpense(ctx, tenant, octobre[0].ID); err != nil {
		t.Fatalf("suppression: %v", err)
	}
	apres, _ := svc.GetExpenses(ctx, tenant, expense.Filter{From: "2026-10-01", To: "2026-10-31"})
	if len(apres) != 2 {
		t.Errorf("2 dépenses attendues après suppression, obtenues %d", len(apres))
	}
}
