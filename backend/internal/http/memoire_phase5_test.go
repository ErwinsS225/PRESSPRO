package http_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"pressing-os/backend/internal/auth"
	"pressing-os/backend/internal/billing"
	"pressing-os/backend/internal/branch"
	"pressing-os/backend/internal/order"
	"pressing-os/backend/internal/pricelist"
)

// Phase 5 (mode dégradé, sans PostgreSQL) : les plafonds sont mesurés en mémoire
// (R-P4-3), le contrôle d'appartenance des affectations est fail-closed (R-P4-2) et
// la création ou le verrouillage des comptes propriétaires est réservé au propriétaire
// (R-P4-6). Les tests TestMemoire_* s'exécutent en mémoire ; TestGerant_* s'exécute
// aussi en PostgreSQL avec -tags=integration (pourChaqueMode).

// TestMemoire_PlafondsFreeMesures : en formule FREE, chaque plafond est appliqué en
// mode mémoire (comptes, établissements, grilles et commandes non annulées).
func TestMemoire_PlafondsFreeMesures(t *testing.T) {
	env := nouvelEnvironnement(t)
	jeton, tenant := env.inscrire("Pressing Plafonds", "plafonds@test.ci")
	env.formule(tenant, billing.PlanFree)

	// Comptes : FREE = 2 comptes actifs (le propriétaire compte pour 1).
	creerCompte(env, jeton, "employe.plafond@test.ci", auth.RoleCashier)
	rec := env.requete("POST", "/api/v1/users", jeton, map[string]string{
		"full_name": "Employé en trop", "email": "trop.plafond@test.ci", "phone": "+225 05 00 00 01",
		"role": auth.RoleCashier, "password": "motdepasse1",
	})
	if rec.Code != http.StatusPaymentRequired {
		t.Errorf("3e compte en FREE : %d, attendu 402 (%s)", rec.Code, rec.Body.String())
	}

	// Établissements : FREE = 1, soit le principal seulement.
	if rec := env.requete("POST", "/api/v1/branches/", jeton, branch.CreateBranchRequest{Name: "Annexe"}); rec.Code != http.StatusPaymentRequired {
		t.Errorf("2e établissement en FREE : %d, attendu 402 (%s)", rec.Code, rec.Body.String())
	}

	// Grilles tarifaires : FREE = 1.
	if rec := env.requete("POST", "/api/v1/price-lists/", jeton, pricelist.CreatePriceListRequest{Name: "Première"}); rec.Code != http.StatusCreated {
		t.Fatalf("1re grille en FREE : %d (%s)", rec.Code, rec.Body.String())
	}
	if rec := env.requete("POST", "/api/v1/price-lists/", jeton, pricelist.CreatePriceListRequest{Name: "Seconde"}); rec.Code != http.StatusPaymentRequired {
		t.Errorf("2e grille en FREE : %d, attendu 402", rec.Code)
	}
}

// TestMemoire_QuotaCommandesFenetreEtAnnulations : FREE accepte 50 commandes sur la
// fenêtre de 30 jours ; la 51e est refusée ; une commande annulée libère une place.
func TestMemoire_QuotaCommandesFenetreEtAnnulations(t *testing.T) {
	env := nouvelEnvironnement(t)
	jeton, tenant := env.inscrire("Pressing Commandes", "commandes.quota@test.ci")
	env.formule(tenant, billing.PlanFree)

	nouvelleCommande := func(i int) *httptest.ResponseRecorder {
		return env.requete("POST", "/api/v1/orders/", jeton, order.CreateOrderRequest{
			CustomerName:  fmt.Sprintf("Client %d", i),
			CustomerPhone: fmt.Sprintf("+225 07 00 00 %02d", i),
			Items:         []order.CreateOrderItemInput{{ItemLabel: "Chemise", Quantity: 1, UnitPrice: 1000}},
		})
	}

	var premiere string
	for i := 1; i <= 50; i++ {
		rec := nouvelleCommande(i)
		if rec.Code != http.StatusCreated {
			t.Fatalf("commande %d en FREE : %d (%s), attendu 201", i, rec.Code, rec.Body.String())
		}
		if i == 1 {
			premiere = env.idDe(rec)
		}
	}
	if rec := nouvelleCommande(51); rec.Code != http.StatusPaymentRequired {
		t.Fatalf("51e commande en FREE : %d, attendu 402 (%s)", rec.Code, rec.Body.String())
	}

	// Une commande annulée ne consomme plus le plafond.
	if rec := env.requete("PATCH", "/api/v1/orders/"+premiere+"/status", jeton, map[string]string{"status": order.StatusCancelled}); rec.Code != http.StatusOK {
		t.Fatalf("annulation de la commande : %d (%s)", rec.Code, rec.Body.String())
	}
	if rec := nouvelleCommande(52); rec.Code != http.StatusCreated {
		t.Errorf("commande après annulation : %d, attendu 201 (%s)", rec.Code, rec.Body.String())
	}
}

