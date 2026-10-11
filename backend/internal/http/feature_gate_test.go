package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"pressing-os/backend/internal/auth"
	"pressing-os/backend/internal/billing"
	apphttp "pressing-os/backend/internal/http"
)

// poolInjoignable retourne un pool NON nil pointant vers un port fermé : il
// permet d'activer le mode « la base fait foi » sans exiger PostgreSQL. Les
// chemins testés ci-dessous refusent avant toute requête réseau.
func poolInjoignable(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(),
		"postgres://utilisateur:motdepasse@127.0.0.1:1/base?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatalf("création du pool de test: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// TestIsMember_FailClosed vérifie que, dès que la base fait foi, seuls les
// comptes de démonstration déclarés et les utilisateurs présents en base
// (ou superadmins) sont acceptés : plus aucun « fail-open » générique.
func TestIsMember_FailClosed(t *testing.T) {
	ctx := context.Background()
	tenant := "a0000000-0000-0000-0000-000000000001"
	demo := func(userID string) bool { return userID == "usr-gerant-001" }

	strict := apphttp.NewMembershipChecker(poolInjoignable(t), demo)
	degrade := apphttp.NewMembershipChecker(nil, demo)

	cas := []struct {
		nom     string
		checker *apphttp.MembershipChecker
		tenant  string
		user    string
		role    string
		attendu bool
	}{
		{"compte de démonstration déclaré", strict, tenant, "usr-gerant-001", auth.RoleOwner, true},
		{"identifiant de démonstration NON déclaré", strict, tenant, "usr-intrus-999", auth.RoleOwner, false},
		{"identifiant arbitraire (jeton forgé)", strict, tenant, "00000000-0000-0000-0000-00000000000X", auth.RoleOperator, false},
		{"superadmin global", strict, tenant, "usr-superadmin-000", auth.RoleSuperAdmin, true},
		{"mode dégradé (aucune base) : compte mémoire", degrade, tenant, "usr-quelconque", auth.RoleOwner, true},
		{"tenant vide hors superadmin", strict, "", "usr-gerant-001", auth.RoleOwner, false},
		{"tenant vide superadmin", strict, "", "usr-superadmin-000", auth.RoleSuperAdmin, true},
	}

	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			got := c.checker.IsMember(ctx, c.tenant, c.user, c.role)
			if got != c.attendu {
				t.Errorf("IsMember(%q, %q, %q) = %v, attendu %v", c.tenant, c.user, c.role, got, c.attendu)
			}
		})
	}
}

// TestHasBranchAccess_FailClosed : un établissement non-UUID ne peut venir que
// d'un client : refus, sauf compte de démonstration ou rôle couvrant.
func TestHasBranchAccess_FailClosed(t *testing.T) {
	ctx := context.Background()
	tenant := "a0000000-0000-0000-0000-000000000001"
	demo := func(userID string) bool { return userID == "usr-gerant-001" }
	strict := apphttp.NewMembershipChecker(poolInjoignable(t), demo)
	degrade := apphttp.NewMembershipChecker(nil, nil)

	// Sans établissement visé, aucune restriction de périmètre.
	if !strict.HasBranchAccess(ctx, tenant, "", "usr-quelconque", auth.RoleOperator) {
		t.Error("branchID vide doit être autorisé (aucune restriction)")
	}
	// Propriétaire : couvre toutes les branches de son établissement.
	if !strict.HasBranchAccess(ctx, tenant, "branche-arbitraire", "uuid-proprietaire", auth.RoleOwner) {
		t.Error("le propriétaire doit couvrir ses établissements")
	}
	// Compte de démonstration déclaré : dispensé de base.
	if !strict.HasBranchAccess(ctx, tenant, "branche-arbitraire", "usr-gerant-001", auth.RoleOperator) {
		t.Error("un compte de démonstration déclaré doit passer")
	}
	// Opérateur inconnu avec une branche non-UUID : refus (fail-closed).
	if strict.HasBranchAccess(ctx, tenant, "branche-arbitraire", "usr-operateur-inconnu", auth.RoleOperator) {
		t.Error("un opérateur inconnu doit être refusé hors comptes de démonstration")
	}
	// Mode dégradé : aucune vérification possible.
	if !degrade.HasBranchAccess(ctx, tenant, "branche-arbitraire", "usr-operateur-inconnu", auth.RoleOperator) {
		t.Error("le mode dégradé ne peut pas vérifier : autorisation")
	}
}

