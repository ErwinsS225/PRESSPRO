package pricelist

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Service expose les opérations sur les grilles tarifaires versionnées.
// Repli mémoire (mode dégradé) quand PostgreSQL est indisponible.
type Service struct {
	store Store
	mu    sync.RWMutex
	lists map[string][]*PriceList
	vers  map[string][]*PriceListVersion
}

// NewService crée un service de grilles en mémoire (repli sans PostgreSQL).
func NewService() *Service {
	return &Service{
		lists: make(map[string][]*PriceList),
		vers:  make(map[string][]*PriceListVersion),
	}
}

// GetPriceLists liste les grilles du tenant (isolation par tenant_id).
func (s *Service) GetPriceLists(ctx context.Context, tenantID string) ([]*PriceList, error) {
	if s.store != nil {
		return s.store.ListPriceLists(ctx, tenantID)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if lists, ok := s.lists[tenantID]; ok && len(lists) > 0 {
		return lists, nil
	}
	return []*PriceList{{
		ID: "pl-demo-" + tenantID, TenantID: tenantID, Name: "Grille principale",
		Description: "Grille tarifaire initiale (mode mémoire)", IsDefault: true,
		Status: "active", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}}, nil
}

// GetPriceListWithVersions retourne une grille et son historique de versions.
// L'isolation est stricte : id ET tenant_id sont vérifiés en base (contrôle #2).
func (s *Service) GetPriceListWithVersions(ctx context.Context, tenantID, id string) (*PriceListWithVersions, error) {
	if s.store != nil {
		pl, err := s.store.GetPriceList(ctx, tenantID, id)
		if err != nil {
			return nil, err
		}
		versions, err := s.store.ListVersions(ctx, tenantID, id)
		if err != nil {
			return nil, err
		}
		out := make([]PriceListVersion, 0, len(versions))
		for _, v := range versions {
			out = append(out, *v)
		}
		return &PriceListWithVersions{PriceList: *pl, Versions: out}, nil
	}
	lists, _ := s.GetPriceLists(ctx, tenantID)
	for _, pl := range lists {
		if pl.ID == id {
			versions := make([]PriceListVersion, 0, len(s.vers[id]))
			for _, vv := range s.vers[id] {
				versions = append(versions, *vv)
			}
			return &PriceListWithVersions{PriceList: *pl, Versions: versions}, nil
		}
	}
	return nil, ErrNotFound
}

// CreatePriceList crée une grille tarifaire pour le tenant.
func (s *Service) CreatePriceList(ctx context.Context, tenantID string, req CreatePriceListRequest) (*PriceList, error) {
	if strings.TrimSpace(req.Name) == "" {
		return nil, fmt.Errorf("le nom de la grille est obligatoire")
	}
	if s.store != nil {
		return s.store.CreatePriceList(ctx, tenantID, req)
	}
	now := time.Now()
	name := strings.TrimSpace(req.Name)
	s.mu.RLock()
	for _, existing := range s.lists[tenantID] {
		if strings.EqualFold(existing.Name, name) {
			s.mu.RUnlock()
			return nil, ErrNameExists
		}
	}
	s.mu.RUnlock()
	pl := &PriceList{
		ID: fmt.Sprintf("pl-%d", now.UnixNano()), TenantID: tenantID, BranchID: req.BranchID,
		Name: name, Description: req.Description, IsDefault: req.IsDefault,
		Status: "active", CreatedAt: now, UpdatedAt: now,
	}
	s.mu.Lock()
	s.lists[tenantID] = append(s.lists[tenantID], pl)
	s.mu.Unlock()
	return pl, nil
}

// CreateVersion fige une nouvelle version : snapshot du catalogue courant
// (éventuellement écrasé par les prix fournis) et supersession atomique de la
// version active. Les anciennes versions ne sont jamais modifiées ni supprimées.
func (s *Service) CreateVersion(ctx context.Context, tenantID, id string, req CreateVersionRequest) (*PriceListVersion, error) {
	// Appartenance de la grille au tenant avant toute écriture.
	if _, err := s.GetPriceListWithVersions(ctx, tenantID, id); err != nil {
		return nil, err
	}

	if s.store != nil {
		snap, err := s.store.CatalogSnapshot(ctx, tenantID)
		if err != nil {
			return nil, err
		}
		if len(req.Prices) > 0 {
			overrides := make(map[string]float64, len(req.Prices))
			for _, p := range req.Prices {
				if p.ServiceID != "" && p.UnitPrice >= 0 {
					overrides[p.ServiceID] = p.UnitPrice
				}
			}
			for i := range snap {
				if v, ok := overrides[snap[i].ServiceID]; ok {
					snap[i].UnitPrice = v
				}
			}
		}
		if snap == nil {
			snap = []PriceListEntry{}
		}
		// InsertVersion supersede la version active + insère dans une transaction.
		return s.store.InsertVersion(ctx, tenantID, id, snap, req.Note)
	}

	// Repli mémoire : version incrémentale non persistée.
	s.mu.Lock()
	defer s.mu.Unlock()
	next := len(s.vers[id]) + 1
	for i := range s.vers[id] {
		if s.vers[id][i].Status == "active" {
			s.vers[id][i].Status = "superseded"
		}
	}
	v := &PriceListVersion{
		ID: fmt.Sprintf("plv-%d", time.Now().UnixNano()), PriceListID: id, TenantID: tenantID,
		Version: next, Status: "active", Snapshot: []PriceListEntry{}, Note: req.Note,
		CreatedAt: time.Now(),
	}
	s.vers[id] = append(s.vers[id], v)
	return v, nil
}

// CompterGrilles retourne le nombre de grilles du pressing (mode mémoire). Alimente
// le plafond « grilles tarifaires » en mode dégradé, comme CountPriceLists en SQL.
func (s *Service) CompterGrilles(_ context.Context, tenantID string) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return int64(len(s.lists[tenantID])), nil
}
