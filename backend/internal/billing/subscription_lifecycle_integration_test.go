//go:build integration

package billing_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"pressing-os/backend/internal/billing"
	"pressing-os/backend/internal/database"
)

// Tests d'intégration PostgreSQL du cycle de vie des abonnements (PLAN, phase 1).
//
// Exécution :
//
//	TEST_DATABASE_URL="postgres://pressing_app:...@localhost:5432/pressing_os?sslmode=disable" \
//	  go test -tags=integration -count=1 ./internal/billing/...
//
// Ces tests caractérisent le comportement réel de la persistance. Ceux qui
// échouent avant la phase 2 reproduisent un défaut documenté dans
// docs/subscription-phase0-baseline.md (R1, R3, erreurs de lecture) ; ils ne
// sont ni désactivés ni assouplis.

// ouvrirBaseLifecycle ouvre la base de test et prépare le moteur adossé à PostgreSQL.
func ouvrirBaseLifecycle(t *testing.T) (context.Context, *database.DB, *billing.Service) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL non défini : test d'intégration ignoré")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	db, err := database.ConnectPool(ctx, dsn)
	if err != nil {
		t.Fatalf("connexion PostgreSQL: %v", err)
	}
	t.Cleanup(db.Close)
	return ctx, db, billing.NewStore(database.NewStore(db.Pool))
}

// creerEtablissementLifecycle insère un établissement avec un compte, un client
// et un abonnement dans l'état voulu. Le nettoyage passe par DeleteTenant.
// trialEnds est une expression SQL (ex. "NOW() - INTERVAL '1 hour'") ou "" pour
// un abonnement sans essai.
func creerEtablissementLifecycle(t *testing.T, ctx context.Context, db *database.DB, svc *billing.Service,
	planCode, status, trialEnds string) string {
	t.Helper()
	suffixe := fmt.Sprintf("%d", time.Now().UnixNano())

	var tenant string
	if err := db.Pool.QueryRow(ctx,
		`INSERT INTO tenants (name, slug, city, status)
		 VALUES ($1, $2, 'Abidjan', 'active') RETURNING id`,
		"Pressing Lifecycle "+suffixe, "lifecycle-"+suffixe).Scan(&tenant); err != nil {
		t.Fatalf("création de l'établissement: %v", err)
	}
	t.Cleanup(func() { _ = svc.DeleteTenant(context.Background(), tenant) })

	if _, err := db.Pool.Exec(ctx,
		`INSERT INTO users (tenant_id, email, password_hash, full_name, role, status)
		 VALUES ($1, $2, 'hash-lifecycle', 'Propriétaire Lifecycle', 'owner', 'active')`,
		tenant, "owner."+suffixe+"@test.ci"); err != nil {
		t.Fatalf("création du compte: %v", err)
	}

	if trialEnds != "" {
		requete := fmt.Sprintf(`INSERT INTO subscriptions (tenant_id, plan_id, status, trial_started_at, trial_ends_at)
			SELECT $1, id, $2, NOW() - INTERVAL '7 days', %s FROM plans WHERE code = $3`, trialEnds)
		if _, err := db.Pool.Exec(ctx, requete, tenant, status, planCode); err != nil {
			t.Fatalf("création de l'abonnement d'essai: %v", err)
		}
	} else if planCode != "" {
		if _, err := db.Pool.Exec(ctx,
			`INSERT INTO subscriptions (tenant_id, plan_id, status)
			 SELECT $1, id, $2 FROM plans WHERE code = $3`,
			tenant, status, planCode); err != nil {
			t.Fatalf("création de l'abonnement: %v", err)
		}
	}
	return tenant
}

// ajouterCommandes insère n commandes (avec client) pour le tenant.
func ajouterCommandes(t *testing.T, ctx context.Context, db *database.DB, tenant string, n int) {
	t.Helper()
	var client string
	if err := db.Pool.QueryRow(ctx,
		`INSERT INTO customers (tenant_id, first_name, last_name, phone)
		 VALUES ($1, 'Client', 'Lifecycle', '+225 07 11 22 33') RETURNING id`, tenant).Scan(&client); err != nil {
		t.Fatalf("création du client: %v", err)
	}
	for i := 0; i < n; i++ {
		if _, err := db.Pool.Exec(ctx,
			`INSERT INTO orders (tenant_id, customer_id, order_number, status, payment_status, total_amount, paid_amount)
			 VALUES ($1, $2, $3, 'received', 'unpaid', 1000, 0)`,
			tenant, client, fmt.Sprintf("CMD-LC-%04d", i+1)); err != nil {
			t.Fatalf("création de la commande %d: %v", i+1, err)
		}
	}
}

