package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"pressing-os/backend/internal/auth"
)

// AuthenticatorMiddleware extrait et valide le jeton JWT
func AuthenticatorMiddleware(jwtManager *auth.JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error": "en-tête Authorization manquant (format: Bearer <token>)",
				})
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error": "format de jeton d'autorisation invalide (attendu: Bearer <token>)",
				})
				return
			}

			tokenString := parts[1]
			claims, err := jwtManager.Verify(tokenString)
			if err != nil {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error": err.Error(),
				})
				return
			}

			// PROTECTION CONTRE L'USURPATION MULTI-TENANT :
			// Si un client envoie un header 'X-Tenant-ID' distinct de son jeton vérifié,
			// et qu'il n'est pas superadmin, la requête est formellement bloquée en 403 Forbidden.
			customTenantHeader := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
			if customTenantHeader != "" && claims.Role != auth.RoleSuperAdmin {
				if customTenantHeader != claims.TenantID {
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.WriteHeader(http.StatusForbidden)
					_ = json.NewEncoder(w).Encode(map[string]string{
						"error": "violation d'isolation multi-tenant : accès refusé aux données d'un autre établissement",
					})
					return
				}
			}

			// Injection sécurisée dans le contexte de requête
			ctx := auth.WithUserClaims(r.Context(), claims)

			// Cas du superadmin (jeton sans établissement) : un X-Tenant-ID
			// explicite et bien formé devient le contexte de travail, ce qui
			// permet à l'éditeur SaaS d'administrer un pressing précis. Sans ce
			// header, l'établissement reste vide et les routes multi-tenant
			// répondent 403 « contexte d'établissement requis ».
			if claims.Role == auth.RoleSuperAdmin && customTenantHeader != "" && looksLikeUUIDForTenant(customTenantHeader) {
				ctx = auth.WithTenantID(ctx, customTenantHeader)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole vérifie que l'utilisateur possède l'un des rôles requis
func RequireRole(allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := auth.GetUserFromContext(r.Context())
			if claims == nil {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "authentification requise"})
				return
			}

			// Le superadmin a accès à tout
			if claims.Role == auth.RoleSuperAdmin {
				next.ServeHTTP(w, r)
				return
			}

			hasRole := false
			for _, role := range allowedRoles {
				if claims.Role == role {
					hasRole = true
					break
				}
			}

			if !hasRole {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error": "autorisations insuffisantes pour exécuter cette opération",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// looksLikeUUIDForTenant valide le format UUID d'un identifiant d'établissement
// fourni par un superadmin (évite de propager une valeur arbitraire vers SQL).
func looksLikeUUIDForTenant(v string) bool {
	if len(v) != 36 {
		return false
	}
	for i := 0; i < len(v); i++ {
		switch i {
		case 8, 13, 18, 23:
			if v[i] != '-' {
				return false
			}
		default:
			c := v[i]
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}
