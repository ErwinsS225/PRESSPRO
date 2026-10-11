// Package notification — notifications clients (WhatsApp / SMS).
//
// Fonctionnalité payante « notifications.advanced » : la composition et le
// journal des messages sont toujours disponibles, mais l'envoi réel via la
// passerelle configurée par l'éditeur est réservé aux formules PRO / BUSINESS
// (l'essai actif vaut BUSINESS). Sans passerelle, l'envoi est journalisé avec
// le statut « simule » : le message reste consultable et copiable au comptoir.
package notification

import (
	"fmt"
	"strings"
	"time"
)

// Canaux supportés.
const (
	ChannelWhatsApp = "whatsapp"
	ChannelSMS      = "sms"
)

// Statuts d'un envoi.
const (
	StatusSimule = "simule" // message composé et journalisé, aucun envoi réseau
	StatusEnvoye = "envoye" // passerelle sollicitée avec succès
	StatusEchec  = "echec"  // passerelle injoignable ou en erreur
)

// Modèles de message à la disposition du comptoir.
const (
	TemplateCommandePrete  = "commande_prete"
	TemplateCommandeLivree = "commande_livree"
	TemplatePaiementRecu   = "paiement_recu"
	TemplateRappelRetrait  = "rappel_retrait"
)

// Notification est un message adressé à un client, journalisé dans le pressing.
type Notification struct {
	ID        string     `json:"id"`
	TenantID  string     `json:"tenant_id"`
	OrderID   string     `json:"order_id,omitempty"`
	Channel   string     `json:"channel"`
	Template  string     `json:"template,omitempty"`
	Recipient string     `json:"recipient"`
	Body      string     `json:"body"`
	Status    string     `json:"status"`
	Provider  string     `json:"provider,omitempty"`
	Detail    string     `json:"detail,omitempty"`
	Attempts  int        `json:"attempts"`
	CreatedBy string     `json:"created_by,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	SentAt    *time.Time `json:"sent_at,omitempty"`
}

// DemandeEnvoi décrit le message demandé par le comptoir.
type DemandeEnvoi struct {
	OrderID   string `json:"order_id"`
	Channel   string `json:"channel"`
	Template  string `json:"template"`
	Message   string `json:"message,omitempty"`   // remplace le modèle si fourni
	Recipient string `json:"recipient,omitempty"` // par défaut : téléphone de la commande
	Pressing  string `json:"-"`                   // nom affiché dans le message (jamais accepté du client)
}

// DonneesCommande regroupe ce dont la composition du message a besoin.
type DonneesCommande struct {
	Pressing      string
	OrderNumber   string
	CustomerName  string
	CustomerPhone string
	Status        string
	TotalXOF      float64
	RemainingXOF  float64
	DueDate       string
}

// Filtre de lecture du journal.
type Filtre struct {
	OrderID string
	Channel string
	Status  string
	Limit   int
}

// Canaux liste les canaux acceptés (validation des requêtes).
func Canaux() []string { return []string{ChannelWhatsApp, ChannelSMS} }

// CanalValide indique si le canal est supporté.
func CanalValide(canal string) bool {
	for _, c := range Canaux() {
		if c == canal {
			return true
		}
	}
	return false
}

// Modeles liste les modèles proposés au comptoir.
func Modeles() []string {
	return []string{TemplateCommandePrete, TemplateCommandeLivree, TemplatePaiementRecu, TemplateRappelRetrait}
}

// ModeleValide indique si le modèle est connu.
func ModeleValide(modele string) bool {
	if modele == "" {
		return false
	}
	for _, m := range Modeles() {
		if m == modele {
			return true
		}
	}
	return false
}

// LibelleModele donne le libellé français d'un modèle (interface et exports).
func LibelleModele(modele string) string {
	switch modele {
	case TemplateCommandePrete:
		return "Commande prête"
	case TemplateCommandeLivree:
		return "Commande livrée"
	case TemplatePaiementRecu:
		return "Paiement reçu"
	case TemplateRappelRetrait:
		return "Rappel de retrait"
	default:
		return modele
	}
}

// LibelleCanal donne le libellé affichable d'un canal.
func LibelleCanal(canal string) string {
	switch canal {
	case ChannelWhatsApp:
		return "WhatsApp"
	case ChannelSMS:
		return "SMS"
	default:
		return canal
	}
}

// LibelleStatut donne le libellé français d'un statut d'envoi.
func LibelleStatut(statut string) string {
	switch statut {
	case StatusSimule:
		return "Simulé (aucune passerelle)"
	case StatusEnvoye:
		return "Envoyé"
	case StatusEchec:
		return "Échec"
	default:
		return statut
	}
}

// Montant formate un montant en FCFA à la française, sans espace insécable
// (les passerelles SMS n'acceptent pas toujours U+202F).
func Montant(valeur float64) string {
	entier := fmt.Sprintf("%.0f", valeur)
	negatif := strings.HasPrefix(entier, "-")
	if negatif {
		entier = strings.TrimPrefix(entier, "-")
	}
	var groupes []string
	for len(entier) > 3 {
		groupes = append([]string{entier[len(entier)-3:]}, groupes...)
		entier = entier[:len(entier)-3]
	}
	groupes = append([]string{entier}, groupes...)
	resultat := strings.Join(groupes, " ") + " FCFA"
	if negatif {
		return "-" + resultat
	}
	return resultat
}

// prenomClient renvoie le premier mot du nom du client (« Awa Koné » → « Awa »).
func prenomClient(nom string) string {
	champs := strings.Fields(strings.TrimSpace(nom))
	if len(champs) == 0 {
		return "cher client"
	}
	return champs[0]
}

// Compose construit le message envoyé au client. Les montants et le solde ne
// sont mentionnés que lorsque l'information est connue (paiement, reste dû).
func Compose(modele string, d DonneesCommande) string {
	pressing := strings.TrimSpace(d.Pressing)
	if pressing == "" {
		pressing = "votre pressing"
	}
	numero := strings.TrimSpace(d.OrderNumber)
	if numero == "" {
		numero = "en cours"
	}
	client := prenomClient(d.CustomerName)
	reste := ""
	if d.RemainingXOF > 0 {
		reste = " Reste à payer : " + Montant(d.RemainingXOF) + "."
	}

	switch modele {
	case TemplateCommandePrete:
		return fmt.Sprintf(
			"Bonjour %s, votre linge est prêt au comptoir (%s, commande n° %s).%s À très vite !",
			client, pressing, numero, reste)
	case TemplateCommandeLivree:
		return fmt.Sprintf(
			"Bonjour %s, votre commande n° %s a bien été remise. Merci de votre confiance — %s.",
			client, numero, pressing)
	case TemplatePaiementRecu:
		montant := d.TotalXOF - d.RemainingXOF
		if montant < 0 {
			montant = 0
		}
		return fmt.Sprintf(
			"Bonjour %s, nous avons bien reçu %s pour la commande n° %s.%s Merci — %s.",
			client, Montant(montant), numero, reste, pressing)
	case TemplateRappelRetrait:
		return fmt.Sprintf(
			"Rappel : votre linge (commande n° %s) vous attend au comptoir de %s.%s",
			numero, pressing, reste)
	default:
		return fmt.Sprintf("Bonjour %s, une information concerne votre commande n° %s (%s).",
			client, numero, pressing)
	}
}
