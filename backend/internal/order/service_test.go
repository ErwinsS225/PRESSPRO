package order_test

import (
	"context"
	"testing"

	"pressing-os/backend/internal/order"
)

func TestCreateOrderAndCalculation(t *testing.T) {
	svc := order.NewService()
	ctx := context.Background()
	tenantA := "tenant-cocody-001"

	req := order.CreateOrderRequest{
		CustomerName:  "Gnamien Konan",
		CustomerPhone: "+225 07 10 20 30 40",
		Items: []order.CreateOrderItemInput{
			{ServiceID: "srv-1", ItemLabel: "Chemise classique", Quantity: 3, UnitPrice: 1000},
			{ServiceID: "srv-2", ItemLabel: "Costume 2 pièces", Quantity: 1, UnitPrice: 3500},
		},
		InitialPaid:   2500,
		PaymentMethod: order.MethodWave,
		DueDate:       "Demain 18:00",
		Notes:         "Plis marqués",
	}

	ord, err := svc.CreateOrder(ctx, tenantA, req)
	if err != nil {
		t.Fatalf("échec création commande: %v", err)
	}

	// 3*1000 + 1*3500 = 6500 FCFA
	if ord.TotalAmount != 6500 {
		t.Errorf("attendu TotalAmount 6500, obtenu %f", ord.TotalAmount)
	}
	if ord.PaidAmount != 2500 {
		t.Errorf("attendu PaidAmount 2500, obtenu %f", ord.PaidAmount)
	}
	if ord.RemainingDue != 4000 {
		t.Errorf("attendu RemainingDue 4000, obtenu %f", ord.RemainingDue)
	}
	if ord.PaymentStatus != order.PaymentStatusPartial {
		t.Errorf("attendu statut partiel, obtenu %s", ord.PaymentStatus)
	}
	if ord.Status != order.StatusReceived {
		t.Errorf("attendu statut initial 'received', obtenu %s", ord.Status)
	}
	if len(ord.Payments) != 1 || ord.Payments[0].PaymentMethod != order.MethodWave {
		t.Errorf("attendu paiement initial Wave")
	}
}

func TestOrderMultiTenantIsolation(t *testing.T) {
	svc := order.NewService()
	ctx := context.Background()

	tenantCocody := "tenant-cocody"
	tenantPlateau := "tenant-plateau"

	// Créer une commande pour Cocody
	ordCocody, err := svc.CreateOrder(ctx, tenantCocody, order.CreateOrderRequest{
		CustomerName:  "Client Cocody",
		CustomerPhone: "+225 07 00 00 01",
		Items:         []order.CreateOrderItemInput{{ItemLabel: "Chemise", Quantity: 1, UnitPrice: 1000}},
	})
	if err != nil {
		t.Fatalf("erreur création: %v", err)
	}

	// Le pressing du Plateau tente de consulter la commande de Cocody -> doit échouer (ErrOrderNotFound)
	_, err = svc.GetOrderByID(ctx, tenantPlateau, ordCocody.ID)
	if err != order.ErrOrderNotFound {
		t.Errorf("l'isolation a échoué : Plateau ne doit pas pouvoir lire la commande de Cocody, obtenu err=%v", err)
	}

	// Liste des commandes de Plateau ne doit pas contenir la commande de Cocody
	listPlateau, err := svc.GetOrders(ctx, tenantPlateau, "", "")
	if err != nil {
		t.Fatalf("erreur get orders: %v", err)
	}
	for _, o := range listPlateau {
		if o.ID == ordCocody.ID {
			t.Errorf("fuite de données multi-tenant : commande Cocody trouvée dans la liste de Plateau")
		}
	}
}

func TestUpdateStatusAndPayment(t *testing.T) {
	svc := order.NewService()
	ctx := context.Background()
	tenant := "tenant-test"

	ord, _ := svc.CreateOrder(ctx, tenant, order.CreateOrderRequest{
		CustomerName:  "Aya Traoré",
		CustomerPhone: "+225 05 99 88 77",
		Items:         []order.CreateOrderItemInput{{ItemLabel: "Robe", Quantity: 1, UnitPrice: 4000}},
		InitialPaid:   0,
	})

	// 1. Avancement du statut
	updated, err := svc.UpdateOrderStatus(ctx, tenant, ord.ID, order.StatusWashing)
	if err != nil || updated.Status != order.StatusWashing {
		t.Errorf("erreur avancement statut: %v", err)
	}

	// 2. Règlement du solde au retrait (4000 FCFA par Orange Money)
	settled, err := svc.AddPayment(ctx, tenant, ord.ID, order.AddPaymentRequest{
		Amount:        4000,
		PaymentMethod: order.MethodOrangeMoney,
		Reference:     "OM-CI-45892",
	})
	if err != nil {
		t.Fatalf("erreur paiement solde: %v", err)
	}

	if settled.PaymentStatus != order.PaymentStatusPaid {
		t.Errorf("attendu statut 'paid', obtenu %s", settled.PaymentStatus)
	}
	if settled.RemainingDue != 0 {
		t.Errorf("attendu solde restant 0, obtenu %f", settled.RemainingDue)
	}
}
