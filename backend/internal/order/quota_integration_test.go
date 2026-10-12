//go:build integration

package order_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"pressing-os/backend/internal/billing"
	"pressing-os/backend/internal/database"
	"pressing-os/backend/internal/order"
)

// Tests d'intégration du plafond de commandes à la création (PLAN phase 3).
//
// Garanties testées : au plus autant de commandes que le plafond du plan, même
// en création simultanée et entre plusieurs instances du backend ; un échec de
// création ne consomme aucune place ; le verrou d'un établissement ne bloque pas
// les autres.
//
// Exécution :
//
//	TEST_DATABASE_URL="..." go test -tags=integration -count=1 -run 'TestQuotaCommandes_' ./internal/order/...

// instanceCommandes assemble un service de commandes persistant et son garde-fou,
// adossés à un pool dédié (une « instance » du backend).
type instanceCommandes struct {
	db      *database.DB
	billing *billing.Service
	orders  *order.Service
}

func ouvrirInstance(t *testing.T, ctx context.Context, dsn string) *instanceCommandes {
	t.Helper()
	db, err := database.ConnectPool(ctx, dsn)
	if err != nil {
		t.Fatalf("connexion PostgreSQL: %v", err)
	}
	t.Cleanup(db.Close)
	bill := billing.NewStore(database.NewStore(db.Pool))
	svc := order.NewService()
	svc.SetStore(order.NewStore(database.NewStore(db.Pool)))
	svc.SetQuotaGuard(&billing.QuotaGuard{
		Billing: bill,
		Counter: billing.NewDBUsageCounter(database.NewStore(db.Pool)),
	})
	return &instanceCommandes{db: db, billing: bill, orders: svc}
}

// ouvrirCommandes ouvre l'instance principale et la base de test.
func ouvrirCommandes(t *testing.T) (context.Context, *instanceCommandes) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL non défini : test d'intégration ignoré")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)
	return ctx, ouvrirInstance(t, ctx, dsn)
}

// creerEtablissementCommandes crée un établissement avec abonnement FREE actif.
func creerEtablissementCommandes(t *testing.T, ctx context.Context, inst *instanceCommandes, plan string) string {
	t.Helper()
	suffixe := fmt.Sprintf("%d", time.Now().UnixNano())
	var tenant string
	if err := inst.db.Pool.QueryRow(ctx,
		`INSERT INTO tenants (name, slug, city, status) VALUES ($1, $2, 'Abidjan', 'active') RETURNING id`,
		"Pressing Quota "+suffixe, "quota-"+suffixe).Scan(&tenant); err != nil {
		t.Fatalf("établissement: %v", err)
	}
	t.Cleanup(func() { _ = inst.billing.DeleteTenant(context.Background(), tenant) })
	if _, err := inst.db.Pool.Exec(ctx,
		`INSERT INTO subscriptions (tenant_id, plan_id, status)
		 SELECT $1, id, 'ACTIVE' FROM plans WHERE code = $2`, tenant, plan); err != nil {
		t.Fatalf("abonnement: %v", err)
	}
	return tenant
}

// remplirCommandes insère n commandes actives directement en base (amorçage).
func remplirCommandes(t *testing.T, ctx context.Context, inst *instanceCommandes, tenant string, n int) {
	t.Helper()
	var client string
	if err := inst.db.Pool.QueryRow(ctx,
		`INSERT INTO customers (tenant_id, first_name, last_name, phone)
		 VALUES ($1, 'Amorçage', 'Quota', '+225 07 00 00 00') RETURNING id`, tenant).Scan(&client); err != nil {
		t.Fatalf("client d'amorçage: %v", err)
	}
	for i := 0; i < n; i++ {
		if _, err := inst.db.Pool.Exec(ctx,
			`INSERT INTO orders (tenant_id, customer_id, order_number, status, payment_status, total_amount, paid_amount)
			 VALUES ($1, $2, $3, 'received', 'unpaid', 1000, 0)`,
			tenant, client, fmt.Sprintf("CMD-SEED-%04d", i+1)); err != nil {
			t.Fatalf("commande d'amorçage %d: %v", i+1, err)
		}
	}
}

