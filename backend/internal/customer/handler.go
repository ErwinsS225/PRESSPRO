package customer

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"pressing-os/backend/internal/auth"
	"pressing-os/backend/internal/httperr"
)

// Handler gère les requêtes HTTP sur les clients du pressing.
type Handler struct {
	service *Service
	logger  *log.Logger
}

// NewHandler initialise le gestionnaire HTTP des clients.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service, logger: log.Default()}
}

// ListCustomers gère GET /api/v1/customers
func (h *Handler) ListCustomers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	tenantID := auth.GetTenantIDFromContext(r.Context())
	if tenantID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "contexte d'établissement manquant"})
		return
	}

	search := r.URL.Query().Get("q")
	items, err := h.service.GetCustomers(r.Context(), tenantID, search)
	if err != nil {
		httperr.Internal(w, h.logger, "customer", err, "impossible de récupérer les clients")
		return
	}
	if items == nil {
		items = []*Customer{}
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(items)
}

// CreateCustomer gère POST /api/v1/customers
func (h *Handler) CreateCustomer(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	tenantID := auth.GetTenantIDFromContext(r.Context())
	if tenantID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "authentification requise"})
		return
	}

	var req CreateCustomerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "format JSON invalide"})
		return
	}

	created, err := h.service.CreateCustomer(r.Context(), tenantID, req)
	if err != nil {
		w.WriteHeader(customerErrorStatus(err))
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

// GetCustomer gère GET /api/v1/customers/{id}
func (h *Handler) GetCustomer(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	tenantID := auth.GetTenantIDFromContext(r.Context())
	if tenantID == "" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "un contexte d'établissement est requis pour cette opération"})
		return
	}
	id := chi.URLParam(r, "id")

	item, err := h.service.GetCustomer(r.Context(), tenantID, id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "client introuvable"})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(item)
}

// UpdateCustomer gère PUT /api/v1/customers/{id}
func (h *Handler) UpdateCustomer(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	tenantID := auth.GetTenantIDFromContext(r.Context())
	if tenantID == "" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "un contexte d'établissement est requis pour cette opération"})
		return
	}
	id := chi.URLParam(r, "id")

	var req UpdateCustomerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "format JSON invalide"})
		return
	}

	updated, err := h.service.UpdateCustomer(r.Context(), tenantID, id, req)
	if err != nil {
		if errors.Is(err, ErrCustomerNotFound) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(customerErrorStatus(err))
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

// DeleteCustomer gère DELETE /api/v1/customers/{id}
func (h *Handler) DeleteCustomer(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	tenantID := auth.GetTenantIDFromContext(r.Context())
	if tenantID == "" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "un contexte d'établissement est requis pour cette opération"})
		return
	}
	id := chi.URLParam(r, "id")

	if err := h.service.DeleteCustomer(r.Context(), tenantID, id); err != nil {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "client introuvable"})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "client supprimé avec succès"})
}

// customerErrorStatus mappe les erreurs de validation vers le bon code HTTP.
func customerErrorStatus(err error) int {
	switch {
	case errors.Is(err, ErrEmptyFirstName), errors.Is(err, ErrEmptyPhone):
		return http.StatusBadRequest
	case errors.Is(err, ErrPhoneExists):
		return http.StatusConflict
	default:
		return http.StatusBadRequest
	}
}
