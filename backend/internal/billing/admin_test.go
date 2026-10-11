package billing_test

import (
	"context"
	"strings"
	"testing"

	"pressing-os/backend/internal/billing"
)

// TestAdminStats_SansEntrepriseFictive : sans base connectée et sans
// inscription réelle, l'éditeur ne voit aucun établissement. Aucune entreprise
// de démonstration n'est injectée dans les indicateurs.
func TestAdminStats_SansEntrepriseFictive(t *testing.T) {
	svc := billing.NewService()
	stats, err := svc.AdminStats(context.Background())
	if err != nil {
		t.Fatalf("statistiques: %v", err)
	}
	if stats.DataSource != "memoire" {
		t.Errorf("source attendue « memoire », obtenue %q", stats.DataSource)
	}
	if stats.TotalPressings != 0 {
		t.Errorf("aucun établissement attendu, obtenu %d", stats.TotalPressings)
	}
	if stats.TrialingCount != 0 {
		t.Errorf("aucun essai attendu, obtenu %d", stats.TrialingCount)
	}
	if len(stats.ByStatus) != 0 || len(stats.ByPlan) != 0 {
		t.Errorf("répartitions attendues vides : %v / %v", stats.ByStatus, stats.ByPlan)
	}
}

// TestAdminStats_CompteLesInscriptionsReelles : seules les inscriptions réelles
// (essai 7 jours) alimentent les indicateurs, comptées au niveau BUSINESS.
func TestAdminStats_CompteLesInscriptionsReelles(t *testing.T) {
	svc := billing.NewService()
	ctx := context.Background()
	for _, tenant := range []string{
		"11111111-1111-4111-8111-111111111111",
		"22222222-2222-4222-8222-222222222222",
	} {
		if err := svc.EnsureTrial(ctx, tenant); err != nil {
			t.Fatalf("inscription de l'établissement de test: %v", err)
		}
	}

	stats, err := svc.AdminStats(ctx)
	if err != nil {
		t.Fatalf("statistiques: %v", err)
	}
	if stats.TotalPressings != 2 || stats.TrialingCount != 2 {
		t.Errorf("attendu 2 établissements en essai, obtenu %d (essais %d)", stats.TotalPressings, stats.TrialingCount)
	}
	if stats.ByStatus[billing.StatusTrialing] != 2 {
		t.Errorf("répartition par statut incorrecte : %v", stats.ByStatus)
	}
	if stats.ByPlan[billing.PlanBusiness] != 2 {
		t.Errorf("l'essai doit être compté au niveau BUSINESS : %v", stats.ByPlan)
	}
}

// TestSetTenantStatusEtSuspension : cycle complet de décision de l'éditeur.
func TestSetTenantStatusEtSuspension(t *testing.T) {
	svc := billing.NewService()
	ctx := context.Background()
	tenant := "33333333-3333-4333-8333-333333333333"
	if err := svc.EnsureTrial(ctx, tenant); err != nil {
		t.Fatalf("inscription de l'établissement de test: %v", err)
	}

	if svc.IsTenantSuspended(ctx, tenant) {
		t.Fatal("un établissement nouvellement inscrit ne doit pas être suspendu")
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

// TestTenantsOverview_AucuneEntrepriseFictive : l'annuaire démarre vide et
// n'affiche que les établissements réellement inscrits.
func TestTenantsOverview_AucuneEntrepriseFictive(t *testing.T) {
	svc := billing.NewService()
	ctx := context.Background()

	tenants, err := svc.TenantsOverview(ctx)
	if err != nil {
		t.Fatalf("annuaire: %v", err)
	}
	if len(tenants) != 0 {
		t.Fatalf("annuaire attendu vide, obtenu %d établissement(s)", len(tenants))
	}

	tenant := "44444444-4444-4444-8444-444444444444"
	if err := svc.EnsureTrial(ctx, tenant); err != nil {
		t.Fatalf("inscription de l'établissement de test: %v", err)
	}
	tenants, err = svc.TenantsOverview(ctx)
	if err != nil {
		t.Fatalf("annuaire: %v", err)
	}
	if len(tenants) != 1 || tenants[0].TenantID != tenant {
		t.Fatalf("annuaire attendu : %s, obtenu %+v", tenant, tenants)
	}
	if !tenants[0].Actif {
		t.Error("l'établissement inscrit doit être actif")
	}
}

// TestDeleteTenant_MemoryFallback : la suppression retire l'établissement de
// l'annuaire et de la facturation ; un identifiant inconnu est refusé
// explicitement (aucune suppression silencieuse).
func TestDeleteTenant_MemoryFallback(t *testing.T) {
	svc := billing.NewService()
	ctx := context.Background()
	tenant := "55555555-5555-4555-8555-555555555555"

	if err := svc.DeleteTenant(ctx, tenant); err != billing.ErrTenantNotFound {
		t.Fatalf("suppression d'un établissement inconnu : attendu ErrTenantNotFound, obtenu %v", err)
	}

	if err := svc.EnsureTrial(ctx, tenant); err != nil {
		t.Fatalf("inscription de l'établissement de test: %v", err)
	}
	if err := svc.DeleteTenant(ctx, tenant); err != nil {
		t.Fatalf("suppression: %v", err)
	}

	tenants, err := svc.TenantsOverview(ctx)
	if err != nil {
		t.Fatalf("annuaire: %v", err)
	}
	if len(tenants) != 0 {
		t.Errorf("l'établissement supprimé ne doit plus apparaître : %+v", tenants)
	}
	if err := svc.DeleteTenant(ctx, tenant); err != billing.ErrTenantNotFound {
		t.Errorf("seconde suppression : attendu ErrTenantNotFound, obtenu %v", err)
	}
	if err := svc.SetTenantStatus(ctx, tenant, billing.StatusSuspended, "", ""); err != billing.ErrTenantSubscriptionNotFound {
		t.Errorf("abonnement supprimé : attendu ErrTenantSubscriptionNotFound, obtenu %v", err)
	}
}
