//go:build integration

package report_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pressing-os/backend/internal/branch"
	"pressing-os/backend/internal/database"
	"pressing-os/backend/internal/expense"
	"pressing-os/backend/internal/report"
)

// Test d'intégration PostgreSQL du triptyque multi-sites / dépenses / rapports.
//
// Exécution (migrations 000011 et 000012 appliquées) :
//
//	TEST_DATABASE_URL="postgres://pressing_app:...@localhost:5432/pressing_os?sslmode=disable" \
//	  go test -tags=integration -v ./internal/report/...
//
// Il vérifie : l'unicité métier des noms d'établissement (casse/espaces), la
// protection du site principal, la persistance des dépenses (filtres inclus),
// et les agrégats réels du rapport d'activité (paiements encaissés, dépenses,
// marge, consolidation par établissement).
func TestIntegration_MultiSitesDepensesRapports(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL non défini : test d'intégration ignoré")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connexion PostgreSQL: %v", err)
	}
	defer pool.Close()

	// 1. Pressing de test isolé (supprimé en fin de test).
	tenantID := creerPressing(t, ctx, pool)
	defer supprimerPressing(t, ctx, pool, tenantID)

	db := database.NewStore(pool)

	// 2. Établissements : site principal (posé par l'inscription) + second site.
	serviceBranches := branch.NewStore(db)
	branches, err := serviceBranches.GetBranches(ctx, tenantID)
	if err != nil || len(branches) != 1 || !branches[0].IsPrimary {
		t.Fatalf("site principal attendu: %v (%d)", err, len(branches))
	}
	principal := branches[0]

	second, err := serviceBranches.CreateBranch(ctx, tenantID, branch.CreateBranchRequest{
		Name: "Site Yopougon", City: "Abidjan", Phone: "+225 07 11 22 33 44",
	})
	if err != nil {
		t.Fatalf("création du second site: %v", err)
	}
	if second.IsPrimary || second.Status != branch.StatusActive {
		t.Errorf("un site créé après l'inscription est secondaire et actif: %+v", second)
	}

	// Unicité métier : casse et espaces ne créent pas de doublon en base.
	if _, err := serviceBranches.CreateBranch(ctx, tenantID, branch.CreateBranchRequest{Name: "  site   yopougon "}); !errors.Is(err, branch.ErrNameExists) {
		t.Errorf("attendu ErrNameExists (doublon insensible casse/espaces), obtenu %v", err)
	}

	// Le site principal ne peut pas être suspendu ni archivé.
	archive := branch.StatusArchived
	if _, err := serviceBranches.UpdateBranch(ctx, tenantID, principal.ID, branch.UpdateBranchRequest{Status: &archive}); !errors.Is(err, branch.ErrPrimaryProtected) {
		t.Errorf("attendu ErrPrimaryProtected, obtenu %v", err)
	}

	// 3. Dépenses : écriture, filtres et suppression.
	serviceDepenses := expense.NewStore(db)
	loyer, err := serviceDepenses.CreateExpense(ctx, tenantID, "", expense.CreateExpenseRequest{
		Label: "Loyer octobre", AmountXOF: 150000, Category: "loyer", SpentAt: "2026-10-01",
	})
	if err != nil {
		t.Fatalf("création de la dépense: %v", err)
	}
	if _, err := serviceDepenses.CreateExpense(ctx, tenantID, "", expense.CreateExpenseRequest{
		Label: "Lessive", AmountXOF: 25000, Category: "matieres_premieres",
		SpentAt: "2026-10-03", BranchID: second.ID,
	}); err != nil {
		t.Fatalf("création de la dépense (site 2): %v", err)
	}
	if _, err := serviceDepenses.CreateExpense(ctx, tenantID, "", expense.CreateExpenseRequest{
		Label: "Loyer septembre", AmountXOF: 150000, Category: "loyer", SpentAt: "2026-09-01",
	}); err != nil {
		t.Fatalf("création de la dépense (septembre): %v", err)
	}

	octobre, err := serviceDepenses.GetExpenses(ctx, tenantID, expense.Filter{From: "2026-10-01", To: "2026-10-31"})
	if err != nil {
		t.Fatalf("dépenses d'octobre: %v", err)
	}
	if len(octobre) != 2 {
		t.Errorf("2 dépenses attendues en octobre, obtenues %d", len(octobre))
	}
	if total := expense.Total(octobre); total != 175000 {
		t.Errorf("total d'octobre attendu 175000, obtenu %v", total)
	}
	site2, _ := serviceDepenses.GetExpenses(ctx, tenantID, expense.Filter{BranchID: second.ID})
	if len(site2) != 1 || site2[0].Label != "Lessive" {
		t.Errorf("filtre par établissement inopérant: %+v", site2)
	}

	// 4. Activité encaissée : un client, une prestation, une commande au site 2.
	client, prestation := creerActivite(t, ctx, pool, tenantID, second.ID)

	// 5. Rapport d'activité (agrégats SQL) sans consolidation.
	serviceRapports := report.NewStore(db)
	rapport, err := serviceRapports.Summary(ctx, tenantID, "2026-10-01", "2026-10-31", false)
	if err != nil {
		t.Fatalf("rapport d'activité: %v", err)
	}
	if rapport.DataSource != "postgresql" {
		t.Errorf("source attendue postgresql, obtenue %q", rapport.DataSource)
	}
	if rapport.RevenueXOF != 6000 {
		t.Errorf("chiffre d'affaires encaissé attendu 6000, obtenu %v", rapport.RevenueXOF)
	}
	if rapport.OrdersCount != 1 || rapport.AverageTicketXOF != 6000 {
		t.Errorf("commandes/panier moyen inattendus: %d / %v", rapport.OrdersCount, rapport.AverageTicketXOF)
	}
	if rapport.ExpensesXOF != 175000 || rapport.MarginXOF != 6000-175000 {
		t.Errorf("dépenses/marge inattendues: %v / %v", rapport.ExpensesXOF, rapport.MarginXOF)
	}
	if len(rapport.ExpensesByCategory) == 0 || rapport.ExpensesByCategory[0].Category != "loyer" {
		t.Errorf("répartition des dépenses inattendue: %+v", rapport.ExpensesByCategory)
	}
	if len(rapport.TopServices) != 1 || rapport.TopServices[0].Label != "Chemise" || rapport.TopServices[0].Count != 2 {
		t.Errorf("top prestations inattendu: %+v", rapport.TopServices)
	}
	if rapport.Consolidated || len(rapport.ByBranch) != 0 || rapport.PlanHint == "" {
		t.Errorf("sans formule BUSINESS : pas de consolidation, mais une note commerciale: %+v", rapport)
	}

	// 6. Consolidation multi-sites : chaque site porte son activité.
	consolide, err := serviceRapports.Summary(ctx, tenantID, "2026-10-01", "2026-10-31", true)
	if err != nil {
		t.Fatalf("rapport consolidé: %v", err)
	}
	if !consolide.Consolidated || len(consolide.ByBranch) != 2 {
		t.Fatalf("2 établissements attendus dans la consolidation: %+v", consolide.ByBranch)
	}
	yopougon := consolide.ByBranch[1]
	if yopougon.BranchID != second.ID || yopougon.OrdersCount != 1 || yopougon.RevenueXOF != 6000 {
		t.Errorf("ligne du site secondaire incohérente: %+v", yopougon)
	}

	// 7. Isolation : un autre pressing ne voit rien de ce jeu de données.
	autre := creerPressing(t, ctx, pool)
	defer supprimerPressing(t, ctx, pool, autre)
	if liste, _ := serviceBranches.GetBranches(ctx, autre); len(liste) != 1 {
		t.Errorf("le nouveau pressing ne doit voir que son site principal (%d)", len(liste))
	}
	if depenses, _ := serviceDepenses.GetExpenses(ctx, autre, expense.Filter{}); len(depenses) != 0 {
		t.Errorf("fuite inter-tenant sur les dépenses: %d", len(depenses))
	}
	if _, err := serviceRapports.Summary(ctx, autre, "2026-10-01", "2026-10-31", false); err != nil {
		t.Errorf("rapport d'un pressing vierge: %v", err)
	}

	// 8. Suppression d'une dépense (et refus si elle appartient à un autre pressing).
	if err := serviceDepenses.DeleteExpense(ctx, tenantID, loyer.ID); err != nil {
		t.Fatalf("suppression de la dépense: %v", err)
	}
	if err := serviceDepenses.DeleteExpense(ctx, autre, loyer.ID); !errors.Is(err, expense.ErrExpenseNotFound) {
		t.Errorf("attendu ErrExpenseNotFound pour un autre pressing, obtenu %v", err)
	}
	_ = client
	_ = prestation
}