// TestRequireFeatureQuota vérifie le garde-fou commercial sur une route payante.
func TestRequireFeatureQuota(t *testing.T) {
	const tenant = "tenant-quota-test" // sans abonnement : formule FREE

	billingSvc := billing.NewService()
	billingSvc.SetUsageCounter(fixedCounter{priceLists: 1}) // une grille existe déjà

	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	garde := apphttp.RequireFeatureQuota(billingSvc, billing.FeaturePriceLists)

	appel := func(userID, role, tenantID string) (int, map[string]interface{}) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/price-lists", nil)
		claims := &auth.UserClaims{UserID: userID, TenantID: tenantID, Role: role}
		req = req.WithContext(auth.WithUserClaims(req.Context(), claims))
		rec := httptest.NewRecorder()
		garde(ok).ServeHTTP(rec, req)
		var body map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		return rec.Code, body
	}

	// FREE : plafond 1 grille, 1 déjà créée => 402 « passez à une formule supérieure ».
	code, body := appel("usr-gerant-001", auth.RoleOwner, tenant)
	if code != http.StatusPaymentRequired {
		t.Fatalf("plafond FREE atteint : attendu 402, obtenu %d (%v)", code, body)
	}
	if body["upgrade_required"] != "true" || body["feature"] != billing.FeaturePriceLists {
		t.Errorf("charge utile 402 incomplète : %v", body)
	}

	// Le superadmin contourne le garde-fou commercial.
	if code, _ := appel("usr-superadmin-000", auth.RoleSuperAdmin, tenant); code != http.StatusCreated {
		t.Errorf("superadmin : attendu 201, obtenu %d", code)
	}

	// Sans contexte d'établissement : refus explicite, jamais un 500.
	if code, _ := appel("usr-gerant-001", auth.RoleOwner, ""); code != http.StatusForbidden {
		t.Errorf("contexte d'établissement manquant : attendu 403, obtenu %d", code)
	}

	// Sous le plafond (aucune grille) : création autorisée.
	billingSvc.SetUsageCounter(fixedCounter{priceLists: 0})
	if code, body := appel("usr-gerant-001", auth.RoleOwner, tenant); code != http.StatusCreated {
		t.Errorf("sous le plafond : attendu 201, obtenu %d (%v)", code, body)
	}
}

// TestRequireActiveSubscription : un abonnement suspendu par l'éditeur gèle les
// modules métier du pressing, sans bloquer la consultation de son compte ni
// l'administration de la plateforme par le superadmin.
func TestRequireActiveSubscription(t *testing.T) {
	const suspendu = "a0000000-0000-0000-0000-000000000001" // établissement de démonstration
	const autre = "tenant-non-suspendu-0000000000000"

	billingSvc := billing.NewService()
	// L'établissement de test est réellement inscrit : plus aucune entreprise
	// fictive n'est pré-enregistrée dans le moteur de facturation.
	if err := billingSvc.EnsureTrial(context.Background(), suspendu); err != nil {
		t.Fatalf("inscription de l'établissement de test: %v", err)
	}
	if err := billingSvc.SetTenantStatus(context.Background(), suspendu,
		billing.StatusSuspended, "usr-superadmin-000", "impayé (test)"); err != nil {
		t.Fatalf("suspension de l'établissement de test: %v", err)
	}

	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	garde := apphttp.RequireActiveSubscription(billingSvc)

	appel := func(userID, role, tenantID string) int {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
		claims := &auth.UserClaims{UserID: userID, TenantID: tenantID, Role: role}
		req = req.WithContext(auth.WithUserClaims(req.Context(), claims))
		rec := httptest.NewRecorder()
		garde(ok).ServeHTTP(rec, req)
		return rec.Code
	}

	if code := appel("usr-gerant-001", auth.RoleOwner, suspendu); code != http.StatusPaymentRequired {
		t.Errorf("établissement suspendu : attendu 402, obtenu %d", code)
	}
	if code := appel("usr-gerant-001", auth.RoleOwner, autre); code != http.StatusOK {
		t.Errorf("établissement en règle : attendu 200, obtenu %d", code)
	}
	if code := appel("usr-superadmin-000", auth.RoleSuperAdmin, suspendu); code != http.StatusOK {
		t.Errorf("superadmin (inspection) : attendu 200, obtenu %d", code)
	}
}

