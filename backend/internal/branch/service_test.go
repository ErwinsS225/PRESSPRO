package branch_test

import (
	"context"
	"errors"
	"testing"

	"pressing-os/backend/internal/branch"
)

const tenant = "a0000000-0000-0000-0000-000000000001"

// service : pressing en mémoire avec un vérificateur d'appartenance permissif
// (tout compte non vide appartient au pressing). Les cas de refus sont testés à part.
func service(t *testing.T) *branch.Service {
	t.Helper()
	svc := branch.NewService()
	svc.SetMemberVerifier(func(_ context.Context, _, userID string) (bool, error) {
		return userID != "", nil
	})
	return svc
}

// TestAffectation_CompteInconnuRefuse : en mode mémoire, un compte inconnu du
// pressing (ou sans vérificateur) ne peut ni être consulté ni être affecté.
func TestAffectation_CompteInconnuRefuse(t *testing.T) {
	ctx := context.Background()
	const inconnu = "user-autre-pressing-999"

	// Vérificateur qui rejette le compte : refus sur lecture et écriture.
	svc := branch.NewService()
	svc.SetMemberVerifier(func(_ context.Context, _, userID string) (bool, error) {
		return userID != inconnu, nil
	})
	principal, err := svc.GetBranches(ctx, tenant)
	if err != nil || len(principal) == 0 {
		t.Fatalf("établissement principal attendu: %v", err)
	}
	if _, err := svc.GetMemberBranches(ctx, tenant, inconnu); !errors.Is(err, branch.ErrMemberNotFound) {
		t.Errorf("lecture d'un compte inconnu : attendu ErrMemberNotFound, obtenu %v", err)
	}
	if _, err := svc.SetMemberBranches(ctx, tenant, inconnu, []string{principal[0].ID}); !errors.Is(err, branch.ErrMemberNotFound) {
		t.Errorf("affectation d'un compte inconnu : attendu ErrMemberNotFound, obtenu %v", err)
	}

	// Sans vérificateur : fail-closed, aucun compte n'est accepté.
	sansVerif := branch.NewService()
	if _, err := sansVerif.GetMemberBranches(ctx, tenant, "user-caissier-001"); !errors.Is(err, branch.ErrMemberNotFound) {
		t.Errorf("sans vérificateur : attendu ErrMemberNotFound, obtenu %v", err)
	}

	// Erreur du vérificateur : elle remonte, sans affectation.
	erreur := errors.New("base indisponible")
	defaillant := branch.NewService()
	defaillant.SetMemberVerifier(func(_ context.Context, _, _ string) (bool, error) {
		return false, erreur
	})
	if _, err := defaillant.SetMemberBranches(ctx, tenant, "user-caissier-001", []string{principal[0].ID}); !errors.Is(err, erreur) {
		t.Errorf("erreur du vérificateur : attendu %v, obtenu %v", erreur, err)
	}
}

// TestGetBranches_PrincipalSeed : un pressing possède toujours au moins son
// établissement principal, y compris en mode mémoire.
func TestGetBranches_PrincipalSeed(t *testing.T) {
	ctx := context.Background()
	svc := service(t)

	liste, err := svc.GetBranches(ctx, tenant)
	if err != nil {
		t.Fatalf("liste des établissements: %v", err)
	}
	if len(liste) != 1 || !liste[0].IsPrimary {
		t.Fatalf("un établissement principal était attendu, obtenu %+v", liste)
	}
	if liste[0].Name != "Établissement principal" {
		t.Errorf("nom attendu « Établissement principal », obtenu %q", liste[0].Name)
	}
	if liste[0].Status != branch.StatusActive {
		t.Errorf("statut attendu actif, obtenu %q", liste[0].Status)
	}
}

