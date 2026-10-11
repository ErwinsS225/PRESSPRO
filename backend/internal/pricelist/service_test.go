package pricelist

import (
	"context"
	"errors"
	"testing"
)

// TestCreatePriceList_RejectsDuplicateName : deux grilles du même nom pour un
// même établissement sont refusées (contrainte d'unicité en base, contrôle
// équivalent en mémoire) — erreur métier traduite en 409 par le handler.
func TestCreatePriceList_RejectsDuplicateName(t *testing.T) {
	svc := NewService()
	ctx := context.Background()
	const tenant = "tenant-test-pricelist"

	if _, err := svc.CreatePriceList(ctx, tenant, CreatePriceListRequest{Name: "Grille principale"}); err != nil {
		t.Fatalf("première création: %v", err)
	}
	_, err := svc.CreatePriceList(ctx, tenant, CreatePriceListRequest{Name: "  grille PRINCIPALE "})
	if !errors.Is(err, ErrNameExists) {
		t.Fatalf("attendu ErrNameExists (comparaison insensible à la casse et aux espaces), obtenu %v", err)
	}

	// Un autre établissement peut porter le même nom de grille.
	if _, err := svc.CreatePriceList(ctx, "autre-tenant", CreatePriceListRequest{Name: "Grille principale"}); err != nil {
		t.Fatalf("création pour un autre établissement: %v", err)
	}
}

// TestCreatePriceList_RequiresName : le nom est obligatoire.
func TestCreatePriceList_RequiresName(t *testing.T) {
	svc := NewService()
	if _, err := svc.CreatePriceList(context.Background(), "t", CreatePriceListRequest{Name: "   "}); err == nil {
		t.Fatal("un nom vide doit être refusé")
	}
}
