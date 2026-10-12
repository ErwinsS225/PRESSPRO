package billing

import (
	"context"
	"testing"
	"time"
)

// Phase 6 — diagnostic de persistance des prix (aucun prix modifié).
//
// Ces tests de caractérisation reproduisent deux défauts en mode mémoire, où aucun
// tarif n'est lu en base. Ils assertent le comportement ACTUEL, défectueux, pour que
// la reproduction soit consignée. Quand la correction sera décidée (affichage absent
// plutôt que 0 : voir docs/subscription-phase6-prix.md), inverser l'assertion.

// TestPrixDiagnostic_MemoireMRRVautZeroPourUnAbonnementPayant : R-P6-2.
// Un abonnement PRO actif en mémoire n'a pas de tarif (aucun catalogue en mémoire),
// donc le MRR de la plateforme vaut 0 alors qu'un abonnement payant existe.
func TestPrixDiagnostic_MemoireMRRVautZeroPourUnAbonnementPayant(t *testing.T) {
	ctx := context.Background()
	s := memoireAvecAbonnement("tenant-diag-mrr", &SubscriptionRecord{
		PlanCode: PlanPro, Status: StatusActive, PlanName: "Pro", CreatedAt: time.Now().UTC(),
	})

	stats, err := s.AdminStats(ctx)
	if err != nil {
		t.Fatalf("indicateurs: %v", err)
	}
	if stats.ActiveSubscriptions != 1 {
		t.Fatalf("un abonnement actif attendu, obtenu %d", stats.ActiveSubscriptions)
	}
	// Défaut actuel : 0 est présenté comme un MRR réel. Attendu après correction : absence.
	if stats.MRR != 0 {
		t.Errorf("MRR mémoire = %v ; caractérisation attend 0 (défaut R-P6-2)", stats.MRR)
	}
}

// TestPrixDiagnostic_MemoireAnnuaireAffiche0Franc : R-P6-1.
// L'annuaire SuperAdmin en mémoire renvoie price_monthly_xof = 0 pour un établissement
// PRO ; l'écran affiche alors « 0 FCFA / mois ».
func TestPrixDiagnostic_MemoireAnnuaireAffiche0Franc(t *testing.T) {
	ctx := context.Background()
	const tenant = "tenant-diag-annuaire"
	s := memoireAvecAbonnement(tenant, &SubscriptionRecord{
		PlanCode: PlanPro, Status: StatusActive, PlanName: "Pro", CreatedAt: time.Now().UTC(),
	})

	overview, err := s.TenantsOverview(ctx)
	if err != nil {
		t.Fatalf("annuaire: %v", err)
	}
	var trouve bool
	for _, o := range overview {
		if o.TenantID != tenant {
			continue
		}
		trouve = true
		if o.PlanCode != PlanPro {
			t.Fatalf("formule attendue PRO, obtenue %s", o.PlanCode)
		}
		// Défaut actuel : 0 au lieu d'un tarif absent. Attendu après correction : absence.
		if o.PriceMonthlyXOF != 0 {
			t.Errorf("tarif annuaire mémoire = %v ; caractérisation attend 0 (défaut R-P6-1)", o.PriceMonthlyXOF)
		}
	}
	if !trouve {
		t.Fatal("établissement absent de l'annuaire mémoire")
	}
}
