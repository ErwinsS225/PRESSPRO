package pricelist

import "time"

// PriceListEntry représente une ligne du snapshot figé (historique immuable) d'une version.
type PriceListEntry struct {
	ServiceID    string  `json:"service_id"`
	Name         string  `json:"name"`
	Article      string  `json:"article,omitempty"`
	Treatment    string  `json:"treatment,omitempty"`
	Category     string  `json:"category,omitempty"`
	UnitPrice    float64 `json:"unit_price"`
	Unit         string  `json:"unit"`
	PricingModel string  `json:"pricing_model"`
	IsActive     bool    `json:"is_active"`
}

// PriceList est une grille tarifaire appartenant au tenant (portée branche facultative).
type PriceList struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	BranchID    string    `json:"branch_id,omitempty"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	IsDefault   bool      `json:"is_default"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// PriceListVersion est une version figée : le snapshot n'est jamais modifié après création.
type PriceListVersion struct {
	ID          string           `json:"id"`
	PriceListID string           `json:"price_list_id"`
	TenantID    string           `json:"tenant_id"`
	Version     int              `json:"version"`
	Status      string           `json:"status"`
	Snapshot    []PriceListEntry `json:"snapshot"`
	Note        string           `json:"note,omitempty"`
	CreatedAt   time.Time        `json:"created_at"`
}

// PriceListWithVersions regroupe une grille et son historique de versions.
type PriceListWithVersions struct {
	PriceList
	Versions []PriceListVersion `json:"versions"`
}

// CreatePriceListRequest valide la création d'une grille.
type CreatePriceListRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	BranchID    string `json:"branch_id,omitempty"`
	IsDefault   bool   `json:"is_default,omitempty"`
}

// CreateVersionRequest fige une nouvelle version. Si Prices est vide, le
// snapshot est généré depuis le catalogue courant ; sinon les prix fournis
// écrasent les lignes correspondantes (service_id -> unit_price).
type CreateVersionRequest struct {
	Note   string               `json:"note,omitempty"`
	Prices []CreateVersionEntry `json:"prices,omitempty"`
}

// CreateVersionEntry correspond à un prix à figer pour une prestation donnée.
type CreateVersionEntry struct {
	ServiceID string  `json:"service_id"`
	UnitPrice float64 `json:"unit_price"`
}
