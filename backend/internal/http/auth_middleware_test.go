package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"pressing-os/backend/internal/auth"
	apphttp "pressing-os/backend/internal/http"
)

func TestAuthMiddlewareAndTenantIsolation(t *testing.T) {
	jwtManager := auth.NewJWTManager("secret-key", 2*time.Hour)
	authMiddleware := apphttp.AuthenticatorMiddleware(jwtManager)

	userCocody := &auth.User{
		ID:       "usr-cocody-01",
		TenantID: "tenant-cocody",
		Email:    "gerant@cocody.ci",
		Role:     auth.RoleOwner,
	}
	validToken, _ := jwtManager.Generate(userCocody, "Pressing Cocody")

	// Handler protégé de test qui renvoie le tenant_id du contexte
	protectedHandler := authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := auth.GetUserFromContext(r.Context())
		tenantID := auth.GetTenantIDFromContext(r.Context())
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"user_id":   claims.UserID,
			"tenant_id": tenantID,
		})
	}))

	// 1. Requête sans en-tête Authorization -> 401
	reqNoAuth := httptest.NewRequest(http.MethodGet, "/test", nil)
	recNoAuth := httptest.NewRecorder()
	protectedHandler.ServeHTTP(recNoAuth, reqNoAuth)
	if recNoAuth.Code != http.StatusUnauthorized {
		t.Errorf("attendait 401 pour requête non authentifiée, obtenu %d", recNoAuth.Code)
	}

	// 2. Requête avec token valide -> 200 et tenant_id injecté
	reqValid := httptest.NewRequest(http.MethodGet, "/test", nil)
	reqValid.Header.Set("Authorization", "Bearer "+validToken)
	recValid := httptest.NewRecorder()
	protectedHandler.ServeHTTP(recValid, reqValid)
	if recValid.Code != http.StatusOK {
		t.Errorf("attendait 200 pour token valide, obtenu %d", recValid.Code)
	}

	var data map[string]string
	_ = json.NewDecoder(recValid.Body).Decode(&data)
	if data["tenant_id"] != "tenant-cocody" {
		t.Errorf("attendu tenant_id 'tenant-cocody', obtenu '%s'", data["tenant_id"])
	}

	// 3. TENTATIVE D'USURPATION MULTI-TENANT :
	// Le client envoie un token pour 'tenant-cocody' mais ajoute un en-tête 'X-Tenant-ID: tenant-plateau'
	// Le middleware doit impérativement intercepter et renvoyer 403 Forbidden !
	reqSpoof := httptest.NewRequest(http.MethodGet, "/test", nil)
	reqSpoof.Header.Set("Authorization", "Bearer "+validToken)
	reqSpoof.Header.Set("X-Tenant-ID", "tenant-plateau-autre-pressing")
	recSpoof := httptest.NewRecorder()
	protectedHandler.ServeHTTP(recSpoof, reqSpoof)

	if recSpoof.Code != http.StatusForbidden {
		t.Errorf("attendait 403 Forbidden lors d'une tentative de spoof de tenant, obtenu %d", recSpoof.Code)
	}
}

func TestRequireRole(t *testing.T) {
	adminOnlyHandler := apphttp.RequireRole(auth.RoleSuperAdmin)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Utilisateur avec rôle 'cashier'
	claimsCashier := &auth.UserClaims{UserID: "u1", Role: auth.RoleCashier}
	ctxCashier := auth.WithUserClaims(httptest.NewRequest(http.MethodGet, "/", nil).Context(), claimsCashier)
	reqCashier := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctxCashier)
	recCashier := httptest.NewRecorder()

	adminOnlyHandler.ServeHTTP(recCashier, reqCashier)
	if recCashier.Code != http.StatusForbidden {
		t.Errorf("attendait 403 pour rôle caissier sur route admin, obtenu %d", recCashier.Code)
	}

	// Utilisateur avec rôle 'superadmin'
	claimsAdmin := &auth.UserClaims{UserID: "u0", Role: auth.RoleSuperAdmin}
	ctxAdmin := auth.WithUserClaims(httptest.NewRequest(http.MethodGet, "/", nil).Context(), claimsAdmin)
	reqAdmin := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctxAdmin)
	recAdmin := httptest.NewRecorder()

	adminOnlyHandler.ServeHTTP(recAdmin, reqAdmin)
	if recAdmin.Code != http.StatusOK {
		t.Errorf("attendait 200 pour superadmin, obtenu %d", recAdmin.Code)
	}
}
