package order

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewService(t *testing.T) {
	t.Parallel()
	service := NewService()
	assert.NotNil(t, service)
	assert.NotNil(t, service.orders)
	assert.NotNil(t, service.tenantSeqNum)
	assert.NotNil(t, service.logger)
}

func TestCreateOrder_EmptyItems(t *testing.T) {
	t.Parallel()
	service := NewService()
	ctx := context.Background()
	tenantID := "test-tenant-1"

	req := CreateOrderRequest{
		CustomerID:    "cust-1",
		CustomerName:  "Test Client",
		CustomerPhone: "+22500000000",
		Items:         []CreateOrderItemInput{}, // Aucun item
	}

	order, err := service.CreateOrder(ctx, tenantID, req)
	assert.ErrorIs(t, err, ErrEmptyOrderItems)
	assert.Nil(t, order)
}

func TestCreateOrder_Success(t *testing.T) {
	t.Parallel()
	service := NewService()
	ctx := context.Background()
	tenantID := "test-tenant-1"

	req := CreateOrderRequest{
		CustomerID:    "cust-1",
		CustomerName:  "Test Client",
		CustomerPhone: "+22500000000",
		Items: []CreateOrderItemInput{
			{
				ServiceID: "srv-1",
				ItemLabel: "Chemise",
				Quantity:  2,
				UnitPrice: 1500,
			},
		},
		InitialPaid:   2000,
		PaymentMethod: MethodCash,
	}

	order, err := service.CreateOrder(ctx, tenantID, req)
	require.NoError(t, err)
	require.NotNil(t, order)

	assert.Equal(t, tenantID, order.TenantID)
	assert.Equal(t, 3000.0, order.TotalAmount)
	assert.Equal(t, 2000.0, order.PaidAmount)
	assert.Equal(t, 1000.0, order.RemainingDue)
	assert.Equal(t, PaymentStatusPartial, order.PaymentStatus)
	assert.Equal(t, StatusReceived, order.Status)
	assert.Len(t, order.Items, 1)
	assert.Len(t, order.Payments, 1)
}

func TestGetOrderByID_NotFound(t *testing.T) {
	t.Parallel()
	service := NewService()
	ctx := context.Background()
	tenantID := "test-tenant-1"

	order, err := service.GetOrderByID(ctx, tenantID, "ord-inexistant")
	assert.ErrorIs(t, err, ErrOrderNotFound)
	assert.Nil(t, order)
}

func TestUpdateOrderStatus_Invalid(t *testing.T) {
	t.Parallel()
	service := NewService()
	ctx := context.Background()
	tenantID := "a0000000-0000-0000-0000-000000000001" // Tenant par défaut

	// Récupère une commande existante
	orders, err := service.GetOrders(ctx, tenantID, "", "")
	require.NoError(t, err)
	require.Len(t, orders, 3)
	firstOrder := orders[0]

	// Tentative de mise à jour avec un statut invalide
	updated, err := service.UpdateOrderStatus(ctx, tenantID, firstOrder.ID, "statut_invalide")
	assert.ErrorIs(t, err, ErrInvalidStatusOrder)
	assert.Nil(t, updated)
}

func TestUpdateOrderStatus_Success(t *testing.T) {
	t.Parallel()
	service := NewService()
	ctx := context.Background()
	tenantID := "a0000000-0000-0000-0000-000000000001"

	orders, err := service.GetOrders(ctx, tenantID, "", "")
	require.NoError(t, err)
	firstOrder := orders[0]

	updated, err := service.UpdateOrderStatus(ctx, tenantID, firstOrder.ID, StatusReady)
	require.NoError(t, err)
	assert.Equal(t, StatusReady, updated.Status)
}

func TestAddPayment_Success(t *testing.T) {
	t.Parallel()
	service := NewService()
	ctx := context.Background()
	tenantID := "a0000000-0000-0000-0000-000000000001"

	orders, err := service.GetOrders(ctx, tenantID, StatusWashing, "")
	require.NoError(t, err)
	require.Len(t, orders, 1)
	order := orders[0] // Celle avec paiement partiel

	initialPaid := order.PaidAmount
	paymentReq := AddPaymentRequest{
		Amount:   3500,
		PaymentMethod: MethodCash,
		Reference: "TEST-REF",
	}

	updated, err := service.AddPayment(ctx, tenantID, order.ID, paymentReq)
	require.NoError(t, err)
	assert.Equal(t, initialPaid+3500, updated.PaidAmount)
	assert.Equal(t, 0.0, updated.RemainingDue)
	assert.Equal(t, PaymentStatusPaid, updated.PaymentStatus)
	assert.Len(t, updated.Payments, 2)
}

func TestIsolationMultitenant(t *testing.T) {
	t.Parallel()
	service := NewService()
	ctx := context.Background()

	// Crée un deuxième tenant
	tenant2 := "test-tenant-2"
	req := CreateOrderRequest{
		CustomerID:    "cust-2",
		CustomerName:  "Client Tenant 2",
		CustomerPhone: "+225111111111",
		Items: []CreateOrderItemInput{
			{ServiceID: "srv-1", ItemLabel: "Test", Quantity: 1, UnitPrice: 1000},
		},
	}
	order2, err := service.CreateOrder(ctx, tenant2, req)
	require.NoError(t, err)

	// Tentative d'accès depuis le tenant par défaut
	defaultTenant := "a0000000-0000-0000-0000-000000000001"
	accessed, err := service.GetOrderByID(ctx, defaultTenant, order2.ID)
	assert.ErrorIs(t, err, ErrOrderNotFound)
	assert.Nil(t, accessed)

	// Les commandes du tenant par défaut ne sont pas accessibles depuis tenant2
	ordersDefault, err := service.GetOrders(ctx, defaultTenant, "", "")
	require.NoError(t, err)
	if len(ordersDefault) > 0 {
		_, err := service.GetOrderByID(ctx, tenant2, ordersDefault[0].ID)
		assert.ErrorIs(t, err, ErrOrderNotFound)
	}
}