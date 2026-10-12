package billing

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// QuotaGuard applique le plafond de commandes du plan.
//
// Règles (PLAN §3.4, réponses validées) :
//   - la consommation est comptée sur une fenêtre glissante de OrdersPeriodDays
//     jours, ancrée sur le début de période de l'abonnement (voir requeteFenetre
//     dans persist.go) ;
//   - les commandes annulées (status = 'cancelled') ne consomment pas le quota ;
//   - le plafond provient des droits effectifs du plan, jamais d'une constante ;
//   - le contrôle est fait par le backend, dans la transaction de création, et
//     un échec de création (rollback) ne consomme aucune place.
type QuotaGuard struct {
	Billing *Service
	Counter UsageCounter
}

// PlafondCommandes est le plafond de commandes résolu depuis les droits effectifs
// du plan. Il est lu HORS transaction : une transaction de création ne doit jamais
// attendre une seconde connexion du pool.
type PlafondCommandes struct {
	enabled bool
	limit   *int64
}

// txOrderCounter est implémenté par le compteur SQL : le comptage se fait alors
// dans la transaction de création, après acquisition du verrou.
type txOrderCounter interface {
	CountOrdersInPeriodTx(ctx context.Context, tx pgx.Tx, tenantID string, now time.Time) (int64, error)
}

// PlafondCommandes résout le plafond de commandes du plan de l'établissement.
func (g *QuotaGuard) PlafondCommandes(ctx context.Context, tenantID string) (PlafondCommandes, error) {
	if g == nil || g.Billing == nil {
		return PlafondCommandes{enabled: true}, nil
	}
	resp, err := g.Billing.EffectiveEntitlements(ctx, tenantID)
	if err != nil {
		return PlafondCommandes{}, err
	}
	for _, e := range resp.Entitlements {
		if e.Key == FeatureOrdersMonthlyMax {
			return PlafondCommandes{enabled: e.Enabled, limit: e.Limit}, nil
		}
	}
	return PlafondCommandes{}, fmt.Errorf("%w : %s", ErrUnknownFeature, FeatureOrdersMonthlyMax)
}

// verifier applique le plafond à la consommation courante.
func (p PlafondCommandes) verifier(current int64) error {
	return verifierPlafond(FeatureOrdersMonthlyMax, p.enabled, p.limit, current)
}

// VerifierCommande contrôle le plafond hors transaction (mode mémoire, sans
// verrou : mode dégradé uniquement).
func (g *QuotaGuard) VerifierCommande(ctx context.Context, tenantID string, p PlafondCommandes) error {
	if g == nil || g.Billing == nil {
		return nil
	}
	current, err := g.counter().CountOrdersInPeriod(ctx, tenantID, time.Now().UTC())
	if err != nil {
		return err
	}
	return p.verifier(current)
}

// VerifierCommandeTx doit être appelé DANS la transaction de création, avant
// toute insertion. Il prend un verrou consultatif transactionnel propre à
// l'établissement, puis compte la fenêtre courante :
//   - deux créations concurrentes du même établissement sont sérialisées : la
//     seconde voit la commande de la première après le COMMIT de celle-ci ;
//   - les autres établissements ne sont pas bloqués ;
//   - le verrou est libéré au COMMIT ou au ROLLBACK, y compris entre plusieurs
//     instances du backend (le verrou vit dans PostgreSQL).
func (g *QuotaGuard) VerifierCommandeTx(ctx context.Context, tx pgx.Tx, tenantID string, p PlafondCommandes) error {
	if g == nil || g.Billing == nil {
		return nil
	}
	if _, err := tx.Exec(ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, cleVerrouQuotaCommandes(tenantID)); err != nil {
		return fmt.Errorf("verrou du quota de commandes: %w", err)
	}
	now := time.Now().UTC()
	var (
		current int64
		err     error
	)
	counter := g.counter()
	if c, ok := counter.(txOrderCounter); ok {
		current, err = c.CountOrdersInPeriodTx(ctx, tx, tenantID, now)
	} else {
		current, err = counter.CountOrdersInPeriod(ctx, tenantID, now)
	}
	if err != nil {
		return err
	}
	return p.verifier(current)
}

// FenetreCommandes calcule la fenêtre de 30 jours courante ancrée sur ancre
// (début d'essai ou création de l'abonnement). Même règle que requeteFenetre :
// début = ancre + k × 30 j, avec k = max(0, floor((now − ancre) / 30 j)).
func FenetreCommandes(ancre, now time.Time) Periode {
	duree := time.Duration(OrdersPeriodDays) * 24 * time.Hour
	var k int64
	if now.After(ancre) {
		k = int64(now.Sub(ancre) / duree)
	}
	debut := ancre.Add(time.Duration(k) * duree)
	return Periode{Start: debut, End: debut.Add(duree)}
}

// ancreFenetre renvoie l'ancre de la fenêtre de commandes (zéro si inconnue).
func ancreFenetre(rec *SubscriptionRecord) time.Time {
	if rec == nil {
		return time.Time{}
	}
	if rec.TrialStartedAt != nil {
		return *rec.TrialStartedAt
	}
	return rec.CreatedAt
}

// counter retourne le compteur branché, ou le compteur mémoire nul par défaut.
func (g *QuotaGuard) counter() UsageCounter {
	if g.Counter != nil {
		return g.Counter
	}
	if g.Billing != nil && g.Billing.counters != nil {
		return g.Billing.counters
	}
	return memoryCounter{}
}

// cleVerrouQuotaCommandes : espace de noms du verrou consultatif (évite toute
// collision avec d'autres verrous applicatifs).
func cleVerrouQuotaCommandes(tenantID string) string {
	return "quota-commandes:" + tenantID
}

// verifierPlafond applique une règle de droit : fonctionnalité désactivée, puis
// plafond atteint. Partagé par CheckLimit et le contrôle des commandes.
func verifierPlafond(key string, enabled bool, limit *int64, current int64) error {
	if !enabled {
		return &refusMetier{cause: ErrFeatureNotIncluded,
			message: fmt.Sprintf("fonctionnalité %s non incluse dans votre formule", key)}
	}
	if limit != nil && current >= *limit {
		return &refusMetier{cause: ErrLimitReached,
			message: fmt.Sprintf("plafond atteint (%d) — passez à une formule supérieure", *limit)}
	}
	return nil
}
