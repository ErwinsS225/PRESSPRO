package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"pressing-os/backend/internal/auth"
	"pressing-os/backend/internal/billing"
	"pressing-os/backend/internal/branch"
	"pressing-os/backend/internal/catalog"
	"pressing-os/backend/internal/config"
	"pressing-os/backend/internal/customer"
	"pressing-os/backend/internal/expense"
	"pressing-os/backend/internal/health"
	"pressing-os/backend/internal/notification"
	"pressing-os/backend/internal/order"
	"pressing-os/backend/internal/pricelist"
	"pressing-os/backend/internal/report"
)

// ServerDependencies regroupe les gestionnaires requis pour assembler le routeur
type ServerDependencies struct {
	Config              *config.Config
	HealthHandler       *health.Handler
	AuthHandler         *auth.Handler
	JWTManager          *auth.JWTManager
	OrderHandler        *order.Handler
	CatalogHandler      *catalog.Handler
	CustomerHandler     *customer.Handler
	PriceListHandler    *pricelist.Handler
	BranchHandler       *branch.Handler
	ExpenseHandler      *expense.Handler
	ReportHandler       *report.Handler
	NotificationHandler *notification.Handler
	Billing             *billing.Service
	BillingHandler      *billing.Handler
	MembershipChecker   *MembershipChecker
}

