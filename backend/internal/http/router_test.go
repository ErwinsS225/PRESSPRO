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

	// 7. Vérification publique d'un ticket (sans JWT) : aucun jeu de
	// démonstration n'étant embarqué, la commande est d'abord créée par
	// l'établissement, puis contrôlée par le client sans authentification.
	tenantID := "a0000000-0000-0000-0000-000000000001"
	newOrderBody, _ := json.Marshal(order.CreateOrderRequest{
		CustomerName:  "Client Vitrine",
		CustomerPhone: "+225 07 00 00 02",
		Items:         []order.CreateOrderItemInput{{ItemLabel: "Chemise", Quantity: 2, UnitPrice: 1000}},
	})
	reqCreateOrder := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewBuffer(newOrderBody))
	reqCreateOrder.Header.Set("Authorization", "Bearer "+ownerToken)
	reqCreateOrder.Header.Set("Content-Type", "application/json")
	recCreateOrder := httptest.NewRecorder()
	router.ServeHTTP(recCreateOrder, reqCreateOrder)
	if recCreateOrder.Code != http.StatusCreated {
		t.Fatalf("attendu 201 sur POST /api/v1/orders, obtenu %d (%s)", recCreateOrder.Code, recCreateOrder.Body.String())
	}
	var createdOrder order.Order
	if err := json.NewDecoder(recCreateOrder.Body).Decode(&createdOrder); err != nil {
		t.Fatalf("commande créée illisible: %v", err)
	}
	if createdOrder.ID == "" || createdOrder.OrderNumber == "" {
		t.Fatalf("commande créée incomplète: %+v", createdOrder)
	}

	reqVerify := httptest.NewRequest(http.MethodGet, "/api/v1/public/verify/"+tenantID+"/"+createdOrder.ID, nil)
	recVerify := httptest.NewRecorder()
	router.ServeHTTP(recVerify, reqVerify)

	if recVerify.Code != http.StatusOK {
		t.Fatalf("attendu 200 sur GET /api/v1/public/verify, obtenu %d", recVerify.Code)
	}
	var verifyResp order.VerifyTicketResponse
	if err := json.NewDecoder(recVerify.Body).Decode(&verifyResp); err != nil {
		t.Fatalf("réponse de vérification illisible: %v", err)
	}
	if !verifyResp.Valid || verifyResp.OrderID != createdOrder.ID {
		t.Errorf("vérification invalide: %+v", verifyResp)
	}

	// 8. Ticket inconnu -> 404 avec valid=false.
	reqVerify404 := httptest.NewRequest(http.MethodGet, "/api/v1/public/verify/"+tenantID+"/ord-inexistant", nil)
	recVerify404 := httptest.NewRecorder()
	router.ServeHTTP(recVerify404, reqVerify404)
	if recVerify404.Code != http.StatusNotFound {
		t.Fatalf("attendu 404 sur ticket inconnu, obtenu %d", recVerify404.Code)
	}
}

// fixedCounter simule un compteur de quotas (ex: 50 commandes déjà créées).
type fixedCounter struct {
	orders     int64
	users      int64
	priceLists int64
	branches   int64
}

func (f fixedCounter) CountMonthlyOrders(_ context.Context, _ string) (int64, error) {
	return f.orders, nil
}
func (f fixedCounter) CountUsers(_ context.Context, _ string) (int64, error) {
	return f.users, nil
}
func (f fixedCounter) CountPriceLists(_ context.Context, _ string) (int64, error) {
	return f.priceLists, nil
}
func (f fixedCounter) CountBranches(_ context.Context, _ string) (int64, error) {
	return f.branches, nil
}

