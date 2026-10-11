package expense

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"pressing-os/backend/internal/database"
)

// Store est le contrat de persistance des dépenses.
type Store interface {
	GetExpenses(ctx context.Context, tenantID string, f Filter) ([]*Expense, error)
	GetExpense(ctx context.Context, tenantID, id string) (*Expense, error)
	CreateExpense(ctx context.Context, tenantID, userID string, req CreateExpenseRequest) (*Expense, error)
	DeleteExpense(ctx context.Context, tenantID, id string) error
}

// Service expose la logique métier des dépenses. Sans base de données, il
// fonctionne en mémoire (mode démonstration).
type Service struct {
	store Store

	mu       sync.RWMutex
	memoire  map[string][]*Expense
	sequence int
}

// NewService crée le service en mode mémoire.
func NewService() *Service {
	return &Service{memoire: make(map[string][]*Expense)}
}

// NewStore construit un service adossé à PostgreSQL.
func NewStore(db *database.Store) *Service {
	return &Service{store: &dbStore{database: db}, memoire: make(map[string][]*Expense)}
}

// SetStore branche la persistance sur un service existant.
func (s *Service) SetStore(store Store) {
	s.store = store
}

func normalize(v string) string { return strings.TrimSpace(v) }

// valider normalise et contrôle une demande de création.
func valider(req *CreateExpenseRequest) error {
	req.Label = normalize(req.Label)
	req.Category = strings.ToLower(normalize(req.Category))
	req.PaymentMethod = normalize(req.PaymentMethod)
	req.BranchID = normalize(req.BranchID)
	req.Note = normalize(req.Note)
	req.SpentAt = normalize(req.SpentAt)

	if req.Label == "" {
		return ErrEmptyLabel
	}
	if req.AmountXOF <= 0 {
		return ErrInvalidAmount
	}
	if req.Category == "" {
		req.Category = "divers"
	}
	if !CategorieValide(req.Category) {
		return ErrInvalidCategory
	}
	if req.SpentAt == "" {
		req.SpentAt = time.Now().UTC().Format("2006-01-02")
	} else if _, err := time.Parse("2006-01-02", req.SpentAt); err != nil {
		return ErrInvalidDate
	}
	return nil
}

// GetExpenses liste les dépenses de la période demandée (défaut : mois courant).
func (s *Service) GetExpenses(ctx context.Context, tenantID string, f Filter) ([]*Expense, error) {
	if s.store != nil {
		return s.store.GetExpenses(ctx, tenantID, f)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Expense
	for _, e := range s.memoire[tenantID] {
		if f.From != "" && e.SpentAt < f.From {
			continue
		}
		if f.To != "" && e.SpentAt > f.To {
			continue
		}
		if f.Category != "" && e.Category != f.Category {
			continue
		}
		if f.BranchID != "" && e.BranchID != f.BranchID {
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SpentAt > out[j].SpentAt })
	return out, nil
}

// CreateExpense valide puis enregistre une dépense.
func (s *Service) CreateExpense(ctx context.Context, tenantID, userID string, req CreateExpenseRequest) (*Expense, error) {
	if err := valider(&req); err != nil {
		return nil, err
	}
	if s.store != nil {
		return s.store.CreateExpense(ctx, tenantID, userID, req)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.sequence++
	now := time.Now().UTC()
	e := &Expense{
		ID:       fmt.Sprintf("expense-memoire-%d", s.sequence),
		TenantID: tenantID,
		BranchID: req.BranchID,
		Category: req.Category, Label: req.Label, AmountXOF: req.AmountXOF,
		PaymentMethod: req.PaymentMethod, SpentAt: req.SpentAt,
		Note: req.Note, CreatedBy: userID, CreatedAt: now, UpdatedAt: now,
	}
	s.memoire[tenantID] = append(s.memoire[tenantID], e)
	return e, nil
}

// DeleteExpense supprime une dépense du pressing (isolation par tenant_id).
func (s *Service) DeleteExpense(ctx context.Context, tenantID, id string) error {
	if s.store != nil {
		return s.store.DeleteExpense(ctx, tenantID, id)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	liste := s.memoire[tenantID]
	for i, e := range liste {
		if e.ID == id {
			s.memoire[tenantID] = append(liste[:i], liste[i+1:]...)
			return nil
		}
	}
	return ErrExpenseNotFound
}

// Total retourne la somme des dépenses (utilisé par les rapports et l'écran).
func Total(depenses []*Expense) float64 {
	var total float64
	for _, e := range depenses {
		total += e.AmountXOF
	}
	return total
}

// ParCategorie agrège les dépenses par catégorie, du poste le plus lourd au plus léger.
func ParCategorie(depenses []*Expense) []CategoryTotal {
	index := make(map[string]*CategoryTotal)
	for _, e := range depenses {
		c, ok := index[e.Category]
		if !ok {
			c = &CategoryTotal{Category: e.Category}
			index[e.Category] = c
		}
		c.TotalXOF += e.AmountXOF
		c.Count++
	}
	out := make([]CategoryTotal, 0, len(index))
	for _, c := range index {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TotalXOF > out[j].TotalXOF })
	return out
}

// timeNow retourne la date du jour (AAAA-MM-JJ) — isolé pour les tests.
func timeNow() string { return time.Now().UTC().Format("2006-01-02") }
