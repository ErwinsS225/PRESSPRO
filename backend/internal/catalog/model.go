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
	CategoryDaily       = "Vêtements du quotidien"
	CategoryTraditional = "Vêtements traditionnels & cérémoniels"
	CategoryHomeLinen   = "Linge de maison"
	CategoryAddons      = "Services complémentaires & Options"
	CategorySpecialized = "Prestations spécialisées & Luxe"
	CategoryDryClean    = "Nettoyage à sec"
	CategoryDelicate    = "Délicat"
)

// Modèles commerciaux supportés
const (
	ModelPiece        = "piece"        // Modèle 1: Paiement à la pièce
	ModelKilogram     = "kilogram"     // Modèle 2: Facturation au kg
	ModelSubscription = "subscription" // Modèle 3: Abonnements mensuels
	ModelContract     = "contract"     // Modèle 4: Contrats professionnels B2B
)

// ServiceItem représente une prestation tarifée dans le catalogue
type ServiceItem struct {
	ID                 string    `json:"id"`
	TenantID           string    `json:"tenant_id"`
	Name               string    `json:"name"`
	Article            string    `json:"article"`
	Treatment          string    `json:"treatment"`
	Category           string    `json:"category"`
	UnitPrice          float64   `json:"unit_price"`
	Unit               string    `json:"unit"`          // pièce, ensemble, kg, panneau, m², paire, forfait
	PricingModel       string    `json:"pricing_model"` // piece, kilogram, subscription, contract
	EstimatedHours     int       `json:"estimated_hours"`
	IsActive           bool      `json:"is_active"`
	IsExpressAvailable bool      `json:"is_express_available"`
	ExpressSurcharge   float64   `json:"express_surcharge"` // en FCFA ou pourcentage
	IsPickupAvailable  bool      `json:"is_pickup_available"`
	IsOutsourced       bool      `json:"is_outsourced"`
	RequiresQuote      bool      `json:"requires_quote"`
	Description        string    `json:"description,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// CreateServiceRequest corps de requête pour créer une prestation
type CreateServiceRequest struct {
	Name               string   `json:"name"`
	Article            string   `json:"article,omitempty"`
	Treatment          string   `json:"treatment,omitempty"`
	Category           string   `json:"category"`
	UnitPrice          float64  `json:"unit_price"`
	Unit               string   `json:"unit"`
	PricingModel       string   `json:"pricing_model,omitempty"`
	EstimatedHours     int      `json:"estimated_hours"`
	IsActive           *bool    `json:"is_active,omitempty"`
	IsExpressAvailable *bool    `json:"is_express_available,omitempty"`
	ExpressSurcharge   *float64 `json:"express_surcharge,omitempty"`
	IsPickupAvailable  *bool    `json:"is_pickup_available,omitempty"`
	IsOutsourced       *bool    `json:"is_outsourced,omitempty"`
	RequiresQuote      *bool    `json:"requires_quote,omitempty"`
	Description        string   `json:"description,omitempty"`
}

// UpdateServiceRequest corps de requête pour modifier une prestation
type UpdateServiceRequest struct {
	Name               string   `json:"name"`
	Article            string   `json:"article,omitempty"`
	Treatment          string   `json:"treatment,omitempty"`
	Category           string   `json:"category"`
	UnitPrice          float64  `json:"unit_price"`
	Unit               string   `json:"unit"`
	PricingModel       string   `json:"pricing_model,omitempty"`
	EstimatedHours     int      `json:"estimated_hours"`
	IsActive           *bool    `json:"is_active,omitempty"`
	IsExpressAvailable *bool    `json:"is_express_available,omitempty"`
	ExpressSurcharge   *float64 `json:"express_surcharge,omitempty"`
	IsPickupAvailable  *bool    `json:"is_pickup_available,omitempty"`
	IsOutsourced       *bool    `json:"is_outsourced,omitempty"`
	RequiresQuote      *bool    `json:"requires_quote,omitempty"`
	Description        string   `json:"description,omitempty"`
}
