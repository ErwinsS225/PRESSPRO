package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"pressing-os/backend/internal/database"
)

// Response structure pour les retours de santé
type Response struct {
	Status    string    `json:"status"`
	Service   string    `json:"service,omitempty"`
	Database  string    `json:"database,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Error     string    `json:"error,omitempty"`
}

// Handler gère les endpoints /health et /ready
type Handler struct {
	db        database.Pinger
	dbTimeout time.Duration
}

// NewHandler initialise le gestionnaire de santé
func NewHandler(db database.Pinger, dbTimeout time.Duration) *Handler {
	if dbTimeout <= 0 {
		dbTimeout = 3 * time.Second
	}
	return &Handler{
		db:        db,
		dbTimeout: dbTimeout,
	}
}

// HealthCheck gère GET /health (indique que le processus Go écoute et répond)
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	resp := Response{
		Status:    "ok",
		Service:   "pressingos-api",
		Timestamp: time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// ReadinessCheck gère GET /ready (vérifie la connectivité réelle à la base de données)
func (h *Handler) ReadinessCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if h.db == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(Response{
			Status:    "unavailable",
			Service:   "pressingos-api",
			Database:  "disconnected",
			Timestamp: time.Now().UTC(),
			Error:     "aucun pool de base de données configuré",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.dbTimeout)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(Response{
			Status:    "unavailable",
			Service:   "pressingos-api",
			Database:  "disconnected",
			Timestamp: time.Now().UTC(),
			Error:     "impossible de joindre postgresql",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(Response{
		Status:    "ready",
		Service:   "pressingos-api",
		Database:  "connected",
		Timestamp: time.Now().UTC(),
	})
}
