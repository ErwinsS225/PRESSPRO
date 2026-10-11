package order

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"pressing-os/backend/internal/database"
)

// dbStore exécute les opérations sur les commandes de linge dans PostgreSQL.
type dbStore struct {
	database *database.Store
}

// NewStore construit le store de commandes persistant.
func NewStore(db *database.Store) *dbStore {
	return &dbStore{database: db}
}

// SetStore branche la persistance PostgreSQL sur un service existant.
// Appelé une seule fois au démarrage (avant de servir les requêtes).
func (s *Service) SetStore(st *dbStore) {
	s.store = st
}

// uuidRE valide le format UUID v4 (les IDs de démo type "srv-1" sont rejetés).
var uuidRE = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// isUUID indique si la chaîne est un UUID valide.
func isUUID(s string) bool { return uuidRE.MatchString(strings.TrimSpace(s)) }

// nullUUID renvoie un pointeur vers l'UUID si valide, sinon nil (colonne NULL).
func nullUUID(s string) *string {
	if !isUUID(s) {
		return nil
	}
	c := strings.TrimSpace(s)
	return &c
}

// nullIfEmptyStr renvoie nil pour une chaîne vide (colonne NULL), utile pour
// les colonnes texte optionnelles (notes, référence…).
func nullIfEmptyStr(v string) *string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	c := v
	return &c
}

// splitName découpe un nom complet en prénom / nom pour la table customers.
func splitName(full string) (string, string) {
	full = strings.TrimSpace(full)
	if full == "" {
		return "Client", ""
	}
	parts := strings.Fields(full)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], strings.Join(parts[1:], " ")
}

// parseFlexibleDate tente de parser une date dans les formats courants.
func parseFlexibleDate(v string) (time.Time, error) {
	layouts := []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02"}
	v = strings.TrimSpace(v)
	for _, l := range layouts {
		if t, err := time.Parse(l, v); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("format de date non reconnu: %q", v)
}

// dueDateString restitue l'échéance au format lisible "02 Jan à 17:00".
// Si due_date est NULL, on retombe sur created_at + 48h (comportement mémoire).
func dueDateString(due *time.Time, createdAt time.Time) string {
	t := createdAt.Add(48 * time.Hour)
	if due != nil {
		t = *due
	}
	return t.Format("02 Jan à 15:04")
}

// nextOrderNumberTx génère un numéro de ticket ATOMIQUE par tenant via la table
// order_sequences (INSERT ... ON CONFLICT DO UPDATE verrouille la ligne tenant).
func nextOrderNumberTx(ctx context.Context, tx pgx.Tx, tenantID string) (string, error) {
	var seq int
	year := time.Now().Year()
	err := tx.QueryRow(ctx, `
		INSERT INTO order_sequences (tenant_id, last_seq, updated_at)
		VALUES ($1, 1, NOW())
		ON CONFLICT (tenant_id) DO UPDATE
			SET last_seq = order_sequences.last_seq + 1, updated_at = NOW()
		RETURNING last_seq`,
		tenantID,
	).Scan(&seq)
	if err != nil {
		return "", fmt.Errorf("génération du numéro de commande: %w", err)
	}
	return fmt.Sprintf("CMD-%d-%03d", year, seq), nil
}

// upsertCustomerTx résout (ou crée) le client du dépôt. L'unicité métier est
// UNIQUE(tenant_id, phone) : un même numéro met à jour le nom existant.
func upsertCustomerTx(ctx context.Context, tx pgx.Tx, tenantID, fullName, phone string) (string, error) {
	first, last := splitName(fullName)
	phone = strings.TrimSpace(phone)
	if phone == "" {
		phone = fmt.Sprintf("anonyme-%d", time.Now().UnixNano())
	}
	var id string
	err := tx.QueryRow(ctx, `
		INSERT INTO customers (tenant_id, first_name, last_name, phone, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		ON CONFLICT (tenant_id, phone) DO UPDATE
			SET first_name = EXCLUDED.first_name,
			    last_name  = EXCLUDED.last_name,
			    updated_at = NOW()
		RETURNING id`,
		tenantID, first, last, phone,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("résolution du client: %w", err)
	}
	return id, nil
}

// primaryBranchTx renvoie l'établissement principal du tenant (NULL si absent).
func primaryBranchTx(ctx context.Context, tx pgx.Tx, tenantID string) (*string, error) {
	var id string
	err := tx.QueryRow(ctx, `
		SELECT id FROM branches
		WHERE tenant_id = $1 AND is_primary = true
		ORDER BY created_at LIMIT 1`,
		tenantID,
	).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("recherche de l'établissement principal: %w", err)
	}
	return &id, nil
}

