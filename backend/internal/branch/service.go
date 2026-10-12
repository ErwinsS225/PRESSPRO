package branch

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"pressing-os/backend/internal/database"
)

// Store est le contrat de persistance des établissements.
type Store interface {
	GetBranches(ctx context.Context, tenantID string) ([]*Branch, error)
	GetBranch(ctx context.Context, tenantID, id string) (*Branch, error)
	CreateBranch(ctx context.Context, tenantID string, req CreateBranchRequest) (*Branch, error)
	UpdateBranch(ctx context.Context, tenantID, id string, req UpdateBranchRequest) (*Branch, error)

	// Affectation des comptes aux établissements (branch_memberships).
	GetMemberBranches(ctx context.Context, tenantID, userID string) ([]*MemberBranch, error)
	SetMemberBranches(ctx context.Context, tenantID, userID string, branchIDs []string) ([]*MemberBranch, error)
}

// Service expose la logique métier multi-sites. Sans base de données, il
// fonctionne en mémoire (mode démonstration) avec un établissement principal
// créé à la volée pour chaque pressing.
type Service struct {
	store Store

	// verifier contrôle l'appartenance d'un compte au pressing (mode mémoire).
	// Sans vérificateur, le mode mémoire refuse les opérations sur les comptes.
	verifier VerificateurMembre

	mu           sync.RWMutex
	memoire      map[string][]*Branch
	affectations map[string]map[string]map[string]bool // tenant -> compte -> établissement
	sequence     int
}

// NewService crée le service en mode mémoire.
func NewService() *Service {
	return &Service{memoire: make(map[string][]*Branch)}
}

// NewStore construit un service adossé à PostgreSQL.
func NewStore(db *database.Store) *Service {
	return &Service{store: &dbStore{database: db}}
}

// SetStore branche la persistance sur un service existant.
func (s *Service) SetStore(store Store) {
	s.store = store
}

// normalize nettoie un libellé : espaces de bord supprimés et espaces
// internes compactés (« Pressing  Yopougon » et « Pressing Yopougon » désignent
// le même site, en base comme en mémoire).
func normalize(v string) string {
	return strings.Join(strings.Fields(v), " ")
}

// GetBranches retourne les établissements du pressing, principal en tête.
func (s *Service) GetBranches(ctx context.Context, tenantID string) ([]*Branch, error) {
	if s.store != nil {
		return s.store.GetBranches(ctx, tenantID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensurePrimaryLocked(tenantID)
	return append([]*Branch{}, s.memoire[tenantID]...), nil
}

// GetBranch retourne un établissement par identifiant.
func (s *Service) GetBranch(ctx context.Context, tenantID, id string) (*Branch, error) {
	if s.store != nil {
		return s.store.GetBranch(ctx, tenantID, id)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensurePrimaryLocked(tenantID)
	for _, b := range s.memoire[tenantID] {
		if b.ID == id {
			return b, nil
		}
	}
	return nil, ErrBranchNotFound
}

// CreateBranch valide puis crée un établissement secondaire.
func (s *Service) CreateBranch(ctx context.Context, tenantID string, req CreateBranchRequest) (*Branch, error) {
	req.Name = normalize(req.Name)
	req.Code = normalize(req.Code)
	req.City = normalize(req.City)
	req.Phone = normalize(req.Phone)
	req.Address = normalize(req.Address)
	if req.Name == "" {
		return nil, ErrEmptyName
	}
	if s.store != nil {
		return s.store.CreateBranch(ctx, tenantID, req)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensurePrimaryLocked(tenantID)
	for _, b := range s.memoire[tenantID] {
		if strings.EqualFold(b.Name, req.Name) {
			return nil, ErrNameExists
		}
	}
	s.sequence++
	now := time.Now().UTC()
	b := &Branch{
		ID:       fmt.Sprintf("branch-memoire-%d", s.sequence),
		TenantID: tenantID,
		Name:     req.Name, Code: req.Code, Address: req.Address,
		Phone: req.Phone, City: req.City, Timezone: "Africa/Abidjan",
		Status: StatusActive, CreatedAt: now, UpdatedAt: now,
	}
	s.memoire[tenantID] = append(s.memoire[tenantID], b)
	return b, nil
}

// UpdateBranch applique une modification partielle en respectant l'ordre
// d'installation : jamais de suspension ni d'archivage du site principal.
func (s *Service) UpdateBranch(ctx context.Context, tenantID, id string, req UpdateBranchRequest) (*Branch, error) {
	if req.Name != nil {
		v := normalize(*req.Name)
		if v == "" {
			return nil, ErrEmptyName
		}
		req.Name = &v
	}
	if req.Status != nil {
		v := normalize(*req.Status)
		if !StatutValide(v) {
			return nil, ErrInvalidStatus
		}
		req.Status = &v
	}
	if req.Code != nil {
		v := normalize(*req.Code)
		req.Code = &v
	}
	if req.City != nil {
		v := normalize(*req.City)
		req.City = &v
	}
	if req.Phone != nil {
		v := normalize(*req.Phone)
		req.Phone = &v
	}
	if req.Address != nil {
		v := normalize(*req.Address)
		req.Address = &v
	}

	if s.store != nil {
		return s.store.UpdateBranch(ctx, tenantID, id, req)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensurePrimaryLocked(tenantID)
	for _, b := range s.memoire[tenantID] {
		if b.ID != id {
			continue
		}
		if b.IsPrimary && req.Status != nil && *req.Status != StatusActive {
			return nil, ErrPrimaryProtected
		}
		if req.Name != nil {
			for _, autre := range s.memoire[tenantID] {
				if autre.ID != id && strings.EqualFold(autre.Name, *req.Name) {
					return nil, ErrNameExists
				}
			}
			b.Name = *req.Name
		}
		if req.Code != nil {
			b.Code = *req.Code
		}
		if req.Address != nil {
			b.Address = *req.Address
		}
		if req.Phone != nil {
			b.Phone = *req.Phone
		}
		if req.City != nil {
			b.City = *req.City
		}
		if req.Status != nil {
			b.Status = *req.Status
		}
		b.UpdatedAt = time.Now().UTC()
		return b, nil
	}
	return nil, ErrBranchNotFound
}

// ensurePrimaryLocked garantit qu'un pressing possède toujours un site
// principal, y compris en mode mémoire.
func (s *Service) ensurePrimaryLocked(tenantID string) {
	if len(s.memoire[tenantID]) > 0 {
		return
	}
	now := time.Now().UTC()
	s.memoire[tenantID] = []*Branch{{
		ID:        "branch-principal-" + tenantID,
		TenantID:  tenantID,
		Name:      "Établissement principal",
		Timezone:  "Africa/Abidjan",
		IsPrimary: true,
		Status:    StatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}}
}
