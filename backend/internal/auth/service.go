package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidCredentials = errors.New("identifiants de connexion invalides")
	ErrUserNotFound       = errors.New("utilisateur introuvable")
	ErrEmailAlreadyExists = errors.New("cette adresse email est déjà utilisée")
	ErrTenantNotFound     = errors.New("établissement introuvable")
)

// Service gère la logique d'authentification et les utilisateurs
type Service struct {
	jwtManager *JWTManager
	mu         sync.RWMutex
	users      map[string]*User          // clé: email normalisé
	tenants    map[string]*TenantSummary // clé: tenant_id
}

// NewService initialise le service avec des comptes de démonstration prêts à l'emploi
func NewService(jwtManager *JWTManager) *Service {
	s := &Service{
		jwtManager: jwtManager,
		users:      make(map[string]*User),
		tenants:    make(map[string]*TenantSummary),
	}

	// Établissement initial
	defaultTenantID := "a0000000-0000-0000-0000-000000000001"
	s.tenants[defaultTenantID] = &TenantSummary{
		ID:       defaultTenantID,
		Name:     "Pressing Ivoire Cocody",
		Slug:     "pressing-ivoire-cocody",
		City:     "Abidjan (Cocody)",
		Currency: "XOF",
		Status:   "active",
	}

	// Compte Gérant / Propriétaire (Pressing Cocody)
	hash1, _ := HashPassword("pass123")
	s.users["gerant@pressing-ivoire.ci"] = &User{
		ID:           "usr-gerant-001",
		TenantID:     defaultTenantID,
		Email:        "gerant@pressing-ivoire.ci",
		PasswordHash: hash1,
		FullName:     "M. Bamba Seydou",
		Phone:        "+225 07 00 11 22 33",
		Role:         RoleOwner,
		Status:       "active",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	// Compte Opérateur Atelier (Blanchisseur)
	hash2, _ := HashPassword("pass123")
	s.users["atelier@pressing-ivoire.ci"] = &User{
		ID:           "usr-operator-002",
		TenantID:     defaultTenantID,
		Email:        "atelier@pressing-ivoire.ci",
		PasswordHash: hash2,
		FullName:     "Koffi Yao (Chef d'atelier)",
		Phone:        "+225 05 12 34 56 78",
		Role:         RoleOperator,
		Status:       "active",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	// Compte SuperAdmin SaaS (Éditeur PressingOS)
	hash3, _ := HashPassword("admin123")
	s.users["admin@pressingos.ci"] = &User{
		ID:           "usr-superadmin-000",
		TenantID:     "", // Non rattaché à un seul pressing
		Email:        "admin@pressingos.ci",
		PasswordHash: hash3,
		FullName:     "Administrateur PressingOS SaaS",
		Phone:        "+225 01 00 00 00 00",
		Role:         RoleSuperAdmin,
		Status:       "active",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	return s
}

// Login vérifie l'email et le mot de passe et retourne un jeton JWT signé
func (s *Service) Login(ctx context.Context, email, password string) (*LoginResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	normEmail := strings.ToLower(strings.TrimSpace(email))
	user, exists := s.users[normEmail]
	if !exists {
		return nil, ErrInvalidCredentials
	}

	if !CheckPassword(user.PasswordHash, password) {
		return nil, ErrInvalidCredentials
	}

	var tenant *TenantSummary
	var tenantName string
	if user.TenantID != "" {
		tenant = s.tenants[user.TenantID]
		if tenant != nil {
			tenantName = tenant.Name
		}
	} else {
		tenantName = "SaaS Administration Globale"
	}

	token, err := s.jwtManager.Generate(user, tenantName)
	if err != nil {
		return nil, fmt.Errorf("impossible de générer le jeton: %w", err)
	}

	return &LoginResponse{
		Token:  token,
		User:   *user,
		Tenant: tenant,
	}, nil
}

// GetUserByID recherche un utilisateur par son identifiant
func (s *Service) GetUserByID(ctx context.Context, userID string) (*User, *TenantSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, u := range s.users {
		if u.ID == userID {
			var tenant *TenantSummary
			if u.TenantID != "" {
				tenant = s.tenants[u.TenantID]
			}
			return u, tenant, nil
		}
	}

	return nil, nil, ErrUserNotFound
}

// RegisterTenant crée un nouvel établissement et son compte propriétaire
func (s *Service) RegisterTenant(ctx context.Context, req RegisterTenantRequest) (*LoginResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	normEmail := strings.ToLower(strings.TrimSpace(req.OwnerEmail))
	if _, exists := s.users[normEmail]; exists {
		return nil, ErrEmailAlreadyExists
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	tenantID := fmt.Sprintf("tenant-%d", time.Now().UnixNano())
	tenantSlug := strings.ToLower(strings.ReplaceAll(req.TenantName, " ", "-"))

	tenant := &TenantSummary{
		ID:       tenantID,
		Name:     req.TenantName,
		Slug:     tenantSlug,
		City:     req.City,
		Currency: "XOF",
		Status:   "trial",
	}
	s.tenants[tenantID] = tenant

	user := &User{
		ID:           fmt.Sprintf("usr-%d", time.Now().UnixNano()),
		TenantID:     tenantID,
		Email:        normEmail,
		PasswordHash: hash,
		FullName:     req.OwnerFullName,
		Phone:        req.Phone,
		Role:         RoleOwner,
		Status:       "active",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	s.users[normEmail] = user

	token, err := s.jwtManager.Generate(user, tenant.Name)
	if err != nil {
		return nil, err
	}

	return &LoginResponse{
		Token:  token,
		User:   *user,
		Tenant: tenant,
	}, nil
}