func TestOrderQuotaFreeAfterTrial(t *testing.T) {
	cfg := &config.Config{
		Port:           "8080",
		ServerEnv:      "test",
		AllowedOrigins: []string{"http://localhost:3000"},
	}
	healthH := health.NewHandler(nil, 1*time.Second)
	jwtManager := auth.NewJWTManager("test-secret-key-1234567890", 2*time.Hour)
	authH := auth.NewHandler(auth.NewService(jwtManager))

	// L'établissement de test est inscrit (essai 7 jours) : plus aucune
	// entreprise fictive n'est pré-enregistrée dans le moteur de facturation.
	billingSvc := billing.NewService()
	if err := billingSvc.EnsureTrial(context.Background(), "a0000000-0000-0000-0000-000000000001"); err != nil {
		t.Fatalf("inscription de l'établissement de test: %v", err)
	}
	billingH := billing.NewHandler(billingSvc)
	orderSvc := order.NewService()
	orderH := order.NewHandler(orderSvc)

	router := apphttp.NewRouter(apphttp.ServerDependencies{
		Config:         cfg,
		HealthHandler:  healthH,
		AuthHandler:    authH,
		JWTManager:     jwtManager,
		OrderHandler:   orderH,
		Billing:        billingSvc,
		BillingHandler: billingH,
	})

	loginBody, _ := json.Marshal(map[string]string{
		"email":    "gerant@pressing-ivoire.ci",
		"password": "pass123",
	})
	doLogin := func() string {
		reqLogin := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBuffer(loginBody))
		reqLogin.Header.Set("Content-Type", "application/json")
		recLogin := httptest.NewRecorder()
		router.ServeHTTP(recLogin, reqLogin)
		var loginResp auth.LoginResponse
		_ = json.NewDecoder(recLogin.Body).Decode(&loginResp)
		return loginResp.Token
	}
	newOrderBody := func() *bytes.Buffer {
		b, _ := json.Marshal(order.CreateOrderRequest{
			CustomerName:  "Client Essai",
			CustomerPhone: "+225 07 00 00 01",
			Items:         []order.CreateOrderItemInput{{ItemLabel: "Chemise", Quantity: 1, UnitPrice: 1000}},
		})
		return bytes.NewBuffer(b)
	}

	// 1. Pendant l'essai (niveau BUSINESS) : création OK même avec 50 commandes.
	orderH.SetQuotaGuard(&billing.QuotaGuard{Billing: billingSvc, Counter: fixedCounter{orders: 50}})
	reqTrial := httptest.NewRequest(http.MethodPost, "/api/v1/orders", newOrderBody())
	reqTrial.Header.Set("Authorization", "Bearer "+doLogin())
	reqTrial.Header.Set("Content-Type", "application/json")
	recTrial := httptest.NewRecorder()
	router.ServeHTTP(recTrial, reqTrial)
	if recTrial.Code != http.StatusCreated {
		t.Fatalf("création pendant l'essai devrait passer (201), obtenu %d : %s", recTrial.Code, recTrial.Body.String())
	}

	// 2. Essai dépassé (FREE, 50 déjà créées) : la 51e est refusée en 402.
	billingSvc.ForcePlanForTest("a0000000-0000-0000-0000-000000000001", billing.PlanFree, billing.StatusActive)
	reqBlocked := httptest.NewRequest(http.MethodPost, "/api/v1/orders", newOrderBody())
	reqBlocked.Header.Set("Authorization", "Bearer "+doLogin())
	reqBlocked.Header.Set("Content-Type", "application/json")
	recBlocked := httptest.NewRecorder()
	router.ServeHTTP(recBlocked, reqBlocked)
	if recBlocked.Code != http.StatusPaymentRequired {
		t.Fatalf("51e commande FREE devrait être refusée (402), obtenu %d : %s", recBlocked.Code, recBlocked.Body.String())
	}

	// 3. FREE avec 49 commandes : la 50e passe.
	orderH.SetQuotaGuard(&billing.QuotaGuard{Billing: billingSvc, Counter: fixedCounter{orders: 49}})
	reqOK := httptest.NewRequest(http.MethodPost, "/api/v1/orders", newOrderBody())
	reqOK.Header.Set("Authorization", "Bearer "+doLogin())
	reqOK.Header.Set("Content-Type", "application/json")
	recOK := httptest.NewRecorder()
	router.ServeHTTP(recOK, reqOK)
	if recOK.Code != http.StatusCreated {
		t.Fatalf("50e commande FREE devrait passer (201), obtenu %d : %s", recOK.Code, recOK.Body.String())
	}
}
