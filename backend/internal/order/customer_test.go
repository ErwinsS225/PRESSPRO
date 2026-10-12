package order

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"pressing-os/backend/internal/auth"
	"pressing-os/backend/internal/customer"
)

func TestNormalizeOrderCustomer(t *testing.T) {
	tests := []struct {
		name string
		req  CreateOrderRequest
		err  error
	}{
		{"empty", CreateOrderRequest{}, customer.ErrEmptyFirstName},
		{"phone required", CreateOrderRequest{CustomerName: "Awa", CustomerPhone: " "}, customer.ErrEmptyPhone},
		{"legacy valid", CreateOrderRequest{CustomerName: " Awa Kouadio ", CustomerPhone: " 0700000000 "}, nil},
		{"existing ID only", CreateOrderRequest{CustomerID: " client "}, nil},
		{"structured", CreateOrderRequest{NewCustomer: &customer.CreateCustomerRequest{FirstName: "Jean Pierre", LastName: "Kouadio", Phone: "0700000000"}}, nil},
		{"ID plus new", CreateOrderRequest{CustomerID: "client", NewCustomer: &customer.CreateCustomerRequest{}}, ErrAmbiguousCustomer},
		{"legacy plus new", CreateOrderRequest{CustomerName: "Awa", NewCustomer: &customer.CreateCustomerRequest{}}, ErrAmbiguousCustomer},
		{"structured missing phone", CreateOrderRequest{NewCustomer: &customer.CreateCustomerRequest{FirstName: "Awa"}}, customer.ErrEmptyPhone},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := normalizeOrderCustomer(tc.req)
			require.ErrorIs(t, err, tc.err)
		})
	}
}

func TestDepotClient_MemoireRefuseIdentiteNonVerifiable(t *testing.T) {
	svc := NewService()
	_, err := svc.CreateOrder(context.Background(), "tenant", CreateOrderRequest{
		CustomerID: "foreign-customer", CustomerName: "Awa", CustomerPhone: "0700000000",
		Items:       []CreateOrderItemInput{{ItemLabel: "Chemise", Quantity: 1, UnitPrice: 1000}},
		InitialPaid: 1000,
	})
	require.ErrorIs(t, err, customer.ErrCustomerNotFound)
	orders, err := svc.GetOrders(context.Background(), "tenant", "", "")
	require.NoError(t, err)
	require.Empty(t, orders)
}

func TestDepotClient_HandlerAccepteObjetStructure(t *testing.T) {
	h := NewHandler(NewService())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewBufferString(`{
		"new_customer":{"first_name":"Jean Pierre","last_name":"Kouadio","phone":"0700000000"},
		"items":[{"item_label":"Chemise","quantity":1,"unit_price":1000}],
		"initial_paid": 1000
	}`))
	req = req.WithContext(auth.WithTenantID(req.Context(), "tenant"))
	rec := httptest.NewRecorder()
	h.CreateOrder(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "Jean Pierre Kouadio")
}

func TestDepotClient_ErreursHTTP(t *testing.T) {
	h := NewHandler(NewService())
	for _, tc := range []struct {
		err    error
		status int
	}{
		{customer.ErrCustomerNotFound, http.StatusNotFound},
		{customer.ErrEmptyPhone, http.StatusBadRequest},
		{ErrAmbiguousCustomer, http.StatusBadRequest},
		{ErrCustomerSelectionRequired, http.StatusConflict},
		{errors.New("sql secret detail"), http.StatusInternalServerError},
	} {
		rec := httptest.NewRecorder()
		h.respondServiceError(rec, tc.err, "échec de création")
		require.Equal(t, tc.status, rec.Code)
		require.NotContains(t, rec.Body.String(), "sql secret detail")
	}
}
