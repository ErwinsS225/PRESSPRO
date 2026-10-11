package catalog

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *dbStore) CreateService(ctx context.Context, tenantID string, req CreateServiceRequest) (*ServiceItem, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrEmptyName
	}
	if req.UnitPrice < 0 {
		return nil, ErrInvalidPrice
	}
	if strings.TrimSpace(tenantID) == "" {
		return nil, ErrServiceNotFound
	}

	category := req.Category
	if category == "" {
		category = CategoryDaily
	}
	unit := req.Unit
	if unit == "" {
		unit = "pièce"
	}
	hours := req.EstimatedHours
	if hours <= 0 {
		hours = 24
	}
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	article := req.Article
	if article == "" {
		article = name
	}
	treatment := req.Treatment
	if treatment == "" {
		treatment = "Lavage & Soin"
	}
	pricingModel := req.PricingModel
	if pricingModel == "" {
		pricingModel = ModelPiece
	}
	isExpress := false
	if req.IsExpressAvailable != nil {
		isExpress = *req.IsExpressAvailable
	}
	var surcharge float64
	if req.ExpressSurcharge != nil {
		surcharge = *req.ExpressSurcharge
	}
	isPickup := true
	if req.IsPickupAvailable != nil {
		isPickup = *req.IsPickupAvailable
	}
	isOutsourced := false
	if req.IsOutsourced != nil {
		isOutsourced = *req.IsOutsourced
	}
	requiresQuote := false
	if req.RequiresQuote != nil {
		requiresQuote = *req.RequiresQuote
	}

	query := `INSERT INTO services (tenant_id, name, article, treatment, category, unit_price, unit, pricing_model,
	                                estimated_hours, is_active, is_express_available, express_surcharge,
	                                is_pickup_available, is_outsourced, requires_quote, description)
	          VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	          RETURNING id, tenant_id, name, article, treatment, category, unit_price, unit, pricing_model,
	                    estimated_hours, is_active, is_express_available, express_surcharge,
	                    is_pickup_available, is_outsourced, requires_quote, description,
	                    created_at, updated_at`
	row := s.database.QueryRowContext(ctx, query,
		tenantID,
		name,
		article,
		treatment,
		category,
		req.UnitPrice,
		unit,
		pricingModel,
		hours,
		isActive,
		isExpress,
		surcharge,
		isPickup,
		isOutsourced,
		requiresQuote,
		req.Description,
	)
	return scanService(row)
}

func (s *dbStore) UpdateService(ctx context.Context, tenantID, id string, req UpdateServiceRequest) (*ServiceItem, error) {
	if id == "" {
		return nil, ErrServiceNotFound
	}

	sets := []string{}
	args := []interface{}{}
	arg := 1

	if strings.TrimSpace(req.Name) != "" {
		sets = append(sets, fmt.Sprintf("name=$%d", arg))
		args = append(args, req.Name)
		arg++
	}
	if req.Article != "" {
		sets = append(sets, fmt.Sprintf("article=$%d", arg))
		args = append(args, req.Article)
		arg++
	}
	if req.Treatment != "" {
		sets = append(sets, fmt.Sprintf("treatment=$%d", arg))
		args = append(args, req.Treatment)
		arg++
	}
	if req.Category != "" {
		sets = append(sets, fmt.Sprintf("category=$%d", arg))
		args = append(args, req.Category)
		arg++
	}
	if req.UnitPrice >= 0 {
		sets = append(sets, fmt.Sprintf("unit_price=$%d", arg))
		args = append(args, req.UnitPrice)
		arg++
	}
	if req.Unit != "" {
		sets = append(sets, fmt.Sprintf("unit=$%d", arg))
		args = append(args, req.Unit)
		arg++
	}
	if req.PricingModel != "" {
		sets = append(sets, fmt.Sprintf("pricing_model=$%d", arg))
		args = append(args, req.PricingModel)
		arg++
	}
	if req.EstimatedHours > 0 {
		sets = append(sets, fmt.Sprintf("estimated_hours=$%d", arg))
		args = append(args, req.EstimatedHours)
		arg++
	}
	if req.IsActive != nil {
		sets = append(sets, fmt.Sprintf("is_active=$%d", arg))
		args = append(args, *req.IsActive)
		arg++
	}
	if req.IsExpressAvailable != nil {
		sets = append(sets, fmt.Sprintf("is_express_available=$%d", arg))
		args = append(args, *req.IsExpressAvailable)
		arg++
	}
	if req.ExpressSurcharge != nil {
		sets = append(sets, fmt.Sprintf("express_surcharge=$%d", arg))
		args = append(args, *req.ExpressSurcharge)
		arg++
	}
	if req.IsPickupAvailable != nil {
		sets = append(sets, fmt.Sprintf("is_pickup_available=$%d", arg))
		args = append(args, *req.IsPickupAvailable)
		arg++
	}
	if req.IsOutsourced != nil {
		sets = append(sets, fmt.Sprintf("is_outsourced=$%d", arg))
		args = append(args, *req.IsOutsourced)
		arg++
	}
	if req.RequiresQuote != nil {
		sets = append(sets, fmt.Sprintf("requires_quote=$%d", arg))
		args = append(args, *req.RequiresQuote)
		arg++
	}
	if req.Description != "" {
		sets = append(sets, fmt.Sprintf("description=$%d", arg))
		args = append(args, req.Description)
		arg++
	}
	sets = append(sets, "updated_at=NOW()")
	args = append(args, id, tenantID)

	query := fmt.Sprintf(`UPDATE services SET %s WHERE id=$%d AND tenant_id=$%d
	                       RETURNING id, tenant_id, name, article, treatment, category, unit_price, unit, pricing_model,
	                                 estimated_hours, is_active, is_express_available, express_surcharge,
	                                 is_pickup_available, is_outsourced, requires_quote, description,
	                                 created_at, updated_at`,
		strings.Join(sets, ", "), arg, arg+1)

	return scanService(s.database.QueryRowContext(ctx, query, args...))
}

