package order

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// VerifyTicketItem est la projection publique d'une ligne de ticket.
type VerifyTicketItem struct {
	Label     string  `json:"label"`
	Quantity  int     `json:"quantity"`
	UnitPrice float64 `json:"unit_price"`
	Subtotal  float64 `json:"subtotal"`
}

// VerifyTicketResponse est la projection publique d'un ticket (sans PII sensible).
type VerifyTicketResponse struct {
	Valid         bool               `json:"valid"`
	TenantID      string             `json:"tenant_id"`
	OrderID       string             `json:"order_id"`
	OrderNumber   string             `json:"order_number"`
	Status        string             `json:"status"`
	PaymentStatus string             `json:"payment_status"`
	TotalAmount   float64            `json:"total_amount"`
	PaidAmount    float64            `json:"paid_amount"`
	RemainingDue  float64            `json:"remaining_due"`
	DueDate       string             `json:"due_date"`
	CustomerName  string             `json:"customer_name"`
	Items         []VerifyTicketItem `json:"items"`
}

// VerifyTicket gère GET /api/v1/public/verify/{tenantID}/{orderID} (public, sans JWT).
// Retourne une projection limitée pour permettre au client de vérifier l'authenticité.
func (h *Handler) VerifyTicket(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	tenantID := chi.URLParam(r, "tenantID")
	orderID := chi.URLParam(r, "orderID")
	if tenantID == "" || orderID == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"valid": false,
			"error": "paramètres de vérification manquants",
		})
		return
	}

	ord, err := h.service.GetOrderByID(r.Context(), tenantID, orderID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"valid":     false,
			"tenant_id": tenantID,
			"order_id":  orderID,
			"error":     "ticket introuvable",
		})
		return
	}

	items := make([]VerifyTicketItem, 0, len(ord.Items))
	for _, it := range ord.Items {
		items = append(items, VerifyTicketItem{
			Label:     it.ItemLabel,
			Quantity:  it.Quantity,
			UnitPrice: it.UnitPrice,
			Subtotal:  it.Subtotal,
		})
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(VerifyTicketResponse{
		Valid:         true,
		TenantID:      ord.TenantID,
		OrderID:       ord.ID,
		OrderNumber:   ord.OrderNumber,
		Status:        ord.Status,
		PaymentStatus: ord.PaymentStatus,
		TotalAmount:   ord.TotalAmount,
		PaidAmount:    ord.PaidAmount,
		RemainingDue:  ord.RemainingDue,
		DueDate:       ord.DueDate,
		CustomerName:  ord.CustomerName,
		Items:         items,
	})
}