// creerPressing insère un pressing de test avec son site principal.
func creerPressing(t *testing.T, ctx context.Context, pool *pgxpool.Pool) string {
	t.Helper()
	suffixe := fmt.Sprintf("%d", time.Now().UnixNano())
	var tenantID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO tenants (name, slug) VALUES ($1, $2) RETURNING id`,
		"Pressing Intégration "+suffixe, "integration-"+suffixe).Scan(&tenantID); err != nil {
		t.Fatalf("création du pressing de test: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO branches (tenant_id, name, is_primary, city) VALUES ($1, 'Établissement principal', true, 'Abidjan')`,
		tenantID); err != nil {
		t.Fatalf("création du site principal: %v", err)
	}
	return tenantID
}

// supprimerPressing purge le jeu de données de test (ordre imposé par les clés
// étrangères : paiements et lignes avant commandes, puis l'organisation).
func supprimerPressing(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID string) {
	t.Helper()
	for _, req := range []string{
		`DELETE FROM payments WHERE tenant_id = $1`,
		`DELETE FROM order_items WHERE order_id IN (SELECT id FROM orders WHERE tenant_id = $1)`,
		`DELETE FROM orders WHERE tenant_id = $1`,
		`DELETE FROM expenses WHERE tenant_id = $1`,
		`DELETE FROM services WHERE tenant_id = $1`,
		`DELETE FROM customers WHERE tenant_id = $1`,
		`DELETE FROM branch_memberships WHERE tenant_id = $1`,
		`DELETE FROM tenant_memberships WHERE tenant_id = $1`,
		`DELETE FROM branches WHERE tenant_id = $1`,
		`DELETE FROM tenants WHERE id = $1`,
	} {
		if _, err := pool.Exec(ctx, req, tenantID); err != nil {
			t.Fatalf("purge (%s): %v", req, err)
		}
	}
}

