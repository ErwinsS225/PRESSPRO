package report

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"pressing-os/backend/internal/auth"
	"pressing-os/backend/internal/billing"
)

// Handler expose le module Rapports (fonctionnalité payante « reports.advanced »,
// consolidation multi-sites « reports.consolidated »).
type Handler struct {
	service *Service
	billing *billing.Service
}

// NewHandler construit le handler des rapports.
func NewHandler(service *Service, billingSvc *billing.Service) *Handler {
	return &Handler{service: service, billing: billingSvc}
}

// Summary traite GET /api/v1/reports/summary?from=AAAA-MM-JJ&to=AAAA-MM-JJ.
// Le droit « reports.advanced » est vérifié par le middleware RequireFeature ;
// la consolidation par établissement n'est ajoutée que si la formule inclut
// « reports.consolidated » (BUSINESS) — sinon le rapport reste complet, avec
// une note commerciale.
func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	claims := auth.GetUserFromContext(r.Context())
	if claims == nil || claims.TenantID == "" {
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "authentification de tenant requise"})
		return
	}

	q := r.URL.Query()
	consolide := h.consolidationActive(r.Context(), claims.TenantID)

	rapport, err := h.service.Summary(r.Context(), claims.TenantID, q.Get("from"), q.Get("to"), consolide)
	if err != nil {
		statut := http.StatusInternalServerError
		if errors.Is(err, ErrInvalidPeriod) {
			statut = http.StatusBadRequest
		}
		w.WriteHeader(statut)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(rapport)
}

// Export traite GET /api/v1/reports/export?format=csv|pdf&from=AAAA-MM-JJ&to=AAAA-MM-JJ.
// Le fichier est produit à partir du MÊME rapport que l'écran : les chiffres
// exportés ne peuvent pas diverger de ceux affichés.
func (h *Handler) Export(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil || claims.TenantID == "" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "authentification de tenant requise"})
		return
	}

	q := r.URL.Query()
	format := strings.ToLower(strings.TrimSpace(q.Get("format")))
	if format == "" {
		format = "csv"
	}
	if format != "csv" && format != "pdf" {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "format d'export invalide (csv ou pdf)"})
		return
	}

	consolide := h.consolidationActive(r.Context(), claims.TenantID)
	rapport, err := h.service.Summary(r.Context(), claims.TenantID, q.Get("from"), q.Get("to"), consolide)
	if err != nil {
		statut := http.StatusInternalServerError
		if errors.Is(err, ErrInvalidPeriod) {
			statut = http.StatusBadRequest
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(statut)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	nom := NomFichier(claims.TenantName, rapport.Period.From, rapport.Period.To, format)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Disposition", `attachment; filename="`+nom+`"`)

	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		if err := rapport.EcrireCSV(w, claims.TenantName); err != nil {
			// L'en-tête est déjà parti : on ne peut plus changer le statut.
			return
		}
		return
	}

	var document bytes.Buffer
	if err := rapport.EcrirePDF(&document, claims.TenantName); err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "impossible de générer le rapport PDF"})
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(document.Bytes())
}

// consolidationActive indique si la formule du pressing inclut la consolidation
// multi-sites (repli prudent : pas de consolidation si l'état est illisible).
func (h *Handler) consolidationActive(ctx context.Context, tenantID string) bool {
	if h.billing == nil {
		return false
	}
	ok, err := h.billing.IsEnabled(ctx, tenantID, billing.FeatureReportsConsol)
	return err == nil && ok
}
