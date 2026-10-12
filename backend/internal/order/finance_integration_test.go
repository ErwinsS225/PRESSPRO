//go:build integration

package order_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"pressing-os/backend/internal/order"
)

func TestFinance_Integration_DepotPaiementMonnaieEtPersistance(t *testing.T) {
	ctx, inst := ouvrirCommandes(t)
	tenant := creerEtablissementCommandes(t, ctx, inst, "FREE")

	received := 5000.0
	req := requeteCommande(1)
	req.Items = []order.CreateOrderItemInput{
		{ItemLabel: "Costume 2 pièces", Quantity: 2, UnitPrice: 1500},
	}
	req.InitialPaid = 0
	req.AmountReceived = &received
	req.PaymentMethod = order.MethodWave
	req.IdempotencyKey = "key-depot-001"

	ord, err := inst.orders.CreateOrder(ctx, tenant, req)
	require.NoError(t, err)
	require.True(t, ord.Persisted)
	require.Equal(t, 3000.0, ord.TotalAmount)
	require.Equal(t, 3000.0, ord.PaidAmount)
	require.Equal(t, 0.0, ord.RemainingDue)
	require.Equal(t, order.PaymentStatusPaid, ord.PaymentStatus)
	require.Len(t, ord.Payments, 1)

	pay := ord.Payments[0]
	require.Equal(t, 3000.0, pay.Amount)
	require.Equal(t, order.MethodWave, pay.PaymentMethod)
	require.NotNil(t, pay.AmountReceived)
	require.Equal(t, 5000.0, *pay.AmountReceived)
	require.NotNil(t, pay.ChangeGiven)
	require.Equal(t, 2000.0, *pay.ChangeGiven)

	// Rejeu identique avec la même clé d'idempotence : même résultat, aucun doublon.
	replayed, err := inst.orders.CreateOrder(ctx, tenant, req)
	require.NoError(t, err)
	require.Equal(t, ord.ID, replayed.ID)
	require.Equal(t, ord.OrderNumber, replayed.OrderNumber)

	// Tentative avec la même clé mais corps modifié : refus conflit 409.
	reqMod := req
	reqMod.Items = []order.CreateOrderItemInput{{ItemLabel: "Robe", Quantity: 1, UnitPrice: 1000}}
	_, err = inst.orders.CreateOrder(ctx, tenant, reqMod)
	require.ErrorIs(t, err, order.ErrIdempotencyConflict)

	// La base contient exactement 1 commande et 1 paiement.
	var countOrders, countPayments int
	require.NoError(t, inst.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM orders WHERE tenant_id=$1", tenant).Scan(&countOrders))
	require.Equal(t, 1, countOrders)
	require.NoError(t, inst.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM payments WHERE tenant_id=$1", tenant).Scan(&countPayments))
	require.Equal(t, 1, countPayments)
}

func TestFinance_Integration_RefusRestitutionSiSoldeNonNul(t *testing.T) {
	ctx, inst := ouvrirCommandes(t)
	tenant := creerEtablissementCommandes(t, ctx, inst, "FREE")

	// Injection directe d'une commande historique partiellement payée (1000 payé sur 2500).
	c := clientDepot(t, ctx, inst, tenant, "Awa", "+2250700000001")
	var orderID string
	require.NoError(t, inst.db.Pool.QueryRow(ctx, `
		INSERT INTO orders (tenant_id, customer_id, order_number, status, payment_status, total_amount, paid_amount)
		VALUES ($1, $2, 'CMD-HIST-01', 'ready', 'partially_paid', 2500, 1000)
		RETURNING id`, tenant, c.ID).Scan(&orderID))
	_, err := inst.db.Pool.Exec(ctx, `
		INSERT INTO payments (tenant_id, order_id, amount, payment_method)
		VALUES ($1, $2, 1000, 'cash')`, tenant, orderID)
	require.NoError(t, err)

	// Tentative de passage à delivered : STRICTEMENT REFUSÉ par le serveur.
	_, err = inst.orders.UpdateOrderStatus(ctx, tenant, orderID, order.StatusDelivered)
	require.ErrorIs(t, err, order.ErrUnpaidDelivery)

	// La commande reste ready.
	reloaded, err := inst.orders.GetOrderByID(ctx, tenant, orderID)
	require.NoError(t, err)
	require.Equal(t, order.StatusReady, reloaded.Status)
	require.Equal(t, 1500.0, reloaded.RemainingDue)

	// Règlement du solde avec monnaie rendue (somme remise 2000, solde 1500 -> monnaie 500).
	remis := 2000.0
	settled, err := inst.orders.AddPayment(ctx, tenant, orderID, order.AddPaymentRequest{
		Amount:         1500,
		AmountReceived: &remis,
		PaymentMethod:  order.MethodOrangeMoney,
		IdempotencyKey: "pay-solde-001",
	})
	require.NoError(t, err)
	require.Equal(t, 2500.0, settled.PaidAmount)
	require.Equal(t, 0.0, settled.RemainingDue)
	require.Equal(t, order.PaymentStatusPaid, settled.PaymentStatus)
	require.Len(t, settled.Payments, 2)
	lastPay := settled.Payments[1]
	require.Equal(t, 1500.0, lastPay.Amount)
	require.Equal(t, 2000.0, *lastPay.AmountReceived)
	require.Equal(t, 500.0, *lastPay.ChangeGiven)

	// Maintenant que le solde est nul, la restitution est autorisée !
	delivered, err := inst.orders.UpdateOrderStatus(ctx, tenant, orderID, order.StatusDelivered)
	require.NoError(t, err)
	require.Equal(t, order.StatusDelivered, delivered.Status)
}

func TestFinance_Integration_ConcurrencePaiements(t *testing.T) {
	ctx, inst := ouvrirCommandes(t)
	tenant := creerEtablissementCommandes(t, ctx, inst, "FREE")

	c := clientDepot(t, ctx, inst, tenant, "Awa", "+2250700000002")
	var orderID string
	require.NoError(t, inst.db.Pool.QueryRow(ctx, `
		INSERT INTO orders (tenant_id, customer_id, order_number, status, payment_status, total_amount, paid_amount)
		VALUES ($1, $2, 'CMD-RACE-01', 'ready', 'unpaid', 1000, 0)
		RETURNING id`, tenant, c.ID).Scan(&orderID))

	// Deux règlements de 1000 tentés simultanément : un seul doit réussir, l'autre doit être refusé ErrPaymentExceedsDue.
	var wg sync.WaitGroup
	var successCount, failCount int
	var mu sync.Mutex

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := inst.orders.AddPayment(ctx, tenant, orderID, order.AddPaymentRequest{
				Amount:         1000,
				PaymentMethod:  order.MethodCash,
				IdempotencyKey: fmt.Sprintf("race-pay-%d", idx),
			})
			mu.Lock()
			if err == nil {
				successCount++
			} else {
				failCount++
			}
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	require.Equal(t, 1, successCount)
	require.Equal(t, 1, failCount)

	reloaded, err := inst.orders.GetOrderByID(ctx, tenant, orderID)
	require.NoError(t, err)
	require.Equal(t, 1000.0, reloaded.PaidAmount)
	require.Equal(t, 0.0, reloaded.RemainingDue)
	require.Len(t, reloaded.Payments, 1)
}
