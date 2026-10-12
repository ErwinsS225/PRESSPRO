package billing

import (
	"context"
	"time"
)

// SourcesMemoire fournit, en mode dégradé (sans PostgreSQL), les données que le
// compteur mémoire ne possède pas : utilisateurs, grilles, établissements et
// commandes sont tenus par leurs propres services. Le module billing ne peut pas
// importer order/auth/branch (order dépend déjà de billing) : l'assemblage
// applicatif branche donc ces fonctions au démarrage.
//
// Une source absente compte 0 — sauf DatesCommandes : sans source, aucune commande
// n'est comptée (la quota est alors non mesurée, ce que l'assemblage doit éviter).
type SourcesMemoire struct {
	// Utilisateurs : comptes actifs du pressing.
	Utilisateurs func(ctx context.Context, tenantID string) (int64, error)
	// GrillesTarifaires : toutes les grilles du pressing.
	GrillesTarifaires func(ctx context.Context, tenantID string) (int64, error)
	// Etablissements : établissements non archivés du pressing.
	Etablissements func(ctx context.Context, tenantID string) (int64, error)
	// DatesCommandes : date de création de chaque commande non annulée du pressing.
	DatesCommandes func(ctx context.Context, tenantID string) ([]time.Time, error)
}

// compteurMemoire implémente UsageCounter à partir de SourcesMemoire. Il applique
// les mêmes règles que les compteurs SQL (persist.go) : comptes actifs, grilles
// totales, établissements non archivés, commandes non annulées dans la fenêtre.
type compteurMemoire struct {
	service *Service
	src     SourcesMemoire
}

// NewCompteursMemoire construit le compteur d'usage du mode dégradé. Le service
// donne l'ancre de fenêtre (essai ou création de l'abonnement) en mémoire.
func NewCompteursMemoire(service *Service, src SourcesMemoire) UsageCounter {
	return &compteurMemoire{service: service, src: src}
}

func (c *compteurMemoire) CountOrdersInPeriod(ctx context.Context, tenantID string, now time.Time) (int64, error) {
	if c.src.DatesCommandes == nil {
		return 0, nil
	}
	dates, err := c.src.DatesCommandes(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	ancre := c.service.ancreMemoire(ctx, tenantID)
	if ancre.IsZero() {
		// Ancre inconnue : on compte toutes les commandes non annulées (conservateur).
		return int64(len(dates)), nil
	}
	fenetre := FenetreCommandes(ancre, now)
	var n int64
	for _, d := range dates {
		if !d.Before(fenetre.Start) && d.Before(fenetre.End) {
			n++
		}
	}
	return n, nil
}

func (c *compteurMemoire) CountUsers(ctx context.Context, tenantID string) (int64, error) {
	if c.src.Utilisateurs == nil {
		return 0, nil
	}
	return c.src.Utilisateurs(ctx, tenantID)
}

func (c *compteurMemoire) CountPriceLists(ctx context.Context, tenantID string) (int64, error) {
	if c.src.GrillesTarifaires == nil {
		return 0, nil
	}
	return c.src.GrillesTarifaires(ctx, tenantID)
}

func (c *compteurMemoire) CountBranches(ctx context.Context, tenantID string) (int64, error) {
	if c.src.Etablissements == nil {
		return 0, nil
	}
	return c.src.Etablissements(ctx, tenantID)
}

// ancreMemoire retourne l'ancre de la fenêtre de commandes du tenant (mode mémoire).
// Zéro si l'abonnement n'est pas connu.
func (s *Service) ancreMemoire(_ context.Context, tenantID string) time.Time {
	if s == nil {
		return time.Time{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.subs[tenantID]
	if !ok || rec == nil {
		return time.Time{}
	}
	return ancreFenetre(rec)
}
