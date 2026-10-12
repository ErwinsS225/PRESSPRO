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

// Test d'intégration PostgreSQL de la grille tarifaire et de la fenêtre de commandes.
//
//	TEST_DATABASE_URL="postgres://..." go test -tags=integration ./internal/billing/...
//
// Il vérifie que : (1) la grille expose les tarifs de la table plans (PRO 10 000,
// BUSINESS 25 000 XOF, tarifs corrigés en phase 8) ; (2) l'abonnement porte sa date de création ;
// (3) la fenêtre de commandes est ancrée sur l'essai puis sur la création, et son
// tarif affiché est celui de la formule souscrite (et non celui de l'essai).
func TestIntegration_CatalogueEtFenetre_DepuisLaBase(t *testing.T) {
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

	svc := billing.NewStore(database.NewStore(db.Pool))

	// 1. Grille : tarifs issus de la base, formules dans l'ordre du tarif.
	plans, err := svc.Catalogue(ctx)
	if err != nil {
		t.Fatalf("grille tarifaire: %v", err)
	}
	prix := map[string]float64{}
	for _, p := range plans {
		if p.PriceMonthlyXOF == nil {
			t.Errorf("%s : tarif absent de la base", p.Code)
			continue
		}
		prix[p.Code] = *p.PriceMonthlyXOF
		if len(p.Entitlements) == 0 {
			t.Errorf("%s : droits absents", p.Code)
		}
	}
	for code, attendu := range map[string]float64{billing.PlanFree: 0, billing.PlanPro: 10000, billing.PlanBusiness: 25000} {
		if got, ok := prix[code]; !ok || got != attendu {
			t.Errorf("tarif %s : attendu %.0f XOF, obtenu %v (présent=%v)", code, attendu, got, ok)
		}
	}

	// 2. Établissement en essai, puis passage en formule PRO (sans essai).
	suffixe := time.Now().Format("150405.000000")
	var tenant string
	if err := db.Pool.QueryRow(ctx,
		`INSERT INTO tenants (name, slug, city, status) VALUES ($1, $2, 'Abidjan', 'active') RETURNING id`,
		"Pressing Catalogue "+suffixe, "catalogue-"+suffixe).Scan(&tenant); err != nil {
		t.Fatalf("création de l'établissement: %v", err)
	}
	defer func() { _ = svc.DeleteTenant(ctx, tenant) }()

	if err := svc.EnsureTrial(ctx, tenant); err != nil {
		t.Fatalf("initialisation de l'essai: %v", err)
	}
	rec, err := svc.GetSubscription(ctx, tenant)
	if err != nil {
		t.Fatalf("lecture de l'abonnement: %v", err)
	}
	if rec.CreatedAt.IsZero() {
		t.Fatal("la date de création de l'abonnement doit être lue depuis la base")
	}
	if rec.TrialStartedAt == nil {
		t.Fatal("l'essai doit démarrer à l'initialisation")
	}

	// Pendant l'essai, la fenêtre est ancrée sur le début d'essai (pas sur la création).
	resp, err := svc.EffectiveEntitlements(ctx, tenant)
	if err != nil {
		t.Fatalf("droits pendant l'essai: %v", err)
	}
	if resp.OrdersPeriod == nil {
		t.Fatal("fenêtre de commandes absente pendant l'essai")
	}
	attendu := billing.FenetreCommandes(rec.TrialStartedAt.UTC(), time.Now().UTC())
	if !resp.OrdersPeriod.Start.Equal(attendu.Start) {
		t.Errorf("fenêtre pendant l'essai : début attendu %s, obtenu %s", attendu.Start, resp.OrdersPeriod.Start)
	}
	// Tarif affiché : celui de la formule souscrite (FREE pendant l'essai).
	if resp.PriceMonthlyXOF == nil || *resp.PriceMonthlyXOF != 0 {
		t.Errorf("tarif pendant l'essai : attendu 0 (formule FREE souscrite), obtenu %v", resp.PriceMonthlyXOF)
	}

	// Souscription PRO : la fenêtre reste ancrée sur l'essai, le tarif passe à 10 000.
	if _, err := db.Pool.Exec(ctx,
		`UPDATE subscriptions SET plan_id = (SELECT id FROM plans WHERE code = 'PRO'),
		        status = 'ACTIVE', trial_started_at = NULL, trial_ends_at = NULL
		  WHERE tenant_id = $1`, tenant); err != nil {
		t.Fatalf("passage en formule PRO: %v", err)
	}
	resp, err = svc.EffectiveEntitlements(ctx, tenant)
	if err != nil {
		t.Fatalf("droits en formule PRO: %v", err)
	}
	if resp.PriceMonthlyXOF == nil || *resp.PriceMonthlyXOF != 10000 {
		t.Errorf("tarif PRO : attendu 10000, obtenu %v", resp.PriceMonthlyXOF)
	}
	if resp.OrdersPeriod == nil {
		t.Fatal("fenêtre absente en formule PRO")
	}
	// Sans essai, l'ancre devient la date de création de l'abonnement.
	rec, err = svc.GetSubscription(ctx, tenant)
	if err != nil {
		t.Fatalf("relecture de l'abonnement: %v", err)
	}
	attendu = billing.FenetreCommandes(rec.CreatedAt.UTC(), time.Now().UTC())
	if !resp.OrdersPeriod.Start.Equal(attendu.Start) {
		t.Errorf("fenêtre en PRO : début attendu %s (création), obtenu %s", attendu.Start, resp.OrdersPeriod.Start)
	}
}
