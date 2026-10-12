package customer

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"pressing-os/backend/internal/auth"
)

func TestNormalizeCreateRequest(t *testing.T) {
	valid := CreateCustomerRequest{FirstName: " Jean Pierre ", LastName: " Kouadio ", Phone: " +2250700000000 "}
	got, err := NormalizeCreateRequest(valid)
	require.NoError(t, err)
	require.Equal(t, "Jean Pierre", got.FirstName)
	require.Equal(t, "Kouadio", got.LastName)
	require.Equal(t, "+2250700000000", got.Phone)
	for _, tc := range []struct {
		name string
		req  CreateCustomerRequest
		err  error
	}{
		{"missing first name", CreateCustomerRequest{Phone: "0700000000"}, ErrEmptyFirstName},
		{"blank phone", CreateCustomerRequest{FirstName: "Awa", Phone: "  "}, ErrEmptyPhone},
		{"long first name", CreateCustomerRequest{FirstName: strings.Repeat("é", 101), Phone: "0700000000"}, ErrInvalidFields},
		{"long phone", CreateCustomerRequest{FirstName: "Awa", Phone: strings.Repeat("1", 51)}, ErrInvalidFields},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NormalizeCreateRequest(tc.req)
			require.ErrorIs(t, err, tc.err)
		})
	}
	valid.FirstName = strings.Repeat("é", 100)
	_, err = NormalizeCreateRequest(valid)
	require.NoError(t, err, "limite PostgreSQL en caractères, pas en octets")
}

type failingStore struct{ Store }

func (f failingStore) CreateCustomer(context.Context, string, CreateCustomerRequest) (*Customer, error) {
	return nil, errors.New("internal SQL detail")
}

func TestCreateCustomer_NeDivulguePasErreurTechnique(t *testing.T) {
	svc := NewService()
	svc.SetStore(failingStore{})
	h := NewHandler(svc)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/customers", strings.NewReader(`{"first_name":"Awa","phone":"0700000000"}`))
	req = req.WithContext(auth.WithTenantID(req.Context(), "tenant"))
	rec := httptest.NewRecorder()
	h.CreateCustomer(rec, req)
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.NotContains(t, rec.Body.String(), "internal SQL detail")
}
