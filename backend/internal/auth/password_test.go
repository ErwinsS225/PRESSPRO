package auth_test

import (
	"testing"

	"pressing-os/backend/internal/auth"
)

func TestPasswordHashing(t *testing.T) {
	password := "SecretP@ssword2026"

	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("erreur lors du hashage: %v", err)
	}

	if hash == password {
		t.Error("le hash ne doit pas être identique au mot de passe en clair")
	}

	if !auth.CheckPassword(hash, password) {
		t.Error("la vérification du mot de passe valide a échoué")
	}

	if auth.CheckPassword(hash, "MauvaisMotDePasse") {
		t.Error("la vérification d'un mot de passe incorrect devrait échouer")
	}

	// Test mot de passe trop court (< 6 caractères)
	if _, err := auth.HashPassword("123"); err == nil {
		t.Error("attendait une erreur pour mot de passe de moins de 6 caractères")
	}
}