// TestCreateBranch : création d'un site secondaire, unicité du nom par pressing.
func TestCreateBranch(t *testing.T) {
	ctx := context.Background()
	svc := service(t)

	second, err := svc.CreateBranch(ctx, tenant, branch.CreateBranchRequest{
		Name: "  Pressing Yopougon  ", City: "Abidjan", Phone: "+225 07 00 00 00 00",
	})
	if err != nil {
		t.Fatalf("création d'un site secondaire: %v", err)
	}
	if second.Name != "Pressing Yopougon" {
		t.Errorf("nom normalisé attendu, obtenu %q", second.Name)
	}
	if second.IsPrimary {
		t.Error("un site créé après l'inscription n'est jamais principal")
	}
	if second.Status != branch.StatusActive {
		t.Errorf("statut attendu actif, obtenu %q", second.Status)
	}

	// Unicité : la casse et les espaces ne permettent pas de dupliquer un nom.
	if _, err := svc.CreateBranch(ctx, tenant, branch.CreateBranchRequest{Name: "pressing  yopougon"}); !errors.Is(err, branch.ErrNameExists) {
		t.Errorf("attendu ErrNameExists, obtenu %v", err)
	}
	// Nom vide refusé.
	if _, err := svc.CreateBranch(ctx, tenant, branch.CreateBranchRequest{Name: "   "}); !errors.Is(err, branch.ErrEmptyName) {
		t.Errorf("attendu ErrEmptyName, obtenu %v", err)
	}

	// Un autre pressing peut porter le même nom de site (isolation).
	autre, err := svc.CreateBranch(ctx, "b62709cf-0fbf-4db2-aa56-d861b7930284", branch.CreateBranchRequest{Name: "Pressing Yopougon"})
	if err != nil || autre.ID == second.ID {
		t.Errorf("les établissements de pressings différents sont indépendants: %v", err)
	}
}

// TestUpdateBranch_Protections : le site principal est protégé, les statuts
// sont validés et les identifiants d'un autre pressing ignorés.
func TestUpdateBranch_Protections(t *testing.T) {
	ctx := context.Background()
	svc := service(t)
	principal := "branch-principal-" + tenant

	// 1. Suspension du site principal refusée.
	suspendu := branch.StatusSuspended
	if _, err := svc.UpdateBranch(ctx, tenant, principal, branch.UpdateBranchRequest{Status: &suspendu}); !errors.Is(err, branch.ErrPrimaryProtected) {
		t.Errorf("attendu ErrPrimaryProtected, obtenu %v", err)
	}

	// 2. Statut hors vocabulaire refusé.
	inconnu := "supprime"
	if _, err := svc.UpdateBranch(ctx, tenant, principal, branch.UpdateBranchRequest{Status: &inconnu}); !errors.Is(err, branch.ErrInvalidStatus) {
		t.Errorf("attendu ErrInvalidStatus, obtenu %v", err)
	}

	// 3. Renommage du principal autorisé, ville mise à jour sur un site secondaire.
	nouveauNom := "Pressing Cocody (siège)"
	principalMisAJour, err := svc.UpdateBranch(ctx, tenant, principal, branch.UpdateBranchRequest{Name: &nouveauNom})
	if err != nil || principalMisAJour.Name != nouveauNom {
		t.Fatalf("renommage du site principal: %v (%+v)", err, principalMisAJour)
	}

	second, err := svc.CreateBranch(ctx, tenant, branch.CreateBranchRequest{Name: "Pressing Treichville"})
	if err != nil {
		t.Fatalf("création secondaire: %v", err)
	}
	archived := branch.StatusArchived
	misAJour, err := svc.UpdateBranch(ctx, tenant, second.ID, branch.UpdateBranchRequest{Status: &archived})
	if err != nil || misAJour.Status != branch.StatusArchived {
		t.Fatalf("archivage d'un site secondaire: %v (%+v)", err, misAJour)
	}

	// 4. Identifiant inconnu (ou d'un autre pressing) : 404 métier.
	if _, err := svc.UpdateBranch(ctx, tenant, "branch-inconnu", branch.UpdateBranchRequest{}); !errors.Is(err, branch.ErrBranchNotFound) {
		t.Errorf("attendu ErrBranchNotFound, obtenu %v", err)
	}
}

