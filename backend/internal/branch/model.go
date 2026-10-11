package branch

import (
	"errors"
	"time"
)

var (
	// ErrBranchNotFound : identifiant inconnu OU appartenant à un autre
	// établissement (même réponse : l'existence n'est jamais révélée).
	ErrBranchNotFound = errors.New("établissement introuvable")
	// ErrEmptyName : le nom est l'identifiant humain d'un site.
	ErrEmptyName = errors.New("le nom de l'établissement est obligatoire")
	// ErrNameExists : deux sites du même pressing ne peuvent porter le même nom.
	ErrNameExists = errors.New("un établissement de ce nom existe déjà")
	// ErrInvalidStatus : statuts autorisés active | suspended | archived.
	ErrInvalidStatus = errors.New("statut d'établissement invalide (active, suspended, archived)")
	// ErrPrimaryProtected : l'établissement principal est le point d'ancrage des
	// commandes et des appartenances ; il ne peut être ni suspendu ni archivé.
	ErrPrimaryProtected = errors.New("l'établissement principal ne peut pas être suspendu ni archivé")
)

// Statuts d'un établissement (contrainte CHECK de la table branches).
const (
	StatusActive    = "active"
	StatusSuspended = "suspended"
	StatusArchived  = "archived"
)

// Branch est un établissement physique d'un pressing (multi-sites).
type Branch struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	Name      string `json:"name"`
	Code      string `json:"code,omitempty"`
	Address   string `json:"address,omitempty"`
	Phone     string `json:"phone,omitempty"`
	City      string `json:"city,omitempty"`
	Timezone  string `json:"timezone,omitempty"`
	IsPrimary bool   `json:"is_primary"`
	Status    string `json:"status"`
	// OrdersCount alimente l'écran de gestion des établissements (volume
	// d'activité par site) ; il n'est pas persisté.
	OrdersCount int       `json:"orders_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateBranchRequest corps de requête de création d'un site.
type CreateBranchRequest struct {
	Name    string `json:"name"`
	Code    string `json:"code,omitempty"`
	Address string `json:"address,omitempty"`
	Phone   string `json:"phone,omitempty"`
	City    string `json:"city,omitempty"`
}

// UpdateBranchRequest modification partielle : seuls les champs fournis changent.
type UpdateBranchRequest struct {
	Name    *string `json:"name,omitempty"`
	Code    *string `json:"code,omitempty"`
	Address *string `json:"address,omitempty"`
	Phone   *string `json:"phone,omitempty"`
	City    *string `json:"city,omitempty"`
	Status  *string `json:"status,omitempty"`
}

// StatutValide indique si un statut est autorisé.
func StatutValide(status string) bool {
	switch status {
	case StatusActive, StatusSuspended, StatusArchived:
		return true
	default:
		return false
	}
}
