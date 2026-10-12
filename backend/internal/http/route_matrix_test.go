package http_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"pressing-os/backend/internal/auth"
	"pressing-os/backend/internal/billing"
	"pressing-os/backend/internal/branch"
	"pressing-os/backend/internal/catalog"
	"pressing-os/backend/internal/config"
	"pressing-os/backend/internal/customer"
	"pressing-os/backend/internal/expense"
	"pressing-os/backend/internal/health"
	apphttp "pressing-os/backend/internal/http"
	"pressing-os/backend/internal/notification"
	"pressing-os/backend/internal/order"
	"pressing-os/backend/internal/pricelist"
	"pressing-os/backend/internal/report"
)

// Matrice des routes (PLAN phase 4).
//
// Chaque entrée décrit une route enregistrée par apphttp.NewRouter :
//   - publique : aucune authentification ;
//   - metier : soumise au contrôle d'abonnement (RequireActiveSubscription) ;
//   - roles : rôles autorisés par RequireRole (vide = tout membre actif) ;
//   - fonctionnalite / quota : droit du plan contrôlé par la route ;
//   - controle : contrôle complémentaire (portée, service, etc.).
//
// TestMatrice_CouvertureDesRoutes échoue si une route est ajoutée sans entrée ici,
// ou si une entrée ne correspond plus à une route enregistrée.
//
// Chaque test de comportement s'exécute dans deux modes (voir pourChaqueMode) :
// « mémoire » (mode dégradé, sans base) et « PostgreSQL » (fichier d'intégration,
// tag integration, ignoré sans TEST_DATABASE_URL).
type entreeMatrice struct {
	methode, modele string
	publique        bool
	metier          bool
	roles           []string
	fonctionnalite  string
	quota           string
	controle        string
}

var (
	rolesGestion = []string{auth.RoleOwner, auth.RoleManager, auth.RoleSuperAdmin}
	rolesAdmin   = []string{auth.RoleSuperAdmin}
)

