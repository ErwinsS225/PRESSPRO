//go:build integration

package auth_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"pressing-os/backend/internal/auth"
	"pressing-os/backend/internal/database"
)

// Test d'intégration PostgreSQL de la gestion des comptes employés.
//
// Exécution :
//
//	TEST_DATABASE_URL="postgres://pressing_app:...@localhost:5432/pressing_os?sslmode=disable" \
//	  go test -tags=integration ./internal/auth/...
//
// Il vérifie qu'un compte créé par le gérant reçoit bien ses DEUX appartenances
// (tenant + établissement principal) : sans elles, le contrôle d'appartenance
// (fail-closed) refuserait le compte sur toutes les routes métier.
func TestIntegration_CreateUser_PoseLesAppartenances(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL non défini : test d'intégration ignoré")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	db, err := database.ConnectPool(ctx, dsn)
	if err != nil {
		t.Fatalf("connexion PostgreSQL: %v", err)
	}
	defer db.Close()
	if err := db.Ping(ctx); err != nil {
		t.Fatalf("base injoignable: %v", err)
	}
	pool := db.Pool

	svc := auth.NewService(auth.NewJWTManager("secret-integration-1234567890", time.Hour))
	svc.SetStore(auth.NewStore(database.NewStore(pool)))

	// Établissement éphémère, créé par le test : plus aucune société fictive
	// n'est livrée par les migrations (000015 a retiré le seed de démonstration).
	var tenant string
	if err := pool.QueryRow(ctx,
		`INSERT INTO tenants (name, slug, city, status)
		 VALUES ($1, $2, 'Abidjan', 'active') RETURNING id`,
		"Pressing Intégration Équipe", "it-equipe-"+time.Now().Format("20060102150405.000000")).Scan(&tenant); err != nil {
		t.Fatalf("création de l'établissement de test: %v", err)
	}
	// Établissement principal (nécessaire aux appartenances de branche) :
	// l'inscription d'un vrai pressing le crée, le test le fait donc aussi.
	if _, err := pool.Exec(ctx,
		`INSERT INTO branches (tenant_id, name, code, city, is_primary, status)
		 VALUES ($1, 'Établissement principal', 'IT-01', 'Abidjan', true, 'active')`,
		tenant); err != nil {
		t.Fatalf("création de l'établissement principal: %v", err)
	}
	// Nettoyage garanti : l'établissement emporte comptes et appartenances.
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenant)
	}()

	email := "integration.equipe+" + time.Now().Format("20060102150405") + "@test.ci"

	user, err := svc.CreateUser(ctx, tenant, auth.CreateUserRequest{
		FullName: "Employé Intégration", Email: email,
		Phone: "+225 01 02 03 04 05", Role: auth.RoleCashier, Password: "motdepasse123",
	})
	if err != nil {
		t.Fatalf("création de l'employé en base: %v", err)
	}
	// Nettoyage garanti en fin de test (les appartenances suivent en cascade).
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, user.ID)
	}()

	// 1. Le compte est relu avec son établissement et son statut actif.
	relu, _, err := svc.GetUserByID(ctx, user.ID)
	if err != nil || relu == nil {
		t.Fatalf("relecture de l'utilisateur: %v", err)
	}
	if relu.Email != email {
		t.Errorf("email attendu %q, obtenu %q", email, relu.Email)
	}
	if relu.Status != "active" {
		t.Errorf("statut attendu actif, obtenu %q", relu.Status)
	}

	// 2. Le compte peut se connecter immédiatement.
	if _, err := svc.Login(ctx, email, "motdepasse123"); err != nil {
		t.Fatalf("connexion du nouvel employé: %v", err)
	}

	// 3. Les deux appartenances sont posées, actives.
	if n := compter(ctx, t, pool, `SELECT COUNT(*) FROM tenant_memberships WHERE user_id = $1 AND status = 'active'`, user.ID); n != 1 {
		t.Errorf("appartenance tenant active attendue : 1, obtenue %d", n)
	}
	if n := compter(ctx, t, pool, `SELECT COUNT(*) FROM branch_memberships WHERE user_id = $1`, user.ID); n < 1 {
		t.Errorf("appartenance à l'établissement principal attendue, obtenue %d", n)
	}

	// 4. La désactivation suspend l'appartenance : le contrôle fail-closed
	//    refuse alors les routes métier, et la connexion est bloquée.
	if _, err := svc.SetUserStatus(ctx, tenant, user.ID, "inactive"); err != nil {
		t.Fatalf("désactivation: %v", err)
	}
	if _, err := svc.Login(ctx, email, "motdepasse123"); err == nil {
		t.Error("un compte désactivé ne doit plus obtenir de jeton")
	}
	if n := compter(ctx, t, pool, `SELECT COUNT(*) FROM tenant_memberships WHERE user_id = $1 AND status = 'active'`, user.ID); n != 0 {
		t.Errorf("l'appartenance doit être suspendue, %d active(s) trouvée(s)", n)
	}

	// 5. Isolation : un autre établissement ne peut pas modifier ce compte.
	if _, err := svc.SetUserStatus(ctx, "b62709cf-0fbf-4db2-aa56-d861b7930284", user.ID, "active"); err == nil {
		t.Error("un autre établissement ne doit pas pouvoir modifier ce compte")
	}
}

func compter(ctx context.Context, t *testing.T, pool *pgxpool.Pool, query string, args ...interface{}) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, query, args...).Scan(&n); err != nil {
		t.Fatalf("requête de comptage: %v", err)
	}
	return n
}
