package http

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"pressing-os/backend/internal/auth"
	"pressing-os/backend/internal/billing"
	"pressing-os/backend/internal/httperr"
)

// SuperAdminStats gère GET /api/v1/admin/stats : indicateurs consolidés de la
// plateforme (établissements, abonnements, MRR) calculés sur les données réelles.
func SuperAdminStats(svc *billing.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := svc.AdminStats(r.Context())
		if err != nil {
			httperr.Internal(w, log.Default(), "admin", err, "impossible de calculer les statistiques de la plateforme")
			return
		}
		httperr.WriteJSON(w, http.StatusOK, stats)
	}
}

// SuperAdminTenants gère GET /api/v1/admin/tenants : annuaire des pressings
// (abonnement et compte propriétaire compris, aucune donnée client/commande).
func SuperAdminTenants(svc *billing.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenants, err := svc.TenantsOverview(r.Context())
		if err != nil {
			httperr.Internal(w, log.Default(), "admin", err, "impossible de charger l'annuaire des établissements")
			return
		}
		httperr.WriteJSON(w, http.StatusOK, map[string]interface{}{
			"tenants": tenants,
			"total":   len(tenants),
		})
	}
}

// SuperAdminDeleteTenant gère DELETE /api/v1/admin/tenants/{id} : retrait
// définitif d'un établissement et de toutes ses données (comptes, clients,
// commandes, catalogue, dépenses, abonnements). Réservé à l'éditeur ; répond
// 204 sans corps en cas de succès et 404 si l'établissement n'existe plus.
func SuperAdminDeleteTenant(svc *billing.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := strings.TrimSpace(chi.URLParam(r, "id"))
		if tenantID == "" {
			httperr.Error(w, http.StatusBadRequest, "identifiant d'établissement manquant")
			return
		}

		if err := svc.DeleteTenant(r.Context(), tenantID); err != nil {
			if err == billing.ErrTenantNotFound {
				httperr.Error(w, http.StatusNotFound, "établissement introuvable")
				return
			}
			httperr.Internal(w, log.Default(), "admin", err, "impossible de supprimer l'établissement")
			return
		}

		log.Printf("[admin] Établissement supprimé avec ses données : %s", tenantID)
		w.WriteHeader(http.StatusNoContent)
	}
}

// SuperAdminSetTenantStatus gère PATCH /api/v1/admin/tenants/{id}/status :
// suspension ou réactivation de l'abonnement d'un établissement par l'éditeur.
// La suspension est effective : les modules métier du pressing sont bloqués (402).
func SuperAdminSetTenantStatus(svc *billing.Service) http.HandlerFunc {
	type payload struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "id")
		if strings.TrimSpace(tenantID) == "" {
			httperr.Error(w, http.StatusBadRequest, "identifiant d'établissement manquant")
			return
		}

		var req payload
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httperr.Error(w, http.StatusBadRequest, "corps de requête JSON invalide")
			return
		}

		actor := ""
		if claims := auth.GetUserFromContext(r.Context()); claims != nil {
			actor = claims.UserID
		}

		err := svc.SetTenantStatus(r.Context(), tenantID, req.Status, actor, req.Reason)
		if err != nil {
			switch {
			case err == billing.ErrTenantSubscriptionNotFound:
				httperr.Error(w, http.StatusNotFound, "aucun abonnement pour cet établissement")
			case strings.HasPrefix(err.Error(), "statut d'abonnement invalide"):
				httperr.Error(w, http.StatusBadRequest, err.Error())
			default:
				httperr.Internal(w, log.Default(), "admin", err, "impossible de modifier le statut de l'abonnement")
			}
			return
		}

		httperr.WriteJSON(w, http.StatusOK, map[string]interface{}{
			"tenant_id": tenantID,
			"status":    strings.ToUpper(req.Status),
		})
	}
}
