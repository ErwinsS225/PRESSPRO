// Package pgutil — petits utilitaires partagés d'accès à PostgreSQL.
//
// Le produit accepte deux familles d'identités : les comptes réels (UUID de la
// table users) et les comptes de démonstration de la vitrine, dont les
// identifiants sont des chaînes mémoire (« usr-gerant-001 »). Une colonne UUID
// ne peut pas accueillir ces derniers : les modules les convertissent en NULL
// plutôt que de laisser PostgreSQL rejeter la requête (SQLSTATE 22P02).
package pgutil

import (
	"regexp"
	"strings"
)

var uuidRE = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// IsUUID indique si la chaîne est un UUID exploitable en base.
func IsUUID(valeur string) bool {
	return uuidRE.MatchString(strings.TrimSpace(valeur))
}

// NullableUUID retourne la valeur utilisable dans une colonne UUID : l'UUID
// débarrassé de ses espaces, ou nil (NULL en base) pour tout autre identifiant.
func NullableUUID(valeur string) interface{} {
	candidat := strings.TrimSpace(valeur)
	if !uuidRE.MatchString(candidat) {
		return nil
	}
	return candidat
}
