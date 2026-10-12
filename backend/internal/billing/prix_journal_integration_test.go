//go:build integration

package billing_test

import (
	"context"
	"errors"
	"testing"

	"pressing-os/backend/internal/billing"
)

// Test d'intégration PostgreSQL du journal des tarifs (migration 000017).
//
//	TEST_DATABASE_URL="postgres://..." go test -tags=integration ./internal/billing/...
//
// Le test travaille sur une formule jetable, inactive et supprimée à la fin : les
// formules publiques (PRO, BUSINESS) ne sont jamais modifiées, car d'autres paquets
// d'intégration les lisent en parallèle.
func TestIntegration_TarifJournaliseAvecActeurEtMotif(t *testing.T) {
	ctx, db, svc := ouvrirBaseLifecycle(t)

	var code string
	if err := db.Pool.QueryRow(ctx,
		`SELECT 'TST' || upper(substr(md5(random()::text), 1, 10))`).Scan(&code); err != nil {
		t.Fatalf("code de test: %v", err)
	}
	var planID string
	if err := db.Pool.QueryRow(ctx,
		`INSERT INTO plans (code, name, price_monthly_xof, is_active) VALUES ($1, 'Formule de test', 1000, false) RETURNING id`,
		code).Scan(&planID); err != nil {
		t.Fatalf("formule de test: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.Pool.Exec(context.Background(), `DELETE FROM plans WHERE id = $1`, planID)
	})

	// 1) Changement sans acteur : une ligne, ancien et nouveau prix, motif, acteur NULL.
	if err := svc.SetPlanPrice(ctx, code, 2000, "", "motif de test"); err != nil {
		t.Fatalf("modification du tarif: %v", err)
	}
	type ligne struct {
		ancien, nouveau float64
		motif           string
		acteurNul       bool
	}
	lignes := func() []ligne {
		rows, err := db.Pool.Query(ctx, `
			SELECT old_price_xof::float8, new_price_xof::float8, reason, changed_by IS NULL
			FROM plan_price_history WHERE plan_id = $1 ORDER BY changed_at, id`, planID)
		if err != nil {
			t.Fatalf("lecture du journal: %v", err)
		}
		defer rows.Close()
		var out []ligne
		for rows.Next() {
			var l ligne
			if err := rows.Scan(&l.ancien, &l.nouveau, &l.motif, &l.acteurNul); err != nil {
				t.Fatalf("scan du journal: %v", err)
			}
			out = append(out, l)
		}
		return out
	}
	journal := lignes()
	if len(journal) != 1 {
		t.Fatalf("journal après 1 changement : %d ligne(s), attendu 1", len(journal))
	}
	if journal[0].ancien != 1000 || journal[0].nouveau != 2000 || journal[0].motif != "motif de test" || !journal[0].acteurNul {
		t.Errorf("ligne de journal inattendue : %+v", journal[0])
	}

	// 2) Même montant : aucune nouvelle ligne (le trigger ne journalise qu'un changement réel).
	if err := svc.SetPlanPrice(ctx, code, 2000, "", "identique"); err != nil {
		t.Fatalf("modification identique: %v", err)
	}
	if n := len(lignes()); n != 1 {
		t.Errorf("tarif inchangé : %d ligne(s) au journal, attendu toujours 1", n)
	}

	// 3) Acteur réel (utilisateur d'un établissement de test) : journalisé.
	tenant := creerEtablissementLifecycle(t, ctx, db, svc, "FREE", "ACTIVE", "")
	var acteur string
	if err := db.Pool.QueryRow(ctx,
		`SELECT id::text FROM users WHERE tenant_id = $1 ORDER BY created_at LIMIT 1`, tenant).Scan(&acteur); err != nil {
		t.Fatalf("compte acteur: %v", err)
	}
	if err := svc.SetPlanPrice(ctx, code, 3000, acteur, "avec acteur"); err != nil {
		t.Fatalf("modification avec acteur: %v", err)
	}
	var acteurJournal *string
	if err := db.Pool.QueryRow(ctx,
		`SELECT changed_by::text FROM plan_price_history WHERE plan_id = $1 ORDER BY changed_at DESC, id DESC LIMIT 1`,
		planID).Scan(&acteurJournal); err != nil {
		t.Fatalf("lecture de l'acteur: %v", err)
	}
	if acteurJournal == nil || *acteurJournal != acteur {
		t.Errorf("acteur journalisé = %v, attendu %s", acteurJournal, acteur)
	}

	// 4) Acteur non UUID : accepté, journalisé NULL (jamais une erreur SQL).
	if err := svc.SetPlanPrice(ctx, code, 4000, "pas-un-uuid", ""); err != nil {
		t.Fatalf("modification avec acteur non UUID: %v", err)
	}
	if n := len(lignes()); n != 3 {
		t.Errorf("journal : %d ligne(s), attendu 3", n)
	}
	var prixLu float64
	if err := db.Pool.QueryRow(ctx, `SELECT price_monthly_xof::float8 FROM plans WHERE id = $1`, planID).Scan(&prixLu); err != nil {
		t.Fatalf("lecture du tarif: %v", err)
	}
	if prixLu != 4000 {
		t.Errorf("tarif enregistré = %v, attendu 4000", prixLu)
	}

	// 5) Formule inconnue : refus propre, sans ligne de journal.
	err := svc.SetPlanPrice(ctx, "INCONNU"+code, 500, "", "")
	if !errors.Is(err, billing.ErrPlanInconnu) {
		t.Errorf("formule inconnue : ErrPlanInconnu attendu, obtenu %v", err)
	}
}
