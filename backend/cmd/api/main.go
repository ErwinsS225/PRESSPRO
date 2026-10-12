package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"pressing-os/backend/internal/auth"
	"pressing-os/backend/internal/billing"
	"pressing-os/backend/internal/branch"
	"pressing-os/backend/internal/catalog"
	"pressing-os/backend/internal/config"
	"pressing-os/backend/internal/customer"
	"pressing-os/backend/internal/database"
	"pressing-os/backend/internal/expense"
	"pressing-os/backend/internal/health"
	apphttp "pressing-os/backend/internal/http"
	"pressing-os/backend/internal/notification"
	"pressing-os/backend/internal/order"
	"pressing-os/backend/internal/pricelist"
	"pressing-os/backend/internal/ratelimit"
	"pressing-os/backend/internal/report"
)

func main() {
	// Chargement du fichier .env (racine du projet puis backend/) si présent.
	// godotenv.Load n'écrase jamais une variable déjà définie dans l'environnement.
	_ = godotenv.Load("../.env")
	_ = godotenv.Load()

	// Initialisation du logger structuré standard
	logger := log.New(os.Stdout, "[PressingOS-API] ", log.LstdFlags|log.Lmsgprefix)
	logger.Println("Démarrage du socle backend PressingOS (v0.5.0-hardening)...")

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
	}

	// La création du pool ne prouve PAS que la base répond : la décision du mode
	// de persistance est prise sur le résultat du Ping, sinon aucune requête ne
	// pourra aboutir (les services ne doivent jamais être adossés à une base
	// injoignable — ils basculent alors sur leur repli mémoire).
	var pinger database.Pinger
	if dbPool != nil {
		pinger = dbPool
		defer dbPool.Close()
	}

	dbReady := false
	if dbPool != nil && dbPool.Pool != nil {
		if pingErr := dbPool.Ping(initCtx); pingErr != nil {
			logger.Printf("Avertissement: Ping PostgreSQL échoué (%v). L'API bascule en mode mémoire (données non persistées) et /ready indiquera 503 jusqu'au rétablissement.", pingErr)
		} else {
			dbReady = true
			logger.Println("Connexion PostgreSQL établie avec succès via pgxpool.")
		}
	}
	if !dbReady {
		logger.Println("Mode dégradé mémoire actif: aucune donnée ne sera persistée tant que PostgreSQL est injoignable.")
	}

	// 3. Construction des services métier et d'authentification

	healthHandler := health.NewHandler(pinger, cfg.DBConnectionTimeout)

	// A4 : le secret JWT ne doit jamais reposer sur une valeur connue du dépôt
	// en production. Hors développement, un secret fourni par l'environnement et
	// suffisamment long est exigé, sinon le démarrage est refusé.
	jwtSecret := os.Getenv("JWT_SECRET")
	if cfg.ServerEnv == "production" && len(jwtSecret) < 32 {
		logger.Fatalf("Configuration refusée: JWT_SECRET doit être défini (32 caractères minimum) en environnement de production")
	}
	if jwtSecret == "" {
		jwtSecret = "pressingos-dev-secret-key-32-bytes-minimum"
		logger.Println("Attention: JWT_SECRET non défini — secret de développement utilisé (interdit en production).")
	}
	jwtManager := auth.NewJWTManager(jwtSecret, 24*time.Hour)
	authService := auth.NewService(jwtManager)
	// Authentification : PostgreSQL quand disponible (login/register persistés),
	// sinon repli mémoire (comptes de démonstration) pour ne jamais bloquer le démarrage.
	if dbReady {
		authService.SetStore(auth.NewStore(database.NewStore(dbPool.Pool)))
		logger.Println("Authentification adossée à PostgreSQL (comptes persistés).")
	} else {
		logger.Println("Authentification en mode mémoire (PostgreSQL indisponible au démarrage).")
	}
	authHandler := auth.NewHandler(authService)
	// A6 : protection anti-force brute sur la connexion (mémoire, par instance).
	// 10 tentatives / 5 min par couple (IP, email) et 40 tentatives / 5 min par IP.
	authHandler.SetLoginLimiters(
		ratelimit.New(10, 5*time.Minute),
		ratelimit.New(40, 5*time.Minute),
	)
	logger.Println("Limitation de débit active sur /api/v1/auth/login (anti-force brute).")

	orderService := order.NewService()
	orderHandler := order.NewHandler(orderService)
	// Commandes : PostgreSQL quand disponible, sinon repli mémoire (données seed).
	if dbReady {
		orderService.SetStore(order.NewStore(database.NewStore(dbPool.Pool)))
		logger.Println("Commandes adossées à PostgreSQL (persistance activée).")
	} else {
		logger.Println("Commandes en mode mémoire (PostgreSQL indisponible au démarrage).")
	}
	// Clients : PostgreSQL quand disponible, sinon repli mémoire (mode dégradé).
	customerService := customer.NewService()
	if dbReady {
		customerService.SetStore(customer.NewStore(database.NewStore(dbPool.Pool)))
		logger.Println("Clients adossés à PostgreSQL (persistance activée).")
	} else {
		logger.Println("Clients en mode mémoire (PostgreSQL indisponible au démarrage).")
	}
	customerHandler := customer.NewHandler(customerService)

	// Grilles tarifaires versionnées : PostgreSQL quand disponible, sinon repli mémoire.
	priceListService := pricelist.NewService()
	if dbReady {
		priceListService = pricelist.NewStore(database.NewStore(dbPool.Pool))
		logger.Println("Grilles tarifaires adossées à PostgreSQL (versions persistées).")
	} else {
		logger.Println("Grilles tarifaires en mode mémoire (PostgreSQL indisponible au démarrage).")
	}
	priceListHandler := pricelist.NewHandler(priceListService)

	// Catalogue : PostgreSQL quand disponible, sinon repli mémoire (données seed).
	catalogService := catalog.NewService()
	if dbReady {
		dbStore := database.NewStore(dbPool.Pool)
		catalogService = catalog.NewStore(dbStore)
		logger.Println("Catalogue adossé à PostgreSQL (persistance activée).")
	} else {
		logger.Println("Catalogue en mode mémoire (PostgreSQL indisponible au démarrage).")
	}
	catalogHandler := catalog.NewHandler(catalogService)

	// Onboarding d'un nouvel établissement : dès l'inscription, le pressing
	// reçoit sa grille tarifaire par défaut et sa première grille versionnée.
	// (Sans cela, un pressing fraîchement inscrit démarrait avec un catalogue vide.)
	authService.SetOnTenantRegistered(func(ctx context.Context, tenantID string) error {
		seeded, err := catalogService.SeedTenantDefaults(ctx, tenantID)
		if err != nil {
			return fmt.Errorf("catalogue initial: %w", err)
		}
		if seeded > 0 {
			logger.Printf("Onboarding: %d prestations par défaut créées pour le tenant %s", seeded, tenantID)
		}
		pl, err := priceListService.CreatePriceList(ctx, tenantID, pricelist.CreatePriceListRequest{
			Name:        "Grille principale",
			Description: "Grille tarifaire initiale du catalogue",
			IsDefault:   true,
		})
		if err != nil {
			return fmt.Errorf("grille tarifaire initiale: %w", err)
		}
		if _, err := priceListService.CreateVersion(ctx, tenantID, pl.ID, pricelist.CreateVersionRequest{
			Note: "Version initiale figée à l'inscription",
		}); err != nil {
			return fmt.Errorf("version initiale de la grille: %w", err)
		}
		logger.Printf("Onboarding: grille tarifaire principale créée pour le tenant %s", tenantID)
		return nil
	})

	// Moteur d'abonnement : PostgreSQL quand disponible, sinon repli mémoire.
	billingService := billing.NewService()
	if dbReady {
		dbBillingStore := database.NewStore(dbPool.Pool)
		billingService = billing.NewStore(dbBillingStore)
		logger.Println("Billing adossé à PostgreSQL (abonnements persistés).")
	} else {
		logger.Println("Billing en mode mémoire (PostgreSQL indisponible au démarrage).")
	}
	billingHandler := billing.NewHandler(billingService)

	// Module Établissements (multi-sites) : PostgreSQL quand disponible.
	branchService := branch.NewService()
	if dbReady {
		branchService = branch.NewStore(database.NewStore(dbPool.Pool))
		logger.Println("Établissements (multi-sites) adossés à PostgreSQL.")
	} else {
		logger.Println("Établissements en mode mémoire (PostgreSQL indisponible au démarrage).")
		// Mode dégradé : compteurs de quota et contrôle d'appartenance branchés sur les services mémoire.
		apphttp.BrancherMemoire(billingService, authService, priceListService, branchService, orderService)
	}
	branchHandler := branch.NewHandler(branchService)

	// Module Dépenses (fonctionnalité payante « expenses.advanced »).
	expenseService := expense.NewService()
	if dbReady {
		expenseService = expense.NewStore(database.NewStore(dbPool.Pool))
		logger.Println("Dépenses adossées à PostgreSQL.")
	} else {
		logger.Println("Dépenses en mode mémoire (PostgreSQL indisponible au démarrage).")
	}
	expenseHandler := expense.NewHandler(expenseService)

	// Module Rapports (fonctionnalité payante « reports.advanced ») : agrégats
	// SQL quand la base est joignable, sinon calcul mémoire à partir des
	// services commandes / dépenses.
	reportService := report.NewService()
	if dbReady {
		reportService = report.NewStore(database.NewStore(dbPool.Pool))
		logger.Println("Rapports adossés à PostgreSQL (agrégats SQL).")
	} else {
		reportService.SetSources(orderService, expenseService)
		logger.Println("Rapports en mode mémoire (PostgreSQL indisponible au démarrage).")
	}
	reportHandler := report.NewHandler(reportService, billingService)

	// Module Notifications clients (WhatsApp / SMS). Sans base : journal en
	// mémoire. L'envoi réel passe par la passerelle configurée par l'éditeur
	// (NOTIFICATIONS_GATEWAY_URL) ; sans elle, chaque message est journalisé
	// avec le statut « simule » — le comptoir peut le copier ou l'ouvrir dans
	// WhatsApp depuis l'interface.
	notificationService := notification.NewService()
	if dbReady {
		notificationService = notification.NewStore(database.NewStore(dbPool.Pool))
		logger.Println("Notifications clients adossées à PostgreSQL (journal persisté).")
	} else {
		logger.Println("Notifications clients en mode mémoire (PostgreSQL indisponible au démarrage).")
	}
	notificationService.SetSourceCommandes(orderService)
	notificationService.SetDroitVerificateur(billingService, billing.FeatureNotificationsAdvanced)
	if dbReady {
		dbStore := database.NewStore(dbPool.Pool)
		notificationService.SetNomPressing(func(ctx context.Context, tenantID string) string {
			var nom string
			if err := dbStore.QueryRowContext(ctx, `SELECT name FROM tenants WHERE id = $1`, tenantID).Scan(&nom); err != nil {
				return ""
			}
			return nom
		})
	}
	passerelleURL := strings.TrimSpace(os.Getenv("NOTIFICATIONS_GATEWAY_URL"))
	if passerelleURL != "" {
		notificationService.SetPasserelle(notification.NewHTTPPasserelle(
			passerelleURL, os.Getenv("NOTIFICATIONS_GATEWAY_TOKEN"), 5*time.Second))
		logger.Printf("Passerelle de notifications activée (%s).", passerelleURL)
	} else {
		logger.Println("Aucune passerelle de notifications : les messages sont journalisés (statut « simule »).")
	}
	notificationHandler := notification.NewHandler(notificationService)

	// Déclenchement automatique : « prête » et « livrée » notifient le client
	// (uniquement pour les formules qui incluent l'envoi et sans jamais
	// retarder la réponse HTTP — le hook s'exécute en tâche de fond).
	orderService.SetHookStatut(func(ctx context.Context, tenantID, orderID, statut string) {
		notificationService.NotifierTransition(ctx, notification.NotificationAuto{
			TenantID:    tenantID,
			OrderID:     orderID,
			StatutLinge: statut,
		})
	})

	// Garde-fou quota : la 51e commande FREE (essai dépassé) est refusée en 402.
	// Compteur mémoire en mode dégradé, compteur SQL quand PostgreSQL est joignable.
	if dbReady {
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
	if dbReady {
		// La base fait foi : contrôle d'appartenance strict (fail-closed), les
		// seuls comptes autorisés hors base étant ceux de la vitrine.
		membershipChecker = apphttp.NewMembershipChecker(dbPool.Pool, authService.IsDemoAccount)
		logger.Println("Contrôle d'appartenance et de périmètre branche adossés à PostgreSQL.")
	} else {
		membershipChecker = apphttp.NewMembershipChecker(nil, authService.IsDemoAccount)
		logger.Println("Contrôle d'appartenance en mode dégradé (PostgreSQL indisponible au démarrage).")
	}

	router := apphttp.NewRouter(apphttp.ServerDependencies{
		Config:              cfg,
		HealthHandler:       healthHandler,
		AuthHandler:         authHandler,
		JWTManager:          jwtManager,
		OrderHandler:        orderHandler,
		CatalogHandler:      catalogHandler,
		CustomerHandler:     customerHandler,
		PriceListHandler:    priceListHandler,
		BranchHandler:       branchHandler,
		ExpenseHandler:      expenseHandler,
		ReportHandler:       reportHandler,
		NotificationHandler: notificationHandler,
		Billing:             billingService,
		BillingHandler:      billingHandler,
		MembershipChecker:   membershipChecker,
	})

	// 4. Configuration du serveur HTTP avec timeouts stricts
	server := &http.Server{
		Addr:         net.JoinHostPort(cfg.Host, cfg.Port),
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
		logger.Printf("Serveur HTTP à l'écoute sur http://%s", net.JoinHostPort(cfg.Host, cfg.Port))
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
