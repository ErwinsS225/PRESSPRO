package http_test

import (
	"net/http"
	"testing"
)

// Phase 8 — PATCH /api/v1/admin/plans/{code}/price (espace éditeur, journalisé).
//
// Ce test n'exerce que les refus (validation, formule gratuite, formule inconnue, mode
// mémoire) : il ne modifie jamais un tarif de la base partagée. La modification réussie
// est couverte par TestIntegration_TarifJournaliseAvecActeurEtMotif (paquet billing).
// Les droits (403 pour un gérant ou un propriétaire) sont couverts par la matrice de routes.
func TestAdmin_TarifDesFormules(t *testing.T) {
	pourChaqueMode(t, func(t *testing.T, env *environnement) {
		superadmin := env.connexion("admin@pressingos.ci", "admin123")

		refus := []struct {
			nom    string
			chemin string
			corps  interface{}
		}{
			{"formule gratuite", "/api/v1/admin/plans/FREE/price", map[string]interface{}{"price_monthly_xof": 500}},
			{"montant décimal", "/api/v1/admin/plans/PRO/price", map[string]interface{}{"price_monthly_xof": 10000.5}},
			{"montant négatif", "/api/v1/admin/plans/PRO/price", map[string]interface{}{"price_monthly_xof": -1}},
			{"sans montant", "/api/v1/admin/plans/PRO/price", map[string]interface{}{"reason": "sans montant"}},
			{"corps non JSON", "/api/v1/admin/plans/PRO/price", "pas du json"},
		}
		for _, c := range refus {
			rec := env.requete(http.MethodPatch, c.chemin, superadmin, c.corps)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("%s : 400 attendu, obtenu %d (%s)", c.nom, rec.Code, rec.Body.String())
			}
		}

		// Formule inconnue : 404 en base ; en mémoire, aucun catalogue : 503.
		rec := env.requete(http.MethodPatch, "/api/v1/admin/plans/INCONNU/price", superadmin,
			map[string]interface{}{"price_monthly_xof": 1000})
		attendu := http.StatusServiceUnavailable
		if env.persistant {
			attendu = http.StatusNotFound
		}
		if rec.Code != attendu {
			t.Errorf("formule inconnue : %d attendu, obtenu %d (%s)", attendu, rec.Code, rec.Body.String())
		}

		// Mémoire : la modification d'une vraie formule est refusée, jamais simulée.
		if !env.persistant {
			rec := env.requete(http.MethodPatch, "/api/v1/admin/plans/PRO/price", superadmin,
				map[string]interface{}{"price_monthly_xof": 10000})
			if rec.Code != http.StatusServiceUnavailable {
				t.Errorf("mémoire : 503 attendu, obtenu %d (%s)", rec.Code, rec.Body.String())
			}
		}
	})
}
