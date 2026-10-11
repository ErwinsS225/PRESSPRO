package notification_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"pressing-os/backend/internal/notification"
	"pressing-os/backend/internal/order"
)

// sourceFixe simule le module Commandes (une commande par identifiant).
type sourceFixe struct {
	commandes map[string]*order.Order
}

func (s sourceFixe) GetOrderByID(_ context.Context, tenantID, orderID string) (*order.Order, error) {
	ord, ok := s.commandes[orderID]
	if !ok || ord.TenantID != tenantID {
		return nil, errors.New("commande introuvable")
	}
	copie := *ord
	return &copie, nil
}

// droitsFixes simule le moteur d'abonnement.
type droitsFixes struct {
	actifs  map[string]bool
	enPanne bool
}

func (d droitsFixes) IsEnabled(_ context.Context, tenantID, featureKey string) (bool, error) {
	if d.enPanne {
		return false, errors.New("abonnement indisponible")
	}
	return d.actifs[tenantID+"|"+featureKey], nil
}

const tenantDemo = "a0000000-0000-0000-0000-000000000001"

func nouvellesCommandes() sourceFixe {
	return sourceFixe{commandes: map[string]*order.Order{
		"cmd-1": {
			ID: "cmd-1", TenantID: tenantDemo, OrderNumber: "CMD-2026-101",
			CustomerName: "Awa Koné", CustomerPhone: "+225 05 44 33 22 11",
			Status: "ready", TotalAmount: 8500, RemainingDue: 3500, DueDate: "Demain 17:00",
		},
		"cmd-sans-tel": {
			ID: "cmd-sans-tel", TenantID: tenantDemo, OrderNumber: "CMD-2026-102",
			CustomerName: "Client Au Comptoir", Status: "received", TotalAmount: 1000,
		},
		"cmd-2": {
			ID: "cmd-2", TenantID: tenantDemo, OrderNumber: "CMD-2026-103",
			CustomerName: "Salif Diomandé", CustomerPhone: "+225 05 12 34 56 78",
			Status: "delivered", TotalAmount: 4000,
		},
		"cmd-autre": {
			ID: "cmd-autre", TenantID: "b0000000-0000-0000-0000-000000000002",
			OrderNumber: "CMD-2026-900", CustomerName: "Client Voisin",
			CustomerPhone: "+225 07 00 00 00 00", Status: "ready", TotalAmount: 2000,
		},
	}}
}

func serviceTest() *notification.Service {
	s := notification.NewService()
	s.SetSourceCommandes(nouvellesCommandes())
	s.SetNomPressing(func(_ context.Context, _ string) string { return "Pressing Ivoire Cocody" })
	return s
}

// --- Composition des messages --------------------------------------------

func TestMontant(t *testing.T) {
	cas := map[float64]string{
		0: "0 FCFA", 500: "500 FCFA", 1500: "1 500 FCFA",
		12345: "12 345 FCFA", 1000000: "1 000 000 FCFA", -2500: "-2 500 FCFA",
	}
	for valeur, attendu := range cas {
		if got := notification.Montant(valeur); got != attendu {
			t.Errorf("Montant(%.0f) = %q, attendu %q", valeur, got, attendu)
		}
	}
	if strings.ContainsRune(notification.Montant(12345), '\u202f') {
		t.Error("le montant ne doit pas contenir d'espace fine insécable (SMS)")
	}
}

