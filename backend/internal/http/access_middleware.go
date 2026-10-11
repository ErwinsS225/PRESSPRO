package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"pressing-os/backend/internal/auth"
)

// MembershipChecker vérifie l'appartenance à un tenant (contrôle #2) et le
// périmètre d'établissement (contrôle #4) du moteur d'autorisation à 5 contrôles.
//
// Règle de sécurité :
//   - Mode DB (pool fourni)  -> fail-CLOSED. Une appartenance suspendée ou
//     absente pour un utilisateur présent en base est refusée.
//   - Compte démo (user absent de la base, vit en mémoire côté auth) -> fail-OPEN,
//     car un JWT valide portant le tenant_id vaut appartenance dans ce mode.
//   - Mode dégradé (pool nil) -> fail-OPEN (aucune vérification possible).
type MembershipChecker struct {
	pool *pgxpool.Pool
}

// NewMembershipChecker construit le checker. Un pool nil active le mode dégradé.
func NewMembershipChecker(pool *pgxpool.Pool) *MembershipChecker {
	return &MembershipChecker{pool: pool}
}

// membershipStateInternal marque un utilisateur présent en base mais sans
// membership pour ce tenant (incohérence -> refus par sécurité).
const membershipStateInternal = "__nomembership__"

// looksLikeUUID indique si la chaîne respecte la forme canonique d'un UUID.
// Les comptes de démonstration (mémoire) utilisent des identifiants non-UUID
// (ex: "usr-gerant-001") et n'existent donc pas en base : ils sont autorisés
// en mode démo sans requête DB, le JWT signé garantissant déjà l'identité.
// Ce garde évite surtout une erreur de cast UUID côté PostgreSQL.
func looksLikeUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i := 0; i < len(s); i++ {
		switch i {
		case 8, 13, 18, 23:
			if s[i] != '-' {
				return false
			}
		default:
			c := s[i]
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
	}
	return true
}

// IsMember indique si l'utilisateur est membre ACTIF du tenant.
func (c *MembershipChecker) IsMember(ctx context.Context, tenantID, userID, role string) bool {
	if tenantID == "" {
		// Superadmin global : pas de tenant d'appartenance requis.
		return role == auth.RoleSuperAdmin
	}
	if c == nil || c.pool == nil {
		// Mode dégradé : le JWT vérifié portant le tenant vaut appartenance.
		return true
	}
	if role == auth.RoleSuperAdmin {
		return true
	}
	// Compte de démonstration (identifiant mémoire non-UUID) : le JWT signé
	// garantit déjà l'identité, et une comparaison UUID en base échouerait.
	// On autorise sans requête DB (mode démo).
	if !looksLikeUUID(userID) {
		return true
	}

	var state string
	// LEFT JOIN : si l'utilisateur n'existe pas en base -> ErrNoRows (compte démo,
	// autorisé). S'il existe sans membership -> '__nomembership__' (refusé).
	err := c.pool.QueryRow(ctx, `
		SELECT COALESCE(tm.status, $3)
		FROM users u
		LEFT JOIN tenant_memberships tm
		       ON tm.user_id = u.id AND tm.tenant_id = $1
		WHERE u.id = $2
	`, tenantID, userID, membershipStateInternal).Scan(&state)

	if err == pgx.ErrNoRows {
		// Utilisateur inconnu en base : compte de démonstration en mémoire.
		return true
	}
	if err != nil {
		// Erreur DB : on refuse par sécurité (fail-closed).
		return false
	}
	return state == "active"
}

// HasBranchAccess indique si l'utilisateur peut opérer sur la branche visée.
// Un branchID vide signifie « aucune restriction de périmètre » (autorisé).
func (c *MembershipChecker) HasBranchAccess(ctx context.Context, tenantID, branchID, userID, role string) bool {
	if branchID == "" {
		return true
	}
	if c == nil || c.pool == nil {
		return true
	}
	// Identifiants mémoire (démo) ou branche non-UUID : pas de vérification DB
	// possible sans erreur de cast. Le JWT signé garantit déjà l'identité, et
	// l'isolation inter-tenant reste portée par le tenant_id des claims.
	if !looksLikeUUID(branchID) || !looksLikeUUID(userID) {
		return true
	}
	// Propriétaire et superadmin couvrent l'ensemble des branches de leur tenant
	// (l'isolation inter-tenant reste garantie par le tenant_id des claims).
	if role == auth.RoleSuperAdmin || role == auth.RoleOwner {
		return true
	}

	var exists bool
	err := c.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM branch_memberships bm
			JOIN branches b ON b.id = bm.branch_id AND b.tenant_id = $1
			WHERE bm.branch_id = $2 AND bm.user_id = $3
		)
	`, tenantID, branchID, userID).Scan(&exists)
	if err != nil {
		return false
	}
	return exists
}

// RequireTenantMembership implémente le contrôle #2 (appartenance active).
func RequireTenantMembership(checker *MembershipChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			claims := auth.GetUserFromContext(r.Context())
			if claims == nil {
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "authentification requise"})
				return
			}
			if !checker.IsMember(r.Context(), claims.TenantID, claims.UserID, claims.Role) {
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error": "accès refusé : vous n'êtes pas membre actif de cet établissement",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireBranchScope implémente le contrôle #4 (périmètre établissement).
// La branche ciblée est lue depuis l'en-tête X-Branch-ID puis le paramètre
// de requête branch_id. Absente => aucune restriction de périmètre.
func RequireBranchScope(checker *MembershipChecker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			claims := auth.GetUserFromContext(r.Context())
			if claims == nil {
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "authentification requise"})
				return
			}
			branchID := r.Header.Get("X-Branch-ID")
			if branchID == "" {
				branchID = r.URL.Query().Get("branch_id")
			}
			if !checker.HasBranchAccess(r.Context(), claims.TenantID, branchID, claims.UserID, claims.Role) {
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"error": "accès refusé : vous n'avez pas accès à cet établissement",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