var matriceRoutes = []entreeMatrice{
	// Techniques et publiques.
	{methode: "GET", modele: "/health", publique: true},
	{methode: "GET", modele: "/ready", publique: true},
	{methode: "GET", modele: "/api/v1/system/info", publique: true},
	{methode: "POST", modele: "/api/v1/auth/login", publique: true},
	{methode: "POST", modele: "/api/v1/auth/register-tenant", publique: true},
	{methode: "GET", modele: "/api/v1/public/verify/{tenantID}/{orderID}", publique: true,
		controle: "projection publique du ticket (voir décision D7)"},

	// Identité et compte : jeton + appartenance, sans abonnement actif requis.
	{methode: "GET", modele: "/api/v1/auth/me"},
	{methode: "GET", modele: "/api/v1/tenant/current"},
	{methode: "GET", modele: "/api/v1/me/entitlements"},

	// Grille tarifaire & prestations.
	{methode: "GET", modele: "/api/v1/services/", metier: true},
	{methode: "GET", modele: "/api/v1/services/{id}", metier: true},
	{methode: "POST", modele: "/api/v1/services/", metier: true, roles: rolesGestion},
	{methode: "PUT", modele: "/api/v1/services/{id}", metier: true, roles: rolesGestion},
	{methode: "DELETE", modele: "/api/v1/services/{id}", metier: true, roles: rolesGestion},

	// Commandes (contrôle de portée établissement sur toutes les routes).
	{methode: "GET", modele: "/api/v1/orders/", metier: true, controle: "portée établissement"},
	{methode: "POST", modele: "/api/v1/orders/", metier: true, quota: "orders.monthly.max (fenêtre 30 j, dans la transaction)", controle: "portée établissement"},
	{methode: "GET", modele: "/api/v1/orders/{id}", metier: true, controle: "portée établissement"},
	{methode: "PATCH", modele: "/api/v1/orders/{id}/status", metier: true, controle: "portée établissement"},
	{methode: "POST", modele: "/api/v1/orders/{id}/payments", metier: true, controle: "portée établissement"},
	{methode: "GET", modele: "/api/v1/orders/{id}/ticket", metier: true, controle: "portée établissement"},

	// Clients : lecture et création pour tous les membres, mutations gérées.
	{methode: "GET", modele: "/api/v1/customers/", metier: true},
	{methode: "GET", modele: "/api/v1/customers/{id}", metier: true},
	{methode: "POST", modele: "/api/v1/customers/", metier: true},
	{methode: "PUT", modele: "/api/v1/customers/{id}", metier: true, roles: rolesGestion},
	{methode: "DELETE", modele: "/api/v1/customers/{id}", metier: true, roles: rolesGestion},

	// Grilles tarifaires versionnées.
	{methode: "GET", modele: "/api/v1/price-lists/", metier: true},
	{methode: "GET", modele: "/api/v1/price-lists/{id}", metier: true},
	{methode: "POST", modele: "/api/v1/price-lists/", metier: true, roles: rolesGestion,
		quota: "price_lists (plafond de la formule)"},
	{methode: "POST", modele: "/api/v1/price-lists/{id}/versions", metier: true, roles: rolesGestion,
		fonctionnalite: billing.FeaturePriceLists},

	// Équipe.
	{methode: "GET", modele: "/api/v1/users", metier: true, roles: rolesGestion},
	{methode: "POST", modele: "/api/v1/users", metier: true, roles: rolesGestion,
		quota: "users.max (plafond de la formule)"},
	{methode: "PATCH", modele: "/api/v1/users/{id}/status", metier: true, roles: rolesGestion},
	{methode: "GET", modele: "/api/v1/users/{id}/branches", metier: true, roles: rolesGestion},
	{methode: "PUT", modele: "/api/v1/users/{id}/branches", metier: true, roles: rolesGestion},

	// Établissements (multi-sites).
	{methode: "GET", modele: "/api/v1/branches/", metier: true},
	{methode: "POST", modele: "/api/v1/branches/", metier: true, roles: rolesGestion,
		quota: "multi_site (plafond de la formule)"},
	{methode: "PATCH", modele: "/api/v1/branches/{id}", metier: true, roles: rolesGestion},

	// Dépenses (fonctionnalité payante).
	{methode: "GET", modele: "/api/v1/expenses/", metier: true},
	{methode: "POST", modele: "/api/v1/expenses/", metier: true, roles: rolesGestion,
		fonctionnalite: billing.FeatureExpensesAdvanced},
	{methode: "DELETE", modele: "/api/v1/expenses/{id}", metier: true, roles: rolesGestion,
		fonctionnalite: billing.FeatureExpensesAdvanced,
		controle:       "suppression bloquée par la fonctionnalité (voir décision D6)"},

	// Notifications clients.
	{methode: "GET", modele: "/api/v1/notifications/", metier: true},
	{methode: "POST", modele: "/api/v1/notifications/", metier: true,
		fonctionnalite: billing.FeatureNotificationsAdvanced},

	// Rapports (fonctionnalité payante).
	{methode: "GET", modele: "/api/v1/reports/summary", metier: true, roles: rolesGestion,
		fonctionnalite: billing.FeatureReportsAdvanced},
	{methode: "GET", modele: "/api/v1/reports/export", metier: true, roles: rolesGestion,
		fonctionnalite: billing.FeatureReportsAdvanced},

	// Administration de la plateforme (éditeur SaaS uniquement).
	{methode: "GET", modele: "/api/v1/admin/stats", roles: rolesAdmin},
	{methode: "GET", modele: "/api/v1/admin/tenants", roles: rolesAdmin},
	{methode: "PATCH", modele: "/api/v1/admin/tenants/{id}/status", roles: rolesAdmin},
	{methode: "DELETE", modele: "/api/v1/admin/tenants/{id}", roles: rolesAdmin},
}

// environnement assemble le routeur complet et expose les opérations de test.
type environnement struct {
	t       *testing.T
	router  http.Handler
	jwt     *auth.JWTManager
	authSvc *auth.Service
	bill    *billing.Service

	// persistant : les clients sont réellement stockés (PostgreSQL). En mode mémoire,
	// le service client ne conserve rien : les tests de clients sont alors sautés.
	persistant bool
	// quotas : les compteurs des plafonds (grilles, équipe, établissements) sont
	// mesurés. En mode mémoire, ils ne le sont pas : les tests de plafonds sont sautés.
	quotas bool
	// formule fixe le plan d'un établissement de test (SQL en base, ForcePlanForTest en mémoire).
	formule func(tenant, plan string)
	// crees : établissements créés pendant le test (supprimés au nettoyage en base).
	crees []string
}

// fabriqueEnvironnement construit un environnement dans un mode donné.
type fabriqueEnvironnement struct {
	nom        string
	construire func(t *testing.T) *environnement
}

// fabriques : modes exécutés par pourChaqueMode. Le mode PostgreSQL est ajouté
// par le fichier d'intégration (tag integration).
var fabriques = []fabriqueEnvironnement{
	{nom: "mémoire", construire: nouvelEnvironnement},
}

// pourChaqueMode exécute fn dans chaque mode disponible, avec un environnement neuf.
func pourChaqueMode(t *testing.T, fn func(t *testing.T, env *environnement)) {
	t.Helper()
	for _, f := range fabriques {
		f := f
		t.Run(f.nom, func(t *testing.T) {
			fn(t, f.construire(t))
		})
	}
}

