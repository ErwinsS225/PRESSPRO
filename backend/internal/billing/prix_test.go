package billing

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

// Phase 6 → 8 — tarifs : correction des défauts diagnostiqués en phase 6.
//
// Les tests de caractérisation R-P6-1 et R-P6-2 (annuaire et MRR à 0 en mémoire) ont été
// inversés : en mémoire, aucun tarif n'est stocké, donc l'annuaire et le MRR sont absents
// (null), jamais 0 inventé.

// TestPrix_MemoireMRRAbsent : R-P6-2 corrigé. Un abonnement payant en mémoire n'a pas de
// tarif connu : le MRR est absent, et non égal à 0.
func TestPrix_MemoireMRRAbsent(t *testing.T) {
	ctx := context.Background()
	s := memoireAvecAbonnement("tenant-prix-mrr", &SubscriptionRecord{
		PlanCode: PlanPro, Status: StatusActive, PlanName: "Pro", CreatedAt: time.Now().UTC(),
	})

	stats, err := s.AdminStats(ctx)
	if err != nil {
		t.Fatalf("indicateurs: %v", err)
	}
	if stats.ActiveSubscriptions != 1 {
		t.Fatalf("un abonnement actif attendu, obtenu %d", stats.ActiveSubscriptions)
	}
	if stats.MRR != nil {
		t.Errorf("MRR mémoire = %v ; attendu absent (aucun tarif n'est stocké en mémoire)", *stats.MRR)
	}
}

// TestPrix_MemoireAnnuaireSansTarif : R-P6-1 corrigé. L'annuaire ne fabrique pas de tarif.
func TestPrix_MemoireAnnuaireSansTarif(t *testing.T) {
	ctx := context.Background()
	const tenant = "tenant-prix-annuaire"
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
		if o.PriceMonthlyXOF != nil {
			t.Errorf("tarif annuaire mémoire = %v ; attendu absent", *o.PriceMonthlyXOF)
		}
	}
	if !trouve {
		t.Fatal("établissement absent de l'annuaire mémoire")
	}
}

// TestSetPlanPrice_MemoireRefusee : sans base, la modification est refusée (503 côté HTTP),
// jamais simulée.
func TestSetPlanPrice_MemoireRefusee(t *testing.T) {
	err := NewService().SetPlanPrice(context.Background(), PlanPro, 10000, "", "")
	if !errors.Is(err, ErrCataloguePrixIndisponible) {
		t.Fatalf("mode mémoire : ErrCataloguePrixIndisponible attendu, obtenu %v", err)
	}
}

// TestSetPlanPrice_ReglesDeValidation : les règles sont vérifiées avant tout accès au
// stockage, donc elles s'appliquent aussi en mode mémoire.
func TestSetPlanPrice_ReglesDeValidation(t *testing.T) {
	ctx := context.Background()
	s := NewService()
	cas := map[string]struct {
		code, motif string
		prix        float64
	}{
		"formule gratuite":     {PlanFree, "", 500},
		"décimale":             {PlanPro, "", 10000.5},
		"négatif":              {PlanPro, "", -1},
		"au-dessus du maximum": {PlanBusiness, "", PrixMaximumXOF + 1},
		"NaN":                  {PlanPro, "", math.NaN()},
		"infini":               {PlanPro, "", math.Inf(1)},
		"motif trop long":      {PlanPro, strings.Repeat("a", motifPrixMaxCaracteres+1), 10000},
	}
	for nom, c := range cas {
		err := s.SetPlanPrice(ctx, c.code, c.prix, "", c.motif)
		if !errors.Is(err, ErrPrixInvalide) {
			t.Errorf("%s : ErrPrixInvalide attendu, obtenu %v", nom, err)
		}
	}
}

// TestSetPlanPrice_BornesAcceptees : 0 et le maximum sont des montants valides ; sans base,
// ils atteignent le contrôle de disponibilité.
func TestSetPlanPrice_BornesAcceptees(t *testing.T) {
	ctx := context.Background()
	s := NewService()
	for _, prix := range []float64{0, 10000, PrixMaximumXOF} {
		if err := s.SetPlanPrice(ctx, PlanPro, prix, "", ""); !errors.Is(err, ErrCataloguePrixIndisponible) {
			t.Errorf("montant %v : ErrCataloguePrixIndisponible attendu (validé), obtenu %v", prix, err)
		}
	}
}
