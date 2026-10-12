package billing

import (
	"context"
	"testing"
	"time"
)

// Ancre fixe : tous les calculs de fenêtre sont déterministes (aucune dépendance à l'horloge).
var ancreTest = time.Date(2026, time.January, 1, 10, 0, 0, 0, time.UTC)

const jourFenetre = 24 * time.Hour

// TestFenetreCommandes_Bornes : la fenêtre de 30 jours est ouverte par tranches
// depuis l'ancre (début d'essai ou création), bornes incluses au début et exclues à la fin.
func TestFenetreCommandes_Bornes(t *testing.T) {
	cas := []struct {
		nom          string
		now          time.Time
		debutAttendu time.Time
	}{
		{"instant de l'ancre", ancreTest, ancreTest},
		{"une seconde avant la fin de la 1re fenêtre", ancreTest.Add(30*jourFenetre - time.Second), ancreTest},
		{"fin exacte de la 1re fenêtre : nouvelle fenêtre", ancreTest.Add(30 * jourFenetre), ancreTest.Add(30 * jourFenetre)},
		{"milieu de la 2e fenêtre", ancreTest.Add(45 * jourFenetre), ancreTest.Add(30 * jourFenetre)},
		{"fin exacte de la 2e fenêtre", ancreTest.Add(60 * jourFenetre), ancreTest.Add(60 * jourFenetre)},
		{"instant antérieur à l'ancre : pas de fenêtre négative", ancreTest.Add(-5 * jourFenetre), ancreTest},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			p := FenetreCommandes(ancreTest, c.now)
			if !p.Start.Equal(c.debutAttendu) {
				t.Errorf("début : attendu %s, obtenu %s", c.debutAttendu, p.Start)
			}
			if got := p.End.Sub(p.Start); got != 30*jourFenetre {
				t.Errorf("durée de fenêtre : attendu 30 jours, obtenu %s", got)
			}
		})
	}
}

// TestAncreFenetre : l'essai prime sur la création de l'abonnement ; sans les deux, zéro.
func TestAncreFenetre(t *testing.T) {
	essai := ancreTest.Add(-3 * jourFenetre)
	creation := ancreTest.Add(-90 * jourFenetre)

	if got := ancreFenetre(&SubscriptionRecord{TrialStartedAt: &essai, CreatedAt: creation}); !got.Equal(essai) {
		t.Errorf("l'essai doit servir d'ancre, obtenu %s", got)
	}
	if got := ancreFenetre(&SubscriptionRecord{CreatedAt: creation}); !got.Equal(creation) {
		t.Errorf("sans essai, la création sert d'ancre, obtenu %s", got)
	}
	if got := ancreFenetre(&SubscriptionRecord{}); !got.IsZero() {
		t.Errorf("ancre inconnue attendue zéro, obtenu %s", got)
	}
	if got := ancreFenetre(nil); !got.IsZero() {
		t.Errorf("enregistrement absent : ancre zéro attendue, obtenu %s", got)
	}
}

// memoireAvecAbonnement prépare un service mémoire dont le tenant a une ancre connue.
func memoireAvecAbonnement(tenant string, rec *SubscriptionRecord) *Service {
	s := NewService()
	s.subs[tenant] = rec
	return s
}

// TestCompteurMemoire_FenetreGlissante : en mode dégradé, seules les commandes non
// annulées de la fenêtre courante (ancrée sur l'essai) sont comptées. Les annulations
// sont retirées par la source ; la fenêtre suit le même découpage que la base.
func TestCompteurMemoire_FenetreGlissante(t *testing.T) {
	ctx := context.Background()
	const tenant = "tenant-memoire"
	essai := ancreTest
	s := memoireAvecAbonnement(tenant, &SubscriptionRecord{
		PlanCode: PlanFree, Status: StatusTrialing, TrialStartedAt: &essai, CreatedAt: essai,
	})

	dates := []time.Time{
		essai.Add(-time.Second),     // avant l'ancre : hors fenêtre
		essai,                       // début de la 1re fenêtre : compté
		essai.Add(29 * jourFenetre), // dernier jour de la 1re fenêtre : compté
		essai.Add(30 * jourFenetre), // fin de la 1re : hors de la 1re fenêtre
		essai.Add(59 * jourFenetre), // dans la 2e fenêtre
		essai.Add(60 * jourFenetre), // fin de la 2e
	}
	compteur := NewCompteursMemoire(s, SourcesMemoire{
		DatesCommandes: func(context.Context, string) ([]time.Time, error) { return dates, nil },
	})

	// Instant dans la 1re fenêtre : 2 commandes (ancre et 29e jour ; avant l'ancre et fin exclues).
	if n, _ := compteur.CountOrdersInPeriod(ctx, tenant, essai.Add(10*jourFenetre)); n != 2 {
		t.Errorf("1re fenêtre : attendu 2 commandes, obtenu %d", n)
	}
	// Instant dans la 2e fenêtre : la fenêtre a glissé, 2 commandes (30 j et 59 j).
	if n, _ := compteur.CountOrdersInPeriod(ctx, tenant, essai.Add(31*jourFenetre)); n != 2 {
		t.Errorf("2e fenêtre : attendu 2 commandes, obtenu %d", n)
	}
}

