package customer

import (
	"errors"
	"time"
)

var (
	ErrCustomerNotFound = errors.New("client introuvable")
	ErrEmptyFirstName   = errors.New("le prénom du client est obligatoire")
	ErrEmptyPhone       = errors.New("le numéro de téléphone du client est obligatoire")
	ErrPhoneExists      = errors.New("ce numéro de téléphone est déjà enregistré pour ce établissement")
)

// Customer représente un client du pressing, isolé par tenant.
// Un même numéro ne peut exister qu'une fois par établissement (UNIQUE tenant_id+phone).
type Customer struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Phone     string    `json:"phone"`
	Email     string    `json:"email,omitempty"`
	Address   string    `json:"address,omitempty"`
	Notes     string    `json:"notes,omitempty"`
	FullName  string    `json:"full_name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreateCustomerRequest corps de requête pour créer un client.
type CreateCustomerRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
	Email     string `json:"email,omitempty"`
	Address   string `json:"address,omitempty"`
	Notes     string `json:"notes,omitempty"`
}

// UpdateCustomerRequest corps de requête pour modifier un client.
type UpdateCustomerRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
	Email     string `json:"email,omitempty"`
	Address   string `json:"address,omitempty"`
	Notes     string `json:"notes,omitempty"`
}
