package order

import (
	"strings"

	"pressing-os/backend/internal/customer"
)

// normalizeOrderCustomer valide la forme du choix avant tout numéro ou écriture.
// Les anciennes coordonnées accompagnant un ID ne sont jamais une modification.
func normalizeOrderCustomer(req CreateOrderRequest) (CreateOrderRequest, error) {
	req.CustomerID = strings.TrimSpace(req.CustomerID)
	if req.NewCustomer != nil && (req.CustomerID != "" ||
		strings.TrimSpace(req.CustomerName) != "" || strings.TrimSpace(req.CustomerPhone) != "") {
		return req, ErrAmbiguousCustomer
	}
	if req.CustomerID != "" {
		return req, nil // la fiche canonique sera vérifiée dans la transaction SQL
	}
	input, err := newCustomerInput(req)
	if err != nil {
		return req, err
	}
	if req.NewCustomer != nil {
		req.NewCustomer = &input
	}
	req.CustomerName = strings.TrimSpace(input.FirstName + " " + input.LastName)
	req.CustomerPhone = input.Phone
	return req, nil
}

func newCustomerInput(req CreateOrderRequest) (customer.CreateCustomerRequest, error) {
	if req.NewCustomer != nil {
		return customer.NormalizeCreateRequest(*req.NewCustomer)
	}
	// Compatibilité de l'ancien nom complet uniquement ; les nouveaux appels
	// structurés conservent les prénoms composés et le nom sans découpage.
	if strings.TrimSpace(req.CustomerName) == "" {
		return customer.CreateCustomerRequest{}, customer.ErrEmptyFirstName
	}
	first, last := splitName(req.CustomerName)
	return customer.NormalizeCreateRequest(customer.CreateCustomerRequest{
		FirstName: first, LastName: last, Phone: req.CustomerPhone,
	})
}
