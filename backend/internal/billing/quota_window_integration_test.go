//go:build integration

package billing_test

import (
	"context"
	"testing"
	"time"

	"pressing-os/backend/internal/billing"
	"pressing-os/backend/internal/database"
)

// Tests d'intégration du quota de commandes (PLAN phase 3, PostgreSQL requis).
//
// Règles validées : fenêtre glissante de 30 jours ancrée sur le début de période
// de l'abonnement (trial_started_at, à défaut created_at, à défaut la création de
// l'établissement) ; les commandes annulées ne consomment pas le quota ; un
// changement de formule ne déplace pas la fenêtre.
//
// Exécution :
//
//	TEST_DATABASE_URL="..." go test -tags=integration -count=1 -run 'TestQuota_' ./internal/billing/...

// quotaDe construit le garde-fou adossé au compteur SQL réel.
func quotaDe(db *database.DB, svc *billing.Service) *billing.QuotaGuard {
	return &billing.QuotaGuard{Billing: svc, Counter: billing.NewDBUsageCounter(database.NewStore(db.Pool))}
}

// commandeDatee insère une commande de test dont created_at = NOW() - age (expression
// d'intervalle SQL) ; age vide = maintenant.
func commandeDatee(t *testing.T, ctx context.Context, db *database.DB, tenant, numero, statut, age string) {
	t.Helper()
	var client string
	if err := db.Pool.QueryRow(ctx, `SELECT id FROM customers WHERE tenant_id = $1 LIMIT 1`, tenant).Scan(&client); err != nil {
		t.Fatalf("client de test: %v", err)
	}
	requete := `INSERT INTO orders (tenant_id, customer_id, order_number, status, payment_status, total_amount, paid_amount, created_at)
		VALUES ($1, $2, $3, $4, 'unpaid', 1000, 0, NOW() - $5::interval)`
	if age == "" {
		requete = `INSERT INTO orders (tenant_id, customer_id, order_number, status, payment_status, total_amount, paid_amount)
			VALUES ($1, $2, $3, $4, 'unpaid', 1000, 0)`
		if _, err := db.Pool.Exec(ctx, requete, tenant, client, numero, statut); err != nil {
			t.Fatalf("commande %s: %v", numero, err)
		}
		return
	}
	if _, err := db.Pool.Exec(ctx, requete, tenant, client, numero, statut, age); err != nil {
		t.Fatalf("commande %s (%s): %v", numero, age, err)
	}
}

// compterFenetre retourne la consommation de la fenêtre courante (instant réel).
func compterFenetre(t *testing.T, ctx context.Context, db *database.DB, tenant string) int64 {
	t.Helper()
	n, err := billing.NewDBUsageCounter(database.NewStore(db.Pool)).CountOrdersInPeriod(ctx, tenant, time.Now().UTC())
	if err != nil {
		t.Fatalf("CountOrdersInPeriod: %v", err)
	}
	return n
}

// Une commande annulée libère sa place : 49 actives + 1 annulée laisse la 50e passer.
func TestQuota_CommandeAnnulee_NeConsommePasLeQuota(t *testing.T) {
	ctx, db, svc := ouvrirBaseLifecycle(t)
	tenant := creerEtablissementLifecycle(t, ctx, db, svc, "FREE", "ACTIVE", "")
	ajouterCommandes(t, ctx, db, tenant, 49)
	commandeDatee(t, ctx, db, tenant, "CMD-QUOTA-ANNULEE", "cancelled", "")

	if n := compterFenetre(t, ctx, db, tenant); n != 49 {
		t.Fatalf("consommation = %d, attendu 49 (la commande annulée ne compte pas)", n)
	}
	guard := quotaDe(db, svc)
	p, err := guard.PlafondCommandes(ctx, tenant)
	if err != nil {
		t.Fatalf("PlafondCommandes: %v", err)
	}
	if err := guard.VerifierCommande(ctx, tenant, p); err != nil {
		t.Errorf("49 actives + 1 annulée : la 50e devrait être autorisée, obtenu %v", err)
	}
}

