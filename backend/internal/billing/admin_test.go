package billing_test

import (
	"context"
	"strings"
	"testing"

	"pressing-os/backend/internal/billing"
)

// TestAdminStats_MemoryFallback : sans base connectée, les indicateurs sont
// calculés sur les abonnements en mémoire et annoncés comme tels.
func TestAdminStats_MemoryFallback(t *testing.T) {
	svc := billing.NewService()
	stats, err := svc.AdminStats(context.Background())
	if err != nil {
		t.Fatalf("statistiques: %v", err)
	}
	if stats.DataSource != "memoire" {
		t.Errorf("source attendue « memoire », obtenue %q", stats.DataSource)
	}
	if stats.TotalPressings < 1 {
		t.Errorf("au moins l'établissement de démonstration doit être compté, obtenu %d", stats.TotalPressings)
	}
	if stats.TrialingCount != 1 {
		t.Errorf("essai actif attendu : 1, obtenu %d", stats.TrialingCount)
	}
	if stats.ByStatus[billing.StatusTrialing] != 1 {
		t.Errorf("répartition par statut incorrecte : %v", stats.ByStatus)
	}
	if stats.ByPlan[billing.PlanBusiness] != 1 {
		t.Errorf("l'essai doit être compté au niveau BUSINESS : %v", stats.ByPlan)
	}
}

// TestSetTenantStatusEtSuspension : cycle complet de décision de l'éditeur.
func TestSetTenantStatusEtSuspension(t *testing.T) {
	svc := billing.NewService()
	ctx := context.Background()
	tenant := "a0000000-0000-0000-0000-000000000001"

	if svc.IsTenantSuspended(ctx, tenant) {
		t.Fatal("l'établissement de démonstration ne doit pas être suspendu au départ")
	}
	if err := svc.SetTenantStatus(ctx, tenant, billing.StatusSuspended, "usr-superadmin-000", "test"); err != nil {
		t.Fatalf("suspension: %v", err)
	}
	if !svc.IsTenantSuspended(ctx, tenant) {
		t.Error("l'établissement doit être suspendu après la décision de l'éditeur")
	}

	// Réactivation : l'accès est rendu.
	if err := svc.SetTenantStatus(ctx, tenant, billing.StatusActive, "usr-superadmin-000", "régularisation"); err != nil {
		t.Fatalf("réactivation: %v", err)
	}
	if svc.IsTenantSuspended(ctx, tenant) {
		t.Error("l'établissement ne doit plus être suspendu")
	}

	// Statut inconnu : refus explicite (jamais d'écriture silencieuse).
	if err := svc.SetTenantStatus(ctx, tenant, "PAS_UN_STATUT", "", ""); err == nil {
		t.Error("un statut invalide doit être refusé")
	} else if !strings.Contains(err.Error(), "invalide") {
		t.Errorf("message d'erreur inattendu : %v", err)
	}

	// Établissement inconnu : erreur métier dédiée (404 côté API).
	if err := svc.SetTenantStatus(ctx, "tenant-inexistant", billing.StatusSuspended, "", ""); err != billing.ErrTenantSubscriptionNotFound {
		t.Errorf("attendu ErrTenantSubscriptionNotFound, obtenu %v", err)
	}
}

// TestTenantsOverview_MemoryFallback : annuaire minimal en mode démonstration.
func TestTenantsOverview_MemoryFallback(t *testing.T) {
	svc := billing.NewService()
	tenants, err := svc.TenantsOverview(context.Background())
	if err != nil {
		t.Fatalf("annuaire: %v", err)
	}
	if len(tenants) != 1 {
		t.Fatalf("annuaire mémoire attendu : 1 établissement, obtenu %d", len(tenants))
	}
	if !tenants[0].Actif {
		t.Error("l'établissement de démonstration doit être actif")
	}
}
