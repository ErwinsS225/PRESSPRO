package catalog

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"pressing-os/backend/internal/auth"
)

// Handler gère les requêtes HTTP sur le catalogue de prestations et tarifs
type Handler struct {
	service *Service
}

// NewHandler initialise le gestionnaire HTTP du catalogue
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ListServices gère GET /api/v1/services
func (h *Handler) ListServices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	tenantID := auth.GetTenantIDFromContext(r.Context())
	if tenantID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "contexte d'établissement manquant"})
		return
	}

	categoryFilter := r.URL.Query().Get("category")
	search := r.URL.Query().Get("q")

	items, err := h.service.GetServices(r.Context(), tenantID, categoryFilter, search)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(items)
}

// CreateService gère POST /api/v1/services
func (h *Handler) CreateService(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	tenantID := auth.GetTenantIDFromContext(r.Context())
	if tenantID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "authentification requise"})
		return
	}

	var req CreateServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "format JSON invalide"})
		return
	}

	created, err := h.service.CreateService(r.Context(), tenantID, req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

// GetService gère GET /api/v1/services/{id}
func (h *Handler) GetService(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	tenantID := auth.GetTenantIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	item, err := h.service.GetServiceByID(r.Context(), tenantID, id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "prestation introuvable"})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(item)
}

// UpdateService gère PUT /api/v1/services/{id}
func (h *Handler) UpdateService(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	tenantID := auth.GetTenantIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	var req UpdateServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "format JSON invalide"})
		return
	}

	updated, err := h.service.UpdateService(r.Context(), tenantID, id, req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

// DeleteService gère DELETE /api/v1/services/{id}
func (h *Handler) DeleteService(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	tenantID := auth.GetTenantIDFromContext(r.Context())
	id := chi.URLParam(r, "id")

	if err := h.service.DeleteService(r.Context(), tenantID, id); err != nil {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "prestation introuvable"})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "prestation supprimée avec succès"})
}
