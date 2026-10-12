package billing

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// Tests unitaires du cycle de vie en mode mémoire (repli sans PostgreSQL).
// Ils caractérisent les règles du PLAN (essai idempotent, expiration persistée
// côté serveur, erreurs explicites). Ceux qui échouent avant la phase 2
// reproduisent un défaut documenté dans docs/subscription-phase0-baseline.md.

// essaiExpire installe dans le moteur mémoire un essai dont la fin est dépassée.
func essaiExpire(s *Service, tenant string) {
	debut := time.Now().UTC().Add(-8 * 24 * time.Hour)
	fin := time.Now().UTC().Add(-1 * time.Hour)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subs[tenant] = &SubscriptionRecord{
		PlanCode:       PlanFree,
		Status:         StatusTrialing,
		TrialStartedAt: &debut,
		TrialEndsAt:    &fin,
		PlanName:       "Gratuit permanent",
	}
}

// Après expiration, l'état effectif et l'état persisté doivent être FREE/ACTIVE,
// sans appel préalable à ExpireTrialIfNeeded par le handler.
func TestLifecycleMemoire_EssaiExpire_PersisteVersFree(t *testing.T) {
	ctx := context.Background()
	s := NewService()
	const tenant = "62000000-0000-4000-8000-000000000001"
	essaiExpire(s, tenant)

	resp, err := s.EffectiveEntitlements(ctx, tenant)
	if err != nil {
		t.Fatalf("EffectiveEntitlements: %v", err)
	}
	if resp.OnTrial || resp.PlanCode != PlanFree {
		t.Fatalf("après expiration : on_trial=%t plan=%s, attendu false/FREE", resp.OnTrial, resp.PlanCode)
	}
	s.mu.RLock()
	rec := s.subs[tenant]
	s.mu.RUnlock()
	if rec.Status != StatusActive {
		t.Errorf("statut persisté = %s, attendu ACTIVE après expiration", rec.Status)
	}
}

// Un nouvel appel d'initialisation après expiration ne redémarre pas d'essai.
func TestLifecycleMemoire_EnsureTrialApresExpiration_NeRedemarrePas(t *testing.T) {
	ctx := context.Background()
	s := NewService()
	const tenant = "62000000-0000-4000-8000-000000000002"
	essaiExpire(s, tenant)
	if _, err := s.ExpireTrialIfNeeded(ctx, tenant, ""); err != nil {
		t.Fatalf("ExpireTrialIfNeeded: %v", err)
	}

	if err := s.EnsureTrial(ctx, tenant); err != nil {
		t.Fatalf("EnsureTrial: %v", err)
	}
	rec, err := s.GetSubscription(ctx, tenant)
	if err != nil {
		t.Fatalf("GetSubscription: %v", err)
	}
	if rec.Status == StatusTrialing {
		t.Errorf("EnsureTrial a redémarré un essai expiré (statut %s)", rec.Status)
	}
	resp, err := s.EffectiveEntitlements(ctx, tenant)
	if err != nil {
		t.Fatalf("EffectiveEntitlements: %v", err)
	}
	if resp.OnTrial || resp.PlanCode != PlanFree {
		t.Errorf("après réinitialisation : on_trial=%t plan=%s, attendu FREE sans essai", resp.OnTrial, resp.PlanCode)
	}
}

// Deux expirations séquentielles : seule la première effectue la transition.
func TestLifecycleMemoire_ExpirationSequentielle_Idempotente(t *testing.T) {
	ctx := context.Background()
	s := NewService()
	const tenant = "62000000-0000-4000-8000-000000000003"
	essaiExpire(s, tenant)

	premier, err := s.ExpireTrialIfNeeded(ctx, tenant, "")
	if err != nil || !premier {
		t.Fatalf("première expiration : transition=%t err=%v, attendu true", premier, err)
	}
	second, err := s.ExpireTrialIfNeeded(ctx, tenant, "")
	if err != nil || second {
		t.Fatalf("seconde expiration : transition=%t err=%v, attendu false", second, err)
	}
}

// Expirations concurrentes : une seule transition effective. Défaut attendu avant
// phase 2 : la lecture de l'état et l'écriture ne sont pas faites sous le même verrou.
func TestLifecycleMemoire_ExpirationConcurrente_UneSeuleTransition(t *testing.T) {
	ctx := context.Background()
	s := NewService()
	const tenant = "62000000-0000-4000-8000-000000000004"
	essaiExpire(s, tenant)

	const parallele = 16
	var (
		mu          sync.Mutex
		transitions int
		depart      sync.WaitGroup
		fin         sync.WaitGroup
	)
	depart.Add(1)
	for i := 0; i < parallele; i++ {
		fin.Add(1)
		go func() {
			defer fin.Done()
			depart.Wait()
			ok, err := s.ExpireTrialIfNeeded(ctx, tenant, "")
			if err != nil {
				t.Errorf("ExpireTrialIfNeeded: %v", err)
				return
			}
			if ok {
				mu.Lock()
				transitions++
				mu.Unlock()
			}
		}()
	}
	depart.Done()
	fin.Wait()

	if transitions != 1 {
		t.Errorf("transitions effectives = %d, attendu exactement 1", transitions)
	}
}

// Une erreur de lecture de la persistance doit remonter jusqu'au moteur : aucun
// droit ne doit être calculé à partir d'un état inconnu.
func TestLifecycleMemoire_ErreurDePersistance_RemonteAuMoteur(t *testing.T) {
	ctx := context.Background()
	s := NewService()
	s.store = storeEnPanne{}

	if _, err := s.EffectiveEntitlements(ctx, "62000000-0000-4000-8000-000000000005"); err == nil {
		t.Errorf("EffectiveEntitlements : erreur de lecture avalée, attendu une erreur")
	}
	if _, err := s.IsEnabled(ctx, "62000000-0000-4000-8000-000000000005", FeatureReportsAdvanced); err == nil {
		t.Errorf("IsEnabled : erreur de lecture avalée, attendu une erreur")
	}
}

// Politique documentée (service.go / admin.go) : en cas d'erreur de lecture, la
// garde de suspension laisse passer l'établissement, afin qu'une panne transitoire
// ne bloque pas tout un pressing. Ce choix est une décision d'exploitation à
// confirmer par le propriétaire du projet ; ce test le fige sans l'approuver.
func TestLifecycleMemoire_GardeDeSuspension_PolitiqueDocumentee(t *testing.T) {
	s := NewService()
	s.store = storeEnPanne{}
	if s.IsTenantSuspended(context.Background(), "62000000-0000-4000-8000-000000000006") {
		t.Errorf("politique documentée : une lecture en erreur ne doit pas suspendre l'établissement")
	}
}

// storeEnPanne simule une persistance injoignable : seules les lectures
// d'abonnement sont utilisées par les tests ci-dessus.
type storeEnPanne struct{ Store }

var errPersistanceEnPanne = errors.New("persistance injoignable (test)")

func (storeEnPanne) GetSubscription(context.Context, string) (*SubscriptionRecord, error) {
	return nil, errPersistanceEnPanne
}