func TestCompose(t *testing.T) {
	donnees := notification.DonneesCommande{
		Pressing: "Pressing Ivoire Cocody", OrderNumber: "CMD-2026-101",
		CustomerName: "Awa Koné", TotalXOF: 8500, RemainingXOF: 3500, DueDate: "Demain 17:00",
	}

	prete := notification.Compose(notification.TemplateCommandePrete, donnees)
	for _, attendu := range []string{"Bonjour Awa", "commande n° CMD-2026-101", "Reste à payer : 3 500 FCFA", "Pressing Ivoire Cocody"} {
		if !strings.Contains(prete, attendu) {
			t.Errorf("modèle « prête » : %q attendu dans %q", attendu, prete)
		}
	}

	// Solde soldé : aucune mention de reste à payer.
	donneesSansReste := donnees
	donneesSansReste.RemainingXOF = 0
	if strings.Contains(notification.Compose(notification.TemplateCommandePrete, donneesSansReste), "Reste à payer") {
		t.Error("modèle « prête » : aucun reste dû ne doit être annoncé quand le solde est nul")
	}

	livree := notification.Compose(notification.TemplateCommandeLivree, donnees)
	if !strings.Contains(livree, "a bien été remise") {
		t.Errorf("modèle « livrée » inattendu : %q", livree)
	}

	paiement := notification.Compose(notification.TemplatePaiementRecu, donnees)
	if !strings.Contains(paiement, "reçu 5 000 FCFA") {
		t.Errorf("modèle « paiement » : montant encaissé (total − reste) attendu, obtenu %q", paiement)
	}

	rappel := notification.Compose(notification.TemplateRappelRetrait, donnees)
	if !strings.Contains(rappel, "Rappel") || !strings.Contains(rappel, "CMD-2026-101") {
		t.Errorf("modèle « rappel » inattendu : %q", rappel)
	}

	// Nom inconnu : message neutre, jamais vide.
	if got := notification.Compose("modele_mystere", donnees); !strings.Contains(got, "CMD-2026-101") {
		t.Errorf("modèle inconnu : repli attendu, obtenu %q", got)
	}
	// Nom du client absent : pas de « Bonjour , ».
	anonyme := notification.Compose(notification.TemplateRappelRetrait, notification.DonneesCommande{OrderNumber: "CMD-1"})
	if strings.Contains(anonyme, "Bonjour  ") || !strings.Contains(anonyme, "votre pressing") {
		t.Errorf("client/pressing inconnus : repli attendu, obtenu %q", anonyme)
	}
}

func TestLibelles(t *testing.T) {
	if notification.LibelleCanal(notification.ChannelWhatsApp) != "WhatsApp" {
		t.Error("libellé WhatsApp attendu")
	}
	if notification.LibelleStatut(notification.StatusSimule) == "" || notification.LibelleModele(notification.TemplateCommandePrete) != "Commande prête" {
		t.Error("libellés de statut/modèle attendus")
	}
	if !notification.CanalValide("sms") || notification.CanalValide("email") {
		t.Error("validation des canaux incorrecte")
	}
	if !notification.ModeleValide(notification.TemplatePaiementRecu) || notification.ModeleValide("") {
		t.Error("validation des modèles incorrecte")
	}
}

// --- Envoi ----------------------------------------------------------------

func TestEnvoyer_Validations(t *testing.T) {
	ctx := context.Background()
	s := serviceTest()

	cas := []struct {
		nom     string
		demande notification.DemandeEnvoi
		attendu error
	}{
		{"canal inconnu", notification.DemandeEnvoi{OrderID: "cmd-1", Channel: "email"}, notification.ErrCanalInvalide},
		{"modèle inconnu", notification.DemandeEnvoi{OrderID: "cmd-1", Channel: "sms", Template: "promo"}, notification.ErrModeleInvalide},
		{"commande inconnue", notification.DemandeEnvoi{OrderID: "cmd-absent", Channel: "sms"}, notification.ErrOrderNotFound},
		{"commande sans numéro", notification.DemandeEnvoi{OrderID: "cmd-sans-tel", Channel: "whatsapp"}, notification.ErrTelephoneManquant},
		{"commande d'un autre pressing", notification.DemandeEnvoi{OrderID: "cmd-autre", Channel: "sms"}, notification.ErrOrderNotFound},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			if _, err := s.Envoyer(ctx, tenantDemo, "usr-1", c.demande); !errors.Is(err, c.attendu) {
				t.Fatalf("attendu %v, obtenu %v", c.attendu, err)
			}
		})
	}
}

