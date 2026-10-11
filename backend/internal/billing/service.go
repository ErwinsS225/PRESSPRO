package billing

import (
	"context"
	"strings"
	"sync"
	"time"
)

// policyRow décrit un droit issu du seed (fallback mémoire).
type policyRow struct {
	enabled bool
	limit   *int64
}

func int64ptr(v int64) *int64 { return &v }

// defaultMatrix reproduit la matrice seedée en base (spec section 2) et les
// ajustements des migrations (000012 : expenses.advanced payant).
// NULL (sans plafond) est représenté par un pointeur nil.
func defaultMatrix() map[string]map[string]policyRow {
	free := map[string]policyRow{
		FeatureOrdersCreate:     {enabled: true},
		FeatureOrdersMonthlyMax: {enabled: true, limit: int64ptr(50)},
		FeatureUsersMax:         {enabled: true, limit: int64ptr(2)},
		FeaturePriceLists:       {enabled: true, limit: int64ptr(1)},
		FeatureMultiSite:        {enabled: true, limit: int64ptr(1)},
		FeatureReportsAdvanced:  {enabled: false},
		// Migration 000012 : le suivi avancé des dépenses est réservé aux
		// formules payantes (PRO / BUSINESS), l'essai actif valant BUSINESS.
		FeatureExpensesAdvanced: {enabled: false},
		FeaturePricingPro:       {enabled: false},
		FeatureReportsConsol:    {enabled: false},
		FeatureAuditAdvanced:    {enabled: true},
		// Migration 000014 : l'envoi réel des notifications clients est réservé
		// aux formules payantes (le journal et les modèles restent gratuits).
		FeatureNotificationsAdvanced: {enabled: false},
	}
	pro := map[string]policyRow{
		FeatureOrdersCreate:          {enabled: true},
		FeatureOrdersMonthlyMax:      {enabled: true},
		FeatureUsersMax:              {enabled: true, limit: int64ptr(5)},
		FeaturePriceLists:            {enabled: true, limit: int64ptr(3)},
		FeatureMultiSite:             {enabled: true, limit: int64ptr(1)},
		FeatureReportsAdvanced:       {enabled: true},
		FeatureExpensesAdvanced:      {enabled: true},
		FeaturePricingPro:            {enabled: true},
		FeatureReportsConsol:         {enabled: false},
		FeatureAuditAdvanced:         {enabled: true},
		FeatureNotificationsAdvanced: {enabled: true},
	}
	business := map[string]policyRow{
		FeatureOrdersCreate:          {enabled: true},
		FeatureOrdersMonthlyMax:      {enabled: true},
		FeatureUsersMax:              {enabled: true, limit: int64ptr(15)},
		FeaturePriceLists:            {enabled: true, limit: int64ptr(10)},
		FeatureMultiSite:             {enabled: true, limit: int64ptr(3)},
		FeatureReportsAdvanced:       {enabled: true},
		FeatureExpensesAdvanced:      {enabled: true},
		FeaturePricingPro:            {enabled: true},
		FeatureReportsConsol:         {enabled: true},
		FeatureAuditAdvanced:         {enabled: true},
		FeatureNotificationsAdvanced: {enabled: true},
	}
	return map[string]map[string]policyRow{
		PlanFree: free, PlanPro: pro, PlanBusiness: business,
	}
}

// UsageCounter fournit les consommations réelles (quotas).
type UsageCounter interface {
	CountMonthlyOrders(ctx context.Context, tenantID string) (int64, error)
	CountUsers(ctx context.Context, tenantID string) (int64, error)
	// CountPriceLists alimente le plafond de la fonctionnalité « grilles
	// tarifaires » (1 en formule gratuite, 3 en PRO, 10 en BUSINESS).
	CountPriceLists(ctx context.Context, tenantID string) (int64, error)
	// CountBranches alimente le plafond multi-sites « branches.multi_site »
	// (1 établissement en gratuit et PRO, 3 en BUSINESS).
	CountBranches(ctx context.Context, tenantID string) (int64, error)
}

// memoryCounter : zéro usage (mode dégradé).
type memoryCounter struct{}

func (memoryCounter) CountMonthlyOrders(_ context.Context, _ string) (int64, error) {
	return 0, nil
}
func (memoryCounter) CountPriceLists(_ context.Context, _ string) (int64, error) {
	return 0, nil
}

func (memoryCounter) CountUsers(_ context.Context, _ string) (int64, error) {
	return 0, nil
}

func (memoryCounter) CountBranches(_ context.Context, _ string) (int64, error) {
	return 0, nil
}

// SubscriptionRecord persiste un abonnement (DB ou mémoire).
type SubscriptionRecord struct {
	PlanCode       string
	Status         string
	TrialStartedAt *time.Time
	TrialEndsAt    *time.Time
	PlanName       string
	PriceMonthly   float64
}

// Service calcule les droits effectifs (moteur centralisé).
type Service struct {
	mu       sync.RWMutex
	subs     map[string]*SubscriptionRecord
	matrix   map[string]map[string]policyRow
	counters UsageCounter
	store    Store
}

// NewService initialise le moteur avec le tenant démo en essai 7 jours.
func NewService() *Service {
	now := time.Now().UTC()
	ends := now.Add(TrialDurationDays * 24 * time.Hour)
	s := &Service{
		subs:     make(map[string]*SubscriptionRecord),
		matrix:   defaultMatrix(),
		counters: memoryCounter{},
	}
	s.subs["a0000000-0000-0000-0000-000000000001"] = &SubscriptionRecord{
		PlanCode:       PlanFree,
		Status:         StatusTrialing,
		TrialStartedAt: &now,
		TrialEndsAt:    &ends,
		PlanName:       "Gratuit permanent",
	}
	return s
}

// SetUsageCounter branche les compteurs réels (DB).
func (s *Service) SetUsageCounter(c UsageCounter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counters = c
}

// OnTrial indique si l'essai est actif (comparaison serveur, pas de cron requis).
func OnTrial(rec *SubscriptionRecord, now time.Time) bool {
	if rec == nil || rec.Status != StatusTrialing || rec.TrialEndsAt == nil {
		return false
	}
	return now.Before(*rec.TrialEndsAt)
}

// EffectivePlan retourne le plan effectif : BUSINESS pendant l'essai.
func EffectivePlan(rec *SubscriptionRecord, now time.Time) string {
	if OnTrial(rec, now) {
		return PlanBusiness
	}
	if rec == nil || rec.PlanCode == "" {
		return PlanFree
	}
	return strings.ToUpper(rec.PlanCode)
}
