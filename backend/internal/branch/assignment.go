package branch

import (
	"context"
	"errors"
	"sort"
	"strings"
)

var (
	// ErrEmptyAssignment : un compte employé doit opérer dans au moins un
	// établissement, sinon il serait refusé sur toutes les opérations physiques
	// (le contrôle de périmètre est fail-closed).
	ErrEmptyAssignment = errors.New("au moins un établissement doit être affecté à ce compte")
	// ErrUnknownBranch : l'identifiant ne correspond à aucun établissement actif
	// du pressing (établissement d'un autre pressing inclus).
	ErrUnknownBranch = errors.New("établissement inconnu pour ce pressing")
	// ErrMemberNotFound : le compte visé n'existe pas dans ce pressing.
	ErrMemberNotFound = errors.New("compte introuvable dans cet établissement")
)

// MemberBranch présente un établissement du point de vue d'un compte employé :
// l'affectation (branch_memberships) décide des sites où il peut opérer.
type MemberBranch struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsPrimary bool   `json:"is_primary"`
	Status    string `json:"status"`
	Assigned  bool   `json:"assigned"`
}

// VerificateurMembre indique si le compte appartient au pressing. Injecté par
// l'assemblage (service d'authentification) pour le mode mémoire.
type VerificateurMembre func(ctx context.Context, tenantID, userID string) (bool, error)

// SetMemberVerifier branche le contrôle d'appartenance du mode mémoire (au démarrage).
func (s *Service) SetMemberVerifier(v VerificateurMembre) {
	s.verifier = v
}

// verifierMembreMemoire applique le contrôle fail-closed du mode mémoire : sans
// vérificateur, ou si le compte n'appartient pas au pressing, l'accès est refusé
// comme pour un compte inconnu.
func (s *Service) verifierMembreMemoire(ctx context.Context, tenantID, userID string) error {
	if s.verifier == nil {
		return ErrMemberNotFound
	}
	ok, err := s.verifier(ctx, tenantID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrMemberNotFound
	}
	return nil
}

// GetMemberBranches liste les établissements du pressing en indiquant ceux
// auxquels le compte est affecté.
func (s *Service) GetMemberBranches(ctx context.Context, tenantID, userID string) ([]*MemberBranch, error) {
	if tenantID == "" || userID == "" {
		return nil, ErrMemberNotFound
	}
	if s.store != nil {
		return s.store.GetMemberBranches(ctx, tenantID, userID)
	}
	if err := s.verifierMembreMemoire(ctx, tenantID, userID); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.memberBranchesLocked(tenantID, userID), nil
}

// memberBranchesLocked construit la vue d'affectation d'un compte ; le verrou du
// service doit être tenu par l'appelant (sync.RWMutex n'est pas réentrant).
func (s *Service) memberBranchesLocked(tenantID, userID string) []*MemberBranch {
	s.ensurePrimaryLocked(tenantID)
	affectations := s.affectationsMemoire(tenantID, userID)
	// Tant qu'aucune affectation n'a été enregistrée, le compte opère sur le
	// site principal ; dès qu'une sélection existe, elle fait foi (un caissier
	// peut donc être cantonné à une annexe).
	aucuneAffectation := len(affectations) == 0
	out := make([]*MemberBranch, 0, len(s.memoire[tenantID]))
	for _, b := range s.memoire[tenantID] {
		if b.Status == StatusArchived {
			continue
		}
		assigned := affectations[b.ID]
		if aucuneAffectation && b.IsPrimary {
			assigned = true
		}
		out = append(out, &MemberBranch{ID: b.ID, Name: b.Name, IsPrimary: b.IsPrimary, Status: b.Status, Assigned: assigned})
	}
	trierSites(out)
	return out
}

// SetMemberBranches remplace l'affectation d'un compte : chaque identifiant
// fourni doit être un établissement actif du pressing, et l'affectation ne peut
// pas être vidée. L'écriture est atomique côté base.
func (s *Service) SetMemberBranches(ctx context.Context, tenantID, userID string, branchIDs []string) ([]*MemberBranch, error) {
	if tenantID == "" || userID == "" {
		return nil, ErrMemberNotFound
	}
	uniques := uniquesNonVides(branchIDs)
	if len(uniques) == 0 {
		return nil, ErrEmptyAssignment
	}
	if s.store != nil {
		return s.store.SetMemberBranches(ctx, tenantID, userID, uniques)
	}
	if err := s.verifierMembreMemoire(ctx, tenantID, userID); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensurePrimaryLocked(tenantID)
	connus := make(map[string]bool, len(s.memoire[tenantID]))
	for _, b := range s.memoire[tenantID] {
		if b.Status != StatusArchived {
			connus[b.ID] = true
		}
	}
	for _, id := range uniques {
		if !connus[id] {
			return nil, ErrUnknownBranch
		}
	}
	if s.affectations == nil {
		s.affectations = make(map[string]map[string]map[string]bool)
	}
	if s.affectations[tenantID] == nil {
		s.affectations[tenantID] = make(map[string]map[string]bool)
	}
	choix := make(map[string]bool, len(uniques))
	for _, id := range uniques {
		choix[id] = true
	}
	s.affectations[tenantID][userID] = choix
	return s.memberBranchesLocked(tenantID, userID), nil
}

// affectationsMemoire retourne l'affectation mémorisée d'un compte (mode démo).
func (s *Service) affectationsMemoire(tenantID, userID string) map[string]bool {
	if s.affectations == nil || s.affectations[tenantID] == nil {
		return nil
	}
	return s.affectations[tenantID][userID]
}

// uniquesNonVides déduplique les identifiants reçus du client.
func uniquesNonVides(ids []string) []string {
	vus := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || vus[id] {
			continue
		}
		vus[id] = true
		out = append(out, id)
	}
	return out
}

// trierSites place l'établissement principal en tête, puis les autres par nom.
func trierSites(sites []*MemberBranch) {
	sort.SliceStable(sites, func(i, j int) bool {
		if sites[i].IsPrimary != sites[j].IsPrimary {
			return sites[i].IsPrimary
		}
		return strings.ToLower(sites[i].Name) < strings.ToLower(sites[j].Name)
	})
}

// CompterEtablissements retourne le nombre d'établissements non archivés du pressing
// (mode mémoire). Alimente le plafond multi-sites en mode dégradé.
func (s *Service) CompterEtablissements(_ context.Context, tenantID string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Le site principal est créé à la demande : il compte dès la première mesure.
	s.ensurePrimaryLocked(tenantID)
	var n int64
	for _, b := range s.memoire[tenantID] {
		if b.Status != StatusArchived {
			n++
		}
	}
	return n, nil
}