// nouvelEnvironnement assemble le routeur en mode mémoire (aucune base).
func nouvelEnvironnement(t *testing.T) *environnement {
	t.Helper()
	cfg := &config.Config{Port: "8080", ServerEnv: "test", AllowedOrigins: []string{"http://localhost:3000"}}
	jwtManager := auth.NewJWTManager("test-secret-key-1234567890", 2*time.Hour)
	authSvc := auth.NewService(jwtManager)
	bill := billing.NewService()

	orderSvc := order.NewService()
	orderSvc.SetQuotaGuard(&billing.QuotaGuard{Billing: bill})
	customerSvc := customer.NewService()
	priceSvc := pricelist.NewService()
	catalogSvc := catalog.NewService()
	branchSvc := branch.NewService()
	expenseSvc := expense.NewService()
	notifSvc := notification.NewService()
	reportSvc := report.NewService()
	reportSvc.SetSources(orderSvc, expenseSvc)

	router := apphttp.NewRouter(apphttp.ServerDependencies{
		Config:              cfg,
		HealthHandler:       health.NewHandler(nil, time.Second),
		AuthHandler:         auth.NewHandler(authSvc),
		JWTManager:          jwtManager,
		OrderHandler:        order.NewHandler(orderSvc),
		CatalogHandler:      catalog.NewHandler(catalogSvc),
		CustomerHandler:     customer.NewHandler(customerSvc),
		PriceListHandler:    pricelist.NewHandler(priceSvc),
		BranchHandler:       branch.NewHandler(branchSvc),
		ExpenseHandler:      expense.NewHandler(expenseSvc),
		ReportHandler:       report.NewHandler(reportSvc, bill),
		NotificationHandler: notification.NewHandler(notifSvc),
		Billing:             bill,
		BillingHandler:      billing.NewHandler(bill),
		MembershipChecker:   apphttp.NewMembershipChecker(nil, authSvc.IsDemoAccount),
	})
	return &environnement{
		t: t, router: router, jwt: jwtManager, authSvc: authSvc, bill: bill,
		formule: func(tenant, plan string) { bill.ForcePlanForTest(tenant, plan, "ACTIVE") },
	}
}

// requete exécute une requête HTTP sur le routeur. corps : chaîne brute ou valeur JSON.
func (e *environnement) requete(methode, chemin, jeton string, corps interface{}, entetes ...string) *httptest.ResponseRecorder {
	e.t.Helper()
	var lecteur *bytes.Reader
	switch c := corps.(type) {
	case nil:
		lecteur = bytes.NewReader(nil)
	case string:
		lecteur = bytes.NewReader([]byte(c))
	default:
		b, err := json.Marshal(c)
		if err != nil {
			e.t.Fatalf("corps JSON: %v", err)
		}
		lecteur = bytes.NewReader(b)
	}
	req := httptest.NewRequest(methode, chemin, lecteur)
	req.Header.Set("Content-Type", "application/json")
	if jeton != "" {
		req.Header.Set("Authorization", "Bearer "+jeton)
	}
	for i := 0; i+1 < len(entetes); i += 2 {
		req.Header.Set(entetes[i], entetes[i+1])
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec
}

// connexion obtient un jeton par identifiants.
func (e *environnement) connexion(email, motDePasse string) string {
	e.t.Helper()
	rec := e.requete("POST", "/api/v1/auth/login", "", map[string]string{"email": email, "password": motDePasse})
	var rep auth.LoginResponse
	_ = json.NewDecoder(rec.Body).Decode(&rep)
	if rec.Code != http.StatusOK || rep.Token == "" {
		e.t.Fatalf("connexion %s refusée: %d %s", email, rec.Code, rec.Body.String())
	}
	return rep.Token
}

// compteUnique ajoute un suffixe unique à une adresse : les tests peuvent s'exécuter
// plusieurs fois sur une même base sans collision.
var compteurUnique atomic.Int64

func compteUnique(email string) string {
	parts := strings.SplitN(email, "@", 2)
	return fmt.Sprintf("%s.%d.%d@%s", parts[0], time.Now().UnixNano(), compteurUnique.Add(1), parts[1])
}

// inscrire crée un établissement (inscription publique) et retourne son jeton propriétaire.
// Le plan est ensuite fixé explicitement par le test (formule).
func (e *environnement) inscrire(nom, email string) (jeton, tenant string) {
	e.t.Helper()
	email = compteUnique(email)
	rec := e.requete("POST", "/api/v1/auth/register-tenant", "", map[string]string{
		"tenant_name": nom + " " + fmt.Sprint(compteurUnique.Add(1)), "city": "Abidjan", "phone": "+225 07 00 00 00",
		"owner_full_name": "Propriétaire " + nom, "owner_email": email, "password": "motdepasse1",
	})
	var rep auth.LoginResponse
	_ = json.NewDecoder(rec.Body).Decode(&rep)
	if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
		e.t.Fatalf("inscription %s refusée: %d %s", nom, rec.Code, rec.Body.String())
	}
	if rep.User.TenantID == "" {
		e.t.Fatalf("inscription %s : établissement absent de la réponse", nom)
	}
	e.crees = append(e.crees, rep.User.TenantID)
	return rep.Token, rep.User.TenantID
}

