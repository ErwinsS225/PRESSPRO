package auth

import (
	"context"
	"errors"
	"fmt"
	"log"
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
	demoIDs    map[string]struct{}       // comptes de démonstration embarqués (non persistés)
	store      *dbStore                  // nil = mode mémoire ; non-nil = persistance PostgreSQL

	// onTenantRegistered est appelé après chaque inscription réussie (onboarding
	// du catalogue, grille tarifaire…). Injecté au démarrage par l'assemblage
	// applicatif afin que le module auth n'ait aucune dépendance métier.
	onTenantRegistered []func(ctx context.Context, tenantID string) error
}

// SetOnTenantRegistered enregistre un ou plusieurs hooks d'onboarding exécutés
// après la création d'un établissement. Un échec de hook est journalisé mais
// n'annule jamais l'inscription (l'établissement et son compte existent déjà).
func (s *Service) SetOnTenantRegistered(hooks ...func(ctx context.Context, tenantID string) error) {
	s.onTenantRegistered = append(s.onTenantRegistered, hooks...)
}

// runOnTenantRegistered exécute les hooks d'onboarding.
func (s *Service) runOnTenantRegistered(ctx context.Context, tenantID string) {
	for _, hook := range s.onTenantRegistered {
		if hook == nil {
			continue
		}
		if err := hook(ctx, tenantID); err != nil {
			log.Printf("[auth] onboarding: échec de l'initialisation du tenant %s: %v", tenantID, err)
		}
	}
}

// NewService initialise le service avec des comptes de démonstration prêts à l'emploi
func NewService(jwtManager *JWTManager) *Service {
	s := &Service{
		jwtManager: jwtManager,
		users:      make(map[string]*User),
		tenants:    make(map[string]*TenantSummary),
		demoIDs:    make(map[string]struct{}),
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

	// Registre explicite des comptes de démonstration embarqués. Ces identifiants
	// n'existent pas en base : le contrôle d'appartenance les autorise nommément,
	// tout autre identifiant inconnu étant refusé (fail-closed).
	for _, u := range s.users {
		s.demoIDs[u.ID] = struct{}{}
	}

	return s
}

// IsDemoAccount indique si l'identifiant appartient à l'un des comptes de
// démonstration embarqués dans le binaire (vitrine / mode dégradé).
//
// Le contrôle d'appartenance s'appuie sur ce registre EXHAUSTIF : hors mode
// dégradé, tout identifiant d'utilisateur qui n'est ni un compte de
// démonstration ni un utilisateur présent en base est refusé (fail-closed).
func (s *Service) IsDemoAccount(userID string) bool {
	if s == nil || userID == "" {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.demoIDs[userID]
	return ok
}

// Login vérifie l'email et le mot de passe et retourne un jeton JWT signé.
// Priorité à la persistance PostgreSQL ; repli mémoire (comptes de démo) si absents.
func (s *Service) Login(ctx context.Context, email, password string) (*LoginResponse, error) {
	normEmail := strings.ToLower(strings.TrimSpace(email))

	var user *User
	if s.store != nil {
		if dbUser, derr := s.store.FindUserByEmail(ctx, normEmail); derr == nil && dbUser != nil {
			user = dbUser
		}
	}
	if user == nil {
		s.mu.RLock()
		if memUser, ok := s.users[normEmail]; ok {
			cp := *memUser
			user = &cp
		}
		s.mu.RUnlock()
	}
	if user == nil {
		return nil, ErrInvalidCredentials
	}

	if !CheckPassword(user.PasswordHash, password) {
		return nil, ErrInvalidCredentials
	}

	return s.buildLoginResponse(user)
}

// buildLoginResponse résout le tenant (DB puis mémoire) et signe le jeton.
func (s *Service) buildLoginResponse(user *User) (*LoginResponse, error) {
	var tenant *TenantSummary
	var tenantName string
	if user.TenantID != "" {
		if s.store != nil {
			if t, terr := s.store.FindTenant(context.Background(), user.TenantID); terr == nil && t != nil {
				tenant = t
			}
		}
		if tenant == nil {
			s.mu.RLock()
			tenant = s.tenants[user.TenantID]
			s.mu.RUnlock()
		}
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

// GetUsersByTenant retourne la liste des utilisateurs pour un tenant donné.
// L'isolation est garantie par la couche de persistance (requête SQL) ou par filtrage en mémoire.
func (s *Service) GetUsersByTenant(ctx context.Context, tenantID string) ([]*User, error) {
	if tenantID == "" {
		return nil, errors.New("l'identifiant du tenant est requis")
	}

	// Mode persistant (PostgreSQL)
	if s.store != nil {
		return s.store.FindUsersByTenant(ctx, tenantID)
	}

	// Repli mémoire (mode dégradé)
	s.mu.RLock()
	defer s.mu.RUnlock()

	var tenantUsers []*User
	for _, u := range s.users {
		if u.TenantID == tenantID {
			// On retourne une copie pour éviter les modifications concurrentes
			cp := *u
			tenantUsers = append(tenantUsers, &cp)
		}
	}

	return tenantUsers, nil
}

// GetUserByID recherche un utilisateur par son identifiant.
// Priorité à la persistance PostgreSQL ; repli mémoire (comptes de démo) si absent.
func (s *Service) GetUserByID(ctx context.Context, userID string) (*User, *TenantSummary, error) {
	if s.store != nil {
		if u, err := s.store.FindUserByID(ctx, userID); err == nil && u != nil {
			var tenant *TenantSummary
			if u.TenantID != "" {
				if t, terr := s.store.FindTenant(ctx, u.TenantID); terr == nil && t != nil {
					tenant = t
				}
			}
			return u, tenant, nil
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, u := range s.users {
		if u.ID == userID {
			cp := *u
			var tenant *TenantSummary
			if cp.TenantID != "" {
				tenant = s.tenants[cp.TenantID]
			}
			return &cp, tenant, nil
		}
	}

	return nil, nil, ErrUserNotFound
}

// RegisterTenant crée un nouvel établissement et son compte propriétaire.
// Persistance prioritaire (tenant + branche + user + memberships + essai J7) ;
// repli mémoire si PostgreSQL n'est pas disponible au démarrage.
func (s *Service) RegisterTenant(ctx context.Context, req RegisterTenantRequest) (*LoginResponse, error) {
	normEmail := strings.ToLower(strings.TrimSpace(req.OwnerEmail))

	// Mode persistant : création transactionnelle complète adossée à PostgreSQL.
	if s.store != nil {
		user, tenant, err := s.store.RegisterTenant(ctx, req)
		if err != nil {
			return nil, err
		}
		token, terr := s.jwtManager.Generate(user, tenant.Name)
		if terr != nil {
			return nil, fmt.Errorf("impossible de générer le jeton: %w", terr)
		}
		// Onboarding du nouvel établissement (catalogue, grille tarifaire…).
		s.runOnTenantRegistered(ctx, tenant.ID)
		return &LoginResponse{Token: token, User: *user, Tenant: tenant}, nil
	}

	// Repli mémoire (mode dégradé sans base de données).
	s.mu.Lock()
	defer s.mu.Unlock()

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

	// Onboarding du nouvel établissement (mode mémoire).
	s.runOnTenantRegistered(ctx, tenant.ID)

	return &LoginResponse{
		Token:  token,
		User:   *user,
		Tenant: tenant,
	}, nil
}
