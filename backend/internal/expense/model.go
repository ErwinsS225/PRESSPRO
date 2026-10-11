package expense

import (
	"errors"
	"time"
)

var (
	// ErrExpenseNotFound : identifiant inconnu OU appartenant à un autre
	// établissement (l'existence n'est jamais révélée).
	ErrExpenseNotFound = errors.New("dépense introuvable")
	ErrEmptyLabel      = errors.New("le libellé de la dépense est obligatoire")
	ErrInvalidAmount   = errors.New("le montant de la dépense doit être supérieur à zéro")
	ErrInvalidCategory = errors.New("catégorie de dépense invalide")
	ErrInvalidDate     = errors.New("date de dépense invalide (format attendu AAAA-MM-JJ)")
)

// Categories : référentiel des charges d'exploitation d'un pressing.
var Categories = []string{
	"matieres_premieres",
	"produits_entretien",
	"loyer",
	"energie_eau",
	"salaires",
	"transport",
	"equipement",
	"maintenance",
	"taxes",
	"divers",
}

// CategorieValide indique si la catégorie appartient au référentiel.
func CategorieValide(c string) bool {
	for _, ref := range Categories {
		if ref == c {
			return true
		}
	}
	return false
}

// Expense : une charge d'exploitation datée, isolée par pressing.
type Expense struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenant_id"`
	BranchID      string    `json:"branch_id,omitempty"`
	Category      string    `json:"category"`
	Label         string    `json:"label"`
	AmountXOF     float64   `json:"amount_xof"`
	PaymentMethod string    `json:"payment_method,omitempty"`
	SpentAt       string    `json:"spent_at"` // AAAA-MM-JJ
	Note          string    `json:"note,omitempty"`
	CreatedBy     string    `json:"created_by,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CreateExpenseRequest corps de requête de création.
type CreateExpenseRequest struct {
	BranchID      string  `json:"branch_id,omitempty"`
	Category      string  `json:"category"`
	Label         string  `json:"label"`
	AmountXOF     float64 `json:"amount_xof"`
	PaymentMethod string  `json:"payment_method,omitempty"`
	SpentAt       string  `json:"spent_at,omitempty"`
	Note          string  `json:"note,omitempty"`
}

// Filter sélectionne les dépenses d'une période.
type Filter struct {
	From     string // AAAA-MM-JJ inclus
	To       string // AAAA-MM-JJ inclus
	Category string
	BranchID string
}

// CategoryTotal agrège les dépenses par catégorie.
type CategoryTotal struct {
	Category string  `json:"category"`
	TotalXOF float64 `json:"total_xof"`
	Count    int     `json:"count"`
}