// TestAffectation_MembreAuxEtablissements : l'affectation décide des sites où
// un employé peut opérer. Le site principal est affecté par défaut, la
// sélection remplace l'existant, et les cas limites sont refusés.
func TestAffectation_MembreAuxEtablissements(t *testing.T) {
	ctx := context.Background()
	svc := service(t)
	const compte = "user-caissier-001"

	// 1. Sans affectation explicite : le site principal est affecté d'office.
	sites, err := svc.GetMemberBranches(ctx, tenant, compte)
	if err != nil {
		t.Fatalf("affectation initiale: %v", err)
	}
	if len(sites) != 1 || !sites[0].IsPrimary || !sites[0].Assigned {
		t.Fatalf("le site principal doit être affecté par défaut, obtenu %+v", sites)
	}

	// 2. Un second site existe : il n'est pas affecté au caissier.
	second, err := svc.CreateBranch(ctx, tenant, branch.CreateBranchRequest{Name: "Site Yopougon"})
	if err != nil {
		t.Fatalf("création du site secondaire: %v", err)
	}
	sites, err = svc.GetMemberBranches(ctx, tenant, compte)
	if err != nil {
		t.Fatalf("affectation après création: %v", err)
	}
	if len(sites) != 2 || sites[1].ID != second.ID || sites[1].Assigned {
		t.Fatalf("le nouveau site ne doit pas être affecté d'office, obtenu %+v", sites)
	}

	// 3. Affectation exclusive au second site : l'ordre d'affichage place le
	//    principal en tête mais il n'est plus affecté.
	sites, err = svc.SetMemberBranches(ctx, tenant, compte, []string{second.ID, second.ID, " "})
	if err != nil {
		t.Fatalf("affectation au site secondaire: %v", err)
	}
	if len(sites) != 2 {
		t.Fatalf("deux sites attendus, obtenu %+v", sites)
	}
	for _, s := range sites {
		if s.ID == second.ID && !s.Assigned {
			t.Error("le site secondaire doit être affecté")
		}
		if s.IsPrimary && s.Assigned {
			t.Error("le site principal ne doit plus être affecté après remplacement")
		}
	}

	// 4. Cas limites : sélection vide et site inconnu.
	if _, err := svc.SetMemberBranches(ctx, tenant, compte, []string{"", "   "}); !errors.Is(err, branch.ErrEmptyAssignment) {
		t.Errorf("sélection vide : attendu ErrEmptyAssignment, obtenu %v", err)
	}
	if _, err := svc.SetMemberBranches(ctx, tenant, compte, []string{"branch-inexistant"}); !errors.Is(err, branch.ErrUnknownBranch) {
		t.Errorf("site inconnu : attendu ErrUnknownBranch, obtenu %v", err)
	}
	if _, err := svc.SetMemberBranches(ctx, tenant, "", []string{second.ID}); !errors.Is(err, branch.ErrMemberNotFound) {
		t.Errorf("compte vide : attendu ErrMemberNotFound, obtenu %v", err)
	}

	// 5. Isolation : l'affectation d'un compte ne touche pas celle d'un autre.
	autres, err := svc.GetMemberBranches(ctx, tenant, "user-caissier-002")
	if err != nil {
		t.Fatalf("affectation du second compte: %v", err)
	}
	if len(autres) != 2 || !autres[0].Assigned || autres[1].Assigned {
		t.Fatalf("l'affectation des autres comptes doit rester intacte, obtenu %+v", autres)
	}

	// 6. Un établissement archivé disparaît de la liste (et n'est plus affectable).
	if _, err := svc.UpdateBranch(ctx, tenant, second.ID, branch.UpdateBranchRequest{Status: ptr("archived")}); err != nil {
		t.Fatalf("archivage du site secondaire: %v", err)
	}
	sites, err = svc.GetMemberBranches(ctx, tenant, compte)
	if err != nil {
		t.Fatalf("affectation après archivage: %v", err)
	}
	if len(sites) != 1 || sites[0].ID == second.ID {
		t.Fatalf("un site archivé ne doit plus être proposé, obtenu %+v", sites)
	}
}

func ptr[T any](v T) *T { return &v }
