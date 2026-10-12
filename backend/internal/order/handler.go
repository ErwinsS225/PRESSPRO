package order

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"pressing-os/backend/internal/auth"
	"pressing-os/backend/internal/billing"
	"pressing-os/backend/internal/httperr"
)

// Handler gère les requêtes HTTP sur les commandes
type Handler struct {
	service *Service
	logger  *log.Logger
}

// NewHandler initialise le gestionnaire HTTP des commandes
func NewHandler(service *Service) *Handler {
	return &Handler{service: service, logger: log.Default()}
}

// respondServiceError traduit une erreur du service en réponse HTTP :
//   - erreurs métier            -> 400 avec le message fonctionnel (utilisable par l'UI) ;
//   - ressource inexistante/hors -> 404 « commande introuvable » (aucune fuite d'existence) ;
//   - erreur technique (SQL/IO)  -> 500 générique, détail journalisé côté serveur.
func (h *Handler) respondServiceError(w http.ResponseWriter, err error, internalMsg string) {
	switch {
	case errors.Is(err, ErrOrderNotFound):
		httperr.Error(w, http.StatusNotFound, "commande introuvable")
	case billing.IsRefusMetier(err):
		// Refus de formule (plafond atteint / fonctionnalité non incluse) : 402 métier.
		httperr.ErrorWithExtra(w, http.StatusPaymentRequired, err.Error(), map[string]string{
			"upgrade_required": "true",
		})
	case errors.Is(err, ErrInvalidStatusOrder),
		errors.Is(err, ErrEmptyOrderItems),
		errors.Is(err, ErrInvalidPayment),
		errors.Is(err, ErrPaymentExceedsDue):
		httperr.Error(w, http.StatusBadRequest, err.Error())
	default:
		httperr.Internal(w, h.logger, "order", err, internalMsg)
	}
}

// SetQuotaGuard transmet le garde-fou au service : le contrôle du plafond vit
// dans le service, pas dans le handler (un seul chemin de décision).
func (h *Handler) SetQuotaGuard(guard *billing.QuotaGuard) {
	h.service.SetQuotaGuard(guard)
}

// CreateOrder gère POST /api/v1/orders
func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	tenantID := auth.GetTenantIDFromContext(r.Context())
	if tenantID == "" {
		w.WriteHeader(http.StatusForbidden)
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

	// Établissement d'imputation : le header X-Branch-ID a déjà été validé par le
	// contrôle de périmètre (#4 RequireBranchScope) en amont. Repli sur le corps
	// de requête si le header est absent.
	if hdr := strings.TrimSpace(r.Header.Get("X-Branch-ID")); hdr != "" {
		req.BranchID = hdr
	}

	created, err := h.service.CreateOrder(r.Context(), tenantID, req)
	if err != nil {
		h.respondServiceError(w, err, "impossible d'enregistrer la commande")
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
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "authentification requise"})
		return
	}

	statusFilter := r.URL.Query().Get("status")
	search := r.URL.Query().Get("q")

	orders, err := h.service.GetOrders(r.Context(), tenantID, statusFilter, search)
	if err != nil {
		h.respondServiceError(w, err, "impossible de récupérer les commandes")
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(orders)
}

// GetOrder gère GET /api/v1/orders/{id}
func (h *Handler) GetOrder(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	tenantID := auth.GetTenantIDFromContext(r.Context())
	if tenantID == "" {
		httperr.Error(w, http.StatusForbidden, "un contexte d'établissement est requis pour cette opération")
		return
	}

	orderID := chi.URLParam(r, "id")

	ord, err := h.service.GetOrderByID(r.Context(), tenantID, orderID)
	if err != nil {
		h.respondServiceError(w, err, "impossible de récupérer la commande")
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ord)
}

// UpdateStatus gère PATCH /api/v1/orders/{id}/status
func (h *Handler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	tenantID := auth.GetTenantIDFromContext(r.Context())
	if tenantID == "" {
		httperr.Error(w, http.StatusForbidden, "un contexte d'établissement est requis pour cette opération")
		return
	}

	orderID := chi.URLParam(r, "id")

	var req UpdateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "format JSON invalide"})
		return
	}

	updated, err := h.service.UpdateOrderStatus(r.Context(), tenantID, orderID, req.Status)
	if err != nil {
		h.respondServiceError(w, err, "impossible de mettre à jour le statut")
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

// AddPayment gère POST /api/v1/orders/{id}/payments
func (h *Handler) AddPayment(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	tenantID := auth.GetTenantIDFromContext(r.Context())
	if tenantID == "" {
		httperr.Error(w, http.StatusForbidden, "un contexte d'établissement est requis pour cette opération")
		return
	}

	orderID := chi.URLParam(r, "id")

	var req AddPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "format JSON invalide"})
		return
	}

	settled, err := h.service.AddPayment(r.Context(), tenantID, orderID, req)
	if err != nil {
		h.respondServiceError(w, err, "impossible d'enregistrer le paiement")
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(settled)
}

// ThermalReceipt gère GET /api/v1/orders/{id}/ticket
// Retourne la maquette formattée pour imprimante thermique 80mm
func (h *Handler) ThermalReceipt(w http.ResponseWriter, r *http.Request) {
	tenantID := auth.GetTenantIDFromContext(r.Context())
	if tenantID == "" {
		httperr.Error(w, http.StatusForbidden, "un contexte d'établissement est requis pour cette opération")
		return
	}

	claims := auth.GetUserFromContext(r.Context())
	orderID := chi.URLParam(r, "id")

	ord, err := h.service.GetOrderByID(r.Context(), tenantID, orderID)
	if err != nil {
		h.respondServiceError(w, err, "impossible de générer le ticket")
		return
	}

	// En-tête du ticket : l'identité vient du jeton de l'établissement connecté.
	// Aucune adresse ni société de démonstration n'est imprimée en dur.
	tenantName := "Votre pressing"
	if claims != nil && claims.TenantName != "" {
		tenantName = claims.TenantName
	}

	// Génération d'un ticket thermique textuel standard ESC/POS (80 colonnes)
	divider := strings.Repeat("-", 42)
	doubleDivider := strings.Repeat("=", 42)

	var sb strings.Builder
	sb.WriteString(doubleDivider + "\n")
	sb.WriteString(fmt.Sprintf("%26s\n", tenantName))
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