func TestEnvoyer_SansPasserelle_Journalise(t *testing.T) {
	ctx := context.Background()
	s := serviceTest()

	n, err := s.Envoyer(ctx, tenantDemo, "usr-gerant-001", notification.DemandeEnvoi{
		OrderID: "cmd-1", Channel: notification.ChannelWhatsApp, Template: notification.TemplateCommandePrete,
	})
	if err != nil {
		t.Fatalf("envoi: %v", err)
	}
	if n.Status != notification.StatusSimule || n.Provider != "journal" {
		t.Errorf("statut/provider attendus simule/journal, obtenus %s/%s", n.Status, n.Provider)
	}
	if n.Recipient != "+225 05 44 33 22 11" {
		t.Errorf("destinataire par défaut attendu (téléphone de la commande), obtenu %q", n.Recipient)
	}
	if !strings.Contains(n.Body, "Bonjour Awa") || !strings.Contains(n.Body, "3 500 FCFA") {
		t.Errorf("corps du message inattendu : %q", n.Body)
	}
	if n.SentAt != nil {
		t.Error("aucun envoi réel : sent_at doit rester vide")
	}
	if n.ID == "" {
		t.Error("identifiant de notification attendu")
	}

	// Message libre : il remplace le modèle.
	libre, err := s.Envoyer(ctx, tenantDemo, "usr-gerant-001", notification.DemandeEnvoi{
		OrderID: "cmd-1", Channel: notification.ChannelSMS, Message: "Votre linge est prêt, venez au comptoir.",
	})
	if err != nil || libre.Body != "Votre linge est prêt, venez au comptoir." {
		t.Fatalf("message libre non respecté: %v / %q", err, libre.Body)
	}

	// Isolation : le pressing voisin ne voit rien dans son journal.
	voisin, _ := s.Journal(ctx, "b0000000-0000-0000-0000-000000000002", notification.Filtre{})
	if len(voisin) != 0 {
		t.Errorf("journal du voisin attendu vide, obtenu %d entrées", len(voisin))
	}
	journal, _ := s.Journal(ctx, tenantDemo, notification.Filtre{})
	if len(journal) != 2 {
		t.Fatalf("journal du pressing : 2 entrées attendues, obtenu %d", len(journal))
	}
	// Le plus récent d'abord.
	if journal[0].Body != "Votre linge est prêt, venez au comptoir." {
		t.Errorf("tri du journal incorrect : %q", journal[0].Body)
	}
}

func TestEnvoyer_AvecPasserelle(t *testing.T) {
	ctx := context.Background()
	var recu map[string]interface{}
	var entete string
	serveur := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		entete = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&recu)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer serveur.Close()

	s := serviceTest()
	s.SetPasserelle(notification.NewHTTPPasserelle(serveur.URL, "jeton-passerelle", 2*time.Second))

	n, err := s.Envoyer(ctx, tenantDemo, "usr-1", notification.DemandeEnvoi{
		OrderID: "cmd-1", Channel: notification.ChannelWhatsApp, Template: notification.TemplateCommandeLivree,
	})
	if err != nil {
		t.Fatalf("envoi: %v", err)
	}
	if n.Status != notification.StatusEnvoye || n.Provider != "passerelle" {
		t.Errorf("statut/provider attendus envoye/passerelle, obtenus %s/%s", n.Status, n.Provider)
	}
	if n.SentAt == nil {
		t.Error("sent_at attendu après un envoi réel")
	}
	if recu["to"] != "+225 05 44 33 22 11" || recu["channel"] != "whatsapp" || recu["tenant_id"] != tenantDemo {
		t.Errorf("charge transmise à la passerelle inattendue : %v", recu)
	}
	if entete != "Bearer jeton-passerelle" {
		t.Errorf("jeton de passerelle attendu, obtenu %q", entete)
	}
	if corps, _ := recu["body"].(string); !strings.Contains(corps, "a bien été remise") {
		t.Errorf("corps transmis inattendu : %q", recu["body"])
	}
}

