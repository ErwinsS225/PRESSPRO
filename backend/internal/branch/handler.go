package branch

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"pressing-os/backend/internal/auth"
)

// Handler expose les endpoints multi-sites (module payant branches.multi_site).
type Handler struct {
	service *Service
}

// NewHandler construit le handler des établissements.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func ecrireJSON(w http.ResponseWriter, statut int, corps interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statut)
	_ = json.NewEncoder(w).Encode(corps)
}

// ListBranches traite GET /api/v1/branches — établissements du pressing.
func (h *Handler) ListBranches(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil || claims.TenantID == "" {
		ecrireJSON(w, http.StatusForbidden, map[string]string{"error": "authentification de tenant requise"})
		return
	}
	branches, err := h.service.GetBranches(r.Context(), claims.TenantID)
	if err != nil {
		ecrireJSON(w, http.StatusInternalServerError, map[string]string{"error": "impossible de récupérer les établissements"})
		return
	}
	if branches == nil {
		branches = []*Branch{}
	}
	ecrireJSON(w, http.StatusOK, branches)
}

// CreateBranch traite POST /api/v1/branches. Le plafond de la formule
// (branches.multi_site : 1 en gratuit et PRO, 3 en BUSINESS) est appliqué par
// le middleware RequireFeatureQuota monté sur la route.
func (h *Handler) CreateBranch(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil || claims.TenantID == "" {
		ecrireJSON(w, http.StatusForbidden, map[string]string{"error": "authentification de tenant requise"})
		return
	}

	var req CreateBranchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ecrireJSON(w, http.StatusBadRequest, map[string]string{"error": "format de requête JSON invalide"})
		return
	}

	b, err := h.service.CreateBranch(r.Context(), claims.TenantID, req)
	if err != nil {
		ecrireJSON(w, statutErreurBranche(err), map[string]string{"error": err.Error()})
		return
	}
	ecrireJSON(w, http.StatusCreated, b)
}

// UpdateBranch traite PATCH /api/v1/branches/{id} — renommage, coordonnées et
// statut (suspension / archivage d'un site secondaire).
func (h *Handler) UpdateBranch(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil || claims.TenantID == "" {
		ecrireJSON(w, http.StatusForbidden, map[string]string{"error": "authentification de tenant requise"})
		return
	}

	var req UpdateBranchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ecrireJSON(w, http.StatusBadRequest, map[string]string{"error": "format de requête JSON invalide"})
		return
	}

	id := chi.URLParam(r, "id")
	b, err := h.service.UpdateBranch(r.Context(), claims.TenantID, id, req)
	if err != nil {
		ecrireJSON(w, statutErreurBranche(err), map[string]string{"error": err.Error()})
		return
	}
	ecrireJSON(w, http.StatusOK, b)
}

// ListMemberBranches traite GET /api/v1/users/{id}/branches — établissements du
// pressing et affectation du compte visé (écran Équipe).
func (h *Handler) ListMemberBranches(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil || claims.TenantID == "" {
		ecrireJSON(w, http.StatusForbidden, map[string]string{"error": "authentification de tenant requise"})
		return
	}
	membre := chi.URLParam(r, "id")
	sites, err := h.service.GetMemberBranches(r.Context(), claims.TenantID, membre)
	if err != nil {
		ecrireJSON(w, statutErreurAffectation(err), map[string]string{"error": err.Error()})
		return
	}
	ecrireJSON(w, http.StatusOK, map[string]interface{}{"user_id": membre, "branches": sites})
}

// UpdateMemberBranches traite PUT /api/v1/users/{id}/branches : remplace
// l'affectation du compte. Corps : {"branch_ids":["…","…"]}.
func (h *Handler) UpdateMemberBranches(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil || claims.TenantID == "" {
		ecrireJSON(w, http.StatusForbidden, map[string]string{"error": "authentification de tenant requise"})
		return
	}

	var req struct {
		BranchIDs []string `json:"branch_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ecrireJSON(w, http.StatusBadRequest, map[string]string{"error": "format de requête JSON invalide"})
		return
	}

	membre := chi.URLParam(r, "id")
	sites, err := h.service.SetMemberBranches(r.Context(), claims.TenantID, membre, req.BranchIDs)
	if err != nil {
		ecrireJSON(w, statutErreurAffectation(err), map[string]string{"error": err.Error()})
		return
	}
	ecrireJSON(w, http.StatusOK, map[string]interface{}{"user_id": membre, "branches": sites})
}

// statutErreurAffectation traduit une erreur d'affectation en code HTTP.
func statutErreurAffectation(err error) int {
	if errors.Is(err, ErrMemberNotFound) {
		return http.StatusNotFound
	}
	return http.StatusBadRequest
}

// statutErreurBranche traduit une erreur métier en code HTTP.
func statutErreurBranche(err error) int {
	switch {
	case errors.Is(err, ErrBranchNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrNameExists):
		return http.StatusConflict
	default:
		return http.StatusBadRequest
	}
}
