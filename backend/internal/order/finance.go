package order

import (
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Échelle existante NUMERIC(12,2), conservée sur l'API. Tous les calculs
// financiers se font en centièmes entiers ; aucun arrondi implicite d'entrée.
const maxMoneyCents int64 = 999999999999

func moneyCents(v float64) (int64, error) {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return 0, ErrInvalidPayment
	}
	r, ok := new(big.Rat).SetString(strconv.FormatFloat(v, 'f', -1, 64))
	if !ok {
		return 0, ErrInvalidPayment
	}
	r.Mul(r, big.NewRat(100, 1))
	if !r.IsInt() || !r.Num().IsInt64() {
		return 0, ErrInvalidPayment
	}
	n := r.Num().Int64()
	if n > maxMoneyCents {
		return 0, ErrInvalidPayment
	}
	return n, nil
}
func money(n int64) float64         { return float64(n) / 100 }
func moneyPointer(n int64) *float64 { v := money(n); return &v }
func paymentMethod(v string) (string, error) {
	if v == "" {
		v = MethodCash
	}
	switch v {
	case MethodCash, MethodWave, MethodOrangeMoney, MethodMTNMoMo, MethodCard:
		return v, nil
	}
	return "", ErrInvalidPaymentMethod
}

type orderFinance struct {
	items                   []OrderItem
	total, received, change int64
	method                  string
}

func validateOrderFinance(req CreateOrderRequest) (orderFinance, error) {
	var f orderFinance
	if len(req.Items) == 0 {
		return f, ErrEmptyOrderItems
	}
	for _, it := range req.Items {
		price, err := moneyCents(it.UnitPrice)
		label := strings.TrimSpace(it.ItemLabel)
		if err != nil || it.Quantity <= 0 || int64(it.Quantity) > 2147483647 || label == "" || utf8.RuneCountInString(label) > 255 {
			return f, ErrInvalidOrderItem
		}
		if price > 0 && int64(it.Quantity) > (maxMoneyCents-f.total)/price {
			return f, ErrInvalidOrderItem
		}
		sub := price * int64(it.Quantity)
		f.total += sub
		f.items = append(f.items, OrderItem{ServiceID: it.ServiceID, ItemLabel: label, Quantity: it.Quantity, UnitPrice: money(price), Subtotal: money(sub), Notes: it.Notes})
	}
	received := req.InitialPaid
	if req.AmountReceived != nil {
		if req.InitialPaid != 0 {
			return f, ErrInvalidPayment
		}
		received = *req.AmountReceived
	}
	var err error
	f.received, err = moneyCents(received)
	if err != nil {
		return f, err
	}
	if f.received < f.total {
		return f, ErrFullPaymentRequired
	}
	// Une prestation gratuite n'est pas un mouvement d'encaissement.
	if f.total == 0 && f.received != 0 {
		return f, ErrInvalidPayment
	}
	f.change = f.received - f.total
	f.method, err = paymentMethod(req.PaymentMethod)
	return f, err
}

type paymentFinance struct {
	amount, received, change int64
	method                   string
}

func validatePayment(req AddPaymentRequest) (paymentFinance, error) {
	var f paymentFinance
	var err error
	f.amount, err = moneyCents(req.Amount)
	if err != nil || f.amount <= 0 {
		return f, ErrInvalidPayment
	}
	received := req.Amount
	if req.AmountReceived != nil {
		received = *req.AmountReceived
	}
	f.received, err = moneyCents(received)
	if err != nil || f.received < f.amount {
		return f, ErrInvalidPayment
	}
	f.change = f.received - f.amount
	f.method, err = paymentMethod(req.PaymentMethod)
	if utf8.RuneCountInString(req.Reference) > 100 {
		return f, ErrInvalidPayment
	}
	return f, err
}
func settled(total, paid float64) bool {
	t, e1 := moneyCents(total)
	p, e2 := moneyCents(paid)
	return e1 == nil && e2 == nil && t == p
}
