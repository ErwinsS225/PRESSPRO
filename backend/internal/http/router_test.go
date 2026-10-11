package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"pressing-os/backend/internal/auth"
	"pressing-os/backend/internal/catalog"
	"pressing-os/backend/internal/config"
	"pressing-os/backend/internal/health"
	apphttp "pressing-os/backend/internal/http"
	"pressing-os/backend/internal/order"
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

	orderService := order.NewService()
	orderH := order.NewHandler(orderService)

	catalogService := catalog.NewService()
	catalogH := catalog.NewHandler(catalogService)

	router := apphttp.NewRouter(apphttp.ServerDependencies{
		Config:         cfg,
		HealthHandler:  healthH,
		AuthHandler:    authH,
		JWTManager:     jwtManager,
		OrderHandler:   orderH,
		CatalogHandler: catalogH,
	})

	// 1. Test GET /health
	reqHealth := httptest.NewRequest(http.MethodGet, "/health", nil)
	recHealth := httptest.NewRecorder()
	router.ServeHTTP(recHealth, reqHealth)
	if recHealth.Code != http.StatusOK {
		t.Errorf("attendu 200 sur /health, obtenu %d", recHealth.Code)
	}

	// 2. Connexion en tant que Propriétaire (Owner)
	loginBody, _ := json.Marshal(map[string]string{
		"email":    "gerant@pressing-ivoire.ci",
		"password": "pass123",
	})
	reqLogin := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(loginBody))
	reqLogin.Header.Set("Content-Type", "application/json")
	recLogin := httptest.NewRecorder()
	router.ServeHTTP(recLogin, reqLogin)

	var loginResp auth.LoginResponse
	_ = json.NewDecoder(recLogin.Body).Decode(&loginResp)
	ownerToken := loginResp.Token

	// 3. Test GET /api/v1/services (Lecture de la grille tarifaire)
	reqServices := httptest.NewRequest(http.MethodGet, "/api/v1/services", nil)
	reqServices.Header.Set("Authorization", "Bearer "+ownerToken)
	recServices := httptest.NewRecorder()
	router.ServeHTTP(recServices, reqServices)

	if recServices.Code != http.StatusOK {
		t.Fatalf("attendu 200 sur GET /api/v1/services, obtenu %d", recServices.Code)
	}

	// 4. Test POST /api/v1/services (Ajout d'une prestation par l'admin)
	newServiceBody, _ := json.Marshal(catalog.CreateServiceRequest{
		Name:           "Costume 3 pièces Deluxe",
		Category:       catalog.CategoryDryClean,
		UnitPrice:      5500,
		Unit:           "ensemble",
		EstimatedHours: 48,
	})
	reqCreateSrv := httptest.NewRequest(http.MethodPost, "/api/v1/services", bytes.NewBuffer(newServiceBody))
	reqCreateSrv.Header.Set("Authorization", "Bearer "+ownerToken)
	reqCreateSrv.Header.Set("Content-Type", "application/json")
	recCreateSrv := httptest.NewRecorder()
	router.ServeHTTP(recCreateSrv, reqCreateSrv)

	if recCreateSrv.Code != http.StatusCreated {
		t.Fatalf("attendu 201 sur POST /api/v1/services, obtenu %d", recCreateSrv.Code)
	}

	var createdSrv catalog.ServiceItem
	_ = json.NewDecoder(recCreateSrv.Body).Decode(&createdSrv)
	if createdSrv.UnitPrice != 5500 {
		t.Errorf("attendu prix 5500, obtenu %f", createdSrv.UnitPrice)
	}

	// 5. Test PUT /api/v1/services/{id} (Modification du tarif)
	updateSrvBody, _ := json.Marshal(catalog.UpdateServiceRequest{
		Name:           "Costume 3 pièces Deluxe (Ajusté)",
		Category:       catalog.CategoryDryClean,
		UnitPrice:      6000,
		Unit:           "ensemble",
		EstimatedHours: 48,
	})
	reqUpdateSrv := httptest.NewRequest(http.MethodPut, "/api/v1/services/"+createdSrv.ID, bytes.NewBuffer(updateSrvBody))
	reqUpdateSrv.Header.Set("Authorization", "Bearer "+ownerToken)
	reqUpdateSrv.Header.Set("Content-Type", "application/json")
	recUpdateSrv := httptest.NewRecorder()
	router.ServeHTTP(recUpdateSrv, reqUpdateSrv)

	if recUpdateSrv.Code != http.StatusOK {
		t.Fatalf("attendu 200 sur PUT /api/v1/services/{id}, obtenu %d", recUpdateSrv.Code)
	}

	// 6. Test DELETE /api/v1/services/{id} (Suppression de la prestation)
	reqDeleteSrv := httptest.NewRequest(http.MethodDelete, "/api/v1/services/"+createdSrv.ID, nil)
	reqDeleteSrv.Header.Set("Authorization", "Bearer "+ownerToken)
	recDeleteSrv := httptest.NewRecorder()
	router.ServeHTTP(recDeleteSrv, reqDeleteSrv)

	if recDeleteSrv.Code != http.StatusOK {
		t.Fatalf("attendu 200 sur DELETE /api/v1/services/{id}, obtenu %d", recDeleteSrv.Code)
	}
}