// TestMemoire_CatalogueDesFormules : GET /api/v1/billing/plans liste les formules
// avec leurs droits. En mode mémoire, aucun tarif n'est affiché (il vient de la base).
func TestMemoire_CatalogueDesFormules(t *testing.T) {
	env := nouvelEnvironnement(t)
	jeton, _ := env.inscrire("Pressing Catalogue", "catalogue.formules@test.ci")

	rec := env.requete("GET", "/api/v1/billing/plans", jeton, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("catalogue : %d (%s)", rec.Code, rec.Body.String())
	}
	var corps struct {
		Plans []struct {
			Code            string           `json:"code"`
			PriceMonthlyXOF *float64         `json:"price_monthly_xof"`
			Entitlements    []map[string]any `json:"entitlements"`
		} `json:"plans"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &corps); err != nil {
		t.Fatalf("catalogue illisible: %v", err)
	}
	codes := map[string]bool{}
	for _, p := range corps.Plans {
		codes[p.Code] = true
		if p.PriceMonthlyXOF != nil {
			t.Errorf("%s : tarif affiché en mode mémoire (%v), attendu absent", p.Code, *p.PriceMonthlyXOF)
		}
		if len(p.Entitlements) == 0 {
			t.Errorf("%s : aucun droit listé", p.Code)
		}
	}
	for _, code := range []string{billing.PlanFree, billing.PlanPro, billing.PlanBusiness} {
		if !codes[code] {
			t.Errorf("formule %s absente du catalogue", code)
		}
	}
	if rec := env.requete("GET", "/api/v1/billing/plans", "", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("catalogue sans jeton : %d, attendu 401", rec.Code)
	}
}

// TestMemoire_AffectationCompteEtranger : R-P4-2 — en mode mémoire, un établissement
// ne peut ni lire ni affecter les sites d'un compte d'un autre établissement (404).
func TestMemoire_AffectationCompteEtranger(t *testing.T) {
	env := nouvelEnvironnement(t)
	l := preparerLocataires(t, env)

	if rec := env.requete("GET", "/api/v1/users/"+l.aCaissierID+"/branches", l.bProprietaire, nil); rec.Code != http.StatusNotFound {
		t.Errorf("lecture des sites d'un compte étranger : %d, attendu 404", rec.Code)
	}
	if rec := env.requete("PUT", "/api/v1/users/"+l.aCaissierID+"/branches", l.bProprietaire,
		map[string]interface{}{"branch_ids": []string{"site-b"}}); rec.Code != http.StatusNotFound {
		t.Errorf("affectation d'un compte étranger : %d, attendu 404", rec.Code)
	}
	// Contrôle positif : le propriétaire de A affecte son propre caissier.
	if rec := env.requete("GET", "/api/v1/users/"+l.aCaissierID+"/branches", l.aProprietaire, nil); rec.Code != http.StatusOK {
		t.Errorf("lecture des sites de son propre compte : %d, attendu 200", rec.Code)
	}
}

// TestGerant_NePeutPasCreerNiVerrouillerUnProprietaire : R-P4-6 — la création d'un
// compte propriétaire et le changement de statut d'un propriétaire sont réservés au
// propriétaire. Un gérant garde la gestion des autres comptes.
func TestGerant_NePeutPasCreerNiVerrouillerUnProprietaire(t *testing.T) {
	pourChaqueMode(t, func(t *testing.T, env *environnement) {
		l := preparerLocataires(t, env)
		gerant, gerantID := creerCompte(env, l.aProprietaire, "gerant.r6@test.ci", auth.RoleManager)
		if gerantID == "" {
			t.Fatal("identifiant du gérant introuvable")
		}

		// Création d'un propriétaire par un gérant : refusée, aucun compte créé.
		rec := env.requete("POST", "/api/v1/users", gerant, map[string]string{
			"full_name": "Faux propriétaire", "email": compteUnique("faux.proprio@test.ci"), "phone": "+225 05 00 00 09",
			"role": auth.RoleOwner, "password": "motdepasse1",
		})
		if rec.Code != http.StatusForbidden {
			t.Errorf("gérant crée un propriétaire : %d, attendu 403 (%s)", rec.Code, rec.Body.String())
		}

		// Le propriétaire, lui, peut créer un second propriétaire.
		_, ownerDeux := creerCompte(env, l.aProprietaire, "proprio.deux@test.ci", auth.RoleOwner)
		if ownerDeux == "" {
			t.Fatal("identifiant du second propriétaire introuvable")
		}

		// Verrouillage d'un propriétaire par un gérant : refusé.
		if rec := env.requete("PATCH", "/api/v1/users/"+ownerDeux+"/status", gerant, map[string]string{"status": "locked"}); rec.Code != http.StatusForbidden {
			t.Errorf("gérant verrouille un propriétaire : %d, attendu 403 (%s)", rec.Code, rec.Body.String())
		}
		// Le gérant garde la gestion d'un compte employé.
		if rec := env.requete("PATCH", "/api/v1/users/"+l.aCaissierID+"/status", gerant, map[string]string{"status": "inactive"}); rec.Code != http.StatusOK {
			t.Errorf("gérant désactive un caissier : %d, attendu 200 (%s)", rec.Code, rec.Body.String())
		}
		// Le propriétaire peut verrouiller un autre propriétaire.
		if rec := env.requete("PATCH", "/api/v1/users/"+ownerDeux+"/status", l.aProprietaire, map[string]string{"status": "locked"}); rec.Code != http.StatusOK {
			t.Errorf("propriétaire verrouille un propriétaire : %d, attendu 200 (%s)", rec.Code, rec.Body.String())
		}
	})
}
