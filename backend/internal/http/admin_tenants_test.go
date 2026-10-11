package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"pressing-os/backend/internal/auth"
	"pressing-os/backend/internal/billing"
	"pressing-os/backend/internal/config"
	"pressing-os/backend/internal/health"
	apphttp "pressing-os/backend/internal/http"
)

// TestAdminDeleteTenantRoute couvre DELETE /api/v1/admin/tenants/{id} :
// la route n'est ouverte qu'à l'éditeur (super-admin), refuse un établissement
// inconnu (404) et, en cas de succès, retire l'établissement de l'annuaire
// (204 sans corps).
func TestAdminDeleteTenantRoute(t *testing.T) {
	cfg := &config.Config{
		Port:           "8080",
		ServerEnv:      "test",
		AllowedOrigins: []string{"http://localhost:3000"},
	}
	jwtManager := auth.NewJWTManager("test-secret-key-1234567890", 2*time.Hour)
	authService := auth.NewService(jwtManager)

	billingSvc := billing.NewService()
	billingH := billing.NewHandler(billingSvc)

	router := apphttp.NewRouter(apphttp.ServerDependencies{
		Config:         cfg,
		HealthHandler:  health.NewHandler(nil, time.Second),
		AuthHandler:    auth.NewHandler(authService),
		JWTManager:     jwtManager,
		Billing:        billingSvc,
		BillingHandler: billingH,
	})

	connexion := func(email, motDePasse string) string {
		t.Helper()
		corps, _ := json.Marshal(map[string]string{"email": email, "password": motDePasse})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(corps))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("connexion %s : attendu 200, obtenu %d (%s)", email, rec.Code, rec.Body.String())
		}
		var reponse auth.LoginResponse
		if err := json.NewDecoder(rec.Body).Decode(&reponse); err != nil {
			t.Fatalf("réponse de connexion illisible : %v", err)
		}
		return reponse.Token
	}

	supprimer := func(jeton, tenantID string) int {
		t.Helper()
		req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/tenants/"+tenantID, nil)
		req.Header.Set("Authorization", "Bearer "+jeton)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}

	// Établissement « entreprise fictive » inscrit puis à supprimer.
	const condamne = "f1c71f00-0000-4000-8000-000000000001"
	const temoin = "f1c71f00-0000-4000-8000-000000000002"

	ownerToken := connexion("gerant@pressing-ivoire.ci", "pass123")
	superToken := connexion("admin@pressingos.ci", "admin123")

	// 1. Un gérant d'établissement n'a pas accès à l'espace éditeur.
	if code := supprimer(ownerToken, condamne); code != http.StatusForbidden {
		t.Errorf("gérant sur DELETE /admin/tenants/{id} : attendu 403, obtenu %d", code)
	}

	// 2. Sans jeton : refus d'authentification.
	if code := supprimer("", condamne); code != http.StatusUnauthorized {
		t.Errorf("requête anonyme : attendu 401, obtenu %d", code)
	}

	// 3. Établissement inconnu : 404, aucune suppression silencieuse.
	if code := supprimer(superToken, condamne); code != http.StatusNotFound {
		t.Errorf("établissement inconnu : attendu 404, obtenu %d", code)
	}

	ctx := context.Background()
	if err := billingSvc.EnsureTrial(ctx, condamne); err != nil {
		t.Fatalf("inscription de l'établissement de test : %v", err)
	}
	if err := billingSvc.EnsureTrial(ctx, temoin); err != nil {
		t.Fatalf("inscription du témoin : %v", err)
	}

	// 4. Suppression réelle par l'éditeur : 204 sans corps.
	reqSuppression := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/tenants/"+condamne, nil)
	reqSuppression.Header.Set("Authorization", "Bearer "+superToken)
	recSuppression := httptest.NewRecorder()
	router.ServeHTTP(recSuppression, reqSuppression)
	if recSuppression.Code != http.StatusNoContent {
		t.Fatalf("suppression : attendu 204, obtenu %d (%s)", recSuppression.Code, recSuppression.Body.String())
	}
	if recSuppression.Body.Len() != 0 {
		t.Errorf("réponse 204 attendue sans corps, obtenu %q", recSuppression.Body.String())
	}

	// 5. L'annuaire de l'éditeur ne contient plus que le témoin.
	reqAnnuaire := httptest.NewRequest(http.MethodGet, "/api/v1/admin/tenants", nil)
	reqAnnuaire.Header.Set("Authorization", "Bearer "+superToken)
	recAnnuaire := httptest.NewRecorder()
	router.ServeHTTP(recAnnuaire, reqAnnuaire)
	if recAnnuaire.Code != http.StatusOK {
		t.Fatalf("annuaire : attendu 200, obtenu %d", recAnnuaire.Code)
	}
	annuaire, err := billingSvc.TenantsOverview(ctx)
	if err != nil {
		t.Fatalf("annuaire : %v", err)
	}
	if len(annuaire) != 1 || annuaire[0].TenantID != temoin {
		t.Errorf("annuaire attendu : %s uniquement, obtenu %+v", temoin, annuaire)
	}

	// 6. Une nouvelle tentative sur l'établissement supprimé répond 404.
	if code := supprimer(superToken, condamne); code != http.StatusNotFound {
		t.Errorf("seconde suppression : attendu 404, obtenu %d", code)
	}
}
