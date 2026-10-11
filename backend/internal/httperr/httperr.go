// Package httperr centralise l'écriture des réponses d'erreur JSON de l'API.
//
// Objectifs :
//   - homogénéiser le format {"error": "…"} sur tous les modules ;
//   - ne JAMAIS exposer une erreur technique (SQL, réseau, transaction) au client :
//     ces erreurs sont journalisées côté serveur et remplacées par un message
//     générique, afin d'éviter toute divulgation d'information d'infrastructure.
package httperr

import (
	"encoding/json"
	"log"
	"net/http"
)

// WriteJSON écrit une réponse JSON avec le code HTTP fourni.
func WriteJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// Error écrit {"error": message} avec le code HTTP fourni.
func Error(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, map[string]string{"error": message})
}

// ErrorWithExtra écrit {"error": message} enrichi de champs supplémentaires
// (ex. {"upgrade_required": "true"} du garde-fou de quota).
func ErrorWithExtra(w http.ResponseWriter, status int, message string, extra map[string]string) {
	payload := map[string]string{"error": message}
	for k, v := range extra {
		payload[k] = v
	}
	WriteJSON(w, status, payload)
}

// Internal journalise l'erreur technique côté serveur et renvoie au client un
// message générique (500) sans détail d'implémentation.
func Internal(w http.ResponseWriter, logger *log.Logger, module string, err error, publicMessage string) {
	if logger != nil {
		logger.Printf("[%s] erreur interne (réponse 500 générique): %v", module, err)
	}
	Error(w, http.StatusInternalServerError, publicMessage)
}
