package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pressing-os/backend/internal/auth"
	"pressing-os/backend/internal/config"
	"pressing-os/backend/internal/database"
	"pressing-os/backend/internal/health"
	apphttp "pressing-os/backend/internal/http"
)

func main() {
	// Initialisation du logger structuré standard
	logger := log.New(os.Stdout, "[PressingOS-API] ", log.LstdFlags|log.Lmsgprefix)
	logger.Println("Démarrage du socle backend PressingOS (v0.2.0-auth)...")

	// 1. Chargement et validation de la configuration
	cfg, err := config.LoadFromEnv()
	if err != nil {
		logger.Fatalf("Erreur critique de configuration: %v", err)
	}

	logger.Printf("Configuration chargée. Environnement: %s, Port: %s", cfg.ServerEnv, cfg.Port)
	logger.Printf("Cible base de données: %s", cfg.RedactedDatabaseURL())

	// 2. Initialisation de la connexion PostgreSQL (avec tentative de connexion non-bloquante au démarrage)
	initCtx, initCancel := context.WithTimeout(context.Background(), cfg.DBConnectionTimeout)
	defer initCancel()

	dbPool, err := database.ConnectPool(initCtx, cfg.DatabaseURL)
	if err != nil {
		logger.Printf("Avertissement: Connexion immédiate à PostgreSQL échouée (%v). L'API démarre en mode dégradé (/ready indiquera 503).", err)
	} else {
		if pingErr := dbPool.Ping(initCtx); pingErr != nil {
			logger.Printf("Avertissement: Ping PostgreSQL échoué (%v). /ready indiquera 503 jusqu'à rétablissement.", pingErr)
		} else {
			logger.Println("Connexion PostgreSQL établie avec succès via pgxpool.")
		}
	}

	// 3. Construction des services métier et d'authentification
	var pinger database.Pinger
	if dbPool != nil {
		pinger = dbPool
		defer dbPool.Close()
	}

	healthHandler := health.NewHandler(pinger, cfg.DBConnectionTimeout)

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "pressingos-dev-secret-key-32-bytes-minimum"
	}
	jwtManager := auth.NewJWTManager(jwtSecret, 24*time.Hour)
	authService := auth.NewService(jwtManager)
	authHandler := auth.NewHandler(authService)

	router := apphttp.NewRouter(apphttp.ServerDependencies{
		Config:        cfg,
		HealthHandler: healthHandler,
		AuthHandler:   authHandler,
		JWTManager:    jwtManager,
	})

	// 4. Configuration du serveur HTTP avec timeouts stricts
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	// 5. Gestion de l'arrêt gracieux (Graceful Shutdown)
	shutdownChan := make(chan os.Signal, 1)
	signal.Notify(shutdownChan, os.Interrupt, syscall.SIGTERM)

	serverErrors := make(chan error, 1)
	go func() {
		logger.Printf("Serveur HTTP à l'écoute sur http://0.0.0.0:%s", cfg.Port)
		if listenErr := server.ListenAndServe(); listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			serverErrors <- listenErr
		}
	}()

	// 6. Attente d'un signal d'arrêt ou d'une erreur
	select {
	case err := <-serverErrors:
		logger.Fatalf("Erreur fatale du serveur HTTP: %v", err)

	case sig := <-shutdownChan:
		logger.Printf("Signal d'arrêt reçu (%s). Début de la fermeture gracieuse...", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if shutdownErr := server.Shutdown(ctx); shutdownErr != nil {
			logger.Printf("Arrêt forcé suite à une erreur: %v", shutdownErr)
			_ = server.Close()
		}

		logger.Println("Serveur arrêté proprement. Toutes les connexions sont libérées.")
	}
}
