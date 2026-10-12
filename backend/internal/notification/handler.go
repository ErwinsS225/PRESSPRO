package notification

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"pressing-os/backend/internal/auth"
)

// Handler expose les endpoints du module Notifications clients.
type Handler struct {
	service *Service
}

// NewHandler construit le handler des notifications.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func ecrireJSON(w http.ResponseWriter, statut int, corps interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statut)
	_ = json.NewEncoder(w).Encode(corps)
}

// ListNotifications traite GET /api/v1/notifications — journal du pressing
// (filtres `order_id`, `channel`, `status`, `limit`). La lecture reste ouverte
// même en formule gratuite : l'historique appartient au pressing.
func (h *Handler) ListNotifications(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil || claims.TenantID == "" {
		ecrireJSON(w, http.StatusForbidden, map[string]string{"error": "authentification de tenant requise"})
		return
	}

	q := r.URL.Query()
	f := Filtre{
		OrderID: strings.TrimSpace(q.Get("order_id")),
		Channel: strings.ToLower(strings.TrimSpace(q.Get("channel"))),
		Status:  strings.ToLower(strings.TrimSpace(q.Get("status"))),
	}
	if v := strings.TrimSpace(q.Get("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.Limit = n
		}
	}

	journal, err := h.service.Journal(r.Context(), claims.TenantID, f)
	if err != nil {
		ecrireJSON(w, http.StatusInternalServerError, map[string]string{"error": "impossible de lire le journal des notifications"})
		return
	}
	if journal == nil {
		journal = []*Notification{}
	}
	ecrireJSON(w, http.StatusOK, map[string]interface{}{
		"notifications": journal,
		"count":         len(journal),
		"channels":      Canaux(),
		"templates":     Modeles(),
	})
}

// CreateNotification traite POST /api/v1/notifications — composition et envoi
// d'un message client (module payant « notifications.advanced », vérifié en
// amont par le routeur). Corps : {order_id, channel, template, message?, recipient?}.
func (h *Handler) CreateNotification(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetUserFromContext(r.Context())
	if claims == nil || claims.TenantID == "" {
		ecrireJSON(w, http.StatusForbidden, map[string]string{"error": "authentification de tenant requise"})
		return
	}

	var d DemandeEnvoi
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		ecrireJSON(w, http.StatusBadRequest, map[string]string{"error": "format de requête JSON invalide"})
		return
	}
	// Le nom du pressing n'est jamais accepté du client : il vient du jeton.
	d.Pressing = claims.TenantName

	n, err := h.service.Envoyer(r.Context(), claims.TenantID, claims.UserID, d)
	if err != nil {
		statut := http.StatusBadRequest
		if errors.Is(err, ErrOrderNotFound) {
			statut = http.StatusNotFound
		}
		ecrireJSON(w, statut, map[string]string{"error": err.Error()})
		return
	}

	code := http.StatusCreated
	message := "message journalisé (aucune passerelle configurée)"
	if n.Status == StatusEnvoye {
		message = "message transmis à la passerelle"
	} else if n.Status == StatusEchec {
		message = "échec de l'envoi — voir le détail"
	}
	ecrireJSON(w, code, map[string]interface{}{
		"notification": n,
		"status":       n.Status,
		"message":      message,
	})
}
