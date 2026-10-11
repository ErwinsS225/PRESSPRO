package order

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"pressing-os/backend/internal/auth"
)

// Handler gère les requêtes HTTP sur les commandes
type Handler struct {
	service *Service
}

// NewHandler initialise le gestionnaire HTTP des commandes
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// CreateOrder gère POST /api/v1/orders
func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	tenantID := auth.GetTenantIDFromContext(r.Context())
	if tenantID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "contexte d'établissement manquant"})
		return
	}

	var req CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "format JSON invalide"})
		return
	}

	if req.CustomerName == "" || len(req.Items) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "le nom du client et les vêtements sont obligatoires"})
		return
	}

	created, err := h.service.CreateOrder(r.Context(), tenantID, req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

// ListOrders gère GET /api/v1/orders
func (h *Handler) ListOrders(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	tenantID := auth.GetTenantIDFromContext(r.Context())
	if tenantID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "authentification requise"})
		return
	}

	statusFilter := r.URL.Query().Get("status")
	search := r.URL.Query().Get("q")

	orders, err := h.service.GetOrders(r.Context(), tenantID, statusFilter, search)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(orders)
}

// GetOrder gère GET /api/v1/orders/{id}
func (h *Handler) GetOrder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	tenantID := auth.GetTenantIDFromContext(r.Context())
	orderID := chi.URLParam(r, "id")

	ord, err := h.service.GetOrderByID(r.Context(), tenantID, orderID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "commande introuvable"})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ord)
}

// UpdateStatus gère PATCH /api/v1/orders/{id}/status
func (h *Handler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	tenantID := auth.GetTenantIDFromContext(r.Context())
	orderID := chi.URLParam(r, "id")

	var req UpdateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "format JSON invalide"})
		return
	}

	updated, err := h.service.UpdateOrderStatus(r.Context(), tenantID, orderID, req.Status)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

// AddPayment gère POST /api/v1/orders/{id}/payments
func (h *Handler) AddPayment(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	tenantID := auth.GetTenantIDFromContext(r.Context())
	orderID := chi.URLParam(r, "id")

	var req AddPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "format JSON invalide"})
		return
	}

	settled, err := h.service.AddPayment(r.Context(), tenantID, orderID, req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(settled)
}

// ThermalReceipt gère GET /api/v1/orders/{id}/ticket
// Retourne la maquette formattée pour imprimante thermique 80mm
func (h *Handler) ThermalReceipt(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.GetTenantIDFromContext(r.Context())
	claims := auth.GetUserFromContext(r.Context())
	orderID := chi.URLParam(r, "id")

	ord, err := h.service.GetOrderByID(r.Context(), tenantID, orderID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "commande introuvable"})
		return
	}

	tenantName := "Pressing Ivoire Cocody"
	if claims != nil && claims.TenantName != "" {
		tenantName = claims.TenantName
	}

	// Génération d'un ticket thermique textuel standard ESC/POS (80 colonnes)
	divider := strings.Repeat("-", 42)
	doubleDivider := strings.Repeat("=", 42)

	var sb strings.Builder
	sb.WriteString(doubleDivider + "\n")
	sb.WriteString(fmt.Sprintf("%26s\n", tenantName))
	sb.WriteString("    Cocody Angre 8e Tranche, Abidjan\n")
	sb.WriteString("         Tel: +225 07 00 11 22 33\n")
	sb.WriteString(doubleDivider + "\n")
	sb.WriteString(fmt.Sprintf(" TICKET DEPOT : %s\n", ord.OrderNumber))
	sb.WriteString(fmt.Sprintf(" Date Depot   : %s\n", ord.CreatedAt.Format("02/01/2006 15:04")))
	sb.WriteString(fmt.Sprintf(" Retrait Prevu: %s\n", ord.DueDate))
	sb.WriteString(divider + "\n")
	sb.WriteString(fmt.Sprintf(" Client : %s\n", ord.CustomerName))
	sb.WriteString(fmt.Sprintf(" Contact: %s\n", ord.CustomerPhone))
	sb.WriteString(divider + "\n")
	sb.WriteString(" QTE  DESIGNATION              TOTAL (F)\n")
	sb.WriteString(divider + "\n")

	for _, item := range ord.Items {
		label := item.ItemLabel
		if len(label) > 20 {
			label = label[:20]
		}
		sb.WriteString(fmt.Sprintf(" %2dx %-22s %7.0f F\n", item.Quantity, label, item.Subtotal))
	}

	sb.WriteString(divider + "\n")
	sb.WriteString(fmt.Sprintf(" TOTAL GENERAL :           %8.0f FCFA\n", ord.TotalAmount))
	sb.WriteString(fmt.Sprintf(" ACOMPTE VERSE :           %8.0f FCFA\n", ord.PaidAmount))
	sb.WriteString(fmt.Sprintf(" RESTE A PAYER :           %8.0f FCFA\n", ord.RemainingDue))
	sb.WriteString(doubleDivider + "\n")
	sb.WriteString("  * Merci de votre confiance ! *\n")
	sb.WriteString(" * Ticket a presenter pour le retrait *\n")
	sb.WriteString(doubleDivider + "\n")

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(sb.String()))
}
