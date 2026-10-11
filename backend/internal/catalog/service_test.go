package catalog_test

import (
	"context"
	"testing"

	"pressing-os/backend/internal/catalog"
)

func TestCatalogCRUDAndIsolation(t *testing.T) {
	svc := catalog.NewService()
	ctx := context.Background()
	tenantA := "tenant-cocody"
	tenantB := "tenant-plateau"

	// 1. Création d'une nouvelle prestation pour Cocody
	item, err := svc.CreateService(ctx, tenantA, catalog.CreateServiceRequest{
		Name:           "Veste en Daim / Cuir",
		Category:       catalog.CategoryDelicate,
		UnitPrice:      6500,
		Unit:           "pièce",
		EstimatedHours: 72,
	})
	if err != nil {
		t.Fatalf("échec création prestation: %v", err)
	}

	if item.Name != "Veste en Daim / Cuir" || item.UnitPrice != 6500 {
		t.Errorf("attributs invalides: %+v", item)
	}

	// 2. Isolation : Tenant B ne peut pas consulter ni modifier la prestation de Tenant A
	_, err = svc.GetServiceByID(ctx, tenantB, item.ID)
	if err != catalog.ErrServiceNotFound {
		t.Errorf("l'isolation a échoué: tenant B a pu accéder à l'item de tenant A")
	}

	_, err = svc.UpdateService(ctx, tenantB, item.ID, catalog.UpdateServiceRequest{
		Name:      "Piratage Prix",
		UnitPrice: 100,
	})
	if err != catalog.ErrServiceNotFound {
		t.Errorf("l'isolation a échoué sur la mise à jour par un autre tenant")
	}

	// 3. Modification légitime par Tenant A
	updated, err := svc.UpdateService(ctx, tenantA, item.ID, catalog.UpdateServiceRequest{
		Name:           "Veste en Daim / Cuir Premium",
		Category:       catalog.CategoryDelicate,
		UnitPrice:      7000,
		Unit:           "pièce",
		EstimatedHours: 72,
	})
	if err != nil {
		t.Fatalf("échec mise à jour: %v", err)
	}
	if updated.UnitPrice != 7000 || updated.Name != "Veste en Daim / Cuir Premium" {
		t.Errorf("mise à jour incorrecte: %+v", updated)
	}

	// 4. Suppression par Tenant A
	if err := svc.DeleteService(ctx, tenantA, item.ID); err != nil {
		t.Fatalf("échec suppression: %v", err)
	}

	// Vérification de la disparition
	_, err = svc.GetServiceByID(ctx, tenantA, item.ID)
	if err != catalog.ErrServiceNotFound {
		t.Errorf("l'élément supprimé ne devrait plus exister")
	}

	// 5. Test validation prix négatif
	_, err = svc.CreateService(ctx, tenantA, catalog.CreateServiceRequest{
		Name:      "Article Invalide",
		UnitPrice: -500,
	})
	if err != catalog.ErrInvalidPrice {
		t.Errorf("attendait ErrInvalidPrice pour prix négatif, obtenu %v", err)
	}
}