// Fenêtre ouverte à D = début de période + 30 jours. Bornes inclusives à gauche,
// exclusives à droite, vérifiées au niveau de la seconde.
func TestQuota_FrontiereDeFenetre_AuxSecondes(t *testing.T) {
	ctx, db, svc := ouvrirBaseLifecycle(t)
	tenant := creerEtablissementLifecycle(t, ctx, db, svc, "FREE", "ACTIVE", "")
	if _, err := db.Pool.Exec(ctx,
		`UPDATE subscriptions SET trial_started_at = NOW() - INTERVAL '35 days' WHERE tenant_id = $1`, tenant); err != nil {
		t.Fatalf("ancrage: %v", err)
	}
	ajouterCommandes(t, ctx, db, tenant, 0)

	var debut time.Time
	if err := db.Pool.QueryRow(ctx,
		`SELECT trial_started_at + INTERVAL '30 days' FROM subscriptions WHERE tenant_id = $1`, tenant).Scan(&debut); err != nil {
		t.Fatalf("début de la fenêtre courante: %v", err)
	}
	fin := debut.Add(30 * 24 * time.Hour)

	// Hors fenêtre : une seconde avant le début (fenêtre précédente).
	commandeDatee(t, ctx, db, tenant, "CMD-FR-AVANT", "received", "5 days 1 second")
	// Dans la fenêtre : exactement au début, et une seconde avant la fin.
	// (Insertion par instant absolu, pas par intervalle relatif à NOW().)
	insererAInstant(t, ctx, db, tenant, "CMD-FR-DEBUT", debut)
	insererAInstant(t, ctx, db, tenant, "CMD-FR-FIN-1S", fin.Add(-time.Second))
	// Hors fenêtre : exactement à la fin (première seconde de la fenêtre suivante).
	insererAInstant(t, ctx, db, tenant, "CMD-FR-FIN", fin)

	if n := compterFenetre(t, ctx, db, tenant); n != 2 {
		t.Errorf("consommation = %d, attendu 2 (début inclus, fin exclue)", n)
	}
}

// insererAInstant insère une commande à un instant absolu (créée en UTC).
func insererAInstant(t *testing.T, ctx context.Context, db *database.DB, tenant, numero string, instant time.Time) {
	t.Helper()
	var client string
	if err := db.Pool.QueryRow(ctx, `SELECT id FROM customers WHERE tenant_id = $1 LIMIT 1`, tenant).Scan(&client); err != nil {
		t.Fatalf("client de test: %v", err)
	}
	if _, err := db.Pool.Exec(ctx,
		`INSERT INTO orders (tenant_id, customer_id, order_number, status, payment_status, total_amount, paid_amount, created_at)
		 VALUES ($1, $2, $3, 'received', 'unpaid', 1000, 0, $4)`,
		tenant, client, numero, instant.UTC()); err != nil {
		t.Fatalf("commande %s à %s: %v", numero, instant, err)
	}
}

// Une fenêtre se renouvelle sans job : à 65 jours de l'ancrage, la fenêtre courante
// est la troisième tranche ; les commandes de la tranche précédente ne comptent plus.
func TestQuota_RoulementDeFenetre_AuBoutDeDeuxMois(t *testing.T) {
	ctx, db, svc := ouvrirBaseLifecycle(t)
	tenant := creerEtablissementLifecycle(t, ctx, db, svc, "FREE", "ACTIVE", "")
	if _, err := db.Pool.Exec(ctx,
		`UPDATE subscriptions SET trial_started_at = NOW() - INTERVAL '65 days' WHERE tenant_id = $1`, tenant); err != nil {
		t.Fatalf("ancrage: %v", err)
	}
	ajouterCommandes(t, ctx, db, tenant, 0)
	commandeDatee(t, ctx, db, tenant, "CMD-ROUL-1", "received", "10 days") // tranche précédente
	commandeDatee(t, ctx, db, tenant, "CMD-ROUL-2", "received", "2 days")  // tranche courante
	commandeDatee(t, ctx, db, tenant, "CMD-ROUL-3", "received", "40 days") // deux tranches avant

	if n := compterFenetre(t, ctx, db, tenant); n != 1 {
		t.Errorf("consommation = %d, attendu 1 (seule la commande de la tranche courante)", n)
	}
}

// Sans essai, l'ancre est created_at de l'abonnement.
func TestQuota_AncrageSansEssai_SurCreatedAtDelAbonnement(t *testing.T) {
	ctx, db, svc := ouvrirBaseLifecycle(t)
	tenant := creerEtablissementLifecycle(t, ctx, db, svc, "FREE", "ACTIVE", "")
	if _, err := db.Pool.Exec(ctx,
		`UPDATE subscriptions SET trial_started_at = NULL, created_at = NOW() - INTERVAL '35 days' WHERE tenant_id = $1`, tenant); err != nil {
		t.Fatalf("ancrage: %v", err)
	}
	ajouterCommandes(t, ctx, db, tenant, 0)
	commandeDatee(t, ctx, db, tenant, "CMD-AC-1", "received", "10 days")
	commandeDatee(t, ctx, db, tenant, "CMD-AC-2", "received", "2 days")

	if n := compterFenetre(t, ctx, db, tenant); n != 1 {
		t.Errorf("consommation = %d, attendu 1 (ancre = created_at de l'abonnement)", n)
	}
}

