//go:build integration

package billing_test

import (
	"context"
	"os"
	"testing"
	"time"

	"pressing-os/backend/internal/billing"
	"pressing-os/backend/internal/database"
)

// Test d'intégration PostgreSQL de la suppression d'un établissement.
//
// Exécution :
//
//	TEST_DATABASE_URL="postgres://pressing_app:...@localhost:5432/pressing_os?sslmode=disable" \
//	  go test -tags=integration ./internal/billing/...
//
// Il vérifie que la suppression par l'éditeur retire l'établissement ET toutes
// ses données (comptes, clients, catalogue, commandes, lignes de commande,
// paiements, abonnement), sans toucher aux autres établissements : les tables
// « orders » et « payments » ne cascadent pas depuis « tenants », la suppression
// doit donc les traiter explicitement, dans la même transaction.
func TestIntegration_DeleteTenant_RetireToutesLesDonnees(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL non défini : test d'intégration ignoré")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	db, err := database.ConnectPool(ctx, dsn)
	if err != nil {
		t.Fatalf("connexion PostgreSQL: %v", err)
	}
	defer db.Close()
	if err := db.Ping(ctx); err != nil {
		t.Fatalf("base injoignable: %v", err)
	}

	suffixe := time.Now().Format("20060102150405.000000")

	// Un établissement « à supprimer » garni de données, un témoin à préserver.
	condamne := creerEtablissementGarni(t, ctx, db, "supprime-"+suffixe, suffixe+"-A")
	temoin := creerEtablissementGarni(t, ctx, db, "temoin-"+suffixe, suffixe+"-B")

	svc := billing.NewStore(database.NewStore(db.Pool))
	if err := svc.DeleteTenant(ctx, condamne); err != nil {
		t.Fatalf("suppression de l'établissement: %v", err)
	}
	// Nettoyage du témoin quoi qu'il arrive.
	defer func() { _ = svc.DeleteTenant(ctx, temoin) }()

	// 1. Toutes les lignes de l'établissement supprimé ont disparu.
	for _, table := range []string{
		"tenants", "users", "customers", "services", "orders", "order_items",
		"payments", "subscriptions", "subscription_events", "branches",
		"tenant_memberships", "branch_memberships", "expenses", "notifications",
		"order_sequences", "price_lists", "price_list_versions",
	} {
		var reste int64
		requete := "SELECT COUNT(*) FROM " + table + " WHERE tenant_id = $1"
		if table == "tenants" {
			requete = "SELECT COUNT(*) FROM tenants WHERE id = $1"
		}
		if table == "order_items" {
			requete = `SELECT COUNT(*) FROM order_items i
				JOIN orders o ON o.id = i.order_id WHERE o.tenant_id = $1`
		}
		if err := db.Pool.QueryRow(ctx, requete, condamne).Scan(&reste); err != nil {
			t.Fatalf("comptage %s: %v", table, err)
		}
		if reste != 0 {
			t.Errorf("%s : %d ligne(s) subsistante(s) après suppression", table, reste)
		}
	}

	// 2. L'établissement témoin est intact (isolation multi-tenant).
	var commandesTemoin int64
	if err := db.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM orders WHERE tenant_id = $1`, temoin).Scan(&commandesTemoin); err != nil {
		t.Fatalf("comptage des commandes du témoin: %v", err)
	}
	if commandesTemoin != 1 {
		t.Errorf("les données du témoin doivent rester intactes : %d commande(s)", commandesTemoin)
	}

	// 3. L'annuaire de l'éditeur ne contient plus l'établissement supprimé.
	annuaire, err := svc.TenantsOverview(ctx)
	if err != nil {
		t.Fatalf("annuaire: %v", err)
	}
	for _, occupant := range annuaire {
		if occupant.TenantID == condamne {
			t.Errorf("l'établissement supprimé figure encore dans l'annuaire")
		}
	}

	// 4. Une seconde suppression (ou un identifiant non UUID) est refusée
	// explicitement : aucune suppression silencieuse.
	if err := svc.DeleteTenant(ctx, condamne); err != billing.ErrTenantNotFound {
		t.Errorf("seconde suppression : attendu ErrTenantNotFound, obtenu %v", err)
	}
	if err := svc.DeleteTenant(ctx, "entreprise-fictive"); err != billing.ErrTenantNotFound {
		t.Errorf("identifiant non UUID : attendu ErrTenantNotFound, obtenu %v", err)
	}
}

// creerEtablissementGarni insère un établissement complet (compte propriétaire,
// client, prestation, commande, ligne de commande, paiement, abonnement et
// journal d'abonnement) et retourne son identifiant.
func creerEtablissementGarni(t *testing.T, ctx context.Context, db *database.DB, slug, marqueur string) string {
	t.Helper()

	var tenant string
	if err := db.Pool.QueryRow(ctx,
		`INSERT INTO tenants (name, slug, city, status)
		 VALUES ($1, $2, 'Abidjan', 'active') RETURNING id`,
		"Pressing Intégration "+marqueur, slug).Scan(&tenant); err != nil {
		t.Fatalf("création de l'établissement %s: %v", slug, err)
	}

	if _, err := db.Pool.Exec(ctx,
		`INSERT INTO users (tenant_id, email, password_hash, full_name, role, status)
		 VALUES ($1, $2, 'hash-integration', 'Propriétaire Intégration', 'owner', 'active')`,
		tenant, "proprietaire."+slug+"@test.ci"); err != nil {
		t.Fatalf("création du compte propriétaire: %v", err)
	}

	var client string
	if err := db.Pool.QueryRow(ctx,
		`INSERT INTO customers (tenant_id, first_name, last_name, phone)
		 VALUES ($1, 'Client', 'Intégration', $2) RETURNING id`,
		tenant, "+225 07 00 00 "+marqueur).Scan(&client); err != nil {
		t.Fatalf("création du client: %v", err)
	}

	var prestation string
	if err := db.Pool.QueryRow(ctx,
		`INSERT INTO services (tenant_id, name, category, unit_price, unit, estimated_hours)
		 VALUES ($1, 'Chemise', 'Lavage & Repassage', 1000, 'pièce', 24) RETURNING id`,
		tenant).Scan(&prestation); err != nil {
		t.Fatalf("création de la prestation: %v", err)
	}

	var commande string
	if err := db.Pool.QueryRow(ctx,
		`INSERT INTO orders (tenant_id, customer_id, order_number, status, payment_status, total_amount, paid_amount)
		 VALUES ($1, $2, 'CMD-INT-001', 'received', 'unpaid', 1000, 0) RETURNING id`,
		tenant, client).Scan(&commande); err != nil {
		t.Fatalf("création de la commande: %v", err)
	}

	if _, err := db.Pool.Exec(ctx,
		`INSERT INTO order_items (order_id, service_id, item_label, quantity, unit_price, subtotal)
		 VALUES ($1, $2, 'Chemise', 1, 1000, 1000)`, commande, prestation); err != nil {
		t.Fatalf("création de la ligne de commande: %v", err)
	}

	if _, err := db.Pool.Exec(ctx,
		`INSERT INTO payments (tenant_id, order_id, amount, payment_method)
		 VALUES ($1, $2, 1000, 'cash')`, tenant, commande); err != nil {
		t.Fatalf("création du paiement: %v", err)
	}

	if _, err := db.Pool.Exec(ctx,
		`INSERT INTO subscriptions (tenant_id, plan_id, status, trial_started_at, trial_ends_at)
		 SELECT $1, id, 'TRIALING', NOW(), NOW() + INTERVAL '7 days' FROM plans WHERE code = 'FREE'`,
		tenant); err != nil {
		t.Fatalf("création de l'abonnement: %v", err)
	}

	if _, err := db.Pool.Exec(ctx,
		`INSERT INTO subscription_events (tenant_id, from_status, to_status, reason)
		 VALUES ($1, 'TRIALING', 'TRIALING', 'inscription (test d''intégration)')`, tenant); err != nil {
		t.Fatalf("journalisation de l'abonnement: %v", err)
	}

	return tenant
}
