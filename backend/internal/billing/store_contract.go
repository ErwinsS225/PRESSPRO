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
	// ListPlans retourne les formules actives (tarif en base), sans droits.
	ListPlans(ctx context.Context) ([]PlanCatalogue, error)
	// SetPlanPrice modifie le tarif d'une formule dans une transaction où l'acteur et le
	// motif sont positionnés pour le trigger de journalisation (plan_price_history).
	SetPlanPrice(ctx context.Context, planCode string, priceXOF float64, actorUserID, reason string) error
	SetPlan(ctx context.Context, tenantID, planCode, status, actorUserID, reason string) error
	StartTrial(ctx context.Context, tenantID string, trialEndsAt time.Time) error
	// ExpireTrial fait passer un essai terminé (now >= trial_ends_at) vers FREE/ACTIVE
	// dans une transaction verrouillée, avec un seul événement « trial_expired ».
	// Retourne false, sans erreur, si aucune transition n'est due.
	ExpireTrial(ctx context.Context, tenantID string, now time.Time, actorUserID string) (bool, error)

	// Espace SuperAdmin (éditeur) : indicateurs consolidés et annuaire réel.
	PlatformStats(ctx context.Context) (*PlatformStats, error)
	ListTenantOverviews(ctx context.Context) ([]TenantOverview, error)
	SetSubscriptionStatus(ctx context.Context, tenantID, status, actorUserID, reason string) error
	// DeleteTenant retire définitivement un établissement et toutes ses données.
	DeleteTenant(ctx context.Context, tenantID string) error
}

// ErrTenantSubscriptionNotFound : aucun abonnement pour cet établissement.
var ErrTenantSubscriptionNotFound = errors.New("aucun abonnement pour cet établissement")

// ErrTenantNotFound : aucun établissement ne correspond à l'identifiant fourni.
var ErrTenantNotFound = errors.New("établissement introuvable")

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
		CreatedAt:      now,
	}
	return nil
}
