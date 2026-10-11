package expense

import "testing"

// TestUUIDPourColonne vérifie la conversion d'un identifiant d'auteur en valeur
// de colonne UUID : les comptes de démonstration (identifiants mémoire) ne
// doivent jamais faire échouer l'enregistrement d'une dépense.
func TestUUIDPourColonne(t *testing.T) {
	cas := []struct {
		nom     string
		id      string
		attendu interface{}
	}{
		{"identifiant de démonstration (forme libre)", "usr-gerant-001", nil},
		{"chaîne vide", "", nil},
		{"identifiant non hexadécimal", "zzzzzzzz-0000-0000-0000-000000000000", nil},
		{"espaces autour d'un UUID valide", "  0b1c2d3e-4f50-4612-8374-95a6b7c8d9e0  ", "0b1c2d3e-4f50-4612-8374-95a6b7c8d9e0"},
		{"UUID en majuscules", "0B1C2D3E-4F50-4612-8374-95A6B7C8D9E0", "0B1C2D3E-4F50-4612-8374-95A6B7C8D9E0"},
		{"UUID v4 standard", "8c9d0e1f-2a3b-4c5d-8e9f-0a1b2c3d4e5f", "8c9d0e1f-2a3b-4c5d-8e9f-0a1b2c3d4e5f"},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			if got := uuidPourColonne(c.id); got != c.attendu {
				t.Errorf("uuidPourColonne(%q) = %v, attendu %v", c.id, got, c.attendu)
			}
		})
	}
}