func compter(t *testing.T, ctx context.Context, db *database.DB, requete string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := db.Pool.QueryRow(ctx, requete, args...).Scan(&n); err != nil {
		t.Fatalf("comptage (%s): %v", requete, err)
	}
	return n
}

// Un essai déjà démarré ne doit être journalisé qu'une fois, même si
// l'initialisation est appelée plusieurs fois (inscription + hook, reprise, etc.).
// Défaut attendu avant phase 2 : chaque appel insère un événement « trial_started ».
func TestLifecycle_EnsureTrialDeuxFois_JournaliseUneSeuleFois(t *testing.T) {
	ctx, db, svc := ouvrirBaseLifecycle(t)
	tenant := creerEtablissementLifecycle(t, ctx, db, svc, "", "", "")

	if err := svc.EnsureTrial(ctx, tenant); err != nil {
		t.Fatalf("EnsureTrial #1: %v", err)
	}
	if err := svc.EnsureTrial(ctx, tenant); err != nil {
		t.Fatalf("EnsureTrial #2: %v", err)
	}
	n := compter(t, ctx, db,
		`SELECT COUNT(*) FROM subscription_events WHERE tenant_id = $1 AND reason = 'trial_started'`, tenant)
	if n != 1 {
		t.Errorf("événements trial_started = %d, attendu 1 (un seul démarrage d'essai)", n)
	}
}