// idDe lit le champ « id » d'une réponse JSON de création.
func (e *environnement) idDe(rec *httptest.ResponseRecorder) string {
	e.t.Helper()
	if rec.Code != http.StatusCreated && rec.Code != http.StatusOK {
		e.t.Fatalf("création attendue: %d %s", rec.Code, rec.Body.String())
	}
	var m map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&m); err != nil {
		e.t.Fatalf("réponse illisible: %v", err)
	}
	id, _ := m["id"].(string)
	if id == "" {
		e.t.Fatalf("identifiant absent de la réponse: %v", m)
	}
	return id
}

// premierIdentifiant extrait le premier champ « id » d'une réponse JSON.
var reIdentifiant = regexp.MustCompile(`"id":"([^"]+)"`)

func premierIdentifiant(corps string) string {
	if m := reIdentifiant.FindStringSubmatch(corps); m != nil {
		return m[1]
	}
	return ""
}

// exemple remplace les paramètres de chemin par un identifiant factice (tests de refus).
func exemple(modele string) string {
	const factice = "00000000-0000-4000-8000-000000000000"
	chemin := modele
	for _, p := range []string{"{id}", "{tenantID}", "{orderID}"} {
		chemin = strings.ReplaceAll(chemin, p, factice)
	}
	return chemin
}

// TestMatrice_CouvertureDesRoutes : toute route enregistrée figure dans la matrice, et réciproquement.
func TestMatrice_CouvertureDesRoutes(t *testing.T) {
	env := nouvelEnvironnement(t)
	trouvees := map[string]bool{}
	if err := chi.Walk(env.router.(chi.Routes), func(methode, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		trouvees[methode+" "+route] = true
		return nil
	}); err != nil {
		t.Fatalf("parcours des routes: %v", err)
	}
	attendues := map[string]bool{}
	for _, e := range matriceRoutes {
		cle := e.methode + " " + e.modele
		if attendues[cle] {
			t.Errorf("entrée de matrice en double : %s", cle)
		}
		attendues[cle] = true
	}
	for cle := range trouvees {
		if !attendues[cle] {
			t.Errorf("route enregistrée absente de la matrice : %s", cle)
		}
	}
	for cle := range attendues {
		if !trouvees[cle] {
			t.Errorf("entrée de matrice sans route enregistrée : %s", cle)
		}
	}
	if len(trouvees) != len(matriceRoutes) {
		t.Errorf("routes enregistrées = %d, entrées de matrice = %d", len(trouvees), len(matriceRoutes))
	}
}

// TestMatrice_SansJetonRefuse : toute route protégée répond 401 sans jeton.
func TestMatrice_SansJetonRefuse(t *testing.T) {
	pourChaqueMode(t, func(t *testing.T, env *environnement) {
		for _, e := range matriceRoutes {
			if e.publique {
				continue
			}
			rec := env.requete(e.methode, exemple(e.modele), "", "{}")
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("%s %s sans jeton : %d, attendu 401", e.methode, e.modele, rec.Code)
			}
		}
	})
}

// TestMatrice_RolesRefusent : un rôle non autorisé (caissier, opérateur) reçoit 403 ;
// les routes d'administration refusent aussi le propriétaire de l'établissement.
func TestMatrice_RolesRefusent(t *testing.T) {
	pourChaqueMode(t, func(t *testing.T, env *environnement) {
		l := preparerLocataires(t, env)
		for _, e := range matriceRoutes {
			if len(e.roles) == 0 {
				continue
			}
			for nom, jeton := range map[string]string{"caissier": l.aCaissier, "opérateur": l.aOperateur} {
				rec := env.requete(e.methode, exemple(e.modele), jeton, "{}")
				if rec.Code != http.StatusForbidden {
					t.Errorf("%s %s pour un %s : %d, attendu 403", e.methode, e.modele, nom, rec.Code)
				}
			}
			if equivalentAdmin(e.roles) {
				rec := env.requete(e.methode, exemple(e.modele), l.aProprietaire, "{}")
				if rec.Code != http.StatusForbidden {
					t.Errorf("%s %s pour le propriétaire : %d, attendu 403 (administration plateforme)", e.methode, e.modele, rec.Code)
				}
			}
		}
	})
}

func equivalentAdmin(roles []string) bool {
	return len(roles) == 1 && roles[0] == auth.RoleSuperAdmin
}

