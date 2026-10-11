package report

import (
	"bytes"
	"fmt"
	"strings"
)

// Ce fichier produit un PDF 1.4 valide sans dépendance externe : la chaîne de
// rapports doit rester compilable hors réseau, et le document se limite à du
// texte Helvetica (encodage WinAnsi, donc les accents français passent).

type pdfLigne struct {
	Texte  string
	Gras   bool
	Taille int
	Espace int // espacement supplémentaire avant la ligne
}

// Construit le contenu texte du rapport, page par page.
func (s *Summary) pagesPDF(tenantName string) [][]pdfLigne {
	entete := []pdfLigne{
		{Texte: "Rapport d'activité — " + tenantName, Gras: true, Taille: 16, Espace: 0},
		{Texte: fmt.Sprintf("Période du %s au %s", s.Period.From, s.Period.To), Taille: 10, Espace: 6},
		{Texte: "Source de données : " + s.DataSource, Taille: 10, Espace: 0},
	}

	corps := []pdfLigne{
		{Texte: "Synthèse", Gras: true, Taille: 12, Espace: 16},
		{Texte: "Chiffre d'affaires encaissé : " + montant(s.RevenueXOF) + " FCFA", Taille: 11, Espace: 4},
		{Texte: fmt.Sprintf("Commandes : %d   ·   Ticket moyen : %s FCFA", s.OrdersCount, montant(s.AverageTicketXOF)), Taille: 11, Espace: 0},
		{Texte: "Dépenses : " + montant(s.ExpensesXOF) + " FCFA", Taille: 11, Espace: 0},
		{Texte: "Marge (CA encaissé - dépenses) : " + montant(s.MarginXOF) + " FCFA", Gras: true, Taille: 11, Espace: 0},
	}

	if len(s.OrdersByStatus) > 0 {
		corps = append(corps, pdfLigne{Texte: "Commandes par statut", Gras: true, Taille: 12, Espace: 16})
		for _, statut := range statutsOrdonnes {
			if n, ok := s.OrdersByStatus[statut]; ok {
				corps = append(corps, pdfLigne{Texte: fmt.Sprintf("  %s : %d", libelleStatut(statut), n), Taille: 11})
			}
		}
	}

	if len(s.ExpensesByCategory) > 0 {
		corps = append(corps, pdfLigne{Texte: "Dépenses par poste", Gras: true, Taille: 12, Espace: 16})
		for _, c := range s.ExpensesByCategory {
			corps = append(corps, pdfLigne{Texte: fmt.Sprintf("  %-28s %s FCFA (%d)", libelleCategorie(c.Category), montant(c.TotalXOF), c.Count), Taille: 10})
		}
	}

	if len(s.TopServices) > 0 {
		corps = append(corps, pdfLigne{Texte: "Prestations les plus vendues", Gras: true, Taille: 12, Espace: 16})
		for _, p := range s.TopServices {
			corps = append(corps, pdfLigne{Texte: fmt.Sprintf("  %-28s x%-4d %s FCFA", tronque(p.Label, 28), p.Count, montant(p.RevenueXOF)), Taille: 10})
		}
	}

	if s.Consolidated && len(s.ByBranch) > 0 {
		corps = append(corps, pdfLigne{Texte: "Consolidation par établissement", Gras: true, Taille: 12, Espace: 16})
		for _, b := range s.ByBranch {
			corps = append(corps, pdfLigne{
				Texte:  fmt.Sprintf("  %-24s %3d cmd   CA %s   Dép. %s   Marge %s", tronque(b.Name, 24), b.OrdersCount, montant(b.RevenueXOF), montant(b.ExpensesXOF), montant(b.MarginXOF)),
				Taille: 9,
			})
		}
	} else if s.PlanHint != "" {
		corps = append(corps, pdfLigne{Texte: s.PlanHint, Taille: 10, Espace: 14})
	}

	corps = append(corps, pdfLigne{Texte: "Généré par PressingOS", Taille: 8, Espace: 24})

	// Pagination simple : 46 lignes utiles par page A4.
	const parPage = 46
	var pages [][]pdfLigne
	restant := corps
	for len(restant) > 0 {
		n := len(restant)
		if n > parPage {
			n = parPage
		}
		page := append([]pdfLigne{}, entete...)
		page = append(page, restant[:n]...)
		pages = append(pages, page)
		restant = restant[n:]
	}
	if len(pages) == 0 {
		pages = append(pages, entete)
	}
	return pages
}