// Une seconde initialisation ne redémarre ni ne prolonge l'essai.
func TestLifecycle_EnsureTrialDeuxFois_NeProlongePasLessai(t *testing.T) {
	ctx, db, svc := ouvrirBaseLifecycle(t)
	tenant := creerEtablissementLifecycle(t, ctx, db, svc, "", "", "")

	if err := svc.EnsureTrial(ctx, tenant); err != nil {
		t.Fatalf("EnsureTrial #1: %v", err)
	}
	var fin1 time.Time
	if err := db.Pool.QueryRow(ctx, `SELECT trial_ends_at FROM subscriptions WHERE tenant_id = $1`, tenant).Scan(&fin1); err != nil {
		t.Fatalf("lecture de trial_ends_at: %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	if err := svc.EnsureTrial(ctx, tenant); err != nil {
		t.Fatalf("EnsureTrial #2: %v", err)
	}
	var fin2 time.Time
	if err := db.Pool.QueryRow(ctx, `SELECT trial_ends_at FROM subscriptions WHERE tenant_id = $1`, tenant).Scan(&fin2); err != nil {
		t.Fatalf("relecture de trial_ends_at: %v", err)
	}
	if !fin1.Equal(fin2) {
		t.Errorf("trial_ends_at modifié par une seconde initialisation : %s -> %s", fin1, fin2)
	}
}

// Un essai expiré doit être persisté vers FREE/ACTIVE lors d'une évaluation
// serveur (middleware, quota, /me), sans dépendre d'un appel du frontend à
// GET /me/entitlements ni d'un cron. Défaut attendu avant phase 2 : seul le
// handler /me/entitlements déclenche la transition, et la lecture ne persiste rien.
func TestLifecycle_EssaiExpire_PersisteVersFreeSansAppelFrontend(t *testing.T) {
	ctx, db, svc := ouvrirBaseLifecycle(t)
	tenant := creerEtablissementLifecycle(t, ctx, db, svc, "FREE", "TRIALING", "NOW() - INTERVAL '1 hour'")

	resp, err := svc.EffectiveEntitlements(ctx, tenant)
	if err != nil {
		t.Fatalf("EffectiveEntitlements: %v", err)
	}
	if resp.OnTrial || resp.PlanCode != billing.PlanFree {
		t.Fatalf("après expiration : on_trial=%t plan=%s, attendu false/FREE", resp.OnTrial, resp.PlanCode)
	}

	var status, plan string
	if err := db.Pool.QueryRow(ctx,
		`SELECT sub.status, p.code FROM subscriptions sub JOIN plans p ON p.id = sub.plan_id WHERE sub.tenant_id = $1`,
		tenant).Scan(&status, &plan); err != nil {
		t.Fatalf("lecture de l'abonnement: %v", err)
	}
	if status != billing.StatusActive || plan != billing.PlanFree {
		t.Errorf("état persisté = %s/%s, attendu ACTIVE/FREE après expiration", status, plan)
	}
	n := compter(t, ctx, db,
		`SELECT COUNT(*) FROM subscription_events WHERE tenant_id = $1 AND reason = 'trial_expired'`, tenant)
	if n != 1 {
		t.Errorf("événements trial_expired = %d, attendu 1", n)
	}
}

// Une expiration déclenchée par plusieurs requêtes concurrentes ne doit produire
// qu'une seule transition et un seul événement. Défaut attendu avant phase 2 :
// lecture puis écriture sans verrou, plusieurs transitions possibles.
func TestLifecycle_ExpirationConcurrente_UneSeuleTransition(t *testing.T) {
	ctx, db, svc := ouvrirBaseLifecycle(t)
	tenant := creerEtablissementLifecycle(t, ctx, db, svc, "FREE", "TRIALING", "NOW() - INTERVAL '1 hour'")

	const parallele = 8
	var transitions atomic.Int64
	var erreurs atomic.Int64
	var barriere sync.WaitGroup
	barriere.Add(1)
	var termine sync.WaitGroup
	for i := 0; i < parallele; i++ {
		termine.Add(1)
		go func() {
			defer termine.Done()
			barriere.Wait()
			ok, err := svc.ExpireTrialIfNeeded(ctx, tenant, "")
			if err != nil {
				erreurs.Add(1)
				return
			}
			if ok {
				transitions.Add(1)
			}
		}()
	}
	barriere.Done()
	termine.Wait()

	if erreurs.Load() != 0 {
		t.Errorf("%d erreur(s) inattendue(s) pendant l'expiration concurrente", erreurs.Load())
	}
	if transitions.Load() != 1 {
		t.Errorf("transitions effectives = %d, attendu exactement 1", transitions.Load())
	}
	n := compter(t, ctx, db,
		`SELECT COUNT(*) FROM subscription_events WHERE tenant_id = $1 AND reason = 'trial_expired'`, tenant)
	if n != 1 {
		t.Errorf("événements trial_expired = %d, attendu 1", n)
	}
}

// L'expiration ne supprime aucune donnée métier.
func TestLifecycle_Expiration_ConserveLesDonneesMetier(t *testing.T) {
	ctx, db, svc := ouvrirBaseLifecycle(t)
	tenant := creerEtablissementLifecycle(t, ctx, db, svc, "FREE", "TRIALING", "NOW() - INTERVAL '1 hour'")
	ajouterCommandes(t, ctx, db, tenant, 60)

	avant := compter(t, ctx, db, `SELECT COUNT(*) FROM orders WHERE tenant_id = $1`, tenant)
	clientsAvant := compter(t, ctx, db, `SELECT COUNT(*) FROM customers WHERE tenant_id = $1`, tenant)

	if _, err := svc.EffectiveEntitlements(ctx, tenant); err != nil {
		t.Fatalf("EffectiveEntitlements: %v", err)
	}
	if _, err := svc.ExpireTrialIfNeeded(ctx, tenant, ""); err != nil {
		t.Fatalf("ExpireTrialIfNeeded: %v", err)
	}

	if apres := compter(t, ctx, db, `SELECT COUNT(*) FROM orders WHERE tenant_id = $1`, tenant); apres != avant {
		t.Errorf("commandes après expiration = %d, attendu %d (aucune suppression)", apres, avant)
	}
	if apres := compter(t, ctx, db, `SELECT COUNT(*) FROM customers WHERE tenant_id = $1`, tenant); apres != clientsAvant {
		t.Errorf("clients après expiration = %d, attendu %d", apres, clientsAvant)
	}
	if apres := compter(t, ctx, db, `SELECT COUNT(*) FROM subscription_events WHERE tenant_id = $1`, tenant); apres < 1 {
		t.Errorf("historique d'abonnement vide après expiration")
	}
}

// Un abonnement payé et actif n'est jamais expiré, même si une date d'essai
// ancienne subsiste en base.
func TestLifecycle_AbonnementActif_NonExpire(t *testing.T) {
	ctx, db, svc := ouvrirBaseLifecycle(t)
	tenant := creerEtablissementLifecycle(t, ctx, db, svc, "PRO", "ACTIVE", "NOW() - INTERVAL '1 hour'")

	resp, err := svc.EffectiveEntitlements(ctx, tenant)
	if err != nil {
		t.Fatalf("EffectiveEntitlements: %v", err)
	}
	if resp.PlanCode != billing.PlanPro || resp.OnTrial {
		t.Errorf("plan effectif = %s (on_trial=%t), attendu PRO sans essai", resp.PlanCode, resp.OnTrial)
	}
	var status, plan string
	if err := db.Pool.QueryRow(ctx,
		`SELECT sub.status, p.code FROM subscriptions sub JOIN plans p ON p.id = sub.plan_id WHERE sub.tenant_id = $1`,
		tenant).Scan(&status, &plan); err != nil {
		t.Fatalf("lecture: %v", err)
	}
	if status != billing.StatusActive || plan != billing.PlanPro {
		t.Errorf("abonnement payé modifié : %s/%s, attendu ACTIVE/PRO", status, plan)
	}
	if n := compter(t, ctx, db,
		`SELECT COUNT(*) FROM subscription_events WHERE tenant_id = $1 AND reason = 'trial_expired'`, tenant); n != 0 {
		t.Errorf("trial_expired journalisé pour un abonnement payé : %d", n)
	}
}

// Un établissement sans abonnement reçoit la formule FREE, sans erreur, et peut
// démarrer son essai ensuite.
func TestLifecycle_SansAbonnement_FallbackFREE(t *testing.T) {
	ctx, db, svc := ouvrirBaseLifecycle(t)
	tenant := creerEtablissementLifecycle(t, ctx, db, svc, "", "", "")

	resp, err := svc.EffectiveEntitlements(ctx, tenant)
	if err != nil {
		t.Fatalf("EffectiveEntitlements sans abonnement: %v", err)
	}
	if resp.PlanCode != billing.PlanFree || resp.OnTrial {
		t.Errorf("sans abonnement : plan=%s on_trial=%t, attendu FREE sans essai", resp.PlanCode, resp.OnTrial)
	}
	if err := svc.EnsureTrial(ctx, tenant); err != nil {
		t.Fatalf("EnsureTrial: %v", err)
	}
	resp, err = svc.EffectiveEntitlements(ctx, tenant)
	if err != nil {
		t.Fatalf("EffectiveEntitlements après essai: %v", err)
	}
	if !resp.OnTrial || resp.PlanCode != billing.PlanBusiness {
		t.Errorf("après démarrage : on_trial=%t plan=%s, attendu essai BUSINESS", resp.OnTrial, resp.PlanCode)
	}
}

// Une erreur de lecture de la persistance ne doit pas être transformée en
// abonnement FREE silencieux. Défaut attendu avant phase 2 : dbStore.GetSubscription
// avale toute erreur de Scan et renvoie (nil, nil).
func TestLifecycle_ErreurDeLecture_NonSilencieuse(t *testing.T) {
	ctx, db, svc := ouvrirBaseLifecycle(t)
	tenant := creerEtablissementLifecycle(t, ctx, db, svc, "PRO", "ACTIVE", "")

	// Connexion volontairement fermée : toute requête doit échouer.
	casse, err := database.ConnectPool(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("connexion de test: %v", err)
	}
	casse.Close()
	cassé := billing.NewStore(database.NewStore(casse.Pool))

	rec, err := cassé.GetSubscription(ctx, tenant)
	if err == nil {
		t.Errorf("GetSubscription sur base injoignable : erreur absente (rec=%+v), attendu une erreur explicite", rec)
	}
}

// Une rétrogradation (ici PRO -> FREE, modifiée directement en base faute de
// route de changement de plan) conserve les données et recalcule immédiatement
// les droits ; les créations au-delà du plafond sont refusées, les lectures non.
func TestLifecycle_Retrogradation_ConserveDonnees(t *testing.T) {
	ctx, db, svc := ouvrirBaseLifecycle(t)
	tenant := creerEtablissementLifecycle(t, ctx, db, svc, "PRO", "ACTIVE", "")
	ajouterCommandes(t, ctx, db, tenant, 60)

	if _, err := db.Pool.Exec(ctx,
		`UPDATE subscriptions SET plan_id = (SELECT id FROM plans WHERE code = 'FREE') WHERE tenant_id = $1`,
		tenant); err != nil {
		t.Fatalf("rétrogradation simulée: %v", err)
	}

	if n := compter(t, ctx, db, `SELECT COUNT(*) FROM orders WHERE tenant_id = $1`, tenant); n != 60 {
		t.Errorf("commandes après rétrogradation = %d, attendu 60", n)
	}
	resp, err := svc.EffectiveEntitlements(ctx, tenant)
	if err != nil {
		t.Fatalf("EffectiveEntitlements: %v", err)
	}
	if resp.PlanCode != billing.PlanFree {
		t.Errorf("plan effectif après rétrogradation = %s, attendu FREE", resp.PlanCode)
	}
	if ok, _ := svc.IsEnabled(ctx, tenant, billing.FeatureReportsAdvanced); ok {
		t.Errorf("reports.advanced reste actif après rétrogradation vers FREE")
	}
	if err := svc.CheckLimit(ctx, tenant, billing.FeatureOrdersMonthlyMax, 60); err == nil {
		t.Errorf("une 61e commande FREE devrait être refusée")
	}
}

// Le prix affiché et utilisé vient du catalogue `plans` (lecture courante).
// Caractérisation : aucun prix n'est copié sur l'abonnement aujourd'hui (R4).
func TestLifecycle_PrixLuDuCatalogue(t *testing.T) {
	ctx, db, svc := ouvrirBaseLifecycle(t)
	tenant := creerEtablissementLifecycle(t, ctx, db, svc, "BUSINESS", "ACTIVE", "")

	var prixCatalogue float64
	if err := db.Pool.QueryRow(ctx, `SELECT price_monthly_xof FROM plans WHERE code = 'BUSINESS'`).Scan(&prixCatalogue); err != nil {
		t.Fatalf("lecture du catalogue: %v", err)
	}
	rec, err := svc.GetSubscription(ctx, tenant)
	if err != nil || rec == nil {
		t.Fatalf("GetSubscription: rec=%+v err=%v", rec, err)
	}
	if rec.PriceMonthly != prixCatalogue {
		t.Errorf("prix de l'abonnement = %v, attendu le prix catalogue %v", rec.PriceMonthly, prixCatalogue)
	}
}

// Isolation des compteurs : les commandes d'un établissement ne consomment pas
// le quota d'un autre (PLAN §3.7).
func TestLifecycle_QuotaIsoleParEtablissement(t *testing.T) {
	ctx, db, svc := ouvrirBaseLifecycle(t)
	a := creerEtablissementLifecycle(t, ctx, db, svc, "FREE", "ACTIVE", "")
	b := creerEtablissementLifecycle(t, ctx, db, svc, "FREE", "ACTIVE", "")
	ajouterCommandes(t, ctx, db, a, 50)

	if err := svc.CheckLimit(ctx, a, billing.FeatureOrdersMonthlyMax, 50); err == nil {
		t.Errorf("établissement A à 50 commandes : la 51e devrait être refusée")
	}
	usageB, err := svc.FeatureUsage(ctx, b, billing.FeatureOrdersMonthlyMax)
	if err != nil {
		t.Fatalf("FeatureUsage(B): %v", err)
	}
	if usageB != 0 {
		t.Errorf("usage du quota de B = %d, attendu 0 : les commandes de A ne doivent pas le consommer", usageB)
	}
	if err := svc.CheckLimit(ctx, b, billing.FeatureOrdersMonthlyMax, usageB); err != nil {
		t.Errorf("établissement B refusé à tort : %v", err)
	}
}
