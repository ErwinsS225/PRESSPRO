package order

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"pressing-os/backend/internal/billing"
	"pressing-os/backend/internal/customer"
)

// Service gère les opérations métier sur les commandes de linge
// Garantit l'isolation multi-tenant des données et la sécurité d'accès concurrent
type Service struct {
	mu           sync.RWMutex
	creation     sync.Mutex        // sérialise les créations en mémoire (contrôle du plafond + insertion)
	orders       map[string]*Order // clé: order_id
	tenantSeqNum map[string]int    // clé: tenant_id -> séquenceur de ticket
	mutations    map[string]memoryMutation
	store        *dbStore // nil = mode mémoire ; non-nil = persistance PostgreSQL
	logger       *log.Logger

	// hookStatut est appelé en tâche de fond après une transition de statut de
	// linge réussie (notifications client, intégrations). Facultatif, et
	// toujours hors du chemin critique : une panne du hook n'empêche jamais la
	// réponse HTTP.
	hookStatut func(ctx context.Context, tenantID, orderID, statut string)

	// quota applique le plafond de commandes du plan. Il est contrôlé par le
	// service (et non par le handler HTTP) pour que tout chemin de création soit
	// couvert. Facultatif : nil = aucun plafond (tests, outils internes).
	quota *billing.QuotaGuard
}

// SetQuotaGuard branche le garde-fou du plafond de commandes (au démarrage).
func (s *Service) SetQuotaGuard(guard *billing.QuotaGuard) {
	s.quota = guard
}

// SetHookStatut branche le récepteur des transitions de statut (facultatif).
func (s *Service) SetHookStatut(hook func(ctx context.Context, tenantID, orderID, statut string)) {
	s.hookStatut = hook
}

// signalerStatut déclenche le hook hors du chemin critique (goroutine isolée,
// panne avalée, délai borné).
func (s *Service) signalerStatut(ctx context.Context, tenantID, orderID, statut string) {
	if s.hookStatut == nil {
		return
	}
	_ = ctx
	go func() {
		defer func() {
			if r := recover(); r != nil && s.logger != nil {
				s.logger.Printf("[order] hook statut interrompu: %v", r)
			}
		}()
		limite, annuler := context.WithTimeout(context.Background(), 10*time.Second)
		defer annuler()
		s.hookStatut(limite, tenantID, orderID, statut)
	}()
}

// validateTenantExistence vérifie que le tenant existe dans la map des séquences (mode mémoire uniquement)
func (s *Service) validateTenantExistence(tenantID string) {
	s.mu.Lock()
	if _, exists := s.tenantSeqNum[tenantID]; !exists {
		s.tenantSeqNum[tenantID] = 0
		s.logger.Printf("[order] Nouveau tenant initialisé: %s", tenantID)
	}
	s.mu.Unlock()
}

// createOrderItems génère les éléments de commande et calcule le montant total
func createOrderItems(orderID string, items []CreateOrderItemInput) ([]OrderItem, float64) {
	var orderItems []OrderItem
	var totalAmount float64

	for idx, it := range items {
		qty := it.Quantity
		if qty <= 0 {
			qty = 1
		}
		subtotal := float64(qty) * it.UnitPrice
		totalAmount += subtotal

		orderItems = append(orderItems, OrderItem{
			ID:        fmt.Sprintf("it-%s-%d", orderID, idx+1),
			ServiceID: it.ServiceID,
			ItemLabel: it.ItemLabel,
			Quantity:  qty,
			UnitPrice: it.UnitPrice,
			Subtotal:  subtotal,
			Notes:     it.Notes,
		})
	}
	return orderItems, totalAmount
}

// calculatePaymentStatus détermine le statut de paiement en fonction des montants
func calculatePaymentStatus(paidAmount, totalAmount float64) string {
	if totalAmount <= 0 {
		return PaymentStatusPaid
	}
	if paidAmount >= totalAmount {
		return PaymentStatusPaid
	}
	if paidAmount > 0 {
		return PaymentStatusPartial
	}
	return PaymentStatusUnpaid
}

// getSafeOrder vérifie l'existence d'une commande et son appartenance au tenant (mode mémoire)
func (s *Service) getSafeOrder(tenantID, orderID string) (*Order, error) {
	ord, exists := s.orders[orderID]
	if !exists || ord.TenantID != tenantID {
		s.logger.Printf("[order] Tentative d'accès non autorisé ou commande introuvable: orderID=%s, tenantID=%s", orderID, tenantID)
		return nil, ErrOrderNotFound
	}
	return ord, nil
}

