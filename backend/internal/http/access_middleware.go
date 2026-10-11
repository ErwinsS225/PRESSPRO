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
//   - Compte de démonstration déclaré (vitrine embarquée, absente de la base)
//     -> autorisé NOMINATIVEMENT (jamais « tout identifiant inconnu »).
//   - Utilisateur inconnu de la base et non déclaré, identifiant hors format
//     UUID, appartenance suspendue, ou toute erreur de base -> REFUS.
//   - Mode dégradé (aucune base : pool nil) -> aucune vérification possible, le
//     JWT signé portant le tenant_id vaut appartenance (mode démonstration).
type MembershipChecker struct {
	pool *pgxpool.Pool

	// isDemoAccount reconnaît les identifiants des comptes de démonstration
	// embarqués (auth.Service.IsDemoAccount). Nil => mode strict : tout
	// utilisateur absent de la base est refusé.
	isDemoAccount func(userID string) bool
}

// NewMembershipChecker construit le contrôleur.
//
// La vérification est STRICTE dès qu'un pool est fourni (la base fait foi) ;
// pool nil = mode dégradé (démonstration, aucune vérification possible).
// isDemoAccount désigne les seuls comptes de vitrine dispensés de base.
func NewMembershipChecker(pool *pgxpool.Pool, isDemoAccount func(userID string) bool) *MembershipChecker {
	return &MembershipChecker{pool: pool, isDemoAccount: isDemoAccount}
}

// estCompteDemo applique le prédicat sans risque de nil (jamais de panique).
func (c *MembershipChecker) estCompteDemo(userID string) bool {
	return c != nil && c.isDemoAccount != nil && c.isDemoAccount(userID)
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
		// Mode dégradé (aucune base) : le JWT vérifié portant le tenant vaut
		// appartenance — c'est le mode démonstration complet.
		return true
	}
	if role == auth.RoleSuperAdmin {
		return true
	}
	// Comptes de vitrine : autorisés nominativement, ils n'existent pas en base.
	if c.estCompteDemo(userID) {
		return true
	}
	// Identifiant hors format UUID : aucun utilisateur réel ne peut le porter
	// (les identifiants persistés sont des UUID) -> refus, sans requête DB.
	if !looksLikeUUID(userID) {
		return false
	}

	var state string
	// LEFT JOIN : s'il existe sans membership -> '__nomembership__' (refusé).
	err := c.pool.QueryRow(ctx, `
		SELECT COALESCE(tm.status, $3)
		FROM users u
		LEFT JOIN tenant_memberships tm
		       ON tm.user_id = u.id AND tm.tenant_id = $1
		WHERE u.id = $2
	`, tenantID, userID, membershipStateInternal).Scan(&state)

	if err == pgx.ErrNoRows {
		// Utilisateur inconnu de la base et non déclaré comme compte de
		// démonstration : jeton orphelin (compte supprimé, autre instance).
		return false
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
	// Propriétaire et superadmin couvrent l'ensemble des branches de leur tenant
	// (l'isolation inter-tenant reste garantie par le tenant_id des claims).
	if role == auth.RoleSuperAdmin || role == auth.RoleOwner {
		return true
	}
	// Comptes de vitrine : autorisés nominativement (absents de la base).
	if c.estCompteDemo(userID) {
		return true
	}
	// Branche ou utilisateur hors format UUID : refus (fail-closed) — un
	// identifiant non-UUID ne correspond à aucun établissement persisté.
	if !looksLikeUUID(branchID) || !looksLikeUUID(userID) {
		return false
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
