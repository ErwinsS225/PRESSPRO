package auth

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"pressing-os/backend/internal/ratelimit"
)

type contextKey string

const (
	UserContextKey   contextKey = "user_claims"
	TenantContextKey contextKey = "tenant_id"
)

// Handler gère les requêtes HTTP d'authentification
type Handler struct {
	service      *Service
	logger       *log.Logger
	loginLimiter *ratelimit.Limiter // protection anti-force brute (optionnel)
	ipLimiter    *ratelimit.Limiter // plafond global par adresse IP (optionnel)
}

// SetLoginLimiters branche les limiteurs de débit appliqués à la connexion :
// un plafond par couple (IP, email) et un plafond global par adresse IP.
// Un limiteur nil (ou son absence) désactive la protection (mode dégradé/tests).
func (h *Handler) SetLoginLimiters(perEmail, perIP *ratelimit.Limiter) {
	h.loginLimiter = perEmail
	h.ipLimiter = perIP
}

// clientIP extrait l'adresse IP cliente (le middleware RealIP a déjà normalisé
// RemoteAddr depuis X-Forwarded-For / X-Real-IP).
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return host
}

// NewHandler initialise le gestionnaire HTTP
func NewHandler(service *Service) *Handler {
	return &Handler{service: service, logger: log.Default()}
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

	// Protection contre la force brute. Les tentatives sont comptées par couple
	// (IP, email) et par adresse IP. Une requête refusée par le limiteur n'est
	// rejetée que si les identifiants sont effectivement invalides : un
	// utilisateur légitime qui retrouve son mot de passe peut toujours entrer
	// (et son compteur est alors remis à zéro).
	ip := clientIP(r)
	emailKey := strings.ToLower(strings.TrimSpace(req.Email))
	ipKey := "login-ip:" + ip
	credKey := "login-ip-email:" + ip + ":" + emailKey

	blocked := false
	var retryAfter time.Duration
	if h.ipLimiter != nil {
		if ok, retry := h.ipLimiter.Allow(ipKey); !ok {
			blocked, retryAfter = true, retry
		}
	}
	if !blocked && h.loginLimiter != nil {
		if ok, retry := h.loginLimiter.Allow(credKey); !ok {
			blocked, retryAfter = true, retry
		}
	}

	resp, err := h.service.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		// Message volontairement identique pour email inconnu et mot de passe
		// erroné (pas d'énumération de comptes) ; les erreurs techniques restent
		// internes au serveur.
		if blocked {
			seconds := int(retryAfter.Seconds()) + 1
			w.Header().Set("Retry-After", strconv.Itoa(seconds))
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "trop de tentatives de connexion — réessayez dans " + strconv.Itoa(seconds) + " secondes",
			})
			return
		}
		if errors.Is(err, ErrAccountInactive) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": ErrAccountInactive.Error()})
			return
		}
		if !errors.Is(err, ErrInvalidCredentials) {
			h.logger.Printf("[auth] erreur interne au login (réponse 401 générique): %v", err)
		}
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": ErrInvalidCredentials.Error()})
		return
	}

	if h.loginLimiter != nil {
		h.loginLimiter.Reset(credKey)
	}
	if h.ipLimiter != nil {
		h.ipLimiter.Reset(ipKey)
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
		if errors.Is(err, ErrEmailAlreadyExists) {
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": ErrEmailAlreadyExists.Error()})
			return
		}
		h.logger.Printf("[auth] erreur interne à l'inscription (réponse 500 générique): %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "impossible de finaliser l'inscription de l'établissement"})
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(resp)
}

// ListUsers traite GET /api/v1/auth/users
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	claims := GetUserFromContext(r.Context())
	if claims == nil || claims.TenantID == "" {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "authentification de tenant requise"})
		return
	}

	users, err := h.service.GetUsersByTenant(r.Context(), claims.TenantID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "impossible de récupérer la liste des utilisateurs"})
		return
	}
	// Tableau vide plutôt que « null » : le client affiche une liste vide et
	// non une erreur lorsque le pressing n'a pas encore de compte employé.
	if users == nil {
		users = []*User{}
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(users)
}

// CreateUser traite POST /api/v1/users — création d'un compte employé.
// Le garde-fou de quota (users.max) est appliqué en amont par le routeur.
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	claims := GetUserFromContext(r.Context())
	if claims == nil || claims.TenantID == "" {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "authentification de tenant requise"})
		return
	}

	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "format de requête JSON invalide"})
		return
	}

	user, err := h.service.CreateUser(r.Context(), claims.TenantID, req)
	if err != nil {
		status := http.StatusBadRequest
		switch {
		case errors.Is(err, ErrEmailAlreadyExists):
			status = http.StatusConflict
		case errors.Is(err, ErrInvalidRole), errors.Is(err, ErrUserNotFound):
			status = http.StatusBadRequest
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(user)
}

// SetUserStatus traite PATCH /api/v1/users/{id}/status — désactivation ou
// réactivation d'un compte employé de l'établissement.
func (h *Handler) SetUserStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	claims := GetUserFromContext(r.Context())
	if claims == nil || claims.TenantID == "" {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "authentification de tenant requise"})
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "format de requête JSON invalide"})
		return
	}

	userID := chi.URLParam(r, "id")
	// Un utilisateur ne peut pas se désactiver lui-même : le pressing se
	// retrouverait sans personne pour administrer les comptes.
	if userID == claims.UserID {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "vous ne pouvez pas modifier le statut de votre propre compte"})
		return
	}

	user, err := h.service.SetUserStatus(r.Context(), claims.TenantID, userID, req.Status)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, ErrUserNotFound) {
			status = http.StatusNotFound
		}
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(user)
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

// WithTenantID force le tenant de la requête (utilisé par le middleware pour un
// superadmin qui cible explicitement un établissement via X-Tenant-ID).
// Sans cette injection, un superadmin (dont le jeton ne porte aucun tenant)
// accéderait aux routes multi-tenant avec un identifiant vide, provoquant des
// erreurs SQL (UUID "") au lieu d'une réponse métier claire.
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	if strings.TrimSpace(tenantID) == "" {
		return ctx
	}
	return context.WithValue(ctx, TenantContextKey, tenantID)
}

// WithUserClaims injecte les claims dans le contexte (utilitaire de test et middleware)
func WithUserClaims(ctx context.Context, claims *UserClaims) context.Context {
	ctx = context.WithValue(ctx, UserContextKey, claims)
	if claims != nil && claims.TenantID != "" {
		ctx = context.WithValue(ctx, TenantContextKey, claims.TenantID)
	}
	return ctx
}
