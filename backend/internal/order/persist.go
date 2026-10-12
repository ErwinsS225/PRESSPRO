package order

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"pressing-os/backend/internal/billing"
	"pressing-os/backend/internal/customer"
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

// resolveCustomerTx ne fusionne jamais par téléphone et ne modifie jamais une fiche.
// La lecture et la création restent dans la transaction du dépôt.
func resolveCustomerTx(ctx context.Context, tx pgx.Tx, tenantID string, req CreateOrderRequest) (string, error) {
	if req.CustomerID != "" {
		if !isUUID(req.CustomerID) {
			return "", customer.ErrCustomerNotFound
		}
		var id, phone string
		err := tx.QueryRow(ctx, `SELECT id, phone FROM customers
			WHERE tenant_id=$1 AND id=$2 FOR SHARE`, tenantID, req.CustomerID).Scan(&id, &phone)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", customer.ErrCustomerNotFound
		}
		if err != nil {
			return "", fmt.Errorf("lecture du client du dépôt: %w", err)
		}
		if strings.TrimSpace(phone) == "" {
			return "", customer.ErrEmptyPhone
		}
		return id, nil
	}

	input, err := newCustomerInput(req)
	if err != nil {
		return "", err
	}
	if req.NewCustomer == nil {
		// Ancien contrat : l'identité est ambiguë dès qu'un numéro correspond.
		// Ne choisir aucune des fiches automatiquement, même si le nom correspond.
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM customers
			WHERE tenant_id=$1 AND phone=$2)`, tenantID, input.Phone).Scan(&exists); err != nil {
			return "", fmt.Errorf("recherche de correspondance client: %w", err)
		}
		if exists {
			return "", ErrCustomerSelectionRequired
		}
	}
	var id string
	err = tx.QueryRow(ctx, `INSERT INTO customers
		(tenant_id, first_name, last_name, phone, email, address, notes)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),NULLIF($6,''),NULLIF($7,'')) RETURNING id`,
		tenantID, input.FirstName, input.LastName, input.Phone, input.Email, input.Address, input.Notes).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("création du client du dépôt: %w", err)
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

// creerCommande persiste un dépôt de linge dans une transaction unique :
// numéro atomique → client → établissement → commande → lignes → paiement initial.
//
// Le plafond de commandes du plan est lu avant la transaction (aucune connexion
// supplémentaire pendant le verrou), puis contrôlé DANS la transaction après le
// verrou propre à l'établissement : deux créations concurrentes ne peuvent pas
// dépasser le plafond, et un échec (rollback) ne consomme aucune place.
func (st *dbStore) creerCommande(ctx context.Context, tenantID string, req CreateOrderRequest, quota *billing.QuotaGuard) (*Order, error) {
	if err := validMutationKey(req.IdempotencyKey); err != nil {
		return nil, err
	}
	finance, err := validateOrderFinance(req)
	if err != nil {
		return nil, err
	}

	var plafond billing.PlafondCommandes
	if quota != nil {
		p, err := quota.PlafondCommandes(ctx, tenantID)
		if err != nil {
			return nil, err
		}
		plafond = p
	}

	tx, err := st.database.Pool().Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("ouverture de transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	hash := mutationHash(ctx, req)
	if existing, err := replayTx(ctx, tx, tenantID, "create_order", req.IdempotencyKey, hash); err != nil || existing != nil {
		return existing, err
	}

	if quota != nil {
		if err := quota.VerifierCommandeTx(ctx, tx, tenantID, plafond); err != nil {
			return nil, err
		}
	}

	orderNumber, err := nextOrderNumberTx(ctx, tx, tenantID)
	if err != nil {
		return nil, err
	}
	customerID, err := resolveCustomerTx(ctx, tx, tenantID, req)
	if err != nil {
		return nil, err
	}
	branchID, err := resolveBranchTx(ctx, tx, tenantID, req.BranchID)
	if err != nil {
		return nil, err
	}

	totalAmount := money(finance.total)
	paidAmount := totalAmount
	paymentStatus := PaymentStatusPaid

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

	for i, it := range finance.items {
		if _, err := tx.Exec(ctx, `
			INSERT INTO order_items (order_id, service_id, item_label, quantity, unit_price, subtotal, notes, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())`,
			orderID, nullUUID(it.ServiceID), it.ItemLabel, it.Quantity, it.UnitPrice, it.Subtotal, nullIfEmptyStr(it.Notes),
		); err != nil {
			return nil, fmt.Errorf("insertion de la ligne article %d: %w", i+1, err)
		}
	}

	if finance.total > 0 {
		actorID := nullUUID(paymentActor(ctx))
		if _, err := tx.Exec(ctx, `
			INSERT INTO payments (tenant_id, order_id, amount, payment_method, amount_received, change_given, received_by, created_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())`,
			tenantID, orderID, money(finance.total), finance.method, money(finance.received), money(finance.change), actorID,
		); err != nil {
			return nil, fmt.Errorf("insertion du paiement initial: %w", err)
		}
	}

	created, err := st.getOrderByIDTx(ctx, tx, tenantID, orderID)
	if err != nil {
		return nil, fmt.Errorf("chargement de la commande créée: %w", err)
	}
	created.Persisted = true
	if err := saveReplayTx(ctx, tx, tenantID, "create_order", req.IdempotencyKey, hash, created); err != nil {
		return nil, fmt.Errorf("sauvegarde de réessai: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("validation de la transaction: %w", err)
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
	o.Persisted = true
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
		SELECT id, amount, payment_method, reference, amount_received, change_given, received_by, created_at
		FROM payments WHERE order_id = $1 ORDER BY created_at`, o.ID)
	if err != nil {
		return fmt.Errorf("chargement des paiements: %w", err)
	}
	defer payRows.Close()
	o.Payments = []PaymentRecord{}
	for payRows.Next() {
		var (
			p     PaymentRecord
			ref   *string
			recBy *string
		)
		if err := payRows.Scan(&p.ID, &p.Amount, &p.PaymentMethod, &ref, &p.AmountReceived, &p.ChangeGiven, &recBy, &p.CreatedAt); err != nil {
			return fmt.Errorf("lecture d'un paiement: %w", err)
		}
		if ref != nil {
			p.Reference = *ref
		}
		if recBy != nil {
			p.ReceivedBy = *recBy
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
	if newStatus == StatusDelivered && !settled(total, paid) {
		return nil, ErrUnpaidDelivery
	}

	tag, err := tx.Exec(ctx, `
		UPDATE orders SET status = $1, updated_at = NOW()
		WHERE tenant_id = $2 AND id = $3`, newStatus, tenantID, orderID)
	if err != nil {
		return nil, fmt.Errorf("mise à jour du statut: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrOrderNotFound
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("validation de la mise à jour statut: %w", err)
	}
	if err != nil {
		return nil, fmt.Errorf("mise à jour du statut: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrOrderNotFound
	}
	return st.GetOrderByID(ctx, tenantID, orderID)
}

// AddPayment enregistre un encaissement et met à jour le solde ainsi que le
// statut de paiement, le tout en transaction (verrou de ligne FOR UPDATE).
// Un encaissement supérieur au solde restant dû est refusé
// (ErrPaymentExceedsDue) afin de préserver l'intégrité comptable.
func (st *dbStore) AddPayment(ctx context.Context, tenantID, orderID string, req AddPaymentRequest) (*Order, error) {
	if err := validMutationKey(req.IdempotencyKey); err != nil {
		return nil, err
	}
	finance, err := validatePayment(req)
	if err != nil {
		return nil, err
	}
	tx, err := st.database.Pool().Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("ouverture de transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	hash := mutationHash(ctx, req)
	if existing, err := replayTx(ctx, tx, tenantID, "add_payment:"+orderID, req.IdempotencyKey, hash); err != nil || existing != nil {
		return existing, err
	}

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

	totalCents, err := moneyCents(total)
	if err != nil {
		return nil, err
	}
	paidCents, err := moneyCents(paid)
	if err != nil {
		return nil, err
	}
	dueCents := totalCents - paidCents
	if finance.amount > dueCents {
		return nil, ErrPaymentExceedsDue
	}
	newPaidCents := paidCents + finance.amount
	newPaid := money(newPaidCents)
	newStatus := PaymentStatusPartial
	if newPaidCents >= totalCents {
		newStatus = PaymentStatusPaid
	}

	actorID := nullUUID(paymentActor(ctx))
	if _, err := tx.Exec(ctx, `
		INSERT INTO payments (tenant_id, order_id, amount, payment_method, reference, amount_received, change_given, received_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())`,
		tenantID, orderID, money(finance.amount), finance.method, nullIfEmptyStr(req.Reference),
		money(finance.received), money(finance.change), actorID,
	); err != nil {
		return nil, fmt.Errorf("insertion du paiement: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE orders SET paid_amount = $1, payment_status = $2, updated_at = NOW()
		WHERE tenant_id = $3 AND id = $4`, newPaid, newStatus, tenantID, orderID,
	); err != nil {
		return nil, fmt.Errorf("mise à jour du solde: %w", err)
	}
	settledOrder, err := st.getOrderByIDTx(ctx, tx, tenantID, orderID)
	if err != nil {
		return nil, fmt.Errorf("chargement de la commande réglée: %w", err)
	}
	settledOrder.Persisted = true
	if err := saveReplayTx(ctx, tx, tenantID, "add_payment:"+orderID, req.IdempotencyKey, hash, settledOrder); err != nil {
		return nil, fmt.Errorf("sauvegarde du réessai paiement: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("validation de la transaction: %w", err)
	}
	return settledOrder, nil
}

func (st *dbStore) getOrderByIDTx(ctx context.Context, tx pgx.Tx, tenantID, orderID string) (*Order, error) {
	o, err := scanOrder(tx.QueryRow(ctx, orderSelect+" WHERE o.tenant_id = $1 AND o.id = $2", tenantID, orderID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}
	rows, err := tx.Query(ctx, `
		SELECT id, service_id, item_label, quantity, unit_price, subtotal, notes
		FROM order_items WHERE order_id = $1 ORDER BY created_at`, o.ID)
	if err != nil {
		return nil, err
	}
	o.Items = []OrderItem{}
	for rows.Next() {
		var it OrderItem
		var svcID, notes *string
		if err := rows.Scan(&it.ID, &svcID, &it.ItemLabel, &it.Quantity, &it.UnitPrice, &it.Subtotal, &notes); err != nil {
			rows.Close()
			return nil, err
		}
		if svcID != nil {
			it.ServiceID = *svcID
		}
		if notes != nil {
			it.Notes = *notes
		}
		o.Items = append(o.Items, it)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	payRows, err := tx.Query(ctx, `
		SELECT id, amount, payment_method, reference, amount_received, change_given, received_by, created_at
		FROM payments WHERE order_id = $1 ORDER BY created_at`, o.ID)
	if err != nil {
		return nil, err
	}
	o.Payments = []PaymentRecord{}
	for payRows.Next() {
		var p PaymentRecord
		var ref, recBy *string
		if err := payRows.Scan(&p.ID, &p.Amount, &p.PaymentMethod, &ref, &p.AmountReceived, &p.ChangeGiven, &recBy, &p.CreatedAt); err != nil {
			payRows.Close()
			return nil, err
		}
		if ref != nil {
			p.Reference = *ref
		}
		if recBy != nil {
			p.ReceivedBy = *recBy
		}
		o.Payments = append(o.Payments, p)
	}
	payRows.Close()
	return o, payRows.Err()
}
