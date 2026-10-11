package auth

import (
	"context"
	"encoding/json"
	"net/http"
)

type contextKey string

const (
	UserContextKey   contextKey = "user_claims"
	TenantContextKey contextKey = "tenant_id"
)

// Handler gère les requêtes HTTP d'authentification
type Handler struct {
	service *Service
}

// NewHandler initialise le gestionnaire HTTP
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Login traite POST /api/v1/auth/login
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "format de requête JSON invalide"})
		return
	}

	if req.Email == "" || req.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "l'email et le mot de passe sont obligatoires"})
		return
	}

	resp, err := h.service.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// Me traite GET /api/v1/auth/me
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	claims := GetUserFromContext(r.Context())
	if claims == nil {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "utilisateur non authentifié"})
		return
	}

	user, tenant, err := h.service.GetUserByID(r.Context(), claims.UserID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "profil utilisateur introuvable"})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"user":   user,
		"tenant": tenant,
	})
}

// RegisterTenant traite POST /api/v1/auth/register-tenant
func (h *Handler) RegisterTenant(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	var req RegisterTenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "format JSON invalide"})
		return
	}

	if req.TenantName == "" || req.OwnerEmail == "" || req.Password == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "nom de l'établissement, email et mot de passe requis"})
		return
	}

	resp, err := h.service.RegisterTenant(r.Context(), req)
	if err != nil {
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// GetUserFromContext extrait les claims de l'utilisateur injectés par le middleware
func GetUserFromContext(ctx context.Context) *UserClaims {
	if val := ctx.Value(UserContextKey); val != nil {
		if claims, ok := val.(*UserClaims); ok {
			return claims
		}
	}
	return nil
}

// GetTenantIDFromContext extrait l'identifiant du tenant courant
func GetTenantIDFromContext(ctx context.Context) string {
	if val := ctx.Value(TenantContextKey); val != nil {
		if tid, ok := val.(string); ok {
			return tid
		}
	}
	return ""
}

// WithUserClaims injecte les claims dans le contexte (utilitaire de test et middleware)
func WithUserClaims(ctx context.Context, claims *UserClaims) context.Context {
	ctx = context.WithValue(ctx, UserContextKey, claims)
	if claims != nil && claims.TenantID != "" {
		ctx = context.WithValue(ctx, TenantContextKey, claims.TenantID)
	}
	return ctx
}
