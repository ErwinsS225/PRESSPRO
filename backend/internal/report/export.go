package report

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ExportOptions rassemble ce qui habille un export (identité du pressing et
// période imprimée) — les chiffres, eux, viennent exclusivement du rapport
// calculé par le service : l'écran et le fichier ne peuvent pas diverger.
type ExportOptions struct {
	TenantName string
}

// LignesCSV met le rapport à plat, section par section, dans l'ordre de lecture
// de l'écran Rapports. Le séparateur est le point-virgule : c'est celui
// qu'attend Excel en configuration française.
func (s *Summary) LignesCSV(tenantName string) [][]string {
	lignes := [][]string{
		{"Rapport d'activité PressingOS"},
		{"Établissement", tenantName},
		{"Période", s.Period.From + " au " + s.Period.To},
		{"Source de données", s.DataSource},
		{},
		{"Synthèse"},
		{"Indicateur", "Valeur (FCFA)"},
		{"Chiffre d'affaires encaissé", montant(s.RevenueXOF)},
		{"Commandes", strconv.Itoa(s.OrdersCount)},
		{"Ticket moyen", montant(s.AverageTicketXOF)},
		{"Dépenses", montant(s.ExpensesXOF)},
		{"Marge (CA - dépenses)", montant(s.MarginXOF)},
	}

	if len(s.OrdersByStatus) > 0 {
		lignes = append(lignes, []string{}, []string{"Commandes par statut"}, []string{"Statut", "Nombre"})
		for _, statut := range statutsOrdonnes {
			if n, ok := s.OrdersByStatus[statut]; ok {
				lignes = append(lignes, []string{libelleStatut(statut), strconv.Itoa(n)})
			}
		}
	}

	if len(s.ExpensesByCategory) > 0 {
		lignes = append(lignes, []string{}, []string{"Dépenses par poste"}, []string{"Poste", "Montant (FCFA)", "Écritures"})
		for _, c := range s.ExpensesByCategory {
			lignes = append(lignes, []string{libelleCategorie(c.Category), montant(c.TotalXOF), strconv.Itoa(c.Count)})
		}
	}

	if len(s.TopServices) > 0 {
		lignes = append(lignes, []string{}, []string{"Prestations les plus vendues"},
			[]string{"Prestation", "Quantité", "Chiffre d'affaires (FCFA)"})
		for _, p := range s.TopServices {
			lignes = append(lignes, []string{p.Label, strconv.Itoa(p.Count), montant(p.RevenueXOF)})
		}
	}

	if s.Consolidated && len(s.ByBranch) > 0 {
		lignes = append(lignes, []string{}, []string{"Consolidation par établissement"},
			[]string{"Établissement", "Commandes", "CA encaissé (FCFA)", "Dépenses (FCFA)", "Marge (FCFA)"})
		for _, b := range s.ByBranch {
			lignes = append(lignes, []string{b.Name, strconv.Itoa(b.OrdersCount), montant(b.RevenueXOF), montant(b.ExpensesXOF), montant(b.MarginXOF)})
		}
	} else if s.PlanHint != "" {
		lignes = append(lignes, []string{}, []string{s.PlanHint})
	}

	lignes = append(lignes, []string{}, []string{"Généré par PressingOS"})
	return lignes
}

// EcrireCSV écrit le rapport au format CSV (séparateur « ; », BOM UTF-8 pour
// qu'Excel affiche correctement les accents).
func (s *Summary) EcrireCSV(w io.Writer, tenantName string) error {
	if _, err := io.WriteString(w, "\ufeff"); err != nil {
		return err
	}
	ecrivain := csv.NewWriter(w)
	ecrivain.Comma = ';'
	ecrivain.UseCRLF = true
	if err := ecrivain.WriteAll(s.LignesCSV(tenantName)); err != nil {
		return err
	}
	ecrivain.Flush()
	return ecrivain.Error()
}

// statutsOrdonnes fixe l'ordre d'affichage des statuts de commande.
var statutsOrdonnes = []string{"received", "washing", "ironing", "ready", "delivered", "cancelled"}

// libelleStatut traduit un statut technique en libellé lisible.
func libelleStatut(statut string) string {
	switch statut {
	case "received":
		return "Réceptionné"
	case "washing":
		return "En lavage"
	case "ironing":
		return "En repassage"
	case "ready":
		return "Prêt"
	case "delivered":
		return "Livré"
	case "cancelled":
		return "Annulé"
	default:
		return statut
	}
}

// LibelleCategorie traduit un poste de dépense en libellé lisible.
func libelleCategorie(categorie string) string {
	switch categorie {
	case "matieres_premieres":
		return "Matières premières"
	case "produits_entretien":
		return "Produits d'entretien"
	case "loyer":
		return "Loyer"
	case "energie_eau":
		return "Énergie et eau"
	case "salaires":
		return "Salaires"
	case "transport":
		return "Transport"
	case "equipement":
		return "Équipement"
	case "maintenance":
		return "Maintenance"
	case "taxes":
		return "Taxes et impôts"
	case "divers":
		return "Divers"
	default:
		return categorie
	}
}

// montant formate un montant en FCFA sans décimale (les francs CFA sont
// entiers) avec un espace insécable comme séparateur de milliers.
func montant(v float64) string {
	negatif := v < 0
	if negatif {
		v = -v
	}
	entier := strconv.FormatInt(int64(v+0.5), 10)
	var groupes []string
	for len(entier) > 3 {
		groupes = append([]string{entier[len(entier)-3:]}, groupes...)
		entier = entier[:len(entier)-3]
	}
	groupes = append([]string{entier}, groupes...)
	out := strings.Join(groupes, "\u00a0")
	if negatif {
		out = "-" + out
	}
	return out
}

// NomFichier construit le nom du fichier exporté, sans caractère interdit.
func NomFichier(tenantName, from, to, extension string) string {
	base := "rapport-" + slug(tenantName) + "-" + from + "_" + to + "." + extension
	return base
}

// slug réduit un libellé à une suite de minuscules et de tirets.
func slug(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	var b strings.Builder
	dernierTiret := false
	for _, r := range v {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dernierTiret = false
		case b.Len() > 0 && !dernierTiret:
			b.WriteRune('-')
			dernierTiret = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// enteteExport est le bloc d'identité commun aux deux formats.
func (s *Summary) enteteExport(tenantName string) []string {
	return []string{
		"Rapport d'activité — " + tenantName,
		fmt.Sprintf("Période du %s au %s", s.Period.From, s.Period.To),
		"Source de données : " + s.DataSource,
	}
}
