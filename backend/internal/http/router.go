package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"pressing-os/backend/internal/auth"
	"pressing-os/backend/internal/catalog"
	"pressing-os/backend/internal/config"
	"pressing-os/backend/internal/health"
	"pressing-os/backend/internal/order"
)

// ServerDependencies regroupe les gestionnaires requis pour assembler le routeur
type ServerDependencies struct {
	Config         *config.Config
	HealthHandler  *health.Handler
	AuthHandler    *auth.Handler
	JWTManager     *auth.JWTManager
	OrderHandler   *order.Handler
	CatalogHandler *catalog.Handler
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
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Tenant-ID"},
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
				"version":       "0.4.0-catalog",
				"environment":   deps.Config.ServerEnv,
				"architecture":  "Modular Monolith (Multi-Tenant Shared DB / Tenant-ID Isolated)",
				"database_type": "PostgreSQL 16 with pgx connection pool",
				"target_market": "Pressings et Buanderies — Côte d'Ivoire (XOF)",
				"modules":       []string{"Health", "Auth JWT", "Tenants", "Orders & Receipts", "Catalog & Pricing Grid"},
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
					})
				}
			})
		}

		// Espace protégé multi-tenant
		if deps.JWTManager != nil {
			api.Group(func(protected chi.Router) {
				protected.Use(AuthenticatorMiddleware(deps.JWTManager))

				// Profil du tenant courant
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

				// Module Grille Tarifaire & Prestations
				if deps.CatalogHandler != nil {
					protected.Route("/services", func(catalogRouter chi.Router) {
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

				// Module Commandes de linge & Tickets thermiques
				if deps.OrderHandler != nil {
					protected.Route("/orders", func(orderRouter chi.Router) {
						orderRouter.Get("/", deps.OrderHandler.ListOrders)
						orderRouter.Post("/", deps.OrderHandler.CreateOrder)
						orderRouter.Get("/{id}", deps.OrderHandler.GetOrder)
						orderRouter.Patch("/{id}/status", deps.OrderHandler.UpdateStatus)
						orderRouter.Post("/{id}/payments", deps.OrderHandler.AddPayment)
						orderRouter.Get("/{id}/ticket", deps.OrderHandler.ThermalReceipt)
					})
				}

				// Route SuperAdmin uniquement
				protected.With(RequireRole(auth.RoleSuperAdmin)).Get("/admin/stats", func(w http.ResponseWriter, req *http.Request) {
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"total_registered_pressings": 12,
						"active_subscriptions":       10,
						"monthly_recurring_mrr_xof":  350000,
						"country":                    "Côte d'Ivoire",
					})
				})
			})
		}
	})

	return r
}
