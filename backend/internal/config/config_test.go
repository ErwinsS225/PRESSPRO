package config_test

import (
	"os"
	"strings"
	"testing"

	"pressing-os/backend/internal/config"
)

func TestLoadFromEnvDefaults(t *testing.T) {
	// Effacer les variables spécifiques pour tester les valeurs par défaut
	os.Unsetenv("PORT")
	os.Unsetenv("DATABASE_URL")
	os.Unsetenv("ALLOWED_ORIGINS")

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("échec de chargement de la config: %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("attendu Port 8080, obtenu %s", cfg.Port)
	}

	if cfg.ServerEnv != "development" {
		t.Errorf("attendu ServerEnv development, obtenu %s", cfg.ServerEnv)
	}

	if len(cfg.AllowedOrigins) == 0 {
		t.Errorf("attendu des AllowedOrigins par défaut")
	}

	redacted := cfg.RedactedDatabaseURL()
	if strings.Contains(redacted, "pressing_secret_local_dev_change_me") {
		t.Errorf("le mot de passe ne doit pas apparaître dans RedactedDatabaseURL()")
	}
	if !strings.Contains(redacted, "xxxxx") {
		t.Errorf("RedactedDatabaseURL() doit contenir le masque standard 'xxxxx', obtenu: %s", redacted)
	}
}

func TestConfigValidation(t *testing.T) {
	cfg := &config.Config{
		Port:        "",
		DatabaseURL: "postgres://localhost/test",
	}
	if err := cfg.Validate(); err == nil {
		t.Error("attendait une erreur pour port vide")
	}

	cfg = &config.Config{
		Port:        "8080",
		DatabaseURL: "",
	}
	if err := cfg.Validate(); err == nil {
		t.Error("attendait une erreur pour database URL vide")
	}
}
