package report_test

import (
	"bytes"
	"encoding/csv"
	"strconv"
	"strings"
	"testing"

	"pressing-os/backend/internal/report"
)

// rapportComplet construit un rapport représentatif (accents, parenthèses,
// libellés longs, consolidation) pour éprouver les deux formats d'export.
func rapportComplet() *report.Summary {
	return &report.Summary{
		Period:           report.Period{From: "2026-10-01", To: "2026-10-31"},
		DataSource:       "postgresql",
		RevenueXOF:       175000,
		OrdersCount:      12,
		OrdersByStatus:   map[string]int{"ready": 3, "delivered": 8, "washing": 1},
		AverageTicketXOF: 14583,
		ExpensesXOF:      19000,
		MarginXOF:        156000,
		ExpensesByCategory: []report.CategoryTotalReport{
			{Category: "matieres_premieres", TotalXOF: 15000, Count: 2},
			{Category: "energie_eau", TotalXOF: 4000, Count: 1},
		},
		TopServices: []report.ServiceTotal{
			{Label: "Chemise (repassage)", Count: 9, RevenueXOF: 4500},
			{Label: "Costume 3 pièces", Count: 2, RevenueXOF: 7000},
		},
		Consolidated: true,
		ByBranch: []report.BranchTotal{
			{Name: "Établissement principal", OrdersCount: 9, RevenueXOF: 120000, ExpensesXOF: 14000, MarginXOF: 106000},
			{Name: "Site Yopougon", OrdersCount: 3, RevenueXOF: 55000, ExpensesXOF: 5000, MarginXOF: 50000},
		},
	}
}

// TestCSV_SeparateurEtContenu : l'export CSV s'ouvre dans Excel français
// (séparateur « ; ») et porte exactement les chiffres de l'écran.
func TestCSV_SeparateurEtContenu(t *testing.T) {
	var tampon bytes.Buffer
	if err := rapportComplet().EcrireCSV(&tampon, "Pressing Ivoire"); err != nil {
		t.Fatalf("écriture CSV: %v", err)
	}

	brut := tampon.String()
	if !strings.HasPrefix(brut, "\ufeff") {
		t.Error("le BOM UTF-8 est requis pour qu'Excel affiche les accents")
	}
	if strings.Contains(brut, ",") && !strings.Contains(brut, ";") {
		t.Error("le séparateur attendu est le point-virgule")
	}

	lecteur := csv.NewReader(strings.NewReader(strings.TrimPrefix(brut, "\ufeff")))
	lecteur.Comma = ';'
	lecteur.FieldsPerRecord = -1 // le rapport mêle titres (1 champ) et tableaux
	lignes, err := lecteur.ReadAll()
	if err != nil {
		t.Fatalf("relecture CSV: %v", err)
	}

	// Première occurrence de chaque libellé : un même mot peut être un titre de
	// section (1 champ) et un en-tête de tableau (plusieurs champs).
	indexees := make(map[string][]string, len(lignes))
	for _, l := range lignes {
		if len(l) > 1 {
			if _, deja := indexees[l[0]]; !deja {
				indexees[l[0]] = l
			}
		}
	}

	attendus := map[string]string{
		"Établissement":               "Pressing Ivoire",
		"Période":                     "2026-10-01 au 2026-10-31",
		"Chiffre d'affaires encaissé": "175\u00a0000",
		"Marge (CA - dépenses)":       "156\u00a0000",
		"Matières premières":          "15\u00a0000",
		"Énergie et eau":              "4\u00a0000",
		"Chemise (repassage)":         "9",
		"Site Yopougon":               "3",
	}
	for cle, valeur := range attendus {
		ligne, ok := indexees[cle]
		if !ok {
			t.Errorf("ligne %q absente du CSV", cle)
			continue
		}
		if ligne[1] != valeur {
			t.Errorf("valeur de %q = %q, attendue %q", cle, ligne[1], valeur)
		}
	}

	if !strings.Contains(brut, "Consolidation par établissement") {
		t.Error("la consolidation doit figurer dans l'export quand la formule l'inclut")
	}
	if !strings.Contains(brut, "Commandes par statut") {
		t.Error("la répartition des commandes doit figurer dans l'export")
	}
	if !strings.Contains(brut, "Généré par PressingOS") {
		t.Error("le pied de fichier est attendu")
	}
}

// TestCSV_HintSansConsolidation : hors formule BUSINESS, l'export porte la note
// commerciale au lieu du tableau consolidé (comme à l'écran).
func TestCSV_HintSansConsolidation(t *testing.T) {
	rapport := rapportComplet()
	rapport.Consolidated = false
	rapport.ByBranch = nil
	rapport.PlanHint = "La consolidation multi-sites est incluse dans la formule BUSINESS."

	var tampon bytes.Buffer
	if err := rapport.EcrireCSV(&tampon, "Pressing Ivoire"); err != nil {
		t.Fatalf("écriture CSV: %v", err)
	}
	brut := tampon.String()
	if strings.Contains(brut, "Consolidation par établissement") {
		t.Error("aucun tableau consolidé ne doit apparaître hors formule BUSINESS")
	}
	if !strings.Contains(brut, "incluse dans la formule BUSINESS") {
		t.Error("la note commerciale doit apparaître dans le fichier")
	}
}

