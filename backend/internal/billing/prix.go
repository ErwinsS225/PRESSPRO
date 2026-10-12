package billing

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
)

// Erreurs de la modification des tarifs (espace éditeur).
var (
	// ErrPlanInconnu : aucune formule ne porte ce code.
	ErrPlanInconnu = errors.New("formule introuvable")
	// ErrCataloguePrixIndisponible : aucun catalogue de prix (mode mémoire). Aucun tarif
	// n'est inventé : la modification est refusée plutôt que simulée.
	ErrCataloguePrixIndisponible = errors.New("catalogue de prix indisponible : base de données non connectée")
	// ErrPrixInvalide : montant ou formule hors règles (entier XOF, formule gratuite, motif trop long).
	ErrPrixInvalide = errors.New("tarif invalide")
)

const (
	// PrixMaximumXOF borne haute d'un tarif mensuel (garde-fou contre une saisie erronée).
	PrixMaximumXOF = 100_000_000
	// motifPrixMaxCaracteres borne le motif enregistré dans le journal des prix.
	motifPrixMaxCaracteres = 500
)

// SetPlanPrice modifie le tarif mensuel d'une formule payante. Le montant doit être un
// entier XOF compris entre 0 et PrixMaximumXOF. La formule gratuite reste à 0.
// Chaque changement effectif est journalisé dans plan_price_history par un trigger SQL
// (ancien et nouveau prix, acteur, motif, date). Les abonnements suivent le catalogue :
// aucun tarif n'est copié sur l'abonnement (décision propriétaire, phase 6).
func (s *Service) SetPlanPrice(ctx context.Context, planCode string, priceXOF float64, actorUserID, reason string) error {
	if s == nil {
		return ErrCataloguePrixIndisponible
	}
	code := strings.ToUpper(strings.TrimSpace(planCode))
	if code == PlanFree {
		return fmt.Errorf("%w : la formule gratuite n'a pas de tarif modifiable", ErrPrixInvalide)
	}
	if math.IsNaN(priceXOF) || math.IsInf(priceXOF, 0) || priceXOF < 0 || priceXOF > PrixMaximumXOF || priceXOF != math.Trunc(priceXOF) {
		return fmt.Errorf("%w : montant entier en FCFA attendu (de 0 à %d)", ErrPrixInvalide, PrixMaximumXOF)
	}
	motif := strings.TrimSpace(reason)
	if len([]rune(motif)) > motifPrixMaxCaracteres {
		return fmt.Errorf("%w : motif trop long (%d caractères au maximum)", ErrPrixInvalide, motifPrixMaxCaracteres)
	}
	if s.store == nil {
		return ErrCataloguePrixIndisponible
	}
	return s.store.SetPlanPrice(ctx, code, priceXOF, strings.TrimSpace(actorUserID), motif)
}
