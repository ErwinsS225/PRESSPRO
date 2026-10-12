package billing

import "time"

// Codes de plans commerciaux (spec MAJ.md section 1-2).
const (
	PlanFree     = "FREE"
	PlanPro      = "PRO"
	PlanBusiness = "BUSINESS"
)

// Statuts d'abonnement possibles.
const (
	StatusTrialing  = "TRIALING"
	StatusActive    = "ACTIVE"
	StatusPastDue   = "PAST_DUE"
	StatusCanceled  = "CANCELED"
	StatusSuspended = "SUSPENDED"
)

// Durée de l'essai gratuit donnant accès à tout (niveau BUSINESS).
const TrialDurationDays = 7

// Clés de fonctionnalités (registre `features`).
const (
	FeatureOrdersCreate     = "orders.create"
	FeatureOrdersMonthlyMax = "orders.monthly.max"

	// OrdersPeriodDays : longueur de la fenêtre glissante du quota de commandes (PLAN §3.4).
	OrdersPeriodDays        = 30
	FeatureUsersMax         = "users.max"
	FeaturePriceLists       = "catalog.price_lists"
	FeatureMultiSite        = "branches.multi_site"
	FeatureReportsAdvanced  = "reports.advanced"
	FeatureExpensesAdvanced = "expenses.advanced"
	FeaturePricingPro       = "pricing.pro"
	FeatureReportsConsol    = "reports.consolidated"
	FeatureAuditAdvanced    = "audit.advanced"
	// Envoi réel des notifications clients (WhatsApp / SMS) : la composition et
	// le journal restent ouverts, seule la transmission est payante.
	FeatureNotificationsAdvanced = "notifications.advanced"
)

// Plan représente une offre commerciale.
type Plan struct {
	ID              string    `json:"id"`
	Code            string    `json:"code"`
	Name            string    `json:"name"`
	PriceMonthlyXOF float64   `json:"price_monthly_xof"`
	IsActive        bool      `json:"is_active"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Subscription est l'abonnement courant d'un pressing.
type Subscription struct {
	ID               string     `json:"id"`
	TenantID         string     `json:"tenant_id"`
	PlanID           string     `json:"plan_id"`
	PlanCode         string     `json:"plan_code"`
	PlanName         string     `json:"plan_name"`
	PriceMonthlyXOF  float64    `json:"price_monthly_xof"`
	Status           string     `json:"status"`
	TrialStartedAt   *time.Time `json:"trial_started_at,omitempty"`
	TrialEndsAt      *time.Time `json:"trial_ends_at,omitempty"`
	CurrentPeriodEnd *time.Time `json:"current_period_end,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// Entitlement est un droit effectif exposé au frontend.
type Entitlement struct {
	Key       string `json:"key"`
	Enabled   bool   `json:"enabled"`
	Limit     *int64 `json:"limit,omitempty"`
	Usage     *int64 `json:"usage,omitempty"`
	Remaining *int64 `json:"remaining,omitempty"`
	Source    string `json:"source"`
}

// EntitlementsResponse alimente l'écran « Mon abonnement ».
type EntitlementsResponse struct {
	TenantID string `json:"tenant_id"`
	// PlanCode : formule effective (droits en vigueur ; BUSINESS pendant l'essai).
	PlanCode string `json:"plan_code"`
	// SubscribedPlanCode : formule réellement souscrite (FREE pendant l'essai).
	SubscribedPlanCode string        `json:"subscribed_plan_code"`
	PlanName           string        `json:"plan_name"`
	Status             string        `json:"status"`
	OnTrial            bool          `json:"on_trial"`
	TrialEndsAt        *time.Time    `json:"trial_ends_at,omitempty"`
	DaysLeft           *int          `json:"days_left_trial,omitempty"`
	Entitlements       []Entitlement `json:"entitlements"`
	// PriceMonthlyXOF : tarif mensuel de la formule, lu en base (absent en mode mémoire).
	PriceMonthlyXOF *float64 `json:"price_monthly_xof,omitempty"`
	// OrdersPeriod : fenêtre de 30 jours sur laquelle le quota de commandes est compté.
	OrdersPeriod *Periode `json:"orders_period,omitempty"`
}

// Periode : intervalle semi-ouvert [start, end[ (end exclu).
type Periode struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// PlanCatalogue : offre publique d'une formule (tarif et droits du plan).
type PlanCatalogue struct {
	Code            string        `json:"code"`
	Name            string        `json:"name"`
	PriceMonthlyXOF *float64      `json:"price_monthly_xof"`
	Entitlements    []Entitlement `json:"entitlements"`
}
