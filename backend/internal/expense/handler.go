package expense

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"pressing-os/backend/internal/auth"
)

// Handler expose les endpoints du module Dépenses (fonctionnalité payante
// « expenses.advanced »).
type Handler struct {
	service *Service
}

// NewHandler construit le handler des dépenses.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func ecrireJSON(w http.ResponseWriter, statut int, corps interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statut)
	_ = json.NewEncoder(w).Encode(corps)
}

func claimsDuPressing(w http.ResponseWriter, r *http.Request) *auth.UserClaims {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil || claims.TenantID == "" {
		ecrireJSON(w, http.StatusUnauthorized, map[string]string{"error": "authentification de tenant requise"})
		return nil
	}
	return claims
}

// ListExpenses traite GET /api/v1/expenses — dépenses de la période demandée
// (`from`, `to`, `category`, `branch_id`), avec total et répartition.
func (h *Handler) ListExpenses(w http.ResponseWriter, r *http.Request) {
	claims := claimsDuPressing(w, r)
	if claims == nil {
		return
	}

	q := r.URL.Query()
	f := Filter{
		From:     q.Get("from"),
		To:       q.Get("to"),
		Category: q.Get("category"),
		BranchID: q.Get("branch_id"),
	}
	if f.From == "" && f.To == "" {
		// Défaut : mois courant (l'écran Dépenses s'ouvre sur le mois en cours).
		now := timeNow()
		f.From = now[:8] + "01"
		f.To = now
	}

	depenses, err := h.service.GetExpenses(r.Context(), claims.TenantID, f)
	if err != nil {
		ecrireJSON(w, http.StatusInternalServerError, map[string]string{"error": "impossible de récupérer les dépenses"})
		return
	}
	if depenses == nil {
		depenses = []*Expense{}
	}
	ecrireJSON(w, http.StatusOK, map[string]interface{}{
		"expenses":    depenses,
		"count":       len(depenses),
		"total_xof":   Total(depenses),
		"by_category": ParCategorie(depenses),
		"period":      map[string]string{"from": f.From, "to": f.To},
	})
}

// CreateExpense traite POST /api/v1/expenses (module payant vérifié en amont).
func (h *Handler) CreateExpense(w http.ResponseWriter, r *http.Request) {
	claims := claimsDuPressing(w, r)
	if claims == nil {
		return
	}

	var req CreateExpenseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ecrireJSON(w, http.StatusBadRequest, map[string]string{"error": "format de requête JSON invalide"})
		return
	}
	if req.BranchID == "" {
		req.BranchID = r.Header.Get("X-Branch-ID")
	}

	e, err := h.service.CreateExpense(r.Context(), claims.TenantID, claims.UserID, req)
	if err != nil {
		ecrireJSON(w, statutErreurDepense(err), map[string]string{"error": err.Error()})
		return
	}
	ecrireJSON(w, http.StatusCreated, e)
}

// DeleteExpense traite DELETE /api/v1/expenses/{id}.
func (h *Handler) DeleteExpense(w http.ResponseWriter, r *http.Request) {
	claims := claimsDuPressing(w, r)
	if claims == nil {
		return
	}
	if err := h.service.DeleteExpense(r.Context(), claims.TenantID, chi.URLParam(r, "id")); err != nil {
		ecrireJSON(w, statutErreurDepense(err), map[string]string{"error": err.Error()})
		return
	}
	ecrireJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// statutErreurDepense traduit une erreur métier en code HTTP.
func statutErreurDepense(err error) int {
	if errors.Is(err, ErrExpenseNotFound) {
		return http.StatusNotFound
	}
	return http.StatusBadRequest
}