// resolveBranchTx détermine l'établissement d'imputation de la commande :
//   - si un branch_id est fourni ET appartient bien au tenant, il est retenu ;
//   - sinon repli sûr sur l'établissement principal du tenant.
//
// Le header X-Branch-ID est déjà validé par le contrôle de périmètre (#4),
// mais on revérifie l'appartenance côté transaction par défense en profondeur.
func resolveBranchTx(ctx context.Context, tx pgx.Tx, tenantID, requested string) (*string, error) {
	if requested != "" {
		var id string
		err := tx.QueryRow(ctx, `
			SELECT id FROM branches
			WHERE id = $1 AND tenant_id = $2
			LIMIT 1`,
			requested, tenantID,
		).Scan(&id)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				// Branche inconnue ou hors périmètre : repli sur la principale.
				return primaryBranchTx(ctx, tx, tenantID)
			}
			return nil, fmt.Errorf("résolution de l'établissement: %w", err)
		}
		return &id, nil
	}
	return primaryBranchTx(ctx, tx, tenantID)
}

// CreateOrder persiste un dépôt de linge dans une transaction unique :
// numéro atomique → client → établissement → commande → lignes → paiement initial.
func (st *dbStore) CreateOrder(ctx context.Context, tenantID string, req CreateOrderRequest) (*Order, error) {
	if len(req.Items) == 0 {
		return nil, ErrEmptyOrderItems
	}

	tx, err := st.database.Pool().Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("ouverture de transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	orderNumber, err := nextOrderNumberTx(ctx, tx, tenantID)
	if err != nil {
		return nil, err
	}
	customerID, err := upsertCustomerTx(ctx, tx, tenantID, req.CustomerName, req.CustomerPhone)
	if err != nil {
		return nil, err
	}
	branchID, err := resolveBranchTx(ctx, tx, tenantID, req.BranchID)
	if err != nil {
		return nil, err
	}

	var totalAmount float64
	type itemCalc struct {
		serviceID *string
		label     string
		qty       int
		unitPrice float64
		subtotal  float64
		notes     *string
	}
	var calc []itemCalc
	for idx, it := range req.Items {
		if it.Quantity <= 0 {
			it.Quantity = 1
		}
		sub := float64(it.Quantity) * it.UnitPrice
		totalAmount += sub
		label := strings.TrimSpace(it.ItemLabel)
		if label == "" {
			label = fmt.Sprintf("Article %d", idx+1)
		}
		calc = append(calc, itemCalc{
			serviceID: nullUUID(it.ServiceID),
			label:     label,
			qty:       it.Quantity,
			unitPrice: it.UnitPrice,
			subtotal:  sub,
			notes:     nullIfEmptyStr(it.Notes),
		})
	}

	paidAmount := req.InitialPaid
	if paidAmount < 0 {
		paidAmount = 0
	} else if paidAmount > totalAmount {
		paidAmount = totalAmount
	}
	paymentStatus := PaymentStatusUnpaid
	if paidAmount >= totalAmount && totalAmount > 0 {
		paymentStatus = PaymentStatusPaid
	} else if paidAmount > 0 {
		paymentStatus = PaymentStatusPartial
	}

	var duePtr *time.Time
	if d := strings.TrimSpace(req.DueDate); d != "" {
		if t, perr := parseFlexibleDate(d); perr == nil {
			duePtr = &t
		}
	}

	var orderID string
	err = tx.QueryRow(ctx, `
		INSERT INTO orders (tenant_id, customer_id, branch_id, order_number, status, payment_status,
		                    total_amount, paid_amount, due_date, notes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
		RETURNING id`,
		tenantID, customerID, branchID, orderNumber, StatusReceived, paymentStatus,
		totalAmount, paidAmount, duePtr, nullIfEmptyStr(req.Notes),
	).Scan(&orderID)
	if err != nil {
		return nil, fmt.Errorf("insertion de la commande: %w", err)
	}

	for i, c := range calc {
		if _, err := tx.Exec(ctx, `
			INSERT INTO order_items (order_id, service_id, item_label, quantity, unit_price, subtotal, notes, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())`,
			orderID, c.serviceID, c.label, c.qty, c.unitPrice, c.subtotal, c.notes,
		); err != nil {
			return nil, fmt.Errorf("insertion de la ligne article %d: %w", i+1, err)
		}
	}

	if paidAmount > 0 {
		method := req.PaymentMethod
		if method == "" {
			method = MethodCash
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO payments (tenant_id, order_id, amount, payment_method, created_at)
			VALUES ($1, $2, $3, $4, NOW())`,
			tenantID, orderID, paidAmount, method,
		); err != nil {
			return nil, fmt.Errorf("insertion du paiement initial: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("validation de la transaction: %w", err)
	}

	// Rechargement complet (client + articles + paiements) après commit.
	created, err := st.GetOrderByID(ctx, tenantID, orderID)
	if err != nil {
		return nil, fmt.Errorf("rechargement de la commande créée: %w", err)
	}
	return created, nil
}

// orderSelect liste les colonnes de orders + le nom/téléphone du client (JOIN).
const orderSelect = `
	SELECT o.id, o.tenant_id, o.customer_id,
	       c.first_name, c.last_name, c.phone,
	       o.order_number, o.status, o.payment_status,
	       o.total_amount, o.paid_amount, o.due_date, o.notes,
	       o.created_at, o.updated_at
	FROM orders o
	JOIN customers c ON c.id = o.customer_id`

// scanOrder lit une ligne issue de orderSelect dans un *Order.
func scanOrder(row pgx.Row) (*Order, error) {
	var (
		o                  Order
		first, last, phone string
		due                *time.Time
		notes              *string
	)
	err := row.Scan(
		&o.ID, &o.TenantID, &o.CustomerID,
		&first, &last, &phone,
		&o.OrderNumber, &o.Status, &o.PaymentStatus,
		&o.TotalAmount, &o.PaidAmount, &due, &notes,
		&o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	o.CustomerName = strings.TrimSpace(first + " " + last)
	o.CustomerPhone = phone
	o.RemainingDue = o.TotalAmount - o.PaidAmount
	o.DueDate = dueDateString(due, o.CreatedAt)
	if notes != nil {
		o.Notes = *notes
	}
	return &o, nil
}

// loadOrderDetails charge les articles et les paiements d'une commande.
func (st *dbStore) loadOrderDetails(ctx context.Context, o *Order) error {
	rows, err := st.database.QueryContext(ctx, `
		SELECT id, service_id, item_label, quantity, unit_price, subtotal, notes
		FROM order_items WHERE order_id = $1 ORDER BY created_at`, o.ID)
	if err != nil {
		return fmt.Errorf("chargement des articles: %w", err)
	}
	defer rows.Close()
	o.Items = []OrderItem{}
	for rows.Next() {
		var (
			it    OrderItem
			svcID *string
			notes *string
		)
		if err := rows.Scan(&it.ID, &svcID, &it.ItemLabel, &it.Quantity, &it.UnitPrice, &it.Subtotal, &notes); err != nil {
			return fmt.Errorf("lecture d'un article: %w", err)
		}
		if svcID != nil {
			it.ServiceID = *svcID
		}
		if notes != nil {
			it.Notes = *notes
		}
		o.Items = append(o.Items, it)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("parcours des articles: %w", err)
	}

	payRows, err := st.database.QueryContext(ctx, `
		SELECT id, amount, payment_method, reference, created_at
		FROM payments WHERE order_id = $1 ORDER BY created_at`, o.ID)
	if err != nil {
		return fmt.Errorf("chargement des paiements: %w", err)
	}
	defer payRows.Close()
	o.Payments = []PaymentRecord{}
	for payRows.Next() {
		var (
			p   PaymentRecord
			ref *string
		)
		if err := payRows.Scan(&p.ID, &p.Amount, &p.PaymentMethod, &ref, &p.CreatedAt); err != nil {
			return fmt.Errorf("lecture d'un paiement: %w", err)
		}
		if ref != nil {
			p.Reference = *ref
		}
		o.Payments = append(o.Payments, p)
	}
	if err := payRows.Err(); err != nil {
		return fmt.Errorf("parcours des paiements: %w", err)
	}
	return nil
}

// GetOrderByID récupère une commande (isolation tenant_id stricte) avec son détail.
func (st *dbStore) GetOrderByID(ctx context.Context, tenantID, orderID string) (*Order, error) {
	o, err := scanOrder(st.database.QueryRowContext(ctx,
		orderSelect+" WHERE o.tenant_id = $1 AND o.id = $2", tenantID, orderID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOrderNotFound
		}
		return nil, fmt.Errorf("récupération de la commande: %w", err)
	}
	if err := st.loadOrderDetails(ctx, o); err != nil {
		return nil, err
	}
	return o, nil
}

// GetOrders liste les commandes du tenant (filtre statut + recherche texte),
// triées par date de création décroissante.
func (st *dbStore) GetOrders(ctx context.Context, tenantID, statusFilter, search string) ([]*Order, error) {
	query := orderSelect + " WHERE o.tenant_id = $1"
	args := []interface{}{tenantID}
	if statusFilter != "" && statusFilter != "all" {
		query += " AND o.status = $2"
		args = append(args, statusFilter)
	}
	if s := strings.TrimSpace(search); s != "" {
		query += fmt.Sprintf(" AND (c.phone ILIKE $%d OR c.first_name ILIKE $%d OR c.last_name ILIKE $%d OR o.order_number ILIKE $%d)",
			len(args)+1, len(args)+2, len(args)+3, len(args)+4)
		like := "%" + s + "%"
		args = append(args, like, like, like, like)
	}
	query += " ORDER BY o.created_at DESC"

	rows, err := st.database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("liste des commandes: %w", err)
	}
	defer rows.Close()
	result := []*Order{}
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, fmt.Errorf("lecture d'une commande: %w", err)
		}
		if err := st.loadOrderDetails(ctx, o); err != nil {
			return nil, err
		}
		result = append(result, o)
	}
	return result, rows.Err()
}

// UpdateOrderStatus change le statut d'une commande (isolation tenant_id stricte).
func (st *dbStore) UpdateOrderStatus(ctx context.Context, tenantID, orderID, newStatus string) (*Order, error) {
	valid := map[string]bool{
		StatusReceived: true, StatusWashing: true, StatusIroning: true,
		StatusReady: true, StatusDelivered: true, StatusCancelled: true,
	}
	if !valid[newStatus] {
		return nil, ErrInvalidStatusOrder
	}
	tag, err := st.database.ExecContext(ctx, `
		UPDATE orders SET status = $1, updated_at = NOW()
		WHERE tenant_id = $2 AND id = $3`, newStatus, tenantID, orderID)
	if err != nil {
		return nil, fmt.Errorf("mise à jour du statut: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrOrderNotFound
	}
	return st.GetOrderByID(ctx, tenantID, orderID)
}

// AddPayment enregistre un encaissement, met à jour le solde (plafonné au total)
// et le statut de paiement, le tout en transaction (verrou de ligne FOR UPDATE).
func (st *dbStore) AddPayment(ctx context.Context, tenantID, orderID string, req AddPaymentRequest) (*Order, error) {
	if req.Amount <= 0 {
		return nil, ErrInvalidPayment
	}
	tx, err := st.database.Pool().Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("ouverture de transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var total, paid float64
	err = tx.QueryRow(ctx, `
		SELECT total_amount, paid_amount FROM orders
		WHERE tenant_id = $1 AND id = $2 FOR UPDATE`, tenantID, orderID).Scan(&total, &paid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOrderNotFound
		}
		return nil, fmt.Errorf("vérification de la commande: %w", err)
	}

	newPaid := paid + req.Amount
	if newPaid > total {
		newPaid = total
	}
	newStatus := PaymentStatusPartial
	if newPaid >= total {
		newStatus = PaymentStatusPaid
	}

	method := req.PaymentMethod
	if method == "" {
		method = MethodCash
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO payments (tenant_id, order_id, amount, payment_method, reference, created_at)
		VALUES ($1, $2, $3, $4, $5, NOW())`,
		tenantID, orderID, req.Amount, method, nullIfEmptyStr(req.Reference),
	); err != nil {
		return nil, fmt.Errorf("insertion du paiement: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE orders SET paid_amount = $1, payment_status = $2, updated_at = NOW()
		WHERE tenant_id = $3 AND id = $4`, newPaid, newStatus, tenantID, orderID,
	); err != nil {
		return nil, fmt.Errorf("mise à jour du solde: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("validation de la transaction: %w", err)
	}
	return st.GetOrderByID(ctx, tenantID, orderID)
}
