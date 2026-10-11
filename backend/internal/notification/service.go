package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"pressing-os/backend/internal/database"
	"pressing-os/backend/internal/order"
)

// Erreurs métier (traduites en codes HTTP par le handler).
var (
	ErrOrderNotFound       = errors.New("commande introuvable")
	ErrCanalInvalide       = errors.New("canal inconnu (whatsapp ou sms)")
	ErrModeleInvalide      = errors.New("modèle de message inconnu")
	ErrTelephoneManquant   = errors.New("aucun numéro de téléphone pour ce client")
	ErrPasserelleAbsente   = errors.New("aucune passerelle d'envoi configurée")
	ErrNotificationAbsente = errors.New("notification introuvable")
)

// Store est le contrat de persistance du journal des notifications.
type Store interface {
	Create(ctx context.Context, n *Notification) error
	List(ctx context.Context, tenantID string, f Filtre) ([]*Notification, error)
}

// SourceCommandes fournit la commande à l'origine du message (le module
// Commandes est la seule source de vérité sur le linge et le solde dû).
type SourceCommandes interface {
	GetOrderByID(ctx context.Context, tenantID, orderID string) (*order.Order, error)
}

// NomPressing résout le nom commercial d'un établissement (affiché dans le
// message). Facultatif : sans résolveur, le message dit « votre pressing ».
type NomPressing func(ctx context.Context, tenantID string) string

// Passerelle transmet réellement le message (WhatsApp Business API, agrégateur
// SMS…). Une passerelle absente (nil) laisse le service en mode simulé.
type Passerelle interface {
	// Nom identifie la passerelle dans le journal (« passerelle », « whatsapp-cloud »…).
	Nom() string
	// Envoyer transmet le message ; l'erreur retournée est journalisée telle quelle.
	Envoyer(ctx context.Context, n *Notification) error
}

// DroitVerificateur indique si un établissement a accès à l'envoi réel
// (fonctionnalité payante). Permet de ne pas journaliser automatiquement les
// transitions des pressings en formule gratuite.
type DroitVerificateur interface {
	IsEnabled(ctx context.Context, tenantID, featureKey string) (bool, error)
}

// Service expose la logique métier des notifications clients.
type Service struct {
	store       Store
	commandes   SourceCommandes
	passerelle  Passerelle
	droits      DroitVerificateur
	feature     string
	nomPressing NomPressing

	mu      sync.RWMutex
	memoire map[string][]*Notification
	seq     int
}

// NewService construit le service en mode mémoire (mode démonstration).
func NewService() *Service {
	return &Service{memoire: make(map[string][]*Notification)}
}

// NewStore construit un service adossé à PostgreSQL.
func NewStore(db *database.Store) *Service {
	return &Service{store: &dbStore{database: db}, memoire: make(map[string][]*Notification)}
}

// SetStore branche la persistance sur un service existant.
func (s *Service) SetStore(store Store) { s.store = store }

// SetSourceCommandes branche la lecture des commandes (composition des messages).
func (s *Service) SetSourceCommandes(source SourceCommandes) { s.commandes = source }

// SetPasserelle branche la passerelle d'envoi réel (nil = mode simulé).
func (s *Service) SetPasserelle(p Passerelle) { s.passerelle = p }

// SetNomPressing branche la résolution du nom du pressing (messages client).
func (s *Service) SetNomPressing(fn NomPressing) { s.nomPressing = fn }

// NomPressingDuTenant retourne le nom résolu, ou une formule neutre.
func (s *Service) NomPressingDuTenant(ctx context.Context, tenantID string) string {
	if s.nomPressing == nil {
		return "votre pressing"
	}
	if nom := strings.TrimSpace(s.nomPressing(ctx, tenantID)); nom != "" {
		return nom
	}
	return "votre pressing"
}

// SetDroitVerificateur branche le moteur d'abonnement (déclenchements automatiques).
func (s *Service) SetDroitVerificateur(v DroitVerificateur, featureKey string) {
	s.droits = v
	s.feature = featureKey
}

