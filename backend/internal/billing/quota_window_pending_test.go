//go:build integration && phase3

package billing_test

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"pressing-os/backend/internal/billing"
	"pressing-os/backend/internal/database"
)

// Tests de caractérisation de la PHASE 3 (quota des commandes sur 30 jours,
// exclusion des commandes annulées, concurrence au dernier emplacement).
//
// Ils sont volontairement derrière le tag `phase3` : ils reproduisent les
// défauts actuels (mois civil, commandes annulées comptées, vérification non
// atomique) et échouent tant que la phase 3 n'est pas implémentée. Ils ne sont
// pas exécutés par la CI par défaut.
//
// Exécution :
//
//	TEST_DATABASE_URL="..." go test -tags='integration phase3' -count=1 -run 'Phase3|Window|Cancel|Concurr' ./internal/billing/...

// Règle retenue (réponse utilisateur) : le début de période est trial_started_at,
// à défaut created_at ; la fenêtre est de 30 jours glissants à partir de ce début.
// Les commandes annulées (status = 'cancelled') ne comptent pas.

// Une commande annulée ne consomme pas le quota. Défaut attendu : CountMonthlyOrders
// compte toutes les commandes, annulées incluses.
func TestPhase3_CommandeAnnulee_NeConsommePasLeQuota(t *testing.T) {
	ctx, db, svc := ouvrirBaseLifecycle(t)
	tenant := creerEtablissementLifecycle(t, ctx, db, svc, "FREE", "ACTIVE", "")
	ajouterCommandes(t, ctx, db, tenant, 49)
	if _, err := db.Pool.Exec(ctx,
		`INSERT INTO orders (tenant_id, customer_id, order_number, status, payment_status, total_amount, paid_amount)
		 SELECT $1, c.id, 'CMD-LC-ANNULEE', 'cancelled', 'unpaid', 1000, 0
		 FROM customers c WHERE c.tenant_id = $1 LIMIT 1`, tenant); err != nil {
		t.Fatalf("commande annulée: %v", err)
	}

	guard := &billing.QuotaGuard{Billing: svc, Counter: billing.NewDBUsageCounter(database.NewStore(db.Pool))}
	if err := guard.CheckOrdersCreation(ctx, tenant); err != nil {
		t.Errorf("49 commandes actives + 1 annulée : la 50e devrait être autorisée, obtenu %v", err)
	}
}

// La consommation est mesurée sur une fenêtre de 30 jours ancrée sur le début de
// période de l'abonnement, et non sur le mois civil. Défaut attendu : le compteur
// utilise date_trunc('month', NOW()).
func TestPhase3_FenetreDe30Jours_AncreeSurLaPeriode(t *testing.T) {
	ctx, db, svc := ouvrirBaseLifecycle(t)
	// Début de période il y a 35 jours : la fenêtre courante commence il y a 5 jours.
	tenant := creerEtablissementLifecycle(t, ctx, db, svc, "FREE", "ACTIVE", "")
	if _, err := db.Pool.Exec(ctx,
		`UPDATE subscriptions SET trial_started_at = NOW() - INTERVAL '35 days',
		        created_at = NOW() - INTERVAL '35 days' WHERE tenant_id = $1`, tenant); err != nil {
		t.Fatalf("ancrage de la période: %v", err)
	}
	// Un client de test (sans commande) pour rattacher les commandes datées.
	ajouterCommandes(t, ctx, db, tenant, 0)
	var client string
	if err := db.Pool.QueryRow(ctx, `SELECT id FROM customers WHERE tenant_id = $1 LIMIT 1`, tenant).Scan(&client); err != nil {
		t.Fatalf("client: %v", err)
	}
	// Une commande dans la fenêtre précédente (il y a 10 jours) et une dans la
	// fenêtre courante (il y a 2 jours) : seule la seconde doit compter.
	for i, age := range []string{"10 days", "2 days"} {
		if _, err := db.Pool.Exec(ctx,
			`INSERT INTO orders (tenant_id, customer_id, order_number, status, payment_status, total_amount, paid_amount, created_at)
			 VALUES ($1, $2, $3, 'received', 'unpaid', 1000, 0, NOW() - $4::interval)`,
			tenant, client, fmt.Sprintf("CMD-WIN-%d", i+1), age); err != nil {
			t.Fatalf("commande %s: %v", age, err)
		}
	}

	usage, err := billing.NewDBUsageCounter(database.NewStore(db.Pool)).CountMonthlyOrders(ctx, tenant)
	if err != nil {
		t.Fatalf("CountMonthlyOrders: %v", err)
	}
	if usage != 1 {
		t.Errorf("commandes sur la fenêtre de 30 jours = %d, attendu 1 (commande d'il y a 10 jours hors fenêtre)", usage)
	}
}

// Plusieurs créations simultanées autour du dernier emplacement disponible ne
// doivent pas dépasser le plafond. Défaut attendu : contrôle « compter puis
// vérifier » sans verrou ni transaction.
func TestPhase3_CreationsConcurrentes_AuDernierEmplacement(t *testing.T) {
	ctx, db, svc := ouvrirBaseLifecycle(t)
	tenant := creerEtablissementLifecycle(t, ctx, db, svc, "FREE", "ACTIVE", "")
	ajouterCommandes(t, ctx, db, tenant, 49)

	guard := &billing.QuotaGuard{Billing: svc, Counter: billing.NewDBUsageCounter(database.NewStore(db.Pool))}
	var client string
	if err := db.Pool.QueryRow(ctx, `SELECT id FROM customers WHERE tenant_id = $1 LIMIT 1`, tenant).Scan(&client); err != nil {
		t.Fatalf("client: %v", err)
	}

	const parallele = 10
	var (
		reussies atomic.Int64
		depart   sync.WaitGroup
		fin      sync.WaitGroup
	)
	depart.Add(1)
	for i := 0; i < parallele; i++ {
		fin.Add(1)
		go func(i int) {
			defer fin.Done()
			depart.Wait()
			if err := guard.CheckOrdersCreation(ctx, tenant); err != nil {
				return
			}
			if _, err := db.Pool.Exec(ctx,
				`INSERT INTO orders (tenant_id, customer_id, order_number, status, payment_status, total_amount, paid_amount)
				 VALUES ($1, $2, $3, 'received', 'unpaid', 1000, 0)`,
				tenant, client, fmt.Sprintf("CMD-RACE-%d-%d", time.Now().UnixNano(), i)); err == nil {
				reussies.Add(1)
			}
		}(i)
	}
	depart.Done()
	fin.Wait()

	if n := reussies.Load(); n != 1 {
		t.Errorf("créations acceptées = %d, attendu exactement 1 (dernier emplacement du plan FREE)", n)
	}
}