// TestCompteurMemoire_AncreInconnue : sans ancre, toutes les commandes non annulées
// comptent (conservateur) plutôt que de laisser la quota ouverte.
func TestCompteurMemoire_AncreInconnue(t *testing.T) {
	ctx := context.Background()
	s := NewService()
	dates := []time.Time{ancreTest, ancreTest.Add(-400 * jourFenetre)}
	compteur := NewCompteursMemoire(s, SourcesMemoire{
		DatesCommandes: func(context.Context, string) ([]time.Time, error) { return dates, nil },
	})
	if n, _ := compteur.CountOrdersInPeriod(ctx, "sans-abonnement", ancreTest); n != 2 {
		t.Errorf("ancre inconnue : attendu 2, obtenu %d", n)
	}
}

// TestCompteurMemoire_SourcesAbsentes : une source non branchée compte 0 (jamais de panique).
func TestCompteurMemoire_SourcesAbsentes(t *testing.T) {
	ctx := context.Background()
	compteur := NewCompteursMemoire(NewService(), SourcesMemoire{})
	if n, err := compteur.CountOrdersInPeriod(ctx, "t", ancreTest); n != 0 || err != nil {
		t.Errorf("commandes : attendu 0, obtenu %d (%v)", n, err)
	}
	if n, err := compteur.CountUsers(ctx, "t"); n != 0 || err != nil {
		t.Errorf("utilisateurs : attendu 0, obtenu %d (%v)", n, err)
	}
	if n, err := compteur.CountPriceLists(ctx, "t"); n != 0 || err != nil {
		t.Errorf("grilles : attendu 0, obtenu %d (%v)", n, err)
	}
	if n, err := compteur.CountBranches(ctx, "t"); n != 0 || err != nil {
		t.Errorf("établissements : attendu 0, obtenu %d (%v)", n, err)
	}
}

// TestEffectiveEntitlements_PeriodeEtTarif : la réponse expose les bornes de la fenêtre
// quand l'ancre est connue. En mode mémoire, le tarif n'est pas inventé (absent).
func TestEffectiveEntitlements_PeriodeEtTarif(t *testing.T) {
	ctx := context.Background()
	const tenant = "tenant-periode"
	creation := time.Now().UTC().Add(-45 * jourFenetre)
	s := memoireAvecAbonnement(tenant, &SubscriptionRecord{
		PlanCode: PlanBusiness, Status: StatusActive, PlanName: "Business", CreatedAt: creation,
	})

	resp, err := s.EffectiveEntitlements(ctx, tenant)
	if err != nil {
		t.Fatalf("droits effectifs: %v", err)
	}
	if resp.OrdersPeriod == nil {
		t.Fatal("la fenêtre de commandes doit être exposée quand l'ancre est connue")
	}
	// 45 jours après la création : la fenêtre courante commence au 30e jour.
	if want := creation.Add(30 * jourFenetre); !resp.OrdersPeriod.Start.Equal(want) {
		t.Errorf("début de fenêtre : attendu %s, obtenu %s", want, resp.OrdersPeriod.Start)
	}
	if got := resp.OrdersPeriod.End.Sub(resp.OrdersPeriod.Start); got != 30*jourFenetre {
		t.Errorf("durée de fenêtre : attendu 30 jours, obtenu %s", got)
	}
	if resp.PriceMonthlyXOF != nil {
		t.Errorf("mode mémoire : aucun tarif ne doit être inventé, obtenu %v", *resp.PriceMonthlyXOF)
	}

	// Sans ancre (abonnement sans date), la fenêtre n'est pas exposée.
	sansAncre := memoireAvecAbonnement("tenant-sans-ancre", &SubscriptionRecord{PlanCode: PlanFree, Status: StatusActive})
	resp, err = sansAncre.EffectiveEntitlements(ctx, "tenant-sans-ancre")
	if err != nil {
		t.Fatalf("droits effectifs sans ancre: %v", err)
	}
	if resp.OrdersPeriod != nil {
		t.Errorf("sans ancre, aucune fenêtre ne doit être exposée")
	}
}