// NewRouter assemble la table de routage Chi avec les middlewares et les routes de base
func NewRouter(deps ServerDependencies) http.Handler {
	r := chi.NewRouter()

	// Middlewares de robustesse
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(15 * time.Second))

	// Configuration CORS restrictive
	corsOrigins := deps.Config.AllowedOrigins
	if len(corsOrigins) == 0 {
		corsOrigins = []string{"http://localhost:3000"}
	}

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   corsOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "Idempotency-Key", "X-Tenant-ID", "X-Branch-ID"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Endpoints racine de santé (probes Kubernetes / Docker)
	r.Get("/health", deps.HealthHandler.HealthCheck)
	r.Get("/ready", deps.HealthHandler.ReadinessCheck)

	// API Versionnée
	r.Route("/api/v1", func(api chi.Router) {
		// Métadonnées système
		api.Get("/system/info", func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"product":       "PressingOS",
				"version":       "0.5.0-hardening",
				"environment":   deps.Config.ServerEnv,
				"architecture":  "Modular Monolith (Multi-Tenant Shared DB / Tenant-ID Isolated)",
				"database_type": "PostgreSQL 16 with pgx connection pool",
				"target_market": "Pressings et Buanderies — Côte d'Ivoire (XOF)",
				"modules":       []string{"Health", "Auth JWT", "Tenants", "Orders & Receipts", "Catalog & Pricing Grid", "Billing & Entitlements", "Price Lists", "Customers"},
				"timestamp":     time.Now().UTC(),
			})
		})

		// Routes publiques d'authentification
		if deps.AuthHandler != nil {
			api.Route("/auth", func(authRouter chi.Router) {
				authRouter.Post("/login", deps.AuthHandler.Login)
				authRouter.Post("/register-tenant", deps.AuthHandler.RegisterTenant)

				// Routes protégées par jeton JWT
				if deps.JWTManager != nil {
					authRouter.Group(func(protected chi.Router) {
						protected.Use(AuthenticatorMiddleware(deps.JWTManager))
						protected.Get("/me", deps.AuthHandler.Me)
						// La liste des utilisateurs est servie par le module Équipe
						// (/api/v1/users), réservé aux gérants et plafonné par la formule.
					})
				}
			})
		}

		// Vérification publique d'un ticket (QR code) — sans JWT.
		if deps.OrderHandler != nil {
			api.Get("/public/verify/{tenantID}/{orderID}", deps.OrderHandler.VerifyTicket)
		}

		// Espace protégé multi-tenant
		if deps.JWTManager != nil {
			api.Group(func(protected chi.Router) {
				// Contrôle #1 — Identité : jeton JWT valide.
				protected.Use(AuthenticatorMiddleware(deps.JWTManager))

				// Contrôle #2 — Appartenance : membre actif du tenant porté par le JWT.
				// Le tenant_id est dérivé des claims (jamais du body client) => anti-fuite inter-tenant.
				if deps.MembershipChecker != nil {
					protected.Use(RequireTenantMembership(deps.MembershipChecker))
				}

				// Profil du tenant courant (toujours accessible : le pressing doit
				// pouvoir constater l'état de son abonnement).
				protected.Get("/tenant/current", func(w http.ResponseWriter, req *http.Request) {
					claims := auth.GetUserFromContext(req.Context())
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"tenant_id":   claims.TenantID,
						"tenant_name": claims.TenantName,
						"user_id":     claims.UserID,
						"role":        claims.Role,
					})
				})

				// Modules métier : gelés si l'éditeur a suspendu l'abonnement.
				protected.Group(func(business chi.Router) {
					business.Use(RequireActiveSubscription(deps.Billing))

					// Module Grille Tarifaire & Prestations
					if deps.CatalogHandler != nil {
						business.Route("/services", func(catalogRouter chi.Router) {
							// Lecture pour tous les membres du pressing
							catalogRouter.Get("/", deps.CatalogHandler.ListServices)
							catalogRouter.Get("/{id}", deps.CatalogHandler.GetService)

							// Mutations réservées aux propriétaires, gérants et superadmins (RBAC)
							catalogRouter.Group(func(managerRouter chi.Router) {
								managerRouter.Use(RequireRole(auth.RoleOwner, auth.RoleManager, auth.RoleSuperAdmin))
								managerRouter.Post("/", deps.CatalogHandler.CreateService)
								managerRouter.Put("/{id}", deps.CatalogHandler.UpdateService)
								managerRouter.Delete("/{id}", deps.CatalogHandler.DeleteService)
							})
						})
					}

					// Module Commandes de linge & Tickets thermiques.
					// Contrôle #4 (périmètre branche) : les opérations physiques sont
					// rattachées à un établissement. Le header X-Branch-ID est validé
					// contre les branch_memberships de l'utilisateur (fail-open comptes démo).
					if deps.OrderHandler != nil {
						business.Route("/orders", func(orderRouter chi.Router) {
							if deps.MembershipChecker != nil {
								orderRouter.Use(RequireBranchScope(deps.MembershipChecker))
							}
							orderRouter.Get("/", deps.OrderHandler.ListOrders)
							orderRouter.Post("/", deps.OrderHandler.CreateOrder)
							orderRouter.Get("/{id}", deps.OrderHandler.GetOrder)
							orderRouter.Patch("/{id}/status", deps.OrderHandler.UpdateStatus)
							orderRouter.Post("/{id}/payments", deps.OrderHandler.AddPayment)
							orderRouter.Get("/{id}/ticket", deps.OrderHandler.ThermalReceipt)
						})
					}

					// Module Clients (fichier client partagé au niveau du tenant)
					if deps.CustomerHandler != nil {
						business.Route("/customers", func(customerRouter chi.Router) {
							// Lecture & création pour tous les membres (le caissier encaisse)
							customerRouter.Get("/", deps.CustomerHandler.ListCustomers)
							customerRouter.Get("/{id}", deps.CustomerHandler.GetCustomer)
							customerRouter.Post("/", deps.CustomerHandler.CreateCustomer)

							// Mutations réservées aux propriétaires, gérants et superadmins (RBAC)
							customerRouter.Group(func(managerRouter chi.Router) {
								managerRouter.Use(RequireRole(auth.RoleOwner, auth.RoleManager, auth.RoleSuperAdmin))
								managerRouter.Put("/{id}", deps.CustomerHandler.UpdateCustomer)
								managerRouter.Delete("/{id}", deps.CustomerHandler.DeleteCustomer)
							})
						})
					}

					// Module Grilles tarifaires versionnées (historique immuable des prix).
					// Lecture pour tous les membres ; mutations (grille + figeage de
					// version) réservées aux propriétaires, gérants et superadmins.
					if deps.PriceListHandler != nil {
						business.Route("/price-lists", func(plRouter chi.Router) {
							// Lecture toujours autorisée : une rétrogradation de formule ne
							// doit jamais rendre l'historique de prix inaccessible.
							plRouter.Get("/", deps.PriceListHandler.ListPriceLists)
							plRouter.Get("/{id}", deps.PriceListHandler.GetPriceList)

							plRouter.Group(func(managerRouter chi.Router) {
								managerRouter.Use(RequireRole(auth.RoleOwner, auth.RoleManager, auth.RoleSuperAdmin))

								// Figeage d'une version : module « grilles tarifaires » requis.
								managerRouter.With(RequireFeature(deps.Billing, billing.FeaturePriceLists)).
									Post("/{id}/versions", deps.PriceListHandler.CreateVersion)

								// Création d'une grille supplémentaire : soumise au PLAFOND de
								// la formule (1 gratuite, 3 en PRO, 10 en BUSINESS).
								managerRouter.With(RequireFeatureQuota(deps.Billing, billing.FeaturePriceLists)).
									Post("/", deps.PriceListHandler.CreatePriceList)
							})
						})
					}

					// Module Équipe : comptes employés de l'établissement, plafonnés
					// par la formule (users.max : 2 en gratuit, 5 en PRO, 15 en BUSINESS).
					if deps.AuthHandler != nil {
						business.Group(func(team chi.Router) {
							team.Use(RequireRole(auth.RoleOwner, auth.RoleManager, auth.RoleSuperAdmin))
							team.Get("/users", deps.AuthHandler.ListUsers)
							team.With(RequireFeatureQuota(deps.Billing, billing.FeatureUsersMax)).
								Post("/users", deps.AuthHandler.CreateUser)
							team.Patch("/users/{id}/status", deps.AuthHandler.SetUserStatus)
							// Affectation d'un employé aux établissements du pressing
							// (un caissier n'opère que sur ses sites).
							if deps.BranchHandler != nil {
								team.Get("/users/{id}/branches", deps.BranchHandler.ListMemberBranches)
								team.Put("/users/{id}/branches", deps.BranchHandler.UpdateMemberBranches)
							}
						})
					}

					// Module Établissements (multi-sites) : lecture ouverte aux
					// membres (sélection du site d'imputation), création soumise au
					// plafond de la formule (1 en gratuit et PRO, 3 en BUSINESS).
					if deps.BranchHandler != nil {
						business.Route("/branches", func(branchRouter chi.Router) {
							branchRouter.Get("/", deps.BranchHandler.ListBranches)
							branchRouter.Group(func(managerRouter chi.Router) {
								managerRouter.Use(RequireRole(auth.RoleOwner, auth.RoleManager, auth.RoleSuperAdmin))
								managerRouter.With(RequireFeatureQuota(deps.Billing, billing.FeatureMultiSite)).
									Post("/", deps.BranchHandler.CreateBranch)
								managerRouter.Patch("/{id}", deps.BranchHandler.UpdateBranch)
							})
						})
					}

					// Module Dépenses (fonctionnalité payante « expenses.advanced »).
					// Lecture pour les membres, écritures réservées aux gérants.
					if deps.ExpenseHandler != nil {
						business.Route("/expenses", func(expenseRouter chi.Router) {
							expenseRouter.Get("/", deps.ExpenseHandler.ListExpenses)
							expenseRouter.Group(func(managerRouter chi.Router) {
								managerRouter.Use(RequireRole(auth.RoleOwner, auth.RoleManager, auth.RoleSuperAdmin))
								managerRouter.With(RequireFeature(deps.Billing, billing.FeatureExpensesAdvanced)).
									Post("/", deps.ExpenseHandler.CreateExpense)
								managerRouter.With(RequireFeature(deps.Billing, billing.FeatureExpensesAdvanced)).
									Delete("/{id}", deps.ExpenseHandler.DeleteExpense)
							})
						})
					}

					// Module Notifications clients (WhatsApp / SMS). Le journal
					// et la composition restent ouverts ; l'envoi réel par la
					// passerelle est réservé aux formules payantes.
					if deps.NotificationHandler != nil {
						business.Route("/notifications", func(notificationRouter chi.Router) {
							notificationRouter.Get("/", deps.NotificationHandler.ListNotifications)
							notificationRouter.With(RequireFeature(deps.Billing, billing.FeatureNotificationsAdvanced)).
								Post("/", deps.NotificationHandler.CreateNotification)
						})
					}

					// consolidation par établissement incluse dans BUSINESS).
					if deps.ReportHandler != nil {
						business.Route("/reports", func(reportRouter chi.Router) {
							reportRouter.Use(RequireRole(auth.RoleOwner, auth.RoleManager, auth.RoleSuperAdmin))
							reportRouter.With(RequireFeature(deps.Billing, billing.FeatureReportsAdvanced)).
								Get("/summary", deps.ReportHandler.Summary)
							// Export du rapport (CSV/PDF) : même garde-fou que l'écran,
							// les chiffres exportés sont ceux du rapport affiché.
							reportRouter.With(RequireFeature(deps.Billing, billing.FeatureReportsAdvanced)).
								Get("/export", deps.ReportHandler.Export)
						})
					}

				}) // fin des modules métier

				// Droits effectifs du pressing (moteur d'abonnement centralisé).
				if deps.BillingHandler != nil {
					protected.Get("/me/entitlements", deps.BillingHandler.Me)
					protected.Get("/billing/plans", deps.BillingHandler.Catalogue)
				}

				// Espace SuperAdmin (éditeur SaaS) : données réelles de la plateforme.
				if deps.Billing != nil {
					protected.Route("/admin", func(adminRouter chi.Router) {
						adminRouter.Use(RequireRole(auth.RoleSuperAdmin))
						adminRouter.Get("/stats", SuperAdminStats(deps.Billing))
						adminRouter.Get("/tenants", SuperAdminTenants(deps.Billing))
						adminRouter.Patch("/tenants/{id}/status", SuperAdminSetTenantStatus(deps.Billing))
						// Retrait définitif d'un établissement (et de ses données) :
						// sert à nettoyer les entreprises de démonstration.
						adminRouter.Delete("/tenants/{id}", SuperAdminDeleteTenant(deps.Billing))
						// Tarif d'une formule payante : journalisé, réservé à l'éditeur.
						adminRouter.Patch("/plans/{code}/price", SuperAdminSetPlanPrice(deps.Billing))
					})
				}
			})
		}
	})

	return r
}
