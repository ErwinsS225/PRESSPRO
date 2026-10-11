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

	"github.com/joho/godotenv"

	"pressing-os/backend/internal/auth"
	"pressing-os/backend/internal/billing"
	"pressing-os/backend/internal/catalog"
	"pressing-os/backend/internal/config"
	"pressing-os/backend/internal/customer"
	"pressing-os/backend/internal/database"
	"pressing-os/backend/internal/health"
	apphttp "pressing-os/backend/internal/http"
	"pressing-os/backend/internal/order"
	"pressing-os/backend/internal/pricelist"
)

func main() {
	// Chargement du fichier .env (racine du projet puis backend/) si présent.
	// godotenv.Load n'écrase jamais une variable déjà définie dans l'environnement.
	_ = godotenv.Load("../.env")
	_ = godotenv.Load()

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
	// Authentification : PostgreSQL quand disponible (login/register persistés),
	// sinon repli mémoire (comptes de démonstration) pour ne jamais bloquer le démarrage.
	if dbPool != nil && dbPool.Pool != nil {
		authService.SetStore(auth.NewStore(database.NewStore(dbPool.Pool)))
		logger.Println("Authentification adossée à PostgreSQL (comptes persistés).")
	} else {
		logger.Println("Authentification en mode mémoire (PostgreSQL indisponible au démarrage).")
	}
	authHandler := auth.NewHandler(authService)

	orderService := order.NewService()
	orderHandler := order.NewHandler(orderService)
	// Commandes : PostgreSQL quand disponible, sinon repli mémoire (données seed).
	if dbPool != nil && dbPool.Pool != nil {
		orderService.SetStore(order.NewStore(database.NewStore(dbPool.Pool)))
		logger.Println("Commandes adossées à PostgreSQL (persistance activée).")
	} else {
		logger.Println("Commandes en mode mémoire (PostgreSQL indisponible au démarrage).")
	}
	// Clients : PostgreSQL quand disponible, sinon repli mémoire (mode dégradé).
	customerService := customer.NewService()
	if dbPool != nil && dbPool.Pool != nil {
		customerService.SetStore(customer.NewStore(database.NewStore(dbPool.Pool)))
		logger.Println("Clients adossés à PostgreSQL (persistance activée).")
	} else {
		logger.Println("Clients en mode mémoire (PostgreSQL indisponible au démarrage).")
	}
	customerHandler := customer.NewHandler(customerService)

	// Grilles tarifaires versionnées : PostgreSQL quand disponible, sinon repli mémoire.
	priceListService := pricelist.NewService()
	if dbPool != nil && dbPool.Pool != nil {
		priceListService = pricelist.NewStore(database.NewStore(dbPool.Pool))
		logger.Println("Grilles tarifaires adossées à PostgreSQL (versions persistées).")
	} else {
		logger.Println("Grilles tarifaires en mode mémoire (PostgreSQL indisponible au démarrage).")
	}
	priceListHandler := pricelist.NewHandler(priceListService)

	// Catalogue : PostgreSQL quand disponible, sinon repli mémoire (données seed).
	catalogService := catalog.NewService()
	if dbPool != nil && dbPool.Pool != nil {
		dbStore := database.NewStore(dbPool.Pool)
		catalogService = catalog.NewStore(dbStore)
		logger.Println("Catalogue adossé à PostgreSQL (persistance activée).")
	} else {
		logger.Println("Catalogue en mode mémoire (PostgreSQL indisponible au démarrage).")
	}
	catalogHandler := catalog.NewHandler(catalogService)

	// Moteur d'abonnement : PostgreSQL quand disponible, sinon repli mémoire.
	billingService := billing.NewService()
	if dbPool != nil && dbPool.Pool != nil {
		dbBillingStore := database.NewStore(dbPool.Pool)
		billingService = billing.NewStore(dbBillingStore)
		logger.Println("Billing adossé à PostgreSQL (abonnements persistés).")
	} else {
		logger.Println("Billing en mode mémoire (PostgreSQL indisponible au démarrage).")
	}
	billingHandler := billing.NewHandler(billingService)

	// Garde-fou quota : la 51e commande FREE (essai dépassé) est refusée en 402.
	// Compteur mémoire en mode dégradé, compteur SQL quand PostgreSQL est joignable.
	if dbPool != nil && dbPool.Pool != nil {
		orderHandler.SetQuotaGuard(&billing.QuotaGuard{
			Billing: billingService,
			Counter: billing.NewDBUsageCounter(database.NewStore(dbPool.Pool)),
		})
	} else {
		orderHandler.SetQuotaGuard(&billing.QuotaGuard{Billing: billingService})
	}

	// Moteur d'autorisation — contrôles #2 (appartenance) et #4 (périmètre branche).
	// Pool nil => mode dégradé (vérification assouplie), sans jamais bloquer le démarrage.
	var membershipChecker *apphttp.MembershipChecker
	if dbPool != nil && dbPool.Pool != nil {
		membershipChecker = apphttp.NewMembershipChecker(dbPool.Pool)
		logger.Println("Contrôle d'appartenance et de périmètre branche adossés à PostgreSQL.")
	} else {
		membershipChecker = apphttp.NewMembershipChecker(nil)
		logger.Println("Contrôle d'appartenance en mode dégradé (PostgreSQL indisponible au démarrage).")
	}

	router := apphttp.NewRouter(apphttp.ServerDependencies{
		Config:            cfg,
		HealthHandler:     healthHandler,
		AuthHandler:       authHandler,
		JWTManager:        jwtManager,
		OrderHandler:      orderHandler,
		CatalogHandler:    catalogHandler,
		CustomerHandler:   customerHandler,
		PriceListHandler:  priceListHandler,
		Billing:           billingService,
		BillingHandler:    billingHandler,
		MembershipChecker: membershipChecker,
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
