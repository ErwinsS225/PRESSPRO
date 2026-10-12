package http

import (
	"encoding/json"
	"log"
	"net/http"

	"pressing-os/backend/internal/auth"
	"pressing-os/backend/internal/billing"
	"pressing-os/backend/internal/httperr"
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
			if err != nil {
				// Droits illisibles : refus technique (500), jamais un 402 trompeur.
				log.Printf("[http] droit %s illisible pour le tenant %s: %v", featureKey, tenantID, err)
				httperr.Error(w, http.StatusInternalServerError,
					"impossible de vérifier votre formule d'abonnement")
				return
			}
			if !ok {
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

// RequireFeatureQuota bloque la création d'une ressource quand le quota de la
// formule est épuisé (ex. grilles tarifaires : 1 en formule gratuite, 3 en PRO,
// 10 en BUSINESS). La consommation est calculée par le moteur de facturation sur
// les données réelles ; le superadmin reste dispensé (administration SaaS).
//
// Différence avec RequireFeature : ce garde-fou tient compte du PLAFOND
// (plan_entitlements.limit_value), pas seulement de l'activation du module.
func RequireFeatureQuota(svc *billing.Service, featureKey string) func(http.Handler) http.Handler {
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
			if tenantID == "" {
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "un contexte d'établissement est requis pour cette opération"})
				return
			}
			if svc == nil {
				next.ServeHTTP(w, r)
				return
			}

			current, err := svc.FeatureUsage(r.Context(), tenantID, featureKey)
			if err != nil {
				// La consommation n'a pas pu être établie : erreur technique
				// (détail journalisé côté serveur, jamais exposé au client).
				log.Printf("[http] quota %s: mesure impossible pour le tenant %s: %v", featureKey, tenantID, err)
				httperr.Error(w, http.StatusInternalServerError,
					"impossible de vérifier votre formule d'abonnement")
				return
			}

			if err := svc.CheckLimit(r.Context(), tenantID, featureKey, current); err != nil {
				if !billing.IsRefusMetier(err) {
					log.Printf("[http] quota %s: vérification impossible pour le tenant %s: %v", featureKey, tenantID, err)
					httperr.Error(w, http.StatusInternalServerError,
						"impossible de vérifier votre formule d'abonnement")
					return
				}
				// Refus métier du moteur de facturation (message compréhensible).
				w.WriteHeader(http.StatusPaymentRequired)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error":            err.Error(),
					"feature":          featureKey,
					"usage":            current,
					"upgrade_required": "true",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireActiveSubscription bloque les modules métier d'un établissement dont
// l'abonnement a été suspendu par l'éditeur (impayé, litige, fermeture).
//
// Les routes de compte (/tenant/current, /me/entitlements) et d'authentification
// restent ouvertes : le pressing doit pouvoir constater sa situation, et le
// superadmin conserve l'accès complet pour intervenir.
func RequireActiveSubscription(svc *billing.Service) func(http.Handler) http.Handler {
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
			if tenantID == "" || svc == nil {
				next.ServeHTTP(w, r)
				return
			}
			if svc.IsTenantSuspended(r.Context(), tenantID) {
				w.WriteHeader(http.StatusPaymentRequired)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error":               "abonnement suspendu — les opérations de cet établissement sont gelées jusqu'à régularisation auprès de l'éditeur",
					"subscription_status": billing.StatusSuspended,
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
