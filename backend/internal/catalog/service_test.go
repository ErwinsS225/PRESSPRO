package catalog

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
	assert.NotNil(t, service.services)
	assert.NotNil(t, service.logger)
	assert.Equal(t, 1, service.sequence)
}

func TestGetServices_DefaultTenant(t *testing.T) {
	t.Parallel()
	service := NewService()
	ctx := context.Background()
	defaultTenant := "a0000000-0000-0000-0000-000000000001"

	services, err := service.GetServices(ctx, defaultTenant, "", "")
	require.NoError(t, err)
	assert.Greater(t, len(services), 0) // Le tenant par défaut a des services
}

func TestGetServiceByID_NotFound(t *testing.T) {
	t.Parallel()
	service := NewService()
	ctx := context.Background()
	tenantID := "test-tenant-1"

	serviceItem, err := service.GetServiceByID(ctx, tenantID, "srv-inexistant")
	assert.ErrorIs(t, err, ErrServiceNotFound)
	assert.Nil(t, serviceItem)
}

func TestIsolationMultitenant(t *testing.T) {
	t.Parallel()
	service := NewService()
	ctx := context.Background()
	defaultTenant := "a0000000-0000-0000-0000-000000000001"

	// Récupère un service du tenant par défaut
	servicesDefault, err := service.GetServices(ctx, defaultTenant, "", "")
	require.NoError(t, err)
	if len(servicesDefault) > 0 {
		// Tentative d'accès depuis un autre tenant
		tenant2 := "test-tenant-2"
		accessed, err := service.GetServiceByID(ctx, tenant2, servicesDefault[0].ID)
		assert.ErrorIs(t, err, ErrServiceNotFound)
		assert.Nil(t, accessed)
	}
}

func TestGetServices_CategoryFilter(t *testing.T) {
	t.Parallel()
	service := NewService()
	ctx := context.Background()
	defaultTenant := "a0000000-0000-0000-0000-000000000001"

	// Filtre par catégorie quotidienne
	services, err := service.GetServices(ctx, defaultTenant, CategoryDaily, "")
	require.NoError(t, err)
	for _, srv := range services {
		assert.Equal(t, CategoryDaily, srv.Category)
	}
}

func TestGetServices_SearchFilter(t *testing.T) {
	t.Parallel()
	service := NewService()
	ctx := context.Background()
	defaultTenant := "a0000000-0000-0000-0000-000000000001"

	// Recherche par mot-clé qui existe dans au moins un service
	services, err := service.GetServices(ctx, defaultTenant, "", "Lavage")
	require.NoError(t, err)
	// Vérifie qu'au moins un service est retourné (la recherche fonctionne)
	assert.Greater(t, len(services), 0)
}