func TestEnvoyer_PasserelleEnEchec(t *testing.T) {
	ctx := context.Background()
	serveur := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "quota dépassé", http.StatusTooManyRequests)
	}))
	defer serveur.Close()

	s := serviceTest()
	s.SetPasserelle(notification.NewHTTPPasserelle(serveur.URL, "", time.Second))

	n, err := s.Envoyer(ctx, tenantDemo, "usr-1", notification.DemandeEnvoi{
		OrderID: "cmd-1", Channel: notification.ChannelSMS, Template: notification.TemplateRappelRetrait,
	})
	if err != nil {
		t.Fatalf("l'échec de passerelle doit être journalisé, pas remonté : %v", err)
	}
	if n.Status != notification.StatusEchec || !strings.Contains(n.Detail, "429") {
		t.Errorf("statut échec et détail technique attendus, obtenus %s / %q", n.Status, n.Detail)
	}
	if n.Body == "" {
		t.Error("le corps du message doit rester consultable après un échec")
	}

	// Passerelle injoignable : même traitement.
	s2 := serviceTest()
	s2.SetPasserelle(notification.NewHTTPPasserelle("http://127.0.0.1:1/injoignable", "", 200*time.Millisecond))
	n2, err := s2.Envoyer(ctx, tenantDemo, "usr-1", notification.DemandeEnvoi{OrderID: "cmd-1", Channel: notification.ChannelSMS})
	if err != nil || n2.Status != notification.StatusEchec || n2.Detail == "" {
		t.Errorf("passerelle injoignable : échec journalisé attendu, obtenu %v / %s / %q", err, n2.Status, n2.Detail)
	}
}

// --- Journal --------------------------------------------------------------

func TestJournal_Filtres(t *testing.T) {
	ctx := context.Background()
	s := serviceTest()

	_, _ = s.Envoyer(ctx, tenantDemo, "usr-1", notification.DemandeEnvoi{OrderID: "cmd-1", Channel: notification.ChannelSMS})
	_, _ = s.Envoyer(ctx, tenantDemo, "usr-1", notification.DemandeEnvoi{OrderID: "cmd-1", Channel: notification.ChannelWhatsApp})
	_, _ = s.Envoyer(ctx, tenantDemo, "usr-1", notification.DemandeEnvoi{OrderID: "cmd-2", Channel: notification.ChannelSMS})

	cas := []struct {
		nom     string
		filtre  notification.Filtre
		attendu int
	}{
		{"tous", notification.Filtre{}, 3},
		{"par commande", notification.Filtre{OrderID: "cmd-1"}, 2},
		{"par canal", notification.Filtre{Channel: notification.ChannelSMS}, 2},
		{"par statut", notification.Filtre{Status: notification.StatusSimule}, 3},
		{"limite", notification.Filtre{Limit: 1}, 1},
		{"combinaison", notification.Filtre{OrderID: "cmd-1", Channel: notification.ChannelWhatsApp}, 1},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			journal, err := s.Journal(ctx, tenantDemo, c.filtre)
			if err != nil {
				t.Fatalf("journal: %v", err)
			}
			if len(journal) != c.attendu {
				t.Errorf("%d entrées attendues, obtenu %d", c.attendu, len(journal))
			}
		})
	}
}

// --- Déclenchement automatique -------------------------------------------

