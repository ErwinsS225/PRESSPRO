package report

import "errors"

var (
	// ErrInvalidPeriod : bornes de période illisibles ou inversées.
	ErrInvalidPeriod = errors.New("période invalide (format AAAA-MM-JJ attendu, début antérieur ou égal à la fin)")
	// ErrReportUnavailable : aucune source de données n'est branchée (pas de
	// base et pas de service métier) : les rapports ne peuvent pas être calculés.
	ErrReportUnavailable = errors.New("rapports indisponibles : aucune source de données")
)

// Period borne le rapport (dates incluses, format AAAA-MM-JJ).
type Period struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// ServiceTotal agrège les prestations les plus vendues de la période.
type ServiceTotal struct {
	Label      string  `json:"label"`
	Count      int     `json:"count"`
	RevenueXOF float64 `json:"revenue_xof"`
}

// BranchTotal consolide l'activité d'un établissement (multi-sites).
type BranchTotal struct {
	BranchID    string  `json:"branch_id"`
	Name        string  `json:"name"`
	OrdersCount int     `json:"orders_count"`
	RevenueXOF  float64 `json:"revenue_xof"`
	ExpensesXOF float64 `json:"expenses_xof"`
	MarginXOF   float64 `json:"margin_xof"`
}

// Summary est le rapport d'activité d'un pressing sur une période :
// chiffre d'affaires encaissé, commandes, dépenses et marge.
type Summary struct {
	Period             Period                `json:"period"`
	DataSource         string                `json:"data_source"` // postgresql | memoire
	RevenueXOF         float64               `json:"revenue_xof"`
	OrdersCount        int                   `json:"orders_count"`
	OrdersByStatus     map[string]int        `json:"orders_by_status"`
	AverageTicketXOF   float64               `json:"average_ticket_xof"`
	ExpensesXOF        float64               `json:"expenses_xof"`
	MarginXOF          float64               `json:"margin_xof"`
	ExpensesByCategory []CategoryTotalReport `json:"expenses_by_category"`
	TopServices        []ServiceTotal        `json:"top_services"`
	/** Consolidated : la consolidation multi-sites est incluse dans la formule. */
	Consolidated bool          `json:"consolidated"`
	ByBranch     []BranchTotal `json:"by_branch,omitempty"`
	/** PlanHint explique l'absence de consolidation (message commercial). */
	PlanHint string `json:"plan_hint,omitempty"`
}

// CategoryTotalReport : dépenses agrégées par poste.
type CategoryTotalReport struct {
	Category string  `json:"category"`
	TotalXOF float64 `json:"total_xof"`
	Count    int     `json:"count"`
}