// EcrirePDF assemble un document PDF 1.4 valide (une page par bloc de 46 lignes).
func (s *Summary) EcrirePDF(w *bytes.Buffer, tenantName string) error {
	pages := s.pagesPDF(tenantName)

	objets := make([]string, 0, 8+len(pages)*2)
	ajoute := func(corps string) int {
		objets = append(objets, corps)
		return len(objets)
	}

	// 1) Catalogue, 2) Pages, puis les pages et leurs flux, enfin les polices.
	catalogue := ajoute("<< /Type /Catalog /Pages 2 0 R >>")
	pagesRef := ajoute("") // rempli après la création des pages
	policeNormale := 0
	policeGras := 0
	refsPages := make([]int, 0, len(pages))

	for _, lignes := range pages {
		flux := contenuPage(lignes)
		refFlux := ajoute(fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(flux), flux))
		if policeNormale == 0 {
			policeNormale = ajoute("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>")
			policeGras = ajoute("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>")
		}
		refsPages = append(refsPages, ajoute(fmt.Sprintf(
			"<< /Type /Page /Parent %d 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 %d 0 R /F2 %d 0 R >> >> /Contents %d 0 R >>",
			pagesRef, policeNormale, policeGras, refFlux)))
	}
	if policeNormale == 0 {
		policeNormale = ajoute("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>")
		policeGras = ajoute("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica-Bold /Encoding /WinAnsiEncoding >>")
	}

	var enfants []string
	for _, ref := range refsPages {
		enfants = append(enfants, fmt.Sprintf("%d 0 R", ref))
	}
	objets[pagesRef-1] = fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(enfants, " "), len(refsPages))
	objets[catalogue-1] = fmt.Sprintf("<< /Type /Catalog /Pages %d 0 R >>", pagesRef)

	// Assemblage : en-tête, objets, table des références croisées, trailer.
	var doc bytes.Buffer
	doc.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	offsets := make([]int, len(objets)+1)
	for i, corps := range objets {
		offsets[i+1] = doc.Len()
		fmt.Fprintf(&doc, "%d 0 obj\n%s\nendobj\n", i+1, corps)
	}
	debutXref := doc.Len()
	fmt.Fprintf(&doc, "xref\n0 %d\n", len(objets)+1)
	doc.WriteString("0000000000 65535 f \n")
	for i := 1; i <= len(objets); i++ {
		fmt.Fprintf(&doc, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&doc, "trailer\n<< /Size %d /Root %d 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objets)+1, catalogue, debutXref)

	_, err := w.Write(doc.Bytes())
	return err
}

// contenuPage transforme les lignes en flux de contenu PDF.
func contenuPage(lignes []pdfLigne) string {
	var b strings.Builder
	y := 792.0
	b.WriteString("BT\n")
	for _, ligne := range lignes {
		taille := ligne.Taille
		if taille == 0 {
			taille = 10
		}
		y -= float64(taille+ligne.Espace) + 2
		police := "/F1"
		if ligne.Gras {
			police = "/F2"
		}
		fmt.Fprintf(&b, "%s %d Tf\n1 0 0 1 50 %.1f Tm\n(%s) Tj\n", police, taille, y, echappePDF(ligne.Texte))
	}
	b.WriteString("ET")
	return b.String()
}

// cp1252Speciaux : caractères typographiques français absents de Latin-1 mais
// présents dans WinAnsiEncoding (tirets, guillemets courbes, œ, €…).
var cp1252Speciaux = map[rune]byte{
	'€': 0x80, '‚': 0x82, 'ƒ': 0x83, '„': 0x84, '…': 0x85, '†': 0x86, '‡': 0x87,
	'ˆ': 0x88, '‰': 0x89, 'Š': 0x8A, '‹': 0x8B, 'Œ': 0x8C, 'Ž': 0x8E,
	'‘': 0x91, '’': 0x92, '“': 0x93, '”': 0x94, '•': 0x95, '–': 0x96, '—': 0x97,
	'˜': 0x98, '™': 0x99, 'š': 0x9A, '›': 0x9B, 'œ': 0x9C, 'ž': 0x9E, 'Ÿ': 0x9F,
}

// echappePDF applique l'échappement des chaînes PDF et convertit en WinAnsi :
// les accents français tiennent sur un octet, le flux reste lisible par
// n'importe quel lecteur PDF sans police embarquée.
func echappePDF(v string) string {
	var b strings.Builder
	for _, r := range v {
		switch {
		case r == '(':
			b.WriteString(`\(`)
		case r == ')':
			b.WriteString(`\)`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\u00a0': // espace insécable -> espace simple
			b.WriteString(" ")
		case r >= 32 && r < 127:
			b.WriteByte(byte(r))
		case r >= 160 && r <= 255: // Latin-1 : identique à WinAnsi
			b.WriteByte(byte(r))
		default:
			if c, ok := cp1252Speciaux[r]; ok {
				b.WriteByte(c)
				continue
			}
			// Hors WinAnsi : remplacé pour ne jamais corrompre le flux.
			b.WriteByte('?')
		}
	}
	return b.String()
}

// tronque limite un libellé pour préserver l'alignement des colonnes.
func tronque(v string, max int) string {
	runes := []rune(v)
	if len(runes) <= max {
		return v
	}
	if max <= 1 {
		return string(runes[:max])
	}
	return string(runes[:max-1]) + "."
}
