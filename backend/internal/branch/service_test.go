package branch_test

import (
	"context"
	"errors"
	"testing"

	"pressing-os/backend/internal/branch"
)

const tenant = "a0000000-0000-0000-0000-000000000001"

func service(t *testing.T) *branch.Service {
	t.Helper()
	return branch.NewService()
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
