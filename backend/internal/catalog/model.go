package catalog

import (
	"errors"
	"time"
)

var (
	ErrServiceNotFound = errors.New("prestation introuvable")
	ErrInvalidPrice    = errors.New("le tarif unitaire doit être supérieur ou égal à zéro")
	ErrEmptyName       = errors.New("la désignation de la prestation est obligatoire")
)

// Categories de soin standard en Côte d'Ivoire
const (
	CategoryWashAndIron = "Lavage & Repassage"
	CategoryDryClean    = "Nettoyage à sec"
	CategoryIronOnly    = "Repassage seul"
	CategoryDelicate    = "Délicat"
	CategoryHomeLinen   = "Linge de maison"
)

// ServiceItem représente une prestation tarifée dans le catalogue
type ServiceItem struct {
	ID             string    `json:"id"`
	TenantID       string    `json:"tenant_id"`
	Name           string    `json:"name"`
	Category       string    `json:"category"`
	UnitPrice      float64   `json:"unit_price"`
	Unit           string    `json:"unit"` // pièce, ensemble, kg, mètre
	EstimatedHours int       `json:"estimated_hours"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// CreateServiceRequest corps de requête pour créer une prestation
type CreateServiceRequest struct {
	Name           string  `json:"name"`
	Category       string  `json:"category"`
	UnitPrice      float64 `json:"unit_price"`
	Unit           string  `json:"unit"`
	EstimatedHours int     `json:"estimated_hours"`
	IsActive       *bool   `json:"is_active,omitempty"`
}

// UpdateServiceRequest corps de requête pour modifier une prestation
type UpdateServiceRequest struct {
	Name           string  `json:"name"`
	Category       string  `json:"category"`
	UnitPrice      float64 `json:"unit_price"`
	Unit           string  `json:"unit"`
	EstimatedHours int     `json:"estimated_hours"`
	IsActive       *bool   `json:"is_active,omitempty"`
}