func TestNotifierTransition(t *testing.T) {
	ctx := context.Background()
	droits := droitsFixes{actifs: map[string]bool{tenantDemo + "|notifications.advanced": true}}

	s := serviceTest()
	s.SetDroitVerificateur(droits, "notifications.advanced")

	s.NotifierTransition(ctx, notification.NotificationAuto{TenantID: tenantDemo, OrderID: "cmd-1", StatutLinge: "ready"})
	s.NotifierTransition(ctx, notification.NotificationAuto{TenantID: tenantDemo, OrderID: "cmd-1", StatutLinge: "delivered"})
	s.NotifierTransition(ctx, notification.NotificationAuto{TenantID: tenantDemo, OrderID: "cmd-1", StatutLinge: "washing"})                               // ignoré
	s.NotifierTransition(ctx, notification.NotificationAuto{TenantID: tenantDemo, OrderID: "cmd-sans-tel", StatutLinge: "ready"})                          // sans téléphone
	s.NotifierTransition(ctx, notification.NotificationAuto{TenantID: "b0000000-0000-0000-0000-000000000002", OrderID: "cmd-autre", StatutLinge: "ready"}) // formule gratuite

	journal, _ := s.Journal(ctx, tenantDemo, notification.Filtre{})
	if len(journal) != 2 {
		t.Fatalf("2 notifications automatiques attendues (prête + livrée), obtenu %d", len(journal))
	}
	modeles := map[string]bool{}
	for _, n := range journal {
		modeles[n.Template] = true
		if n.Status != notification.StatusSimule {
			t.Errorf("statut simule attendu sans passerelle, obtenu %s", n.Status)
		}
		if !strings.Contains(n.Body, "CMD-2026-101") {
			t.Errorf("corps automatique inattendu : %q", n.Body)
		}
	}
	if !modeles[notification.TemplateCommandePrete] || !modeles[notification.TemplateCommandeLivree] {
		t.Errorf("modèles attendus absents du journal : %v", modeles)
	}

	// Droits refusés : aucun bruit dans le journal d'un pressing gratuit.
	s2 := serviceTest()
	s2.SetDroitVerificateur(droitsFixes{actifs: map[string]bool{}}, "notifications.advanced")
	s2.NotifierTransition(ctx, notification.NotificationAuto{TenantID: tenantDemo, OrderID: "cmd-1", StatutLinge: "ready"})
	if journal2, _ := s2.Journal(ctx, tenantDemo, notification.Filtre{}); len(journal2) != 0 {
		t.Errorf("aucune notification attendue sans le droit « notifications.advanced », obtenu %d", len(journal2))
	}

	// Moteur d'abonnement en panne : on échoue fermé (pas d'envoi).
	s3 := serviceTest()
	s3.SetDroitVerificateur(droitsFixes{enPanne: true}, "notifications.advanced")
	s3.NotifierTransition(ctx, notification.NotificationAuto{TenantID: tenantDemo, OrderID: "cmd-1", StatutLinge: "ready"})
	if journal3, _ := s3.Journal(ctx, tenantDemo, notification.Filtre{}); len(journal3) != 0 {
		t.Errorf("abonnement indisponible : aucune notification attendue, obtenu %d", len(journal3))
	}
}

// TestNotifierTransition_Passerelle : le déclenchement automatique emprunte le
// même chemin d'envoi que le comptoir (statut envoye + sent_at).
func TestNotifierTransition_Passerelle(t *testing.T) {
	ctx := context.Background()
	appels := 0
	serveur := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appels++
		w.WriteHeader(http.StatusOK)
	}))
	defer serveur.Close()

	s := serviceTest()
	s.SetPasserelle(notification.NewHTTPPasserelle(serveur.URL, "", time.Second))
	s.SetDroitVerificateur(droitsFixes{actifs: map[string]bool{tenantDemo + "|notifications.advanced": true}}, "notifications.advanced")

	s.NotifierTransition(ctx, notification.NotificationAuto{TenantID: tenantDemo, OrderID: "cmd-1", StatutLinge: "ready"})

	journal, _ := s.Journal(ctx, tenantDemo, notification.Filtre{})
	if len(journal) != 1 || journal[0].Status != notification.StatusEnvoye || journal[0].SentAt == nil {
		t.Fatalf("envoi automatique attendu (envoye), obtenu %v", journal)
	}
	if appels != 1 {
		t.Errorf("1 appel de passerelle attendu, obtenu %d", appels)
	}
}

// TestNotificationJSON expose la charge utile telle que l'interface la consomme.
func TestNotificationJSON(t *testing.T) {
	n := notification.Notification{
		ID: "ntf-1", TenantID: tenantDemo, OrderID: "cmd-1", Channel: "whatsapp",
		Template: "commande_prete", Recipient: "+225 05 44 33 22 11", Body: "Bonjour Awa",
		Status: "simule", Provider: "journal", Attempts: 1, CreatedAt: time.Now().UTC(),
	}
	brut, err := json.Marshal(n)
	if err != nil {
		t.Fatalf("sérialisation: %v", err)
	}
	champs := string(brut)
	for _, attendu := range []string{`"order_id":"cmd-1"`, `"status":"simule"`, `"channel":"whatsapp"`, `"body":"Bonjour Awa"`} {
		if !strings.Contains(champs, attendu) {
			t.Errorf("champ %s attendu dans %s", attendu, fmt.Sprint(champs))
		}
	}
}
