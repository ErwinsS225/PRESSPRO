package customer

import (
	"context"
	"strings"
)

// Service expose la logique métier clients et délègue la persistance au store.
type Service struct {
	store Store
}

// NewService crée un service sans persistance (mode mémoire dégradé).
func NewService() *Service {
	return &Service{}
}

// SetStore branche la persistance PostgreSQL quand elle est disponible.
func (s *Service) SetStore(store Store) {
	s.store = store
}

func normalize(v string) string {
	return strings.TrimSpace(v)
}

// GetCustomers retourne les clients du tenant (recherche optionnelle).
func (s *Service) GetCustomers(ctx context.Context, tenantID, search string) ([]*Customer, error) {
	if s.store != nil {
		return s.store.GetCustomers(ctx, tenantID, search)
	}
	return []*Customer{}, nil
}

// GetCustomer retourne un client par ID.
func (s *Service) GetCustomer(ctx context.Context, tenantID, id string) (*Customer, error) {
	if s.store != nil {
		return s.store.GetCustomer(ctx, tenantID, id)
	}
	return nil, ErrCustomerNotFound
}

// CreateCustomer valide puis crée un client.
func (s *Service) CreateCustomer(ctx context.Context, tenantID string, req CreateCustomerRequest) (*Customer, error) {
	req.FirstName = normalize(req.FirstName)
	req.LastName = normalize(req.LastName)
	req.Phone = normalize(req.Phone)
	if req.FirstName == "" {
		return nil, ErrEmptyFirstName
	}
	if req.Phone == "" {
		return nil, ErrEmptyPhone
	}
	if s.store != nil {
		return s.store.CreateCustomer(ctx, tenantID, req)
	}
	return &Customer{
		TenantID:  tenantID,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Phone:     req.Phone,
		Email:     req.Email,
		Address:   req.Address,
		Notes:     req.Notes,
		FullName:  buildFullName(req.FirstName, req.LastName),
	}, nil
}

// UpdateCustomer valide puis met à jour un client.
func (s *Service) UpdateCustomer(ctx context.Context, tenantID, id string, req UpdateCustomerRequest) (*Customer, error) {
	req.FirstName = normalize(req.FirstName)
	req.LastName = normalize(req.LastName)
	req.Phone = normalize(req.Phone)
	if req.FirstName == "" {
		return nil, ErrEmptyFirstName
	}
	if req.Phone == "" {
		return nil, ErrEmptyPhone
	}
	if s.store != nil {
		return s.store.UpdateCustomer(ctx, tenantID, id, req)
	}
	return nil, ErrCustomerNotFound
}

// DeleteCustomer supprime un client.
func (s *Service) DeleteCustomer(ctx context.Context, tenantID, id string) error {
	if s.store != nil {
		return s.store.DeleteCustomer(ctx, tenantID, id)
	}
	return ErrCustomerNotFound
}
