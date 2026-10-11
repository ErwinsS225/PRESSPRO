package health_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"pressing-os/backend/internal/health"
)

type mockPinger struct {
	err error
}

func (m *mockPinger) Ping(ctx context.Context) error {
	return m.err
}

func (m *mockPinger) Close() {}

func TestHealthCheck(t *testing.T) {
	h := health.NewHandler(nil, 1*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.HealthCheck(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("attendu statut 200, obtenu %d", rec.Code)
	}

	var resp health.Response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("impossible de décoder le JSON: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("attendu status 'ok', obtenu '%s'", resp.Status)
	}
	if resp.Service != "pressingos-api" {
		t.Errorf("attendu service 'pressingos-api', obtenu '%s'", resp.Service)
	}
}

func TestReadinessCheck_Success(t *testing.T) {
	mockDB := &mockPinger{err: nil}
	h := health.NewHandler(mockDB, 1*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	h.ReadinessCheck(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("attendu statut 200, obtenu %d", rec.Code)
	}

	var resp health.Response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("impossible de décoder le JSON: %v", err)
	}

	if resp.Status != "ready" || resp.Database != "connected" {
		t.Errorf("attendu status 'ready' et database 'connected', obtenu %+v", resp)
	}
}

func TestReadinessCheck_Failure(t *testing.T) {
	mockDB := &mockPinger{err: errors.New("connexion refusée")}
	h := health.NewHandler(mockDB, 1*time.Second)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	h.ReadinessCheck(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("attendu statut 503, obtenu %d", rec.Code)
	}

	var resp health.Response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("impossible de décoder le JSON: %v", err)
	}

	if resp.Status != "unavailable" || resp.Database != "disconnected" {
		t.Errorf("attendu status 'unavailable' et database 'disconnected', obtenu %+v", resp)
	}
}
