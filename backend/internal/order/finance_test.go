package order

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func financialRequest(paid float64) CreateOrderRequest {
	return CreateOrderRequest{CustomerName: "Awa Kouadio", CustomerPhone: "0700000000",
		Items: []CreateOrderItemInput{{ItemLabel: "Chemise", Quantity: 1, UnitPrice: 1000}}, InitialPaid: paid, PaymentMethod: MethodCash}
}

func TestFinance_DepotExigeIntegral(t *testing.T) {
	for _, amount := range []float64{0, 500, -1} {
		s := NewService()
		_, err := s.CreateOrder(context.Background(), "tenant", financialRequest(amount))
		require.Error(t, err)
		require.Empty(t, s.orders)
	}
}
func TestFinance_RefuseRestitutionHistoriqueImpayee(t *testing.T) {
	s := NewService()
	s.orders["old"] = &Order{ID: "old", TenantID: "tenant", Status: StatusReady, TotalAmount: 1000, PaidAmount: 500, RemainingDue: 500}
	_, err := s.UpdateOrderStatus(context.Background(), "tenant", "old", StatusDelivered)
	require.Error(t, err)
	require.Equal(t, StatusReady, s.orders["old"].Status)
}
func TestFinance_QuantiteInvalideRefusee(t *testing.T) {
	req := financialRequest(1000)
	req.Items[0].Quantity = 0
	_, err := NewService().CreateOrder(context.Background(), "tenant", req)
	require.Error(t, err)
}
