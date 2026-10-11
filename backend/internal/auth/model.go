package auth

import "time"

// Rôles utilisateurs du système SaaS PressingOS
const (
	RoleSuperAdmin = "superadmin" // Administrateur global de l'éditeur SaaS (abonnements, plateformes)
	RoleOwner      = "owner"      // Propriétaire du pressing (gestion complète, finances, employés)
	RoleManager    = "manager"    // Gérant de pressing (tarifs, commandes, clients)
	RoleCashier    = "cashier"    // Caissier (dépôts, encaissements, remise du linge)
	RoleOperator   = "operator"   // Blanchisseur / Repasseur (traitement du linge)
)

// User représente un utilisateur dans le système
type User struct {
	ID           string    `json:"id"`
	TenantID     string    `json:"tenant_id,omitempty"` // Optionnel pour le superadmin global
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	FullName     string    `json:"full_name"`
	Phone        string    `json:"phone,omitempty"`
	Role         string    `json:"role"`
	Status       string    `json:"status"` // active, inactive, locked
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TenantSummary représente un résumé de l'établissement rattaché
type TenantSummary struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	City     string `json:"city"`
	Currency string `json:"currency"`
	Status   string `json:"status"`
}

// LoginRequest corps de requête d'authentification
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse retour avec le jeton et le profil
type LoginResponse struct {
	Token  string         `json:"token"`
	User   User           `json:"user"`
	Tenant *TenantSummary `json:"tenant,omitempty"`
}

// CreateUserRequest formulaire de création d'un compte employé du pressing.
// Le rôle « superadmin » est volontairement hors de portée d'un pressing :
// il n'est attribuable que par l'éditeur de la plateforme.
type CreateUserRequest struct {
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Role     string `json:"role"`
	Password string `json:"password"`
}

// RegisterTenantRequest formulaire d'inscription d'un nouveau pressing (Onboarding)
type RegisterTenantRequest struct {
	TenantName    string `json:"tenant_name"`
	City          string `json:"city"`
	Phone         string `json:"phone"`
	OwnerFullName string `json:"owner_full_name"`
	OwnerEmail    string `json:"owner_email"`
	Password      string `json:"password"`
}
