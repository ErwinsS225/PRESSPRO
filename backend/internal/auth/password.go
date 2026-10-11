package auth

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword génère un hash sécurisé bcrypt à partir d'un mot de passe en clair
func HashPassword(password string) (string, error) {
	if len(password) < 6 {
		return "", fmt.Errorf("le mot de passe doit comporter au moins 6 caractères")
	}
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("échec du hashage du mot de passe: %w", err)
	}
	return string(bytes), nil
}

// CheckPassword vérifie si un mot de passe en clair correspond au hash stocké
func CheckPassword(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
