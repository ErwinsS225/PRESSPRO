package catalog

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Service gère le catalogue et la grille tarifaire en FCFA
type Service struct {
	mu       sync.RWMutex
	services map[string]*ServiceItem // clé: service_id
}

// NewService initialise le catalogue avec les tarifs de base du pressing
func NewService() *Service {
	s := &Service{
		services: make(map[string]*ServiceItem),
	}

	defaultTenantID := "a0000000-0000-0000-0000-000000000001"
	seedItems := []ServiceItem{
		{
			ID:             "srv-1",
			TenantID:       defaultTenantID,
			Name:           "Chemise classique (Lavage & Repassage)",
			Category:       CategoryWashAndIron,
			UnitPrice:      1000,
			Unit:           "pièce",
			EstimatedHours: 24,
			IsActive:       true,
			CreatedAt:      time.Now().Add(-48 * time.Hour),
			UpdatedAt:      time.Now().Add(-48 * time.Hour),
		},
		{
			ID:             "srv-2",
			TenantID:       defaultTenantID,
			Name:           "Pantalon classique / jean",
			Category:       CategoryWashAndIron,
			UnitPrice:      1000,
			Unit:           "pièce",
			EstimatedHours: 24,
			IsActive:       true,
			CreatedAt:      time.Now().Add(-48 * time.Hour),
			UpdatedAt:      time.Now().Add(-48 * time.Hour),
		},
		{
			ID:             "srv-3",
			TenantID:       defaultTenantID,
			Name:           "T-shirt / Polo en coton",
			Category:       CategoryWashAndIron,
			UnitPrice:      700,
			Unit:           "pièce",
			EstimatedHours: 24,
			IsActive:       true,
			CreatedAt:      time.Now().Add(-48 * time.Hour),
			UpdatedAt:      time.Now().Add(-48 * time.Hour),
		},
		{
			ID:             "srv-4",
			TenantID:       defaultTenantID,
			Name:           "Costume 2 pièces (Veste + Pantalon)",
			Category:       CategoryDryClean,
			UnitPrice:      3500,
			Unit:           "ensemble",
			EstimatedHours: 48,
			IsActive:       true,
			CreatedAt:      time.Now().Add(-48 * time.Hour),
			UpdatedAt:      time.Now().Add(-48 * time.Hour),
		},
		{
			ID:             "srv-5",
			TenantID:       defaultTenantID,
			Name:           "Veste blazer simple / Costume",
			Category:       CategoryDryClean,
			UnitPrice:      2000,
			Unit:           "pièce",
			EstimatedHours: 24,
			IsActive:       true,
			CreatedAt:      time.Now().Add(-48 * time.Hour),
			UpdatedAt:      time.Now().Add(-48 * time.Hour),
		},
		{
			ID:             "srv-6",
			TenantID:       defaultTenantID,
			Name:           "Pantalon de costume ville (Pressing)",
			Category:       CategoryDryClean,
			UnitPrice:      1500,
			Unit:           "pièce",
			EstimatedHours: 24,
			IsActive:       true,
			CreatedAt:      time.Now().Add(-48 * time.Hour),
			UpdatedAt:      time.Now().Add(-48 * time.Hour),
		},
		{
			ID:             "srv-7",
			TenantID:       defaultTenantID,
			Name:           "Chemise sur cintre (Repassage seul)",
			Category:       CategoryIronOnly,
			UnitPrice:      500,
			Unit:           "pièce",
			EstimatedHours: 12,
			IsActive:       true,
			CreatedAt:      time.Now().Add(-48 * time.Hour),
			UpdatedAt:      time.Now().Add(-48 * time.Hour),
		},
		{
			ID:             "srv-8",
			TenantID:       defaultTenantID,
			Name:           "Pantalon à plis (Repassage seul)",
			Category:       CategoryIronOnly,
			UnitPrice:      600,
			Unit:           "pièce",
			EstimatedHours: 12,
			IsActive:       true,
			CreatedAt:      time.Now().Add(-48 * time.Hour),
			UpdatedAt:      time.Now().Add(-48 * time.Hour),
		},
		{
			ID:             "srv-9",
			TenantID:       defaultTenantID,
			Name:           "Robe de soirée / Cérémonie",
			Category:       CategoryDelicate,
			UnitPrice:      4000,
			Unit:           "pièce",
			EstimatedHours: 48,
			IsActive:       true,
			CreatedAt:      time.Now().Add(-48 * time.Hour),
			UpdatedAt:      time.Now().Add(-48 * time.Hour),
		},
		{
			ID:             "srv-10",
			TenantID:       defaultTenantID,
			Name:           "Pagne traditionnel 3 morceaux",
			Category:       CategoryDelicate,
			UnitPrice:      3000,
			Unit:           "ensemble",
			EstimatedHours: 48,
			IsActive:       true,
			CreatedAt:      time.Now().Add(-48 * time.Hour),
			UpdatedAt:      time.Now().Add(-48 * time.Hour),
		},
		{
			ID:             "srv-11",
			TenantID:       defaultTenantID,
			Name:           "Bazin riche brodé / Grand Boubou",
			Category:       CategoryDelicate,
			UnitPrice:      4500,
			Unit:           "ensemble",
			EstimatedHours: 48,
			IsActive:       true,
			CreatedAt:      time.Now().Add(-48 * time.Hour),
			UpdatedAt:      time.Now().Add(-48 * time.Hour),
		},
		{
			ID:             "srv-12",
			TenantID:       defaultTenantID,
			Name:           "Drap 2 places / Grande Couette",
			Category:       CategoryHomeLinen,
			UnitPrice:      2500,
			Unit:           "pièce",
			EstimatedHours: 48,
			IsActive:       true,
			CreatedAt:      time.Now().Add(-48 * time.Hour),
			UpdatedAt:      time.Now().Add(-48 * time.Hour),
		},
		{
			ID:             "srv-13",
			TenantID:       defaultTenantID,
			Name:           "Rideaux occultants / Voilages (la paire)",
			Category:       CategoryHomeLinen,
			UnitPrice:      3500,
			Unit:           "pièce",
			EstimatedHours: 48,
			IsActive:       true,
			CreatedAt:      time.Now().Add(-48 * time.Hour),
			UpdatedAt:      time.Now().Add(-48 * time.Hour),
		},
	}

	for _, it := range seedItems {
		itemCopy := it
		s.services[it.ID] = &itemCopy
	}

	return s
}