// isValidOrderStatus vérifie la validité d'un statut de commande
func isValidOrderStatus(status string) bool {
	validStatuses := map[string]bool{
		StatusReceived:  true,
		StatusWashing:   true,
		StatusIroning:   true,
		StatusReady:     true,
		StatusDelivered: true,
		StatusCancelled: true,
	}
	return validStatuses[status]
}

// NewService initialise le service des commandes vide : seules les données
// réellement enregistrées par l'établissement existent (aucun jeu de
// démonstration embarqué). La séquence de numérotation démarre à 1 par tenant.
func NewService() *Service {
	return &Service{
		orders:       make(map[string]*Order),
		tenantSeqNum: make(map[string]int),
		mutations:    make(map[string]memoryMutation),
		logger:       log.Default(),
	}
}

// CreateOrder crée un nouveau dépôt de linge isolé par tenant_id
// Valide les entrées, génère les identifiants uniques et persiste la commande
func (s *Service) CreateOrder(ctx context.Context, tenantID string, req CreateOrderRequest) (*Order, error) {
	if err := validMutationKey(req.IdempotencyKey); err != nil {
		return nil, err
	}
	var err error
	req, err = normalizeOrderCustomer(req)
	if err != nil {
		return nil, err
	}
	finance, err := validateOrderFinance(req)
	if err != nil {
		return nil, err
	}

	if s.store != nil {
		s.logger.Printf("[order] Délégation création à persistance PostgreSQL, tenant=%s", tenantID)
		return s.store.creerCommande(ctx, tenantID, req, s.quota)
	}

	// Le fichier clients mémoire n'est pas persistant : ne pas accepter un ID
	// dont l'appartenance ne peut pas être établie.
	if req.CustomerID != "" {
		return nil, customer.ErrCustomerNotFound
	}

	// Validation et initialisation du tenant hors du verrou principal pour réduire le temps de blocage
	s.validateTenantExistence(tenantID)

	// Plafond de commandes (mode mémoire). Le contrôle se fait hors du verrou des
	// maps (s.mu) : le compteur d'usage relit les commandes via ce service et
	// prendrait sinon un verrou déjà tenu. L'atomicité check/insertion est assurée
	// par s.creation, qui sérialise les créations ; un refus ne consomme aucun numéro.
	s.creation.Lock()
	defer s.creation.Unlock()
	if s.quota != nil {
		plafond, err := s.quota.PlafondCommandes(ctx, tenantID)
		if err != nil {
			return nil, err
		}
		if err := s.quota.VerifierCommande(ctx, tenantID, plafond); err != nil {
			return nil, err
		}
	}

	// Verrouillage exclusif seulement pour la mise à jour des maps partagées
	s.mu.Lock()
	defer s.mu.Unlock()

	hash := mutationHash(ctx, req)
	if existing, err := s.replayMemory(tenantID, "create_order", req.IdempotencyKey, hash); err != nil || existing != nil {
		return existing, err
	}

	s.tenantSeqNum[tenantID]++
	seq := s.tenantSeqNum[tenantID]
	orderNumber := fmt.Sprintf("CMD-2026-%03d", seq)
	orderID := fmt.Sprintf("ord-%d", time.Now().UnixNano())

	totalAmount := money(finance.total)
	paidAmount := totalAmount
	paymentStatus := PaymentStatusPaid
	var items []OrderItem
	for i, it := range finance.items {
		it.ID = fmt.Sprintf("it-%s-%d", orderID, i+1)
		items = append(items, it)
	}

	var payments []PaymentRecord
	if finance.total > 0 {
		payments = append(payments, PaymentRecord{
			ID:             fmt.Sprintf("pay-%d", time.Now().UnixNano()),
			Amount:         totalAmount,
			PaymentMethod:  finance.method,
			AmountReceived: moneyPointer(finance.received),
			ChangeGiven:    moneyPointer(finance.change),
			ReceivedBy:     paymentActor(ctx),
			CreatedAt:      time.Now().UTC(),
		})
	}

	// Gestion de la date d'échéance
	dueDate := req.DueDate
	if dueDate == "" {
		dueDate = time.Now().Add(48 * time.Hour).Format("02 Jan à 17:00")
	}

	order := &Order{
		ID:            orderID,
		TenantID:      tenantID,
		CustomerID:    req.CustomerID,
		CustomerName:  req.CustomerName,
		CustomerPhone: req.CustomerPhone,
		OrderNumber:   orderNumber,
		Status:        StatusReceived,
		PaymentStatus: paymentStatus,
		TotalAmount:   totalAmount,
		PaidAmount:    paidAmount,
		RemainingDue:  totalAmount - paidAmount,
		Items:         items,
		Payments:      payments,
		DueDate:       dueDate,
		Notes:         req.Notes,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}

	order.Persisted = false
	s.orders[orderID] = order
	s.saveMemory(tenantID, "create_order", req.IdempotencyKey, hash, order)
	s.logger.Printf("[order] Commande créée: %s (%s), total=%.2f, tenant=%s", orderID, orderNumber, totalAmount, tenantID)
	return cloneOrder(order), nil
}