// creerCompte crée un employé via la route d'équipe. Retourne son jeton et son identifiant.
func creerCompte(env *environnement, jetonGerant, email, role string) (jeton, id string) {
	env.t.Helper()
	email = compteUnique(email)
	rec := env.requete("POST", "/api/v1/users", jetonGerant, map[string]string{
		"full_name": "Employé " + role, "email": email, "phone": "+225 05 00 00 00",
		"role": role, "password": "motdepasse1",
	})
	if rec.Code != http.StatusCreated {
		env.t.Fatalf("création du compte %s : %d %s", role, rec.Code, rec.Body.String())
	}
	id = env.idDe(rec)
	return env.connexion(email, "motdepasse1"), id
}

// TestMatrice_SuperAdminSansEtablissement : sur les routes métier, le superadmin sans
// établissement choisi (X-Tenant-ID) ne lit ni n'écrit rien : 403.
func TestMatrice_SuperAdminSansEtablissement(t *testing.T) {
	pourChaqueMode(t, func(t *testing.T, env *environnement) {
		superadmin := env.connexion("admin@pressingos.ci", "admin123")
		for _, e := range matriceRoutes {
			if !e.metier {
				continue
			}
			rec := env.requete(e.methode, exemple(e.modele), superadmin, "{}")
			if rec.Code != http.StatusForbidden {
				t.Errorf("%s %s (superadmin sans établissement) : %d, attendu 403", e.methode, e.modele, rec.Code)
			}
		}
	})
}

// locataires regroupe les jetons et identifiants des établissements de test.
type locataires struct {
	// A : établissement de référence (formule BUSINESS).
	aProprietaire, aTenant   string
	aCaissier, aCaissierID   string
	aOperateur, aOperateurID string
	// B : autre établissement (BUSINESS) : ne doit jamais voir ni modifier A.
	bProprietaire, bTenant string
	// C : établissement en formule FREE.
	cProprietaire, cTenant string
}

func preparerLocataires(t *testing.T, env *environnement) locataires {
	t.Helper()
	var l locataires
	l.aProprietaire, l.aTenant = env.inscrire("Pressing Alpha", "proprietaire.a@test.ci")
	env.formule(l.aTenant, billing.PlanBusiness)
	l.aCaissier, l.aCaissierID = creerCompte(env, l.aProprietaire, "caissier.a@test.ci", auth.RoleCashier)
	l.aOperateur, l.aOperateurID = creerCompte(env, l.aProprietaire, "operateur.a@test.ci", auth.RoleOperator)

	l.bProprietaire, l.bTenant = env.inscrire("Pressing Bravo", "proprietaire.b@test.ci")
	env.formule(l.bTenant, billing.PlanBusiness)

	l.cProprietaire, l.cTenant = env.inscrire("Pressing Charlie", "proprietaire.c@test.ci")
	env.formule(l.cTenant, billing.PlanFree)
	return l
}

