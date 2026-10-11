//go:build integration

package catalog

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pressing-os/backend/internal/database"
)

// Test d'intégration PostgreSQL du catalogue.
//
// Exécution (PostgreSQL 16 joignable et migrations appliquées) :
//
//	TEST_DATABASE_URL="postgres://pressing_app:...@localhost:5432/pressing_os?sslmode=disable" \
//	  go test -tags=integration ./internal/catalog/...
//
// Il couvre la régression P0 « GET /api/v1/services -> 500 » : la lecture du
// catalogue adossé à PostgreSQL doit fonctionner, respecter l'isolation
// multi-tenant et ne jamais paniquer (logger/index initialisés par NewStore).
func TestIntegration_DBReadServices(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL non défini : test d'intégration ignoré")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool, err := database.ConnectPool(ctx, dsn)
	require.NoError(t, err, "connexion PostgreSQL")
	defer pool.Close()
	require.NoError(t, pool.Ping(ctx))

	db := database.NewStore(pool.Pool)
	service := NewStore(db)
	require.NotNil(t, service.logger, "le logger doit être initialisé (régression P0)")

	// Jeu de données éphémère : un tenant dédié + une prestation.
	const tenantID = "c0000000-0000-0000-0000-0000000000ff"
	var tenantName = "IT Catalogue " + time.Now().UTC().Format("20060102T150405")

	if _, err := db.ExecContext(ctx, `DELETE FROM services WHERE tenant_id = $1`, tenantID); err != nil {
		t.Fatalf("nettoyage préalable: %v", err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM tenants WHERE id = $1`, tenantID); err != nil {
		t.Fatalf("nettoyage préalable tenant: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO tenants (id, name, slug, city, status)
		VALUES ($1, $2, $3, 'Abidjan', 'active')`,
		tenantID, tenantName, "it-catalogue-"+time.Now().UTC().Format("150405"),
	); err != nil {
		t.Fatalf("insertion du tenant de test: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM services WHERE tenant_id = $1`, tenantID)
		_, _ = db.ExecContext(ctx, `DELETE FROM tenants WHERE id = $1`, tenantID)
	})

	// 1) Création en base
	created, err := service.CreateService(ctx, tenantID, CreateServiceRequest{
		Name:      "Prestation IT — Chemise",
		Category:  CategoryDaily,
		UnitPrice: 750,
		Unit:      "pièce",
	})
	require.NoError(t, err, "création d'une prestation en base")
	require.NotEmpty(t, created.ID)

	// 2) Lecture de liste — c'est cet appel qui renvoyait un 500 avant correctif.
	items, err := service.GetServices(ctx, tenantID, "", "Chemise")
	require.NoError(t, err, "lecture du catalogue PostgreSQL")
	assert.Len(t, items, 1)
	assert.Equal(t, "Prestation IT — Chemise", items[0].Name)
	assert.Equal(t, 750.0, items[0].UnitPrice)
	assert.Equal(t, tenantID, items[0].TenantID)

	// 3) Lecture par identifiant
	one, err := service.GetServiceByID(ctx, tenantID, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, one.ID)

	// 4) Isolation multi-tenant : un autre tenant ne voit rien.
	const otherTenant = "c0000000-0000-0000-0000-0000000000fe"
	others, err := service.GetServices(ctx, otherTenant, "", "")
	require.NoError(t, err)
	assert.Empty(t, others)

	_, err = service.GetServiceByID(ctx, otherTenant, created.ID)
	assert.ErrorIs(t, err, ErrServiceNotFound)

	// 5) Filtre par catégorie
	filtered, err := service.GetServices(ctx, tenantID, CategoryDaily, "")
	require.NoError(t, err)
	assert.Len(t, filtered, 1)

	// 6) Mise à jour puis suppression
	updated, err := service.UpdateService(ctx, tenantID, created.ID, UpdateServiceRequest{
		Name: "Prestation IT — Chemise", Category: CategoryDaily, UnitPrice: 900, Unit: "pièce",
	})
	require.NoError(t, err)
	assert.Equal(t, 900.0, updated.UnitPrice)

	require.NoError(t, service.DeleteService(ctx, tenantID, created.ID))
	_, err = service.GetServiceByID(ctx, tenantID, created.ID)
	assert.ErrorIs(t, err, ErrServiceNotFound)
}