// CreateService enregistre une nouvelle prestation dans la grille
func (s *Service) CreateService(ctx context.Context, tenantID string, req CreateServiceRequest) (*ServiceItem, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrEmptyName
	}
	if req.UnitPrice < 0 {
		return nil, ErrInvalidPrice
	}

	unit := req.Unit
	if unit == "" {
		unit = "pièce"
	}
	hours := req.EstimatedHours
	if hours <= 0 {
		hours = 24
	}
	cat := req.Category
	if cat == "" {
		cat = CategoryWashAndIron
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id := fmt.Sprintf("srv-%d", time.Now().UnixNano())
	now := time.Now().UTC()

	item := &ServiceItem{
		ID:             id,
		TenantID:       tenantID,
		Name:           name,
		Category:       cat,
		UnitPrice:      req.UnitPrice,
		Unit:           unit,
		EstimatedHours: hours,
		IsActive:       isActive,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	s.services[id] = item
	return item, nil
}

// GetServices liste les prestations du catalogue filtrées par tenant_id
func (s *Service) GetServices(ctx context.Context, tenantID string, categoryFilter, search string) ([]*ServiceItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ServiceItem
	searchLower := strings.ToLower(strings.TrimSpace(search))

	for _, item := range s.services {
		if item.TenantID != tenantID {
			continue
		}
		if categoryFilter != "" && categoryFilter != "all" && item.Category != categoryFilter {
			continue
		}
		if searchLower != "" && !strings.Contains(strings.ToLower(item.Name), searchLower) {
			continue
		}
		result = append(result, item)
	}

	return result, nil
}

// GetServiceByID retourne une prestation par son ID avec isolation multi-tenant
func (s *Service) GetServiceByID(ctx context.Context, tenantID, id string) (*ServiceItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, exists := s.services[id]
	if !exists || item.TenantID != tenantID {
		return nil, ErrServiceNotFound
	}

	return item, nil
}

// UpdateService met à jour les informations ou le tarif d'une prestation
func (s *Service) UpdateService(ctx context.Context, tenantID, id string, req UpdateServiceRequest) (*ServiceItem, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrEmptyName
	}
	if req.UnitPrice < 0 {
		return nil, ErrInvalidPrice
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	item, exists := s.services[id]
	if !exists || item.TenantID != tenantID {
		return nil, ErrServiceNotFound
	}

	item.Name = name
	if req.Category != "" {
		item.Category = req.Category
	}
	item.UnitPrice = req.UnitPrice
	if req.Unit != "" {
		item.Unit = req.Unit
	}
	if req.EstimatedHours > 0 {
		item.EstimatedHours = req.EstimatedHours
	}
	if req.IsActive != nil {
		item.IsActive = *req.IsActive
	}
	item.UpdatedAt = time.Now().UTC()

	return item, nil
}

// DeleteService supprime une prestation de la grille tarifaire
func (s *Service) DeleteService(ctx context.Context, tenantID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, exists := s.services[id]
	if !exists || item.TenantID != tenantID {
		return ErrServiceNotFound
	}

	delete(s.services, id)
	return nil
}