// NewHTTPPasserelle construit une passerelle générique par webhook HTTP :
// POST {url} avec {to, channel, body, order_id, tenant_id} en JSON. Toute
// réponse hors 2xx est un échec journalisé. Un jeton facultatif est transmis
// en en-tête Authorization.
func NewHTTPPasserelle(url, token string, timeout time.Duration) Passerelle {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &httpPasserelle{url: strings.TrimSpace(url), token: strings.TrimSpace(token), client: &http.Client{Timeout: timeout}}
}

type httpPasserelle struct {
	url    string
	token  string
	client *http.Client
}

func (p *httpPasserelle) Nom() string { return "passerelle" }

func (p *httpPasserelle) Envoyer(ctx context.Context, n *Notification) error {
	if p.url == "" {
		return ErrPasserelleAbsente
	}
	charge := map[string]interface{}{
		"to":        n.Recipient,
		"channel":   n.Channel,
		"body":      n.Body,
		"order_id":  n.OrderID,
		"tenant_id": n.TenantID,
	}
	corps, err := json.Marshal(charge)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.url, bytes.NewReader(corps))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.token != "" {
		req.Header.Set("Authorization", "Bearer "+p.token)
	}
	rep, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer rep.Body.Close()
	brut, _ := io.ReadAll(io.LimitReader(rep.Body, 512))
	if rep.StatusCode < 200 || rep.StatusCode > 299 {
		return fmt.Errorf("passerelle HTTP %d: %s", rep.StatusCode, strings.TrimSpace(string(brut)))
	}
	return nil
}

// NotificationAuto décrit une transition de commande à notifier.
type NotificationAuto struct {
	TenantID    string
	Pressing    string
	OrderID     string
	StatutLinge string
}

