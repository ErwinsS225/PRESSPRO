package billing

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// GetSubscription retourne l'abonnement (DB puis mémoire). Un essai terminé est
// fait passer à FREE/ACTIVE côté serveur au moment de la lecture : l'expiration
// ne dépend ni d'un cron ni d'un appel du frontend à /me/entitlements.
func (s *Service) GetSubscription(ctx context.Context, tenantID string) (*SubscriptionRecord, error) {
	now := time.Now().UTC()
	rec, err := s.readSubscription(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if !trialExpired(rec, now) {
		return rec, nil
	}
	if _, err := s.applyTrialExpiry(ctx, tenantID, now, ""); err != nil {
		return nil, err
	}
	return s.readSubscription(ctx, tenantID)
}

// readSubscription lit l'état courant sans effectuer de transition.
func (s *Service) readSubscription(ctx context.Context, tenantID string) (*SubscriptionRecord, error) {
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
		// Copie sous verrou : l'appelant ne partage pas l'enregistrement mémoire.
		c := *rec
		return &c, nil
	}
	return &SubscriptionRecord{PlanCode: PlanFree, Status: StatusActive, PlanName: "Gratuit permanent"}, nil
}

// applyTrialExpiry effectue la transition d'expiration si elle est due, de façon
// atomique (verrou de ligne en base, verrou mutex en mémoire). Retourne true
// uniquement pour l'appel qui a effectivement fait la transition.
func (s *Service) applyTrialExpiry(ctx context.Context, tenantID string, now time.Time, actorUserID string) (bool, error) {
	if s.store != nil {
		return s.store.ExpireTrial(ctx, tenantID, now, actorUserID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.subs[tenantID]
	if !ok || !trialExpired(r, now) {
		return false, nil
	}
	r.PlanCode = PlanFree
	r.Status = StatusActive
	r.PlanName = "Gratuit permanent"
	return true, nil
}

// trialExpired : même borne que OnTrial (actif si now < trial_ends_at).
func trialExpired(rec *SubscriptionRecord, now time.Time) bool {
	return rec != nil && rec.Status == StatusTrialing && rec.TrialEndsAt != nil && !now.Before(*rec.TrialEndsAt)
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

	entitlements, err := s.entitlementsDuPlan(ctx, plan)
	if err != nil {
		return nil, err
	}

	// Enrichir les plafonds avec la consommation réelle (commandes de la fenêtre de 30 jours,
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

	souscrit := strings.ToUpper(rec.PlanCode)
	if souscrit == "" {
		souscrit = PlanFree
	}
	resp := &EntitlementsResponse{
		TenantID:           tenantID,
		PlanCode:           plan,
		SubscribedPlanCode: souscrit,
		PlanName:           rec.PlanName,
		Status:             rec.Status,
		OnTrial:            onTrial,
		Entitlements:       entitlements,
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
	// Tarif de la formule souscrite (et non de la formule effective pendant l'essai).
	if s.store != nil {
		prix := rec.PriceMonthly
		resp.PriceMonthlyXOF = &prix
	}
	if anchor := ancreFenetre(rec); !anchor.IsZero() {
		periode := FenetreCommandes(anchor, now)
		resp.OrdersPeriod = &periode
	}
	return resp, nil
}

// entitlementsDuPlan retourne les droits d'une formule (table plan_entitlements en
// base, matrice en mémoire). Source unique pour les droits effectifs et le catalogue.
func (s *Service) entitlementsDuPlan(ctx context.Context, plan string) ([]Entitlement, error) {
	if s.store != nil {
		return s.store.ListEntitlements(ctx, plan)
	}
	s.mu.RLock()
	rows := s.matrix[plan]
	s.mu.RUnlock()
	var out []Entitlement
	for key, row := range rows {
		var lim *int64
		if row.limit != nil {
			v := *row.limit
			lim = &v
		}
		out = append(out, Entitlement{Key: key, Enabled: row.enabled, Limit: lim, Source: "plan"})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

// Catalogue retourne les formules publiques avec leur tarif et leurs droits.
// Le tarif vient de la base ; en mode mémoire il est absent (aucun prix n'est
// inventé côté serveur ni côté interface).
func (s *Service) Catalogue(ctx context.Context) ([]PlanCatalogue, error) {
	var plans []PlanCatalogue
	if s.store != nil {
		var err error
		plans, err = s.store.ListPlans(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		plans = []PlanCatalogue{
			{Code: PlanFree, Name: "Gratuit permanent"},
			{Code: PlanPro, Name: "Pro"},
			{Code: PlanBusiness, Name: "Business"},
		}
	}
	for i := range plans {
		droits, err := s.entitlementsDuPlan(ctx, plans[i].Code)
		if err != nil {
			return nil, err
		}
		plans[i].Entitlements = droits
	}
	return plans, nil
}

// isCountedFeature indique les fonctionnalités dont la consommation est mesurée.
func isCountedFeature(featureKey string) bool {
	switch featureKey {
	case FeatureOrdersMonthlyMax, FeatureUsersMax, FeaturePriceLists, FeatureMultiSite:
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
		return s.counters.CountOrdersInPeriod(ctx, tenantID, time.Now().UTC())
	case FeatureUsersMax:
		return s.counters.CountUsers(ctx, tenantID)
	case FeaturePriceLists:
		return s.counters.CountPriceLists(ctx, tenantID)
	case FeatureMultiSite:
		return s.counters.CountBranches(ctx, tenantID)
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
// Un refus métier est retourné avec une cause (IsRefusMetier) ; une erreur de
// lecture de l'abonnement est retournée telle quelle (erreur technique).
func (s *Service) CheckLimit(ctx context.Context, tenantID, featureKey string, current int64) error {
	resp, err := s.EffectiveEntitlements(ctx, tenantID)
	if err != nil {
		return err
	}
	for _, e := range resp.Entitlements {
		if e.Key != featureKey {
			continue
		}
		return verifierPlafond(featureKey, e.Enabled, e.Limit, current)
	}
	return fmt.Errorf("%w : %s", ErrUnknownFeature, featureKey)
}

// ExpireTrialIfNeeded applique l'expiration de l'essai si elle est due (idempotent) :
// TRIALING dépassé -> ACTIVE/FREE, sans purge des données. Retourne true uniquement
// pour l'appel qui effectue la transition.
func (s *Service) ExpireTrialIfNeeded(ctx context.Context, tenantID string, actorUserID string) (bool, error) {
	return s.applyTrialExpiry(ctx, tenantID, time.Now().UTC(), actorUserID)
}
