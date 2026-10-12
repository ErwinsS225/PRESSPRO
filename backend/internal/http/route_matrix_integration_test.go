//go:build integration

package http_test

import (
	"context"
	"os"
	"testing"
	"time"

	"pressing-os/backend/internal/auth"
	"pressing-os/backend/internal/billing"
	"pressing-os/backend/internal/branch"
	"pressing-os/backend/internal/catalog"
	"pressing-os/backend/internal/config"
	"pressing-os/backend/internal/customer"
	"pressing-os/backend/internal/database"
	"pressing-os/backend/internal/expense"
	"pressing-os/backend/internal/health"
	apphttp "pressing-os/backend/internal/http"
	"pressing-os/backend/internal/notification"
	"pressing-os/backend/internal/order"
	"pressing-os/backend/internal/pricelist"
	"pressing-os/backend/internal/report"
)

// Matrice des routes en mode PostgreSQL (PLAN phase 4) : même matrice et mêmes
// tests que le mode mémoire, sur les magasins réels (câblage identique à cmd/api).
//
// Exécution :
//
//	TEST_DATABASE_URL="postgres://..." go test -tags=integration -count=1 ./internal/http/...
//
// Sans TEST_DATABASE_URL, le sous-test PostgreSQL est ignoré (pas de faux succès).

func init() {
	fabriques = append(fabriques, fabriqueEnvironnement{nom: "PostgreSQL", construire: nouvelEnvironnementBase})
}

// nouvelEnvironnementBase assemble le routeur adossé à PostgreSQL. Les établissements
// créés pendant le test sont supprimés au nettoyage (DeleteTenant).
func nouvelEnvironnementBase(t *testing.T) *environnement {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL non défini : mode PostgreSQL ignoré")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)

	db, err := database.ConnectPool(ctx, dsn)
	if err != nil {
		t.Fatalf("connexion PostgreSQL: %v", err)
	}
	t.Cleanup(db.Close)

	store := database.NewStore(db.Pool)
	cfg := &config.Config{Port: "8080", ServerEnv: "test", AllowedOrigins: []string{"http://localhost:3000"}}
	jwtManager := auth.NewJWTManager("test-secret-key-1234567890", 2*time.Hour)
	authSvc := auth.NewService(jwtManager)
	authSvc.SetStore(auth.NewStore(store))
	bill := billing.NewStore(store)

	orderSvc := order.NewService()
	orderSvc.SetStore(order.NewStore(store))
	orderSvc.SetQuotaGuard(&billing.QuotaGuard{Billing: bill, Counter: billing.NewDBUsageCounter(store)})
	customerSvc := customer.NewService()
	customerSvc.SetStore(customer.NewStore(store))
	priceSvc := pricelist.NewStore(store)
	catalogSvc := catalog.NewStore(store)
	branchSvc := branch.NewStore(store)
	expenseSvc := expense.NewStore(store)
	notifSvc := notification.NewStore(store)
	notifSvc.SetSourceCommandes(orderSvc)
	notifSvc.SetDroitVerificateur(bill, billing.FeatureNotificationsAdvanced)
	reportSvc := report.NewStore(store)

	router := apphttp.NewRouter(apphttp.ServerDependencies{
		Config:              cfg,
		HealthHandler:       health.NewHandler(db, time.Second),
		AuthHandler:         auth.NewHandler(authSvc),
		JWTManager:          jwtManager,
		OrderHandler:        order.NewHandler(orderSvc),
		CatalogHandler:      catalog.NewHandler(catalogSvc),
		CustomerHandler:     customer.NewHandler(customerSvc),
		PriceListHandler:    pricelist.NewHandler(priceSvc),
		BranchHandler:       branch.NewHandler(branchSvc),
		ExpenseHandler:      expense.NewHandler(expenseSvc),
		ReportHandler:       report.NewHandler(reportSvc, bill),
		NotificationHandler: notification.NewHandler(notifSvc),
		Billing:             bill,
		BillingHandler:      billing.NewHandler(bill),
		MembershipChecker:   apphttp.NewMembershipChecker(db.Pool, authSvc.IsDemoAccount),
	})

	env := &environnement{
		t: t, router: router, jwt: jwtManager, authSvc: authSvc, bill: bill,
		persistant: true,
		formule: func(tenant, plan string) {
			t.Helper()
			// Fixe le plan de la base (abonnement actif, essai effacé) : même état que
			// l'administration de l'éditeur. Aucun paiement n'est simulé.
			if _, err := db.Pool.Exec(ctx, `
				INSERT INTO subscriptions (tenant_id, plan_id, status)
				VALUES ($1, (SELECT id FROM plans WHERE code = $2), 'ACTIVE')
				ON CONFLICT (tenant_id) DO UPDATE
				SET plan_id = EXCLUDED.plan_id, status = 'ACTIVE',
				    trial_started_at = NULL, trial_ends_at = NULL, updated_at = NOW()`,
				tenant, plan); err != nil {
				t.Fatalf("fixation de la formule %s : %v", plan, err)
			}
		},
	}
	t.Cleanup(func() {
		for _, id := range env.crees {
			if err := bill.DeleteTenant(context.Background(), id); err != nil {
				t.Errorf("nettoyage de l'établissement %s : %v", id, err)
			}
		}
	})
	return env
}
