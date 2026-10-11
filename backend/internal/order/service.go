package order

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// Service gère les opérations métier sur les commandes de linge
// Garantit l'isolation multi-tenant des données et la sécurité d'accès concurrent
type Service struct {
	mu           sync.RWMutex
	orders       map[string]*Order // clé: order_id
	tenantSeqNum map[string]int    // clé: tenant_id -> séquenceur de ticket
	store        *dbStore          // nil = mode mémoire ; non-nil = persistance PostgreSQL
	logger       *log.Logger

	// hookStatut est appelé en tâche de fond après une transition de statut de
	// linge réussie (notifications client, intégrations). Facultatif, et
	// toujours hors du chemin critique : une panne du hook n'empêche jamais la
	// réponse HTTP.
	hookStatut func(ctx context.Context, tenantID, orderID, statut string)
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

// NewService initialise le service des commandes avec les données de test initiales
// Configure le logger et initialise les structures de stockage mémoire
func NewService() *Service {
	s := &Service{
		orders:       make(map[string]*Order),
		tenantSeqNum: make(map[string]int),
		logger:       log.Default(),
	}

	defaultTenantID := "a0000000-0000-0000-0000-000000000001"
	s.tenantSeqNum[defaultTenantID] = 87
	s.logger.Printf("[order] Service initialisé, tenant par défaut: %s (séquence: %d)", defaultTenantID, s.tenantSeqNum[defaultTenantID])

	// Commande exemple 1
	s.orders["ord-101"] = &Order{
		ID:            "ord-101",
		TenantID:      defaultTenantID,
		CustomerID:    "cust-1",
		CustomerName:  "Kouassi Jean-Marc",
		CustomerPhone: "+225 07 48 12 90 11",
		OrderNumber:   "CMD-2026-084",
		Status:        StatusIroning,
		PaymentStatus: PaymentStatusPaid,
		TotalAmount:   6000,
		PaidAmount:    6000,
		RemainingDue:  0,
		DueDate:       "Demain à 17:00",
		Notes:         "Amidon léger sur les cols de chemise",
		Items: []OrderItem{
			{ID: "it-1", ServiceID: "srv-1", ItemLabel: "Chemise classique", Quantity: 3, UnitPrice: 1000, Subtotal: 3000},
			{ID: "it-2", ServiceID: "srv-2", ItemLabel: "Pantalon costume", Quantity: 2, UnitPrice: 1500, Subtotal: 3000},
		},
		Payments: []PaymentRecord{
			{ID: "pay-1", Amount: 6000, PaymentMethod: MethodWave, Reference: "WAVE-CI-78912", CreatedAt: time.Now().Add(-2 * time.Hour)},
		},
		CreatedAt: time.Now().Add(-4 * time.Hour),
		UpdatedAt: time.Now().Add(-1 * time.Hour),
	}

	// Commande exemple 2
	s.orders["ord-102"] = &Order{
		ID:            "ord-102",
		TenantID:      defaultTenantID,
		CustomerID:    "cust-2",
		CustomerName:  "Bamba Aïcha",
		CustomerPhone: "+225 05 66 34 78 90",
		OrderNumber:   "CMD-2026-085",
		Status:        StatusWashing,
		PaymentStatus: PaymentStatusPartial,
		TotalAmount:   7500,
		PaidAmount:    4000,
		RemainingDue:  3500,
		DueDate:       "13 Octobre à 15:00",
		Notes:         "Tissu soie fragile, traitement délicat",
		Items: []OrderItem{
			{ID: "it-3", ServiceID: "srv-4", ItemLabel: "Robe de soirée / Cérémonie", Quantity: 1, UnitPrice: 4000, Subtotal: 4000},
			{ID: "it-4", ServiceID: "srv-3", ItemLabel: "Costume 2 pièces", Quantity: 1, UnitPrice: 3500, Subtotal: 3500},
		},
		Payments: []PaymentRecord{
			{ID: "pay-2", Amount: 4000, PaymentMethod: MethodCash, CreatedAt: time.Now().Add(-3 * time.Hour)},
		},
		CreatedAt: time.Now().Add(-3 * time.Hour),
		UpdatedAt: time.Now().Add(-3 * time.Hour),
	}

	// Commande exemple 3 (Prête au retrait)
	s.orders["ord-103"] = &Order{
		ID:            "ord-103",
		TenantID:      defaultTenantID,
		CustomerID:    "cust-3",
		CustomerName:  "Konan Yao Serge",
		CustomerPhone: "+225 01 02 44 88 12",
		OrderNumber:   "CMD-2026-086",
		Status:        StatusReady,
		PaymentStatus: PaymentStatusUnpaid,
		TotalAmount:   5000,
		PaidAmount:    0,
		RemainingDue:  5000,
		DueDate:       "Aujourd'hui à 18:00",
		Notes:         "Prêt pour retrait au comptoir",
		Items: []OrderItem{
			{ID: "it-5", ServiceID: "srv-5", ItemLabel: "Drap 2 places / Couette", Quantity: 2, UnitPrice: 2500, Subtotal: 5000},
		},
		Payments:  []PaymentRecord{},
		CreatedAt: time.Now().Add(-24 * time.Hour),
		UpdatedAt: time.Now().Add(-2 * time.Hour),
	}

	return s
}

// CreateOrder crée un nouveau dépôt de linge isolé par tenant_id
// Valide les entrées, génère les identifiants uniques et persiste la commande
func (s *Service) CreateOrder(ctx context.Context, tenantID string, req CreateOrderRequest) (*Order, error) {
	if len(req.Items) == 0 {
		s.logger.Printf("[order] Création échouée: aucune item fournie, tenant=%s", tenantID)
		return nil, ErrEmptyOrderItems
	}

	if s.store != nil {
		s.logger.Printf("[order] Délégation création à persistance PostgreSQL, tenant=%s", tenantID)
		return s.store.CreateOrder(ctx, tenantID, req)
	}

	// Validation et initialisation du tenant hors du verrou principal pour réduire le temps de blocage
	s.validateTenantExistence(tenantID)

	// Verrouillage exclusif seulement pour la mise à jour des maps partagées
	s.mu.Lock()
	defer s.mu.Unlock()

	s.tenantSeqNum[tenantID]++
	seq := s.tenantSeqNum[tenantID]
	orderNumber := fmt.Sprintf("CMD-2026-%03d", seq)
	orderID := fmt.Sprintf("ord-%d", time.Now().UnixNano())

	// Utilisation du helper pour créer les items et calculer le total
	items, totalAmount := createOrderItems(orderID, req.Items)

	// Validation du montant payé initial
	paidAmount := req.InitialPaid
	if paidAmount < 0 {
		paidAmount = 0
	} else if paidAmount > totalAmount {
		paidAmount = totalAmount
	}

	// Utilisation du helper pour calculer le statut de paiement
	paymentStatus := calculatePaymentStatus(paidAmount, totalAmount)

	// Création de l'enregistrement de paiement si nécessaire
	var payments []PaymentRecord
	if paidAmount > 0 {
		method := req.PaymentMethod
		if method == "" {
			method = MethodCash
		}
		payments = append(payments, PaymentRecord{
			ID:            fmt.Sprintf("pay-%d", time.Now().UnixNano()),
			Amount:        paidAmount,
			PaymentMethod: method,
			CreatedAt:     time.Now().UTC(),
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

	s.orders[orderID] = order
	s.logger.Printf("[order] Commande créée: %s (%s), total=%.2f, tenant=%s", orderID, orderNumber, totalAmount, tenantID)
	return order, nil
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

	if req.Amount <= 0 {
		s.logger.Printf("[order] Paiement refusé: montant invalide %.2f pour commande %s", req.Amount, orderID)
		return nil, ErrInvalidPayment
	}

	if s.store != nil {
		s.logger.Printf("[order] Délégation paiement à PostgreSQL: orderID=%s", orderID)
		return s.store.AddPayment(ctx, tenantID, orderID, req)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Vérification d'accès sécurisée
	ord, err := s.getSafeOrder(tenantID, orderID)
	if err != nil {
		return nil, err
	}

	// Intégrité comptable : refus de tout encaissement supérieur au solde dû.
	due := ord.TotalAmount - ord.PaidAmount
	if req.Amount > due {
		s.logger.Printf("[order] Paiement refusé: montant %.2f > solde dû %.2f (commande %s, tenant=%s)",
			req.Amount, due, orderID, tenantID)
		return nil, ErrPaymentExceedsDue
	}

	// Mise à jour des montants
	ord.PaidAmount += req.Amount
	ord.RemainingDue = ord.TotalAmount - ord.PaidAmount

	// Utilisation du helper pour calculer le nouveau statut de paiement
	ord.PaymentStatus = calculatePaymentStatus(ord.PaidAmount, ord.TotalAmount)

	// Enregistrement du paiement
	method := req.PaymentMethod
	if method == "" {
		method = MethodCash
	}

	ord.Payments = append(ord.Payments, PaymentRecord{
		ID:            fmt.Sprintf("pay-%d", time.Now().UnixNano()),
		Amount:        req.Amount,
		PaymentMethod: method,
		Reference:     req.Reference,
		CreatedAt:     time.Now().UTC(),
	})
	ord.UpdatedAt = time.Now().UTC()

	s.logger.Printf("[order] Paiement enregistré: commande=%s, nouveau statut paiement=%s, restant=%.2f",
		orderID, ord.PaymentStatus, ord.RemainingDue)

	// Copie de sécurité
	ordCopy := *ord
	return &ordCopy, nil
}
