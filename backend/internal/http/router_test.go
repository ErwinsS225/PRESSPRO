package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"pressing-os/backend/internal/auth"
	"pressing-os/backend/internal/config"
	"pressing-os/backend/internal/health"
	apphttp "pressing-os/backend/internal/http"
)

func TestRouterEndpoints(t *testing.T) {
	cfg := &config.Config{
		Port:           "8080",
		ServerEnv:      "test",
		AllowedOrigins: []string{"http://localhost:3000"},
	}
	healthH := health.NewHandler(nil, 1*time.Second)

	jwtManager := auth.NewJWTManager("test-secret-key-1234567890", 2*time.Hour)
	authService := auth.NewService(jwtManager)
	authH := auth.NewHandler(authService)

	router := apphttp.NewRouter(apphttp.ServerDependencies{
		Config:        cfg,
		HealthHandler: healthH,
		AuthHandler:   authH,
		JWTManager:    jwtManager,
	})

	// 1. Test GET /health
	reqHealth := httptest.NewRequest(http.MethodGet, "/health", nil)
	recHealth := httptest.NewRecorder()
	router.ServeHTTP(recHealth, reqHealth)
	if recHealth.Code != http.StatusOK {
		t.Errorf("attendu 200 sur /health, obtenu %d", recHealth.Code)
	}

	// 2. Test GET /ready (sans DB -> 503)
	reqReady := httptest.NewRequest(http.MethodGet, "/ready", nil)
	recReady := httptest.NewRecorder()
	router.ServeHTTP(recReady, reqReady)
	if recReady.Code != http.StatusServiceUnavailable {
		t.Errorf("attendu 503 sur /ready sans DB, obtenu %d", recReady.Code)
	}

	// 3. Test POST /api/v1/auth/login (Succès avec compte pré-configuré)
	loginBody, _ := json.Marshal(map[string]string{
		"email":    "gerant@pressing-ivoire.ci",
		"password": "pass123",
	})
	reqLogin := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(loginBody))
	reqLogin.Header.Set("Content-Type", "application/json")
	recLogin := httptest.NewRecorder()
	router.ServeHTTP(recLogin, reqLogin)

	if recLogin.Code != http.StatusOK {
		t.Fatalf("attendu 200 sur /api/v1/auth/login, obtenu %d", recLogin.Code)
	}

	var loginResp auth.LoginResponse
	if err := json.NewDecoder(recLogin.Body).Decode(&loginResp); err != nil {
		t.Fatalf("échec décodage réponse login: %v", err)
	}

	if loginResp.Token == "" {
		t.Fatal("le jeton JWT ne doit pas être vide")
	}
	if loginResp.User.Email != "gerant@pressing-ivoire.ci" {
		t.Errorf("attendu email gerant@pressing-ivoire.ci, obtenu %s", loginResp.User.Email)
	}

	// 4. Test GET /api/v1/auth/me avec le jeton reçu
	reqMe := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	reqMe.Header.Set("Authorization", "Bearer "+loginResp.Token)
	recMe := httptest.NewRecorder()
	router.ServeHTTP(recMe, reqMe)

	if recMe.Code != http.StatusOK {
		t.Fatalf("attendu 200 sur /api/v1/auth/me avec token, obtenu %d", recMe.Code)
	}

	// 5. Test GET /api/v1/tenant/current
	reqTenant := httptest.NewRequest(http.MethodGet, "/api/v1/tenant/current", nil)
	reqTenant.Header.Set("Authorization", "Bearer "+loginResp.Token)
	recTenant := httptest.NewRecorder()
	router.ServeHTTP(recTenant, reqTenant)

	if recTenant.Code != http.StatusOK {
		t.Fatalf("attendu 200 sur /api/v1/tenant/current, obtenu %d", recTenant.Code)
	}
}
