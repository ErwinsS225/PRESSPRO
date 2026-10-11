package billing

import (
	"encoding/json"
	"net/http"

	"pressing-os/backend/internal/auth"
)

// Handler expose les endpoints d'abonnement.
type Handler struct {
	service *Service
}

// NewHandler initialise le gestionnaire HTTP du billing.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Me traite GET /api/me/entitlements (menus/boutons du frontend).
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
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
