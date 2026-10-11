package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config regroupe toutes les options de configuration de l'API PressingOS
type Config struct {
	Port                string
	ServerEnv           string // "development", "staging", "production"
	DatabaseURL         string
	AllowedOrigins      []string
	ReadTimeout         time.Duration
	WriteTimeout        time.Duration
	IdleTimeout         time.Duration
	DBConnectionTimeout time.Duration
}

// LoadFromEnv extrait les variables d'environnement avec des valeurs par défaut sécurisées
func LoadFromEnv() (*Config, error) {
	port := getEnv("PORT", "8080")
	serverEnv := getEnv("SERVER_ENV", "development")

	// Priorité à NEON_DATABASE_URL (base managée Neon) si défini,
	// sinon repli sur DATABASE_URL puis sur les variables PostgreSQL (Docker local).
	dbURL := getEnv("NEON_DATABASE_URL", "")
	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
	}
	if dbURL == "" {
		dbUser := getEnv("POSTGRES_USER", "pressing_app")
		dbPass := getEnv("POSTGRES_PASSWORD", "pressing_secret_local_dev_change_me")
		dbHost := getEnv("POSTGRES_HOST", "localhost")
		dbPort := getEnv("POSTGRES_PORT", "5432")
		dbName := getEnv("POSTGRES_DB", "pressing_os")
		dbURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
			url.QueryEscape(dbUser),
			url.QueryEscape(dbPass),
			dbHost,
			dbPort,
			dbName,
		)
	}

	// Neon impose TLS : on force sslmode=require si l'hôte est *.neon.tech
	// et qu'aucun sslmode n'est déjà précisé dans l'URL.
	if strings.Contains(dbURL, "neon.tech") && !strings.Contains(dbURL, "sslmode=") {
		if strings.Contains(dbURL, "?") {
			dbURL += "&sslmode=require"
		} else {
			dbURL += "?sslmode=require"
		}
	}

	originsRaw := getEnv("ALLOWED_ORIGINS", "http://localhost:3000,http://127.0.0.1:3000")
	var origins []string
	for _, o := range strings.Split(originsRaw, ",") {
		trimmed := strings.TrimSpace(o)
		if trimmed != "" {
			origins = append(origins, trimmed)
		}
	}

	readTimeoutSec := getEnvAsInt("READ_TIMEOUT_SECONDS", 10)
	writeTimeoutSec := getEnvAsInt("WRITE_TIMEOUT_SECONDS", 10)
	idleTimeoutSec := getEnvAsInt("IDLE_TIMEOUT_SECONDS", 60)
	dbConnTimeoutSec := getEnvAsInt("DB_TIMEOUT_SECONDS", 5)

	cfg := &Config{
		Port:                port,
		ServerEnv:           serverEnv,
		DatabaseURL:         dbURL,
		AllowedOrigins:      origins,
		ReadTimeout:         time.Duration(readTimeoutSec) * time.Second,
		WriteTimeout:        time.Duration(writeTimeoutSec) * time.Second,
		IdleTimeout:         time.Duration(idleTimeoutSec) * time.Second,
		DBConnectionTimeout: time.Duration(dbConnTimeoutSec) * time.Second,
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration invalide: %w", err)
	}

	return cfg, nil
}

// Validate s'assure que les paramètres obligatoires sont conformes
func (c *Config) Validate() error {
	if c.Port == "" {
		return fmt.Errorf("le port HTTP ne peut pas être vide")
	}
	if c.DatabaseURL == "" {
		return fmt.Errorf("l'URL de base de données ne peut pas être vide")
	}
	return nil
}

// RedactedDatabaseURL masque le mot de passe pour les logs sécurisés
func (c *Config) RedactedDatabaseURL() string {
	parsed, err := url.Parse(c.DatabaseURL)
	if err != nil {
		return "[URL MASQUÉE]"
	}
	return parsed.Redacted()
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	if valStr, ok := os.LookupEnv(key); ok {
		if val, err := strconv.Atoi(valStr); err == nil {
			return val
		}
	}
	return defaultVal
}