// TestRequireFeatureQuota_Users vérifie le plafond de comptes employés
// (users.max : 2 en formule gratuite, 5 en PRO, 15 en BUSINESS).
func TestRequireFeatureQuota_Users(t *testing.T) {
	const tenant = "tenant-quota-users-test"

	billingSvc := billing.NewService()
	billingSvc.SetUsageCounter(fixedCounter{users: 2}) // 2 comptes déjà actifs (plafond gratuit)

	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusCreated) })
	garde := apphttp.RequireFeatureQuota(billingSvc, billing.FeatureUsersMax)

	appel := func(role, tenantID string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/users", nil)
		claims := &auth.UserClaims{UserID: "usr-gerant-001", TenantID: tenantID, Role: role}
		req = req.WithContext(auth.WithUserClaims(req.Context(), claims))
		rec := httptest.NewRecorder()
		garde(ok).ServeHTTP(rec, req)
		return rec.Code
	}

	if code := appel(auth.RoleOwner, tenant); code != http.StatusPaymentRequired {
		t.Errorf("plafond gratuit atteint (2 comptes) : attendu 402, obtenu %d", code)
	}
	if code := appel(auth.RoleSuperAdmin, tenant); code != http.StatusCreated {
		t.Errorf("superadmin : attendu 201, obtenu %d", code)
	}

	// Sous le plafond : création autorisée.
	billingSvc.SetUsageCounter(fixedCounter{users: 1})
	if code := appel(auth.RoleOwner, tenant); code != http.StatusCreated {
		t.Errorf("sous le plafond : attendu 201, obtenu %d", code)
	}
}

// TestRequireFeature_RapportsEtDepenses : les modules Rapports et Dépenses sont
// des fonctionnalités payantes — un établissement gratuit reçoit 402 avec le
// message commercial, tandis qu'une formule payante (ou un essai actif, niveau
// BUSINESS) passe.
func TestRequireFeature_RapportsEtDepenses(t *testing.T) {
	const gratuit = "a0000000-0000-0000-0000-000000000001" // formule gratuite, essai terminé
	const payant = "b62709cf-0fbf-4db2-aa56-d861b7930283"  // formule PRO

	billingSvc := billing.NewService()
	// Essai terminé => matrice FREE ; abonnement PRO pour le second établissement.
	billingSvc.ForcePlanForTest(gratuit, billing.PlanFree, billing.StatusActive)
	billingSvc.ForcePlanForTest(payant, billing.PlanPro, billing.StatusActive)

	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	appel := func(garde func(http.Handler) http.Handler, role, tenantID string) (int, map[string]interface{}) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/summary", nil)
		claims := &auth.UserClaims{UserID: "usr-gerant-001", TenantID: tenantID, Role: role}
		req = req.WithContext(auth.WithUserClaims(req.Context(), claims))
		rec := httptest.NewRecorder()
		garde(ok).ServeHTTP(rec, req)

		var corps map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &corps)
		return rec.Code, corps
	}

	// 1. Rapports avancés : refusés en formule gratuite, autorisés en PRO.
	gardeRapports := apphttp.RequireFeature(billingSvc, billing.FeatureReportsAdvanced)
	if code, corps := appel(gardeRapports, auth.RoleOwner, gratuit); code != http.StatusPaymentRequired {
		t.Errorf("rapports en gratuit : attendu 402, obtenu %d (%v)", code, corps)
	} else if corps["feature"] != billing.FeatureReportsAdvanced {
		t.Errorf("la réponse doit désigner la fonctionnalité refusée : %v", corps)
	}
	if code, _ := appel(gardeRapports, auth.RoleOwner, payant); code != http.StatusOK {
		t.Errorf("rapports en PRO : attendu 200, obtenu %d", code)
	}

	// 2. Dépenses avancées : même matrice commerciale.
	gardeDepenses := apphttp.RequireFeature(billingSvc, billing.FeatureExpensesAdvanced)
	if code, _ := appel(gardeDepenses, auth.RoleOwner, gratuit); code != http.StatusPaymentRequired {
		t.Errorf("dépenses en gratuit : attendu 402, obtenu %d", code)
	}
	if code, _ := appel(gardeDepenses, auth.RoleOwner, payant); code != http.StatusOK {
		t.Errorf("dépenses en PRO : attendu 200, obtenu %d", code)
	}

	// 3. Le superadmin de l'éditeur n'est jamais bloqué par la matrice.
	if code, _ := appel(gardeRapports, auth.RoleSuperAdmin, gratuit); code != http.StatusOK {
		t.Errorf("superadmin : attendu 200, obtenu %d", code)
	}
}

