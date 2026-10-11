package auth_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"pressing-os/backend/internal/auth"
)

func nouveauService(t *testing.T) *auth.Service {
	t.Helper()
	return auth.NewService(auth.NewJWTManager("secret-test-equipe-1234567890", time.Hour))
}

// TestCreateUser_Validation : les comptes employés sont contrôlés avant écriture.
func TestCreateUser_Validation(t *testing.T) {
	ctx := context.Background()
	svc := nouveauService(t)
	const tenant = "a0000000-0000-0000-0000-000000000001"

	// 1. Création nominale : le rôle par défaut est opérateur.
	user, err := svc.CreateUser(ctx, tenant, auth.CreateUserRequest{
		FullName: "Koffi Yao", Email: "  KOFFI.YAO@Pressing-Ivoire.CI ",
		Phone: "+225 05 12 34 56 78", Password: "motdepasse123",
	})
	if err != nil {
		t.Fatalf("création d'un employé: %v", err)
	}
	if user.Email != "koffi.yao@pressing-ivoire.ci" {
		t.Errorf("email normalisé attendu, obtenu %q", user.Email)
	}
	if user.Role != auth.RoleOperator {
		t.Errorf("rôle par défaut attendu opérateur, obtenu %q", user.Role)
	}
	if user.Status != "active" {
		t.Errorf("un nouveau compte doit être actif, obtenu %q", user.Status)
	}

	cas := []struct {
		nom  string
		req  auth.CreateUserRequest
		faut bool // erreur attendue
	}{
		{"email déjà utilisé", auth.CreateUserRequest{FullName: "X", Email: "koffi.yao@pressing-ivoire.ci", Password: "motdepasse123"}, errors.Is(err, nil)},
		{"role superadmin interdit", auth.CreateUserRequest{FullName: "X", Email: "admin2@test.ci", Role: "superadmin", Password: "motdepasse123"}, false},
		{"role inconnu", auth.CreateUserRequest{FullName: "X", Email: "admin3@test.ci", Role: "patron", Password: "motdepasse123"}, false},
		{"mot de passe trop court", auth.CreateUserRequest{FullName: "X", Email: "court@test.ci", Password: "123"}, false},
		{"nom manquant", auth.CreateUserRequest{Email: "sansnom@test.ci", Password: "motdepasse123"}, false},
		{"email invalide", auth.CreateUserRequest{FullName: "X", Email: "pas-un-email", Password: "motdepasse123"}, false},
	}

	for _, c := range cas[1:] {
		t.Run(c.nom, func(t *testing.T) {
			if _, err := svc.CreateUser(ctx, tenant, c.req); err == nil {
				t.Errorf("%s : une erreur était attendue", c.nom)
			}
		})
	}

	// Doublon d'email : refus explicite (409 côté API).
	_, err = svc.CreateUser(ctx, tenant, auth.CreateUserRequest{
		FullName: "Doublon", Email: "koffi.yao@pressing-ivoire.ci", Password: "motdepasse123",
	})
	if !errors.Is(err, auth.ErrEmailAlreadyExists) {
		t.Errorf("attendu ErrEmailAlreadyExists, obtenu %v", err)
	}

	// Le rôle superadmin ne peut pas être attribué par un pressing.
	_, err = svc.CreateUser(ctx, tenant, auth.CreateUserRequest{
		FullName: "Faux admin", Email: "faux-admin@test.ci", Role: auth.RoleSuperAdmin, Password: "motdepasse123",
	})
	if !errors.Is(err, auth.ErrInvalidRole) {
		t.Errorf("attendu ErrInvalidRole, obtenu %v", err)
	}
}

// TestSetUserStatus_Desactivation : un compte désactivé ne peut plus se connecter
// (le jeton ne doit plus être délivré) et les rôles du pressing restent limités.
func TestSetUserStatus_Desactivation(t *testing.T) {
	ctx := context.Background()
	svc := nouveauService(t)
	const tenant = "a0000000-0000-0000-0000-000000000001"

	user, err := svc.CreateUser(ctx, tenant, auth.CreateUserRequest{
		FullName: "Awa Caisse", Email: "awa.caisse@test.ci", Role: auth.RoleCashier, Password: "motdepasse123",
	})
	if err != nil {
		t.Fatalf("création: %v", err)
	}

	// Avant désactivation : connexion autorisée.
	if _, err := svc.Login(ctx, "awa.caisse@test.ci", "motdepasse123"); err != nil {
		t.Fatalf("connexion initiale: %v", err)
	}

	// Désactivation puis reconnexion : refus avec message dédié.
	if _, err := svc.SetUserStatus(ctx, tenant, user.ID, "inactive"); err != nil {
		t.Fatalf("désactivation: %v", err)
	}
	if _, err := svc.Login(ctx, "awa.caisse@test.ci", "motdepasse123"); !errors.Is(err, auth.ErrAccountInactive) {
		t.Errorf("attendu ErrAccountInactive, obtenu %v", err)
	}

	// Réactivation : la connexion est rendue.
	if _, err := svc.SetUserStatus(ctx, tenant, user.ID, "active"); err != nil {
		t.Fatalf("réactivation: %v", err)
	}
	if _, err := svc.Login(ctx, "awa.caisse@test.ci", "motdepasse123"); err != nil {
		t.Errorf("connexion après réactivation: %v", err)
	}

	// Isolation : un autre établissement ne peut pas modifier ce compte.
	if _, err := svc.SetUserStatus(ctx, "tenant-etranger", user.ID, "inactive"); !errors.Is(err, auth.ErrUserNotFound) {
		t.Errorf("attendu ErrUserNotFound pour un autre établissement, obtenu %v", err)
	}

	// Statut hors vocabulaire : refus.
	if _, err := svc.SetUserStatus(ctx, tenant, user.ID, "supprime"); err == nil {
		t.Error("un statut inconnu doit être refusé")
	}
}

// TestLogin_CompteDemoTouoursActif : non-régression du mode vitrine.
func TestLogin_CompteDemoTouoursActif(t *testing.T) {
	svc := nouveauService(t)
	if _, err := svc.Login(context.Background(), "gerant@pressing-ivoire.ci", "pass123"); err != nil {
		t.Fatalf("compte de démonstration: %v", err)
	}
	if _, err := svc.Login(context.Background(), " gerant@pressing-ivoire.ci ", "pass123"); err != nil {
		t.Fatalf("email insensible à la casse et aux espaces: %v", err)
	}
	if _, err := svc.Login(context.Background(), "gerant@pressing-ivoire.ci", "faux"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Errorf("mot de passe erroné: attendu ErrInvalidCredentials, obtenu %v", err)
	}
	_ = strings.TrimSpace("")
}
