package order

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Service gère les opérations métier sur les commandes de linge
type Service struct {
	mu           sync.RWMutex
	orders       map[string]*Order // clé: order_id
	tenantSeqNum map[string]int    // clé: tenant_id -> séquenceur de ticket
}

// NewService initialise le service des commandes avec les données de test initiales
func NewService() *Service {
	s := &Service{
		orders:       make(map[string]*Order),
		tenantSeqNum: make(map[string]int),
	}

	defaultTenantID := "a0000000-0000-0000-0000-000000000001"
	s.tenantSeqNum[defaultTenantID] = 87

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
func (s *Service) CreateOrder(ctx context.Context, tenantID string, req CreateOrderRequest) (*Order, error) {
	if len(req.Items) == 0 {
		return nil, ErrEmptyOrderItems
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.tenantSeqNum[tenantID]++
	seq := s.tenantSeqNum[tenantID]
	orderNumber := fmt.Sprintf("CMD-2026-%03d", seq)

	orderID := fmt.Sprintf("ord-%d", time.Now().UnixNano())

	var items []OrderItem
	var totalAmount float64

	for idx, it := range req.Items {
		if it.Quantity <= 0 {
			it.Quantity = 1
		}
		subtotal := float64(it.Quantity) * it.UnitPrice
		totalAmount += subtotal

		items = append(items, OrderItem{
			ID:        fmt.Sprintf("it-%s-%d", orderID, idx+1),
			ServiceID: it.ServiceID,
			ItemLabel: it.ItemLabel,
			Quantity:  it.Quantity,
			UnitPrice: it.UnitPrice,
			Subtotal:  subtotal,
			Notes:     it.Notes,
		})
	}

	paidAmount := req.InitialPaid
	if paidAmount < 0 {
		paidAmount = 0
	} else if paidAmount > totalAmount {
		paidAmount = totalAmount
	}

	paymentStatus := PaymentStatusUnpaid
	if paidAmount >= totalAmount && totalAmount > 0 {
		paymentStatus = PaymentStatusPaid
	} else if paidAmount > 0 {
		paymentStatus = PaymentStatusPartial
	}

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
	return order, nil
}

// GetOrders liste les commandes en filtrant obligatoirement par tenant_id
func (s *Service) GetOrders(ctx context.Context, tenantID string, statusFilter string, search string) ([]*Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Order
	searchLower := strings.ToLower(search)

	for _, ord := range s.orders {
		// ISOLATION MULTI-TENANT STRICTE
		if ord.TenantID != tenantID {
			continue
		}

		if statusFilter != "" && statusFilter != "all" && ord.Status != statusFilter {
			continue
		}

		if searchLower != "" {
			matchName := strings.Contains(strings.ToLower(ord.CustomerName), searchLower)
			matchPhone := strings.Contains(ord.CustomerPhone, searchLower)
			matchNumber := strings.Contains(strings.ToLower(ord.OrderNumber), searchLower)
			if !matchName && !matchPhone && !matchNumber {
				continue
			}
		}

		result = append(result, ord)
	}

	return result, nil
}

// GetOrderByID consulte une commande avec vérification de l'isolation tenant_id
func (s *Service) GetOrderByID(ctx context.Context, tenantID, orderID string) (*Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ord, exists := s.orders[orderID]
	if !exists || ord.TenantID != tenantID {
		return nil, ErrOrderNotFound
	}

	return ord, nil
}

// UpdateOrderStatus avance le statut d'un vêtement dans le flux atelier
func (s *Service) UpdateOrderStatus(ctx context.Context, tenantID, orderID, newStatus string) (*Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ord, exists := s.orders[orderID]
	if !exists || ord.TenantID != tenantID {
		return nil, ErrOrderNotFound
	}

	// Validation du cycle
	validStatuses := map[string]bool{
		StatusReceived:  true,
		StatusWashing:   true,
		StatusIroning:   true,
		StatusReady:     true,
		StatusDelivered: true,
		StatusCancelled: true,
	}
	if !validStatuses[newStatus] {
		return nil, ErrInvalidStatusOrder
	}

	ord.Status = newStatus
	ord.UpdatedAt = time.Now().UTC()
	return ord, nil
}

// AddPayment enregistre un paiement acompte ou solde au retrait
func (s *Service) AddPayment(ctx context.Context, tenantID, orderID string, req AddPaymentRequest) (*Order, error) {
	if req.Amount <= 0 {
		return nil, ErrInvalidPayment
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	ord, exists := s.orders[orderID]
	if !exists || ord.TenantID != tenantID {
		return nil, ErrOrderNotFound
	}

	ord.PaidAmount += req.Amount
	if ord.PaidAmount > ord.TotalAmount {
		ord.PaidAmount = ord.TotalAmount
	}
	ord.RemainingDue = ord.TotalAmount - ord.PaidAmount

	if ord.PaidAmount >= ord.TotalAmount {
		ord.PaymentStatus = PaymentStatusPaid
	} else {
		ord.PaymentStatus = PaymentStatusPartial
	}

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

	return ord, nil
}