// requeteCommande : une commande d'un article, client distinct selon i.
func requeteCommande(i int) order.CreateOrderRequest {
	return order.CreateOrderRequest{
		CustomerName:  fmt.Sprintf("Client Concurrent %d", i),
		CustomerPhone: fmt.Sprintf("+225 07 11 %02d %02d", i/100%100, i%100),
		Items:         []order.CreateOrderItemInput{{ItemLabel: "Chemise", Quantity: 1, UnitPrice: 1000}},
		InitialPaid:   1000,
		PaymentMethod: order.MethodCash,
	}
}

// nombreCommandes compte les commandes persistées de l'établissement.
func nombreCommandes(t *testing.T, ctx context.Context, inst *instanceCommandes, tenant string) int64 {
	t.Helper()
	var n int64
	if err := inst.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM orders WHERE tenant_id = $1`, tenant).Scan(&n); err != nil {
		t.Fatalf("comptage des commandes: %v", err)
	}
	return n
}

// lancerSimultanees exécute n créations en parallèle (départ simultané) et
// retourne le nombre d'acceptées, de refus métier et d'autres erreurs.
func lancerSimultanees(ctx context.Context, n int, instances []*instanceCommandes, tenant string) (acceptees, refusees, autres int64) {
	var (
		depart sync.WaitGroup
		fin    sync.WaitGroup
		a, r   atomic.Int64
		x      atomic.Int64
	)
	depart.Add(1)
	for i := 0; i < n; i++ {
		fin.Add(1)
		inst := instances[i%len(instances)]
		go func(i int, inst *instanceCommandes) {
			defer fin.Done()
			depart.Wait()
			_, err := inst.orders.CreateOrder(ctx, tenant, requeteCommande(i))
			switch {
			case err == nil:
				a.Add(1)
			case billing.IsRefusMetier(err):
				r.Add(1)
			default:
				x.Add(1)
			}
		}(i, inst)
	}
	depart.Done()
	fin.Wait()
	return a.Load(), r.Load(), x.Load()
}

// Dix créations simultanées autour du dernier emplacement FREE : une seule passe.
func TestQuotaCommandes_CreationsConcurrentes_AuDernierEmplacement(t *testing.T) {
	ctx, inst := ouvrirCommandes(t)
	tenant := creerEtablissementCommandes(t, ctx, inst, "FREE")
	remplirCommandes(t, ctx, inst, tenant, 49)

	acceptees, refusees, autres := lancerSimultanees(ctx, 10, []*instanceCommandes{inst}, tenant)
	if acceptees != 1 || refusees != 9 || autres != 0 {
		t.Errorf("acceptées=%d refusées=%d autres=%d, attendu 1/9/0", acceptees, refusees, autres)
	}
	if n := nombreCommandes(t, ctx, inst, tenant); n != 50 {
		t.Errorf("commandes persistées = %d, attendu 50 (plafond FREE)", n)
	}
}

// Deux instances du backend (deux pools) se disputent le même dernier emplacement :
// le verrou vit dans PostgreSQL, donc la garantie tient entre processus.
func TestQuotaCommandes_MultiInstance_AuDernierEmplacement(t *testing.T) {
	ctx, a := ouvrirCommandes(t)
	b := ouvrirInstance(t, ctx, os.Getenv("TEST_DATABASE_URL"))
	tenant := creerEtablissementCommandes(t, ctx, a, "FREE")
	remplirCommandes(t, ctx, a, tenant, 49)

	acceptees, refusees, autres := lancerSimultanees(ctx, 8, []*instanceCommandes{a, b}, tenant)
	if acceptees != 1 || refusees != 7 || autres != 0 {
		t.Errorf("acceptées=%d refusées=%d autres=%d, attendu 1/7/0", acceptees, refusees, autres)
	}
	if n := nombreCommandes(t, ctx, a, tenant); n != 50 {
		t.Errorf("commandes persistées = %d, attendu 50", n)
	}
}

// Une création qui échoue (rollback) ne consomme aucune place du quota.
func TestQuotaCommandes_CreationEchouee_NeConsommePasLeQuota(t *testing.T) {
	ctx, inst := ouvrirCommandes(t)
	tenant := creerEtablissementCommandes(t, ctx, inst, "FREE")
	remplirCommandes(t, ctx, inst, tenant, 49)

	// Prix négatif : rejeté par la contrainte CHECK de order_items, donc rollback.
	echec := requeteCommande(900)
	echec.Items[0].UnitPrice = -1
	_, err := inst.orders.CreateOrder(ctx, tenant, echec)
	if err == nil {
		t.Fatalf("création au prix négatif : erreur attendue")
	}
	if billing.IsRefusMetier(err) {
		t.Fatalf("l'échec technique ne doit pas être présenté comme un refus de formule : %v", err)
	}
	if n := nombreCommandes(t, ctx, inst, tenant); n != 49 {
		t.Fatalf("après échec : commandes = %d, attendu 49", n)
	}

	// La 50e création valide passe, puis la 51e est refusée.
	if _, err := inst.orders.CreateOrder(ctx, tenant, requeteCommande(901)); err != nil {
		t.Errorf("50e commande valide : %v", err)
	}
	_, err = inst.orders.CreateOrder(ctx, tenant, requeteCommande(902))
	if !billing.IsRefusMetier(err) || !errors.Is(err, billing.ErrLimitReached) {
		t.Errorf("51e commande : refus ErrLimitReached attendu, obtenu %v", err)
	}
}

// Le verrou d'un établissement ne bloque pas les autres : pendant qu'une création
// de A tient le verrou, B crée sans attendre ; la création de A attend puis est
// refusée, puisque la commande en cours a pris la dernière place.
func TestQuotaCommandes_Isolement_VerrouParEtablissement(t *testing.T) {
	ctx, inst := ouvrirCommandes(t)
	tenantA := creerEtablissementCommandes(t, ctx, inst, "FREE")
	tenantB := creerEtablissementCommandes(t, ctx, inst, "FREE")
	remplirCommandes(t, ctx, inst, tenantA, 49)

	// Création en cours de A, simulée à la main : verrou du quota puis insertion, sans COMMIT.
	tx, err := inst.db.Pool.Begin(ctx)
	if err != nil {
		t.Fatalf("transaction: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`,
		"quota-commandes:"+tenantA); err != nil {
		t.Fatalf("verrou: %v", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO orders (tenant_id, customer_id, order_number, status, payment_status, total_amount, paid_amount)
		 SELECT $1, c.id, 'CMD-INFLIGHT', 'received', 'unpaid', 1000, 0 FROM customers c WHERE c.tenant_id = $1 LIMIT 1`,
		tenantA); err != nil {
		t.Fatalf("insertion en cours: %v", err)
	}

	// A attend le verrou.
	resultatA := make(chan error, 1)
	go func() {
		_, err := inst.orders.CreateOrder(ctx, tenantA, requeteCommande(700))
		resultatA <- err
	}()
	select {
	case err := <-resultatA:
		t.Fatalf("création de A exécutée pendant le verrou (err=%v) : sérialisation absente", err)
	case <-time.After(300 * time.Millisecond):
	}

	// B n'attend pas.
	bDone := make(chan error, 1)
	go func() {
		_, err := inst.orders.CreateOrder(ctx, tenantB, requeteCommande(800))
		bDone <- err
	}()
	select {
	case err := <-bDone:
		if err != nil {
			t.Errorf("création de B : %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("création de B bloquée par le verrou de A : isolement absent")
	}

	// Fin de la création de A : la sienne est alors refusée.
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	select {
	case err := <-resultatA:
		if !billing.IsRefusMetier(err) {
			t.Errorf("création de A après libération : refus attendu, obtenu %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("création de A toujours bloquée après libération du verrou")
	}
}
