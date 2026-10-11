package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"pressing-os/backend/internal/auth"
	"pressing-os/backend/internal/ratelimit"
)

// loginRequest envoie une requête de connexion au handler et retourne le code HTTP.
func loginRequest(t *testing.T, h *auth.Handler, email, password string) (int, map[string]string) {
	t.Helper()
	body := `{"email":"` + email + `","password":"` + password + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.Login(rec, req)

	var payload map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)
	return rec.Code, payload
}

// TestLoginRateLimiting vérifie la protection anti-force brute :
//   - 10 tentatives erronées sur un compte passent en 401, la 11e est refusée en 429 ;
//   - un autre compte n'est pas pénalisé ;
//   - un mot de passe CORRECT reste accepté malgré le plafond atteint, et la
//     connexion réussie remet le compteur à zéro (l'utilisateur légitime n'est
//     jamais enfermé dehors par ses propres fautes de frappe).
func TestLoginRateLimiting(t *testing.T) {
	jwtManager := auth.NewJWTManager("secret-key-test-rate-limit", time.Hour)
	service := auth.NewService(jwtManager)
	handler := auth.NewHandler(service)
	// Plafond par identifiants : 10 / 5 min. Plafond IP volontairement large.
	handler.SetLoginLimiters(ratelimit.New(10, 5*time.Minute), ratelimit.New(1000, 5*time.Minute))

	const compte = "gerant@pressing-ivoire.ci"
	for i := 1; i <= 10; i++ {
		code, _ := loginRequest(t, handler, compte, "mauvais-mot-de-passe")
		if code != http.StatusUnauthorized {
			t.Fatalf("tentative erronée %d: attendu 401, obtenu %d", i, code)
		}
	}

	code, payload := loginRequest(t, handler, compte, "mauvais-mot-de-passe")
	if code != http.StatusTooManyRequests {
		t.Fatalf("11e tentative erronée: attendu 429, obtenu %d", code)
	}
	if !strings.Contains(payload["error"], "trop de tentatives") {
		t.Errorf("message inattendu: %q", payload["error"])
	}

	// Un autre compte (autre clé) n'est pas affecté par le plafond.
	if code, _ := loginRequest(t, handler, "atelier@pressing-ivoire.ci", "mauvais"); code != http.StatusUnauthorized {
		t.Errorf("autre compte: attendu 401, obtenu %d", code)
	}

	// Mot de passe correct malgré le plafond : connexion autorisée + reset.
	code, _ = loginRequest(t, handler, compte, "pass123")
	if code != http.StatusOK {
		t.Fatalf("connexion légitime malgré le plafond: attendu 200, obtenu %d", code)
	}

	// Après le reset, une nouvelle faute de frappe repart de zéro (401, pas 429).
	if code, _ := loginRequest(t, handler, compte, "encore-faux"); code != http.StatusUnauthorized {
		t.Errorf("après reset: attendu 401, obtenu %d", code)
	}
}

// TestLoginRateLimiting_PerIP vérifie le plafond global par adresse IP.
func TestLoginRateLimiting_PerIP(t *testing.T) {
	jwtManager := auth.NewJWTManager("secret-key-test-rate-limit-ip", time.Hour)
	handler := auth.NewHandler(auth.NewService(jwtManager))
	handler.SetLoginLimiters(nil, ratelimit.New(3, time.Minute))

	for i := 1; i <= 3; i++ {
		if code, _ := loginRequest(t, handler, "inconnu"+string(rune('a'+i))+"@x.ci", "mauvais"); code != http.StatusUnauthorized {
			t.Fatalf("tentative IP %d: attendu 401, obtenu %d", i, code)
		}
	}
	// 4e tentative depuis la même IP : refusée même avec un compte différent.
	if code, _ := loginRequest(t, handler, "autre@x.ci", "mauvais"); code != http.StatusTooManyRequests {
		t.Errorf("plafond IP: attendu 429, obtenu %d", code)
	}
}
