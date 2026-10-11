package auth_test

import (
	"context"
	"testing"
	"time"

	"pressing-os/backend/internal/auth"
)

func TestServiceLogin(t *testing.T) {
	jwtManager := auth.NewJWTManager("secret-key", 2*time.Hour)
	service := auth.NewService(jwtManager)

	ctx := context.Background()

	// 1. Succès connexion gérant
	resp, err := service.Login(ctx, "gerant@pressing-ivoire.ci", "pass123")
	if err != nil {
		t.Fatalf("échec de connexion avec identifiants valides: %v", err)
	}

	if resp.Token == "" {
		t.Error("le jeton ne doit pas être vide")
	}
	if resp.User.Role != auth.RoleOwner {
		t.Errorf("attendu rôle %s, obtenu %s", auth.RoleOwner, resp.User.Role)
	}
	if resp.Tenant == nil || resp.Tenant.Name != "Pressing Ivoire Cocody" {
		t.Errorf("attendu tenant 'Pressing Ivoire Cocody', obtenu %+v", resp.Tenant)
	}

	// 2. Échec mot de passe incorrect
	_, err = service.Login(ctx, "gerant@pressing-ivoire.ci", "mauvais_pass")
	if err != auth.ErrInvalidCredentials {
		t.Errorf("attendait ErrInvalidCredentials, obtenu %v", err)
	}

	// 3. Échec email inexistant
	_, err = service.Login(ctx, "inconnu@test.ci", "pass123")
	if err != auth.ErrInvalidCredentials {
		t.Errorf("attendait ErrInvalidCredentials, obtenu %v", err)
	}
}

func TestServiceRegisterTenant(t *testing.T) {
	jwtManager := auth.NewJWTManager("secret-key", 2*time.Hour)
	service := auth.NewService(jwtManager)

	ctx := context.Background()

	req := auth.RegisterTenantRequest{
		TenantName:    "Pressing du Plateau",
		City:          "Abidjan (Plateau)",
		Phone:         "+225 07 88 99 00 11",
		OwnerFullName: "Kone Amadou",
		OwnerEmail:    "kone@plateau-pressing.ci",
		Password:      "Securite2026",
	}

	resp, err := service.RegisterTenant(ctx, req)
	if err != nil {
		t.Fatalf("erreur lors de l'enregistrement: %v", err)
	}

	if resp.Tenant.Name != "Pressing du Plateau" {
		t.Errorf("attendu tenant 'Pressing du Plateau', obtenu %s", resp.Tenant.Name)
	}
	if resp.User.Role != auth.RoleOwner {
		t.Errorf("attendu rôle owner, obtenu %s", resp.User.Role)
	}

	// Test doublon email
	_, err = service.RegisterTenant(ctx, req)
	if err != auth.ErrEmailAlreadyExists {
		t.Errorf("attendait ErrEmailAlreadyExists pour email dupliqué, obtenu %v", err)
	}
}