// TestIsolation_RessourcesDUnAutreEtablissement : un établissement ne lit, ne modifie ni
// ne supprime les ressources d'un autre, par identifiant. Les listes ne les exposent pas.
func TestIsolation_RessourcesDUnAutreEtablissement(t *testing.T) {
	pourChaqueMode(t, func(t *testing.T, env *environnement) {
		l := preparerLocataires(t, env)
		a := l.aProprietaire

		svcA := env.idDe(env.requete("POST", "/api/v1/services/", a, catalog.CreateServiceRequest{
			Name: "Chemise A", Category: catalog.CategoryDryClean, UnitPrice: 1500, Unit: "pièce", EstimatedHours: 24,
		}))
		plA := env.idDe(env.requete("POST", "/api/v1/price-lists/", a, pricelist.CreatePriceListRequest{Name: "Grille A"}))
		brA := env.idDe(env.requete("POST", "/api/v1/branches/", a, branch.CreateBranchRequest{Name: "Site A2"}))
		expA := env.idDe(env.requete("POST", "/api/v1/expenses/", a, expense.CreateExpenseRequest{
			Category: "energie_eau", Label: "Facture A", AmountXOF: 20000,
		}))
		ordA := env.idDe(env.requete("POST", "/api/v1/orders/", a, order.CreateOrderRequest{
			CustomerName: "Yao Koffi", CustomerPhone: "+225 07 22 22 22 22",
			Items: []order.CreateOrderItemInput{{ItemLabel: "Robe", Quantity: 1, UnitPrice: 3000}},
		}))
		// Clients : persistés uniquement en base (mode mémoire : non stockés).
		custA := ""
		if env.persistant {
			custA = env.idDe(env.requete("POST", "/api/v1/customers/", a, customer.CreateCustomerRequest{
				FirstName: "Awa", LastName: "Kouadio", Phone: "+225 07 11 11 11 11",
			}))
		}

		// Un site de B (valide pour B) ne doit pas pouvoir être affecté au compte de A.
		brB := premierIdentifiant(env.requete("GET", "/api/v1/branches/", l.bProprietaire, nil).Body.String())
		if brB == "" {
			t.Fatal("identifiant d'un site de B introuvable")
		}

		type attaque struct {
			methode, chemin string
			corps           interface{}
			client          bool
			affectation     bool
		}
		attaques := []attaque{
			{"GET", "/api/v1/services/" + svcA, "", false, false},
			{"PUT", "/api/v1/services/" + svcA, catalog.CreateServiceRequest{Name: "Piraté", Category: catalog.CategoryDryClean, UnitPrice: 1, Unit: "pièce", EstimatedHours: 1}, false, false},
			{"DELETE", "/api/v1/services/" + svcA, "", false, false},
			{"GET", "/api/v1/customers/" + custA, "", true, false},
			{"PUT", "/api/v1/customers/" + custA, map[string]string{"first_name": "X", "last_name": "Y", "phone": "+225 01"}, true, false},
			{"DELETE", "/api/v1/customers/" + custA, "", true, false},
			{"GET", "/api/v1/price-lists/" + plA, "", false, false},
			{"POST", "/api/v1/price-lists/" + plA + "/versions", "{}", false, false},
			{"GET", "/api/v1/orders/" + ordA, "", false, false},
			{"PATCH", "/api/v1/orders/" + ordA + "/status", map[string]string{"status": "ready"}, false, false},
			{"POST", "/api/v1/orders/" + ordA + "/payments", map[string]interface{}{"amount": 100, "payment_method": "cash"}, false, false},
			{"GET", "/api/v1/orders/" + ordA + "/ticket", "", false, false},
			{"PATCH", "/api/v1/branches/" + brA, map[string]string{"name": "Piraté"}, false, false},
			{"PATCH", "/api/v1/users/" + l.aCaissierID + "/status", map[string]string{"status": "inactive"}, false, false},
			// Affectations aux sites : persistées en base avec contrôle d'appartenance.
			// En mode mémoire, le service des sites ne connaît pas les comptes (limite
			// documentée du mode dégradé) : ces deux contrôles sont sautés.
			{"GET", "/api/v1/users/" + l.aCaissierID + "/branches", "", false, true},
			{"PUT", "/api/v1/users/" + l.aCaissierID + "/branches", map[string]interface{}{"branch_ids": []string{brB}}, false, true},
			{"DELETE", "/api/v1/expenses/" + expA, "", false, false},
		}
		for _, at := range attaques {
			if at.client && !env.persistant {
				continue
			}
			if at.affectation && !env.persistant {
				continue
			}
			rec := env.requete(at.methode, at.chemin, l.bProprietaire, at.corps)
			if rec.Code != http.StatusNotFound {
				t.Errorf("%s %s par un autre établissement : %d, attendu 404 (%s)", at.methode, at.chemin, rec.Code, rec.Body.String())
			}
		}

		// Les listes de B ne contiennent aucun identifiant de A.
		ids := []string{svcA, custA, plA, brA, expA, ordA}
		for _, liste := range []string{"/api/v1/services/", "/api/v1/customers/", "/api/v1/orders/", "/api/v1/price-lists/", "/api/v1/branches/", "/api/v1/expenses/"} {
			rec := env.requete("GET", liste, l.bProprietaire, nil)
			for _, id := range ids {
				if id != "" && strings.Contains(rec.Body.String(), id) {
					t.Errorf("GET %s (établissement B) expose l'identifiant %s de A", liste, id)
				}
			}
		}

		// A n'a pas été altéré par les tentatives de B.
		if rec := env.requete("GET", "/api/v1/orders/"+ordA, a, nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"received"`) {
			t.Errorf("commande de A après tentatives de B : %d %s (statut attendu inchangé)", rec.Code, rec.Body.String())
		}
		if rec := env.requete("GET", "/api/v1/services/"+svcA, a, nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"unit_price":1500`) {
			t.Errorf("prestation de A après tentatives de B : %d %s (prix attendu 1500)", rec.Code, rec.Body.String())
		}
		if env.persistant {
			if rec := env.requete("GET", "/api/v1/customers/"+custA, a, nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Kouadio") {
				t.Errorf("client de A après tentatives de B : %d %s", rec.Code, rec.Body.String())
			}
		}
		if rec := env.requete("GET", "/api/v1/expenses/", a, nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), expA) {
			t.Errorf("dépense de A après tentatives de B : %d (dépense absente)", rec.Code)
		}
	})
}

