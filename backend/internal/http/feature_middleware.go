package http

import (
	"encoding/json"
	"net/http"

	"pressing-os/backend/internal/auth"
	"pressing-os/backend/internal/billing"
)

// RequireFeature bloque sans le droit effectif (niveau 1 : plan/essai).
// Le superadmin contourne le garde-fou commercial (administration SaaS).
func RequireFeature(svc *billing.Service, featureKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			claims := auth.GetUserFromContext(r.Context())
			if claims == nil {
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "authentification requise"})
				return
			}
			if claims.Role == auth.RoleSuperAdmin {
				next.ServeHTTP(w, r)
				return
			}
			tenantID := auth.GetTenantIDFromContext(r.Context())
			ok, err := svc.IsEnabled(r.Context(), tenantID, featureKey)
			if err != nil || !ok {
				w.WriteHeader(http.StatusPaymentRequired)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error":            "fonctionnalité non incluse dans votre formule",
					"feature":          featureKey,
					"upgrade_required": "true",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
