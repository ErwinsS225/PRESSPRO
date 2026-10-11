package billing

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
)

// PlatformStats regroupe les indicateurs consolidés de la plateforme
// (espace SuperAdmin / éditeur SaaS). Toutes les valeurs proviennent des données
// réelles : abonnements persistés en base, ou mémoire en mode dégradé.
type PlatformStats struct {
	TotalPressings      int64            `json:"total_registered_pressings"`
	TotalUsers          int64            `json:"total_users"`
	ActiveSubscriptions int64            `json:"active_subscriptions"`
	TrialingCount       int64            `json:"trialing_subscriptions"`
	MRR                 float64          `json:"monthly_recurring_mrr_xof"`
	ByStatus            map[string]int64 `json:"subscriptions_by_status"`
	ByPlan              map[string]int64 `json:"subscriptions_by_plan"`
	// DataSource indique l'origine des chiffres : "postgresql" (données
	// persistées) ou "memoire" (mode démonstration, base non connectée).
	DataSource string `json:"data_source"`
}

// TenantOverview décrit un établissement pour l'annuaire de l'éditeur.
type TenantOverview struct {
	TenantID        string     `json:"tenant_id"`
	Name            string     `json:"name"`
	Slug            string     `json:"slug"`
	City            string     `json:"city"`
	Phone           string     `json:"phone"`
	OwnerEmail      string     `json:"owner_email"`
	PlanCode        string     `json:"plan_code"`
	PlanName        string     `json:"plan_name"`
	PriceMonthlyXOF float64    `json:"price_monthly_xof"`
	Status          string     `json:"status"`
	TrialEndsAt     *time.Time `json:"trial_ends_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	// Actif indique si l'établissement peut opérer (abonnement non suspendu).
	Actif bool `json:"active"`
}

// DeleteTenant supprime un établissement et l'intégralité de ses données :
// comptes, clients, commandes, catalogue, dépenses, abonnements. Réservé à
// l'éditeur (espace SuperAdmin) ; l'opération est irréversible.
//
// Sans base connectée (mode mémoire), l'équivalent est le retrait de
// l'abonnement de démonstration : l'appel refuse explicitement si rien ne
// correspond, plutôt que de laisser croire à une suppression.
func (s *Service) DeleteTenant(ctx context.Context, tenantID string) error {
	if s == nil {
		return ErrTenantNotFound
	}
	if s.store != nil {
		return s.store.DeleteTenant(ctx, tenantID)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.subs[tenantID]; !exists {
		return ErrTenantNotFound
	}
	delete(s.subs, tenantID)
	return nil
}

// AdminStats retourne les indicateurs de la plateforme. En mode persistant, les
// chiffres sont calculés en base ; en mode démonstration (aucune base) ils sont
// dérivés des abonnements en mémoire et signalés comme tels.
func (s *Service) AdminStats(ctx context.Context) (*PlatformStats, error) {
	if s == nil {
		return &PlatformStats{ByStatus: map[string]int64{}, ByPlan: map[string]int64{}, DataSource: "memoire"}, nil
	}
	if s.store != nil {
		stats, err := s.store.PlatformStats(ctx)
		if err != nil {
			return nil, fmt.Errorf("statistiques plateforme: %w", err)
		}
		if stats.ByStatus == nil {
			stats.ByStatus = map[string]int64{}
		}
		if stats.ByPlan == nil {
			stats.ByPlan = map[string]int64{}
		}
		stats.DataSource = "postgresql"
		return stats, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	stats := &PlatformStats{
		ByStatus:   map[string]int64{},
		ByPlan:     map[string]int64{},
		DataSource: "memoire",
	}
	now := time.Now().UTC()
	for _, rec := range s.subs {
		stats.TotalPressings++
		stats.ByStatus[rec.Status]++
		plan := EffectivePlan(rec, now)
		stats.ByPlan[plan]++
		switch strings.ToUpper(rec.Status) {
		case StatusActive:
			stats.ActiveSubscriptions++
			stats.MRR += rec.PriceMonthly
		case StatusTrialing:
			stats.TrialingCount++
		}
	}
	return stats, nil
}

// TenantsOverview retourne l'annuaire des établissements (données réelles).
func (s *Service) TenantsOverview(ctx context.Context) ([]TenantOverview, error) {
	if s == nil {
		return []TenantOverview{}, nil
	}
	if s.store != nil {
		tenants, err := s.store.ListTenantOverviews(ctx)
		if err != nil {
			return nil, fmt.Errorf("annuaire des établissements: %w", err)
		}
		if tenants == nil {
			tenants = []TenantOverview{}
		}
		return tenants, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]TenantOverview, 0, len(s.subs))
	for tenantID, rec := range s.subs {
		out = append(out, TenantOverview{
			TenantID:    tenantID,
			Slug:        "demo-" + shortID(tenantID),
			PlanCode:    EffectivePlan(rec, time.Now().UTC()),
			PlanName:    rec.PlanName,
			Status:      rec.Status,
			TrialEndsAt: rec.TrialEndsAt,
			Actif:       !strings.EqualFold(rec.Status, StatusSuspended),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TenantID < out[j].TenantID })
	return out, nil
}

// SetTenantStatus suspend ou réactive l'abonnement d'un établissement
// (décision de l'éditeur : impayé, litige, fermeture). Journalisé dans
// subscription_events en mode persistant.
func (s *Service) SetTenantStatus(ctx context.Context, tenantID, status, actorUserID, reason string) error {
	if s == nil {
		return fmt.Errorf("moteur de facturation indisponible")
	}
	if tenantID == "" {
		return fmt.Errorf("l'identifiant de l'établissement est requis")
	}
	switch strings.ToUpper(status) {
	case StatusSuspended, StatusActive, StatusPastDue, StatusCanceled:
	default:
		return fmt.Errorf("statut d'abonnement invalide : %s", status)
	}
	status = strings.ToUpper(status)

	if s.store != nil {
		rec, err := s.store.GetSubscription(ctx, tenantID)
		if err != nil {
			return err
		}
		if rec == nil {
			return ErrTenantSubscriptionNotFound
		}
		return s.store.SetSubscriptionStatus(ctx, tenantID, status, actorUserID, reason)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.subs[tenantID]
	if !ok {
		return ErrTenantSubscriptionNotFound
	}
	rec.Status = status
	return nil
}

// IsTenantSuspended indique si l'abonnement de l'établissement est suspendu.
// En cas d'erreur de lecture, l'état « non suspendu » est retourné : le contrôle
// d'accès ne doit jamais transformer une panne transitoire en blocage général.
func (s *Service) IsTenantSuspended(ctx context.Context, tenantID string) bool {
	if s == nil || tenantID == "" {
		return false
	}
	if s.store != nil {
		rec, err := s.store.GetSubscription(ctx, tenantID)
		if err != nil {
			log.Printf("[billing] état d'abonnement illisible pour le tenant %s: %v", tenantID, err)
			return false
		}
		if rec == nil {
			return false
		}
		return strings.EqualFold(rec.Status, StatusSuspended)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.subs[tenantID]
	return ok && strings.EqualFold(rec.Status, StatusSuspended)
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
