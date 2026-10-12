//go:build integration

package order_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"pressing-os/backend/internal/customer"
	"pressing-os/backend/internal/database"
	"pressing-os/backend/internal/order"
)

func clientDepot(t *testing.T, ctx context.Context, inst *instanceCommandes, tenant, first, phone string) *customer.Customer {
	t.Helper()
	svc := customer.NewService()
	svc.SetStore(customer.NewStore(database.NewStore(inst.db.Pool)))
	c, err := svc.CreateCustomer(ctx, tenant, customer.CreateCustomerRequest{
		FirstName: first, LastName: "Kouadio", Phone: phone,
	})
	require.NoError(t, err)
	return c
}

func TestDepotClient_SelectionSansEcrasement(t *testing.T) {
	ctx, inst := ouvrirCommandes(t)
	tenant := creerEtablissementCommandes(t, ctx, inst, "FREE")
	c := clientDepot(t, ctx, inst, tenant, "Awa", "+2250700000001")
	req := requeteCommande(1)
	req.CustomerID = c.ID
	req.CustomerName = "Nom à ne pas appliquer"
	req.CustomerPhone = "+2250700009999"
	ord, err := inst.orders.CreateOrder(ctx, tenant, req)
	require.NoError(t, err)
	require.Equal(t, c.ID, ord.CustomerID)
	require.Equal(t, c.FullName, ord.CustomerName)
	require.Equal(t, c.Phone, ord.CustomerPhone)
	var count int
	require.NoError(t, inst.db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM customers WHERE tenant_id=$1`, tenant).Scan(&count))
	require.Equal(t, 1, count)
}

func TestDepotClient_RefuseIDAutreTenant(t *testing.T) {
	ctx, inst := ouvrirCommandes(t)
	a := creerEtablissementCommandes(t, ctx, inst, "FREE")
	b := creerEtablissementCommandes(t, ctx, inst, "FREE")
	c := clientDepot(t, ctx, inst, a, "Awa", "+2250700000001")
	req := requeteCommande(1)
	req.CustomerID = c.ID
	_, err := inst.orders.CreateOrder(ctx, b, req)
	require.ErrorIs(t, err, customer.ErrCustomerNotFound)
	require.Zero(t, nombreCommandes(t, ctx, inst, b))
}

func TestDepotClient_TelephoneObligatoire(t *testing.T) {
	ctx, inst := ouvrirCommandes(t)
	tenant := creerEtablissementCommandes(t, ctx, inst, "FREE")
	req := requeteCommande(1)
	req.CustomerPhone = "   "
	_, err := inst.orders.CreateOrder(ctx, tenant, req)
	require.ErrorIs(t, err, customer.ErrEmptyPhone)
	require.Zero(t, nombreCommandes(t, ctx, inst, tenant))
}

func TestDepotClient_AncienContratNeFusionnePas(t *testing.T) {
	ctx, inst := ouvrirCommandes(t)
	tenant := creerEtablissementCommandes(t, ctx, inst, "FREE")
	c := clientDepot(t, ctx, inst, tenant, "Awa", "+2250700000001")
	req := requeteCommande(1)
	req.CustomerName = "Autre Personne"
	req.CustomerPhone = c.Phone
	_, err := inst.orders.CreateOrder(ctx, tenant, req)
	require.ErrorIs(t, err, order.ErrCustomerSelectionRequired)
	var first string
	require.NoError(t, inst.db.Pool.QueryRow(ctx, `SELECT first_name FROM customers WHERE id=$1`, c.ID).Scan(&first))
	require.Equal(t, "Awa", first)
	require.Zero(t, nombreCommandes(t, ctx, inst, tenant))
}

func TestDepotClient_CRUDNumeroPartage(t *testing.T) {
	ctx, inst := ouvrirCommandes(t)
	tenant := creerEtablissementCommandes(t, ctx, inst, "FREE")
	a := clientDepot(t, ctx, inst, tenant, "Awa", "+2250700000001")
	b := clientDepot(t, ctx, inst, tenant, "Yao", a.Phone)
	require.NotEqual(t, a.ID, b.ID)
}

func TestDepotClient_IDSeulEtIdentifiantsInvalides(t *testing.T) {
	ctx, inst := ouvrirCommandes(t)
	tenant := creerEtablissementCommandes(t, ctx, inst, "FREE")
	c := clientDepot(t, ctx, inst, tenant, "Awa", "+2250700000001")
	req := requeteCommande(1)
	req.CustomerName, req.CustomerPhone = "", ""
	req.CustomerID = c.ID
	ord, err := inst.orders.CreateOrder(ctx, tenant, req)
	require.NoError(t, err)
	require.Equal(t, c.ID, ord.CustomerID)
	for _, id := range []string{"invalide", "ffffffff-ffff-ffff-ffff-ffffffffffff"} {
		req.CustomerID = id
		_, err = inst.orders.CreateOrder(ctx, tenant, req)
		require.ErrorIs(t, err, customer.ErrCustomerNotFound)
	}
	require.EqualValues(t, 1, nombreCommandes(t, ctx, inst, tenant))
}

func TestDepotClient_NouveauStructurePartageEtRecherche(t *testing.T) {
	ctx, inst := ouvrirCommandes(t)
	tenant := creerEtablissementCommandes(t, ctx, inst, "FREE")
	c := clientDepot(t, ctx, inst, tenant, "Awa", "+2250700000001")
	req := requeteCommande(1)
	req.CustomerName, req.CustomerPhone = "", ""
	req.NewCustomer = &customer.CreateCustomerRequest{
		FirstName: " Jean Pierre ", LastName: " Kouadio ", Phone: c.Phone, Notes: "Numéro familial",
	}
	ord, err := inst.orders.CreateOrder(ctx, tenant, req)
	require.NoError(t, err)
	require.NotEqual(t, c.ID, ord.CustomerID)
	require.Equal(t, "Jean Pierre Kouadio", ord.CustomerName)
	svc := customer.NewService()
	svc.SetStore(customer.NewStore(database.NewStore(inst.db.Pool)))
	created, err := svc.GetCustomer(ctx, tenant, ord.CustomerID)
	require.NoError(t, err)
	require.Equal(t, "Jean Pierre", created.FirstName)
	require.Equal(t, "Kouadio", created.LastName)
	require.Equal(t, "Numéro familial", created.Notes)
	matches, err := svc.GetCustomers(ctx, tenant, c.Phone)
	require.NoError(t, err)
	require.Len(t, matches, 2)
	other := creerEtablissementCommandes(t, ctx, inst, "FREE")
	matches, err = svc.GetCustomers(ctx, other, c.Phone)
	require.NoError(t, err)
	require.Empty(t, matches)
}

func TestDepotClient_RollbackClientEtNumero(t *testing.T) {
	ctx, inst := ouvrirCommandes(t)
	tenant := creerEtablissementCommandes(t, ctx, inst, "FREE")
	req := requeteCommande(1)
	req.CustomerName, req.CustomerPhone = "", ""
	req.NewCustomer = &customer.CreateCustomerRequest{FirstName: "Awa", LastName: "Kouadio", Phone: "0700000001"}
	// Échec APRÈS insertion du client : contrainte SQL du paiement (lot financier séparé).
	req.InitialPaid, req.PaymentMethod = 1000, "mode-inconnu"
	_, err := inst.orders.CreateOrder(ctx, tenant, req)
	require.Error(t, err)
	require.Zero(t, nombreCommandes(t, ctx, inst, tenant))
	for _, table := range []string{"customers", "payments", "order_sequences"} {
		var count int
		require.NoError(t, inst.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM "+table+" WHERE tenant_id=$1", tenant).Scan(&count))
		require.Zero(t, count, table)
	}
	req.PaymentMethod = "cash"
	_, err = inst.orders.CreateOrder(ctx, tenant, req)
	require.NoError(t, err)
	var seq int
	require.NoError(t, inst.db.Pool.QueryRow(ctx, `SELECT last_seq FROM order_sequences WHERE tenant_id=$1`, tenant).Scan(&seq))
	require.Equal(t, 1, seq)
}

func TestDepotClient_ContrainteSQLIntertenant(t *testing.T) {
	ctx, inst := ouvrirCommandes(t)
	a := creerEtablissementCommandes(t, ctx, inst, "FREE")
	b := creerEtablissementCommandes(t, ctx, inst, "FREE")
	c := clientDepot(t, ctx, inst, a, "Awa", "0700000001")
	_, err := inst.db.Pool.Exec(ctx, `INSERT INTO orders (tenant_id,customer_id,order_number)
		VALUES ($1,$2,'INTERTENANT-INTERDIT')`, b, c.ID)
	require.ErrorContains(t, err, "orders_tenant_customer_fkey")
	require.Zero(t, nombreCommandes(t, ctx, inst, b))
}