// Sans abonnement, l'ancre est la création de l'établissement (tenants.created_at).
func TestQuota_AncrageSansAbonnement_SurCreatedAtDelEtablissement(t *testing.T) {
	ctx, db, svc := ouvrirBaseLifecycle(t)
	tenant := creerEtablissementLifecycle(t, ctx, db, svc, "", "", "")
	if _, err := db.Pool.Exec(ctx,
		`UPDATE tenants SET created_at = NOW() - INTERVAL '35 days' WHERE id = $1`, tenant); err != nil {
		t.Fatalf("ancrage: %v", err)
	}
	ajouterCommandes(t, ctx, db, tenant, 0)
	commandeDatee(t, ctx, db, tenant, "CMD-AT-1", "received", "10 days")
	commandeDatee(t, ctx, db, tenant, "CMD-AT-2", "received", "2 days")

	if n := compterFenetre(t, ctx, db, tenant); n != 1 {
		t.Errorf("consommation = %d, attendu 1 (ancre = created_at de l'établissement)", n)
	}
}

// Un changement de formule ne remet pas la fenêtre à zéro : l'ancre ne dépend pas du plan.
func TestQuota_ChangementDeFormule_NeDeplacePasLaFenetre(t *testing.T) {
	ctx, db, svc := ouvrirBaseLifecycle(t)
	tenant := creerEtablissementLifecycle(t, ctx, db, svc, "FREE", "ACTIVE", "")
	if _, err := db.Pool.Exec(ctx,
		`UPDATE subscriptions SET trial_started_at = NOW() - INTERVAL '35 days' WHERE tenant_id = $1`, tenant); err != nil {
		t.Fatalf("ancrage: %v", err)
	}
	ajouterCommandes(t, ctx, db, tenant, 0)
	commandeDatee(t, ctx, db, tenant, "CMD-PL-1", "received", "10 days")
	commandeDatee(t, ctx, db, tenant, "CMD-PL-2", "received", "2 days")
	if n := compterFenetre(t, ctx, db, tenant); n != 1 {
		t.Fatalf("avant changement : consommation = %d, attendu 1", n)
	}

	if _, err := db.Pool.Exec(ctx,
		`UPDATE subscriptions SET plan_id = (SELECT id FROM plans WHERE code = 'BUSINESS') WHERE tenant_id = $1`, tenant); err != nil {
		t.Fatalf("changement de formule: %v", err)
	}
	if n := compterFenetre(t, ctx, db, tenant); n != 1 {
		t.Errorf("après passage à BUSINESS : consommation = %d, attendu 1 (fenêtre inchangée)", n)
	}
}

// Le plafond lu depuis les droits effectifs bloque la 51e commande FREE, et la
// formule BUSINESS (sans plafond) n'est jamais bloquée.
func TestQuota_PlafondDuPlan_FreeBloqueLa51e_BusinessNon(t *testing.T) {
	ctx, db, svc := ouvrirBaseLifecycle(t)
	free := creerEtablissementLifecycle(t, ctx, db, svc, "FREE", "ACTIVE", "")
	ajouterCommandes(t, ctx, db, free, 50)
	guard := quotaDe(db, svc)
	p, err := guard.PlafondCommandes(ctx, free)
	if err != nil {
		t.Fatalf("PlafondCommandes FREE: %v", err)
	}
	if err := guard.VerifierCommande(ctx, free, p); !billing.IsRefusMetier(err) {
		t.Errorf("FREE avec 50 commandes : refus métier attendu, obtenu %v", err)
	}

	business := creerEtablissementLifecycle(t, ctx, db, svc, "BUSINESS", "ACTIVE", "")
	ajouterCommandes(t, ctx, db, business, 60)
	pb, err := guard.PlafondCommandes(ctx, business)
	if err != nil {
		t.Fatalf("PlafondCommandes BUSINESS: %v", err)
	}
	if err := guard.VerifierCommande(ctx, business, pb); err != nil {
		t.Errorf("BUSINESS avec 60 commandes : aucun refus attendu, obtenu %v", err)
	}
}