// TestRequireFeatureQuota_MultiSites : le plafond d'établissements d'une
// formule doit être appliqué en création (1 site en gratuit et PRO, 3 en
// BUSINESS) — la consommation est celle des établissements réellement ouverts.
func TestRequireFeatureQuota_MultiSites(t *testing.T) {
	const gratuit = "tenant-multisites-gratuit"
	const business = "tenant-multisites-business"

	billingSvc := billing.NewService()
	billingSvc.ForcePlanForTest(gratuit, billing.PlanFree, billing.StatusActive)
	billingSvc.ForcePlanForTest(business, billing.PlanBusiness, billing.StatusActive)
	billingSvc.SetUsageCounter(fixedCounter{branches: 1})

	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusCreated) })
	garde := apphttp.RequireFeatureQuota(billingSvc, billing.FeatureMultiSite)

	appel := func(role, tenantID string) (int, map[string]interface{}) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/branches", nil)
		claims := &auth.UserClaims{UserID: "usr-gerant-001", TenantID: tenantID, Role: role}
		req = req.WithContext(auth.WithUserClaims(req.Context(), claims))
		rec := httptest.NewRecorder()
		garde(ok).ServeHTTP(rec, req)
		var corps map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &corps)
		return rec.Code, corps
	}

	code, corps := appel(auth.RoleOwner, gratuit)
	if code != http.StatusPaymentRequired {
		t.Errorf("2e établissement en formule gratuite : attendu 402, obtenu %d", code)
	}
	if corps["feature"] != billing.FeatureMultiSite {
		t.Errorf("fonctionnalité annoncée = %v, attendue %s", corps["feature"], billing.FeatureMultiSite)
	}

	if code, _ = appel(auth.RoleOwner, business); code != http.StatusCreated {
		t.Errorf("2e établissement en BUSINESS (plafond 3) : attendu 201, obtenu %d", code)
	}

	// Plafond BUSINESS atteint : le 4e établissement est refusé.
	billingSvc.SetUsageCounter(fixedCounter{branches: 3})
	if code, _ = appel(auth.RoleOwner, business); code != http.StatusPaymentRequired {
		t.Errorf("4e établissement en BUSINESS : attendu 402, obtenu %d", code)
	}

	// Le superadmin reste dispensé (administration de la plateforme).
	if code, _ = appel(auth.RoleSuperAdmin, business); code != http.StatusCreated {
		t.Errorf("superadmin : attendu 201, obtenu %d", code)
	}
}