// creerActivite crée client, prestation, commande et paiement sur un site.
func creerActivite(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, branchID string) (string, string) {
	t.Helper()
	var clientID, serviceID, commandeID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO customers (tenant_id, first_name, last_name, phone) VALUES ($1, 'Awa', 'Koné', $2) RETURNING id`,
		tenantID, "+225 05 99 88 77 66").Scan(&clientID); err != nil {
		t.Fatalf("création du client: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO services (tenant_id, name, category, unit_price, unit) VALUES ($1, 'Chemise', 'Lavage & Repassage', 3000, 'pièce') RETURNING id`,
		tenantID).Scan(&serviceID); err != nil {
		t.Fatalf("création de la prestation: %v", err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO orders (tenant_id, customer_id, branch_id, order_number, status, payment_status, total_amount, paid_amount, created_at)
		VALUES ($1, $2, $3, 'CMD-INTEGRATION-1', 'delivered', 'paid', 6000, 6000, TIMESTAMPTZ '2026-10-05 10:00:00+00')
		RETURNING id`, tenantID, clientID, branchID).Scan(&commandeID); err != nil {
		t.Fatalf("création de la commande: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO order_items (order_id, service_id, item_label, quantity, unit_price, subtotal)
		VALUES ($1, $2, 'Chemise', 2, 3000, 6000)`, commandeID, serviceID); err != nil {
		t.Fatalf("création de la ligne de commande: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO payments (tenant_id, order_id, amount, payment_method, created_at)
		VALUES ($1, $2, 6000, 'cash', TIMESTAMPTZ '2026-10-05 10:05:00+00')`, tenantID, commandeID); err != nil {
		t.Fatalf("enregistrement du paiement: %v", err)
	}
	return clientID, serviceID
}