func (s *dbStore) DeleteService(ctx context.Context, tenantID, id string) error {
	if id == "" {
		return ErrServiceNotFound
	}
	result, err := s.database.ExecContext(ctx, `DELETE FROM services WHERE id = $1 AND tenant_id = $2`, id, tenantID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrServiceNotFound
	}
	return nil
}

// scanServices reads a pgx.Rows into a []*ServiceItem slice.
func scanServices(rows pgx.Rows) ([]*ServiceItem, error) {
	items := []*ServiceItem{}
	for rows.Next() {
		var (
			id                 string
			tenantID           string
			name               string
			article            *string
			treatment          *string
			category           string
			unit               string
			pricingModel       *string
			unitPrice          float64
			estimatedHours     int
			isActive           bool
			isExpressAvailable bool
			expressSurcharge   float64
			isPickupAvailable  bool
			isOutsourced       bool
			requiresQuote      bool
			description        *string
			createdAt          time.Time
			updatedAt          time.Time
		)
		if err := rows.Scan(&id, &tenantID, &name, &article, &treatment, &category, &unitPrice, &unit,
			&pricingModel, &estimatedHours, &isActive, &isExpressAvailable, &expressSurcharge,
			&isPickupAvailable, &isOutsourced, &requiresQuote, &description,
			&createdAt, &updatedAt); err != nil {
			return nil, err
		}
		items = append(items, &ServiceItem{
			ID:                 id,
			TenantID:           tenantID,
			Name:               name,
			Article:            derefStr(article, name),
			Treatment:          derefStr(treatment, "Lavage & Soin"),
			Category:           category,
			UnitPrice:          unitPrice,
			Unit:               unit,
			PricingModel:       derefStr(pricingModel, ModelPiece),
			EstimatedHours:     estimatedHours,
			IsActive:           isActive,
			IsExpressAvailable: isExpressAvailable,
			ExpressSurcharge:   expressSurcharge,
			IsPickupAvailable:  isPickupAvailable,
			IsOutsourced:       isOutsourced,
			RequiresQuote:      requiresQuote,
			Description:        derefStr(description, ""),
			CreatedAt:          createdAt,
			UpdatedAt:          updatedAt,
		})
	}
	return items, nil
}

// scanService reads a single pgx.Row into a ServiceItem.
func scanService(row pgx.Row) (*ServiceItem, error) {
	var (
		id                 string
		tenantID           string
		name               string
		article            *string
		treatment          *string
		category           string
		unit               string
		pricingModel       *string
		unitPrice          float64
		estimatedHours     int
		isActive           bool
		isExpressAvailable bool
		expressSurcharge   float64
		isPickupAvailable  bool
		isOutsourced       bool
		requiresQuote      bool
		description        *string
		createdAt          time.Time
		updatedAt          time.Time
	)
	if err := row.Scan(&id, &tenantID, &name, &article, &treatment, &category, &unitPrice, &unit,
		&pricingModel, &estimatedHours, &isActive, &isExpressAvailable, &expressSurcharge,
		&isPickupAvailable, &isOutsourced, &requiresQuote, &description,
		&createdAt, &updatedAt); err != nil {
		// Une ligne absente (identifiant inconnu ou appartenant à un autre tenant)
		// est une erreur métier : sans cette traduction, l'API remonterait le
		// message pgx brut « no rows in result set » et répondrait 500 au lieu de 404.
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrServiceNotFound
		}
		return nil, err
	}
	return &ServiceItem{
		ID:                 id,
		TenantID:           tenantID,
		Name:               name,
		Article:            derefStr(article, name),
		Treatment:          derefStr(treatment, "Lavage & Soin"),
		Category:           category,
		UnitPrice:          unitPrice,
		Unit:               unit,
		PricingModel:       derefStr(pricingModel, ModelPiece),
		EstimatedHours:     estimatedHours,
		IsActive:           isActive,
		IsExpressAvailable: isExpressAvailable,
		ExpressSurcharge:   expressSurcharge,
		IsPickupAvailable:  isPickupAvailable,
		IsOutsourced:       isOutsourced,
		RequiresQuote:      requiresQuote,
		Description:        derefStr(description, ""),
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
	}, nil
}

func derefStr(v *string, fallback string) string {
	if v == nil {
		return fallback
	}
	return *v
}