// Journal liste les notifications d'un pressing (filtres facultatifs).
func (s *Service) Journal(ctx context.Context, tenantID string, f Filtre) ([]*Notification, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	if s.store != nil {
		return s.store.List(ctx, tenantID, f)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Notification
	for _, n := range s.memoire[tenantID] {
		if f.OrderID != "" && n.OrderID != f.OrderID {
			continue
		}
		if f.Channel != "" && n.Channel != f.Channel {
			continue
		}
		if f.Status != "" && n.Status != f.Status {
			continue
		}
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	if len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out, nil
}

// Envoyer compose le message demandé par le comptoir, le journalise puis tente
// l'envoi réel lorsque la passerelle est configurée.
func (s *Service) Envoyer(ctx context.Context, tenantID, userID string, d DemandeEnvoi) (*Notification, error) {
	d.Channel = strings.ToLower(strings.TrimSpace(d.Channel))
	d.Template = strings.ToLower(strings.TrimSpace(d.Template))
	d.Message = strings.TrimSpace(d.Message)
	d.Recipient = strings.TrimSpace(d.Recipient)
	d.OrderID = strings.TrimSpace(d.OrderID)

	if d.Channel == "" {
		d.Channel = ChannelWhatsApp
	}
	if !CanalValide(d.Channel) {
		return nil, ErrCanalInvalide
	}
	if d.OrderID == "" {
		return nil, ErrOrderNotFound
	}
	if d.Template != "" && !ModeleValide(d.Template) {
		return nil, ErrModeleInvalide
	}
	if s.commandes == nil {
		return nil, ErrOrderNotFound
	}
	ord, err := s.commandes.GetOrderByID(ctx, tenantID, d.OrderID)
	if err != nil || ord == nil {
		return nil, ErrOrderNotFound
	}

	recipient := d.Recipient
	if recipient == "" {
		recipient = ord.CustomerPhone
	}
	if strings.TrimSpace(recipient) == "" {
		return nil, ErrTelephoneManquant
	}

	modele := d.Template
	if modele == "" && d.Message == "" {
		modele = TemplateCommandePrete
	}
	pressing := strings.TrimSpace(d.Pressing)
	if pressing == "" {
		pressing = s.NomPressingDuTenant(ctx, tenantID)
	}
	corps := d.Message
	if corps == "" {
		corps = Compose(modele, DonneesCommande{
			Pressing:      pressing,
			OrderNumber:   ord.OrderNumber,
			CustomerName:  ord.CustomerName,
			CustomerPhone: ord.CustomerPhone,
			Status:        ord.Status,
			TotalXOF:      ord.TotalAmount,
			RemainingXOF:  ord.RemainingDue,
			DueDate:       ord.DueDate,
		})
	}

	n := &Notification{
		TenantID:  tenantID,
		OrderID:   ord.ID,
		Channel:   d.Channel,
		Template:  modele,
		Recipient: recipient,
		Body:      corps,
		Status:    StatusSimule,
		Provider:  "journal",
		Detail:    "aucune passerelle d'envoi configurée : message journalisé, à envoyer depuis le comptoir",
		Attempts:  1,
		CreatedBy: userID,
		CreatedAt: time.Now().UTC(),
	}
	if s.passerelle != nil && s.passerelle.Nom() != "" {
		if err := s.passerelle.Envoyer(ctx, n); err != nil {
			n.Status = StatusEchec
			n.Provider = s.passerelle.Nom()
			n.Detail = err.Error()
		} else {
			maintenant := time.Now().UTC()
			n.Status = StatusEnvoye
			n.Provider = s.passerelle.Nom()
			n.Detail = ""
			n.SentAt = &maintenant
		}
	}
	return s.enregistrer(ctx, n)
}

// NotifierTransition journalise (et envoie si possible) le message d'une
// transition de commande. Appelé en tâche de fond : jamais bloquant, jamais
// d'erreur remontée à l'appelant. Les pressings sans « notifications.advanced »
// ne sont pas notifiés automatiquement (aucun bruit dans leur journal).
func (s *Service) NotifierTransition(ctx context.Context, auto NotificationAuto) {
	modele := ""
	switch auto.StatutLinge {
	case "ready":
		modele = TemplateCommandePrete
	case "delivered":
		modele = TemplateCommandeLivree
	default:
		return
	}
	if s.droits != nil && s.feature != "" {
		actif, err := s.droits.IsEnabled(ctx, auto.TenantID, s.feature)
		if err != nil || !actif {
			return
		}
	}
	if s.commandes == nil {
		return
	}
	ord, err := s.commandes.GetOrderByID(ctx, auto.TenantID, auto.OrderID)
	if err != nil || ord == nil || strings.TrimSpace(ord.CustomerPhone) == "" {
		return
	}

	pressing := strings.TrimSpace(auto.Pressing)
	if pressing == "" {
		pressing = s.NomPressingDuTenant(ctx, auto.TenantID)
	}
	canal := ChannelWhatsApp
	n := &Notification{
		TenantID:  auto.TenantID,
		OrderID:   ord.ID,
		Channel:   canal,
		Template:  modele,
		Recipient: ord.CustomerPhone,
		Body: Compose(modele, DonneesCommande{
			Pressing:      pressing,
			OrderNumber:   ord.OrderNumber,
			CustomerName:  ord.CustomerName,
			CustomerPhone: ord.CustomerPhone,
			Status:        ord.Status,
			TotalXOF:      ord.TotalAmount,
			RemainingXOF:  ord.RemainingDue,
			DueDate:       ord.DueDate,
		}),
		Status:    StatusSimule,
		Provider:  "journal",
		Detail:    "notification automatique (aucune passerelle d'envoi configurée)",
		Attempts:  1,
		CreatedAt: time.Now().UTC(),
	}
	if s.passerelle != nil && s.passerelle.Nom() != "" {
		if err := s.passerelle.Envoyer(ctx, n); err != nil {
			n.Status = StatusEchec
			n.Provider = s.passerelle.Nom()
			n.Detail = err.Error()
		} else {
			maintenant := time.Now().UTC()
			n.Status = StatusEnvoye
			n.Provider = s.passerelle.Nom()
			n.Detail = ""
			n.SentAt = &maintenant
		}
	}
	_, _ = s.enregistrer(ctx, n)
}

// enregistrer persiste la notification (mémoire ou PostgreSQL).
func (s *Service) enregistrer(ctx context.Context, n *Notification) (*Notification, error) {
	if s.store != nil {
		if err := s.store.Create(ctx, n); err != nil {
			return nil, err
		}
		return n, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	n.ID = fmt.Sprintf("ntf-%s-%d", n.TenantID, s.seq)
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now().UTC()
	}
	s.memoire[n.TenantID] = append(s.memoire[n.TenantID], n)
	return n, nil
}
