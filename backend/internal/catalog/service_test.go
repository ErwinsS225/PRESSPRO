package catalog

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pressing-os/backend/internal/database"
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

// stubStore est une doublure de persistance : elle permet de tester le chemin
// « store branché » sans PostgreSQL.
type stubStore struct{ called bool }

func (s *stubStore) GetServices(ctx context.Context, tenantID, category, search string) ([]*ServiceItem, error) {
	s.called = true
	return []*ServiceItem{{ID: "stub-1", TenantID: tenantID, Name: "Stub"}}, nil
}
func (s *stubStore) GetService(ctx context.Context, tenantID, id string) (*ServiceItem, error) {
	s.called = true
	return &ServiceItem{ID: id, TenantID: tenantID, Name: "Stub"}, nil
}
func (s *stubStore) CreateService(ctx context.Context, tenantID string, req CreateServiceRequest) (*ServiceItem, error) {
	s.called = true
	return &ServiceItem{ID: "stub-new", TenantID: tenantID, Name: req.Name}, nil
}
func (s *stubStore) UpdateService(ctx context.Context, tenantID, id string, req UpdateServiceRequest) (*ServiceItem, error) {
	s.called = true
	return &ServiceItem{ID: id, TenantID: tenantID}, nil
}
func (s *stubStore) DeleteService(ctx context.Context, tenantID, id string) error {
	s.called = true
	return nil
}
func (s *stubStore) SeedIfEmpty(ctx context.Context, tenantID string, items []ServiceItem) (int, error) {
	s.called = true
	return len(items), nil
}

// TestNewStore_RuntimeInitialization garantit qu'un service adossé à PostgreSQL
// est utilisable immédiatement (logger et index initialisés). C'est le test de
// non-régression du bug bloquant « GET /api/v1/services -> 500 » : la lecture du
// catalogue paniquait car NewStore construisait un Service partiel (logger nil).
func TestNewStore_RuntimeInitialization(t *testing.T) {
	t.Parallel()
	service := NewStore(database.NewStore(nil))
	require.NotNil(t, service)
	assert.NotNil(t, service.logger, "le logger doit être initialisé")
	assert.NotNil(t, service.services, "l'index mémoire doit être initialisé")
	assert.NotNil(t, service.store, "la persistance PostgreSQL doit être branchée")

	// Les méthodes de lecture journalisent AVANT de déléguer : c'est précisément
	// cette première ligne qui provoquait la panique avec un logger nil.
	stub := &stubStore{}
	service.store = stub
	assert.NotPanics(t, func() {
		_, _ = service.GetServices(context.Background(), "a0000000-0000-0000-0000-000000000001", "", "")
	})
	assert.NotPanics(t, func() {
		_, _ = service.GetServiceByID(context.Background(), "a0000000-0000-0000-0000-000000000001", "srv-101")
	})
	assert.True(t, stub.called, "la lecture doit être déléguée à la persistance")
}

// TestSeedDefaultServices_NewTenant vérifie qu'un nouveau tenant obtient sa
// propre copie de la grille tarifaire, avec des identifiants qui lui sont
// propres (plus de stub vide : les prestations par défaut sont réellement créées).
func TestSeedDefaultServices_NewTenant(t *testing.T) {
	t.Parallel()
	service := NewService()
	ctx := context.Background()
	newTenant := "b0000000-0000-0000-0000-000000000002"

	items, err := service.GetServices(ctx, newTenant, "", "")
	require.NoError(t, err)
	require.NotEmpty(t, items, "un nouveau tenant doit recevoir les prestations par défaut")

	for _, it := range items {
		assert.Equal(t, newTenant, it.TenantID, "isolation stricte par tenant")
		assert.NotEqual(t, "srv-101", it.ID, "les identifiants doivent être préfixés par tenant")
	}

	// Les prestations du tenant par défaut ne doivent pas être accessibles au nouveau tenant.
	_, err = service.GetServiceByID(ctx, newTenant, "srv-101")
	assert.ErrorIs(t, err, ErrServiceNotFound)

	// Un second accès ne doit pas dupliquer la grille.
	again, err := service.GetServices(ctx, newTenant, "", "")
	require.NoError(t, err)
	assert.Equal(t, len(items), len(again), "le seed ne doit pas se répéter")
}