// TestNomFichier : le nom du fichier est sûri (accents, espaces, ponctuation).
func TestNomFichier(t *testing.T) {
	cas := map[string]string{
		"Pressing Ivoire":        "rapport-pressing-ivoire-2026-10-01_2026-10-31.csv",
		"Pressing  Étoile  d'Or": "rapport-pressing-toile-d-or-2026-10-01_2026-10-31.csv",
		"Ci-Dessous":             "rapport-ci-dessous-2026-10-01_2026-10-31.csv",
	}
	for entree, attendu := range cas {
		if got := report.NomFichier(entree, "2026-10-01", "2026-10-31", "csv"); got != attendu {
			t.Errorf("NomFichier(%q) = %q, attendu %q", entree, got, attendu)
		}
	}
	if got := report.NomFichier("Pressing Ivoire", "2026-10-01", "2026-10-31", "pdf"); !strings.HasSuffix(got, ".pdf") {
		t.Errorf("extension PDF attendue, obtenu %q", got)
	}
}

// TestPDF_Structure : le document produit est un PDF 1.4 refermé (xref et
// trailer cohérents), généré sans dépendance externe.
func TestPDF_Structure(t *testing.T) {
	var tampon bytes.Buffer
	if err := rapportComplet().EcrirePDF(&tampon, "Pressing Ivoire"); err != nil {
		t.Fatalf("écriture PDF: %v", err)
	}
	doc := tampon.Bytes()

	if !bytes.HasPrefix(doc, []byte("%PDF-1.4")) {
		t.Fatalf("en-tête PDF attendu, obtenu %q", string(doc[:min(16, len(doc))]))
	}
	for _, attendu := range []string{"/Type /Catalog", "/Type /Pages", "/Type /Page", "/BaseFont /Helvetica", "%%EOF"} {
		if !bytes.Contains(doc, []byte(attendu)) {
			t.Errorf("le PDF doit contenir %q", attendu)
		}
	}

	// startxref doit pointer sur la table des références croisées.
	pos := bytes.LastIndex(doc, []byte("startxref"))
	if pos < 0 {
		t.Fatal("startxref absent")
	}
	reste := strings.TrimSpace(string(doc[pos+len("startxref"):]))
	chiffres := reste
	for i, r := range reste {
		if r < '0' || r > '9' {
			chiffres = reste[:i]
			break
		}
	}
	offset, err := strconv.Atoi(chiffres)
	if err != nil {
		t.Fatalf("offset startxref illisible (%q): %v", chiffres, err)
	}
	if offset <= 0 || offset >= len(doc) || !bytes.HasPrefix(doc[offset:], []byte("xref")) {
		t.Errorf("startxref ne pointe pas sur la table xref (offset %d)", offset)
	}

	// Les accents sont livrés en WinAnsi (é = 0xE9) : « activité » lisible par
	// une visionneuse sans police embarquée.
	if !bytes.Contains(doc, []byte("activit\xe9")) {
		t.Error("les accents doivent être encodés en WinAnsi (0xE9 pour é)")
	}
	if bytes.Contains(doc, []byte("activit\\351")) {
		t.Error("les accents ne doivent pas être doublés par un échappement octal")
	}
}

// TestPDF_Echappement : parenthèses et tirets typographiques ne cassent pas le
// flux de contenu.
func TestPDF_Echappement(t *testing.T) {
	rapport := rapportComplet()
	rapport.TopServices = []report.ServiceTotal{{Label: "Nettoyage à sec (premium) — rapide", Count: 1, RevenueXOF: 5000}}

	var tampon bytes.Buffer
	if err := rapport.EcrirePDF(&tampon, "Pressing (Ivoire) SARL"); err != nil {
		t.Fatalf("écriture PDF: %v", err)
	}
	doc := tampon.Bytes()

	if !bytes.Contains(doc, []byte(`Pressing \(Ivoire\) SARL`)) {
		t.Error("les parenthèses du nom d'établissement doivent être échappées")
	}
	if !bytes.Contains(doc, []byte("premium")) {
		t.Error("le libellé de prestation doit être présent")
	}
	if !bytes.Contains(doc, []byte{0x97}) {
		t.Error("le tiret cadratin (0x97 en WinAnsi) doit être encodé")
	}
}

// TestPDF_Pagination : au-delà de 46 lignes, une seconde page est créée et la
// table des pages le déclare.
func TestPDF_Pagination(t *testing.T) {
	rapport := rapportComplet()
	rapport.ExpensesByCategory = nil
	rapport.OrdersByStatus = nil
	rapport.TopServices = nil
	rapport.ByBranch = nil
	rapport.PlanHint = ""
	for i := 0; i < 60; i++ {
		rapport.TopServices = append(rapport.TopServices, report.ServiceTotal{
			Label: "Prestation " + strconv.Itoa(i), Count: i, RevenueXOF: float64(i * 500),
		})
	}

	var tampon bytes.Buffer
	if err := rapport.EcrirePDF(&tampon, "Pressing Ivoire"); err != nil {
		t.Fatalf("écriture PDF: %v", err)
	}
	doc := string(tampon.Bytes())
	if !strings.Contains(doc, "/Count 2") {
		t.Error("deux pages attendues pour 60 lignes de prestations")
	}
	if strings.Count(doc, "/Type /Page ") < 2 {
		t.Error("les deux objets page doivent être présents")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
