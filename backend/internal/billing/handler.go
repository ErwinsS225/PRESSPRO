package billing

import (
	"encoding/json"
	"log"
	"net/http"

	"pressing-os/backend/internal/auth"
	"pressing-os/backend/internal/httperr"
)

// Handler expose les endpoints d'abonnement.
type Handler struct {
	service *Service
	logger  *log.Logger
}

// NewHandler initialise le gestionnaire HTTP du billing.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service, logger: log.Default()}
}

// Me traite GET /api/v1/me/entitlements (menus/boutons du frontend).
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	tenantID := auth.GetTenantIDFromContext(r.Context())
	if tenantID == "" {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "contexte d'établissement manquant"})
		return
	}
	// J7 paresseux : expirer l'essai dépassé sans purger les données.
	_, _ = h.service.ExpireTrialIfNeeded(r.Context(), tenantID, "")
	resp, err := h.service.EffectiveEntitlements(r.Context(), tenantID)
	if err != nil {
		httperr.Internal(w, h.logger, "billing", err, "impossible de calculer les droits de l'abonnement")
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// Catalogue traite GET /api/v1/billing/plans : formules, tarifs (base) et droits.
// Route authentifiée mais sans contrôle d'abonnement : la grille tarifaire doit
// rester lisible même lorsque l'abonnement est expiré.
func (h *Handler) Catalogue(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	plans, err := h.service.Catalogue(r.Context())
	if err != nil {
		httperr.Internal(w, h.logger, "billing", err, "impossible de lire la grille tarifaire")
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"plans": plans})
}
