package billing_test

import (
	"context"
	"strings"
	"testing"

	"pressing-os/backend/internal/billing"
)

// compteurFixe simule les consommations mesurées (commandes du mois, comptes,
// grilles, établissements) pour éprouver les plafonds sans PostgreSQL.
type compteurFixe struct {
	commandes    int64
	comptes      int64
	grilles      int64
	etablissemts int64
}

func (c compteurFixe) CountMonthlyOrders(context.Context, string) (int64, error) {
	return c.commandes, nil
}
func (c compteurFixe) CountUsers(context.Context, string) (int64, error) { return c.comptes, nil }
func (c compteurFixe) CountPriceLists(context.Context, string) (int64, error) {
	return c.grilles, nil
}
func (c compteurFixe) CountBranches(context.Context, string) (int64, error) {
	return c.etablissemts, nil
}

// TestFeatureUsage_MultiSites : le plafond multi-sites doit être mesuré sur les
// établissements réels — sans ce compteur, la consommation restait à 0 et le
// quota « branches.multi_site » n'était jamais appliqué (2e site créé en
// formule gratuite, 4e site créé en BUSINESS).
func TestFeatureUsage_MultiSites(t *testing.T) {
	ctx := context.Background()
	const tenant = "tenant-multisites-test"

	svc := billing.NewService()
	svc.SetUsageCounter(compteurFixe{etablissemts: 3})

	usage, err := svc.FeatureUsage(ctx, tenant, billing.FeatureMultiSite)
	if err != nil {
		t.Fatalf("consommation multi-sites: %v", err)
	}
	if usage != 3 {
		t.Fatalf("consommation attendue 3 établissements, obtenue %d", usage)
	}

	// Formule gratuite : 1 site maximum, déjà 3 => refus.
	svc.ForcePlanForTest(tenant, billing.PlanFree, billing.StatusActive)
	err = svc.CheckLimit(ctx, tenant, billing.FeatureMultiSite, usage)
	if err == nil {
		t.Fatal("la création d'un 4e site en formule gratuite devait être refusée")
	}
	if !strings.Contains(err.Error(), "plafond atteint") {
		t.Errorf("message commercial attendu, obtenu: %v", err)
	}

	// BUSINESS : 3 sites maximum, déjà 3 => refus également (plafond atteint).
	svc.ForcePlanForTest(tenant, billing.PlanBusiness, billing.StatusActive)
	if err := svc.CheckLimit(ctx, tenant, billing.FeatureMultiSite, usage); err == nil {
		t.Error("le 4e site d'une formule BUSINESS (plafond 3) devait être refusé")
	}

	// Sous le plafond : 2 sites en BUSINESS => création autorisée.
	svc.SetUsageCounter(compteurFixe{etablissemts: 2})
	if err := svc.CheckLimit(ctx, tenant, billing.FeatureMultiSite, 2); err != nil {
		t.Errorf("le 3e site d'une formule BUSINESS devait être autorisé: %v", err)
	}

	// Les droits effectifs exposent la consommation mesurée (affichage UI).
	resp, err := svc.EffectiveEntitlements(ctx, tenant)
	if err != nil {
		t.Fatalf("droits effectifs: %v", err)
	}
	for _, e := range resp.Entitlements {
		if e.Key == billing.FeatureMultiSite {
			if e.Usage == nil || *e.Usage != 2 {
				t.Errorf("consommation multi-sites exposée = %v, attendue 2", e.Usage)
			}
		}
	}
}

// TestNotificationsAdvanced_ReserveeAuxFormulesPayantes : l'envoi réel des
// notifications clients est un droit booléen — composition et journal restent
// ouverts à tous, seule la transmission est payante (migration 000014).
func TestNotificationsAdvanced_ReserveeAuxFormulesPayantes(t *testing.T) {
	ctx := context.Background()

	lire := func(plan, statut string) bool {
		tenant := "tenant-notif-" + plan + "-" + statut
		svc := billing.NewService()
		svc.ForcePlanForTest(tenant, plan, statut)
		actif, err := svc.IsEnabled(ctx, tenant, billing.FeatureNotificationsAdvanced)
		if err != nil {
			t.Fatalf("IsEnabled(%s/%s): %v", plan, statut, err)
		}
		return actif
	}

	if lire(billing.PlanFree, billing.StatusActive) {
		t.Error("formule gratuite : l'envoi réel des notifications doit être désactivé")
	}
	if !lire(billing.PlanPro, billing.StatusActive) {
		t.Error("formule PRO : l'envoi réel des notifications doit être activé")
	}
	if !lire(billing.PlanBusiness, billing.StatusActive) {
		t.Error("formule BUSINESS : l'envoi réel des notifications doit être activé")
	}
	// L'essai en cours (niveau BUSINESS) débloque la fonctionnalité, comme les
	// autres droits payants : le nouveau pressing la découvre pendant 7 jours.
	if !lire(billing.PlanBusiness, billing.StatusTrialing) {
		t.Error("essai (niveau BUSINESS) : l'envoi réel doit être disponible")
	}
}
