package order

import (
	"errors"
	"time"

	"pressing-os/backend/internal/customer"
)

// Constantes des statuts du cycle de traitement du linge
const (
	StatusReceived  = "received"  // 1. Réceptionné au comptoir
	StatusWashing   = "washing"   // 2. En cours de lavage / nettoyage à sec
	StatusIroning   = "ironing"   // 3. En cours de repassage
	StatusReady     = "ready"     // 4. Prêt au comptoir pour retrait
	StatusDelivered = "delivered" // 5. Livré / Remis au client
	StatusCancelled = "cancelled" // Annulé
)

// Constantes des statuts de paiement
const (
	PaymentStatusUnpaid  = "unpaid"         // Non payé (à régler au retrait)
	PaymentStatusPartial = "partially_paid" // Acompte versé
	PaymentStatusPaid    = "paid"           // Soldé intégralement
)

// Modes de paiement courants en Côte d'Ivoire
const (
	MethodCash        = "cash"         // Espèces
	MethodWave        = "wave"         // Wave Côte d'Ivoire
	MethodOrangeMoney = "orange_money" // Orange Money
	MethodMTNMoMo     = "mtn_momo"     // MTN Mobile Money
	MethodCard        = "card"         // Carte bancaire
)

var (
	ErrFullPaymentRequired       = errors.New("le paiement intégral est obligatoire au dépôt")
	ErrUnpaidDelivery            = errors.New("restitution impossible : le solde doit être nul")
	ErrInvalidOrderItem          = errors.New("article invalide : désignation, quantité ou prix hors limites")
	ErrInvalidPaymentMethod      = errors.New("mode de paiement non autorisé")
	ErrIdempotencyConflict       = errors.New("cette clé de requête a déjà été utilisée avec un contenu différent")
	ErrInvalidIdempotencyKey     = errors.New("clé d'idempotence invalide")
	ErrCustomerSelectionRequired = errors.New("un client utilise déjà ce téléphone : sélectionnez sa fiche ou confirmez explicitement un nouveau client")
	ErrAmbiguousCustomer         = errors.New("choisissez un client existant ou un nouveau client, sans mélanger les deux saisies")
	ErrOrderNotFound             = errors.New("commande introuvable")
	ErrInvalidStatusOrder        = errors.New("transition de statut de linge non autorisée")
	ErrEmptyOrderItems           = errors.New("une commande doit comporter au moins un vêtement")
	ErrInvalidPayment            = errors.New("le montant du paiement est invalide")
	// ErrPaymentExceedsDue protège l'intégrité comptable : un encaissement ne
	// peut jamais dépasser le solde restant dû (le journal des paiements doit
	// toujours correspondre au montant encaissable de la commande).
	ErrPaymentExceedsDue = errors.New("le montant dépasse le solde restant dû de la commande")
)

// OrderItem représente une ligne d'article dans une commande
type OrderItem struct {
	ID        string  `json:"id"`
	ServiceID string  `json:"service_id"`
	ItemLabel string  `json:"item_label"`
	Quantity  int     `json:"quantity"`
	UnitPrice float64 `json:"unit_price"`
	Subtotal  float64 `json:"subtotal"`
	Notes     string  `json:"notes,omitempty"`
}

// PaymentRecord enregistre un paiement perçu
type PaymentRecord struct {
	AmountReceived *float64  `json:"amount_received,omitempty"`
	ChangeGiven    *float64  `json:"change_given,omitempty"`
	ReceivedBy     string    `json:"received_by,omitempty"`
	ID             string    `json:"id"`
	Amount         float64   `json:"amount"`
	PaymentMethod  string    `json:"payment_method"`
	Reference      string    `json:"reference,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// Order représente une commande de pressing
type Order struct {
	Persisted     bool            `json:"persisted"`
	ID            string          `json:"id"`
	TenantID      string          `json:"tenant_id"`
	CustomerID    string          `json:"customer_id"`
	CustomerName  string          `json:"customer_name"`
	CustomerPhone string          `json:"customer_phone"`
	OrderNumber   string          `json:"order_number"` // ex: CMD-2026-101
	Status        string          `json:"status"`
	PaymentStatus string          `json:"payment_status"`
	TotalAmount   float64         `json:"total_amount"`
	PaidAmount    float64         `json:"paid_amount"`
	RemainingDue  float64         `json:"remaining_due"`
	Items         []OrderItem     `json:"items"`
	Payments      []PaymentRecord `json:"payments"`
	DueDate       string          `json:"due_date"`
	Notes         string          `json:"notes,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// CreateOrderItemInput saisie d'un article lors du dépôt
type CreateOrderItemInput struct {
	ServiceID string  `json:"service_id"`
	ItemLabel string  `json:"item_label"`
	Quantity  int     `json:"quantity"`
	UnitPrice float64 `json:"unit_price"`
	Notes     string  `json:"notes,omitempty"`
}

// CreateOrderRequest corps de requête pour la création d'un dépôt
type CreateOrderRequest struct {
	AmountReceived *float64                        `json:"amount_received,omitempty"`
	IdempotencyKey string                          `json:"-"`
	CustomerID     string                          `json:"customer_id,omitempty"`
	NewCustomer    *customer.CreateCustomerRequest `json:"new_customer,omitempty"`
	BranchID       string                          `json:"branch_id,omitempty"`
	CustomerName   string                          `json:"customer_name"`
	CustomerPhone  string                          `json:"customer_phone"`
	Items          []CreateOrderItemInput          `json:"items"`
	InitialPaid    float64                         `json:"initial_paid"`
	PaymentMethod  string                          `json:"payment_method,omitempty"`
	DueDate        string                          `json:"due_date,omitempty"`
	Notes          string                          `json:"notes,omitempty"`
}

// AddPaymentRequest saisie d'un règlement acompte ou solde
type AddPaymentRequest struct {
	AmountReceived *float64 `json:"amount_received,omitempty"`
	IdempotencyKey string   `json:"-"`
	Amount         float64  `json:"amount"`
	PaymentMethod  string   `json:"payment_method"`
	Reference      string   `json:"reference,omitempty"`
}

// UpdateStatusRequest mise à jour du statut en atelier
type UpdateStatusRequest struct {
	Status string `json:"status"`
}
