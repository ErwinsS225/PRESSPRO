package billing

import "context"

// QuotaGuard bloque la création au-delà du plafond mensuel du plan.
// Essai actif = niveau BUSINESS (sans plafond prédéfini) ; essai dépassé = FREE (50).
type QuotaGuard struct {
	Billing *Service
	Counter UsageCounter
}

// CheckOrdersCreation vérifie le quota avant POST /orders.
func (g *QuotaGuard) CheckOrdersCreation(ctx context.Context, tenantID string) error {
	if g == nil || g.Billing == nil {
		return nil
	}
	counter := g.Counter
	if counter == nil {
		counter = g.Billing.counters
	}
	if counter == nil {
		counter = memoryCounter{}
	}
	current, err := counter.CountMonthlyOrders(ctx, tenantID)
	if err != nil {
		return err
	}
	return g.Billing.CheckLimit(ctx, tenantID, FeatureOrdersMonthlyMax, current)
}
