package auth_test

import (
	"testing"
	"time"

	"pressing-os/backend/internal/auth"
)

func TestJWTGenerationAndVerification(t *testing.T) {
	jwtManager := auth.NewJWTManager("test-super-secret-key-1234567890", 2*time.Hour)

	user := &auth.User{
		ID:       "usr-001",
		TenantID: "tenant-cocody-001",
		Email:    "gerant@pressing-ivoire.ci",
		FullName: "M. Bamba",
		Role:     auth.RoleOwner,
	}

	token, err := jwtManager.Generate(user, "Pressing Ivoire Cocody")
	if err != nil {
		t.Fatalf("échec de génération du jeton: %v", err)
	}

	if token == "" {
		t.Fatal("le jeton généré ne doit pas être vide")
	}

	// Vérification du jeton valide
	claims, err := jwtManager.Verify(token)
	if err != nil {
		t.Fatalf("échec de vérification du jeton: %v", err)
	}

	if claims.UserID != user.ID {
		t.Errorf("attendu UserID %s, obtenu %s", user.ID, claims.UserID)
	}
	if claims.TenantID != user.TenantID {
		t.Errorf("attendu TenantID %s, obtenu %s", user.TenantID, claims.TenantID)
	}
	if claims.Role != auth.RoleOwner {
		t.Errorf("attendu rôle %s, obtenu %s", auth.RoleOwner, claims.Role)
	}
}

func TestJWTExpiredToken(t *testing.T) {
	// Jeton expirant immédiatement (durée négative)
	jwtManager := auth.NewJWTManager("test-key", -1*time.Minute)

	user := &auth.User{
		ID:       "usr-002",
		TenantID: "tenant-cocody-001",
		Email:    "expired@test.ci",
		Role:     auth.RoleCashier,
	}

	token, err := jwtManager.Generate(user, "Pressing Ivoire")
	if err != nil {
		t.Fatalf("erreur génération: %v", err)
	}

	_, err = jwtManager.Verify(token)
	if err != auth.ErrExpiredToken {
		t.Errorf("attendait ErrExpiredToken, obtenu %v", err)
	}
}

func TestJWTInvalidSignature(t *testing.T) {
	manager1 := auth.NewJWTManager("secret-un", 1*time.Hour)
	manager2 := auth.NewJWTManager("secret-deux-different", 1*time.Hour)

	user := &auth.User{ID: "usr-003", Email: "fake@test.ci", Role: auth.RoleOperator}
	token, _ := manager1.Generate(user, "Pressing")

	_, err := manager2.Verify(token)
	if err != auth.ErrInvalidToken {
		t.Errorf("attendait ErrInvalidToken pour signature erronée, obtenu %v", err)
	}
}
