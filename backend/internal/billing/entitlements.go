package billing

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// GetSubscription retourne l'abonnement (DB puis mémoire).
func (s *Service) GetSubscription(ctx context.Context, tenantID string) (*SubscriptionRecord, error) {
	if s.store != nil {
		rec, err := s.store.GetSubscription(ctx, tenantID)
		if err != nil {
			return nil, err
		}
		if rec != nil {
			return rec, nil
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if rec, ok := s.subs[tenantID]; ok {
		return rec, nil
	}
	return &SubscriptionRecord{PlanCode: PlanFree, Status: StatusActive, PlanName: "Gratuit permanent"}, nil
}

// EffectiveEntitlements calcule les droits effectifs + usages.
func (s *Service) EffectiveEntitlements(ctx context.Context, tenantID string) (*EntitlementsResponse, error) {
	rec, err := s.GetSubscription(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	plan := EffectivePlan(rec, now)
	onTrial := OnTrial(rec, now)

	var entitlements []Entitlement
	if s.store != nil {
		entitlements, err = s.store.ListEntitlements(ctx, plan)
		if err != nil {
			return nil, err
		}
	} else {
		s.mu.RLock()
		rows := s.matrix[plan]
		s.mu.RUnlock()
		for key, row := range rows {
			var lim *int64
			if row.limit != nil {
				v := *row.limit
				lim = &v
			}
			entitlements = append(entitlements, Entitlement{
				Key: key, Enabled: row.enabled, Limit: lim, Source: "plan",
			})
		}
	}

	// Enrichir les plafonds avec la consommation réelle (commandes du mois,
	// comptes utilisateurs, grilles tarifaires).
	for i := range entitlements {
		if usage, err := s.FeatureUsage(ctx, tenantID, entitlements[i].Key); err == nil && usage >= 0 && isCountedFeature(entitlements[i].Key) {
			u := usage
			entitlements[i].Usage = &u
			if entitlements[i].Limit != nil {
				r := *entitlements[i].Limit - u
				if r < 0 {
					r = 0 // un quota consommé au-delà du plafond n'affiche jamais un reste négatif
				}
				entitlements[i].Remaining = &r
			}
		}
		if onTrial {
			entitlements[i].Source = "trial"
		}
	}

	resp := &EntitlementsResponse{
		TenantID:     tenantID,
		PlanCode:     plan,
		PlanName:     rec.PlanName,
		Status:       rec.Status,
		OnTrial:      onTrial,
		Entitlements: entitlements,
	}
	if onTrial && rec.TrialEndsAt != nil {
		t := *rec.TrialEndsAt
		resp.TrialEndsAt = &t
		days := int(t.Sub(now).Hours() / 24)
		if days < 0 {
			days = 0
		}
		resp.DaysLeft = &days
	}
	if resp.PlanName == "" {
		resp.PlanName = plan
	}
	return resp, nil
}

// isCountedFeature indique les fonctionnalités dont la consommation est mesurée.
func isCountedFeature(featureKey string) bool {
	switch featureKey {
	case FeatureOrdersMonthlyMax, FeatureUsersMax, FeaturePriceLists:
		return true
	}
	return false
}

// FeatureUsage retourne la consommation réelle d'une fonctionnalité plafonnée.
// Une clé sans compteur (fonctionnalité booléenne) retourne 0.
func (s *Service) FeatureUsage(ctx context.Context, tenantID, featureKey string) (int64, error) {
	if s == nil || s.counters == nil || tenantID == "" {
		return 0, nil
	}
	switch featureKey {
	case FeatureOrdersMonthlyMax:
		return s.counters.CountMonthlyOrders(ctx, tenantID)
	case FeatureUsersMax:
		return s.counters.CountUsers(ctx, tenantID)
	case FeaturePriceLists:
		return s.counters.CountPriceLists(ctx, tenantID)
	}
	return 0, nil
}

// IsEnabled vérifie une fonctionnalité (essai = tout débloqué).
func (s *Service) IsEnabled(ctx context.Context, tenantID, featureKey string) (bool, error) {
	resp, err := s.EffectiveEntitlements(ctx, tenantID)
	if err != nil {
		return false, err
	}
	for _, e := range resp.Entitlements {
		if e.Key == featureKey {
			return e.Enabled, nil
		}
	}
	return false, nil
}

// CheckLimit vérifie un plafond avant création (ex: 51e commande FREE).
func (s *Service) CheckLimit(ctx context.Context, tenantID, featureKey string, current int64) error {
	resp, err := s.EffectiveEntitlements(ctx, tenantID)
	if err != nil {
		return err
	}
	for _, e := range resp.Entitlements {
		if e.Key != featureKey {
			continue
		}
		if !e.Enabled {
			return fmt.Errorf("fonctionnalité %s non incluse dans votre formule", featureKey)
		}
		if e.Limit != nil && current >= *e.Limit {
			return fmt.Errorf("plafond atteint (%d) — passez à une formule supérieure", *e.Limit)
		}
		return nil
	}
	return fmt.Errorf("fonctionnalité inconnue : %s", featureKey)
}

// ExpireTrialIfNeeded applique J7 : TRIALING dépassé -> ACTIVE/FREE sans purge.
func (s *Service) ExpireTrialIfNeeded(ctx context.Context, tenantID string, actorUserID string) (bool, error) {
	rec, err := s.GetSubscription(ctx, tenantID)
	if err != nil {
		return false, err
	}
	if rec.Status != StatusTrialing || rec.TrialEndsAt == nil {
		return false, nil
	}
	if !time.Now().UTC().After(*rec.TrialEndsAt) {
		return false, nil
	}
	fromPlan := rec.PlanCode
	if s.store != nil {
		if err := s.store.SetPlan(ctx, tenantID, PlanFree, StatusActive, actorUserID, "trial_expired"); err != nil {
			return false, err
		}
		return true, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if r, ok := s.subs[tenantID]; ok {
		r.PlanCode = PlanFree
		r.Status = StatusActive
		r.PlanName = "Gratuit permanent"
	}
	_ = fromPlan
	_ = strings.ToUpper(fromPlan)
	return true, nil
}