// DatesCommandesNonAnnulees retourne la date de création des commandes non annulées
// du pressing (mode mémoire). Alimente le compteur de quota du mode dégradé.
// Ne doit pas être appelée depuis un chemin qui tient déjà s.mu.
func (s *Service) DatesCommandesNonAnnulees(_ context.Context, tenantID string) ([]time.Time, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var dates []time.Time
	for _, ord := range s.orders {
		if ord.TenantID != tenantID || ord.Status == StatusCancelled {
			continue
		}
		dates = append(dates, ord.CreatedAt)
	}
	return dates, nil
}

// GetOrders liste les commandes en filtrant obligatoirement par tenant_id
// Supporte le filtrage par statut et la recherche textuelle sur le client/numéro de commande
func (s *Service) GetOrders(ctx context.Context, tenantID string, statusFilter string, search string) ([]*Order, error) {
	s.logger.Printf("[order] Récupération commandes: tenant=%s, statut=%s, recherche='%s'", tenantID, statusFilter, search)

	if s.store != nil {
		s.logger.Printf("[order] Délégation récupération à PostgreSQL, tenant=%s", tenantID)
		return s.store.GetOrders(ctx, tenantID, statusFilter, search)
	}

	// Verrou lecture seul pour les accès en lecture aux maps partagées
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Order
	searchLower := strings.ToLower(search)

	for _, ord := range s.orders {
		// ISOLATION MULTI-TENANT STRICTE : exclusion des commandes des autres tenants
		if ord.TenantID != tenantID {
			continue
		}

		// Filtre par statut si spécifié
		if statusFilter != "" && statusFilter != "all" && ord.Status != statusFilter {
			continue
		}

		// Filtre par recherche textuelle si spécifiée
		if searchLower != "" {
			matchName := strings.Contains(strings.ToLower(ord.CustomerName), searchLower)
			matchPhone := strings.Contains(ord.CustomerPhone, searchLower)
			matchNumber := strings.Contains(strings.ToLower(ord.OrderNumber), searchLower)
			if !matchName && !matchPhone && !matchNumber {
				continue
			}
		}

		// Copie de sécurité pour éviter modification externe des données partagées
		ordCopy := *ord
		result = append(result, &ordCopy)
	}

	s.logger.Printf("[order] %d commandes trouvées pour le tenant %s", len(result), tenantID)
	return result, nil
}