// TestCatalogue_MemoireSansPrix : la grille mémoire liste les trois formules dans un
// ordre stable, avec leurs droits, et sans tarif (le prix vient de la base).
func TestCatalogue_MemoireSansPrix(t *testing.T) {
	ctx := context.Background()
	plans, err := NewService().Catalogue(ctx)
	if err != nil {
		t.Fatalf("catalogue: %v", err)
	}
	codes := make([]string, 0, len(plans))
	for _, p := range plans {
		codes = append(codes, p.Code)
		if p.PriceMonthlyXOF != nil {
			t.Errorf("%s : aucun tarif attendu en mode mémoire", p.Code)
		}
		if len(p.Entitlements) == 0 {
			t.Errorf("%s : les droits de la formule doivent être listés", p.Code)
		}
		for i := 1; i < len(p.Entitlements); i++ {
			if p.Entitlements[i-1].Key > p.Entitlements[i].Key {
				t.Errorf("%s : droits non triés (%s avant %s)", p.Code, p.Entitlements[i-1].Key, p.Entitlements[i].Key)
			}
		}
	}
	if want := []string{PlanFree, PlanPro, PlanBusiness}; len(codes) != len(want) ||
		codes[0] != want[0] || codes[1] != want[1] || codes[2] != want[2] {
		t.Errorf("formules : attendu %v, obtenu %v", want, codes)
	}
}

// TestEntitlements_FormuleSouscriteDistincteDeLEffective : pendant l'essai, les droits
// effectifs sont ceux de BUSINESS, mais la formule souscrite reste FREE. L'écran
// « Mon abonnement » doit pouvoir distinguer les deux.
func TestEntitlements_FormuleSouscriteDistincteDeLEffective(t *testing.T) {
	ctx := context.Background()

	fin := time.Now().UTC().Add(5 * 24 * time.Hour)
	essai := memoireAvecAbonnement("tenant-essai", &SubscriptionRecord{
		PlanCode: PlanFree, Status: StatusTrialing, PlanName: "Gratuit", TrialEndsAt: &fin,
		CreatedAt: time.Now().UTC(),
	})
	resp, err := essai.EffectiveEntitlements(ctx, "tenant-essai")
	if err != nil {
		t.Fatalf("droits effectifs pendant l'essai: %v", err)
	}
	if !resp.OnTrial || resp.PlanCode != PlanBusiness {
		t.Fatalf("pendant l'essai : droits BUSINESS attendus, obtenu plan=%s essai=%v", resp.PlanCode, resp.OnTrial)
	}
	if resp.SubscribedPlanCode != PlanFree {
		t.Errorf("formule souscrite attendue FREE pendant l'essai, obtenu %q", resp.SubscribedPlanCode)
	}

	pro := memoireAvecAbonnement("tenant-pro", &SubscriptionRecord{
		PlanCode: PlanPro, Status: StatusActive, PlanName: "Pro", CreatedAt: time.Now().UTC(),
	})
	resp, err = pro.EffectiveEntitlements(ctx, "tenant-pro")
	if err != nil {
		t.Fatalf("droits effectifs PRO: %v", err)
	}
	if resp.SubscribedPlanCode != PlanPro || resp.PlanCode != PlanPro {
		t.Errorf("abonnement PRO : attendu plan=PRO souscrit=PRO, obtenu plan=%s souscrit=%s",
			resp.PlanCode, resp.SubscribedPlanCode)
	}
}
