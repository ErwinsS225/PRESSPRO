package billing

import (
	"context"
	"errors"
	"time"
)

// Store est le contrat de persistance des abonnements.
type Store interface {
	GetSubscription(ctx context.Context, tenantID string) (*SubscriptionRecord, error)
	ListEntitlements(ctx context.Context, planCode string) ([]Entitlement, error)
	SetPlan(ctx context.Context, tenantID, planCode, status, actorUserID, reason string) error
	StartTrial(ctx context.Context, tenantID string, trialEndsAt time.Time) error

	// Espace SuperAdmin (éditeur) : indicateurs consolidés et annuaire réel.
	PlatformStats(ctx context.Context) (*PlatformStats, error)
	ListTenantOverviews(ctx context.Context) ([]TenantOverview, error)
	SetSubscriptionStatus(ctx context.Context, tenantID, status, actorUserID, reason string) error
}

// ErrTenantSubscriptionNotFound : aucun abonnement pour cet établissement.
var ErrTenantSubscriptionNotFound = errors.New("aucun abonnement pour cet établissement")

// EnsureTrial démarre l'essai 7 jours pour un nouveau tenant (DB ou mémoire).
func (s *Service) EnsureTrial(ctx context.Context, tenantID string) error {
	ends := time.Now().UTC().Add(TrialDurationDays * 24 * time.Hour)
	if s.store != nil {
		return s.store.StartTrial(ctx, tenantID, ends)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.subs[tenantID]; ok {
		return nil
	}
	now := time.Now().UTC()
	s.subs[tenantID] = &SubscriptionRecord{
		PlanCode:       PlanFree,
		Status:         StatusTrialing,
		TrialStartedAt: &now,
		TrialEndsAt:    &ends,
		PlanName:       "Gratuit permanent",
	}
	return nil
}