// TestIsolation_XTenantIDUsurpe : un établissement ne peut pas viser un autre via X-Tenant-ID.
func TestIsolation_XTenantIDUsurpe(t *testing.T) {
	pourChaqueMode(t, func(t *testing.T, env *environnement) {
		l := preparerLocataires(t, env)
		for _, e := range matriceRoutes {
			if e.publique || (!e.metier && !equivalentAdmin(e.roles)) {
				continue
			}
			rec := env.requete(e.methode, exemple(e.modele), l.bProprietaire, "{}", "X-Tenant-ID", l.aTenant)
			if rec.Code != http.StatusForbidden {
				t.Errorf("%s %s avec X-Tenant-ID d'un autre établissement : %d, attendu 403", e.methode, e.modele, rec.Code)
			}
		}
		// Contrôle positif : le même établissement, déclaré explicitement, passe.
		if rec := env.requete("GET", "/api/v1/services/", l.bProprietaire, nil, "X-Tenant-ID", l.bTenant); rec.Code != http.StatusOK {
			t.Errorf("X-Tenant-ID identique au jeton : %d, attendu 200", rec.Code)
		}
	})
}

// TestSuspension_GeleLesModulesMetier : un abonnement suspendu gèle les modules métier
// (402) mais laisse le pressing consulter sa situation (compte, droits).
func TestSuspension_GeleLesModulesMetier(t *testing.T) {
	pourChaqueMode(t, func(t *testing.T, env *environnement) {
		if !env.persistant {
			// Le mode mémoire attribue des identifiants d'établissement non-UUID, refusés
			// par la route d'administration (400). La suspension se teste donc en base.
			t.Skip("suspension : contrôle exécuté en mode PostgreSQL (identifiants UUID)")
		}
		l := preparerLocataires(t, env)
		superadmin := env.connexion("admin@pressingos.ci", "admin123")

		if rec := env.requete("PATCH", "/api/v1/admin/tenants/"+l.bTenant+"/status", superadmin,
			map[string]string{"status": "suspended", "reason": "test de suspension"}); rec.Code != http.StatusOK {
			t.Fatalf("suspension par l'éditeur : %d %s", rec.Code, rec.Body.String())
		}

		for _, chemin := range []string{"/api/v1/services/", "/api/v1/orders/", "/api/v1/customers/", "/api/v1/expenses/"} {
			rec := env.requete("GET", chemin, l.bProprietaire, nil)
			if rec.Code != http.StatusPaymentRequired {
				t.Errorf("GET %s abonnement suspendu : %d, attendu 402", chemin, rec.Code)
			}
		}
		for _, chemin := range []string{"/api/v1/auth/me", "/api/v1/tenant/current", "/api/v1/me/entitlements"} {
			rec := env.requete("GET", chemin, l.bProprietaire, nil)
			if rec.Code != http.StatusOK {
				t.Errorf("GET %s abonnement suspendu : %d, attendu 200 (consultation de la situation)", chemin, rec.Code)
			}
		}

		if rec := env.requete("PATCH", "/api/v1/admin/tenants/"+l.bTenant+"/status", superadmin,
			map[string]string{"status": "active"}); rec.Code != http.StatusOK {
			t.Fatalf("réactivation : %d %s", rec.Code, rec.Body.String())
		}
		if rec := env.requete("GET", "/api/v1/services/", l.bProprietaire, nil); rec.Code != http.StatusOK {
			t.Errorf("après réactivation : %d, attendu 200", rec.Code)
		}
	})
}

// TestPlan_FormuleFree_RefuseLesModulesPayants : sans formule payante, les modules
// payants répondent 402 ; les données restent consultables.
func TestPlan_FormuleFree_RefuseLesModulesPayants(t *testing.T) {
	pourChaqueMode(t, func(t *testing.T, env *environnement) {
		l := preparerLocataires(t, env)
		c := l.cProprietaire

		cas := []struct{ methode, chemin, corps string }{
			{"POST", "/api/v1/expenses/", `{"category":"energie_eau","label":"Facture","amount_xof":1000}`},
			{"POST", "/api/v1/notifications/", `{"order_id":"x","channel":"whatsapp","template":"pret"}`},
			{"GET", "/api/v1/reports/summary", ""},
			{"GET", "/api/v1/reports/export", ""},
		}
		for _, x := range cas {
			rec := env.requete(x.methode, x.chemin, c, x.corps)
			if rec.Code != http.StatusPaymentRequired {
				t.Errorf("%s %s en FREE : %d, attendu 402 (%s)", x.methode, x.chemin, rec.Code, rec.Body.String())
			}
		}

		// Plafond de grilles tarifaires : FREE = 1 (mesuré en base uniquement).
		if env.quotas {
			if rec := env.requete("POST", "/api/v1/price-lists/", c, pricelist.CreatePriceListRequest{Name: "Première"}); rec.Code != http.StatusCreated {
				t.Fatalf("première grille en FREE : %d %s", rec.Code, rec.Body.String())
			}
			if rec := env.requete("POST", "/api/v1/price-lists/", c, pricelist.CreatePriceListRequest{Name: "Seconde"}); rec.Code != http.StatusPaymentRequired {
				t.Errorf("seconde grille en FREE : %d, attendu 402", rec.Code)
			}
		} else {
			t.Log("plafonds de grilles non mesurés en mode mémoire : contrôle sauté")
		}

		// Les services et commandes restent accessibles en FREE.
		for _, chemin := range []string{"/api/v1/services/", "/api/v1/orders/", "/api/v1/price-lists/"} {
			if rec := env.requete("GET", chemin, c, nil); rec.Code != http.StatusOK {
				t.Errorf("GET %s en FREE : %d, attendu 200", chemin, rec.Code)
			}
		}
	})
}

