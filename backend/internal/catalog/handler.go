package catalog

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"pressing-os/backend/internal/auth"
	"pressing-os/backend/internal/httperr"
)

// Handler gère les requêtes HTTP sur le catalogue de prestations et tarifs
type Handler struct {
	service *Service
	logger  *log.Logger
}

// NewHandler initialise le gestionnaire HTTP du catalogue
func NewHandler(service *Service) *Handler {
	return &Handler{service: service, logger: log.Default()}
}

// respondServiceError traduit une erreur du service catalogue en réponse HTTP :
//   - prestation inconnue (ou hors tenant) -> 404 « prestation introuvable » ;
//   - erreurs de validation métier         -> 400 avec le message fonctionnel ;
//   - erreur technique (SQL/IO)            -> 500 générique journalisé côté serveur.
func (h *Handler) respondServiceError(w http.ResponseWriter, err error, internalMsg string) {
	switch {
	case errors.Is(err, ErrServiceNotFound):
		httperr.Error(w, http.StatusNotFound, "prestation introuvable")
	case errors.Is(err, ErrEmptyName), errors.Is(err, ErrInvalidPrice):
		httperr.Error(w, http.StatusBadRequest, err.Error())
	default:
		httperr.Internal(w, h.logger, "catalog", err, internalMsg)
	}
}

// ListServices gère GET /api/v1/services
func (h *Handler) ListServices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	tenantID := auth.GetTenantIDFromContext(r.Context())
	if tenantID == "" {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "contexte d'établissement manquant"})
		return
	}

	categoryFilter := r.URL.Query().Get("category")
	search := r.URL.Query().Get("q")

	items, err := h.service.GetServices(r.Context(), tenantID, categoryFilter, search)
	if err != nil {
		h.respondServiceError(w, err, "impossible de récupérer le catalogue")
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
		w.WriteHeader(http.StatusForbidden)
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
		h.respondServiceError(w, err, "impossible d'enregistrer la prestation")
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

// GetService gère GET /api/v1/services/{id}
func (h *Handler) GetService(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	tenantID := auth.GetTenantIDFromContext(r.Context())
	if tenantID == "" {
		httperr.Error(w, http.StatusForbidden, "un contexte d'établissement est requis pour cette opération")
		return
	}
	id := chi.URLParam(r, "id")

	item, err := h.service.GetServiceByID(r.Context(), tenantID, id)
	if err != nil {
		h.respondServiceError(w, err, "impossible de récupérer la prestation")
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(item)
}

// UpdateService gère PUT /api/v1/services/{id}
func (h *Handler) UpdateService(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	tenantID := auth.GetTenantIDFromContext(r.Context())
	if tenantID == "" {
		httperr.Error(w, http.StatusForbidden, "un contexte d'établissement est requis pour cette opération")
		return
	}
	id := chi.URLParam(r, "id")

	var req UpdateServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "format JSON invalide"})
		return
	}

	updated, err := h.service.UpdateService(r.Context(), tenantID, id, req)
	if err != nil {
		h.respondServiceError(w, err, "impossible de mettre à jour la prestation")
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(updated)
}

// DeleteService gère DELETE /api/v1/services/{id}
func (h *Handler) DeleteService(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	tenantID := auth.GetTenantIDFromContext(r.Context())
	if tenantID == "" {
		httperr.Error(w, http.StatusForbidden, "un contexte d'établissement est requis pour cette opération")
		return
	}
	id := chi.URLParam(r, "id")

	if err := h.service.DeleteService(r.Context(), tenantID, id); err != nil {
		h.respondServiceError(w, err, "impossible de supprimer la prestation")
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": "prestation supprimée avec succès"})
}