// GetOrderByID consulte une commande avec vérification de l'isolation tenant_id
// Retourne une copie de sécurité pour éviter modification externe des données partagées
func (s *Service) GetOrderByID(ctx context.Context, tenantID, orderID string) (*Order, error) {
	s.logger.Printf("[order] Consultation commande: orderID=%s, tenant=%s", orderID, tenantID)

	if s.store != nil {
		s.logger.Printf("[order] Délégation consultation à PostgreSQL: orderID=%s", orderID)
		return s.store.GetOrderByID(ctx, tenantID, orderID)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Utilisation du helper de vérification d'accès sécurisé
	ord, err := s.getSafeOrder(tenantID, orderID)
	if err != nil {
		return nil, err
	}

	// Copie de sécurité pour éviter toute modification externe des données partagées
	ordCopy := *ord
	s.logger.Printf("[order] Commande trouvée: %s, client=%s", orderID, ord.CustomerName)
	return &ordCopy, nil
}

// UpdateOrderStatus avance le statut d'un vêtement dans le flux atelier
// Vérifie l'isolation tenant et la validité du nouveau statut dans le cycle de production
func (s *Service) UpdateOrderStatus(ctx context.Context, tenantID, orderID, newStatus string) (*Order, error) {
	s.logger.Printf("[order] Mise à jour statut commande: %s, ancien=%s, nouveau=%s, tenant=%s", orderID, "?", newStatus, tenantID)

	if s.store != nil {
		s.logger.Printf("[order] Délégation mise à jour statut à PostgreSQL: orderID=%s", orderID)
		maj, err := s.store.UpdateOrderStatus(ctx, tenantID, orderID, newStatus)
		if err != nil {
			return nil, err
		}
		s.signalerStatut(ctx, tenantID, orderID, newStatus)
		return maj, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Vérification d'accès sécurisée
	ord, err := s.getSafeOrder(tenantID, orderID)
	if err != nil {
		return nil, err
	}

	// Utilisation du helper de validation de statut
	if !isValidOrderStatus(newStatus) {
		s.logger.Printf("[order] Statut invalide: %s pour commande %s", newStatus, orderID)
		return nil, ErrInvalidStatusOrder
	}
	if newStatus == StatusDelivered && !settled(ord.TotalAmount, ord.PaidAmount) {
		return nil, ErrUnpaidDelivery
	}

	oldStatus := ord.Status
	ord.Status = newStatus
	ord.UpdatedAt = time.Now().UTC()

	s.logger.Printf("[order] Statut mis à jour: %s, %s → %s", orderID, oldStatus, newStatus)

	// Copie de sécurité
	ordCopy := *ord
	s.signalerStatut(ctx, tenantID, orderID, newStatus)
	return &ordCopy, nil
}

// AddPayment enregistre un paiement acompte ou solde au retrait
// Vérifie l'isolation tenant, met à jour les montants et le statut de paiement
func (s *Service) AddPayment(ctx context.Context, tenantID, orderID string, req AddPaymentRequest) (*Order, error) {
	s.logger.Printf("[order] Ajout paiement: commande=%s, montant=%.2f, tenant=%s", orderID, req.Amount, tenantID)

	if err := validMutationKey(req.IdempotencyKey); err != nil {
		return nil, err
	}
	finance, err := validatePayment(req)
	if err != nil {
		return nil, err
	}

	if s.store != nil {
		s.logger.Printf("[order] Délégation paiement à PostgreSQL: orderID=%s", orderID)
		return s.store.AddPayment(ctx, tenantID, orderID, req)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	hash := mutationHash(ctx, req)
	if existing, err := s.replayMemory(tenantID, "add_payment:"+orderID, req.IdempotencyKey, hash); err != nil || existing != nil {
		return existing, err
	}

	// Vérification d'accès sécurisée
	ord, err := s.getSafeOrder(tenantID, orderID)
	if err != nil {
		return nil, err
	}

	totalCents, err := moneyCents(ord.TotalAmount)
	if err != nil {
		return nil, err
	}
	paidCents, err := moneyCents(ord.PaidAmount)
	if err != nil {
		return nil, err
	}
	dueCents := totalCents - paidCents
	if finance.amount > dueCents {
		s.logger.Printf("[order] Paiement refusé: montant %d > solde dû %d (commande %s, tenant=%s)",
			finance.amount, dueCents, orderID, tenantID)
		return nil, ErrPaymentExceedsDue
	}

	newPaidCents := paidCents + finance.amount
	ord.PaidAmount = money(newPaidCents)
	ord.RemainingDue = ord.TotalAmount - ord.PaidAmount
	ord.PaymentStatus = PaymentStatusPartial
	if newPaidCents >= totalCents {
		ord.PaymentStatus = PaymentStatusPaid
	}

	ord.Payments = append(ord.Payments, PaymentRecord{
		ID:             fmt.Sprintf("pay-%d", time.Now().UnixNano()),
		Amount:         money(finance.amount),
		PaymentMethod:  finance.method,
		Reference:      req.Reference,
		AmountReceived: moneyPointer(finance.received),
		ChangeGiven:    moneyPointer(finance.change),
		ReceivedBy:     paymentActor(ctx),
		CreatedAt:      time.Now().UTC(),
	})
	ord.UpdatedAt = time.Now().UTC()

	s.logger.Printf("[order] Paiement enregistré: commande=%s, nouveau statut paiement=%s, restant=%.2f",
		orderID, ord.PaymentStatus, ord.RemainingDue)

	ordCopy := cloneOrder(ord)
	s.saveMemory(tenantID, "add_payment:"+orderID, req.IdempotencyKey, hash, ordCopy)
	return ordCopy, nil
}