// TestPlan_Downgrade_ConserveLesDonnees : après rétrogradation, les données restent
// consultables et seules les nouvelles créations au-delà du plafond sont refusées (PLAN §3.3).
func TestPlan_Downgrade_ConserveLesDonnees(t *testing.T) {
	pourChaqueMode(t, func(t *testing.T, env *environnement) {
		l := preparerLocataires(t, env)
		b := l.bProprietaire

		plA := env.idDe(env.requete("POST", "/api/v1/price-lists/", b, pricelist.CreatePriceListRequest{Name: "Historique 1"}))
		env.idDe(env.requete("POST", "/api/v1/price-lists/", b, pricelist.CreatePriceListRequest{Name: "Historique 2"}))
		expB := env.idDe(env.requete("POST", "/api/v1/expenses/", b, expense.CreateExpenseRequest{
			Category: "energie_eau", Label: "Facture historique", AmountXOF: 9000,
		}))

		env.formule(l.bTenant, billing.PlanFree)

		if rec := env.requete("GET", "/api/v1/price-lists/"+plA, b, nil); rec.Code != http.StatusOK {
			t.Errorf("grille historique après rétrogradation : %d, attendu 200", rec.Code)
		}
		if rec := env.requete("GET", "/api/v1/price-lists/", b, nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), plA) {
			t.Errorf("liste des grilles après rétrogradation : %d (historique absent)", rec.Code)
		}
		if env.quotas {
			if rec := env.requete("POST", "/api/v1/price-lists/", b, pricelist.CreatePriceListRequest{Name: "Nouvelle"}); rec.Code != http.StatusPaymentRequired {
				t.Errorf("nouvelle grille au-delà du plafond FREE : %d, attendu 402", rec.Code)
			}
		}
		if rec := env.requete("GET", "/api/v1/expenses/", b, nil); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), expB) {
			t.Errorf("dépenses historiques après rétrogradation : %d (dépense %s absente)", rec.Code, expB)
		}
		// Décision D6 (en attente) : suppression d'une dépense après rétrogradation. Le
		// comportement observé est consigné, il n'est pas encore une règle approuvée.
		t.Logf("D6 observé : DELETE /expenses/{id} après rétrogradation = %d",
			env.requete("DELETE", "/api/v1/expenses/"+expB, b, nil).Code)
	})
}

// TestAdmin_RoutesPlateforme : les routes d'administration sont réservées à l'éditeur.
func TestAdmin_RoutesPlateforme(t *testing.T) {
	pourChaqueMode(t, func(t *testing.T, env *environnement) {
		l := preparerLocataires(t, env)
		superadmin := env.connexion("admin@pressingos.ci", "admin123")

		for _, chemin := range []string{"/api/v1/admin/stats", "/api/v1/admin/tenants"} {
			if rec := env.requete("GET", chemin, superadmin, nil); rec.Code != http.StatusOK {
				t.Errorf("GET %s (éditeur) : %d, attendu 200", chemin, rec.Code)
			}
		}

		// Identifiants mal formés : rejet de validation (400), jamais une panne (500).
		for _, rec := range []*httptest.ResponseRecorder{
			env.requete("PATCH", "/api/v1/admin/tenants/pas-un-uuid/status", superadmin, map[string]string{"status": "active"}),
			env.requete("DELETE", "/api/v1/admin/tenants/pas-un-uuid", superadmin, nil),
		} {
			if rec.Code != http.StatusBadRequest {
				t.Errorf("identifiant d'établissement mal formé : %d, attendu 400 (%s)", rec.Code, rec.Body.String())
			}
		}

		// Établissement inconnu (UUID valide) : 404.
		inconnu := "00000000-0000-4000-8000-0000000000ff"
		if rec := env.requete("DELETE", "/api/v1/admin/tenants/"+inconnu, superadmin, nil); rec.Code != http.StatusNotFound {
			t.Errorf("suppression d'un établissement inconnu : %d, attendu 404", rec.Code)
		}

		// Le propriétaire d'un établissement ne voit pas l'annuaire de la plateforme.
		if rec := env.requete("GET", "/api/v1/admin/tenants", l.bProprietaire, nil); rec.Code != http.StatusForbidden {
			t.Errorf("annuaire plateforme pour un propriétaire : %d, attendu 403", rec.Code)
		}
	})
}
