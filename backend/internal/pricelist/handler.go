package pricelist

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"pressing-os/backend/internal/auth"
	"pressing-os/backend/internal/httperr"
)

// Handler gère les requêtes HTTP sur les grilles tarifaires versionnées.
type Handler struct {
	service *Service
	logger  *log.Logger
}

// NewHandler initialise le gestionnaire HTTP des grilles tarifaires.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service, logger: log.Default()}
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (h *Handler) writeError(w http.ResponseWriter, status int, msg string) {
	h.writeJSON(w, status, map[string]string{"error": msg})
}

func (h *Handler) tenantFromContext(w http.ResponseWriter, r *http.Request) (string, bool) {
	tenantID := auth.GetTenantIDFromContext(r.Context())
	if tenantID == "" {
		h.writeError(w, http.StatusForbidden, "un contexte d'établissement est requis pour cette opération")
		return "", false
	}
	return tenantID, true
}

// ListPriceLists gère GET /api/v1/price-lists
func (h *Handler) ListPriceLists(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.tenantFromContext(w, r)
	if !ok {
		return
	}
	items, err := h.service.GetPriceLists(r.Context(), tenantID)
	if err != nil {
		httperr.Internal(w, h.logger, "pricelist", err, "impossible de traiter la grille tarifaire")
		return
	}
	if items == nil {
		items = []*PriceList{}
	}
	h.writeJSON(w, http.StatusOK, items)
}

// GetPriceList gère GET /api/v1/price-lists/{id} (grille + historique de versions)
func (h *Handler) GetPriceList(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.tenantFromContext(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	detail, err := h.service.GetPriceListWithVersions(r.Context(), tenantID, id)
	if err != nil {
		if err == ErrNotFound {
			h.writeError(w, http.StatusNotFound, "grille tarifaire introuvable")
			return
		}
		httperr.Internal(w, h.logger, "pricelist", err, "impossible de traiter la grille tarifaire")
		return
	}
	h.writeJSON(w, http.StatusOK, detail)
}

// CreatePriceList gère POST /api/v1/price-lists (mutations RBAC côté routeur)
func (h *Handler) CreatePriceList(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.tenantFromContext(w, r)
	if !ok {
		return
	}
	var req CreatePriceListRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "format JSON invalide")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		h.writeError(w, http.StatusBadRequest, "le nom de la grille est obligatoire")
		return
	}
	pl, err := h.service.CreatePriceList(r.Context(), tenantID, req)
	if err != nil {
		if errors.Is(err, ErrNameExists) {
			// Conflit métier : le nom de grille est déjà pris pour ce pressing.
			h.writeError(w, http.StatusConflict, ErrNameExists.Error())
			return
		}
		httperr.Internal(w, h.logger, "pricelist", err, "impossible de traiter la grille tarifaire")
		return
	}
	h.writeJSON(w, http.StatusCreated, pl)
}

// CreateVersion gère POST /api/v1/price-lists/{id}/versions (fige le snapshot)
func (h *Handler) CreateVersion(w http.ResponseWriter, r *http.Request) {
	tenantID, ok := h.tenantFromContext(w, r)
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	var req CreateVersionRequest
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.writeError(w, http.StatusBadRequest, "format JSON invalide")
			return
		}
	}
	v, err := h.service.CreateVersion(r.Context(), tenantID, id, req)
	if err != nil {
		if err == ErrNotFound {
			h.writeError(w, http.StatusNotFound, "grille tarifaire introuvable")
			return
		}
		httperr.Internal(w, h.logger, "pricelist", err, "impossible de traiter la grille tarifaire")
		return
	}
	h.writeJSON(w, http.StatusCreated, v)
}
