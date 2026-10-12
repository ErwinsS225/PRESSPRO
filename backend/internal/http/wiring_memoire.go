package http

import (
	"pressing-os/backend/internal/auth"
	"pressing-os/backend/internal/billing"
	"pressing-os/backend/internal/branch"
	"pressing-os/backend/internal/order"
	"pressing-os/backend/internal/pricelist"
)

// BrancherMemoire relie, en mode dégradé (sans PostgreSQL), les compteurs de quota
// aux services qui détiennent les données, et le contrôle d'appartenance des
// établissements au service d'authentification. Point d'assemblage unique, utilisé
// par cmd/api et par les tests HTTP en mémoire : sans ce branchement, le mode
// dégradé ne mesurerait ni les utilisateurs, ni les grilles, ni les sites, ni les
// commandes, et les affectations de comptes seraient refusées (fail-closed).
func BrancherMemoire(
	bill *billing.Service,
	authSvc *auth.Service,
	prices *pricelist.Service,
	branches *branch.Service,
	orders *order.Service,
) {
	branches.SetMemberVerifier(authSvc.UtilisateurDansTenant)
	bill.SetUsageCounter(billing.NewCompteursMemoire(bill, billing.SourcesMemoire{
		Utilisateurs:      authSvc.CompterUtilisateursActifs,
		GrillesTarifaires: prices.CompterGrilles,
		Etablissements:    branches.CompterEtablissements,
		DatesCommandes:    orders.DatesCommandesNonAnnulees,
	}))
}
